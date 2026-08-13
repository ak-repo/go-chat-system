# Current Repository Documentation

This document describes the current source code in this repository. It was prepared from the implementation, migrations, configuration, and tests rather than from older documentation.

## 1. Project Overview

`go-chat-system` is a backend-first real-time chat application with a Go HTTP/WebSocket API and a React TypeScript frontend.

The implemented product path is:

1. users register or log in;
2. authenticated users search for other users;
3. users send, accept, reject, or cancel friend requests;
4. accepted friend requests create mutual friendships;
5. friends can load message history and send one-to-one chat messages over WebSocket;
6. messages are persisted in PostgreSQL before WebSocket delivery.

## 2. Technology Stack

### Backend

- Go module: `github.com/ak-repo/go-chat-system`
- Router: Chi (`github.com/go-chi/chi`)
- PostgreSQL driver/pool: `pgx/v5`
- Redis client: `redis/go-redis/v9`
- WebSocket: Gorilla WebSocket
- JWT: `github.com/golang-jwt/jwt/v4`
- Config: Viper reading YAML plus selected environment overrides
- Password hashing: `golang.org/x/crypto` bcrypt helpers under `internal/shared/utils/`
- Migrations: Goose SQL migrations in `migrations/`

### Frontend

- React 19
- TypeScript
- Vite
- React Router
- Axios
- Tailwind CSS Vite plugin

### Runtime dependencies

- PostgreSQL 16 and Redis 7 are defined in `docker-compose.yml` for local infrastructure.

## 3. Repository Structure

Important current paths:

```text
cmd/server/                    Go server entry point
config/                        YAML config and example config
internal/domain/model/         Domain and DTO structs
internal/platform/config/      Viper config loading
internal/platform/database/    PostgreSQL and Redis initialization
internal/repository/           PostgreSQL repositories
internal/service/              HTTP-facing service methods and business logic
internal/shared/               JWT, errors, logger, helpers, responses, validation
internal/transport/injector/   Manual dependency wiring
internal/transport/middleware/ HTTP middleware
internal/transport/routes/     Chi route registration
internal/transport/websocket/  Hub, client pumps, room scaffold, WS message type
internal/transport/wrapper/    HTTP response wrapper and WebSocket upgrade handler
migrations/                    Goose SQL migrations
web/src/api/                   REST and WebSocket client code
web/src/context/               Auth and Socket React contexts
web/src/pages/                 Login, register, friends, chat pages
docs/                          Documentation
plans/                         Feature plans
```

## 4. Backend Architecture

The backend follows the repository's intended flow:

```text
HTTP / WebSocket transport
  -> service layer
  -> repository layer
  -> PostgreSQL / Redis
```

### Startup

`cmd/server/main.go` performs startup in this order:

1. initializes logging;
2. loads configuration from `config/config.yaml` or `./config.yaml`;
3. connects to PostgreSQL;
4. initializes Redis;
5. builds the router using `routes.Router()`;
6. starts `http.Server` on `config.Config.Server.Port`;
7. on SIGINT/SIGTERM, stops the WebSocket hub if present and gracefully shuts down HTTP.

### Dependency wiring

`internal/transport/injector/injector.go` creates repositories and services manually:

- repositories: users, friends, friend requests, blocks, messages;
- services: users, friends, friend requests, blocks, messages.

### Routes and middleware

`internal/transport/routes/routes.go` defines routes under `/api/v1` plus health checks.

Global middleware:

- Chi logger;
- Chi request ID;
- custom CORS;
- custom panic recovery.

Public auth routes use Redis-backed IP rate limiting: 10 requests per minute.

Protected routes use JWT auth and Redis-backed user rate limiting: 120 requests per minute.

### Response envelope

HTTP services return through `internal/transport/wrapper/wrapper.go`.

Success responses with data use:

```json
{ "status": "ok", "data": { } }
```

Success responses with no body currently use:

```json
{ "message": "ok" }
```

Error responses use:

```json
{ "status": "error", "message": "..." }
```

## 5. Frontend Architecture

The frontend lives under `web/`.

### API layer

