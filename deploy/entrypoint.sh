#!/bin/sh
# AutoOps 容器入口：首次启动自动生成并持久化 AES 主密钥与 JWT 密钥。
# 密钥保存在数据卷（/app/data），容器重建不丢失；通过环境变量显式传入时优先使用。
set -e

DATA_DIR="${AUTOOPS_DATA_DIR:-/app/data}"
mkdir -p "$DATA_DIR"

# 密钥来源优先级：环境变量 > 数据卷已有文件 > 自动生成。
# 无论来源如何都持久化到数据卷，保证后续重启/升级使用同一密钥（避免已加密数据无法解密）。
if [ -z "$AUTOOPS_AES_KEY" ] && [ -f "$DATA_DIR/aes_key" ]; then
  AUTOOPS_AES_KEY="$(cat "$DATA_DIR/aes_key")"
  export AUTOOPS_AES_KEY
fi
if [ -z "$AUTOOPS_AES_KEY" ]; then
  /app/autoops-server -genkey > "$DATA_DIR/aes_key"
  chmod 600 "$DATA_DIR/aes_key"
  AUTOOPS_AES_KEY="$(cat "$DATA_DIR/aes_key")"
  export AUTOOPS_AES_KEY
  echo "[entrypoint] 已生成 AES 主密钥并保存到 $DATA_DIR/aes_key"
elif [ ! -f "$DATA_DIR/aes_key" ]; then
  printf '%s' "$AUTOOPS_AES_KEY" > "$DATA_DIR/aes_key"
  chmod 600 "$DATA_DIR/aes_key"
  echo "[entrypoint] 已将环境变量提供的 AES 主密钥持久化到 $DATA_DIR/aes_key"
fi

if [ -z "$AUTOOPS_JWT_SECRET" ] && [ -f "$DATA_DIR/jwt_secret" ]; then
  AUTOOPS_JWT_SECRET="$(cat "$DATA_DIR/jwt_secret")"
  export AUTOOPS_JWT_SECRET
fi
if [ -z "$AUTOOPS_JWT_SECRET" ]; then
  head -c 48 /dev/urandom | base64 > "$DATA_DIR/jwt_secret"
  chmod 600 "$DATA_DIR/jwt_secret"
  AUTOOPS_JWT_SECRET="$(cat "$DATA_DIR/jwt_secret")"
  export AUTOOPS_JWT_SECRET
  echo "[entrypoint] 已生成 JWT 密钥并保存到 $DATA_DIR/jwt_secret"
elif [ ! -f "$DATA_DIR/jwt_secret" ]; then
  printf '%s' "$AUTOOPS_JWT_SECRET" > "$DATA_DIR/jwt_secret"
  chmod 600 "$DATA_DIR/jwt_secret"
  echo "[entrypoint] 已将环境变量提供的 JWT 密钥持久化到 $DATA_DIR/jwt_secret"
fi

exec /app/autoops-server -config /app/config.yaml
