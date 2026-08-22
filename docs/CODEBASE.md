# go-chat-system: Current Codebase Guide

This document describes the repository as it exists in source code, migrations,
configuration, and tests. Source code is authoritative over older plans or
documentation. Feature status is explicitly classified as **implemented**,
**partial**, **scaffolded**, or **missing**.

## 1. Project overview

`go-chat-system` is a Go backend with a React/TypeScript single-page frontend.
The implemented product path is registration/login, authenticated user search,
friend requests and friendships, blocking, direct message history, and direct
real-time messaging over authenticated WebSockets.

The normal backend dependency direction is:

```text
HTTP/WebSocket transport -> service -> repository -> PostgreSQL/Redis
```

The server does not run migrations automatically. The frontend is a separate
Vite application and is not embedded in the Go binary.

## 2. Technology stack

### Backend

- Go (`go.mod` declares Go 1.25.5)
- Chi router
- pgx/pgxpool for PostgreSQL
- Redis (`go-redis/v9`) for rate limiting and health checks
- Gorilla WebSocket
- Goose SQL migrations
- Viper/YAML configuration
- HS256 JWTs (`golang-jwt/jwt/v4`)
- bcrypt password hashing
- zap logging

### Frontend

- React 19, TypeScript, and Vite
- React Router
- Axios
- Tailwind CSS Vite integration

### Local infrastructure

`docker-compose.yml` defines PostgreSQL 16 and Redis 7 only. It does not build
or run the application or frontend.

## 3. Repository structure

```text
cmd/server/main.go                    startup and graceful shutdown
config/                               runtime YAML and example YAML
internal/domain/model/                domain and DTO structs
internal/platform/config/             Viper loader and env overrides
internal/platform/database/           PostgreSQL pool and Redis client
internal/repository/                  PostgreSQL queries and transactions
internal/service/                     validation, policy, orchestration
internal/shared/                      JWT, errors, logging, helpers, utilities
internal/transport/injector/          manual dependency wiring
internal/transport/middleware/        auth, CORS, logging, recovery, limits
internal/transport/routes/            Chi route registration
internal/transport/websocket/         hub, clients, rooms, WS envelope
internal/transport/wrapper/           REST response and WS upgrade wrappers
migrations/                           Goose schema and demo seed
web/src/api/                          REST and WebSocket client layer
web/src/context/                      auth and socket lifecycle state
web/src/pages/                        login, register, friends, chat UI
docs/                                 repository and deployment documentation
plans/                                planning documents; not runtime behavior
```

## 4. Backend architecture

`cmd/server/main.go` loads configuration, initializes zap logging, connects to
PostgreSQL and Redis, constructs the Chi router, and starts one `http.Server`.
SIGINT/SIGTERM stops the in-memory WebSocket hub first and then performs a
10-second HTTP graceful shutdown.

`internal/transport/injector/injector.go` is the composition root. It creates
the five repositories and five services and passes them to the route layer.

Transport owns parsing, authentication context, routing, WebSocket framing,
and serialization. Services own business rules. Repositories own SQL,
transactions, and scans. The global router middleware adds request IDs, CORS,
logging, panic recovery, JWT authentication on protected routes, and Redis
rate limits.

The REST wrapper returns `{"status":"ok","data":...}` for data responses,
`{"message":"ok"}` for successful nil responses, and
`{"status":"error","message":"..."}` for handled errors.

## 5. Frontend architecture

- `web/src/api/client.ts` owns the Axios client, bearer-token injection,
  localStorage token storage, and one-at-a-time 401 refresh queuing.
- `web/src/api/auth.ts`, `users.ts`, `friends.ts`, and `messages.ts` wrap REST
  contracts and normalize the backend response envelope.
- `web/src/api/websocket.ts` owns the singleton WebSocket, event dispatch,
  sending, and bounded exponential reconnects.
- `AuthContext` owns the current user and login/register/logout state.
- `SocketContext` connects the socket while authenticated and exposes socket
  actions/listeners to pages.
- `App.tsx` routes `/login`, `/register`, `/friends`, and `/chat/:userId` and
  applies public/protected route guards.
- `FriendsPage` implements friend listing, incoming requests, and user search.
- `ChatPage` loads history, displays direct messages, sends messages, and
  displays typing state.

The frontend REST base URL is currently hard-coded to
`http://localhost:8002/api/v1` in `web/src/api/client.ts`.

## 6. Database structure

The only Goose migration is
`migrations/20260126104003_initial_schema.sql`.

