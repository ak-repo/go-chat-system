# Phase 1 Completion Plan

This document is the implementation plan for the **Phase 1 — MVP / Core Chat**
items in [`chat_application_feature_roadmap.md`](./chat_application_feature_roadmap.md).
It is based on the current source, migrations, and tests in this repository;
the roadmap itself is not treated as proof that a feature exists.

The plan deliberately does not describe Phase 2+ work. The current application
is a single Go server with a React SPA, PostgreSQL persistence, Redis-backed
HTTP rate limiting, and an in-memory WebSocket hub.

## Project overview and technology stack

The project is a backend-first direct-chat application. The Go backend serves
the REST API and authenticated WebSocket endpoint from one process. The React
client uses REST for authentication, discovery, and history, and WebSocket for
live events.

| Layer | Technology currently used |
| --- | --- |
| Backend | Go 1.25.5, Chi |
| Authentication | JWT HS256, bcrypt passwords |
| Persistence | PostgreSQL via pgx/pgxpool |
| Infrastructure | Redis via go-redis; Docker Compose for local services |
| Realtime | Gorilla WebSocket and an in-memory hub |
| Migrations | Goose-formatted SQL |
| Frontend | React, TypeScript, Vite, React Router, Axios |

## Status vocabulary

- **Implemented**: a usable path exists from UI/transport through service,
  repository, and database where persistence is required.
- **Partial**: important code exists, but the end-to-end requirement is not
  complete or has a material correctness gap.
- **Scaffolded**: types or transport primitives exist, but there is no complete
  business/persistence flow.
- **Missing**: no meaningful implementation exists.

## 1. Current repository map and architecture

| Concern | Current location | Role |
| --- | --- | --- |
| Bootstrap and shutdown | `cmd/server/main.go` | Loads config, connects PostgreSQL/Redis, starts Chi, stops the hub gracefully. |
| Configuration | `internal/platform/config/`, `config/config.example.yaml` | YAML loading plus a small set of environment overrides. |
| Domain models | `internal/domain/model/` | User, friend, friend request, and message structures. |
| Persistence | `internal/repository/` | PostgreSQL SQL and scans. |
| Business logic | `internal/service/` | Auth, discovery, friendship, blocks, and message authorization/persistence. |
| HTTP transport | `internal/transport/routes/`, `wrapper/`, `middleware/` | Routes, auth context, response envelope, rate limits, CORS, logging, recovery. |
| WebSocket transport | `internal/transport/websocket/` | Authenticated clients, read/write pumps, hub routing, presence, message persistence and delivery. |
| Frontend API | `web/src/api/` | Axios REST client, token storage/refresh, message/friend/user APIs, WebSocket client. |
| Frontend state/UI | `web/src/context/`, `web/src/pages/` | Auth/socket lifecycle and login, registration, friends, and direct chat screens. |
| Schema | `migrations/20260126104003_initial_schema.sql` | Goose migration for users, friendships, blocks, requests, and messages. |

The dependency direction to preserve is:

```text
transport -> service -> repository -> PostgreSQL
                         \-> Redis/platform infrastructure where required
```

Keep HTTP parsing and WebSocket framing in transport, business rules in
services, and SQL in repositories. The WebSocket hub is transport infrastructure;
it must not become a second business or persistence layer.

### Backend architecture

`cmd/server/main.go` loads configuration and platform connections, then
`internal/transport/routes` creates the router and manually wires repositories
and services through `internal/transport/injector`. Middleware authenticates
requests and places the JWT user ID in context. Services validate and authorize
operations before repositories execute SQL. `wrapper.HTTPResponseWrapper`
normalizes responses and logs request IDs.

### Frontend architecture

`web/src/api/` owns Axios calls, token storage/refresh, REST types, and the
WebSocket client. `AuthContext` owns login/register/logout state;
`SocketContext` owns socket connection state and subscriptions. `App.tsx`
protects `/friends` and `/chat/:userId`; pages compose these APIs and contexts
rather than owning transport implementation.

