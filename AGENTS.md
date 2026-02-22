# AGENTS.md - Durable Embedding Migration

## Project Overview

A semantic search system demonstrating zero-downtime embedding model migration using Temporal's durable execution. Users search hotel listings by natural language, with support for upgrading the embedding model without search downtime, full crash recovery, and progress visibility.

**Tech Stack:**
- Backend: Go 1.24, net/http, Temporal Go SDK v1.40, PostgreSQL 15, pgx v5
- Frontend: Next.js 15 (App Router), TypeScript, Tailwind CSS v4
- Infrastructure: Docker Compose

---

## Build / Lint / Test Commands

### Go Backend

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run single test
go test -run TestName ./...

# Run tests with verbose output
go test -v ./...

# Lint code
golangci-lint run

# Run migrations
go run -tags goose github.com/pressly/goose/cmd/goose@latest -dir backend/migrations up

# Start API server + Temporal worker
go run backend/cmd/api
```

### Frontend

```bash
# Install dependencies
cd frontend && npm install

# Build for production
npm run build

# Run development server
npm run dev

# Run linter
npm run lint
```

### Docker / Full Stack

```bash
# Build and start all services
docker compose up --build

# Start services (no rebuild)
docker compose up

# Stop services
docker compose down

# View logs
docker compose logs -f

# Restart a specific service
docker compose restart api
```

### Justfile Recipes

```just
# Build all components
build:
    go build ./...
    cd frontend && npm run build

# Start development environment
up:
    docker compose up --build

# Stop development environment
down:
    docker compose down

# Run all tests
test:
    go test ./...

# Run Go tests
test-go:
    go test -v ./...

# Run single Go test (pass TEST_NAME=TestFunction)
test-go-single TEST_NAME="":
    go test -v -run {{TEST_NAME}} ./...

# Run all linters
lint:
    golangci-lint run
    cd frontend && npm run lint

# Run database migrations
migrate:
    go run -tags goose github.com/pressly/goose/cmd/goose@latest -dir backend/migrations up

# Seed database with hotel data
seed:
    # TODO: Implement seed command

# Development mode (live reload)
dev:
    docker compose up

# View logs for a service
logs SERVICE="":
    docker compose logs -f {{SERVICE}}
