# Microservicio DB Go

Microservicio de persistencia MongoDB escrito en Go para el proyecto pdf-extractext.

## Arquitectura

```
microservicio-db-go/
├── cmd/api/                 # Punto de entrada (main.go)
├── config/                  # Configuración validada via variables de entorno
├── domain/                  # Modelos de dominio, puertos y errores
│   ├── document.go          # DocumentCreate, DocumentResponse, DocumentUpdate, PageQuery
│   ├── errors.go            # Errores de dominio (DocumentNotFound, InvalidID, AlreadyExists)
│   └── ports.go             # Interfaces DocumentRepository, HealthRepository
├── application/             # Casos de uso (lógica de negocio)
│   ├── document_usecase.go  # Orquesta operaciones de documentos
│   └── health_usecase.go    # Verifica salud de BD
├── infrastructure/mongo/    # Adaptador MongoDB (implementa puertos)
│   └── repository.go        # CRUD + índices + health check
└── presentation/            # Capa HTTP (Gin)
    ├── config.go            # Configuración de presentación
    ├── problem.go           # RFC 9457 Problem Details
    ├── handler.go           # Handlers HTTP
    └── router.go            # Rutas y middleware
```

## API Endpoints

### Health
- `GET /health/live` - Liveness probe (solo verifica que el proceso responde)
- `GET /health` - Readiness probe (verifica conectividad MongoDB)

### Documents
- `POST /api/v1/documents` - Crear documento
- `GET /api/v1/documents` - Listar documentos (paginación: `skip`, `limit`)
- `GET /api/v1/documents/:id` - Obtener documento por ID
- `PATCH /api/v1/documents/:id` - Actualizar documento (PATCH semántico)
- `DELETE /api/v1/documents/:id` - Eliminar documento

## Contrato de Errores (RFC 9457)

Todas las respuestas de error usan `application/problem+json`:

```json
{
  "type": "https://errors.example.com/document-not-found",
  "title": "Documento no encontrado",
  "status": 404,
  "detail": "El documento solicitado no existe.",
  "instance": "/api/v1/documents/60d5ecb8b392d70008051234"
}
```

Códigos de error mapeados:
- `document-not-found` → 404
- `invalid-document-id` → 400
- `document-already-exists` → 409
- `too-large` → 413
- `service-unavailable` → 503
- `busy` → 503
- `server-error` → 500

## Variables de Entorno

| Variable | Requerida | Default | Descripción |
|----------|-----------|---------|-------------|
| `MONGODB_URI` | Sí | - | URI de conexión MongoDB |
| `MONGODB_DB` | No | `pdf_db` | Nombre de base de datos |
| `MONGODB_COLLECTION` | No | `extracted_texts` | Nombre de colección |
| `PORT` | No | `8080` | Puerto HTTP |
| `MAX_UPLOAD_MB` | No | `25` | Tamaño máx. subida en MB |
| `MAX_IN_FLIGHT` | No | `32` | Subidas simultáneas máx. |
| `REQUEST_TIMEOUT` | No | `10s` | Timeout por request |
| `SHUTDOWN_TIMEOUT` | No | `15s` | Timeout apagado grácil |
| `ERR_BASE_URL` | No | `https://errors.example.com` | Base URL para tipos RFC 9457 |
| `TZ` | No | `UTC` | Zona horaria |
| `SHARED_NETWORK_NAME` | No | `default-shared-network` | Red Docker compartida |

## Desarrollo

```bash
# Instalar dependencias
go mod tidy

# Ejecutar tests
go test ./...

# Verificar código
go vet ./...

# Formatear
gofmt -w .

# Build
go build -o microservicio-db-go ./cmd/api
```

## Docker

```bash
# Build imagen
docker build -t microservicio-db-go:v1.0.0 .

# Desde esta carpeta, iniciar primero MongoDB en la red compartida
docker compose --env-file ..\pdf-extractext\.env -f ..\pdf-extractext\docker-compose.db.yml up -d

# Iniciar el microservicio con su configuración local en .env
docker compose --env-file .env -f docker-compose.yml up --build -d
```

## Integración con FastAPI (Orquestador)

El orquestador FastAPI reemplaza su acceso directo a MongoDB con un adaptador HTTP que implementa `IDocumentRepository` y `IHealthRepository`, llamando a este microservicio Go.

El adaptador Python:
- Implementa el puerto `IDocumentRepository` consumido por `DocumentService`
- Aplica timeout configurable
- Convierte respuestas HTTP y errores a modelos/excepciones de dominio existentes
- Es testeable con HTTP client mockeado (sin MongoDB real)
- No tiene reintentos automáticos para operaciones no idempotentes