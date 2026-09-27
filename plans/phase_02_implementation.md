# Phase 2 - Full Chat Experience Implementation Plan

This document turns Phase 2 of
[`../docs/chat_application_feature_roadmap.md`](../docs/chat_application_feature_roadmap.md)
into an implementation and verification plan for this repository. The canonical
implementation inventory remains [`../docs/CODEBASE.md`](../docs/CODEBASE.md).

## Status

Planned. No Phase 2 feature is complete solely because a database field, event
name, or UI scaffold already exists. Completion requires an authorized backend
contract, persistence where appropriate, frontend integration, and verification
of important success and failure paths.

Some Phase 2 capabilities are partially available from Phase 1:

- Reply persistence, REST/WebSocket delivery, and basic reply UI exist.
- Direct-chat typing uses an ephemeral boolean WebSocket event.
- The WebSocket hub emits process-local online/offline events.
- Per-conversation mute state is persisted, but no notification system consumes
  it.
- Group-related message fields and an in-memory room type are scaffolding only;
  they are not a secure or persisted group-chat implementation.

The roadmap must be updated as each coherent increment is completed so it can
distinguish implemented, partial, and deferred behavior.

## Scope and completion rule

Phase 2 includes:

- Persisted group conversations, membership, roles, administration, text
  messaging, delivery/read state, and authorized realtime fan-out.
- Live online/offline presence for authorized users in a single server process.
- Direct and group typing indicators, including multiple users typing.
- Completion of reply UX, copy, forwarding, message context actions, and emoji
  reactions.
- Structured mentions, permission-controlled `@everyone`, and mention
  highlighting.
- Persisted in-app notifications for messages, replies, and mentions, including
  preferences and conversation mute behavior.
- Group invite links after group roles and membership behavior are stable.

The following are explicitly deferred:

- Group avatar upload. Phase 2 uses generated initials/default avatars; file
  upload and object storage belong to Phase 3.
- External push notifications. Device registration, service workers/provider
  adapters, background jobs, retry, and operational monitoring are prerequisites.
- Distributed presence, typing, and WebSocket fan-out. Phase 2 documents
  single-server semantics; Redis Pub/Sub belongs to later reliability and
  infrastructure work.
- Durable replay of missed WebSocket events. Clients must refresh authoritative
  REST state after reconnect until Phase 4 synchronization is implemented.
- Moderator, ban, restricted-posting, report, and other safety workflows assigned
  to Phase 6.

All implementation must preserve the project layering:

```text
HTTP/WebSocket transport -> service -> repository -> PostgreSQL/Redis
```

Authenticated identity is always authoritative. A client-provided sender ID,
member role, notification recipient, or room membership must never grant access.

## Product and authorization decisions

The initial implementation uses these rules unless the product requirements are
changed before the relevant increment starts:

1. Group roles are `owner`, `admin`, and `member`.
2. A new group has exactly one owner: its creator.
3. A group must retain at least one owner. The final owner must transfer
   ownership before leaving or being demoted.
4. Owners can manage all roles, transfer ownership, edit group information,
   manage members, and manage invite links.
5. Admins can edit group information, add members, remove ordinary members, and
   manage invite links. They cannot promote to owner, demote owners, or remove
   owners.
6. Members can view the group, send messages, react, mention members, and leave.
7. `@everyone` is restricted to owners and admins.
8. New members can read messages created at or after their current `joined_at`.
   Leaving and rejoining starts a new visibility window.
9. Removed or departed members immediately lose access to group metadata, member
   lists, new history, mutations, typing, and realtime events.
10. Conversation mute suppresses in-app notification creation/display according
    to preference policy, but does not suppress messages, unread counts, mention
    metadata, or read/delivery state.
11. Mentions and replies do not bypass conversation mute in the initial policy.
12. Copying a message is client-side. Forwarding is a first-class persisted
    message relationship and must not be represented as an unmarked copy.
13. Message deletion remains sender-owned in Phase 2. Group moderation deletion
    is deferred to Phase 6.
14. Presence is visible only to friends and users sharing an active conversation,
    subject to existing block and inactive-account rules. It is never broadcast
    globally.

## Existing foundation to retain

- `conversation_members` and per-member archive, pin, mute, and hide state.
- Authenticated conversation membership checks in the repository/service layers.
- Message persistence, idempotent client message IDs, delivery/read rows, unread
  summaries, pagination, edit/delete/reply, and server timestamps.
- Server-derived WebSocket sender identity, bounded frames and queues,
  authentication, connection lifecycle management, and event acknowledgements.
- Redis startup/configuration and HTTP rate limiting. Redis is not currently a
  WebSocket distribution or presence authority.
