# Phase 1 — MVP / Core Chat Implementation Guide

This document turns the Phase 1 checklist in
[`../chat_application_feature_roadmap.md`](../chat_application_feature_roadmap.md)
into an implementation and verification guide for this repository. The current
source-of-truth inventory is [`../CODEBASE.md`](../CODEBASE.md); this guide
records the intended implementation areas and the Phase 1 code changes applied.

## Implementation status

The Phase 1 gap work in this guide has been applied. Verification is required
before login; SMTP is the production delivery adapter and must be configured
through environment/configuration. The development adapter prints usable links
only when `app.environment: development` or `APP_ENV=development` is explicit.
Conversation delete is member-scoped hiding, and mark-all-
read applies to active non-archived conversations only. The new migration,
backend/unit/integration checks, frontend lint, and frontend build have been
verified; actual delivery through an external SMTP service depends on runtime
credentials and was not part of local automated verification.

## Scope and completion rule

Phase 1 is complete when the listed capabilities have an authorized backend
contract, persistence behavior where appropriate, usable frontend integration,
and automated coverage for their key success and failure paths. A backend route
or database column by itself does not make a user-facing feature complete.

The implementation must preserve the project layering:

```text
HTTP/WebSocket transport -> service -> repository -> PostgreSQL/Redis
```

Do not modify the already-existing canonical schema migration. Database changes
belong in a new Goose migration with safe `Up` and `Down` behavior. Do not include
credentials or provider secrets in the repository.

### Existing Phase 1 foundation

The following capabilities already have substantial implementation:

- Registration, login, password hashing, access/refresh tokens, session
  rotation/revocation, logout, profile update, password change, and account
  deactivation.
- Password reset and verification token workflows, now enforced before login
  and backed by configurable SMTP delivery; unconfigured development mode uses
  the in-memory adapter.
- Authenticated username/email search, friend discovery and requests, direct
  conversation creation/get/list, and membership/access checks.
- Text message persistence and history pagination, timestamps, sender-only
  edit/delete, reply persistence, durable delivery/read state, unread summary,
  and idempotent client-message IDs.
- Authenticated WebSocket messaging, server-derived sender identity, persistence
  acknowledgements/errors, presence broadcasts, connection lifecycle controls,
  and bounded frontend reconnect attempts.
- Frontend auth/profile/friend/chat screens, optimistic sends, sent/sending/
  failed display, retry, message history pagination, read marking, and typing UI.

Treat the above as a baseline to preserve and complete, not a reason to duplicate
the existing contracts.

## Work areas

### 1. Authentication, account lifecycle, and user discovery

**Current code to retain and review**

- `internal/service/users_service.go`
- `internal/service/account_token_service.go`
- `internal/repository/user_repo.go`
- `internal/shared/utils/password.go`
- `internal/shared/jwt/jwt.go`
- `internal/transport/middleware/authmiddleware.go`
- `internal/transport/routes/routes.go`
- `web/src/api/auth.ts`, `web/src/api/users.ts`, `web/src/context/AuthContext.tsx`
- `web/src/pages/LoginPage.tsx`, `RegisterPage.tsx`, `RecoveryPage.tsx`,
  `VerificationPage.tsx`, and `ProfilePage.tsx`

**Changes needed**

1. Preserve existing registration/login validation and bcrypt hashing. Verify
   that all response DTOs exclude password hashes and that inactive accounts
   cannot authenticate or refresh sessions.
2. Make verification/recovery delivery configurable behind the existing
   `Delivery` interface. Keep `DevelopmentDelivery` for local/test use and add a
   production provider adapter/configuration without storing secrets in source.
   Delivery failures must be observable without changing the generic public
   response in a way that reveals whether an account exists.
3. Confirm and document the product policy for verification. If verification is
   an access requirement, enforce it consistently in login/refresh and relevant
   protected operations; otherwise label it explicitly as optional/informational
   and do not count a delivery token flow as enforced verification.
4. Confirm account deletion semantics. The existing operation is soft
   deactivation and revokes sessions; ensure search, authentication, friends,
   conversations, and message access consistently filter deactivated users.
5. Add a protected public-profile lookup contract if users need to open a profile
   by ID. `GET /users/me` only describes the current user; discovery results
   currently expose a deliberately small `PublicUser` representation. Define
   which fields may be returned and apply the same privacy and inactive-user
   policy to search and lookup.
