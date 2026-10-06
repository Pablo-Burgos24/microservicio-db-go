# Build stage
FROM golang:1.27-alpine AS builder

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
FROM alpine:3.20
ENV GIN_MODE=release

# Instalar ca-certificates para HTTPS y curl para healthcheck
RUN apk add --no-cache ca-certificates curl

WORKDIR /app

# Copiar binario desde builder
COPY --from=builder /app/microservicio-db-go .

# Usuario no-root para seguridad
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

# Healthcheck para Docker (liveness)
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health/live || exit 1

ENTRYPOINT ["./microservicio-db-go"]