- Frontend API isolation under `web/src/api/`, authentication/socket contexts,
  optimistic sending, retries, message status merging, and conversation
  preferences.

Do not treat `ReceiverGroup`, `Message.IsGroup`, or the process-local `Room` map as
authorization. PostgreSQL membership is authoritative.

## Work areas

### 1. Group schema and persistence foundation

**Primary files**

- `migrations/20260829120500_canonical_schema.sql` (read-only reference)
- New timestamped Goose migration(s)
- `internal/domain/model/message.go`
- `internal/repository/conversation_repo.go`
- `internal/repository/message_repo.go`
- `internal/repository/integration_test.go`

**Changes**

1. Permit conversation kinds `direct` and `group` with constraints that require
   canonical user-pair columns only for direct conversations.
2. Add group metadata: bounded name, optional description, and creator ID. Keep
   avatar metadata nullable/reserved; do not add local-disk upload behavior.
3. Extend active membership with role, `added_by`, and modification metadata.
4. Add indexes for active membership, role checks, group listing, and
   conversation history visibility.
5. Make `conversation_id` plus `sender_id` authoritative for messages. Adapt the
   direct-only `receiver_id` constraint so group messages do not invent one user
   recipient.
6. Create one `message_deliveries` row for every active member except the sender.
7. Apply the member's current `joined_at` visibility boundary in group history.
8. Update the integration harness to apply the additive migration set and clean
   all new tables.

Every migration must have production-safe `Up` and `Down` sections. Do not edit
an already-deployed migration.

**Acceptance criteria**

- Direct conversation uniqueness and existing Phase 1 behavior remain intact.
- Group creation and membership changes are transactional.
- Invalid direct/group column combinations and invalid roles are rejected by the
  database as well as the service.
- Group messages produce exactly one recipient delivery row per eligible active
  member.
- A rejoined member cannot read messages from an earlier membership window.

### 2. Group service, authorization, and REST API

**Primary files**

- `internal/repository/conversation_repo.go`
- `internal/service/conversation_service.go`
- New group-focused service/repository files if the existing files become
  difficult to navigate
- `internal/transport/routes/routes.go`
- `internal/transport/wrapper/`

**Changes**

1. Add transactional group creation with creator ownership and validated initial
   members.
2. Add get/list operations returning group metadata, member count, the current
   user's role, and server-derived capabilities.
3. Add list/add/remove/leave membership operations and role changes.
4. Enforce the role matrix and last-owner invariant in the service layer and,
   where possible, transactionally in repository operations.
5. Apply friendship/block/inactive-user policy when adding members. Existing
   members must not be duplicated; rejoining resets the membership window.
6. Add group information update and invite-link lifecycle operations.
7. Return stable public member summaries only; do not expose email or private
   profile data.

**Suggested API**

```text
POST   /api/v1/groups
GET    /api/v1/groups/{conversationID}
PATCH  /api/v1/groups/{conversationID}
GET    /api/v1/groups/{conversationID}/members
POST   /api/v1/groups/{conversationID}/members
DELETE /api/v1/groups/{conversationID}/members/{userID}
POST   /api/v1/groups/{conversationID}/leave
PATCH  /api/v1/groups/{conversationID}/members/{userID}/role

POST   /api/v1/groups/{conversationID}/invites
GET    /api/v1/groups/{conversationID}/invites
DELETE /api/v1/groups/{conversationID}/invites/{inviteID}
POST   /api/v1/group-invites/{token}/accept
```

Invite tokens must be opaque, cryptographically random, stored as hashes, have an
expiry, support revocation and optional usage limits, and never be logged.

**Acceptance criteria**

- Non-members cannot read or mutate group state.
- Capabilities are computed by the backend and match enforced authorization.
- Members cannot elevate themselves or alter owners.
- Removing, leaving, and role transitions remain correct under concurrent calls.
- The final owner cannot leave or be demoted without a successful transfer.

### 3. Conversation-centric messaging and realtime delivery (in progress)

**Primary files**

- `internal/service/message_service.go`
- `internal/repository/message_repo.go`
- `internal/transport/websocket/client.go`
- `internal/transport/websocket/hub.go`
- `internal/transport/websocket/ws_message.go`
- `internal/transport/websocket/room.go`

**Changes**

1. Refactor history, send, reply, mutation, read, delivery, and unread operations
   around an authorized conversation rather than a client-selected receiver.
2. Derive all realtime recipients from persisted active membership. Process-local
   rooms may optimize fan-out but cannot grant membership.
3. Recheck membership in the same transaction used to persist a group message
   and its delivery rows.
