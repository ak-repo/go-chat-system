# Engineering Report: Production-Ready MVP Chat Application

**Repository:** go-chat-system  
**Scope:** Full-stack analysis (backend Go, frontend React)  
**Goal:** Convert to production-ready MVP chat application

---

## 1. Project Overview

### Architecture
- **Layout:** Monorepo with two applications: `backend-go/` (Go API + WebSocket server) and `frontend-react/` (React SPA). No shared code between them.
- **Backend flow:** Clean layering — **Transport → Service → Repository → Database**. Dependencies wired via a manual DI container in `transport/injector/injector.go`. Single process serves both HTTP and WebSocket.

### Technologies
| Layer | Stack |
|-------|--------|
| **Backend** | Go 1.x, Chi router, Gorilla WebSocket, pgx (PostgreSQL), go-redis, Viper (config), JWT (golang-jwt/jwt), Goose (migrations), Zap (logging) |
| **Frontend** | React 18, Vite, React Router, Axios. No TypeScript. |
| **Data** | PostgreSQL (primary), Redis (rate limiting) |
| **Local** | Docker Compose (Postgres, Redis), Makefile |

### Frontend architecture
- **Entry:** `main.jsx` → `App.jsx` with `BrowserRouter` and `AuthProvider`.
- **Routing:** Public `/` (AuthPage), protected `/home` (Home → Chat). `ProtectedRoute` uses `useAuth()` and redirects unauthenticated users.
- **State:** React Context only — `AuthContext` for user, loading, login, register, logout. User persisted in `localStorage`; no global token storage or API interceptor.
- **API:** Axios instance in `api/api.js` with `baseURL: "http://localhost:8002/api/v1"`, `withCredentials: true`. Thin helpers `get`/`post`/etc. in `api/services.js`.

### Backend architecture
- **Entry:** `cmd/server/main.go` — init logger, load config, connect DB + Redis, build router, start HTTP server, graceful shutdown.
- **Config:** Single `config/config.yaml` (Viper); used by app, migrations, tooling.
- **Transport:** Chi router in `transport/routes/routes.go`; middleware (CORS, logger, JWT auth, Redis rate limit, recovery); HTTP handlers wrapped in `wrapper.HTTPResponseWrapper`; WebSocket at `GET /api/v1/ws` with same JWT middleware.
- **Domain:** `model/` (User, Message, Friend, FriendRequest, blocks); `repository/` (User, Friend, FriendRequest, Block — **no Message repository**); `service/` (User, Friend, FriendRequest, Block). WebSocket hub in `transport/websocket/` (Hub, Client, Room, WSMessage).

### Communication
- **REST:** JSON over HTTP. Success envelope `{ status, data }`; errors via `utils.ErrorResponse`.
- **Real-time:** **Native WebSocket** (Gorilla) on `/api/v1/ws`. Single hub goroutine; message types `ReceiverUser` and `ReceiverGroup`; no Socket.io.

### Database structure
- **PostgreSQL:** Tables `users`, `friends`, `blocks`, `friend_requests` (Goose migrations). **No `messages` table** — real-time messages are not persisted.
- **Redis:** Used for rate-limiting (sliding window Lua script). No pub/sub or caching yet.

### Authentication
- **Method:** JWT (HS256). Backend: `Authorization: Bearer <token>` or cookie `access`. Middleware injects `userID` into context.
- **Login response:** Returns `{ user, token, exp }`. Frontend stores only `user` in state and `localStorage`; **token is never stored or sent** on subsequent requests (no Axios interceptor). WebSocket upgrade does not attach token (browser does not send cookies to cross-origin WS by default).

---

## 2. Current Features Implemented

