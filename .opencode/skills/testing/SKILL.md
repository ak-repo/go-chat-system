---
name: testing
description: Verify go-chat-system features with targeted Go tests, WebSocket behavior tests, frontend checks, and regression-focused coverage.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Testing

Testing should prove the business behavior changed correctly, not merely that the
project compiles.

## Existing high-value areas

Inspect current patterns around:

```text
internal/service/message_service_test.go
internal/shared/jwt/jwt_test.go
internal/transport/websocket/hub_test.go
```

Use current repository tests as the style reference.

## Order

Run narrow checks first:

```text
changed package tests
        |
        v
related feature tests
        |
        v
full Go suite
        |
        v
frontend lint/build/tests
```

## Service tests

Test:

- successful business path
- validation
- authorization
- blocked/friend relationship rules
- repository failure mapping
- edge cases

Mock or fake persistence only if it preserves the behavior being tested.

## WebSocket tests

For realtime changes test relevant combinations:

- receiver online
- receiver offline
- multiple receiver connections
- sender spoof attempt
- invalid event
- blocked unauthorized actor
- persistence failure
- connection unregister
- presence transition
- slow/backpressured client
- duplicate event behavior

## Repository tests

Use them when SQL semantics matter:

- pagination
- ordering
- unique constraints
- transactional behavior
- message history
- read receipts
- complex filters

## Frontend

At minimum require:

```bash
npm run lint
npm run build
```

When frontend testing exists, cover protocol/state behavior rather than only
snapshot markup.

## Regression rule

Every bug fix should include a test reproducing the old failure when practical.

## Completion rule

Do not report a test as passing unless it was actually run.

If a check cannot run because a dependency is unavailable, report that
explicitly and separate it from verified checks.
