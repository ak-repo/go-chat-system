# go-chat-system

> A backend-first real-time chat system in Go, built around HTTP APIs, authenticated WebSockets, PostgreSQL persistence, Redis-backed middleware, and explicit database migrations.

`go-chat-system` is an application backend for user authentication, friendship workflows, blocking, direct messaging, and real-time message delivery. The current implementation uses a single server entrypoint with application code organized under `internal/`.

## Features

- User registration, login, and JWT refresh tokens.
- JWT-protected HTTP and WebSocket routes.
- User search with bounded query limits.
- Friend listing with pagination.
- Friend request creation, acceptance, rejection, cancellation, and listing.
- User blocking and unblocking.
- Direct WebSocket messaging with server-injected sender identity.
- Message persistence and conversation history retrieval.
- WebSocket presence events for online and offline transitions.
- WebSocket read/write pumps, ping/pong deadlines, message size limits, and per-client rate limiting.
- Redis-backed HTTP rate limiting.
- PostgreSQL schema migrations managed by Goose.
- Health checks for process liveness, PostgreSQL, and Redis.

For the full implementation-based feature inventory, see [`docs/Features.md`](docs/Features.md).

## Project Structure

```text
go-chat-system/
├── cmd/
│   └── server/
│       └── main.go                    # Application entrypoint and graceful shutdown
│
├── config/
│   ├── config.example.yaml            # Example local configuration
│   └── config.yaml                    # Runtime configuration, ignored per environment when needed
│
├── docs/
│   ├── Features.md                    # Current feature inventory
│   ├── DEPLOYMENT.md                  # Deployment notes
│   └── ENGINEERING_REPORT_MVP.md      # Engineering report
│
├── internal/
│   ├── domain/
│   │   └── model/                     # User, friend, request, and message models
│   │
│   ├── platform/
│   │   ├── config/                    # YAML configuration loader
│   │   └── database/                  # PostgreSQL and Redis setup
│   │
│   ├── repository/                    # PostgreSQL repositories
│   │
│   ├── service/                       # Auth, user, friend, block, and message services
│   │
│   ├── shared/
│   │   ├── errs/                      # Shared error values
│   │   ├── helper/                    # Shared helpers
│   │   ├── jwt/                       # JWT creation and validation
│   │   ├── logger/                    # Logger initialization
│   │   └── utils/                     # Password, validation, response, and DB helpers
│   │
│   └── transport/
│       ├── injector/                  # Manual dependency wiring
│       ├── middleware/                # Auth, CORS, recovery, rate limit, request middleware
│       ├── routes/                    # HTTP and WebSocket route registration
│       ├── websocket/                 # Hub, client pumps, rooms, and WS message model
│       └── wrapper/                   # HTTP response wrapper and WebSocket handler
│
├── migrations/                        # Goose SQL migrations
├── docker-compose.yml                 # Local PostgreSQL and Redis
├── Makefile                           # Build, run, migration, and Docker targets
├── go.mod
└── README.md
```

## Architecture Flow

```text
Client
  ↓
HTTP / WebSocket transport
  ↓
Middleware and route wrappers
  ↓
Services
  ↓
Repositories
  ↓
PostgreSQL / Redis
```

The application keeps transport, business logic, persistence, and platform setup in separate packages. Dependencies are wired manually in `internal/transport/injector`.

## API Overview

Base API path: `/api/v1`

Public routes:

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/auth/register` | Create a user and issue an access token. |
| `POST` | `/auth/login` | Authenticate with email and password; returns access and refresh tokens. |
| `POST` | `/auth/refresh` | Rotate refresh token and issue a new access token. |

Protected routes:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/users` | Search users with `filter` and optional `limit`. |
| `GET` | `/friends` | List friends with optional `limit` and `offset`. |
| `GET` | `/friend-requests/` | List friend requests. |
| `POST` | `/friend-requests/` | Create a friend request. |
| `POST` | `/friend-requests/accept` | Accept a friend request. |
| `POST` | `/friend-requests/reject` | Reject a friend request. |
| `POST` | `/friend-requests/cancel` | Cancel a sent friend request. |
| `POST` | `/blocks/` | Block a user. |
| `POST` | `/blocks/unblock` | Unblock a user. |
| `GET` | `/messages` | Get direct conversation history with `user_id`, `limit`, and `offset`. |
| `GET` | `/ws` | Open an authenticated WebSocket connection. |

Health routes:

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health/live` | Process liveness check. |
| `GET` | `/health/ready` | PostgreSQL and Redis readiness check. |
| `GET` | `/redis-health` | Redis ping check. |
| `GET` | `/db-health` | PostgreSQL ping check. |

Protected routes accept JWTs from an `Authorization: Bearer <token>` header. The auth middleware also supports `token` query parameter fallback for WebSocket clients and an `access` cookie fallback.

## Configuration

Configuration is loaded from `config/config.yaml` at startup. Use `config/config.example.yaml` as the reference shape for local configuration.

The configuration includes:

- PostgreSQL host, port, credentials, database name, SSL mode, and pool settings
- JWT secret, issuer, access token expiry, and refresh token expiry when configured
- HTTP server host and port
- CORS settings
- logging settings
- Redis host, port, password, and database index

## Database Migrations

Migrations are stored in `migrations/` and are managed with Goose through Make targets.

Current schema areas:

- `users`
- `friends`
- `blocks`
- `friend_requests`
- `messages`

## Local Development

Prerequisites:

- Go
- Docker and Docker Compose
- `yq`
- `goose`

Start local infrastructure:

```bash
make docker-up
```

Apply migrations:

```bash
make migrate-up
```

Run the server:

```bash
make run
```

Run tests:

```bash
go test ./...
```

Build the server binary:

```bash
make build
```

## Notes

- `cmd/server/main.go` is the only executable entrypoint.
- New application code should stay under `internal/`.
- `make check` is a prerequisite for Make targets that need config, Go, `yq`, or `goose`.
- Group chat types are scaffolded, but public group management routes and persisted room state are not currently implemented.

## Repository

[https://github.com/ak-repo/go-chat-system.git](https://github.com/ak-repo/go-chat-system.git)
