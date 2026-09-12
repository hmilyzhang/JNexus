#!/bin/sh
# JNexus 容器入口：首次启动自动生成并持久化 AES 主密钥与 JWT 密钥。
# 密钥保存在数据卷（/app/data），容器重建不丢失；通过环境变量显式传入时优先使用。
set -e

DATA_DIR="${JNEXUS_DATA_DIR:-/app/data}"
mkdir -p "$DATA_DIR"

# 密钥来源优先级：环境变量 > 数据卷已有文件 > 自动生成。
# 无论来源如何都持久化到数据卷，保证后续重启/升级使用同一密钥（避免已加密数据无法解密）。
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

# RDP 网关密钥：写入数据卷，供 rdp-gateway 容器只读挂载（同 JWT/AES 的自动生成模式）
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

# 主机自定义 CA 证书（可选：compose 挂载 /cacerts 只读目录，放入 .crt/.pem）：
# 与系统 CA 合并生成信任 bundle，通过 SSL_CERT_FILE 供 Go TLS 使用
# （LDAPS / HTTPS 拨测 / Kubernetes 自签证书即可被容器信任，无需 root）。
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