### Database architecture

The current Goose migration contains `users`, `friends`, `blocks`,
`friend_requests`, and `messages`. It does **not** contain conversations,
sessions, message receipts, read states, or unread counters. PostgreSQL is the
source of truth for users, relationships, and persisted direct messages.

## 2. Phase 1 status at the current baseline

| Roadmap requirement | Status | Evidence and gap |
| --- | --- | --- |
| User registration | **Implemented** | `POST /api/v1/auth/register` validates, bcrypt-hashes, inserts `users`, and returns an access token. Duplicate database errors are not yet mapped to a stable conflict response. |
| Login | **Implemented** | `POST /api/v1/auth/login` verifies bcrypt and returns access plus refresh JWTs. |
| Logout | **Partial** | `web/src/api/auth.ts` only clears browser storage; there is no server logout/session revocation endpoint. |
| Password hashing | **Implemented** | `internal/shared/utils/password.go` uses bcrypt; the database stores `password_hash`. |
| Access/refresh tokens | **Partial** | JWT generation/validation and refresh endpoint exist, but refresh tokens are stateless, not revoked or stored, and registration returns no refresh token. |
| Email/phone verification | **Missing** | No verification token table, delivery provider, endpoint, or UI. |
| Forgot/reset password | **Missing** | No reset token, delivery, endpoint, or UI. |
| User profile | **Partial** | User data is returned by auth/search, but there is no authenticated profile-get/update route. |
| Change password | **Missing** | No route, service method, or repository method. |
| Account deletion | **Missing** | `deleted_at` exists in the model/schema, but no deletion flow uses it. |
| Search users / username | **Implemented** | Protected `GET /users` searches `username` or `email`, requires two filter characters, and caps limit at 100. |
| User profile lookup | **Missing** | `GetByID` is an internal repository helper only; no public lookup endpoint. |
| Contacts/friends | **Partial** | Friend listing and request lifecycle exist, but this is not a conversation/contact model and has no profile/contact management. |
| 1-to-1 conversation creation/get/list | **Missing** | There is no `conversations` table or conversation service. Direct messages use a sender/receiver pair. |
| Send/receive text message | **Partial** | Authenticated WebSocket `message` events are persisted and delivered to connected receivers; delivery is absent while offline and there is no REST send endpoint. |
| Message timestamps | **Implemented** | PostgreSQL `created_at`/`modified_at` are persisted and returned; WebSocket output uses RFC3339Nano. |
| Edit/delete/reply | **Missing** | No service, route/event, repository operation, or client flow. `deleted_at` is only a field. |
| Message pagination | **Partial** | `GET /messages` supports bounded `limit` and `offset`, but ordering is newest-first and there is no cursor or frontend “load older” flow. |
| WebSocket connection/authentication | **Implemented** | Protected `GET /api/v1/ws` validates JWT context before upgrade; origin checking, size limit, ping/pong, deadlines, cleanup, and per-client rate limiting exist. |
| New message events | **Partial** | Durable direct message and sender ack exist. Sender identity is overwritten from the authenticated client; event authorization is still dependent on service friendship/block checks. |
| Edited/deleted events | **Missing** | No event handling. |
| Disconnect handling | **Implemented** | Client unregisters, send channel closes, and presence changes only after a user’s final connection closes. |
| Automatic reconnection | **Partial** | Frontend retries exponentially up to five attempts, but does not refresh an expired socket token or synchronize missed events. |
| Sending/sent state | **Partial** | UI inserts an optimistic message and backend sends `ack` with `sent`; the optimistic ID is not reconciled with the persisted server ID. |
| Delivered/read/failed/retry status | **Partial** | TypeScript types and generic `read`/`ack` shapes exist, but there are no persisted receipts, read-state tables, failure state machine, or retry queue. |
| Unread counts/read marking | **Missing** | No read-state schema/service/API/UI. |