4. Generalize edit, delete, reply, idempotency, delivered/read receipts, and unread
   summaries without regressing direct conversations.
5. Ensure every persisted mutation event includes canonical `conversation_id`,
   message ID, server timestamp, and sufficient sender data for group rendering.
6. Emit group metadata/member/role events after successful persistence. Clients
   must refresh authoritative state when an event is missed or rejected.
7. Never expose the current `isGroup=true` path if it bypasses group membership
   authorization.

**Acceptance criteria**

- A sender cannot route a message to arbitrary users by changing receiver data.
- Active members receive group messages once per connected session as intended;
  non-members and removed members receive nothing.
- Per-recipient delivery/read state and unread counts are independent.
- Direct and group sends share the same authorization and persistence invariants.
- Message and membership races have deterministic, tested outcomes.

Current backend increment: group text create/history/reply/edit/delete/read and
single-process WebSocket message, mutation, status, typing fan-out, and scoped
presence with reconnect snapshots. Membership metadata events remain deferred.

### 4. Frontend conversation model and chat structure

**Primary files**

- `web/src/App.tsx`
- `web/src/api/conversations.ts`
- `web/src/api/messages.ts`
- `web/src/api/websocket.ts`
- `web/src/context/SocketContext.tsx`
- `web/src/pages/ChatPage.tsx`
- `web/src/pages/ConversationsPage.tsx`

**Changes**

1. Introduce discriminated direct/group conversation response types. Group
   responses include name, member count, current role, and capabilities.
2. Move the primary chat route to `/conversations/:conversationId`. Keep a direct
   conversation entry action that creates/resolves a conversation before routing.
3. Key message, typing, unread, and group-member state by conversation ID.
4. Replace ad hoc WebSocket payload assertions with a typed event-payload map.
5. Keep `SocketContext` transport-focused; apply domain events in dedicated hooks
   or reducers.
6. Extract testable UI boundaries from `ChatPage`: `ChatHeader`, `MessageList`,
   `MessageBubble`, `MessageActionMenu`, `ReplyPreview`, `MessageComposer`, and
   `TypingIndicator`.
7. Distinguish local socket connectivity from another user's presence.
8. Add Vitest and React Testing Library before substantial Phase 2 UI state is
   introduced.

This restructuring must be incremental and preserve working Phase 1 flows; it is
not permission to redesign unrelated screens.

**Acceptance criteria**

- Direct chats still open from discovery, profiles, and conversation lists.
- Direct and group conversations use one conversation-based message flow.
- Domain event reducers are unit tested independently of a live WebSocket.
- No page assumes every conversation has exactly two user IDs.

### 5. Group frontend experience

**Changes**

1. Add a create-group flow with a bounded name and multi-user picker based on the
   existing friend/search behavior.
2. Render group entries in the conversation list using generated initials,
   group name, member count, unread state, and existing per-member preferences.
3. Add a group chat header and settings screen.
4. Add member listing, add/remove/leave actions, and role controls based on
   backend-provided capabilities.
5. Reconcile group metadata and membership events. If the current user is
   removed, close the group view and refresh the conversation list.
6. Add invite creation, revocation, copy, and acceptance after core membership
   operations are stable.

**Acceptance criteria**

- Creator, owner, admin, and member interfaces expose only permitted controls.
- Stale frontend capabilities cannot authorize a forbidden backend operation.
- Group creation through messaging, member management, and leaving work on
  desktop and mobile layouts.
- Default/generated avatars render without requiring object storage.

### 6. Presence and typing

**Presence changes**

1. Scope presence updates to authorized friends/shared-conversation users and
   apply block/inactive-account rules.
2. Preserve multi-socket aggregation: first connection means online, and only the
   final local connection means offline.
3. Add an authoritative presence snapshot API or connection-time event with
   server timestamps so clients can initialize and recover after reconnect.
4. Store frontend presence by user ID and ignore stale out-of-order updates.
5. Render presence in conversation rows, profiles, and direct chat headers.
6. Do not claim cross-instance synchronization until Redis fan-out is added.

Durable `last_seen_at` is included only if a privacy/retention contract is added
before implementation. Otherwise the Phase 2 UI reports live online/offline or
unknown state and the roadmap marks last seen deferred.

**Typing changes**

1. Address typing by `conversation_id`; the server derives active recipients.
2. Normalize started/stopped payloads and include sender and server timestamp.
3. Send start only on an idle-to-typing transition and stop on inactivity,
   submit, blur, navigation, and disconnect where possible.
4. Expire stale remote typing state client-side when stop is lost.
5. Store a set of typing users per conversation and render multiple-user text.
6. Rate-limit typing separately from persisted message traffic.