- `web/src/api/client.ts` creates an Axios client with hard-coded `BASE_URL = http://localhost:8002/api/v1`.
- The Axios request interceptor attaches `Authorization: Bearer <token>` when a token exists.
- The Axios response interceptor refreshes access tokens on HTTP 401 using `/auth/refresh` and stored refresh token.
- `web/src/api/auth.ts`, `users.ts`, `friends.ts`, and `messages.ts` wrap REST endpoints.
- `web/src/api/websocket.ts` owns WebSocket connection/reconnect/send/listener behavior.

### Contexts

- `AuthContext` stores authenticated user state and persists the user in `localStorage`.
- `SocketContext` connects or disconnects the singleton WebSocket client based on auth state.

### Pages

- `/login`: login form.
- `/register`: registration form.
- `/friends`: authenticated page for friends, incoming requests, and search.
- `/chat/:userId`: authenticated one-to-one chat page.

## 6. Database Structure

The current schema is defined by `migrations/20260126104003_initial_schema.sql`.

### Tables

#### `users`

- `id UUID PRIMARY KEY`
- `username TEXT NOT NULL`
- `email TEXT NOT NULL UNIQUE`
- `password_hash TEXT NOT NULL`
- `role TEXT NOT NULL DEFAULT 'user'`
- timestamps: `created_at`, `modified_at`, `deleted_at`
- index: `idx_users_email`

#### `friends`

- mutual friendship rows are stored as two rows: `(a,b)` and `(b,a)`
- primary key: `(user_id, friend_id)`
- self-friendship check constraint
- index: `idx_friends_user`

#### `blocks`

- primary key: `(blocker_id, blocked_id)`
- self-block check constraint
- index: `idx_blocks_blocker`

#### `friend_requests`

- `id UUID PRIMARY KEY`
- `sender_id`, `receiver_id`
- `status` limited to `pending`, `accepted`, `rejected`, `blocked`
- no self-request check constraint
- unique unordered pending request index prevents duplicate pending requests in either direction
- pending receiver index

#### `messages`

- `id UUID PRIMARY KEY`
- `sender_id`, `receiver_id`
- `body TEXT NOT NULL`
- `is_group BOOLEAN NOT NULL DEFAULT FALSE`
- timestamps: `created_at`, `modified_at`, `deleted_at`
- indexes by receiver/time and sender/time

### Notable behavior

- The schema contains soft-delete timestamp columns, but repositories generally use hard deletes or ignore `deleted_at` filters.
- Group messaging has an `is_group` field in `messages`, but no group tables exist.

## 7. Authentication

### Implemented

- Registration hashes passwords and creates a user.
- Login verifies bcrypt password hashes.
- Access JWTs include `user_id`, `email`, `role`, issuer, issued-at, and expiry.
- Refresh JWTs include `user_id`, issuer, issued-at, and expiry.
- Protected HTTP and WebSocket routes use `AuthMiddleware`.
- Auth middleware accepts tokens from, in order:
  1. `Authorization: Bearer <token>` header;
  2. `?token=` query parameter, used by WebSocket;
  3. `access` cookie.
- Authenticated user ID is injected into request context using `middleware.UserIDKey`.
- WebSocket client input `sender_id` is overwritten server-side with the authenticated user ID in `Client.ReadPump`.

### Partial / limitations

- Refresh tokens are stateless JWTs; there is no server-side refresh token storage, rotation tracking, revocation, or logout invalidation.
- The frontend stores access tokens, refresh tokens, and user data in `localStorage`.
- Registration response includes an access token but not a refresh token, while login and refresh responses include refresh tokens.

## 8. REST APIs

All API routes are under `/api/v1` unless noted.

### Public auth endpoints

#### Register

- Method: `POST`
- Path: `/auth/register`
- Auth: none
- Body: `{ "username": string, "email": string, "password": string }`
- Success: `201` with `data.user`, `data.token`, `data.exp`
- Validation: requires all fields, valid email, password length at least 8 in backend validation

#### Login

- Method: `POST`
- Path: `/auth/login`
- Auth: none
- Body: `{ "email": string, "password": string }`
- Success: `200` with `data.user`, `data.token`, `data.exp`, `data.refresh_token`, `data.refresh_exp`

