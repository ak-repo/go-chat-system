# Feature Inventory

`go-chat-system` is a backend-first real-time chat service written in Go. This document lists the project features that are currently represented in the codebase and database migrations.

## Runtime Features

### HTTP API

- Versioned API namespace under `/api/v1`.
- Public authentication routes for registration, login, and token refresh.
- Protected routes use JWT authentication and receive the authenticated user ID through request context.
- Consistent JSON response envelope through the HTTP response wrapper.
- CORS, panic recovery, request logging, request ID, and Redis-backed rate limiting middleware.

### Authentication

- User registration with username, email, and password.
- Email format validation.
- Password minimum-length validation.
- Password hashing before persistence.
- Login with email and password.
- JWT access token generation with user ID, email, role, expiry, and issuer claims.
- JWT refresh token generation and refresh endpoint.
- Protected route authentication from `Authorization: Bearer`, `token` query parameter, or `access` cookie.

### Users

- User records contain ID, username, email, password hash, role, timestamps, and soft-delete timestamp.
- User-facing responses use a DTO that omits password hashes.
- User search endpoint supports a `filter` query parameter.
- Search supports a bounded `limit` query parameter with default and maximum limits.

### Friends

- Friend relationships are stored in a dedicated `friends` table.
- Friend listing endpoint returns the authenticated user's friends.
- Friend listing supports `limit` and `offset` pagination parameters.
- Database constraints prevent users from being friends with themselves.

### Friend Requests

- Friend request records contain sender, receiver, status, timestamps, and soft-delete timestamp.
- Supported request lifecycle endpoints:
  - create a friend request
  - accept a friend request
  - reject a friend request
  - cancel a sent request
  - list requests for the authenticated user
- Friend request creation rejects self-requests.
- Friend request creation rejects users who are already friends.
- Friend request creation rejects duplicate pending requests in either direction.
- Friend request creation checks block relationships before creating a request.
- Database constraints prevent self-requests and duplicate pending requests from the same sender to receiver.

### Blocking

- Blocking relationships are stored in a dedicated `blocks` table.
- Authenticated users can block another user.
- Authenticated users can unblock another user.
- Self-blocking is rejected.
- Blocking is considered during friend request creation.
- Database constraints prevent self-blocking.

### Direct Messaging

- WebSocket endpoint is exposed at `/api/v1/ws` behind JWT authentication.
- The authenticated user ID is injected server-side into outbound message routing, so clients do not control `sender_id`.
- Direct user messages can be routed to all active connections for the receiver.
- Direct user WebSocket messages are persisted through the message service.
- Message history endpoint returns conversation messages between the authenticated user and another user.
- Message history supports `user_id`, `limit`, and `offset` query parameters.
- Message persistence stores sender, receiver, body, group flag, timestamps, and soft-delete timestamp.

### WebSocket Behavior

- WebSocket origin checks use configured CORS origins, with a local development fallback.
- Multiple active connections per user are supported.
- Per-client read and write pumps isolate socket reads from writes.
- Server sends periodic WebSocket ping frames.
- Read deadlines are extended on pong responses.
- Incoming message size is capped at 10KB.
- Incoming messages are rate limited per client.
- Backpressure on outbound client queues closes the overloaded connection.
- Presence events are broadcast when users connect or fully disconnect:
  - `user_online`
  - `user_offline`
- WebSocket hub shutdown closes active client send channels during application shutdown.

### Health Checks

- `/health/live` returns process liveness.
- `/health/ready` verifies Redis and PostgreSQL readiness.
- `/redis-health` checks Redis availability.
- `/db-health` checks PostgreSQL availability.

## Data And Persistence

### PostgreSQL

- PostgreSQL is the primary data store.
- Schema is managed through Goose migrations.
- Current tables:
  - `users`
  - `friends`
  - `blocks`
  - `friend_requests`
  - `messages`
- Migrations include indexes for common access patterns such as user email lookup, friend lookup, block lookup, pending friend request lookup, and message lookup by sender or receiver.

### Redis

- Redis is initialized as an application dependency.
- Redis is used by rate limiting middleware.
- Redis readiness is included in health checks.

## Architecture Features

- Single executable entrypoint: `cmd/server/main.go`.
- Application code is organized under `internal/`.
- Manual dependency injection wires repositories and services in `internal/transport/injector`.
- Domain models live under `internal/domain/model`.
- Data access lives under `internal/repository`.
- Business logic and HTTP handler methods live under `internal/service`.
- Transport concerns live under `internal/transport`.
- Shared infrastructure helpers live under `internal/shared`.
- Platform configuration and database setup live under `internal/platform`.

## Development Features

- Docker Compose starts local PostgreSQL and Redis.
- `config/config.yaml` is the runtime configuration source.
- `config/config.example.yaml` documents the expected local config shape.
- Make targets are available for pre-flight checks, build, run, migrations, and Docker lifecycle.
- `go test ./...` is the primary verification command.

## Current Limitations

- Group chat is scaffolded in the WebSocket message model, room type, and message schema, but there is no public API for room creation or group management.
- Group message routing is incomplete; room state is not persisted and room creation is not exposed through public routes.
- Direct message persistence currently runs asynchronously from WebSocket routing, so a message may be delivered even if persistence later fails.
- Friend request `rejected` status exists in the Go model, while the migration status check currently allows `pending`, `accepted`, and `blocked`.
- Admin actions are listed as TODO in the user service and are not implemented.
- The OTP helper exists but is not wired into an exposed authentication or verification flow.
