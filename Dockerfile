# Build stage
FROM golang:1.27-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS builder

WORKDIR /app

# Instalar dependencias de build
RUN apk add --no-cache git

# Copiar go.mod y go.sum para cache de dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Build del binario
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o microservicio-db-go ./cmd/api

# Final stage
FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
ENV GIN_MODE=release

# Parchea el SO y añade ca-certificates para HTTPS y curl para healthcheck (D2).
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl

WORKDIR /app

# Copiar binario desde builder
COPY --from=builder /app/microservicio-db-go .

# Usuario no-root con UID/GID fijos (D1).
RUN addgroup -S -g 10001 app \
    && adduser -S -u 10001 -G app -h /app -s /sbin/nologin app
USER 10001:10001

EXPOSE 8080

# Healthcheck para Docker (liveness)
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health/live || exit 1

ENTRYPOINT ["./microservicio-db-go"]