---
description: Implement a WebSocket/realtime chat change across server protocol, persistence, and frontend lifecycle.
agent: realtime
subtask: true
---

Implement this realtime change:

$ARGUMENTS

Before editing, define the complete event lifecycle:

- direction
- payload
- authenticated actor
- authorization
- persistence
- acknowledgement/error
- duplicate/reconnect behavior
- multiple-device behavior
- offline behavior

Never trust client-supplied sender identity.

Keep backend and TypeScript event contracts synchronized.

Run targeted WebSocket/service tests plus relevant frontend checks.
