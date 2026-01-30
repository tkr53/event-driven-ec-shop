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

# View logs
make logs
make logs-projector  # Lambda Projector logs
make logs-notifier   # Lambda Notifier logs

# Clean up (removes volumes/data)
make clean
```

## Architecture Overview

This is an **Event-Driven EC Shop** implementing **CQRS** (Command Query Responsibility Segregation) and **Event Sourcing** with **AWS Kinesis Data Streams**.

### Core Pattern: Write/Read Separation

```
Write Path:
  HTTP Request → Command Handler → Domain Service → DynamoDB (Event Store)
                                                          ↓
                                                    (Auto CDC)
                                                          ↓
                                                  Kinesis Data Streams

Read Path:
  Kinesis → Lambda Projector → PostgreSQL (Read Store) → Query Handler → HTTP Response
```

### Components

1. **API Server** (`cmd/api/main.go`) - HTTP server handling commands and queries
2. **Lambda Projector** (`cmd/lambda/projector/main.go`) - Kinesis consumer updating read models
3. **Lambda Notifier** (`cmd/lambda/notifier/main.go`) - Kinesis consumer sending order confirmation emails

### Key Directories

- `internal/domain/` - Aggregates (Product, Cart, Order, Inventory, User, Category) with event definitions
- `internal/command/` - Command handlers (write operations)
- `internal/query/` - Query handlers (read operations)
- `internal/projection/` - Event → Read model transformations
- `internal/infrastructure/store/` - Event store and read store implementations
- `internal/infrastructure/kinesis/` - Kinesis record adapter
- `cmd/lambda/` - Lambda function entry points
- `infra/terraform/` - Infrastructure as Code

### Database Tables

| Table | Purpose |
|-------|---------|
| DynamoDB `events` | Append-only event store (write side) |
| DynamoDB `snapshots` | Aggregate snapshots |
| PostgreSQL `read_products` | Product queries with full-text search |
| PostgreSQL `read_carts` | Cart data (JSONB items) |
| PostgreSQL `read_orders` | Order history (JSONB items) |
| PostgreSQL `read_inventory` | Stock tracking |
| PostgreSQL `read_users` | User accounts |
| PostgreSQL `read_categories` | Product categories |

### Event Flow

When a command is executed:
1. Domain service creates events and appends to DynamoDB `events` table
2. DynamoDB automatically streams changes to Kinesis Data Streams (CDC)
3. Lambda Projector consumes events and updates PostgreSQL `read_*` tables
4. Lambda Notifier consumes events and sends email notifications
5. Queries read from PostgreSQL `read_*` tables (eventual consistency)

### Authentication

- JWT-based with access tokens (15min) and refresh tokens (7 days)
- Middleware in `internal/api/middleware.go`
- JWT service in `internal/auth/jwt.go`

## Development URLs

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |
| LocalStack | http://localhost:4566 |
| Mailpit (email) | http://localhost:8025 |

## Admin Access

- Email: `admin@example.com`
- Password: `admin123`
