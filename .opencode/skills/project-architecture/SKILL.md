---
name: project-architecture
description: Apply the current go-chat-system architecture boundaries and source-of-truth rules when planning or implementing features.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Project Architecture

Use this skill whenever a change crosses packages, introduces new code, or
requires deciding where logic belongs.

## Current architecture

Backend dependency direction:

```text
Transport
   |
   v
Service
   |
   v
Repository
   |
   v
PostgreSQL / Redis
```

Primary locations:

```text
cmd/server/                    bootstrap

internal/domain/model/         domain models

internal/platform/config/      configuration
internal/platform/database/    PostgreSQL / Redis setup

internal/repository/           SQL and persistence

internal/service/              business rules and orchestration

internal/shared/               errors, JWT, logger, helpers, utilities

internal/transport/injector/   manual dependency wiring
internal/transport/middleware/ HTTP middleware
internal/transport/routes/     Chi route registration
internal/transport/websocket/  WebSocket hub/client/room protocol transport
internal/transport/wrapper/    HTTP/WS transport wrappers

migrations/                    Goose PostgreSQL migrations

web/src/api/                   TypeScript REST/WebSocket client layer
web/src/context/               auth/socket lifecycle state
web/src/pages/                 page composition
```

## Layer ownership

### Transport

Owns:

- HTTP request parsing
- route registration
- middleware
- WebSocket framing/transport
- response serialization
- authenticated context extraction

Transport should not contain durable business rules.

### Service

Owns:

- validation beyond syntax
- authorization/business policy
- orchestration
- cross-repository business operations
- domain-level error decisions

### Repository

Owns:

- SQL
- database scans
- persistence
- transaction-oriented storage behavior

Repositories must not depend on transport or service packages.

### Domain

Owns stable domain structures, not HTTP framework concerns.

## Realtime boundary

The WebSocket hub is transport infrastructure.

Persisted chat behavior belongs in message/domain/service/repository layers.

For any realtime event, distinguish:

```text
transport event routing
business authorization
persistence
delivery
frontend state
```

Do not collapse all five into the hub.

## Frontend boundary

Protocol and network behavior belongs under `web/src/api/` and socket lifecycle
under the relevant context/hook.

Pages should compose behavior and UI rather than becoming protocol engines.

## Dependency rules

Allowed:

```text
transport -> service
service   -> repository
repository -> database
```

Avoid:

```text
repository -> service
service -> transport
domain -> transport
```

## Source-of-truth rule

The live repository wins over old architecture reports.

Before planning a feature, inspect the current files and migrations that own the
behavior. Do not recreate functionality because an older document says it is
missing.

## Scope rule

Prefer the smallest coherent modification.

Do not introduce a new package or abstraction until existing ownership has been
checked.

Do not refactor unrelated code during a feature change.
