#!/bin/sh
# AutoOps 容器入口：首次启动自动生成并持久化 AES 主密钥与 JWT 密钥。
# 密钥保存在数据卷（/app/data），容器重建不丢失；通过环境变量显式传入时优先使用。
set -e

DATA_DIR="${AUTOOPS_DATA_DIR:-/app/data}"
mkdir -p "$DATA_DIR"

if [ -z "$AUTOOPS_AES_KEY" ]; then
  if [ ! -f "$DATA_DIR/aes_key" ]; then
    /app/autoops-server -genkey > "$DATA_DIR/aes_key"
    chmod 600 "$DATA_DIR/aes_key"
    echo "[entrypoint] 已生成 AES 主密钥并保存到 $DATA_DIR/aes_key"
  fi
  AUTOOPS_AES_KEY="$(cat "$DATA_DIR/aes_key")"
  export AUTOOPS_AES_KEY
fi

if [ -z "$AUTOOPS_JWT_SECRET" ]; then
  if [ ! -f "$DATA_DIR/jwt_secret" ]; then
    head -c 48 /dev/urandom | base64 > "$DATA_DIR/jwt_secret"
    chmod 600 "$DATA_DIR/jwt_secret"
    echo "[entrypoint] 已生成 JWT 密钥并保存到 $DATA_DIR/jwt_secret"
  fi
  AUTOOPS_JWT_SECRET="$(cat "$DATA_DIR/jwt_secret")"
  export AUTOOPS_JWT_SECRET
fi

exec /app/autoops-server -config /app/config.yaml
