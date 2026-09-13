# go-chat-system: Current Codebase Guide

This document describes the repository as it exists in source code, migrations,
configuration, and tests. Source code is authoritative over older plans or
documentation. Feature status is explicitly classified as **implemented**,
**partial**, **scaffolded**, or **missing**.

## 1. Project overview

`go-chat-system` is a Go backend with a React/TypeScript single-page frontend.
The implemented product path is registration/login with persisted sessions,
logout and token rotation, profile/account actions, password recovery and
verification token flows, authenticated user search, friend requests and
friendships, blocking, direct conversations, direct message history and
mutations, delivery/read/unread state, and direct real-time messaging over
authenticated WebSockets. Verification is not an authentication gate, and
recovery/verification delivery is development-only.

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
web/src/pages/                        auth, recovery, verification, profile, friends, chat UI
docs/                                 repository and deployment documentation
plans/                                planning documents; not runtime behavior
```

## 4. Backend architecture

`cmd/server/main.go` loads configuration, initializes zap logging, connects to
PostgreSQL and Redis, constructs the Chi router, and starts one `http.Server`.
SIGINT/SIGTERM stops the in-memory WebSocket hub first and then performs a
10-second HTTP graceful shutdown.

`internal/transport/injector/injector.go` is the composition root. It creates
the repositories and services for users/sessions/account tokens, friendships,
blocks, conversations, and messages, then passes them to the route layer. The
account-token service is wired with `DevelopmentDelivery`, which records the
last token in memory rather than sending email.

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
- `web/src/api/auth.ts`, `users.ts`, `friends.ts`, `conversations.ts`, and
  `messages.ts` wrap REST contracts and normalize the backend response envelope.
- `web/src/api/websocket.ts` owns the singleton WebSocket, event dispatch,
  sending, and bounded exponential reconnects.
- `AuthContext` owns the current user and login/register/logout state.
- `SocketContext` connects the socket while authenticated and exposes socket
  actions/listeners to pages.
- `App.tsx` routes `/login`, `/register`, `/recover`, `/verify`, `/friends`,
  `/profile`, and `/chat/:userId`, and applies public/protected route guards.
- `FriendsPage` implements friend listing, incoming requests, and user search.
- `ChatPage` creates/opens a direct conversation, loads paginated history,
  displays direct messages and statuses, sends messages with client IDs,
  retries failed sends, marks incoming messages read, and displays typing state.

The frontend REST base URL is currently hard-coded to
`http://localhost:8002/api/v1` in `web/src/api/client.ts`.

## 6. Database structure

The final schema is defined by the single fresh-install migration
`migrations/20260829120500_canonical_schema.sql`. It replaces the previously
split Phase 1 migration set and must be applied to a database with no prior
versions from that set. Its `Down` is intentionally destructive.

| Table | Current purpose |
| --- | --- |
| `users` | UUID identity, username, unique email, bcrypt hash, role, timestamps, `deleted_at`, and nullable `verified_at`. |
| `friends` | Directed rows; mutual friendship is two rows. Composite primary key and no-self constraint. |
| `blocks` | Directed blocker/blocked rows with composite primary key and no-self constraint. |
| `friend_requests` | Sender, receiver, UUID, status (`pending`, `accepted`, `rejected`, `blocked`), timestamps. |
| `messages` | Sender, receiver, conversation, body, `is_group`, timestamps, deletion/edit state, client idempotency ID, and optional reply. |
| `sessions` | Hashed refresh-token sessions with expiry, rotation, and revocation state. |
| `account_tokens` | One-time hashed verification and password-reset tokens. |
| `conversations` | Currently direct conversations with a canonical user pair. |
| `conversation_members` | Active/left membership for conversations. |
| `message_deliveries` | Per-recipient monotonic sent/delivered/read/failed state. |
| `conversation_read_state` | Per-user last-read message and timestamp. |

Foreign keys constrain relationships, conversations, and messages. Indexes cover
user discovery, sessions, account-token lookup, friend/block lookup, pending
requests, conversations/members, messages, idempotency, replies, delivery, and
read state. The seed file contains demo data.

User and message deactivation/deletion use `deleted_at` filtering in the active
flows. `messages.is_group` and group receiver types remain scaffolding; the
conversation schema currently permits only direct conversations.

## 7. Authentication

Registration validates required fields, email format, and an eight-character
minimum password, hashes with bcrypt, creates a user and session, and returns
access and refresh JWTs. Login does the same after verifying bcrypt. Refresh
validates the JWT, rotates the stored session, and issues both new tokens.
Logout validates the authenticated user's refresh token and revokes its session;
password changes/resets and deactivation revoke the user's sessions.