```

---

## Code Style Guidelines

### Go (Backend)

**General Principles:**
- Keep things simple and clean - senior engineer level
- Use standard library where possible (net/http for HTTP, context for cancellation)
- Avoid premature abstraction - add complexity only when needed

**Project Structure:**
```
backend/
├── cmd/
│   ├── api/main.go              # HTTP server + Temporal worker
│   └── seed-embeddings/main.go  # One-off v1 embedding seeder
├── internal/
│   ├── api/                 # HTTP handlers + CORS
│   ├── db/                  # Database access (hotels, embeddings, versions)
│   ├── embeddings/          # Ollama client
│   ├── search/              # Cosine similarity
│   ├── workflows/           # Temporal workflows
│   ├── activities/          # Temporal activities
│   └── config/              # Configuration
├── migrations/              # goose SQL migrations
└── go.mod
```

**Naming Conventions:**
- Variables and functions: `camelCase` (e.g., `fetchBatch`, `hotelID`)
- Types and interfaces: `PascalCase` (e.g., `HotelService`, `MigrationInput`)
- Constants: `PascalCase` or `camelCase` for grouped constants (e.g., `StatusActive`)
- Packages: `lowercase`, short, descriptive (e.g., `db`, `api`, `workflows`)

**Imports:**
```go
import (
    "context"
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "go.temporal.io/sdk/client"
)
```

Order:stdlib → external → local. Use `go fmt` and `goimports` to enforce.

**Error Handling:**
- Return errors explicitly - avoid swallowing them
- Use `fmt.Errorf` with %w for wrapped errors
- In activities: return errors to let Temporal handle retries
- In handlers: log errors and return appropriate HTTP status codes

**Database:**
- Use pgx v5 with pgxpool for connection management
- Use parameterized queries to prevent SQL injection
- Use transactions for multi-statement operations
- Keep queries in the `db` package, not in handlers

**Temporal:**
- Activities should be small, idempotent, and retryable
- Use context.Context properly - pass through to downstream calls
- Define retry policies in workflow options, not inside activities
- Use signals for pause/resume, queries for progress reporting

### TypeScript / Next.js (Frontend)

**General Principles:**
- Keep components small and focused
- Use TypeScript strictly - avoid `any`
- Prefer composition over abstraction
- Keep it simple - senior engineer level

**Project Structure:**
```
frontend/
├── src/
│   ├── app/                 # Next.js App Router pages
│   │   ├── page.tsx         # Search page
│   │   ├── layout.tsx       # Root layout
│   │   ├── globals.css      # Tailwind CSS v4 imports
│   │   └── migrations/
│   │       └── page.tsx     # Migration dashboard
│   └── lib/                 # Utilities
│       ├── api.ts           # API client (fetch wrapper)
│       └── types.ts         # TypeScript interfaces
├── package.json
├── tsconfig.json
├── next.config.ts
├── postcss.config.mjs
└── Dockerfile
```

**Naming Conventions:**
- Components: `PascalCase` (e.g., `SearchBar.tsx`)
- Hooks: `camelCase` starting with `use` (e.g., `useSearch`)
- Utilities: `camelCase` (e.g., `api.ts`)
- Types/Interfaces: `PascalCase` (e.g., `Hotel`, `MigrationStatus`)

**TypeScript:**
- Enable strict mode in tsconfig.json
- Avoid `any` - use `unknown` or proper types
- Define interfaces for all API responses
- Use generic types for reusable components

**React / Next.js:**
- Use Server Components where possible
- Keep client components at the leaf nodes
- Use proper error boundaries
- Handle loading states gracefully

**Tailwind:**
- Use utility classes consistently
- Prefer semantic class names (e.g., `bg-primary` over arbitrary colors)
- Keep responsive design in mind

---

## Database Schema

### hotels
| Column          | Type          |
| --------------- | ------------- |
| id              | UUID PK       |
| name            | TEXT NOT NULL |
| description     | TEXT NOT NULL |
| location        | TEXT NOT NULL |
| price_per_night | DECIMAL(10,2) |
| amenities       | TEXT[]        |
| created_at      | TIMESTAMPTZ   |

### hotel_embeddings
| Column     | Type               |
| ---------- | ------------------ |
| id         | UUID PK            |
| hotel_id   | UUID FK            |
| version    | TEXT NOT NULL      |
| model_name | TEXT NOT NULL      |
| dimensions | INT NOT NULL       |
| embedding  | DOUBLE PRECISION[] |
| created_at | TIMESTAMPTZ        |

### embedding_versions
| Column            | Type        |
| ----------------- | ----------- |
| version           | TEXT PK     |
| model_name        | TEXT        |
| dimensions        | INT         |
| status            | TEXT        |
| total_records     | INT         |
| processed_records | INT         |
| is_active         | BOOLEAN     |
| created_at        | TIMESTAMPTZ |
| completed_at      | TIMESTAMPTZ |

---

## API Endpoints

| Method | Path                              | Description                     |
| ------ | --------------------------------- | -------------------------------  |
| GET    | /api/health                       | Health check                    |
| GET    | /api/search                       | Semantic search                 |
| GET    | /api/hotels                       | List hotels                     |
| GET    | /api/versions                     | List embedding versions        |
| POST   | /api/migrations                   | Start new migration            |
| GET    | /api/migrations/:version          | Get migration progress         |
| POST   | /api/migrations/:version/pause    | Pause migration                 |
| POST   | /api/migrations/:version/resume  | Resume migration                |

---

## Temporal Workflow

### EmbeddingMigrationWorkflow

**Input:**
```go
type MigrationInput struct {
    Version   string // "v2"
    ModelName string // "nomic-embed-text"
    BatchSize int    // 10
}
```

**Signals:**
- `pause` - Pause migration
- `resume` - Resume migration

**Query:**
- `progress` - Returns MigrationProgress struct

**Retry Policy:**
```go
RetryPolicy{
    InitialInterval:    1 * time.Second,
    BackoffCoefficient: 2.0,
    MaximumInterval:    30 * time.Second,
    MaximumAttempts:    5,
}
```

---

## Docker Services

| Service      | Port | Purpose              |
| ------------ | ---- | -------------------- |
| postgresql   | 5432 | Database             |
| temporal     | 7233 | Temporal server      |
| temporal-ui  | 8233 | Temporal Web UI      |
| ollama       | 11434 | Embedding server    |
| api          | 8080 | Go HTTP API         |
| frontend     | 3000 | Next.js app         |

---

## Best Practices Summary

1. **Simplicity first** - Don't add complexity until needed
2. **Senior-level code** - Clean, readable, well-structured
3. **Proper error handling** - Don't swallow errors, log meaningfully
4. **Type safety** - TypeScript strict mode, Go interfaces
5. **Idempotency** - Activities must be idempotent for Temporal retries
6. **Context usage** - Pass context.Context through all I/O operations
7. **Configuration** - Environment variables, no hardcoded values
8. **Testing** - Test critical paths, single tests for debugging
9. **Code review mindset** - Write code you'd be proud to have reviewed
