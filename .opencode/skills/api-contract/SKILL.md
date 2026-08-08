---
name: api-contract
description: Keep Go REST and WebSocket contracts synchronized with the React TypeScript client and define compatibility explicitly.
compatibility: opencode
metadata:
  project: go-chat-system
---

# API Contract

Use this whenever HTTP request/response shapes, routes, auth behavior, or
WebSocket payloads change.

## REST contract template

Define:

```text
Method:
Path:
Auth:
Path params:
Query params:
Request body:
Success status:
Success body:
Error statuses:
Authorization:
Pagination:
Compatibility notes:
```

## Project HTTP expectations

The API namespace is under `/api/v1`.

Protected endpoints receive authenticated identity from backend context.

Do not treat user IDs supplied in bodies as authenticated identity.

Preserve the project's response envelope unless the task explicitly changes it.

## Frontend synchronization

A backend contract change is incomplete until the TypeScript API layer is
checked.

Inspect:

```text
web/src/api/
web/src/context/
web/src/pages/
```

Update types and callers together.

Do not hide contract drift with `any`.

## Pagination

For potentially unbounded collections define:

- limit
- cursor or offset
- deterministic order
- default
- maximum
- empty result behavior

Message history should have deterministic chronological semantics.

## Errors

Prefer stable machine-understandable error semantics.

Do not make UI logic depend on raw database error strings.

## WebSocket contract

For each event define:

```text
event:
direction:
authenticated actor:
payload:
authorization:
persistence:
ack:
error:
duplicate semantics:
reconnect semantics:
```

Example categories:

```text
chat.message
typing.started
typing.stopped
message.delivered
message.read
presence.updated
```

Event names are examples, not requirements. Follow the project's existing naming
before adding a new convention.

## Compatibility

Before removing/renaming fields ask:

- Is the frontend already using it?
- Could older clients still send it?
- Is a migration period required?
- Can a new optional field be used instead?

Prefer additive changes for evolving clients unless a breaking change is
explicitly accepted.
