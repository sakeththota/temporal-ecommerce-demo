# =============================================================================
# Temporal Ecommerce Demo - Development Commands
# =============================================================================

@_:
  just --list

# -----------------------------------------------------------------------------
# Main Commands
# -----------------------------------------------------------------------------

# Full reset and start (clears db volumes)
reset:
    just clean up

# Stop and start with rebuild
up:
    docker compose up --build -d --remove-orphans

# Start without rebuilding
dev:
    docker compose up --remove-orphans

# Stop containers
down:
    docker compose down

# Stop and remove volumes
clean:
    docker compose down -v

# -----------------------------------------------------------------------------
# Logs Commands
# -----------------------------------------------------------------------------

# View logs (specify SERVICE=search, booking, frontend, etc.)
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