#### Refresh token

- Method: `POST`
- Path: `/auth/refresh`
- Auth: refresh token in body
- Body: `{ "refresh_token": string }`
- Success: `200` with `data.token`, `data.exp`, `data.refresh_token`, `data.refresh_exp`

### Protected endpoints

Protected endpoints require JWT auth.

#### Search users

- Method: `GET`
- Path: `/users`
- Query: `filter`, optional `limit` default 20, max 100
- Success: `200` with `data.users`
- Behavior: repository returns an empty list if `filter` is shorter than 2 characters.

#### List friends

- Method: `GET`
- Path: `/friends`
- Query: `limit` default 20 max 100, `offset` default 0
- Success: `200` with `data.friends`, `data.limit`, `data.offset`

#### List received friend requests

- Method: `GET`
- Path: `/friend-requests/`
- Success: `200` with `data.requests`
- Behavior: returns requests where authenticated user is the receiver.

#### Create friend request

- Method: `POST`
- Path: `/friend-requests/`
- Body: `{ "to": string }`
- Success: `201` with `{ "message": "ok" }`
- Authorization/business rules: sender is authenticated user; self-request rejected; already friends rejected; duplicate pending requests rejected; blocked relationship rejected if receiver has blocked sender.

#### Accept friend request

- Method: `POST`
- Path: `/friend-requests/accept`
- Body: `{ "request_id": string }`
- Success: `200` with `{ "message": "ok" }`
- Behavior: only the receiver from authenticated context can accept; repository updates request to accepted and inserts mutual friendship rows in a transaction.

#### Reject friend request

- Method: `POST`
- Path: `/friend-requests/reject`
- Body: `{ "request_id": string }`
- Success: `200` with `{ "message": "ok" }`
- Behavior: only the receiver from authenticated context can reject.

#### Cancel friend request

- Method: `POST`
- Path: `/friend-requests/cancel`
- Body: `{ "request_id": string }`
- Success: `200` with `{ "message": "ok" }`
- Behavior: only the sender from authenticated context can cancel; repository deletes the pending request.

#### Block user

- Method: `POST`
- Path: `/blocks/`
- Body: `{ "target": string }`
- Success: `200` with `{ "message": "ok" }`
- Behavior: inserts a block, deletes friendship rows both ways, marks related friend requests as `blocked` in one transaction.

#### Unblock user

- Method: `POST`
- Path: `/blocks/unblock`
- Body: `{ "target": string }`
- Success: `200` with `{ "message": "ok" }`
- Behavior: deletes the block row for authenticated user -> target.

#### Get message history

- Method: `GET`
- Path: `/messages`
- Query: required `user_id`, optional `limit` default 50 max 100, optional `offset` default 0
- Success: `200` with `data.messages`, `data.limit`, `data.offset`
- Behavior: returns messages between authenticated user and `user_id`, ordered newest-first from SQL; frontend sorts them chronologically for display.
- Limitation: `GetConversation` does not currently re-check friendship or block status for history reads.

### Health endpoints

- `GET /health/live`: returns `ok`.
- `GET /health/ready`: pings Redis and PostgreSQL.
- `GET /redis-health`: pings Redis.
- `GET /db-health`: pings PostgreSQL.

## 9. WebSocket Architecture

### Endpoint

- Method: `GET`
- Path: `/api/v1/ws`
- Auth: protected by `AuthMiddleware`; frontend connects with `?token=<access token>`.
- Upgrade handler: `internal/transport/wrapper/ws_handler.go`.

### Server-side flow

```text
Authenticated HTTP request
  -> Gorilla WebSocket upgrade
  -> ws.NewClient(hub, conn, authenticatedUserID)
  -> client ReadPump / WritePump goroutines
  -> Hub incoming channel
  -> Hub routing / service persistence
  -> receiver client send queues
```

### Message type

`internal/transport/websocket/ws_message.go` defines:

```json
{
  "event": "message",
  "sender_id": "server-set",
  "receiver_id": "target-user-or-room",
  "receiver_type": "user|group",
  "data": {}
}
```

### Implemented realtime events

#### `message`

