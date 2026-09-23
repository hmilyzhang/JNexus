#!/bin/sh
# JNexus container entrypoint: on first start, auto-generates and persists the AES master key and JWT secret.
# Secrets are kept on the data volume (/app/data) and survive container recreation; env vars take precedence when explicitly provided.
set -e

DATA_DIR="${JNEXUS_DATA_DIR:-/app/data}"
mkdir -p "$DATA_DIR"

# Secret source priority: env var > existing file on the data volume > auto-generated.
# Whatever the source, the secret is persisted to the data volume so restarts/upgrades reuse the same key (otherwise already-encrypted data could not be decrypted).
if [ -z "$JNEXUS_AES_KEY" ] && [ -f "$DATA_DIR/aes_key" ]; then
  JNEXUS_AES_KEY="$(cat "$DATA_DIR/aes_key")"
  export JNEXUS_AES_KEY
fi
if [ -z "$JNEXUS_AES_KEY" ]; then
  /app/jnexus-server -genkey > "$DATA_DIR/aes_key"
  chmod 600 "$DATA_DIR/aes_key"
  JNEXUS_AES_KEY="$(cat "$DATA_DIR/aes_key")"
  export JNEXUS_AES_KEY
  echo "[entrypoint] 已生成 AES 主密钥并保存到 $DATA_DIR/aes_key"
elif [ ! -f "$DATA_DIR/aes_key" ]; then
  printf '%s' "$JNEXUS_AES_KEY" > "$DATA_DIR/aes_key"
  chmod 600 "$DATA_DIR/aes_key"
  echo "[entrypoint] 已将环境变量提供的 AES 主密钥持久化到 $DATA_DIR/aes_key"
fi

if [ -z "$JNEXUS_JWT_SECRET" ] && [ -f "$DATA_DIR/jwt_secret" ]; then
  JNEXUS_JWT_SECRET="$(cat "$DATA_DIR/jwt_secret")"
  export JNEXUS_JWT_SECRET
fi
if [ -z "$JNEXUS_JWT_SECRET" ]; then
  head -c 48 /dev/urandom | base64 > "$DATA_DIR/jwt_secret"
  chmod 600 "$DATA_DIR/jwt_secret"
  JNEXUS_JWT_SECRET="$(cat "$DATA_DIR/jwt_secret")"
  export JNEXUS_JWT_SECRET
  echo "[entrypoint] 已生成 JWT 密钥并保存到 $DATA_DIR/jwt_secret"
elif [ ! -f "$DATA_DIR/jwt_secret" ]; then
  printf '%s' "$JNEXUS_JWT_SECRET" > "$DATA_DIR/jwt_secret"
  chmod 600 "$DATA_DIR/jwt_secret"
  echo "[entrypoint] 已将环境变量提供的 JWT 密钥持久化到 $DATA_DIR/jwt_secret"
fi

# RDP gateway secret: written to the data volume for the rdp-gateway container to mount read-only (same auto-generation pattern as JWT/AES)
if [ -z "$GW_SECRET" ] && [ -f "$DATA_DIR/gw_secret" ]; then
  GW_SECRET="$(cat "$DATA_DIR/gw_secret")"
  export GW_SECRET
fi
if [ -z "$GW_SECRET" ]; then
  head -c 24 /dev/urandom | base64 > "$DATA_DIR/gw_secret"
  chmod 600 "$DATA_DIR/gw_secret"
  GW_SECRET="$(cat "$DATA_DIR/gw_secret")"
  export GW_SECRET
  echo "[entrypoint] 已生成 RDP 网关密钥并保存到 $DATA_DIR/gw_secret"
elif [ ! -f "$DATA_DIR/gw_secret" ]; then
  printf '%s' "$GW_SECRET" > "$DATA_DIR/gw_secret"
  chmod 600 "$DATA_DIR/gw_secret"
  echo "[entrypoint] 已将环境变量提供的 RDP 网关密钥持久化到 $DATA_DIR/gw_secret"
fi

# Host-provided custom CA certificates (optional: compose mounts a read-only /cacerts directory with .crt/.pem files),
# plus optional JNEXUS_TRUST_CA_URL — one or more URLs to fetch internal CA root certificates from at startup
# (e.g. the company CA served on the Docker host: http://host.docker.internal:8899/root-ca.crt, needs the
# "host.docker.internal:host-gateway" extra_host on Linux). Both are merged with the system CAs into a trust
# bundle exposed via SSL_CERT_FILE for Go TLS — LDAPS / HTTPS checks / Kubernetes internal certs are trusted
# by the container, no root needed. PEM content is validated (an HTML error page is never trusted).
EXTRA_CA_DIR="${EXTRA_CA_DIR:-/cacerts}"
CA_TMP="$DATA_DIR/extra-ca"
mkdir -p "$CA_TMP"

fetch_url() {
    _url="$1" _out="$2"
    if command -v wget >/dev/null 2>&1; then
        wget -q -T 15 -O "$_out" "$_url" || return 1
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL -m 15 -o "$_out" "$_url" || return 1
    else
        return 1
    fi
    return 0
}

if [ -n "$JNEXUS_TRUST_CA_URL" ]; then
  n=0
  for u in $(echo "$JNEXUS_TRUST_CA_URL" | tr ',;' '  '); do
    n=$((n + 1))
    f="$CA_TMP/ca-$n.crt"
    if fetch_url "$u" "$f" && grep -q "BEGIN CERTIFICATE" "$f"; then
      echo "[entrypoint] 已从 $u 获取内部 CA 证书"
    else
      echo "[entrypoint] 警告: 从 $u 获取证书失败或内容不是 PEM，已忽略"
      rm -f "$f"
    fi
  done
fi

BUNDLE="$DATA_DIR/ca-bundle.crt"
{
  cat /etc/ssl/certs/ca-certificates.crt 2>/dev/null || true
  cat "$EXTRA_CA_DIR"/*.crt "$EXTRA_CA_DIR"/*.pem "$CA_TMP"/*.crt 2>/dev/null || true
} > "$BUNDLE"
if grep -q "BEGIN CERTIFICATE" "$BUNDLE" 2>/dev/null; then
  export SSL_CERT_FILE="$BUNDLE"
  echo "[entrypoint] CA bundle merged: $BUNDLE"
fi
rm -rf "$CA_TMP"

# testability hook: verify the CA/secret setup without starting the server
if [ "$JNEXUS_ENTRYPOINT_TEST" = "1" ]; then
  echo "[entrypoint] TEST mode: setup complete"
  exit 0
fi

exec /app/jnexus-server -config /app/config.yaml