## 3. Target Phase 1 definition

Phase 1 should be considered complete only when all of the following are true:

1. A user can register, log in, refresh an access token, log out, manage a
   profile/password, and delete or deactivate the account safely.
2. Verification and password recovery have an explicit, testable token flow
   (an email provider may initially be replaced by a development delivery
   adapter, but tokens must not be returned in production API responses).
3. A user can search by username, inspect a safe public profile, manage direct
   contacts, create/open/list one-to-one conversations, and enforce membership.
4. A user can send, receive, paginate, edit, delete, and reply to text messages.
5. WebSocket authentication comes from the server’s authenticated context;
   client-supplied `sender_id` is never authoritative.
6. Durable messages are persisted before delivery, have stable IDs, and produce
   duplicate-safe acknowledgements/events.
7. Sent, delivered, read, and failed states are represented consistently in
   PostgreSQL, REST responses, WebSocket events, and the React UI.
8. Unread totals and read operations survive reconnects and multiple sessions.
9. Backend, WebSocket, and frontend tests cover the acceptance cases below.

## 4. Step-by-step implementation procedure

### Step 0 — Establish contracts and freeze the current baseline

1. Run `go test ./...` and record the baseline.
2. Run `cd web && npm run lint && npm run build` and record the baseline.
3. Inventory every route in `internal/transport/routes/routes.go` and every
   caller in `web/src/api/`; do not introduce an endpoint that duplicates an
   existing contract.
4. Define the response envelope used by `internal/shared/utils/response.go`
   before adding endpoints. Every new endpoint must specify method, path, auth,
   request, success body/status, errors, authorization, and pagination.
5. Decide whether “account deletion” means immediate hard deletion or a
   reversible soft deletion. The existing `deleted_at` columns suggest soft
   deletion, but the policy must be explicit before implementation.

### Step 1 — Add the missing authentication persistence model

Create a new Goose migration; do not edit the existing initial migration.

1. Add a `sessions` or `refresh_tokens` table containing a UUID/session ID,
   user ID, token hash (never the raw refresh token), expiry, revoked timestamp,
   created timestamp, and optional device metadata.
2. Add indexes for user lookup, token-hash lookup, expiry, and active sessions.
3. Add the constraints needed for user deletion and inactive users. Decide how
   messages, friendships, requests, blocks, and sessions behave on deletion.
4. Add a unique/case-normalization policy for usernames. The current schema
   only makes email unique and does not make username unique.
5. Add repository interfaces and implementations for session creation,
   rotation, revocation, and lookup. Hash refresh tokens before persistence.
6. Update the service layer so login and registration issue the same token shape,
   refresh rotates/revokes the old session, and logout revokes the current
   session. Keep the raw token only at the client boundary.
7. Update `web/src/api/auth.ts`, `client.ts`, and `AuthContext.tsx` together.
   Registration must store the refresh token if the backend returns one; logout
   must close the socket before clearing authentication state.

### Step 2 — Complete user account and profile flows

1. Add protected `GET /users/me` and `PATCH /users/me` (or an equivalent
   explicit profile route). Allow only approved public fields to change.
2. Add `POST /users/me/change-password`; require the current password, validate
   the new password, hash it with bcrypt, and revoke all active sessions after
   success.
3. Add account deletion/deactivation with authorization from context, not a body
   user ID. Revoke sessions, prevent login/search/message access for deleted
   users, and define whether dependent rows are anonymized or cascaded.
4. Add verification and reset-token tables with one-time use, expiry, hashed
   token storage, rate limits, and generic responses that do not reveal whether
   an email exists.
5. Add delivery interfaces under a platform/service boundary. A development
   adapter can log a link, but production must use a configured provider and
   must not expose secrets or reset tokens in normal API responses.
6. Add matching TypeScript API functions and profile/password/recovery pages.

### Step 3 — Make discovery and contacts safe and complete