Access claims contain user ID, email, role, issuer, issue time, and expiry.
Refresh claims contain user ID, issuer, issue time, and expiry. Protected
routes accept, in order, `Authorization: Bearer`, `?token=`, or the `access`
cookie. The validated user ID is stored in request context.

The WebSocket path uses the same middleware. `Client.ReadPump` overwrites any
client-supplied `sender_id` with the authenticated context identity.

**Implemented with limitations:** refresh-token hashes and session state are
stored server-side and checked on protected routes and by the WebSocket hub.
Browser tokens and the stored user remain in localStorage. Recovery and
verification use development-only delivery, and verification is informational:
`verified_at` is not enforced as an authentication gate.

## 8. REST APIs

All API routes below are prefixed with `/api/v1`. Protected routes require the
access JWT. Successful nil responses are encoded as `{"message":"ok"}`.

### Public authentication

| Method/path | Body | Success |
| --- | --- | --- |
| `POST /auth/register` | `username`, `email`, `password` | `201`; user, access `token`, `exp` |
| `POST /auth/login` | `email`, `password` | `200`; user, access/refresh tokens and expiries |
| `POST /auth/refresh` | `refresh_token` | `200`; new access/refresh tokens and expiries |
| `POST /auth/logout` | `refresh_token` | Revokes the authenticated session. |
| `POST /auth/password-reset/request` | `email` | Generic recovery response; development delivery records the token. |
| `POST /auth/password-reset/confirm` | `token`, `password` | Consumes token, changes password, and revokes sessions. |
| `POST /auth/verification/request` | `email` | Generic verification response; development delivery records the token. |
| `POST /auth/verification/confirm` | `token` | Consumes token and sets `users.verified_at`. |

### Protected application routes

| Method/path | Body/query | Behavior |
| --- | --- | --- |
| `GET /users` | `filter`, `limit` (default 20, max 100) | Username/email search; filters shorter than two characters return empty. |
| `GET /users/me` | none | Returns the authenticated user's profile DTO. |
| `PATCH /users/me` | `username`, `email` | Updates the authenticated user's profile fields. |
| `POST /users/me/change-password` | `password` | Changes password and revokes all sessions. |
| `DELETE /users/me` | none | Soft-deactivates the account and revokes all sessions. |
| `GET /friends` | `limit` (20/100), `offset` (0+) | Lists the authenticated user's friends. |
| `GET /friend-requests/` | none | Lists requests received by the authenticated user. |
| `POST /friend-requests/` | `{to}` | Creates a pending request after self, duplicate, friendship, and block checks. |
| `POST /friend-requests/accept` | `{request_id}` | Receiver-only acceptance; transaction creates mutual friendship. |
| `POST /friend-requests/reject` | `{request_id}` | Receiver-only rejection. |
| `POST /friend-requests/cancel` | `{request_id}` | Sender-only deletion of a pending request. |
| `POST /blocks/` | `{target}` | Creates a block, removes friendship rows, marks related requests blocked. |
| `POST /blocks/unblock` | `{target}` | Removes the authenticated user's block row. |
| `POST /conversations` | `{user_id}` | Creates or gets a direct conversation; requires friendship and no block. |
| `GET /conversations` | `limit` (50/100), `offset` (0+) | Lists active conversations for the authenticated member. |
| `GET /conversations/{conversationID}` | none | Gets a conversation only for an active member. |
| `GET /conversations/{conversationID}/messages` | `limit` (50/100), `offset` (0+) | Returns authorized direct conversation history. |
| `POST /conversations/{conversationID}/messages` | `content`, optional client/reply IDs | Persists an authorized message with duplicate-safe client ID handling. |
| `PATCH /messages/{messageID}` | `{content}` | Sender-only message edit. |
| `DELETE /messages/{messageID}` | none | Sender-only soft delete. |
| `POST /messages/delivery` | `{message_id}`, `status` (`delivered`/`read`) | Recipient-only monotonic delivery update. |
| `POST /conversations/{conversationID}/read` | `{message_id}` | Marks the recipient's conversation messages through a message as read. |
| `GET /unread` | none | Returns unread counts keyed by conversation ID. |
| `GET /ws` | authenticated upgrade | Opens a WebSocket connection. |

Conversation access and message history/send operations check authenticated
membership and the direct friendship/block relationship. Message edit/delete
is sender-only; replies are available through the message send contract and
WebSocket mutation path. API errors are mostly stable only by HTTP status and
human-readable message; there are no public error codes.

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