**Acceptance criteria**

- Presence is not globally disclosed.
- Closing one of several sessions does not incorrectly report the user offline.
- Typing state cannot be sent to a conversation the actor cannot access.
- Lost stop events clear automatically and route changes do not leak typing UI.

### 7. Message interactions

**Reply completion**

- Use normal optimistic-send/client-ID reconciliation for replies.
- Return/render a reply preview with sender and content, including an unavailable
  or deleted-target fallback.
- Support retry and optional scroll-to-original behavior.

**Copy and context actions**

- Add an accessible action menu for reply, copy, edit, delete, forward, and
  react, filtered by backend/domain capabilities.
- Support keyboard focus and mobile interaction. Copy uses the browser clipboard
  and requires no backend endpoint.

**Forwarding**

- Add persisted forward provenance such as `forwarded_from_message_id`.
- Verify source visibility and target conversation membership in the service.
- Forward through the normal idempotent message creation path.
- Define a safe fallback when source content is later deleted or inaccessible.

**Reactions**

- Add `message_reactions` keyed by message, user, and bounded reaction value.
- Require active conversation membership and a visible, non-deleted message.
- Return reaction aggregates and whether the authenticated user reacted.
- Add idempotent add/remove REST operations and canonical
  `message.reaction.added` / `message.reaction.removed` events.
- Start with a fixed emoji palette; a full emoji-picker dependency is optional.

**Acceptance criteria**

- Optimistic reply/reaction state reconciles or rolls back deterministically.
- Duplicate reaction requests remain idempotent.
- Forwarding never grants access to source content the actor cannot read.
- Context actions are usable on keyboard, pointer, and mobile-width interfaces.

### 8. Mentions

**Changes**

1. Add structured `message_mentions` records with stable user IDs, mention type,
   and text ranges or equivalent rendering metadata.
2. Accept structured mention input with message sends; never use a mutable
   username as authorization identity.
3. Validate that mentioned users are active members of the same conversation and
   cap mention count per message.
4. Treat `@everyone` as a distinct mention type guarded by group capability.
   Plain text must not bypass that permission check.
5. Add member suggestions in the composer and safely highlight server-returned
   mention entities.
6. Preserve mention metadata independently of later username changes.

**Acceptance criteria**

- Non-members cannot be mentioned or receive group mention notifications.
- Ordinary members cannot create a functional `@everyone` mention.
- Mention rendering is XSS-safe and remains correct after username changes.
- Mention persistence and message creation are transactional.

### 9. In-app notifications and preferences

**Persistence and backend changes**

1. Add a `notifications` table with recipient, actor, type, conversation/message
   references, payload, creation/read timestamps, and a deduplication key.
2. Add user-level notification preferences for message, reply, and mention
   categories. Reuse per-conversation mute state rather than duplicating it.
3. Generate message, reply, and mention notifications transactionally with the
   triggering operation or through a durable outbox if introduced.
4. Apply recipient preferences and conversation mute before creating/displaying
   notification work. Notification recipient always comes from server state.
5. Add paginated list, mark-read, mark-all-read, and preference APIs.
6. Add canonical `notification.created` and `notification.read` events plus an
   unread notification summary.

**Suggested API**

```text
GET   /api/v1/notifications
POST  /api/v1/notifications/{notificationID}/read
POST  /api/v1/notifications/read-all
GET   /api/v1/notification-preferences
PATCH /api/v1/notification-preferences
```

**Frontend changes**

- Add a notification store, unread badge, list/panel, and useful deep links.
- Define foreground behavior so an active conversation does not create duplicate
  visual interruption while preserving durable state where required.
- Add preference controls and useful mute durations, including unmute.
- Refresh authoritative notification state after reconnect.

**Acceptance criteria**

- Users can read or mutate only their own notifications/preferences.
- Duplicate events do not create duplicate notifications.
- Mute and preferences produce documented behavior for message, reply, and
  mention events.
- External browser/mobile push is not presented as implemented.

## API and event compatibility

Phase 2 may evolve internal response DTOs, but existing direct-chat clients must
not silently receive ambiguous group payloads. Prefer additive typed fields and a
single documented transition to conversation-addressed WebSocket commands.

All canonical mutation events should include:

- event type
- conversation ID
- actor/sender public identity where needed
- canonical entity ID
- server timestamp or version
- persisted result data needed for deterministic reconciliation

Events are delivery hints, not authorization or durable state. REST responses and
database state remain authoritative after reconnect.

## Implementation order

1. Reconcile roadmap statuses and finalize the policies in this plan.
2. Add frontend test infrastructure and extract pure event/state reducers where
   needed for safe incremental changes.
