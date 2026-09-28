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
unverified accounts cannot sign in. Email verification and password recovery
support configurable SMTP delivery; without SMTP configuration, the server uses
the development-only adapter, which prints usable verification/reset links to
the server log. This log behavior is enabled only when `app.environment` is
explicitly `development` (or `APP_ENV=development`).

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
- `App.tsx` routes authentication, discovery/profile, conversation, group,
  notification, and invite-acceptance screens and applies public/protected route
  guards. `/chat/:userId` resolves a direct conversation before routing to
  `/conversations/:conversationId`.
- `FriendsPage` implements friend listing, incoming requests, and user search.
- `ChatPage` handles direct and group conversations, paginated history,
  idempotent optimistic sends/retries, edit/delete/reply/forward/copy/reactions,
  structured mentions, persisted statuses, read receipts, presence, and
  multi-user typing.
- `ConversationsPage` lists active/archived conversations, preferences, unread
  counts, and the active-conversation mark-all-as-read action.
- `GroupSettingsPage` manages group metadata, members, roles, and owner/admin
  invite links. `/invites/:token` accepts an invite for an authenticated user.

The frontend REST base URL is currently hard-coded to
`http://localhost:8002/api/v1` in `web/src/api/client.ts`.

## 6. Database structure

The fresh-install schema starts with
`migrations/20260829120500_canonical_schema.sql`; additive changes are defined by
later migrations for conversation preferences and Phase 2 groups, interactions,
mentions/notifications, invites, and integrity constraints.
Never edit an already-deployed migration. The canonical migration's `Down` is
intentionally destructive; the conversation-state migration's `Down` removes
only the new preference data/table.

| Table | Current purpose |
| --- | --- |
| `users` | UUID identity, username, unique email, bcrypt hash, role, timestamps, `deleted_at`, and nullable `verified_at`. |
| `friends` | Directed rows; mutual friendship is two rows. Composite primary key and no-self constraint. |
| `blocks` | Directed blocker/blocked rows with composite primary key and no-self constraint. |
| `friend_requests` | Sender, receiver, UUID, status (`pending`, `accepted`, `rejected`, `blocked`), timestamps. |
| `messages` | Sender, conversation, optional direct receiver, body, direct/group kind, timestamps, deletion/edit state, client idempotency ID, reply, and forward provenance. |
| `sessions` | Hashed refresh-token sessions with expiry, rotation, and revocation state. |
| `account_tokens` | One-time hashed verification and password-reset tokens. |
| `conversations` | Direct conversations with a canonical user pair and group conversations with metadata/creator. |
| `conversation_members` | Active/left membership, current join window, group role, and adding actor. |
| `conversation_user_state` | Per-member archive, hide, pin, and mute preferences. |
| `message_deliveries` | Per-recipient monotonic sent/delivered/read/failed state. |
| `conversation_read_state` | Per-user last-read message and timestamp. |
| `message_mentions` | Stable user/everyone entities and Unicode rendering ranges for messages. |
| `notifications` | Durable deduplicated message/reply/mention notifications with read state. |
| `notification_preferences` | Per-user message/reply/mention notification switches. |
| `group_invites` | Expiring/revocable group invites with SHA-256 token hashes and optional use limits; raw tokens are never persisted. |

Foreign keys constrain relationships, conversations, and messages. Indexes cover
user discovery, sessions, account-token lookup, friend/block lookup, pending
requests, conversations/members, messages, idempotency, replies, delivery, and
read state. The seed file contains demo data.

User and message deactivation/deletion use `deleted_at` filtering in active
flows. Group membership is PostgreSQL-authoritative; group messages have no
single receiver and create per-recipient delivery rows for active members.

## 7. Authentication

Registration validates required fields, email format, and an eight-character
minimum password, hashes with bcrypt, creates an unverified user, and requests
verification delivery. It does not issue a session or JWT. Login verifies
bcrypt and requires `verified_at`; refresh also checks verification before
rotating the persisted session and issuing new tokens.
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
Email verification and password recovery use `SMTPDelivery` when SMTP is
configured. Runtime keys are `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`,
`SMTP_PASSWORD`, `EMAIL_FROM`, and `APP_URL`; otherwise the development adapter
prints links only when the configured application environment is explicitly
development; non-development environments never select token-logging delivery.
Browser tokens and the stored user remain in localStorage.

## 8. REST APIs

All API routes below are prefixed with `/api/v1`. Protected routes require the
access JWT. Successful nil responses are encoded as `{"message":"ok"}`.

### Public authentication

| Method/path | Body | Success |
| --- | --- | --- |
| `POST /auth/register` | `username`, `email`, `password` | `201`; user and `verification_required: true`; no session is issued. |
| `POST /auth/login` | `email`, `password` | `200`; user, access/refresh tokens and expiries; `403` until verified. |
| `POST /auth/refresh` | `refresh_token` | `200`; new access/refresh tokens and expiries; verification is rechecked. |
| `POST /auth/logout` | `refresh_token` | Revokes the authenticated session. |
| `POST /auth/password-reset/request` | `email` | Generic response; configured SMTP sends a reset link. |
| `POST /auth/password-reset/confirm` | `token`, `password` | Consumes token, changes password, and revokes sessions. |
| `POST /auth/verification/request` | `email` | Generic response; configured SMTP sends a verification link. |
| `POST /auth/verification/confirm` | `token` | Consumes token and sets `users.verified_at`. |

