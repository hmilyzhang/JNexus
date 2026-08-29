# ---- 前端构建 ----
FROM node:20-alpine AS fe
WORKDIR /fe
COPY frontend/package*.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- 后端构建 ----
FROM golang:1.22-alpine AS be
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /autoops-server ./cmd/server

# ---- 运行镜像 ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 autoops
WORKDIR /app
COPY --from=be /autoops-server /app/autoops-server
COPY --from=fe /fe/dist /app/frontend/dist
COPY backend/config.example.yaml /app/config.example.yaml
ENV GIN_MODE=release
EXPOSE 8080
USER autoops
ENTRYPOINT ["/app/autoops-server", "-config", "/app/config.yaml"]
