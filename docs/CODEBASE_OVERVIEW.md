# Codebase Overview

This document describes the current repository state. Source code and migrations are treated as the source of truth.

## 1. Project overview

`go-chat-system` is a backend-first real-time chat application with a Go HTTP/WebSocket API, PostgreSQL persistence, Redis-backed rate limiting, and a React TypeScript frontend.

The backend follows this dependency direction:

```text
HTTP / WebSocket transport
  -> service layer
  -> repository layer
  -> PostgreSQL / Redis
```

## 2. Technology stack

- Backend: Go, Chi, pgx/pgxpool, PostgreSQL, Redis, Gorilla WebSocket, Viper, zap, JWT, bcrypt.
- Database migrations: Goose SQL migrations under `migrations/`.
- Frontend: React, TypeScript, Vite, React Router, Axios.
- Runtime support: `docker-compose.yml` starts PostgreSQL 16 and Redis 7 only.

## 3. Repository structure

```text
cmd/server/main.go                    application startup/shutdown
config/                               YAML config examples and local config
internal/domain/model/                domain/data transfer structs
internal/platform/config/             Viper config loading and env overrides
internal/platform/database/           PostgreSQL and Redis clients
internal/repository/                  SQL persistence code
internal/service/                     business logic and HTTP service methods
internal/shared/                      errors, JWT, logger, utils
internal/transport/injector/          manual dependency wiring
internal/transport/middleware/        auth, CORS, recovery, rate limit helpers
internal/transport/routes/            Chi route registration
internal/transport/websocket/         hub, client pumps, message envelope, room scaffold
internal/transport/wrapper/           REST response wrapper and WS upgrade handler
migrations/                           Goose PostgreSQL migrations
web/src/api/                          frontend REST and WebSocket clients
web/src/context/                      auth and socket context providers
web/src/pages/                        login, register, friends, chat pages
docs/                                 documentation
```

## 4. Backend architecture

Startup in `cmd/server/main.go`:

1. Initializes logging.
2. Loads YAML config with environment overrides.
3. Connects to PostgreSQL and Redis.
4. Builds the Chi router.
5. Starts an HTTP server on `config.Config.Server.Port`.
6. On SIGINT/SIGTERM, stops the WebSocket hub if it exists and shuts down HTTP gracefully.

Dependency wiring is manual in `internal/transport/injector/injector.go`:

- Repositories are created from the global PostgreSQL pool.
- Services are constructed from repositories.
- Routes wrap service methods with `wrapper.HTTPResponseWrapper`.

Middleware in `internal/transport/routes/routes.go` includes Chi logging/request ID plus project CORS and recovery. Protected `/api/v1` routes use JWT auth and Redis user rate limiting.

## 5. Frontend architecture

The React app is under `web/`.

- `web/src/App.tsx` defines routes:
  - `/login`
  - `/register`
  - `/friends`
  - `/chat/:userId`
- `AuthContext` stores user state from `localStorage`, calls auth API methods, and guards routes.
- `SocketContext` connects/disconnects the singleton WebSocket client based on auth state.
- `web/src/api/client.ts` creates an Axios client with hard-coded `http://localhost:8002/api/v1`, adds bearer tokens, and tries access-token refresh on 401.
- `FriendsPage` lists friends, incoming requests, and user search.
- `ChatPage` loads message history over REST and sends/receives realtime events over WebSocket.

## 6. Database structure

Implemented migrations:

- `users`: UUID primary key, username, unique email, password hash, role, timestamps, soft-delete column.
- `friends`: directed rows `(user_id, friend_id)` with mutual friendship represented as two rows; self-friend check.
- `blocks`: directed block rows `(blocker_id, blocked_id)`; blocking deletes friendships and marks friend requests blocked.
- `friend_requests`: sender, receiver, status, timestamps. Migration allows statuses `pending`, `accepted`, `blocked`.
- `messages`: sender, receiver, body, `is_group`, timestamps.

Indexes exist for user email, friend lookup, block lookup, pending request lookup, and message sender/receiver timestamp queries.

## 7. Authentication

Implemented:

- Passwords are hashed with bcrypt.
- Access tokens are HS256 JWTs containing user ID, email, role, expiry, issue time, and issuer.
- Refresh tokens are HS256 JWTs containing user ID and issuer.
- `AuthMiddleware` accepts tokens from:
  1. `Authorization: Bearer ...`
  2. `?token=...` query parameter, used by WebSocket
  3. `access` cookie
- Authenticated user ID is stored in request context as `middleware.UserIDKey`.

Partial / limitations:

- Refresh tokens are stateless; there is no server-side refresh-token storage, rotation tracking, revocation list, or logout invalidation.
- Frontend token-expiry handling expects numeric seconds, while backend responses encode `time.Time`; this is contract drift.
- Register returns an access token but does not return a refresh token. Login and refresh do return refresh tokens.

## 8. REST APIs

All API routes are under `/api/v1`.

### Public auth routes

| Method | Path | Body | Response |
| --- | --- | --- | --- |
| POST | `/auth/register` | `{username,email,password}` | `201` with `user`, `token`, `exp` |
| POST | `/auth/login` | `{email,password}` | `200` with `user`, `token`, `exp`, `refresh_token`, `refresh_exp` |
| POST | `/auth/refresh` | `{refresh_token}` | `200` with new `token`, `exp`, `refresh_token`, `refresh_exp` |

### Protected routes

Require JWT auth.

| Method | Path | Parameters / body | Implemented behavior |
| --- | --- | --- | --- |
| GET | `/users` | query `filter`, optional `limit` | searches username/email with minimum filter length of 2 |
| GET | `/friends` | optional `limit`, `offset` | lists authenticated user's friends |
| GET | `/friend-requests/` | none | lists incoming friend requests for authenticated user |
| POST | `/friend-requests/` | `{to}` | creates pending friend request after self/friend/duplicate/block checks |
| POST | `/friend-requests/accept` | `{request_id, received_id}` | accepts request and creates mutual friendship |
| POST | `/friend-requests/cancel` | `{request_id}` | cancels authenticated sender's pending request |
| POST | `/friend-requests/reject` | `{request_id, receiver_id}` | attempts to reject request |
| POST | `/blocks/` | `{target}` | blocks target user, deletes friendship, marks requests blocked |
| POST | `/blocks/unblock` | `{target}` | deletes block relationship |
| GET | `/messages` | `user_id`, optional `limit`, `offset` | returns conversation between authenticated user and `user_id` ordered newest first |
| GET | `/ws` | auth token by header/cookie/query | upgrades to WebSocket |

Response wrapper:

- Success with data: `{"status":"ok","data":...}`.
- Success without data: `{"message":"ok"}`.
- Error: `{"status":"error","message":"..."}`.

Health routes outside `/api/v1`:

- `GET /health/live`
- `GET /health/ready`
- `GET /redis-health`
- `GET /db-health`

Known REST/API drift:

- Frontend block/unblock API sends `{user_id}` but backend expects `{target}`.
- `friend_requests.status` migration does not allow `rejected`, but reject code writes `rejected`; reject can fail at the database constraint.

## 9. WebSocket architecture

Implemented files:

- `internal/transport/wrapper/ws_handler.go`: authenticated upgrade handler and origin check.
- `internal/transport/websocket/client.go`: read/write pumps, ping, deadlines, message size limit, per-connection message rate limit.
- `internal/transport/websocket/hub.go`: in-memory user connection registry and routing.
- `internal/transport/websocket/ws_message.go`: message envelope.

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

- Sender identity is overwritten from authenticated context in `Client.ReadPump`; client-supplied `sender_id` is not trusted.
- Multiple connections per user are supported in memory.
- Presence events `user_online` and `user_offline` are broadcast to all connected clients when first connection opens / last connection closes.
- Any event with `receiver_type: "user"` is routed to active connections for `receiver_id`.
- For user-targeted messages, the hub asynchronously extracts `data.text` or `data.content` and persists a message through `MessageService.CreateMessage`.

Partial / scaffolded:

- Group routing code exists (`Room`, `CreateRoom`, `sendToGroup`), but no REST API, database schema, frontend group UI, or room initialization is implemented. `NewHub` does not initialize the `rooms` map, so `CreateRoom` would panic if called as-is.
- Typing and read events are routed as transient WebSocket events but are not persisted and have no backend-specific validation.
- There is no server acknowledgement for persisted messages and no error event on persistence failure.
- Realtime routing is process-local only; Redis is not used for WebSocket fan-out.

## 10. Currently implemented features

Implemented end-to-end or with complete backend path:

- User registration and login.
- JWT-protected REST routes.
- Access-token refresh endpoint, stateless.
- User search by username/email.
- Friend request creation with duplicate/self/friend/block checks.
- Friend request acceptance creating mutual friendship.
- Friend request cancellation by sender.
- Friend listing.
- Blocking/unblocking users on backend, including removal of friendship and updating related friend requests.
- Message persistence for direct user messages.
- REST conversation history retrieval.
- Authenticated WebSocket connection.
- Server-injected WebSocket sender identity.
- Direct user WebSocket delivery to currently connected receivers.
- Multi-connection tracking per user in the hub.
- Presence broadcast for online/offline transitions.
- Basic frontend flows for login, registration, friends, requests, search, and direct chat.
- Redis-backed HTTP rate limiting and in-memory per-socket message rate limiting.

## 11. Partial / scaffolded / missing features

Partial:

- Direct chat is functional but lacks persisted delivery acknowledgements, read receipts storage, message status, retry reconciliation, and strict ordering guarantees.
- Frontend optimistic messages use a client-generated ID, but persisted backend messages use a separate server-generated ID; there is no ack to reconcile them.
- Friend request rejection is coded but conflicts with the database status constraint.
- Frontend block/unblock API functions exist but do not match backend body shape and are not used by current pages.
- Frontend WebSocket types include `ack`, `typing`, and `read`; backend only generically routes these events and only persists user-targeted events with text/content.

Scaffolded:

- Group chat routing structs/functions.
- Admin actions comment in user service.
- Soft-delete columns in database tables; no code uses soft delete.

Missing:

- Group chat product flow, group tables, membership management, and group UI.
- Message delete/edit APIs.
- Read receipt persistence.
- Delivery status persistence.
- Offline message delivery notifications beyond REST history.
- Redis Pub/Sub or distributed WebSocket scaling.
- Server-side refresh-token revocation/logout.
- Automated frontend tests.

## 12. Configuration

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

Limitations:

- `config/config.yaml` may contain local secrets and should not be documented or exposed.
- `config.example.yaml` has `CORS.host: http://localhost`, but CORS default construction expects a hostname and formats `http://<host>:<port>`; using a full URL there can produce an invalid default origin unless `allowed_origins` is configured.
- PostgreSQL pool duration strings are defined in config structs but current connection setup hard-codes lifetime and idle time.

## 13. Tests

Current Go tests:

- `internal/service/message_service_test.go`: message history uses the correct auth context key, rejects missing auth context, and returns an empty slice instead of nil.
- `internal/transport/websocket/hub_test.go`: message text extraction supports `text` and `content` payload fields.
- `internal/shared/jwt/jwt_test.go`: refresh token generation/validation uses configured issuer.

No frontend test suite is present in `web/package.json`; available frontend checks are `npm run lint` and `npm run build`.

## 14. Deployment/runtime structure

- `docker-compose.yml` runs only PostgreSQL and Redis for local development.
- The Go server runs separately via `cmd/server/main.go` and connects to configured database/Redis.
- Migrations are SQL Goose files under `migrations/`; the server does not run migrations automatically.
- Frontend is a separate Vite application under `web/`.
- WebSocket hub state is in memory inside one Go process.

## 15. Current limitations

- No automatic migration runner in application startup.
- WebSocket persistence is asynchronous and failures are only logged; clients are not told persistence failed.
- WebSocket hub has no distributed fan-out; multiple backend instances would not share connected users.
- Direct-message REST history is ordered newest first; UI appends new messages and does not reverse history.
- No authorization check verifies users are friends before messaging.
- No block check is performed before direct message persistence/routing.
- Error semantics are mostly human-readable messages, not stable machine codes.
- Some frontend/backend contract drift exists for token expiry and block API bodies.
- Friend request rejection currently conflicts with the migration-defined allowed statuses.