1. Change user search to return only the intended public DTO. The current search
   includes email, so decide whether email discovery is allowed and document it.
2. Add an authenticated profile lookup endpoint, excluding `password_hash` and
   other private fields.
3. Review friend request authorization and response field naming. The frontend
   currently expects PascalCase friend-request fields while user/message DTOs
   use snake_case; standardize the contract or add a deliberate compatibility
   adapter.
4. Add pagination limits and deterministic ordering to friend requests and
   friends, and add frontend pagination if the lists can grow.
5. Prevent deleted users from appearing in search, friends, requests, and
   message authorization queries.

### Step 4 — Introduce a real one-to-one conversation model

The current `messages` table is a direct sender/receiver ledger. It is not a
conversation implementation, and the `is_group` flag is not enough to provide
conversation identity or conversation settings.

1. Add `conversations` with ID, kind (`direct`), created/modified timestamps,
   and archive/delete/pin/mute policy fields as appropriate.
2. Add `conversation_members` with conversation ID, user ID, membership state,
   and per-user settings. Enforce one direct conversation per user pair with a
   canonical pair key or equivalent unique constraint.
3. Add a migration/backfill strategy for existing direct messages. Do not claim
   old messages belong to a conversation until the backfill is deterministic.
4. Add repository methods for create-or-get direct conversation, membership
   lookup, list, and settings updates.
5. Add service authorization that checks the authenticated user is a member for
   every conversation read/write operation.
6. Add routes such as `POST /conversations`, `GET /conversations`, and
   `GET /conversations/{id}` only after the request/response contract is fixed.
   The client must never choose the authenticated owner from a request body.
7. Update message storage to use `conversation_id`; retain sender ID from the
   authenticated context and validate the target as a conversation member.

### Step 5 — Complete durable text messaging

1. Extend the message schema with conversation ID, optional `edited_at`,
   deletion metadata, reply-to message ID, and a client idempotency key if the
   product requires retry-safe sends.
2. Add foreign keys and indexes for `(conversation_id, created_at, id)` and
   reply/deletion relationships. Use a deterministic ordering key; timestamps
   alone are not sufficient for ties.
3. Add service methods for create, edit, delete, reply, and history retrieval.
   Validate body length and content, verify membership, and enforce which
   sender can edit/delete and for how long.
4. Add REST history with a documented cursor or, during migration, strict
   offset semantics. Return a stable chronological contract. The current SQL
   returns newest-first and the React page reverses it locally.
5. Add REST endpoints for message mutation only if they are part of the chosen
   client contract; otherwise keep mutation events on WebSocket but reuse the
   same service methods.
6. Update `internal/transport/websocket/hub.go` so the sequence for a durable
   send is: authenticate actor, validate payload, authorize membership, persist,
   then deliver and acknowledge.
7. Replace flexible `data` handling with versioned, typed payloads for message,
   edit, delete, and reply events. Preserve additive compatibility for existing
   clients while migrating `text`/`content`.
8. Reconcile optimistic frontend messages using the server message ID and
   client idempotency key. Do not leave a temporary UUID and persisted UUID as
   two displayed messages.

### Step 6 — Implement message status and unread/read state

Add another migration (or a coherent migration in the same unreleased change)
for:

1. `message_deliveries` or an equivalent per-message/per-recipient table with
   sent, delivered, read timestamps and failure metadata.
2. `conversation_read_states` with user, conversation, last-read message/event,
   and updated timestamp. Add a unique `(user_id, conversation_id)` constraint.
3. Repository operations that are idempotent: repeating a delivered/read event
   cannot move a state backward or create duplicates.
4. Service methods for mark-one-message-read, mark-conversation-read, and
   mark-all-read, each authorized against the authenticated user.
