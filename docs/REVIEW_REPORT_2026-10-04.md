# Chat application readiness review — 2026-10-04

## 1. Executive assessment

**Recommendation: do not treat the current application as fully verified for the next phase yet.**

The frontend compiles and its existing tests pass. Backend vet passes, and a fresh backend test run passes when one sandbox-incompatible SMTP socket test is explicitly excluded. Six actionable regressions were identified in the latest UI change. Important verification gaps remain around real PostgreSQL execution, browser layout, end-to-end account/chat flows, and concurrency.

This is a source review and executable-check assessment, **not production certification**. Passing utility tests does not prove the complete application works.

### Scope and baseline

- Reviewed working tree was clean at the start.
- Current commit: `f178b55227c1f1b687a935a1478cda9cb64a1f21`.
- Regression baseline: its parent, `64083b8`.
- Confirmed regression findings below are restricted to that change; unchanged behavior is not attributed to it.
- Broader assessment includes route/dependency wiring, authentication and session checks, WebSocket client/hub paths, message persistence and visibility predicates, group/invite flows, frontend contexts/API clients, tests, and project instructions.
- No application fixes or deployed migrations were modified. This report is the only retained review artifact.

## 2. Verification results

| Check | Result | Interpretation |
| --- | --- | --- |
| `gofmt -l cmd internal` | No output | Application Go files are formatted; checked without rewriting files. |
| `go vet ./...` | Passed | No diagnostics from the standard vet checks. |
| `go test ./...` | Passed using cached package results | Not sufficient evidence of a fresh successful run. |
| `go test ./... -count=1` | Failed | `TestSMTPDeliveryTimesOutWaitingForServerGreeting` cannot listen on TCP in this sandbox (`operation not permitted`). Other tested packages passed. |
| `go test ./... -count=1 -skip '^TestSMTPDeliveryTimesOutWaitingForServerGreeting$'` | Passed | Fresh validation excluding exactly the environment-blocked SMTP test. |
| `go test ./... -cover` | Blocked | Installed Go toolchain cannot locate coverage components, including `covdata`; no trustworthy coverage percentage obtained. |
| `go test -race ./internal/...` | Blocked | Toolchain reports `runtime/race: package testmain: cannot find package`. Concurrency safety is not verified. |
| `go test -tags=integration ./internal/repository/... -v` | All 23 integration tests skipped | `TEST_DATABASE_URL` unset; an exit status of zero is not a database test pass. |
| Isolated PostgreSQL startup under `/tmp` | Blocked | Sandbox denies TCP and Unix-domain socket creation. No existing database was used or truncated. |
| `cd web && npm run lint` | Passed | Existing frontend lint rules pass. |
| `cd web && npm run build` | Passed | TypeScript and production Vite build succeed. |
| `cd web && npm test -- --run` | Passed | 3 existing test files, 9 tests. |
| Temporary regression-observation tests | Passed | 3 tests reproduced the offline badge, archive-navigation state, and stale-sidebar behavior; temporary test file removed afterward. |
| Headless Chrome layout check | Blocked | Browser startup fails under sandbox socket restrictions; CSS findings are source-derived, not screenshot-verified. |

The installed Go version is `go1.27.1-X:nodwarf5`. Toolchain/environment failures above are not classified as application regressions.

## 3. Confirmed regressions requiring fixes

### R1 — Mobile account controls disappear [P2]

**Location:** `web/src/components/AppShell.tsx:38-41`, with the mobile rail-hiding rule in `web/src/index.css:195`.

At widths of 760px or less, the desktop rail is hidden. Profile and ordinary sign-out controls exist only inside that rail. The mobile topbar renders the username as plain text, and the mobile navigation contains only Chats, People, and Alerts. The previous Friends header exposed Profile and Logout; that header was removed.

**Impact:** Mobile users cannot reach profile/account settings or perform normal logout through the main navigation. Typing `/profile` manually is not a usable replacement.

