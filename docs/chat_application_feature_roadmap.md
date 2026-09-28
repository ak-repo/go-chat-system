# Chat Application — Feature Roadmap

## Phase 1 — MVP / Core Chat

### Authentication & User Management
- [x] User registration
- [x] Login / logout
- [x] Password hashing
- [x] Access/refresh tokens
- [x] Email verification (SMTP-configurable; phone verification is not included)
- [x] Forgot/reset password
- [x] User profile
- [x] Change password
- [x] Account deletion (soft deactivation)

### User Discovery
- [x] Search users
- [x] Search by username
- [x] User profile lookup
- [x] Contacts/friends

### Conversations
- [x] Create 1-to-1 conversation
- [x] Get conversation
- [x] Conversation list
- [x] Archive conversation (per-user)
- [x] Delete conversation (per-user hide; preserves conversation history)
- [x] Pin conversation (per-user)
- [x] Mute conversation (per-user preference)

### Messaging
- [x] Send text message
- [x] Receive text message
- [x] Edit message
- [x] Delete message
- [x] Message timestamps
- [x] Reply to message
- [x] Message pagination

### Real-Time Communication
- [x] WebSocket connection
- [x] WebSocket authentication
- [x] New message events
- [x] Message edited events
- [x] Message deleted events
- [x] Connection/disconnection handling
- [x] Automatic reconnection

### Message Status
- [x] Sending state
- [x] Sent state
- [x] Delivered state (recipient receipt)
- [x] Read state
- [x] Failed state (client-side send failure)
- [x] Retry failed message

### Unread Messages
- [x] Unread count
- [x] Mark conversation as read
- [x] Mark message as read
- [x] Mark all as read (active, non-archived conversations)

---

## Phase 2 — Full Chat Experience

### Group Chat
- [x] Create group
- [x] Group name
- [ ] Group avatar (deferred: generated initials are used; no upload)
- [x] Add members
- [x] Remove members
- [x] Leave group
- [x] Group administrators
- [x] Promote/demote members
- [x] Group permissions
- [x] Change group information
- [x] Group invite link

### Presence
- [x] Online status (single-server, authorized audience)
- [x] Offline status (single-server, authorized audience)
- [ ] Last seen (deferred pending privacy/retention policy)
- [x] Presence synchronization (connection snapshot/reconnect; distributed sync deferred)

### Typing Indicators
- [x] Typing started
- [x] Typing stopped
- [x] Multiple users typing

### Message Interactions
- [x] Reply to message
- [x] Forward message
- [x] Copy message
- [x] Message reactions
- [x] Emoji reactions
- [x] Message context actions

### Mentions
- [x] Mention users
- [x] Mention notifications
- [x] @everyone
- [x] Highlight mentions

### Notifications
- [x] New message notifications (in-app)
- [x] Mention notifications (in-app)
- [x] Reply notifications (in-app)
- [ ] Push notifications (deferred: no device/provider worker infrastructure)
- [x] Notification preferences
- [x] Mute notifications

---

## Phase 3 — Media & Files

### File Handling
- [ ] Image messages
- [ ] Video messages
- [ ] Audio messages
- [ ] Voice messages
- [ ] Document/file messages
- [ ] File size validation
- [ ] MIME type validation
- [ ] Secure file downloads

### Object Storage
- [ ] S3/MinIO integration
- [ ] Upload service
- [ ] Presigned upload URLs
- [ ] Presigned download URLs
- [ ] File deletion

### Media Processing
- [ ] Image compression
- [ ] Image thumbnails
- [ ] Video thumbnails
- [ ] Media metadata
- [ ] Upload progress

---

## Phase 4 — Reliability & Synchronization

### Offline Support
- [ ] Detect connection loss
- [ ] WebSocket reconnect
- [ ] Offline message queue
- [ ] Retry queued messages
- [ ] Failed message recovery

### Message Synchronization
- [ ] Track last received message/event
- [ ] Sync missed messages after reconnect
- [ ] Prevent duplicate messages
- [ ] Idempotent message sending
- [ ] Message ordering

### Multi-Device
- [ ] Multiple active sessions
- [ ] Device registration
- [ ] Device-specific push tokens
- [ ] Synchronize messages across devices
- [ ] Synchronize read states
- [ ] Logout individual device
- [ ] Logout all devices

### Database Reliability
- [ ] Database transactions
- [ ] Proper indexes
- [ ] Foreign keys and constraints
- [ ] Database backups
- [ ] Recovery procedures

---

## Phase 5 — Search & Advanced Messaging

### Message Search
- [ ] Search messages
- [ ] Search within conversation
- [ ] Search by sender
- [ ] Search by date
- [ ] Search attachments
- [ ] Search users/groups
- [ ] Full-text search
- [ ] Elasticsearch/OpenSearch integration (optional)

