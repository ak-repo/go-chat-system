---
name: production-review
description: Perform a severity-ranked production review of go-chat-system changes with focus on correctness, regressions, security, concurrency, database safety, and contract drift.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Production Review

Review actual changed code plus enough surrounding implementation to understand
behavior.

## Review order

1. Requested requirement
2. Current diff
3. Architecture
4. Correctness
5. Authorization/security
6. Concurrency/realtime lifecycle
7. Database
8. API/frontend compatibility
9. Error handling/observability
10. Tests
11. Deployment/rollback

## Severity

### Critical

Likely data loss, auth bypass, secret exposure, remote compromise, or system-wide
production failure.

### High

Major feature failure, cross-user data exposure, severe message loss/duplication,
unsafe migration, or likely outage.

### Medium

Incorrect edge behavior, recoverability issue, performance problem, partial
contract drift, or missing important regression coverage.

### Low

Maintainability, clarity, small robustness gap, or non-blocking cleanup.

## Finding format

For each finding:

```text
Severity:
Location:
Problem:
Why it matters:
Smallest safe fix:
```

Do not inflate severity.

## Chat-specific checks

For message/realtime changes verify:

- authenticated sender identity
- receiver authorization
- block behavior
- multiple active connections
- persistence ordering
- reconnect/duplicate semantics
- history ordering
- offline receiver behavior
- rate limits
- backpressure

## Database checks

Verify:

- existing data compatibility
- indexes match query
- unique/FK constraints
- truthful rollback
- no accidental data destruction

## Frontend checks

Verify:

- endpoint/event name matches backend
- request/response type matches
- error/loading/disconnected states
- socket cleanup
- duplicate message prevention

## Scope discipline

Do not request unrelated refactoring.

A review is successful when it identifies concrete production risk, not when it
maximizes the number of comments.

If there are no blocking findings, say so explicitly and state residual risk.