- Direction: client -> server -> receiver; server also sends `ack` to sender.
- Authenticated actor: `sender_id` is set from the WebSocket authenticated user.
- Client payload accepted by backend: `data.text` or `data.content`.
- Persistence: server calls `MessageService.CreateMessage` before delivery.
- Authorization: non-group messages require no block relationship and require friendship.
- Receiver delivery payload: `data.message_id`, `data.content`, `data.timestamp`.
- Sender ack payload: `{ "message_id": string, "status": "sent" }`.
- Error event: server sends `event: "error"` to sender if parsing or persistence fails.

#### `typing` and `read`

- Partial: frontend can send and listen for `typing` and `read` events.
- Backend behavior: these are not persisted and not specially handled; they fall through to direct routing by `receiver_type`.
- No authorization beyond WebSocket authentication is applied for these fall-through events.

#### Presence events

- Partial: hub broadcasts `user_online` on registration and `user_offline` after the final connection for a user disconnects.
- Frontend type union does not include `user_online` or `user_offline`, and pages do not consume these events.

### Connection lifecycle

Implemented server controls:

- maximum WebSocket message size: 10 KB;
- read deadline: 60 seconds;
- write deadline: 10 seconds;
- ping every 30 seconds;
- pong handler refreshes read deadline;
- per-client inbound rate limit: 10 messages per second;
- send queue buffer size: 256;
- slow/full receiver queues are closed and removed;
- multiple active connections per user are supported with `map[userID]map[*Client]bool`.

Frontend WebSocket behavior:

- singleton `wsClient`;
- token passed as query parameter;
- reconnects up to 5 attempts with exponential delay starting at 1 second;
- closes an existing socket before opening a new one;
- dispatches events by event name to registered handlers.

### Scaffolded / missing realtime pieces

- Group rooms exist in memory through `Room` and `CreateRoom`, but no API creates rooms and no database group model exists.
- No distributed WebSocket routing; hub is in-memory and process-local.
- Offline delivery is persistence-only: messages are saved, but no push/notification system exists.
- No durable read receipts.
- No durable typing state.

## 10. Currently Implemented Features

- User registration with password hashing and access token creation.
- User login with access and refresh token creation.
- Access token validation for protected routes and WebSocket upgrades.
- Refresh token endpoint using stateless refresh JWTs.
- User search with minimum two-character filter.
- Friend request creation, listing received requests, acceptance, rejection, and cancellation.
- Mutual friendship creation when a request is accepted.
- Friend listing.
- Blocking and unblocking users.
- Blocking removes existing friendships and marks related friend requests blocked.
- Message history retrieval between two users.
- One-to-one WebSocket message persistence and delivery to online receivers.
- Sender acknowledgement for persisted WebSocket messages.
- WebSocket sender identity override using authenticated context.
- WebSocket ping/pong, read/write deadlines, input size limit, per-client rate limit, and multiple connections per user.
- React login, register, friends/search/requests, and chat pages.
- Axios token attachment and 401 refresh flow.
- Local Docker Compose infrastructure for PostgreSQL and Redis.
- Health endpoints for liveness, readiness, Redis, and database.

## 11. Partially Implemented / Scaffolded / Missing Features

### Partial

- **Typing indicators:** frontend sends and displays them; backend only routes them without domain validation or persistence.
- **Read receipts:** frontend API can send/listen; backend only routes generic events; chat page does not currently use `onRead`.
- **Presence:** backend broadcasts online/offline; frontend does not consume those events.
- **Refresh token lifecycle:** refresh tokens work, but are stateless and not revocable.
- **Soft deletes:** schema has `deleted_at`, but repository queries mostly ignore it.
- **Message authorization:** message send checks friendship/block status; history retrieval does not currently apply the same checks.

### Scaffolded

- **Group messaging:** `receiver_type: "group"`, `is_group`, and in-memory `Room` exist; no group tables, routes, services, repository methods, or frontend group UI exist.
- **Admin role/actions:** users have a `role`; service contains TODO comments for admin actions, but no admin routes are implemented.

### Missing

