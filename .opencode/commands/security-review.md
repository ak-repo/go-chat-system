---
description: Perform read-only security review of the supplied feature or current diff.
agent: security
subtask: true
---

Security review scope:

$ARGUMENTS

Inspect the affected implementation and current diff.

Focus only on security-relevant behavior:

- authentication
- authorization
- cross-user access
- sender identity
- JWT
- CORS / WebSocket origin
- rate limiting / abuse
- secrets
- error/data leakage
- SQL injection
- replay / duplicate event abuse

Classify findings by severity and provide the smallest safe fix.

Do not edit files.