5. REST endpoints for unread summaries and read mutations.
6. WebSocket events with an explicit contract, for example:

   | Event | Direction | Durable? | Required behavior |
   | --- | --- | --- | --- |
   | `message` | client → server, server → recipient | yes | Persist first; sender comes from socket context. |
   | `message.created` | server → members | yes/event replayable | Include server message ID and conversation ID. |
   | `message.edited` | client → server, server → members | yes | Authorize editor and persist before broadcast. |
   | `message.deleted` | client → server, server → members | yes | Persist deletion and broadcast the stable ID. |
   | `message.delivered` | client → server, server → sender | yes | Idempotently advance delivery state. |
   | `message.read` | client → server, server → sender/members | yes | Idempotently advance read state and unread totals. |
   | `typing.started/stopped` | client → recipient | no | Validate membership; do not persist. |
   | `ack` / `error` | server → sender | no | Include client/server IDs and stable error codes. |

7. Add frontend pending/sent/delivered/read/failed rendering, retry action,
   unread badges, and read-on-open/read-on-scroll behavior.

### Step 7 — Finish WebSocket lifecycle and reconnection

1. Keep `AuthMiddleware` before the upgrade and continue injecting the user ID
   into context. Do not accept the incoming `sender_id` as identity; the current
   `Client.ReadPump` overwrite is the required invariant.
2. Decide whether query-string JWTs are acceptable for production. Prefer a
   short-lived authenticated upgrade mechanism or a secure cookie where logs
   and intermediaries cannot capture the token.
3. Add an explicit protocol error for unknown events, malformed receiver IDs,
   unauthorized recipients, and oversized payloads.
4. Ensure all hub send paths handle slow clients without double-closing a send
   channel. Add tests for multi-connection users, unregister races, and
   backpressure.
5. On reconnect, refresh an expired access token, reconnect with bounded
   backoff, fetch missed history/events, and deduplicate using server message
   IDs or event IDs. A successful TCP/WebSocket reconnect alone is not message
   synchronization.
6. Keep the in-memory hub for a single process. Before deploying multiple
   backend instances, add an explicit Redis Pub/Sub or equivalent fan-out plan;
   the current hub cannot deliver between processes.

### Step 8 — Integrate the React client by contract

1. Centralize REST types and WebSocket payload types under `web/src/api/`.
2. Move the hard-coded `BASE_URL` in `web/src/api/client.ts` to Vite build-time
   configuration before production use; derive the WebSocket URL from the same
   origin/configuration.
3. Keep connection lifecycle in `SocketContext` and protocol behavior in
   `api/websocket.ts`; pages should only subscribe and render state.
4. Add conversation list, conversation creation/opening, message pagination,
   status, unread, edit/delete/reply, profile, password, and account screens.
5. Ensure logout calls `disconnect()` and clears both stored user data and all
   tokens. Ensure token refresh updates the socket authentication path.
6. Test the UI with a fresh browser, expired access token, two tabs, offline
   transition, reconnect, duplicate event, empty history, and failed send.

## 5. REST contract checklist for new work

For each new endpoint, document in code review and API documentation:

```text
Method and path:
Authentication:
Authenticated actor source:
Path/query parameters:
Request body:
Success status and response envelope:
Error codes/statuses:
Authorization rule:
Pagination/order:
Idempotency/duplicate behavior:
Frontend caller:
```

The current API base is `/api/v1`. Protected routes use `Authorization:
Bearer <token>`; the middleware also accepts a `token` query parameter for the
WebSocket route and an `access` cookie fallback.

## 6. Database and migration procedure

1. Inspect the current migration and live database before changing schema.
2. Create a new timestamped Goose migration; never rewrite
   `20260126104003_initial_schema.sql` once deployed.
3. Write both `-- +goose Up` and `-- +goose Down`, unless rollback is
   intentionally impossible and that reason is recorded.
4. Add foreign keys, uniqueness constraints, check constraints, and indexes as
   part of the feature, not as later cleanup.
5. For existing messages/users, write and test a backfill before making new
   columns non-null.