- Password reset / email verification.
- Profile update or account deletion endpoints.
- Logout endpoint or token revocation.
- Server-side session management.
- Message edit/delete endpoints.
- Attachments/media upload.
- Message delivery receipts beyond the immediate sender `ack`.
- Durable read receipts.
- Notification system for offline users.
- Production container image or Kubernetes manifests.
- Automated migration execution during application startup.
- Frontend environment-based API URL configuration.

## 12. Configuration

Configuration is loaded by `internal/platform/config/config.go` with Viper.

### Config file

The app reads `config.yaml` from either the working directory or `./config/`. `config/config.example.yaml` documents expected settings.

Important sections:

- `database`: host, port, user, password, name, sslmode, and pool settings.
- `jwt`: secret, expiry, issuer, refresh expiry field in code.
- `server`: host and port.
- `CORS`: host, port, allowed origins field in code.
- `logging`: level, format, output.
- `redis`: host, port, password, db.

### Environment overrides

The loader supports these environment variables when non-empty:

- `DB_HOST`
- `DB_PORT`
- `DB_USER`
- `DB_PASSWORD`
- `DB_NAME`
- `REDIS_HOST`
- `REDIS_PORT`
- `JWT_SECRET`
- `PORT`

### CORS and WebSocket origin checks

Both HTTP CORS and WebSocket upgrade origin checks use `config.Config.CORS.AllowedOrigins` if configured; otherwise they default to `http://<CORS.host>:<CORS.port>`.

### Frontend configuration limitation

The frontend API base URL is hard-coded in `web/src/api/client.ts` as `http://localhost:8002/api/v1`.

## 13. Tests

Current tests are focused and mostly unit-level:

- `internal/service/message_service_test.go`
  - context key behavior for authenticated message history;
  - empty message response shape;
  - message send requires friendship;
  - blocked relationships reject messages;
  - message body trimming and persistence.
- `internal/service/friend_request_service_test.go`
  - accept/reject uses authenticated receiver ID rather than body-supplied user IDs.
- `internal/transport/websocket/hub_test.go`
  - WebSocket payload accepts `text` and `content`;
  - persisted messages are delivered and acked;
  - persistence failures send an error and do not deliver to receiver.
- `internal/shared/jwt/jwt_test.go`
  - refresh token generation/validation uses configured issuer.

No frontend tests are currently present in the inspected tree.

Recommended verification commands from repository conventions:

```bash
go fmt ./...
go vet ./...
go test ./...

cd web
npm run lint
npm run build
```

## 14. Deployment / Runtime Structure

### Local infrastructure

`docker-compose.yml` starts:

- PostgreSQL 16 on host port `5433`, container port `5432`;
- Redis 7 on host port `6380`, container port `6379`;
- named volumes `pgdata` and `redis_data`.

### Application runtime

- Backend runs as a Go HTTP server from `cmd/server`.
- Migrations are managed externally with Goose; Makefile targets call `goose -dir migrations postgres "<dsn>" up/down/status`.
- Frontend runs separately with Vite using `npm run dev` or builds static assets with `npm run build`.
- There is no current production Dockerfile for the Go app or frontend.

### Makefile targets

- `make run`: run backend locally.
- `make build`: build Go binary to `bin/server`.
- `make web-run`: run frontend dev server.
- `make migrate-up`, `migrate-down`, `migrate-status`, `migrate-create`: Goose migration helpers.
- `make docker-up`, `docker-down`, `docker-clean`: local infrastructure helpers.

## 15. Current Limitations

- WebSocket hub is in-memory and does not support horizontal scaling across multiple backend instances.
- Message history uses offset pagination ordered newest-first in SQL; frontend re-sorts for display.
- No migration runner is wired into server startup.
- No group chat operational flow exists despite group-oriented fields and room scaffold.
- Typing/read/presence are not fully integrated product features.
- Frontend API URL is hard-coded to localhost.
- Tokens are stored in browser `localStorage`.
- Refresh tokens cannot be revoked server-side.
- Error semantics are mostly human-readable messages rather than stable machine codes.
- Some repository queries do not filter out soft-deleted rows.
- Registration frontend allows `minLength={6}`, but backend requires at least 8 characters.
- `config/config.yaml` may contain local secrets and should not be exposed.