| Table | Current purpose |
| --- | --- |
| `users` | UUID identity, username, unique email, bcrypt hash, role, timestamps, `deleted_at`. |
| `friends` | Directed rows; mutual friendship is two rows. Composite primary key and no-self constraint. |
| `blocks` | Directed blocker/blocked rows with composite primary key and no-self constraint. |
| `friend_requests` | Sender, receiver, UUID, status (`pending`, `accepted`, `rejected`, `blocked`), timestamps. |
| `messages` | Sender, receiver, body, `is_group`, timestamps, and `deleted_at`. |

Foreign keys cascade user deletion for relationship tables. Indexes cover user
email, friend/block lookup, pending request receiver, and message sender or
receiver by creation time. The seed file contains demo data.

The schema has soft-delete columns, but current repositories mostly hard-delete
or omit `deleted_at` filtering. `messages.is_group` is present, but there are
no group or membership tables.

## 7. Authentication

Registration validates required fields, email format, and an eight-character
minimum password, hashes with bcrypt, creates a user, and returns an access
JWT. Login verifies the hash and returns access and refresh JWTs. Refresh
validates the refresh JWT and reloads the user before issuing both tokens.

Access claims contain user ID, email, role, issuer, issue time, and expiry.
Refresh claims contain user ID, issuer, issue time, and expiry. Protected
routes accept, in order, `Authorization: Bearer`, `?token=`, or the `access`
cookie. The validated user ID is stored in request context.

The WebSocket path uses the same middleware. `Client.ReadPump` overwrites any
client-supplied `sender_id` with the authenticated context identity.

**Partial/limitations:** refresh tokens are stateless and cannot be revoked;
there is no logout endpoint or server-side token store; browser tokens and the
stored user are kept in localStorage; registration does not return a refresh
token, unlike login and refresh.

## 8. REST APIs

All API routes below are prefixed with `/api/v1`. Protected routes require the
access JWT. Successful nil responses are encoded as `{"message":"ok"}`.

### Public authentication

| Method/path | Body | Success |
| --- | --- | --- |
| `POST /auth/register` | `username`, `email`, `password` | `201`; user, access `token`, `exp` |
| `POST /auth/login` | `email`, `password` | `200`; user, access/refresh tokens and expiries |
| `POST /auth/refresh` | `refresh_token` | `200`; new access/refresh tokens and expiries |

### Protected application routes

| Method/path | Body/query | Behavior |
| --- | --- | --- |
| `GET /users` | `filter`, `limit` (default 20, max 100) | Username/email search; filters shorter than two characters return empty. |
| `GET /friends` | `limit` (20/100), `offset` (0+) | Lists the authenticated user's friends. |
| `GET /friend-requests/` | none | Lists requests received by the authenticated user. |
| `POST /friend-requests/` | `{to}` | Creates a pending request after self, duplicate, friendship, and block checks. |
| `POST /friend-requests/accept` | `{request_id}` | Receiver-only acceptance; transaction creates mutual friendship. |
| `POST /friend-requests/reject` | `{request_id}` | Receiver-only rejection. |
| `POST /friend-requests/cancel` | `{request_id}` | Sender-only deletion of a pending request. |
| `POST /blocks/` | `{target}` | Creates a block, removes friendship rows, marks related requests blocked. |
| `POST /blocks/unblock` | `{target}` | Removes the authenticated user's block row. |
| `GET /messages` | `user_id`; `limit` (50/100), `offset` (0+) | Returns direct conversation history, newest-first from SQL. |
| `GET /ws` | authenticated upgrade | Opens a WebSocket connection. |

Message sending checks friendship and block status in the service. History
reads currently do not repeat those checks. API errors are mostly stable only
by HTTP status and human-readable message; there are no public error codes.

Health routes outside the API namespace are `GET /health/live`,
`GET /health/ready`, `GET /redis-health`, and `GET /db-health`.

## 9. WebSocket architecture

The endpoint is `GET /api/v1/ws`. The upgrade handler validates the authenticated
context and allowed origin, then creates a client with a 256-item send queue.
Each client has a read pump and a write pump. The hub stores multiple active
connections per user in memory.

Envelope:

```json
{"event":"message","sender_id":"server-set","receiver_id":"user-id","receiver_type":"user","data":{}}
```

For `message` to a user, the hub accepts `data.text` or `data.content`, calls
`MessageService.CreateMessage`, persists first, sends the persisted message to
all active receiver connections, and sends `{message_id,status:"sent"}` ack to
the sender. Persistence or malformed payload failures send an `error` event
to the sender. Direct messages require friendship and no block.

The server also routes generic user-targeted events. The frontend defines
`message`, `typing`, `read`, `ack`, and `error`; typing and read are currently
ephemeral generic routing only. The hub broadcasts `user_online` on a user's
first connection and `user_offline` after its last connection closes, but the
frontend does not consume these presence events.