For `message` to a user, the hub validates the envelope, derives the actor from
the authenticated socket, calls the message service, persists first (including
conversation and delivery creation), sends the persisted message to all active
receiver connections, and sends an ack containing server/client IDs and status
to the sender. Client IDs make retries duplicate-safe. Persistence or malformed
payload failures send an `error` event. Direct messages require conversation
membership, friendship, and no block.

The server supports `message.edited`, `message.deleted`, `message.replied`,
`message.delivered`, and `message.read` mutation/status events. These are
authorized and persisted through `MessageService` before being routed and
acknowledged. `typing` is authorized but intentionally ephemeral. The hub also
broadcasts `user_online` on a user's first connection and `user_offline` after
its last connection closes; the frontend accepts these event types but does
not maintain a presence view.

Connection controls are a 10 KB read limit, 60-second read deadline refreshed
by pong, 10-second write deadline, 30-second ping, 10 inbound messages per
second, and removal of clients whose send queue is full. The frontend retries
up to five times with exponential delays starting at one second.

**Scaffolded:** `ReceiverType: group`, `Room`, `CreateRoom`, and group routing
exist in memory. There are no group routes, group persistence, or group UI. The
hub is process-local; Redis is not used for WebSocket fan-out or presence.

## 10. Currently implemented features

- Registration, login, password hashing, access validation, and refresh.
- Persisted sessions, refresh rotation, logout/revocation, profile updates,
  password changes, soft deactivation, password recovery, and verification
  token consumption.
- Protected REST API and Redis HTTP rate limiting (10/minute public auth,
  120/minute protected user limit).
- User search, friend requests, mutual friendships, friend listing, blocking,
  and unblocking.
- Direct conversation creation/list/get, membership checks, message persistence
  and REST history retrieval.
- Message edit/delete/reply, client-idempotent sends, durable delivery/read
  state, and unread summaries through REST and service/repository paths.
- Authenticated direct WebSocket delivery and mutation/status events, sender
  identity protection, persistence ack/error, presence broadcast, ping/pong,
  limits, and multiple connections per user.
- React login/register, recovery/verification, profile, friends/search/request,
  and direct chat screens with pagination, optimistic send reconciliation,
  retry, read marking, and unread loading.
- PostgreSQL/Redis health checks and local Docker infrastructure.

## 11. Partial, scaffolded, and missing features

### Partial

- Typing indicators are membership-authorized and display in direct chat, but
  are ephemeral and not persisted.
- Presence is emitted by the hub but not represented in frontend state.
- REST and WebSocket mutation/status contracts exist, but the chat UI does not
  yet render edit/delete/reply controls or all inbound mutation events.
- Soft delete is modeled but not consistently enforced.
- Refresh lifecycle, localStorage token storage, and hard-coded frontend URL
  are usable locally but incomplete for production.
- Recovery and verification flows are operational, but delivery is only the
  in-memory development adapter and verification is not enforced.

### Scaffolded

- Group chat types, room registry, `is_group`, and group routing.
- User role field and TODO marker for admin actions.
- Database `deleted_at` fields and model fields without complete behavior.

### Missing

- Group management and membership persistence/UI.
- Offline notifications and queued delivery while a recipient is disconnected.

Registration deliberately does not require email verification for compatibility
with the current client and existing accounts. Verification tokens are
implemented, but `verified_at` is informational rather than an authentication
gate.
- Distributed WebSocket fan-out/presence for multiple backend instances.
- Automated frontend tests, production application containers, and Kubernetes
  manifests. PostgreSQL repository integration tests are available with the
  `integration` build tag and require `TEST_DATABASE_URL`.

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

Current Go tests cover service authorization, validation, persistence behavior,
WebSocket message parsing and delivery/ack/error behavior, JWT refresh
validation, response wrappers, error helpers, and recovery middleware.
Repository integration tests cover PostgreSQL persistence and transactions when
run with the `integration` build tag. There are no automated frontend tests.

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
- No startup migration runner or offline delivery/event replay.
- The chat UI does not yet expose edit/delete/reply controls or render every
  mutation/status event, although the backend REST and WebSocket paths exist.
- Error responses lack machine-readable codes.
- Soft-delete semantics are incomplete.
- Browser tokens are stored in localStorage. WebSocket credentials are passed
  in the `token` query string, which can expose access tokens to intermediary
  logs; use a safer upgrade mechanism before production deployment.
- Recovery and verification delivery is development-only (`DevelopmentDelivery`)
  rather than email/phone delivery, and verification is not enforced.
- Automated frontend tests and PostgreSQL repository/integration tests are not
  implemented.

For deployment-specific commands and checklist, see `docs/DEPLOYMENT.md`.
