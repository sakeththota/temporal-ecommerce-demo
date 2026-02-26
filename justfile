# =============================================================================
# Durable Embedding Migration - Development Commands
# =============================================================================

@_:
  just --list

# -----------------------------------------------------------------------------
# Main Commands
# -----------------------------------------------------------------------------

# Full reset and start
reset:
    just down up migrate seed

# Stop and start with rebuild
up:
    docker compose up --build -d

# Start without rebuilding
dev:
    docker compose up

# Stop containers
down:
    docker compose down

# Stop and remove volumes
clean:
    docker compose down -v

# -----------------------------------------------------------------------------
# Database Commands
# -----------------------------------------------------------------------------

# Wait for database to be ready
wait-db:
    @echo "Waiting for database..."
    @docker compose exec -T postgresql pg_isready -U postgres || (sleep 30 && docker compose exec -T postgresql pg_isready -U postgres) || (sleep 30 && docker compose exec -T postgresql pg_isready -U postgres)
    @echo "Database is ready!"

# Run database migrations
migrate: wait-db
    @echo "Running migrations..."
    @go run -tags goose github.com/pressly/goose/cmd/goose@latest -dir backend/migrations postgres "postgres://postgres:postgres@localhost:5432/embeddings?sslmode=disable" up

# Seed database with hotels and embeddings
seed:
    @echo "Seeding database..."
    @DATABASE_URL="postgres://postgres:postgres@localhost:5432/embeddings?sslmode=disable" go run backend/cmd/seed/main.go
    @echo "Resetting embeddings..."
    @curl -s -X POST http://localhost:8080/api/migrations/reset

# Reset migrations via API
reset-migrations:
    curl -X POST http://localhost:8080/api/migrations/reset

# -----------------------------------------------------------------------------
# Logs Commands
# -----------------------------------------------------------------------------

# View logs (specify SERVICE=api, frontend, etc.)
logs SERVICE="":
    docker compose logs -f {{SERVICE}}

# -----------------------------------------------------------------------------
# Dev Commands
# -----------------------------------------------------------------------------

# Build all components locally
build:
    go build ./...
    cd frontend && npm run build

# Run tests
test:
    go test ./...

# Run linters
lint:
    golangci-lint run
    cd frontend && npm run lint