| Feature | Status | Notes |
|--------|--------|--------|
| **User authentication** | Partial | Register + login exist; frontend uses wrong paths (`/login` vs `/auth/login`); token not sent on API/WS |
| **Real-time messaging** | Partial | WS hub + client/room routing work; no persistence; frontend WS does not send JWT |
| **Message persistence** | **Missing** | No `messages` table or repository; WS only broadcasts in-memory |
| **Chat rooms / DM** | Partial | Hub supports `ReceiverUser` and `ReceiverGroup`; `CreateRoom` exists but is never called; no API to create rooms |
| **Typing indicators** | Missing | No events or UI |
| **Online presence** | Missing | No presence events or UI (hub has per-user connection map but does not expose it) |
| **Message history** | Missing | No DB storage or history API |
| **File/image sharing** | Missing | Not implemented |
| **Notifications** | Missing | Not implemented |
| **Friends / friend requests / blocks** | Implemented | REST API and repos exist; frontend does not use them in UI (only Chat with hardcoded receiver) |
| **User search** | Implemented | `GET /users?filter=...`; not used in frontend |
| **Health checks** | Implemented | `/redis-health`, `/db-health` |

---

## 3. Code Quality Review

### Project structure
- **Backend:** Clear separation (cmd, config, database, migrations, model, repository, service, transport, pkg). Hub is created once at router setup inside the protected route group in `routes.go`; a single `go hub.Run()` runs per process. For clarity and testability, consider creating the hub in main or injector and passing it into routes.
- **Frontend:** Flat `src/` (api, context, pages). No feature-based or domain folders; no shared components or hooks.

### Separation of concerns
- **Backend:** Good — handlers delegate to services; services use repos. WS logic is in transport/websocket; business rules in service.
- **Frontend:** Auth and API are separated; Chat mixes connection, state, and UI.

### Naming
- **Minor:** `errs.ErrBadRequest` typo "inputes"; model `message.go` tag `josn:"is_group"`; route comment "serever" in main.go.
- **Minor:** Inconsistent naming (e.g. `UserDTO` vs `UsersDTO`).

### Reusability
- **Backend:** Wrapper and middleware are reusable; no shared validation layer (validation ad-hoc in services).
- **Frontend:** No shared form/input components; no reusable WS hook.

### Component design
- **Important:** `ProtectedRoute` uses `Navigate` but it is not imported in `App.jsx` — runtime error.
- **Minor:** Chat.jsx has hardcoded receiver ID; no conversation list or friend picker.

### API design
- **Backend:** Consistent envelope and status codes; auth and rate limiting applied per group. **Important:** Register returns only `userID` and `created_at`, not user + token — frontend cannot auto-login after register.
- **Frontend:** Services use wrong paths (`/login`, `/register` instead of `/auth/login`, `/auth/register`).

### Error handling
- **Backend:** **Important:** `utils.ErrorResponse` sends `err.Error()` to the client — information leakage (e.g. DB errors). Wrapper catches handler errors and returns JSON; recovery middleware catches panic. No structured error codes for clients.
- **Frontend:** Auth and form submit use try/catch; errors only `console.error`; no user-facing error messages or toasts.

### Logging
- **Backend:** Zap initialized; used in main. WebSocket uses `log` (stdlib). Inconsistent; no request IDs or correlation.
- **Frontend:** Console only; no error reporting or logging library.

### Type safety
- **Frontend:** JavaScript only — no TypeScript. No PropTypes or runtime checks. **Minor:** Increases risk of contract drift with API.

---

## 4. Architecture Problems

| Problem | Severity | Why it matters |
|---------|----------|-----------------|
| **Hub in route registration** | Minor | Hub is created inside the route group closure (runs once at startup). Prefer creating hub in main or injector and passing it in for clarity and testability. |
| **Frontend auth paths wrong** | Critical | `loginService`/`registerService` call `/login` and `/register`; backend serves `/auth/login` and `/auth/register`. Auth calls 404. |
| **Token never sent** | Critical | Login response contains `token` but frontend does not store it or attach to Axios (no interceptor). Protected REST and WebSocket (cross-origin) fail with 401. |
| **WebSocket auth** | Critical | Browser does not send cookies on cross-origin WS. Chat.jsx opens `ws://localhost:8002/...` without token in query or subprotocol; upgrade will often be 401. |
| **No message persistence** | Critical | No `messages` table or repository; WS only broadcasts. History, delivery guarantee, and multi-device sync impossible. |
| **Register does not return session** | Important | After register, backend does not return user + token; frontend cannot log user in without a second login call. |
| **ErrorResponse leaks errors** | Important | Sending `err.Error()` to client exposes internal details (DB, paths). |
| **Hardcoded values** | Important | Frontend: `baseURL` and WS URL hardcoded; backend: CORS and config in YAML (good) but no env override pattern documented. |
| **No input validation layer** | Important | Validation is ad-hoc (e.g. `utils.Required`); no central validation or max length for messages/usernames. |
| **Recovery not in chain** | Minor | `routes.go` does not use `mdware.Recover()`; panics can crash the server. |
| **CreateRoom never called** | Minor | Group chat data path exists in hub but no API or flow creates rooms; group messaging is dead code. |

