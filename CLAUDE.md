# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Start all services (Kafka, PostgreSQL, API, Projector, Notifier, Frontend)
make up

# Stop all services
make down

# Start only infrastructure (for local Go development)
make infra

# Run API server locally (requires infra running)
go run cmd/api/main.go

# Run Projector locally
go run cmd/projector/main.go

# View logs
make logs
docker-compose logs api        # API logs only
docker-compose logs projector  # Projector logs only

# Clean up (removes volumes/data)
make clean
```

## Architecture Overview

This is an **Event-Driven EC Shop** implementing **CQRS** (Command Query Responsibility Segregation) and **Event Sourcing**.

### Core Pattern: Write/Read Separation

```
Write Path:
  HTTP Request → Command Handler → Domain Service → Event Store (PostgreSQL) → Kafka

Read Path:
  Kafka → Projector → Read Tables (PostgreSQL) → Query Handler → HTTP Response
```

### Three Independent Processes

1. **API Server** (`cmd/api/main.go`) - HTTP server handling commands and queries
2. **Projector** (`cmd/projector/main.go`) - Kafka consumer updating read models
3. **Notifier** (`cmd/notifier/main.go`) - Kafka consumer sending order confirmation emails

### Key Directories

- `internal/domain/` - Aggregates (Product, Cart, Order, Inventory, User, Category) with event definitions
- `internal/command/` - Command handlers (write operations)
- `internal/query/` - Query handlers (read operations)
- `internal/projection/` - Event → Read model transformations
- `internal/infrastructure/store/` - Event store and read store implementations

### Database Tables

| Table | Purpose |
|-------|---------|
| `events` | Append-only event store (write side) |
| `read_products` | Product queries with full-text search |
| `read_carts` | Cart data (JSONB items) |
| `read_orders` | Order history (JSONB items) |
| `read_inventory` | Stock tracking |
| `read_users` | User accounts |
| `read_categories` | Product categories |

### Event Flow

When a command is executed:
1. Domain service creates events and appends to `events` table
2. Events are published to Kafka topic `ec-events`
3. Projector consumes events and updates `read_*` tables
4. Queries read from `read_*` tables (eventual consistency)

### Authentication

- JWT-based with access tokens (15min) and refresh tokens (7 days)
- Middleware in `internal/api/middleware.go`
- JWT service in `internal/auth/jwt.go`

## Development URLs

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |
| Kafka UI | http://localhost:8081 |
| Mailpit (email) | http://localhost:8025 |

## Admin Access

- Email: `admin@example.com`
- Password: `admin123`