6. Test migration up, application behavior, migration down in a disposable
   database, and migration up again.
7. Update repository scans and domain DTOs together; database `body` currently
   maps to JSON `content`, so contract changes must update both Go and
   TypeScript callers.

## 7. Verification and acceptance tests

### Backend

- `go fmt ./...`
- `go vet ./...`
- `go test ./...`
- Repository integration tests against PostgreSQL for constraints, pagination,
  soft deletion, conversation membership, statuses, and unread counts.
- Service tests for unauthorized actor, non-member, blocked user, duplicate
  send, edit/delete policy, and idempotent read/delivery operations.
- HTTP tests for every status and response envelope, including expired and
  revoked tokens.

### WebSocket

- Upgrade requires a valid access token and allowed origin.
- Client-supplied `sender_id` is ignored/overwritten.
- Message is not delivered when persistence fails.
- Message is not accepted for a non-member or blocked relationship.
- Sender receives an acknowledgement containing stable IDs.
- Receiver receives a persisted message on every connected session.
- Disconnect cleanup works for one and multiple connections per user.
- Unknown/malformed events and slow clients are handled safely.
- Reconnect plus history synchronization does not duplicate messages.

### Frontend

- `cd web && npm run lint && npm run build`
- Registration and login store the complete token response.
- Refresh retries one request and updates queued requests safely.
- Logout disconnects the socket and removes local credentials.
- Optimistic sends reconcile with server IDs; failures expose retry.
- History pagination, unread counts, read state, edit/delete/reply, and status
  events render from typed API/context state rather than page-local protocol
  parsing.

## 8. Runtime and deployment implications

`cmd/server/main.go` starts one process serving both REST and WebSocket traffic.
`docker-compose.yml` currently starts PostgreSQL on host port `5433` and Redis
on host port `6380`; it does not run the Go server or frontend. The frontend is
a Vite static build under `web/dist/`. Production must provide a real
`JWT_SECRET`, database/Redis credentials, configured CORS/WebSocket origins,
TLS at a reverse proxy, and migrations before startup.

The current configuration loader reads YAML from `config/config.yaml` and
supports only the environment overrides implemented in
`internal/platform/config/config.go`. Do not assume every YAML setting has an
environment equivalent. The current `config/config.yaml` is local runtime
configuration and must not be copied into documentation or committed with
secrets.

## 9. Current limitations to resolve before calling Phase 1 complete

- No conversation or conversation-member tables.
- No durable session/revocation model for refresh tokens or logout.
- No email/phone verification or password recovery.
- No public profile update/lookup, password change, or deletion flow.
- No message edit, delete, reply, durable status, or unread/read model.
- WebSocket `typing` and `read` payloads are accepted by generic routing but
  are not backed by the complete Phase 1 state model.
- Offline recipients do not receive queued messages/events from the in-memory
  hub; reconnection does not replay missed events.
- Frontend optimistic messages are not reconciled with server IDs.
- Frontend has a five-attempt reconnect strategy but no token-refresh and
  missed-history synchronization protocol.
- The initial message query is offset-based and newest-first; it is not a
  scalable cursor history contract.
- Group receiver types and `Room` exist as scaffolding, not as Phase 1 group
  functionality.
- Redis is used for HTTP rate limiting and health checks, not for WebSocket
  fan-out, durable event processing, or presence storage.
- The frontend REST base URL is hard-coded to localhost.

## 10. Recommended delivery order

```text
baseline/contracts
  -> sessions + auth completion
  -> profile/password/deletion + verification/recovery
  -> discovery/contact cleanup
  -> conversations + migration/backfill
  -> durable message mutations + pagination
  -> delivery/read/unread state
  -> WebSocket protocol + reconnect synchronization
  -> React integration
  -> integration/security/load verification
```

Do not mark a roadmap checkbox from the existence of a model, TypeScript type,
route, or WebSocket event alone. Mark it only after the complete path and the
corresponding acceptance tests pass.
