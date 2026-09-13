# ---- Frontend build ----
FROM node:20-alpine AS fe
WORKDIR /fe
COPY frontend/package*.json ./
RUN npm install --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- Backend build ----
# Note: the image version must be >= the go directive version in backend/go.mod
# APP_VERSION: version injected at build time (compose passes JNEXUS_VERSION through), e.g.:
#   JNEXUS_VERSION="1.$(git rev-list --count HEAD)" docker compose up -d --build
# If unset, the runtime fallback chain is: JNEXUS_VERSION env var > git inside the container > VERSION file > 1.0
FROM golang:1.25-alpine AS be
ARG APP_VERSION
WORKDIR /be
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X jnexus/internal/buildinfo.Version=${APP_VERSION}" -o /jnexus-server ./cmd/server

# ---- Runtime image ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 jnexus
WORKDIR /app
COPY --from=be /jnexus-server /app/jnexus-server
COPY --from=fe /fe/dist /app/frontend/dist
COPY backend/config.example.yaml /app/config.example.yaml
COPY VERSION /app/VERSION
COPY deploy/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh && mkdir -p /app/data /app/uploads && chown jnexus /app/data /app/uploads
ENV GIN_MODE=release
EXPOSE 8080
USER jnexus
ENTRYPOINT ["/app/entrypoint.sh"]