**Remedy:** Provide a mobile account menu or equivalent visible Profile and Sign out controls.

**Acceptance check:** At 390px and 760px widths, navigate from Friends and Chat to Profile, then sign out without editing the URL; verify credentials and socket state are cleared.

### R2 — Conversation action menus are clipped [P2]

**Location:** `web/src/index.css:103`, together with `.conversation-panel { overflow: hidden; }`.

The row menu is absolutely positioned below its summary, but the enclosing conversation panel clips overflow. Absolute menus do not increase the panel's height. For the final conversation row—especially a list containing just one conversation—the dropdown extends below the panel and its lower actions are clipped. Increasing its z-index cannot escape ancestor clipping.

**Impact:** Archive/Unarchive and Hide actions in those dropdowns are inaccessible with normal pointer interaction.

**Remedy:** Render the menu outside the clipping ancestor, remove/restructure clipping, or position it upward with adequate collision handling.

**Acceptance check:** In a real browser, open menus for a single-row list and the last two rows of a longer list; all actions must remain visible and clickable on desktop and mobile.

### R3 — Offline direct-chat peers appear online [P2]

**Location:** `web/src/pages/ConversationsPage.tsx:57`.

The green presence dot is rendered for every direct conversation. Its condition checks `item.kind`, not the peer's authenticated presence state. `SocketContext` already provides presence, and ChatPage uses it correctly, but this page does not consume it.

**Impact:** Offline and unknown-status peers appear online, even after an offline event or while the socket is disconnected.

**Remedy:** Gate the indicator on `presence[peer]?.online`; do not treat unknown state as online.

**Evidence:** A temporary component test supplied an explicitly offline peer and still observed `.avatar-presence` in the rendered conversation row.

### R4 — Sidebar fails as a whole when a lookup rejects [P2]

**Location:** `web/src/components/ConversationSidebar.tsx:21-23`.

All peer-profile requests are combined with `Promise.all`, and the conversation list is not installed until every lookup succeeds. Neither the profile batch nor the outer list request has rejection handling. A single timeout, network error, or HTTP error rejects the entire chain, leaves the sidebar empty, and produces an unhandled rejection.

**Impact:** Unaffected direct and group conversations disappear from the sidebar because an unrelated profile request failed. There is no error state or retry control explaining the failure.

**Remedy:** Preserve successfully loaded conversations independently of name hydration; tolerate individual profile failures with a fallback; handle list errors visibly and allow retry.

**Acceptance check:** Return a rejected profile request alongside successful direct/group data, and separately reject the list request. Verify partial data remains usable or a recoverable error is shown.

### R5 — Sidebar preferences remain stale after successful mutations [P2]

**Location:** `web/src/components/ConversationSidebar.tsx:25`.

The sidebar loads only on mount/user-ID change and keeps a separate list cache. ChatPage's Pin/Archive actions update only its own `conversation` state. A successful mutation does not unmount the sidebar, change the effect dependency, or update its cached list.

**Impact:** Pinning does not reorder the sidebar; archiving leaves the conversation in the supposedly active list. The sidebar contradicts successfully persisted preferences until a later reload/remount.

**Remedy:** Share conversation-list state, invalidate/refetch it after mutations, or propagate the mutation result to the sidebar.

**Evidence:** A temporary test changed the mock server list and rerendered the mounted sidebar; the original row remained and `listConversations` was still called only once.

### R6 — “View archived” opens the active list [P2]

**Location:** `web/src/components/ConversationSidebar.tsx:33`.

The button navigates to `/conversations` without query parameters or router state. ConversationsPage always initializes its `archived` flag to `false` and does not read a requested view from navigation state. The result is the All chats view, not the archived view advertised by the button.

**Impact:** The archive shortcut does not perform its stated action; users must discover and click the separate Archived tab afterward.

**Remedy:** Carry the intended archive filter in the URL/state and initialize the destination from it, or implement the archived view directly in the sidebar.