---

## 5. Real-Time Messaging Implementation

### WebSocket architecture
- **Stack:** Gorilla WebSocket. Single hub type: `clients map[userID]map[*Client]bool`, `rooms map[roomID]*Room`, channels `register`, `unregister`, `incoming`.
- **Lifecycle:** Client created in `ws_handler.go` after upgrade; `ReadPump` and `WritePump` started (caller must run them — need to confirm both are started). On read error or close, client sends self to `unregister`; hub closes `send` and removes from map. Slow clients: if `send` is full, hub closes connection and removes client (backpressure).

### Connection lifecycle
- **Missing:** No explicit ping/pong or read/write deadlines in client — connections can hang. No reconnection logic on frontend; no heartbeat.

### Message broadcasting
- **Routing:** `routeMessage` switches on `ReceiverType` → `sendToUser` or `sendToGroup`. User: lookup `clients[ReceiverID]`, send to all connections of that user. Group: lookup `rooms[ReceiverID]`, send to all members’ connections. No persistence.

### Room management
- **CreateRoom** exists but is never invoked. Rooms are in-memory only; no DB or API. **Scalability:** Single process; rooms and clients lost on restart.

### User presence
- Hub has per-user connection set but does not broadcast join/leave or presence. No “online” indicator or events.

### Reconnection
- **Frontend:** No reconnection, exponential backoff, or token re-send on reconnect. **Backend:** No idempotency or sequence IDs; duplicate messages on reconnect possible.

### Event naming
- Frontend sends `event: "chat.message"`; backend `WSMessage` has `Event` field but routing uses only `ReceiverType` and `ReceiverID`. No event-based handlers (e.g. typing, presence) — only relay by receiver.

### Scalability
- **Single hub per process:** One goroutine and one in-memory map. Horizontal scaling would require either sticky sessions to the same server or a shared bus (e.g. Redis pub/sub) so messages are forwarded across instances. Not implemented.

---

## 6. Database & Data Models

### User model
- **Table:** `users` (id UUID PK, username, email UNIQUE, password_hash, role, created_at, modified_at, deleted_at). Index on `email`. FKs from `friends`, `blocks`, `friend_requests`.
- **Model:** `model.User` and `UserDTO` (no password). Adequate for MVP.

### Message model
- **Table:** **None.** `model.Message` exists (id, sender_id, receiver_id, content, is_group, timestamps) but no migration or repository. **Critical gap for MVP.**

### Chat / room model
- No DB table for conversations or rooms. Hub’s `Room` is in-memory only.

### Indexing
- Users: `email` indexed. Friends: `user_id` indexed. No message indexes (no table).

### Message retrieval
- Not implemented. For MVP: need messages table, repository with pagination (e.g. by conversation + created_at DESC, limit/offset or cursor), and REST or WS replay for history.

### Pagination
- SearchUser returns all matching rows; no limit. Friend/friend-request list endpoints not inspected but should be paginated for production.

### Timestamps
- Consistent use of `TIMESTAMPTZ` and `time.Now().UTC()` in Go. Good.

### Normalization
- Users normalized; friends as junction table. Messages should reference `users.id` for sender/receiver; optional `conversations` or `rooms` table if group chat is in scope.

**Recommendations:**
- Add migration: `messages` table (id, sender_id, receiver_id, conversation_id or room_id, body, created_at, etc.) with indexes on (receiver_id, created_at) and (sender_id, created_at) or (conversation_id, created_at).
- Add message repository with paginated list and insert; call from service layer and optionally from WS path (write-through) for persistence.
- Consider `conversations` table (e.g. for DM: two user IDs or hash) to support “last N messages per conversation” and future room-based chats.