### Protected application routes

| Method/path | Body/query | Behavior |
| --- | --- | --- |
| `GET /users` | `filter`, `limit` (default 20, max 100) | Username/email search; filters shorter than two characters return empty. |
| `GET /users/me` | none | Returns the authenticated user's profile DTO. |
| `GET /users/{userID}` | none | Returns public profile fields (`id`, `username`) for an active user. |
| `PATCH /users/me` | `username`, `email` | Updates profile; an email change clears verification and revokes sessions, then sends verification. |
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
| `GET /conversations` | `limit` (50/100), `offset` (0+), `archived` (optional bool) | Lists non-hidden active or archived conversations for the authenticated member. |
| `GET /conversations/{conversationID}` | none | Gets a conversation only for an active member. |
| `PATCH /conversations/{conversationID}/preferences` | `archived`, `pinned`, `mute_minutes` (at least one) | Updates the acting member's preferences; mute `0` clears mute. |
| `DELETE /conversations/{conversationID}` | none | Hides the conversation only from the acting member; history is retained. Reopening via create/get restores that member's view. |
| `POST /groups` | `name`, optional `description`, `member_ids` | Creates a group with the authenticated creator as owner and eligible friends as members. |
| `GET/PATCH /groups/{conversationID}` | `name`, `description` | Gets or updates group information for active members; owner/admin capabilities are enforced by the backend. |
| `GET/POST /groups/{conversationID}/members` | `member_ids` | Lists active public member summaries or adds eligible members as ordinary members. |
| `DELETE /groups/{conversationID}/members/{userID}` | none | Owner removes members; admin may remove ordinary members only. |
| `POST /groups/{conversationID}/leave` | none | Leaves the group; the final owner must transfer ownership first. |
| `PATCH /groups/{conversationID}/members/{userID}/role` | `role` | Owner promotes/demotes members and admins; final-owner invariant is enforced. |
| `POST/GET /groups/{conversationID}/invites` | expiry/max uses or none | Owner/admin creates an invite (raw token returned once) or lists non-secret invite metadata. |
| `DELETE /groups/{conversationID}/invites/{inviteID}` | none | Owner/admin idempotently revokes an invite. |
| `POST /group-invites/{token}/accept` | none | Hashes the opaque token, validates it transactionally, and joins/reactivates the authenticated user as a member. |
| `GET /conversations/{conversationID}/messages` | `limit` (50/100), `offset` (0+) | Returns authorized direct/group history; group history is bounded by the current membership join time. |
| `POST /conversations/{conversationID}/messages` | `content`, optional client/reply/forward IDs and structured mentions | Persists an authorized conversation message with duplicate-safe client ID handling, mentions, notifications, and per-member group deliveries. |
| `POST /conversations/{conversationID}/messages/{messageID}/forward` | target conversation and optional client ID | Forwards a visible message through the normal idempotent send path. |
| `PATCH /messages/{messageID}` | `{content}` | Sender-only message edit. |
| `DELETE /messages/{messageID}` | none | Sender-only soft delete. |
| `PUT /messages/{messageID}/reactions/{reaction}` | none | Adds the authenticated member's idempotent reaction to a visible message. |
| `DELETE /messages/{messageID}/reactions/{reaction}` | none | Removes the authenticated member's reaction. |
| `POST /messages/delivery` | `{message_id}`, `status` (`delivered`/`read`) | Recipient-only monotonic delivery update within the current membership visibility window. |
| `POST /conversations/{conversationID}/read` | `{message_id}` | Marks the recipient's conversation messages through a message as read; read cursor cannot move backwards. |
| `GET /unread` | none | Returns unread counts keyed by conversation ID. |
| `POST /unread/read-all` | none | Marks visible incoming messages read in active (non-archived, non-hidden) conversations and returns receipt targets. |
| `GET /notifications` | `limit`, `offset` | Lists the authenticated user's newest notifications. |
| `GET /notifications/unread` | none | Returns the unread notification count. |
| `POST /notifications/{notificationID}/read` | none | Marks one owned notification read. |
| `POST /notifications/read-all` | none | Marks all owned notifications read. |
| `GET/PATCH /notification-preferences` | message/reply/mention booleans | Reads or updates the authenticated user's notification preferences. |
| `GET /ws` | authenticated upgrade | Opens a WebSocket connection. |

Message history includes persisted per-recipient status, forward provenance,
reaction aggregates, and structured mention entities. WebSocket delivery
status means the recipient client received the event and sent a server-authorized
receipt; “sent” means persisted. Conversation access and message history/send operations check authenticated
membership and direct friendship/block rules or active group membership. Message edit/delete
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