**Evidence:** A temporary test clicked the button and observed `{ path: '/conversations', state: null, search: '' }`.

## 4. Existing test coverage and missing verification

There are **97 named Go test functions** in the source inventory: 23 are PostgreSQL integration tests. This is a test-function inventory, not a coverage percentage or a count of subtests.

### Backend strengths already present

- User service: authenticated identity, validation, registration/login/refresh/logout/deactivation.
- Account tokens: generic recovery responses, token consumption, verification, development delivery, SMTP timeout behavior.
- Friendship/block services: actor ownership, self-action checks, pending requests, cancellation, pagination and dependency errors.
- Conversation services: member-scoped lookup/preferences, friendship/block prerequisites.
- Group services: authenticated actor, member validation, role error mapping, invite token response.
- Message services: authorization, validation, mutations, reactions, read-all and mention ranges.
- WebSocket hub: sender spoofing protection, persistence-before-delivery, errors/acks, group recipient fanout, event validation, scoped presence and multi-socket lifecycle.
- PostgreSQL tests exist for sessions/account tokens, direct/group conversation state, invites, membership lifecycle, message mutations, mentions/notifications, visibility, read cursor behavior and interactions.

These tests are useful, but fake repositories do not validate SQL, and integration tests must actually execute before making persistence claims.

### Frontend tests currently present

| File | What it covers |
| --- | --- |
| `web/src/api/messages.test.ts` | Message ordering/status merges, optimistic reconciliation/reactions, pagination deduplication. |
| `web/src/context/realtimeState.test.ts` | Presence ordering, typing expiry, unread snapshot merging. |
| `web/src/components/MentionText.test.tsx` | Unicode mention offsets and safe text rendering. |

There are no retained page tests, auth/socket provider lifecycle tests, API interceptor tests, or real-browser end-to-end tests in the current frontend source inventory. Documentation claiming there are no frontend tests is outdated: these three files do exist.

### Suggested tests before the next phase

**Highest priority:**

1. Regression tests for all six findings above, including browser tests for responsive controls and clipped popovers.
2. Two-user registration → verification → login → friendship → direct conversation → send → delivered/read flow.
3. Offline recipient → reconnect → history/unread reconciliation, without losing or duplicating messages.
4. Group creation → member addition/removal → permission changes → invite acceptance → leave/rejoin visibility.
5. Concurrent HTTP 401s and WebSocket refresh/reconnect, including logout while refresh is in flight.
6. Sender spoofing, non-member access, blocked direct messages, expired/revoked sessions and unauthorized edit/delete through actual HTTP/WS boundaries.
7. Run every PostgreSQL integration test against a disposable database; run migrations through Goose, not just the custom integration helper.
8. Run race-enabled backend tests with a complete supported Go toolchain.

**Additional targeted gaps:**

- Auth middleware tests for malformed credentials, access-vs-refresh tokens, inactive accounts/sessions and credential-source precedence.
- Redis-backed rate-limit tests for allow/deny, window expiration and fail-closed behavior.
- Notification-service ownership, preference updates, read/read-all publication and reconnect UI reconciliation.
- CORS and WebSocket upgrade origin matrix.
- Bootstrap/config environment overrides and readiness behavior during dependency failure.
- Failure/rollback and concurrent mutation tests for final group owner, max-use invites, idempotent sends and monotonic read state.
- Frontend pagination with realtime arrivals; route changes while requests are pending; optimistic mutation failure recovery.
- Mobile keyboard, narrow viewports, focus management and keyboard-only menu use.

Missing dedicated filenames do not automatically mean an area is untested: group repository tests live in conversation integration tests, and block/friend service checks share one test file.

## 5. Unused-code inventory and quality assessment

The following are **cleanup candidates**, not introduced correctness findings. Repository-wide reference searches support the inventory, but this is not a formal reachability analysis of public symbols.