---

## 7. Security Analysis

| Issue | Severity | Notes |
|-------|----------|--------|
| **Token not sent** | Critical | Frontend never sends JWT on REST or WS; protected resources and WS are effectively unusable or rely on cookie in same-origin only. |
| **Token storage** | Important | If token is later stored: localStorage is XSS-visible; prefer memory + refresh or httpOnly cookie. |
| **Input validation** | Important | Only basic “required” checks; no max length, sanitization, or schema validation. Risk of DoS (huge payloads) or abuse. |
| **ErrorResponse** | Important | `err.Error()` in JSON leaks internal info. |
| **WS origin** | Important | `CheckOrigin: func(r *http.Request) bool { return true }` — accepts any origin. Should validate origin for production. |
| **Rate limiting** | Implemented | Redis-based on auth and IP; good. |
| **CORS** | Configured | Origin from config; credentials true. Ensure config matches deployed frontend origin. |
| **SQL injection** | Mitigated | Parameters used in queries (e.g. SearchUser uses `$1`). |
| **Password** | Implemented | Hashing in service (e.g. utils.HashPassword); no plaintext storage. |
| **JWT method** | OK | HS256; expiry and validation. Consider refresh tokens and short-lived access for production. |

---

## 8. Performance Concerns

- **Inefficient queries:** SearchUser uses `ILIKE '%'+filter+'%'`; no limit — can return large sets. Add limit and consider index for prefix search if needed.
- **Large payloads:** No max size on WS message (only ReadBufferSize 1024); JSON decode in ReadPump can accept large frames. Consider max message size and timeouts.
- **Re-renders:** Frontend: no memoization; Chat appends to messages array and re-renders list. For large histories, virtualize or paginate.
- **Memory:** Hub holds all clients and rooms in memory; no cap. Under load, need backpressure and possibly connection limits.
- **WS flooding:** No per-client rate limit on WS messages; one client can flood the hub. Recommend rate limit or throttle in ReadPump.
- **Pagination:** Message history and user/friend lists need pagination to avoid large responses.

---

## 9. MVP Requirements

| Requirement | Status | Action |
|-------------|--------|--------|
| Authentication | Partial | Fix paths, store and send token, optional auto-login after register |
| Real-time messaging | Partial | Fix WS auth (token in query/subprotocol), optional ping/pong and reconnection |
| Message persistence | Missing | Add messages table, repo, and write path from WS or API |
| Chat rooms or DM | Partial | DM path exists in hub; add persistence and history; rooms optional for MVP |
| User presence | Missing | Add presence events from hub (join/leave) and simple UI |
| Message history | Missing | Implement with messages table + paginated API or WS replay |
| Basic UI | Partial | Auth and minimal chat exist; fix auth, add token, conversation list, and history |

---

## 10. Recommended Architecture

### Frontend
- **Folder structure:** Group by feature or domain (e.g. `auth/`, `chat/`, `api/`, `hooks/`, `components/`) with clear entry points.
- **State:** Keep Auth context; add token storage (memory + optional secure cookie) and Axios interceptor that sets `Authorization: Bearer <token>`. For chat: consider a small store (context or module) for current conversation and messages, or React Query for server state.
- **Components:** Reusable Input/Button/Form; Chat split into ConnectionProvider (WS + auth), ConversationList, ConversationView, MessageList (with pagination), MessageInput.

### Backend
- **API:** Keep Chi and envelope; add `/auth/register` response with user + token; add `GET /conversations` and `GET /conversations/:id/messages?limit=&before=` (or cursor). Optional `POST /messages` for persistence and compatibility.
- **Service layer:** Add MessageService and MessageRepository; from WS handler or dedicated “message handler” call service to persist after broadcast. Validate payload size and rate limit.
- **WebSocket:** Single hub created at startup (in main or injector); pass hub into route. Optional: persist message in same flow (broadcast + DB insert). Add ping/pong or read deadline; consider throttling per client.

