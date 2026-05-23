# PawonWarga Backend

RESTful API backend for PawonWarga, built with Go and Gin.

## Tech Stack

| Layer | Technology |
|---|---|
| Framework | [Gin](https://github.com/gin-gonic/gin) |
| Database | TimescaleDB / PostgreSQL via [GORM](https://gorm.io) |
| Cache | Redis via [go-redis v9](https://github.com/redis/go-redis) |
| Auth | HTTP Basic Auth + JWT (HS256) |
| Storage | S3-compatible object storage (AWS SDK v2) |
| API Docs | Swagger via [swaggo](https://github.com/swaggo/swag) |

---

## Project Structure

```
PawonWarga-BE/
├── main.go                        # Entry point, Swagger annotations
├── docs/                          # Auto-generated Swagger files (do not edit)
├── internal/
│   ├── config/config.go           # All config loaded from environment
│   ├── server/server.go           # HTTP server, graceful shutdown
│   ├── router/router.go           # Route registration
│   ├── middleware/
│   │   ├── auth.go                # HTTP Basic Auth
│   │   ├── jwt.go                 # JWT Bearer Auth
│   │   └── logger.go              # Structured request logging
│   ├── handler/
│   │   ├── health.go              # GET /health
│   │   └── auth.go                # Auth & profile handlers
│   ├── service/
│   │   └── auth.go                # Business logic
│   ├── repository/
│   │   └── user.go                # Database access layer
│   └── model/
│       ├── base.go                # Shared BaseModel (id, created_at, updated_at)
│       └── user.go                # User entity
└── pkg/
    ├── cache/redis.go             # Redis client wrapper
    ├── database/postgres.go       # TimescaleDB connection + pool config
    ├── jwtutil/jwt.go             # Token generation & parsing
    ├── response/response.go       # Typed API response helpers
    └── storage/
        ├── storage.go             # Provider-agnostic Storage interface
        └── s3.go                  # S3-compatible implementation
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- TimescaleDB or PostgreSQL
- Redis
- S3-compatible object storage (optional, required for profile pictures)
- `swag` CLI — install once:
  ```bash
  go install github.com/swaggo/swag/cmd/swag@latest
  ```

### Setup

```bash
# 1. Clone and enter the project
git clone <repo-url>
cd PawonWarga-BE

# 2. Copy and fill in environment variables
cp .env.example .env

# 3. Install dependencies
go mod tidy

# 4. Generate Swagger docs
swag init --parseDependency --parseInternal

# 5. Run
go run main.go
```

The server starts on `http://localhost:8080` (or the port in `SERVER_PORT`).

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP listen port |
| `GIN_MODE` | `debug` | `debug` / `release` / `test` |
| `DB_HOST` | `localhost` | TimescaleDB host |
| `DB_PORT` | `5432` | TimescaleDB port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | — | Database password |
| `DB_NAME` | `pawonwarga` | Database name |
| `DB_SSLMODE` | `disable` | SSL mode |
| `REDIS_ENABLED` | `true` | Enable Redis cache |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | — | Redis password |
| `AUTH_USERNAME` | `admin` | HTTP Basic Auth username |
| `AUTH_PASSWORD` | `secret` | HTTP Basic Auth password |
| `JWT_SECRET` | — | JWT signing secret (**change in production**) |
| `JWT_EXPIRY_HOURS` | `24` | Token lifetime in hours |
| `STORAGE_ENDPOINT` | — | S3 endpoint URL (empty = real AWS) |
| `STORAGE_REGION` | `us-east-1` | S3 region |
| `STORAGE_BUCKET` | — | Bucket name (empty = storage disabled) |
| `STORAGE_ACCESS_KEY_ID` | — | S3 access key |
| `STORAGE_SECRET_ACCESS_KEY` | — | S3 secret key |
| `STORAGE_PUBLIC_BASE_URL` | — | Public base URL for uploaded files |
| `STORAGE_FORCE_PATH_STYLE` | `false` | `true` for Minio and some providers |

---

## API Endpoints

### System

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | None | Health check |
| `GET` | `/swagger/index.html` | None | API documentation |

### Auth

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | None | Register a new user |
| `POST` | `/api/v1/auth/login` | None | Login, returns JWT token |
| `GET` | `/api/v1/auth/profile` | JWT | Get own profile |
| `PUT` | `/api/v1/auth/profile` | JWT | Update name |
| `POST` | `/api/v1/auth/profile/picture` | JWT | Upload profile picture |

#### Register

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com","password":"password123"}'
```

#### Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"john@example.com","password":"password123"}'
```

Response:
```json
{
  "success": true,
  "message": "login successful",
  "data": {
    "token": "eyJhbGci...",
    "user": { "id": 1, "name": "John Doe", "email": "john@example.com", ... }
  }
}
```

#### Authenticated requests

Pass the token in the `Authorization` header:

```bash
curl http://localhost:8080/api/v1/auth/profile \
  -H "Authorization: Bearer eyJhbGci..."
```

#### Upload profile picture

```bash
curl -X POST http://localhost:8080/api/v1/auth/profile/picture \
  -H "Authorization: Bearer eyJhbGci..." \
  -F "file=@/path/to/photo.jpg"
```

Accepted formats: `jpg`, `png`, `webp` — max **5 MB**.

---

## Authentication Model

The API uses two independent auth layers:

| Layer | Mechanism | Applies to |
|---|---|---|
| **Basic Auth** | `Authorization: Basic <base64>` | Future admin / service-to-service routes under `/api/v1` |
| **JWT Bearer** | `Authorization: Bearer <token>` | User-facing routes (profile, etc.) |

Public routes (register, login, health, swagger) require no authentication.

---

## Object Storage

Storage is provider-agnostic via the `Storage` interface in `pkg/storage/storage.go`. Only two methods need to be implemented to add a new provider: `Upload` and `Delete`.

The included S3 implementation works with any S3-compatible service. Change only the environment variables to migrate:

| Provider | `STORAGE_ENDPOINT` | `STORAGE_FORCE_PATH_STYLE` |
|---|---|---|
| **idcloudhost** | `https://is3.cloudhost.id` | `false` |
| **AWS S3** | *(leave empty)* | `false` |
| **DigitalOcean Spaces** | `https://sgp1.digitaloceanspaces.com` | `false` |
| **Cloudflare R2** | `https://<account>.r2.cloudflarestorage.com` | `false` |
| **Minio** | `http://localhost:9000` | `true` |

Storage is **optional** — the server starts normally when `STORAGE_BUCKET` is not set. Upload endpoints return `503` with a clear error when storage is unconfigured.

---

## API Response Format

All endpoints return a consistent JSON envelope:

```json
{
  "success": true,
  "message": "...",
  "data": { ... }
}
```

Validation errors return per-field messages:

```json
{
  "success": false,
  "message": "validation failed",
  "errors": {
    "email": "must be a valid email address",
    "password": "must be at least 8 characters"
  }
}
```

---

## Development

```bash
# Run dev server
make run

# Regenerate Swagger docs after changing annotations
make swagger

# Build binary → bin/pawonwarga
make build

# Run tests
make test

# Tidy dependencies
make tidy
```

---

## Adding a New Feature

Follow the layered pattern used by the auth feature:

1. **Model** — add `internal/model/<entity>.go` embedding `BaseModel`
2. **Repository** — add `internal/repository/<entity>.go` with an interface + GORM implementation
3. **Service** — add `internal/service/<entity>.go` with business logic
4. **Handler** — add `internal/handler/<entity>.go` with Gin handlers and Swagger annotations
5. **Routes** — register in `internal/router/router.go` inside the appropriate group
6. **Migration** — add the model to `db.AutoMigrate(...)` in `internal/server/server.go`
7. **Docs** — run `make swagger` to regenerate

---

## Database

The app connects to TimescaleDB (PostgreSQL-compatible). GORM handles schema migrations automatically on startup via `AutoMigrate`.

All models embed `BaseModel` which provides `id`, `created_at`, `updated_at` (exposed in JSON) and `deleted_at` (soft delete, hidden from responses).