| Candidate | Evidence / caution |
| --- | --- |
| `internal/shared/helper/helper.go` | No imports/call sites found in application packages. Entire generic-helper package appears unused. |
| `internal/shared/utils/ott.go: GenerateOTP` | Declaration found, no application call site. Account token implementation uses a different token mechanism. |
| `internal/transport/websocket/room.go: CreateRoom`, `sendToGroup` | No call sites found; active group messaging routes to persisted recipients instead. `Hub.rooms` remains initialized for this scaffold. |
| `web/src/api/client.ts: isTokenExpiringSoon` | No consumers found. Its helper `getTokenExpiry` is referenced only by this unused function. |
| `FriendRequestServiceImpl.GetPendingRequest` | Service method has tests but no registered HTTP route; the repository lookup is used by request creation and is not dead. Confirm intended public contract before removing the service method. |

### Maintainability observations

- The normal message path keeps SQL in repositories and WebSocket persistence/business decisions in MessageService; sender identity is overwritten from the authenticated client.
- `ChatPage.tsx` is 1,182 lines and coordinates history, subscriptions, optimistic mutations, mentions, forwarding, receipts and layout. Add characterization tests before extracting cohesive hooks/components.
- `message_service.go` is 851 lines; `message_repo.go` is 561 lines. These are high-change surfaces needing focused tests, not reasons for an unrelated rewrite.
- Multiple pages compress substantial async logic and JSX into single lines. Lint/build can pass while error handling and state transitions remain hard to review.
- Conversation list/name-loading logic is now duplicated between ConversationsPage and ConversationSidebar; the divergent error handling and cache ownership directly contribute to R4/R5.
- Do not remove scaffolding or reorganize packages as part of the immediate regression fixes. Keep cleanup separate and preserve the documented layer boundaries.

## 6. Runtime/production considerations (not new patch defects)

- WebSocket fanout/presence are process-local. Multi-instance operation needs an explicit design; Redis is not currently providing distributed WS fanout.
- Browser tokens are stored in localStorage and WebSocket upgrades put the access token in the query string. Review intermediary logging and XSS exposure before production deployment.
- `VITE_API_ORIGIN` is supported in the current API client; documentation describing the origin as unconditionally hard-coded is stale. Verify the actual production build-time origin and HTTPS/WSS setup.
- Email verification is a sign-in gate. Configure working SMTP outside development, and test real delivery/recovery rather than relying on development logs.
- Migrations are external to startup. Test the actual Goose upgrade path and explicitly decide rollback policy before deployment.
- `docs/DEPLOYMENT.md`, referenced by root instructions and CODEBASE, is absent from this checkout. Locate or supply an authoritative deployment runbook before relying on it.
- Do not run repository integration tests against an existing development/production database: their helper truncates application tables.
- No application Dockerfile was found; the existing compose file is infrastructure, not a complete production application deployment.

## 7. Proposed exit gate for the next phase

Advance once all of the following are recorded with reproducible evidence:

- [ ] R1–R6 fixed and regression-tested.
- [ ] Fresh full Go tests pass without excluding the SMTP test in an environment that permits local sockets.
- [ ] `go vet` and formatting checks pass.
- [ ] All 23 PostgreSQL integration tests execute, rather than skip, on a disposable database.
- [ ] Race tests execute successfully with a complete Go installation.
- [ ] Frontend lint, build and existing/new tests pass.
- [ ] Two-user direct-chat and group/invite browser smoke tests pass on desktop and mobile.
- [ ] Reconnect, logout/session revocation, dependency failure and unauthorized-access cases are exercised.
- [ ] Deployment configuration, SMTP, origin policy and Goose migration procedure are documented and tested.

**Bottom line:** Existing tests provide a meaningful foundation, but they currently leave most UI/application flow integration unverified. Fix the confirmed regressions and execute the missing integration/browser checks before using this review as a phase-completion signoff.