### Database
- **Schema:** Add `messages` (id, sender_id, receiver_id, body, created_at; optional conversation_id). Index (receiver_id, created_at DESC), (sender_id, created_at DESC). Optional `conversations` (id, type, created_at) and `conversation_members` for future groups.

---

## 11. Refactoring Roadmap

**Phase 1 — Critical fixes**
1. (Optional) Move hub creation to main or injector and inject into WS handler for clarity.
2. Fix frontend auth: use `/auth/login` and `/auth/register`; store token (e.g. in memory or cookie); add Axios interceptor to set `Authorization: Bearer <token>`.
3. Fix WebSocket auth: pass token in query (e.g. `?token=`) or subprotocol; validate in handler and inject userID.
4. Add `Navigate` import in `App.jsx`.
5. Register response: return user + token so frontend can log in after signup (or document “login after register” and fix frontend flow).
6. Add recovery middleware to router; stop exposing raw `err.Error()` in API error responses.

**Phase 2 — MVP features**
1. Add migration and model for `messages`; implement MessageRepository and MessageService.
2. Persist messages: on WS receive, validate and persist then broadcast (or sync via API).
3. Add message history API (e.g. paginated by conversation) and optional WS “load history” event.
4. Frontend: conversation list (e.g. from friends or recent); select conversation; load history and show in Chat.
5. Basic presence: on register/unregister, broadcast “user_online”/“user_offline” (or maintain list); simple UI indicator.

**Phase 3 — Performance**
1. Paginate SearchUser and friend lists; add limit to message history.
2. WS: max message size and read deadline; per-client rate limit or throttle.
3. Frontend: paginate or virtualize message list; avoid storing unbounded history in state.
4. Add request ID and structured logging; remove `err.Error()` from client-facing error response.

**Phase 4 — Production readiness**
1. Env-based config (e.g. env vars override YAML); no secrets in repo.
2. CORS and WS CheckOrigin from config; tighten for production.
3. Optional: refresh token, httpOnly cookie for token.
4. Health checks used by orchestrator; optional readiness vs liveness.
5. Document deployment (single binary + DB + Redis; or containerized).

---

## 12. Improved Folder Structure

### Backend (Go)
```
backend-go/
├── cmd/server/main.go
├── config/
├── database/
├── migrations/
├── internal/           # or keep at repo root
│   ├── model/
│   ├── repository/
│   ├── service/
│   └── transport/
│       ├── http/
│       │   ├── middleware/
│       │   ├── routes/
│       │   └── wrapper/
│       └── websocket/
├── pkg/                # reusable libs
└── docker-compose.yml
```
- Single hub created in `main` or injector and passed to routes; no hub creation inside route handlers.

### Frontend (React)
```
frontend-react/src/
├── api/
│   ├── client.js       # axios instance + interceptor
│   └── services/
│       ├── auth.js
│       └── chat.js
├── components/
│   ├── ui/             # Button, Input, FormField
│   └── layout/
├── features/
│   ├── auth/
│   │   ├── AuthPage.jsx
│   │   └── context/
│   └── chat/
│       ├── Chat.jsx
│       ├── ConversationList.jsx
│       ├── MessageList.jsx
│       └── hooks/useWebSocket.js
├── pages/
│   └── Home.jsx
├── App.jsx
└── main.jsx
```
- Central API client with auth interceptor; feature-based grouping; shared hooks for WS and auth.

---

## 13. Optional Scaling Suggestions

- **Message queue:** For multi-instance deployment, use Redis (or similar) pub/sub: each instance subscribes to channels (e.g. per user or per room); WS handler publishes after local broadcast so other instances can push to their clients. Single hub per process remains; scaling is across processes.
- **Redis:** Already used for rate limiting. Extend for: session or token blocklist (logout); presence store (user_id → last_seen); pub/sub for cross-instance WS.
- **WebSocket scaling:** Sticky sessions (load balancer) or Redis pub/sub to fan out messages. Consider dedicated WS service vs combined API+WS.
- **Caching:** Cache user lookup by ID for WS routing if needed; short TTL. Cache conversation list or last message per conversation for list view.
- **Deployment:** Single binary; DB and Redis as services. Optional: Docker image for app; env-based config; health checks for k8s or Docker.

---

*End of report.*