Connection controls are a 10 KB read limit, 60-second read deadline refreshed
by pong, 10-second write deadline, 30-second ping, 10 inbound messages per
second, and removal of clients whose send queue is full. The frontend retries
up to five times with exponential delays starting at one second.

**Scaffolded:** `ReceiverType: group`, `Room`, `CreateRoom`, and group routing
exist in memory. There are no group routes, persistence, membership service, or
group UI. The hub is process-local; Redis is not used for WebSocket fan-out.

## 10. Currently implemented features

- Registration, login, password hashing, access validation, and refresh.
- Protected REST API and Redis HTTP rate limiting (10/minute public auth,
  120/minute protected user limit).
- User search, friend requests, mutual friendships, friend listing, blocking,
  and unblocking.
- Direct message persistence and REST history retrieval.
- Authenticated direct WebSocket delivery, sender identity protection,
  persistence ack/error, presence broadcast, ping/pong, limits, and multiple
  connections per user.
- React login/register, friends/search/request, and direct chat screens.
- PostgreSQL/Redis health checks and local Docker infrastructure.

## 11. Partial, scaffolded, and missing features

### Partial

- Typing indicators route through the backend and display in direct chat, but
  have no backend validation or persistence.
- Read receipt types and send helpers exist, but chat does not use them and no
  receipts are stored.
- Presence is emitted by the hub but not represented in frontend state.
- Chat uses optimistic client IDs while the server creates message IDs; ack
  reconciliation and failed-send rollback are not implemented.
- Soft delete is modeled but not consistently enforced.
- Refresh lifecycle, localStorage token storage, and hard-coded frontend URL
  are usable locally but incomplete for production.

### Scaffolded

- Group chat types, room registry, `is_group`, and group routing.
- User role field and TODO marker for admin actions.
- Database `deleted_at` fields and model fields without complete behavior.

### Missing

- Group management and membership persistence/UI.
- Message edit/delete, durable delivery/read status, and offline notifications.
- Password reset, email verification, profile/account management.
- Server-side logout or refresh-token revocation.
- Distributed WebSocket fan-out/presence for multiple backend instances.
- Automated frontend tests, production application containers, and Kubernetes
  manifests.

## 12. Configuration

`internal/platform/config/config.go` reads `config.yaml` from the working
directory or `./config/`, then applies non-empty overrides for `DB_HOST`,
`DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `REDIS_HOST`, `REDIS_PORT`,
`JWT_SECRET`, and `PORT`.

Configuration sections are `database`, `jwt`, `server`, `CORS`, `logging`, and
`redis`. `config/config.example.yaml` documents local defaults. Pool duration
fields are declared, but the current PostgreSQL setup applies fixed lifetime
and idle values instead. Do not expose `config/config.yaml` secrets.

HTTP CORS and WebSocket origin checks use configured `CORS.allowed_origins`, or
default to `http://<CORS.host>:<CORS.port>`.

## 13. Tests and verification

Current Go tests cover message service authorization/persistence behavior,
friend-request authenticated-actor behavior, WebSocket message parsing and
delivery/ack/error behavior, JWT refresh validation, response wrappers, error
helpers, and recovery middleware. There are no frontend tests in `web`.

Repository verification commands are:

```bash
go fmt ./...
go vet ./...
go test ./...
cd web && npm run lint && npm run build
```

## 14. Deployment and runtime structure

The runtime is one Go process serving REST and WebSocket traffic, plus external
PostgreSQL and Redis. `docker-compose.yml` maps PostgreSQL to host port 5433
and Redis to host port 6380, with named data volumes. Goose runs migrations
separately through Makefile targets. The frontend is run by Vite in development
or built to `web/dist` for a static host/reverse proxy.

Useful targets include `make docker-up`, `make migrate-up`, `make run`,
`make web-run`, `make build`, and `make migrate-status`. The repository has no
production Dockerfile for the application or frontend.

## 15. Current limitations

- WebSocket state and delivery are single-process only.
- Message history is offset-paginated and SQL-ordered newest-first; the UI
  re-sorts it chronologically.
- No startup migration runner, durable event status, or offline delivery.
- Some frontend/backend contract edges remain incomplete, especially optimistic
  message reconciliation and optional event handling.
- Error responses lack machine-readable codes.
- Soft-delete semantics are incomplete.
- Browser tokens are stored in localStorage and refresh tokens are revocable
  only by changing the signing secret.

For deployment-specific commands and checklist, see `docs/DEPLOYMENT.md`.
