# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run Commands

```bash
# Start all services (LocalStack, PostgreSQL, API, Frontend)
make up

# Stop all services
make down

# Start only infrastructure (for local Go development)
make infra

# Build and deploy Lambda functions to LocalStack
make deploy-local

# Run API server locally (requires infra running)
make api

# Generate protobuf code
make proto

# View logs
make logs
make logs-projector  # Lambda Projector logs
make logs-notifier   # Lambda Notifier logs
make logs-loki       # Loki + Promtail logs

# Clean up (removes volumes/data)
make clean
```

## Architecture Overview

This is an **Event-Driven EC Shop** implementing **CQRS** (Command Query Responsibility Segregation) and **Event Sourcing** with **Proto.Actor** and **AWS Kinesis Data Streams**.

### Core Pattern: Actor-Based Write / Async Read

```
Write Path:
  HTTP Request → ActorHandler → Aggregate Actor (PersistReceive) → DynamoDB (Event Store)
                                                                          ↓
                                                                    (Auto CDC)
                                                                          ↓
                                                                  Kinesis Data Streams

Read Path:
  Kinesis → Lambda Projector → PostgreSQL (Read Store) → Query Handler → HTTP Response
```

### Components

1. **API Server** (`cmd/api/main.go`) - HTTP server handling commands and queries
2. **Actor System** (`internal/actor/`) - Proto.Actor based aggregate actors with event sourcing
3. **Lambda Projector** (`cmd/lambda/projector/main.go`) - Kinesis consumer updating read models
4. **Lambda Notifier** (`cmd/lambda/notifier/main.go`) - Kinesis consumer sending order confirmation emails

### Key Directories

- `proto/domain/` - Protobuf definitions for commands, events, snapshots
- `internal/actor/` - Proto.Actor system, persistence provider, aggregate actors
  - `internal/actor/system.go` - ActorSystem lifecycle and actor registry
  - `internal/actor/persistence/` - DynamoDB-backed Proto.Actor PersistenceProvider (protobuf binary)
  - `internal/actor/aggregate/` - Event-sourced actors (Product, Cart, Order, Inventory, User, Category)
  - `internal/actor/saga/` - Saga actors (PlaceOrder)
- `internal/command/` - ActorHandler (sends commands to actors), CommandHandler interface
- `internal/query/` - Query handlers (read operations from PostgreSQL)
- `internal/projection/` - Event → Read model transformations (protobuf binary deserialization)
- `internal/infrastructure/store/` - Event store and read store implementations
- `internal/infrastructure/kinesis/` - Kinesis record adapter
- `internal/domain/` - Legacy aggregates (retained for User/Category auth flows)
- `cmd/lambda/` - Lambda function entry points
- `infra/terraform/` - Infrastructure as Code

### Database Tables

| Table | Purpose |
|-------|---------|
| DynamoDB `events` | Append-only event store (write side, protobuf binary data) |
| DynamoDB `snapshots` | Aggregate snapshots (protobuf binary) |
| PostgreSQL `read_products` | Product queries with full-text search |
| PostgreSQL `read_carts` | Cart data (JSONB items) |
| PostgreSQL `read_orders` | Order history (JSONB items) |
| PostgreSQL `read_inventory` | Stock tracking |
| PostgreSQL `read_users` | User accounts |
| PostgreSQL `read_categories` | Product categories |

### Event Flow

When a command is executed:
1. ActorHandler sends a protobuf command to the aggregate actor
2. Actor validates, calls `PersistReceive(event)` to persist protobuf binary to DynamoDB
3. Actor applies state change and responds to caller
4. DynamoDB automatically streams changes to Kinesis Data Streams (CDC)
5. Lambda Projector deserializes protobuf binary events and updates PostgreSQL `read_*` tables
6. Lambda Notifier deserializes protobuf binary events and sends email notifications
7. Queries read from PostgreSQL `read_*` tables (eventual consistency)

### Proto.Actor Patterns

- **PersistReceive**: Persists event only (does NOT replay through Receive). State must be applied manually after call.
- **Sender capture**: `ctx.Sender()` must be saved before `PersistReceive` — use `ctx.Send(sender, response)` instead of `ctx.Respond()`.
- **Passivation**: Actors auto-stop after 5 minutes of inactivity via `SetReceiveTimeout`.
- **Event type registry**: Bidirectional mapping between `event_type` strings and protobuf message types in `internal/actor/persistence/`.
- **Serialization**: Protobuf binary stored as base64-encoded JSON in DynamoDB (via `json.Marshal([]byte)`).

### Authentication

- JWT-based with access tokens (15min) and refresh tokens (7 days)
- Middleware in `internal/api/middleware.go`
- JWT service in `internal/auth/jwt.go`
- User registration/login still uses legacy `user.Service` directly

## Development URLs

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |
| LocalStack | http://localhost:4566 |
| Mailpit (email) | http://localhost:8025 |
| Jaeger (traces) | http://localhost:16686 |
| Prometheus (metrics) | http://localhost:9090 |
| Grafana (dashboards) | http://localhost:3001 (admin/admin) |
| Loki (logs) | http://localhost:3100 |

## Admin Access

- Email: `admin@example.com`
- Password: `admin123`
