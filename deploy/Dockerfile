# ---- 前端构建 ----
FROM node:20-alpine AS fe
WORKDIR /fe
COPY frontend/package*.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 后端构建 ----
# 注意：镜像版本需 >= backend/go.mod 的 go 指令版本
# APP_VERSION：构建时注入版本号（compose 从 AUTOOPS_VERSION 透传），例如：
#   AUTOOPS_VERSION="1.$(git rev-list --count HEAD)" docker compose up -d --build
# 不传时运行时回退链：AUTOOPS_VERSION 环境变量 > 容器内 git > VERSION 文件 > 1.0
FROM golang:1.25-alpine AS be
ARG APP_VERSION
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X autoops/internal/buildinfo.Version=${APP_VERSION}" -o /autoops-server ./cmd/server

# ---- 运行镜像 ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 autoops
WORKDIR /app
COPY --from=be /autoops-server /app/autoops-server
COPY --from=fe /fe/dist /app/frontend/dist
COPY backend/config.example.yaml /app/config.example.yaml
COPY deploy/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh && mkdir -p /app/data /app/uploads && chown autoops /app/data /app/uploads
ENV GIN_MODE=release
EXPOSE 8080
USER autoops
ENTRYPOINT ["/app/entrypoint.sh"]
