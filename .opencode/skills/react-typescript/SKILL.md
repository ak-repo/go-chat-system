---
name: react-typescript
description: Implement maintainable React TypeScript chat UI, API modules, auth/socket context, and client lifecycle behavior in go-chat-system.
compatibility: opencode
metadata:
  project: go-chat-system
---

# React + TypeScript

## Project locations

```text
web/src/api/       HTTP and WebSocket client modules
web/src/context/   auth/socket lifecycle state
web/src/pages/     page-level composition
```

Extend with reusable components/hooks only when repetition or lifecycle
complexity justifies them.

## API rule

Do not make ad-hoc HTTP calls throughout pages.

Put transport calls and request/response types under `web/src/api/`.

Keep endpoint paths synchronized with backend routes.

## Type safety

Do not hide contract mismatches with broad `any`.

Type:

- request payloads
- response payloads
- WebSocket events
- nullable/optional fields
- pagination structures

Use discriminated unions for materially different WebSocket event shapes when it
improves safety.

## Authentication

Keep authentication/session handling centralized.

Do not make individual pages each invent token behavior.

Handle unauthenticated/expired state explicitly.

## WebSocket lifecycle

Prefer one intentional connection lifecycle for the authenticated session.

Avoid:

- one socket per component render
- sockets created without cleanup
- stale event handlers
- reconnect loops without backoff
- protocol JSON parsing scattered across unrelated UI

## UI state

Distinguish:

- server-persisted state
- optimistic/transient UI state
- connection state

Do not treat a transient typing indicator as persisted chat history.

## Chat behavior

For message history:

- use stable IDs as keys
- preserve deterministic ordering
- avoid unbounded in-memory growth as history grows
- design pagination before history becomes large

For realtime events:

- merge persisted/history data with live data carefully
- avoid duplicate messages after reconnect/refetch
- make event cleanup explicit

## User experience

Handle:

- loading
- empty list
- API failure
- socket disconnected
- reconnecting
- unauthorized/expired session
- send failure when persistence/ack fails

## Verification

Run from `web/`:

```bash
npm run lint
npm run build
```

Run tests when the project adds or already has relevant test commands.