6. Improve frontend auth lifecycle as required for correct operation: restore a
   valid session after reload, handle refresh failure by clearing stale auth
   state, and show useful validation/error states for verification and recovery.

**Acceptance criteria**

- Registration, login, logout, refresh rotation, password change/reset, profile
  update, and account deactivation work through the UI/API with authenticated
  identity derived from server context.
- Verification behavior and delivery mode are explicit and match actual policy.
- Search and profile lookup never expose private account fields or inactive
  accounts.

### 2. Direct conversations and per-user conversation actions

**Current code to retain and review**

- `internal/domain/model/message.go` (`Conversation` model)
- `internal/repository/conversation_repo.go`
- `internal/service/conversation_service.go`
- `internal/transport/routes/routes.go`
- `web/src/api/conversations.ts`
- `web/src/pages/ChatPage.tsx` and `FriendsPage.tsx`
- `migrations/20260829120500_canonical_schema.sql` as a read-only reference

**Changes needed**

1. Keep direct conversation create-or-get idempotent, membership-protected, and
   subject to the existing friendship/block policy. Ensure listing and retrieval
   do not leak conversations to non-members.
2. Implement archive, pin, and mute as **per-user membership preferences**, not
   conversation-wide flags: one person's action must not alter the other
   participant's inbox. Store suitable state on a membership/preference record
   (for example, archive timestamp, pin ordering/timestamp, and mute-until),
   created by a new migration and exposed in conversation list DTOs.
