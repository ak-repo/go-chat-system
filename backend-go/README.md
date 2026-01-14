

# go-chat-system

> **“A real chat system is not about sending messages; it is about guaranteeing delivery under failure.”**

`go-chat-system` is a **backend-first, real-time chat system** built in **Go**, designed with **clean architecture principles**, **explicit database migrations**, and **production-oriented configuration management**.

The project emphasizes:

* clear separation of concerns
* deterministic schema evolution
* infrastructure-aware development
* real-time communication via WebSockets

---

## Project Structure

```text
go-chat-system/
├── cmd/
│   └── server/
│       └── main.go              # Application entrypoint (config, DB, Redis, HTTP & WS bootstrap)
│
├── config/
│   ├── config.go               # Configuration loader (YAML → Go structs)
│   └── config.yaml             # Single source of truth for app, DB, Redis, JWT
│
├── database/
│   ├── postgres.go             # PostgreSQL connection & pool setup
│   └── redis.go                # Redis client initialization
│
├── migrations/
│   └── 001_users.up.sql        # Goose-managed database migrations
│
├── model/
│   ├── common.go               # Shared domain fields (IDs, timestamps)
│   ├── user.go                 # User domain model
│   └── message.go              # Message domain model
│
├── transport/
│   ├── handler/
│   │   ├── user_auth.go        # Auth HTTP handlers
│   │   └── ws_handler.go       # WebSocket entry handlers
│   │
│   ├── middleware/
│   │   ├── authmiddleware.go   # JWT authentication
│   │   ├── cors.go             # CORS handling
│   │   ├── logger.go           # Request logging
│   │   ├── ratelimit.go        # Rate limiting
│   │   └── recovery.go         # Panic recovery
│   │
│   ├── routes/
│   │   └── routes.go           # HTTP & WS route registration
│   │
│   └── websocket/
│       ├── hub.go              # Connection registry & broadcast logic
│       ├── client.go           # Per-client read/write pumps
│       ├── room.go             # Chat room abstraction
│       └── ws_message.go       # WebSocket message models
│
├── pkg/
│   ├── helper/
│   │   └── helper.go
│   ├── jwt/
│   │   └── jwt.go              # JWT creation & validation
│   ├── logger/
│   │   └── logger.go           # Structured logging abstraction
│   └── utils/
│       ├── db_utils.go
│       ├── password.go
│       ├── response.go
│       └── validation.go
│
├── docker-compose.yml           # Local infra (Postgres + Redis)
├── Makefile                     # Task runner (build, run, migrate, docker)
├── go.mod
├── go.sum
└── README.md
```

---

## Architecture Flow

```text
Client
  ↓
HTTP / WebSocket Transport
  ↓
Handlers & Middleware
  ↓
Domain Models & Business Rules
  ↓
Database / Redis
```

**Flow summary:**

**Outside World → Transport → Domain Logic → Persistence**

This ensures:

* transport-agnostic core logic
* testable business rules
* replaceable infrastructure components

---

## Configuration Strategy

* All configuration lives in **`config/config.yaml`**
* Parsed once at startup
* Reused by:

  * Go application
  * Database migrations
  * Docker & Makefile tooling

This avoids:

* duplicated `.env` files
* hardcoded credentials
* configuration drift

---

## Database Migrations

* Managed using **Goose**
* Explicit, versioned SQL migrations
* Applied via Makefile commands

This guarantees:

* schema consistency across environments
* safe rollbacks
* auditability of schema changes

---

## Local Development Workflow

```bash
# Start Postgres & Redis
make docker-up

# Apply database migrations
make migrate-up

# Run the server
make run
```

---

## Repository

[https://github.com/ak-repo/go-chat-system.git](https://github.com/ak-repo/go-chat-system.git)

---