For `message`, the hub derives the actor from the authenticated socket and uses
the persisted conversation to resolve authorized recipients. A client-supplied
direct `receiver_id` cannot redirect content. Group recipients come from active
database membership. The service persists first, including per-recipient delivery
rows, then the hub routes canonical payloads and acknowledges server/client IDs
and status. Client IDs make retries duplicate-safe. Correlated persistence or
validation failures include client/conversation IDs where available. Direct
messages require conversation membership, friendship, and no block.

The server supports `message.edited`, `message.deleted`, `message.replied`,
`message.delivered`, and `message.read` mutation/status events. These are
authorized and persisted through `MessageService` before being routed and
acknowledged. `typing` is authorized, timestamped, and intentionally ephemeral.
Message events include persisted structured mention ranges. The hub also emits
`notification.created` and `notification.read` to all local sessions of the
recipient; clients refresh durable notification state after reconnect.
The hub emits timestamped `user_online` on a user's first connection and
`user_offline` after its last connection closes only to active friends and
active shared-conversation members, excluding blocked or inactive users. Each
connection receives an authoritative `presence.snapshot` for its permitted
audience. This state is single-process and the frontend rejects stale updates.

Connection controls are a 10 KB read limit, 60-second read deadline refreshed
by pong, 10-second write deadline, 30-second ping, 10 inbound messages per
second, and removal of clients whose send queue is full. The frontend retries
up to five times with exponential delays starting at one second.

Group conversation persistence, administration, invite links, messaging, and UI
are implemented. The hub is process-local; Redis is not used for WebSocket
fan-out or presence.

## 10. Currently implemented features

- Registration, login, password hashing, access validation, and refresh.
- Persisted sessions, refresh rotation, logout/revocation, profile updates,
  password changes, soft deactivation, password recovery, and verification
  token consumption.
- Protected REST API and Redis HTTP rate limiting (10/minute public auth,
  120/minute protected user limit).
- User search, friend requests, mutual friendships, friend listing, blocking,
  and unblocking.
- Direct conversation creation/list/get, per-user archive/hide/pin/mute state,
  membership checks, message persistence, and REST history retrieval.
- Group creation, metadata, roles, membership lifecycle, owner/admin invite-link
  management, hashed-token acceptance, and group settings/chat UI.
- Message edit/delete/reply, client-idempotent sends, persisted status in history,
  durable delivery/read state, unread summaries, and mark-all-read for active
  conversations through REST and service/repository paths.
- Authenticated direct WebSocket delivery and mutation/status events, sender
  identity protection, persistence ack/error, scoped single-server presence,
  ping/pong, limits, and multiple connections per user.
- React login/register, recovery/verification, private/public profile, friends/
  search/request, conversation list/preferences, and direct chat screens with
  pagination, optimistic send reconciliation, retry, realtime mutations, status
  updates, read marking, and unread loading.
- Structured member mentions, owner/admin `@everyone`, safe mention highlighting,
  and persisted in-app message/reply/mention notifications with mute/preferences.
- PostgreSQL/Redis health checks and local Docker infrastructure.

## 11. Partial, scaffolded, and missing features

### Partial

- Typing indicators are membership-authorized, support groups and multiple
  users, and expire client-side, but are intentionally not persisted.
- Presence is scoped and reconnect-hydrated, but remains single-server and has
  no durable last-seen value.
- Refresh lifecycle, localStorage token storage, and hard-coded frontend URL
  are usable locally but incomplete for production.
- SMTP must be configured for email delivery outside development; a missing SMTP
  configuration in other environments fails delivery without exposing tokens.

### Scaffolded

- Group chat room types remain an optional in-memory routing aid; persisted
  conversation membership is the only authorization source.
- User role field and TODO marker for admin actions.
- Database `deleted_at` fields and model fields without complete behavior.

### Missing

- External push notifications and durable WebSocket event replay.
- Distributed WebSocket fan-out/presence for multiple backend instances.
- Production application containers and Kubernetes manifests. PostgreSQL
  repository integration tests are available with the `integration` build tag
  and require `TEST_DATABASE_URL`.

## 12. Configuration

`internal/platform/config/config.go` reads `config.yaml` from the working
directory or `./config/`, then applies non-empty overrides for `DB_HOST`,
`DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `REDIS_HOST`, `REDIS_PORT`,
`JWT_SECRET`, `PORT`, `APP_ENV`, and the SMTP/email variables listed above.

Configuration sections are `database`, `jwt`, `server`, `CORS`, `logging`,
`redis`, and `email`. `config/config.example.yaml` documents local defaults. Pool duration
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
- Error responses lack machine-readable codes.
- Browser tokens are stored in localStorage. WebSocket credentials are passed
  in the `token` query string, which can expose access tokens to intermediary
  logs; use a safer upgrade mechanism before production deployment.
- Frontend automated tests are not implemented. PostgreSQL repository integration
  tests are available with the `integration` build tag and require
  `TEST_DATABASE_URL`.

For deployment-specific commands and checklist, see `docs/DEPLOYMENT.md`.