3. Define “delete conversation” as a per-user action (hide/remove from that
   participant's active list) unless product requirements explicitly call for
   destructive deletion. Do not cascade-delete the other participant's history.
   Implement authorization, idempotency, and a clear restore/reopen policy.
4. Add repository methods for state mutation and list filtering/order; add
   service methods for validation and active-member authorization; register
   protected REST handlers. Keep SQL exclusively in the repository.
5. Add frontend API methods and conversation-list controls. Surface archived
   conversations in an archive view or define that archived conversations are
   accessible only by search/direct navigation. Display pin and mute state and
   make actions reversible where relevant.
6. Decide how a newly sent/received message affects archived state and ordering.
   Document the rule and test it rather than silently unarchiving or reordering
   in only one client.

**Suggested API shape**

Use the existing `/api/v1/conversations` resource, for example member-scoped
action routes or a validated preferences patch. The exact route and request
shape should be selected consistently with existing API conventions. Every
action must derive the acting user from authenticated context; never accept a
user ID as authority from the request body.

**Acceptance criteria**

- Create/get/list is safe for both participants and rejects unrelated users.
- Archive, pin, mute, and delete affect only the authenticated participant's
  view and survive reloads.
- Conversation list order, hidden/archive behavior, and message-arrival behavior
  are documented and covered by service/repository tests.

### 3. Message contracts, history, and mutations

**Current code to retain and review**

- `internal/domain/model/message.go`
- `internal/repository/message_repo.go`
- `internal/service/message_service.go`
- `internal/transport/routes/routes.go`
- `web/src/api/messages.ts`
- `web/src/pages/ChatPage.tsx`

**Changes needed**

1. Preserve membership, friendship, and block checks for REST and WebSocket
   sends. Sender identity must always come from authenticated context/socket
   identity, never the payload.
2. Keep timestamps server-generated and message history access scoped to the
   requested conversation. Continue validating pagination bounds and stable
   chronological ordering. The current API uses offset pagination; if retaining
   it, ensure the UI loads older pages without duplicates when live messages
   arrive during pagination.
3. Finish edit/delete/reply user workflows in the chat UI. Provide controls only
   on messages the current user may mutate, validate reply targets in the same
   conversation, and render edited/deleted/reply metadata consistently.
4. Ensure soft-deleted messages are consistently omitted or represented as a
   tombstone according to one documented policy. Current repository reads filter
   deleted messages, while deletion semantics are described as incomplete in
   `CODEBASE.md`.
5. Reconcile REST and WebSocket event payloads. Mutation events must identify the
   canonical persisted message and conversation; client-supplied sender identity
   or arbitrary recipient must not override authorization decisions.

**Acceptance criteria**

- Send, receive, history pagination, edit, delete, and reply work via the agreed
  UI/API path; unauthorized mutations are rejected.
- Message timestamps and ordering are stable across reload and realtime arrival.
- Deleted messages and reply references follow the documented policy.

### 4. WebSocket authentication, realtime events, and reconnection

**Current code to retain and review**

- `internal/transport/websocket/client.go`, `hub.go`, `ws_message.go`, `room.go`
- `internal/transport/wrapper/ws_handler.go`
- `internal/transport/middleware/authmiddleware.go`
- `web/src/api/websocket.ts`
- `web/src/context/SocketContext.tsx`

**Changes needed**

1. Preserve authenticated upgrade/session checks, allowed-origin validation,
   server-owned sender identity, read/write deadlines, ping/pong, message-size
   limits, and inbound rate limiting.
2. Preserve persist-before-deliver behavior and idempotent client message IDs.
   A positive “sent” acknowledgement means accepted/persisted; do not describe it
   as recipient delivery.
3. Complete frontend subscriptions and state reconciliation for `message.edited`,
   `message.deleted`, `message.replied`, `message.delivered`, and `message.read`.
   Update only the matching conversation/message and prevent duplicate rendering
   when the sender receives its own routed event and acknowledgement.
4. Clarify “delivered”: currently the backend persists delivery state and exposes
   status updates, but the implementation should define whether delivery means
   enqueued to an active socket, received by the client, or explicitly confirmed
   by the client. If client confirmation is required, send and authorize a
   receipt only after receiving the message. Disconnected recipients must not be
   reported delivered merely because the message was persisted.
5. Preserve error-to-failed state mapping and retry with the same
   `client_message_id`. Do not create a second persisted message on retry.
6. Preserve bounded automatic reconnect behavior and reconnect only while the
   user remains authenticated. On reconnect, reload unread/history state so the
   UI does not imply that missed events were replayed; durable offline sync is a
   later roadmap phase.
7. Review the browser token-in-query-string limitation documented in
   `CODEBASE.md`; select a safer upgrade authentication mechanism before a
   production deployment requirement is claimed.

**Acceptance criteria**

- Unauthenticated/expired sessions cannot keep an active socket; client
  `sender_id` spoofing has no effect.
- Message and mutation events reach authorized active participants and update
  the UI exactly once.
- Reconnect attempts are bounded, stop after logout, refresh credentials as
  needed, and visibly reflect connection state.

### 5. Message status and unread/read workflows

**Current code to retain and review**

- `message_deliveries` and `conversation_read_state` schema in the canonical
  migration (do not edit that migration)
- `internal/repository/message_repo.go`
- `internal/service/message_service.go`
- `internal/transport/routes/routes.go`
- `web/src/api/messages.ts`, `web/src/api/websocket.ts`, and `ChatPage.tsx`

**Changes needed**

1. Define status transitions as monotonic: `sending` is client-local; persisted
   server state is `sent`, then `delivered`, then `read`; `failed` must have a
   defined source and must not overwrite a later successful state. Keep duplicate
   and out-of-order receipts harmless.
2. Complete delivery/read event handling in the frontend and display the status
   on the corresponding outgoing message. A generic ack/error must be correlated
   by client ID and then server ID.
3. Preserve recipient-only authorization for delivery and read updates. Validate
   that the referenced message belongs to the specified conversation and was
   sent to the acting user.
4. Define mark-conversation-read semantics precisely. The current
   `POST /conversations/{conversationID}/read` accepts a message ID and marks
   incoming messages through that point read. Keep it idempotent and ensure the
   stored read cursor cannot move backwards.
5. Add mark-all-as-read behavior if retaining that roadmap item. Specify whether
   it affects all active conversations or also archived conversations, add a
   protected endpoint/service/repository operation, and perform the updates in a
   transaction or other bounded atomic strategy. It should advance each
   conversation only to its latest eligible incoming message.
6. Load unread summaries in the relevant frontend entry/list view and reconcile
   counts when messages arrive or read operations succeed. Render zero-count
   conversations consistently.

**Acceptance criteria**

- Sending shows `sending`, persisted acknowledgement shows `sent`, recipient
  confirmation shows `delivered`, read receipt shows `read`, and a genuine send
  failure shows a retryable `failed` state.
- Unread counts exclude messages at/before the read cursor, update after reads,
  and are scoped to the authenticated user.
- Single-conversation, single-message-through-cursor, and all-conversation read
  actions have defined and tested authorization and cursor behavior.

### 6. Frontend completion and consistency

**Primary code areas**

- `web/src/api/` for REST and WebSocket contracts
- `web/src/context/AuthContext.tsx` and `SocketContext.tsx` for auth/socket state
- `web/src/pages/` for usable user flows

**Changes needed**

- Keep transport details out of UI components; API modules own endpoint/event
  contracts.
- Define shared frontend DTO/status types to match server payloads and remove
  assumptions that are not guaranteed by the API.
- Add loading, empty, error, permission, and reconnect states to new controls.
- Ensure current conversation state is cleared on route changes and stale
  requests/events cannot update another conversation's screen.
- Verify keyboard and disabled states for sending, retry, mutation, and
  conversation actions.
- Add frontend automated coverage for the key state transformations and API
  contract handling if the project adopts a test runner; the current codebase
  guide reports no automated frontend tests.

### 7. Persistence and migration requirements

Only conversation user preferences and any newly justified account behavior are
expected schema additions from the current gap review. Before implementation:

1. Add a new timestamped Goose migration; do not alter
   `20260829120500_canonical_schema.sql`.
2. Use foreign keys and constraints consistent with existing ownership/member
   relationships. Add indexes based on the actual list filtering and ordering
   queries.
3. Make `Down` remove only the new structures/columns and document any
   intentionally irreversible behavior. Avoid destructive removal of user data.
4. Extend repository integration tests for migration-backed SQL behavior. These
   tests require PostgreSQL and `TEST_DATABASE_URL` with the repository's
   integration build tag.

Do not add schema fields for UI-only states such as `sending`; those remain
client-local. Existing message delivery/read tables already cover persistent
statuses and should be extended only if a verified contract gap requires it.

## Recommended implementation order

1. **Contract decisions:** verification policy/delivery provider, account
   deletion semantics, profile lookup privacy, delivered semantics, conversation
   delete/archive behavior, and mark-all-read scope.
2. **Plan and schema:** record API/data/event impact in a plan under `plans/`;
   create and review any additive Goose migration.
3. **Backend persistence and rules:** repository interfaces/SQL, service
   authorization/business rules, then thin REST handlers and any WebSocket
   contract adjustments.
4. **Backend tests:** service tests for authorization/transitions and PostgreSQL
   integration tests for persistence, transactions, list filtering, and cursor
   behavior.
5. **Frontend integration:** API modules/types, auth/socket state, chat and
   conversation controls, then loading/error/status/unread behavior.
6. **Documentation and review:** update the roadmap checkboxes and
   `docs/CODEBASE.md` only after verified implementation; review route/security,
   migration rollback, event duplication, and stale UI state.

## Verification checklist

Run the checks required by `AGENTS.md`:

```bash
go fmt ./...
go vet ./...
go test ./...
cd web && npm run lint && npm run build
```

When repository SQL or migrations change, also run the relevant PostgreSQL
integration tests using `TEST_DATABASE_URL` and the `integration` build tag.
Manually exercise registration/login/recovery/verification policy, profile and
account actions, search/profile lookup, two-user conversation preferences,
message send/mutation/retry, reconnect, receipts, and unread/read flows against a
running local stack.

## Phase 1 completion checklist

Use the roadmap entries as the checklist and mark an item complete only after
the acceptance criteria above are met:

- [x] Authentication, verification/recovery policy, profile, password change,
  and account deletion are complete end to end.
- [x] User search, username search, profile lookup, and contacts/friends are
  complete and privacy-safe.
- [x] Direct conversations support create/get/list/archive/delete/pin/mute with
  per-user authorization and persisted preferences.
- [x] Text send/receive, edit/delete/reply, timestamps, and pagination are
  complete in backend and frontend.
- [x] Authenticated WebSocket messaging, mutation events, connection handling,
  and reconnect are integrated and verified.
- [x] Sending/sent/delivered/read/failed states and retry have clear, consistent
  semantics.
- [x] Unread counts, mark conversation/message/all read are implemented and
  reflected in the UI.
- [x] Go checks, frontend lint/build, and relevant database integration tests
  pass.
