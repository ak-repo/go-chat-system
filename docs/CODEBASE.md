# go-chat-system Codebase Guide

This is the canonical project and codebase reference for humans and agents planning or implementing features. Source code, migrations, configuration, and tests are the source of truth; older documentation should not override what is implemented.

## Project Overview

`go-chat-system` is a backend-first real-time chat application. It provides authentication, friendship workflows, blocking, direct messaging, message history, and authenticated WebSocket delivery.

The backend owns the core business rules and persistence. The frontend is a React TypeScript SPA that talks to the backend through REST and WebSocket APIs.

The main backend dependency direction is:

```text
HTTP / WebSocket transport
  -> service layer
  -> repository layer
  -> PostgreSQL / Redis
```

Do not bypass this layering for feature work unless there is an explicit architectural reason.

## Technology Stack

- Backend: Go, Chi, pgx/pgxpool, PostgreSQL, Redis, Gorilla WebSocket, Viper, zap, JWT, bcrypt.
- Database migrations: Goose SQL migrations in `migrations/`.
- Frontend: React, TypeScript, Vite, React Router, Axios.
- Local runtime support: `docker-compose.yml` starts PostgreSQL and Redis only.

## Repository Structure

```text
cmd/server/main.go                    application startup and graceful shutdown
config/                               runtime config and example config
internal/domain/model/                domain/data transfer structs
internal/platform/config/             Viper config loading and env overrides
internal/platform/database/           PostgreSQL and Redis clients
internal/repository/                  SQL persistence code
internal/service/                     business logic and HTTP service methods
internal/shared/                      errors, JWT, logger, helpers, utilities
internal/transport/injector/          manual dependency wiring
internal/transport/middleware/        auth, CORS, recovery, rate limiting
internal/transport/routes/            Chi route registration
internal/transport/websocket/         hub, client pumps, message envelope, room scaffold
internal/transport/wrapper/           REST response wrapper and WS upgrade handler
migrations/                           Goose PostgreSQL migrations
web/src/api/                          frontend REST and WebSocket clients
web/src/context/                      auth and socket context providers
web/src/pages/                        login, register, friends, chat pages
docs/                                 codebase and deployment documentation
```

## Backend Architecture

Startup in `cmd/server/main.go`:

1. Initializes logging.
2. Loads YAML configuration and environment overrides.
3. Connects to PostgreSQL and Redis.
4. Builds the Chi router.
5. Starts the HTTP server on `config.Config.Server.Port`.
6. On shutdown, stops the WebSocket hub if it exists and shuts down HTTP gracefully.

Dependency wiring is manual in `internal/transport/injector/injector.go`:

- Repositories are created from the PostgreSQL pool.
- Services are created from repositories.
- Routes expose service methods through `wrapper.HTTPResponseWrapper`.

Routing and middleware live in `internal/transport/routes/routes.go`:

- Public auth routes use Redis IP rate limiting.
- Protected routes use JWT auth and Redis user rate limiting.
- Health routes are registered outside `/api/v1`.

Transport code should stay thin. Business rules belong in `internal/service/`. SQL belongs in `internal/repository/`.

## Frontend Architecture

The React app lives under `web/`.

- `web/src/App.tsx` defines `/login`, `/register`, `/friends`, and `/chat/:userId` routes.
- `web/src/context/AuthContext.tsx` owns auth state and token persistence through API helpers.
- `web/src/context/SocketContext.tsx` connects and disconnects the singleton WebSocket client based on auth state.
- `web/src/api/client.ts` creates the Axios REST client with base URL `http://localhost:8002/api/v1`, bearer token injection, and 401 refresh handling.
- `web/src/api/websocket.ts` creates a singleton WebSocket client from `BASE_URL`, adds the token query parameter, reconnects with exponential backoff, and exposes event handlers.
- Page code should call API/socket abstractions instead of embedding transport logic directly.

## Database Architecture

The current schema is defined by Goose migrations under `migrations/`.

Current tables:

- `users`: UUID primary key, username, unique email, password hash, role, timestamps, soft-delete column.
- `friends`: directed rows `(user_id, friend_id)`; mutual friendship is represented as two rows; self-friend check exists.
- `blocks`: directed block rows `(blocker_id, blocked_id)`; blocking removes friendships and marks related friend requests blocked.
- `friend_requests`: sender, receiver, status, timestamps. The migration allows `pending`, `accepted`, and `blocked`.
- `messages`: sender, receiver, body, `is_group`, timestamps, soft-delete column.

Indexes exist for user email lookup, friend lookup, block lookup, pending request lookup, and message sender/receiver timestamp queries.

## Authentication Flow

Implemented behavior:

- Passwords are hashed with bcrypt.
- Access tokens are HS256 JWTs containing user ID, email, role, expiry, issue time, and issuer.
- Refresh tokens are HS256 JWTs containing user ID and issuer.
- `AuthMiddleware` accepts tokens from `Authorization: Bearer ...`, `?token=...`, or an `access` cookie.
- WebSocket authentication uses the same middleware path; frontend currently passes the token as a query parameter.
- The authenticated user ID is stored in request context as `middleware.UserIDKey`.

Known limitations:

- Refresh tokens are stateless; there is no server-side storage, rotation tracking, revocation list, or logout invalidation.
- Register returns an access token but not a refresh token. Login and refresh return refresh tokens.

## REST API

All API routes are under `/api/v1`.

Public auth routes:

| Method | Path | Body | Response |
| --- | --- | --- | --- |
| `POST` | `/auth/register` | `{username,email,password}` | `201` with `user`, `token`, `exp` |
| `POST` | `/auth/login` | `{email,password}` | `200` with `user`, `token`, `exp`, `refresh_token`, `refresh_exp` |
| `POST` | `/auth/refresh` | `{refresh_token}` | `200` with new `token`, `exp`, `refresh_token`, `refresh_exp` |

Protected routes require JWT auth:

| Method | Path | Parameters / body | Implemented behavior |
| --- | --- | --- | --- |
| `GET` | `/users` | query `filter`, optional `limit` | Searches username/email with minimum filter length of 2. |
| `GET` | `/friends` | optional `limit`, `offset` | Lists authenticated user's friends. |
| `GET` | `/friend-requests/` | none | Lists incoming friend requests. |
| `POST` | `/friend-requests/` | `{to}` | Creates pending friend request after self/friend/duplicate/block checks. |
| `POST` | `/friend-requests/accept` | `{request_id, received_id}` | Accepts request and creates mutual friendship. |
| `POST` | `/friend-requests/cancel` | `{request_id}` | Cancels authenticated sender's pending request. |
| `POST` | `/friend-requests/reject` | `{request_id, receiver_id}` | Attempts to reject a request. |
| `POST` | `/blocks/` | `{target}` | Blocks target user, deletes friendship, marks requests blocked. |
| `POST` | `/blocks/unblock` | `{target}` | Deletes block relationship. |
| `GET` | `/messages` | `user_id`, optional `limit`, `offset` | Returns conversation between authenticated user and `user_id`, ordered newest first. |
| `GET` | `/ws` | auth by header/cookie/query | Upgrades to WebSocket. |

Response wrapper:

- Success with data: `{"status":"ok","data":...}`.
- Success without data: `{"message":"ok"}`.
- Error: `{"status":"error","message":"..."}`.

Health routes outside `/api/v1`:

- `GET /health/live`
- `GET /health/ready`
- `GET /redis-health`
- `GET /db-health`

## WebSocket Architecture

Implemented files:

- `internal/transport/wrapper/ws_handler.go`: authenticated upgrade handler and origin check.
- `internal/transport/websocket/client.go`: read/write pumps, ping, deadlines, message size limit, per-connection message rate limit.
- `internal/transport/websocket/hub.go`: in-memory user connection registry and routing.
- `internal/transport/websocket/ws_message.go`: message envelope.
- `web/src/api/websocket.ts`: frontend singleton WebSocket client.

Wire envelope:

```json
{
  "event": "message",
  "sender_id": "server-injected",
  "receiver_id": "target-user-id",
  "receiver_type": "user",
  "data": {}
}
```

Important behavior:

- Sender identity is overwritten from authenticated context in the WebSocket read path; client-supplied `sender_id` is not trusted.
- Multiple connections per user are supported in memory.
- Presence events `user_online` and `user_offline` are broadcast when first connection opens or last connection closes.
- Any event with `receiver_type: "user"` is routed to active connections for `receiver_id`.
- For user-targeted messages, the hub asynchronously extracts `data.text` or `data.content` and persists a message through `MessageService.CreateMessage`.

Partial/scaffolded behavior:

- Group routing types and room code exist, but no REST API, database schema, frontend group UI, or room initialization is implemented.
- Typing and read events are routed as transient events but are not persisted and have no backend-specific validation.
- There is no server acknowledgement for persisted messages and no error event on persistence failure.
- Realtime routing is process-local only; Redis is not used for WebSocket fan-out.

## Implemented Features

Implemented end-to-end or with a complete backend path:

- User registration and login.
- JWT-protected REST routes.
- Stateless access-token refresh endpoint.
- User search by username/email.
- Friend request creation with duplicate, self, friend, and block checks.
- Friend request acceptance creating mutual friendship.
- Friend request cancellation by sender.
- Friend listing.
- Backend blocking/unblocking, including friendship removal and related friend-request updates.
- Message persistence for direct user messages.
- REST conversation history retrieval.
- Authenticated WebSocket connection.
- Server-injected WebSocket sender identity.
- Direct user WebSocket delivery to currently connected receivers.
- Multi-connection tracking per user in the WebSocket hub.
- Online/offline presence broadcast.
- Basic frontend flows for login, registration, friends, requests, search, and direct chat.
- Redis-backed HTTP rate limiting and in-memory per-socket message rate limiting.
- Health checks for process, PostgreSQL, and Redis readiness.

## Partial / Scaffolded / Missing Features

Partial:

- Direct chat lacks persisted delivery acknowledgements, read receipt storage, message status, retry reconciliation, and strict ordering guarantees.
- Frontend optimistic messages use a client-generated ID; persisted backend messages use a server-generated ID; there is no ack to reconcile them.
- Friend request rejection is coded but conflicts with the database status constraint because the migration does not allow `rejected`.
- Frontend block/unblock API functions may not match backend body shape and are not part of the current primary UI flow.
- Frontend WebSocket types include `ack`, `typing`, and `read`; backend generically routes these events but only persists user-targeted events with text/content.

Scaffolded:

- Group chat routing structs/functions.
- Admin actions comment in user service.
- Soft-delete columns in database tables; application code does not consistently use soft delete.

Missing:

- Group chat product flow, group tables, membership management, and group UI.
- Message delete/edit APIs.
- Read receipt persistence.
- Delivery status persistence.
- Offline delivery notifications beyond REST history.
- Redis Pub/Sub or distributed WebSocket scaling.
- Server-side refresh-token revocation/logout.
- Automated frontend tests.

## Configuration

Config is loaded by `internal/platform/config/config.go` from `config.yaml` in `.` or `./config`.

Supported environment overrides:

- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- `REDIS_HOST`, `REDIS_PORT`
- `JWT_SECRET`
- `PORT`

Main config sections:

- `database`: host, port, user, password, name, sslmode, pool settings.
- `jwt`: secret, expiry, issuer, refresh_expiry.
- `server`: host, port.
- `CORS`: host, port, allowed_origins.
- `redis`: host, port, password, db.
- `logging`: level, format, output.

Important notes:

- Do not expose or commit local secrets from `config/config.yaml`.
- `config/config.example.yaml` is the reference shape for local configuration.
- PostgreSQL pool duration fields exist in config structs; verify current database setup before assuming every field is applied.

## Development Workflow

Local infrastructure:

```bash
make docker-up
```

Apply migrations:

```bash
make migrate-up
```

Run backend:

```bash
make run
```

Run frontend:

```bash
make web-run
```

Build backend:

```bash
make build
```

The Makefile expects `config/config.yaml`, `go`, `yq`, and `goose` for targets that depend on `make check`.

## Testing And Verification

Backend verification before completing backend changes:

```bash
go fmt ./...
go vet ./...
go test ./...
```

Frontend verification before completing frontend changes:

```bash
cd web
npm run lint
npm run build
```

Current Go tests include message service behavior, WebSocket message text extraction, and JWT refresh token generation/validation.

There is no frontend test suite in `web/package.json`; available frontend checks are lint and build.

## Deployment Architecture

See `docs/DEPLOYMENT.md` for deployment commands and production checklist.

Runtime shape:

- One Go binary serves REST and WebSocket traffic.
- PostgreSQL and Redis are external runtime dependencies.
- Goose migrations are run separately; the server does not run migrations automatically.
- The Vite frontend is built and served as static assets separately from the Go process unless a deployment adds a reverse proxy/static file layer.
- WebSocket connection state is in memory inside one Go process.

## Current Limitations

- No automatic migration runner in application startup.
- WebSocket persistence is asynchronous and failures are only logged; clients are not told persistence failed.
- WebSocket hub has no distributed fan-out; multiple backend instances would not share connected users.
- Direct-message REST history is ordered newest first; frontend behavior should account for that ordering.
- No authorization check verifies users are friends before messaging.
- No block check is performed before direct message persistence/routing.
- Error semantics are mostly human-readable messages, not stable machine codes.
- Some frontend/backend contract drift exists for token expiry, block API bodies, and friend request rejection status.

## Important Implementation Rules

- Keep handlers thin.
- Put business rules in `internal/service/`.
- Put SQL in `internal/repository/`.
- Never trust `sender_id` from WebSocket clients; sender identity must come from authenticated context.
- Create new Goose migrations for database changes; do not edit already-deployed migrations.
- Keep frontend API calls under `web/src/api/` and socket transport behavior out of UI components.
