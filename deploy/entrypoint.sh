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

# Host-provided custom CA certificates (optional: compose mounts a read-only /cacerts directory with .crt/.pem files):
# Merged with the system CAs into a trust bundle exposed via SSL_CERT_FILE for Go TLS
# (so LDAPS / HTTPS checks / Kubernetes self-signed certs are trusted by the container, no root needed).
EXTRA_CA_DIR=/cacerts
if [ -d "$EXTRA_CA_DIR" ] && ls "$EXTRA_CA_DIR"/*.crt "$EXTRA_CA_DIR"/*.pem >/dev/null 2>&1; then
  BUNDLE="$DATA_DIR/ca-bundle.crt"
  {
    cat /etc/ssl/certs/ca-certificates.crt 2>/dev/null
    cat "$EXTRA_CA_DIR"/*.crt "$EXTRA_CA_DIR"/*.pem 2>/dev/null
  } > "$BUNDLE"
  export SSL_CERT_FILE="$BUNDLE"
  echo "[entrypoint] CA bundle merged: $BUNDLE"
fi

exec /app/jnexus-server -config /app/config.yaml
