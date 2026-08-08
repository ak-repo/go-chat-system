---
name: websocket-realtime
description: Design and implement safe WebSocket events, presence, messaging, reconnect, persistence, and multi-connection behavior for go-chat-system.
compatibility: opencode
metadata:
  project: go-chat-system
---

# WebSocket / Realtime

## Security invariant

Authenticated server context is authoritative for sender identity.

Never trust a `sender_id` or equivalent actor field supplied by the client.

## Current conceptual flow

```text
authenticated HTTP upgrade
        |
        v
WebSocket handler
        |
        v
Client read/write pumps
        |
        v
Hub routing
        |
        +--> service / repository when persistence or business rules are required
        |
        v
receiver active connections
```

The exact implementation must be confirmed from current code before editing.

## Event design checklist

Every new client/server event must answer:

1. What is the event name?
2. Which direction does it travel?
3. Who is the authenticated actor?
4. Which identifiers may the client supply?
5. What authorization is checked?
6. Is the event durable or ephemeral?
7. If durable, when is it persisted?
8. Is there an acknowledgement?
9. What is the error event/response?
10. What happens on duplicate delivery?
11. What happens after reconnect?
12. What happens with multiple devices?
13. What happens if the receiver is offline?

## Durable versus ephemeral

Typically durable:

- chat messages
- edits/deletions when product requires them
- read state if it must survive reconnect/device changes

Typically ephemeral:

- typing indicator
- transient presence event

Do not persist an ephemeral event without a product reason.

## Delivery and persistence

Explicitly choose ordering.

Common safe default for durable messages:

```text
validate
authorize
persist
route/broadcast
ack
```

If current code intentionally delivers before persistence, document that tradeoff
rather than silently changing it.

## Multiple connections

A user may have:

- multiple tabs
- multiple browsers
- multiple devices

Do not assume one socket per user.

Presence should transition offline only when the user's final active connection
disconnects.

## Connection lifecycle

Review:

- origin validation
- authentication
- read limits
- write deadlines
- ping/pong
- read deadline refresh
- connection cleanup
- send queue backpressure
- rate limiting
- malformed event handling

## Reconnect

The frontend should avoid reconnect storms.

Where relevant use:

- bounded/exponential retry
- cleanup before reconnect
- authentication refresh strategy
- duplicate-safe event semantics
- history synchronization after reconnect

## Ordering

Do not promise strict distributed ordering unless implementation guarantees it.

Use persisted timestamps/IDs and deterministic history queries.

## Horizontal scaling

An in-memory hub routes only sockets connected to its own process.

When multi-instance deployment becomes required, evaluate:

- sticky sessions
- Redis Pub/Sub or Streams
- dedicated realtime service

Do not add distributed infrastructure before the deployment requirement exists.

## Tests

Prefer tests around:

- routing to all receiver connections
- sender identity override/prevention
- disconnect cleanup
- online/offline transitions
- slow-client backpressure
- invalid payloads
- authorization failures
- persistence failure behavior
- duplicate/reconnect behavior for new durable events