3. Add the group/conversation/message migration and repository integration tests.
4. Implement group repository operations and service authorization.
5. Add group REST APIs and service/transport tests.
6. Refactor messaging to conversation-centric persistence and recipient fan-out.
7. Add group WebSocket messaging, delivery/read behavior, and concurrency tests.
8. Generalize frontend conversation types/routes and modularize the chat view.
9. Implement group creation, conversation, and administration UI.
10. Implement scoped presence and direct/group typing.
11. Complete reply UX, context actions, copy, forwarding, and reactions.
12. Implement mentions and `@everyone` authorization.
13. Implement in-app notifications, preferences, and mute behavior.
14. Implement group invite links.
15. Update roadmap, codebase, deployment/runtime documentation, and complete the
    full review and verification matrix.

Each numbered feature increment should remain independently reviewable. Do not
combine schema generalization, every API, and the complete frontend into one
unreviewable change.

## Verification

### Backend automation

Run before completing every backend increment:

```bash
go fmt ./...
go vet ./...
go test ./...
TEST_DATABASE_URL='postgres://...' go test -tags=integration ./internal/repository
```

Required backend coverage includes:

- Group role and capability matrix.
- Last-owner and ownership-transfer behavior.
- Non-member, removed-member, and stale-membership rejection.
- Transaction rollback for partial group/member/message creation.
- Group history visibility windows.
- Multi-recipient delivery/read/unread behavior.
- Sender/recipient spoofing resistance.
- Idempotent message, reaction, invite, and notification operations.
- Presence audience filtering and multi-session lifecycle.
- Mention membership and `@everyone` authorization.
- Notification ownership, mute, preferences, and deduplication.

### Frontend automation

Add a `test` script using Vitest and React Testing Library, then run:

```bash
cd web
npm run test
npm run lint
npm run build
```

Prioritize tests for:

- Conversation and message event reducers.
- Optimistic reply/reaction reconciliation and rollback.
- Presence update ordering and reconnect snapshot hydration.
- Typing transition, cleanup, and expiry using fake timers.
- Role/capability-based control visibility.
- Group member picker and settings actions.
- Mention rendering and notification preference behavior.

### Manual multi-client matrix

Verify at minimum:

- Two direct-chat users in separate browsers.
- One user with two tabs plus another user.
- Three users in a group with owner/admin/member roles.
- Add, remove, leave, promote, demote, and ownership transfer.
- A member removed while their group view is open.
- Concurrent message/reaction activity and pagination.
- Delivery/read state for different group members.
- Multiple users typing and a lost typing-stop event.
- Presence after closing only one of multiple sessions.
- Reconnect after message, membership, reaction, and notification changes.
- Message, reply, and mention behavior in muted conversations.
- Desktop and mobile-width group/chat/action interfaces.

Multi-instance synchronization is outside Phase 2 acceptance until distributed
WebSocket and presence infrastructure is implemented.

## Documentation impact

As increments land, update:

- `docs/chat_application_feature_roadmap.md` with accurate complete/partial status.
- `docs/CODEBASE.md` with schema, route, event, policy, and known-limit changes.
- `docs/DEPLOYMENT.md` when that missing canonical deployment document is added,
  especially for Redis, single-server WebSocket semantics, and notification
  runtime requirements.
- API/event examples and migration/runtime instructions where applicable.

## Phase 2 completion checklist

- [ ] Group schema and membership-role constraints are migrated safely.
- [ ] Group creation, metadata, member lifecycle, and role management are complete.
- [ ] Direct and group messaging are conversation-centric and authorized.
- [ ] Group delivery/read/unread state works per recipient.
- [ ] Frontend conversation routing and models support direct and group chats.
- [ ] Group creation, chat, settings, and administration UI are usable.
- [ ] Presence is scoped, initialized, displayed, and documented as single-server.
- [ ] Direct/group typing supports cleanup, expiry, and multiple users.
- [ ] Reply UX, copy, forwarding, context actions, and reactions are complete.
- [ ] Mentions and permission-controlled `@everyone` are complete.
- [ ] In-app notifications, preferences, and mute behavior are complete.
- [ ] Invite links are hashed, expiring, revocable, and authorization-protected.
- [ ] Backend unit and PostgreSQL integration tests pass.
- [ ] Frontend tests, lint, and production build pass.
- [ ] Manual multi-client authorization and realtime scenarios pass.
- [ ] Roadmap and canonical codebase documentation match shipped behavior.
- [ ] Deferred avatar upload, external push, distributed fan-out, and replay are
      clearly documented and are not represented as complete.