### Threads
- [ ] Message threads
- [ ] Thread replies
- [ ] Thread participant tracking
- [ ] Thread unread counts

### Advanced Messages
- [ ] Location sharing
- [ ] Contact sharing
- [ ] GIFs
- [ ] Stickers
- [ ] Polls
- [ ] Link previews
- [ ] Scheduled messages
- [ ] Disappearing messages

---

## Phase 6 — Security & Moderation

### Security
- [ ] HTTPS
- [ ] Secure authentication
- [ ] Authorization
- [ ] Conversation-level access control
- [ ] WebSocket authentication
- [ ] Input validation
- [ ] Rate limiting
- [ ] SQL injection protection
- [ ] XSS protection
- [ ] Secure file validation
- [ ] Token expiration
- [ ] Session management

### User Safety
- [ ] Block user
- [ ] Unblock user
- [ ] Report user
- [ ] Report message
- [ ] Spam protection

### Group Moderation
- [ ] Admin roles
- [ ] Moderator roles
- [ ] Ban users
- [ ] Remove users
- [ ] Restrict messaging
- [ ] Delete reported messages

---

## Phase 7 — Production Infrastructure

### Redis
- [ ] Redis integration
- [ ] Presence storage
- [ ] WebSocket Pub/Sub
- [ ] Distributed event delivery
- [ ] Caching
- [ ] Rate limiting
- [ ] Temporary state

### Message/Event Processing
- [ ] Event-driven architecture
- [ ] Message queue
- [ ] Background workers
- [ ] Retry mechanism
- [ ] Dead-letter handling
- [ ] Kafka/RabbitMQ integration if required

### Scalability
- [ ] Stateless API servers
- [ ] Load balancer
- [ ] Multiple WebSocket servers
- [ ] Redis-based event distribution
- [ ] Horizontal scaling
- [ ] Database connection pooling
- [ ] Database read replicas if required

### Observability
- [ ] Structured logging
- [ ] Request IDs
- [ ] WebSocket connection logging
- [ ] Error tracking
- [ ] Metrics
- [ ] Database metrics
- [ ] Redis metrics
- [ ] Message delivery latency
- [ ] Active connection metrics
- [ ] Messages/second metrics
- [ ] Failed message metrics
- [ ] OpenTelemetry
- [ ] Prometheus
- [ ] Grafana

---

## Phase 8 — Advanced / Optional Features

### End-to-End Encryption
- [ ] Client-side encryption
- [ ] Key generation
- [ ] Key exchange
- [ ] Encrypted message storage
- [ ] Device key management
- [ ] Key rotation

> E2EE should only be implemented when there is a clear security requirement because it significantly increases system complexity.

### Voice & Video
- [ ] Voice calls
- [ ] Video calls
- [ ] Group calls
- [ ] Call history
- [ ] Screen sharing
- [ ] WebRTC integration

### AI Features
- [ ] AI chat assistant
- [ ] Conversation summaries
- [ ] Message translation
- [ ] Smart replies
- [ ] Spam detection
- [ ] Content moderation

### Other
- [ ] Stories/status
- [ ] Chat export
- [ ] Backup/restore
- [ ] QR/contact sharing
- [ ] Advanced notification controls

---

# Suggested Implementation Order

```text
Phase 1
  ↓
Phase 2
  ↓
Phase 3
  ↓
Phase 4
  ↓
Phase 5
  ↓
Phase 6
  ↓
Phase 7
  ↓
Phase 8
```

## MVP Definition

The first usable version should contain:

- [x] Authentication
- [x] User profiles
- [x] User search
- [x] 1-to-1 conversations
- [x] Text messages
- [x] WebSocket real-time communication
- [x] Sent/delivered/read status
- [x] Unread counts
- [x] Message pagination
- [x] Basic reconnection

## Recommended Backend Stack

```text
React Client
     │
     ├── REST API
     │
     └── WebSocket
           │
           ▼
      Go Chat Server
           │
      ┌────┼────┐
      ▼    ▼    ▼
 PostgreSQL Redis Object Storage
      │    │       │
      │    │       └── Images / Files
      │    │
      │    ├── Presence
      │    ├── Pub/Sub
      │    └── Cache
      │
      ├── Users
      ├── Conversations
      ├── Messages
      ├── Read States
      └── Reactions
```

## Core Database Entities

Recommended initial entities:

- `users`
- `sessions`
- `conversations`
- `conversation_members`
- `messages`
- `message_reads`

Later:

- `message_reactions`
- `attachments`
- `user_presence`
- `notifications`
- `devices`
- `message_mentions`
- `reports`
- `blocks`
- `group_roles`
