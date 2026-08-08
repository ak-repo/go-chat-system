---
name: go-backend
description: Implement production Go backend changes using go-chat-system service, repository, error, context, and verification conventions.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Go Backend

## Core rule

Preserve:

```text
transport -> service -> repository
```

## Transport

Transport should:

- parse path/query/body values
- read authenticated user ID from context
- call a service
- serialize the standard response envelope

Transport should not:

- perform SQL
- contain durable authorization policy
- duplicate service validation

## Service

Services should own:

- business validation
- user-to-resource authorization
- block/friend/message rules
- orchestration across repositories
- mapping repository failures into stable application errors

Prefer explicit input DTOs when signatures otherwise become ambiguous.

## Repository

Repositories should own SQL and scanning.

Rules:

- parameterize all user-controlled values
- avoid string-built SQL for user input
- use context-aware calls when the project DB API supports them
- return enough error context for logs, not raw SQL details to HTTP clients
- select only required columns where practical
- add indexes for new high-frequency access patterns

## Errors

Client-visible errors should be stable and safe.

Do not expose:

- raw PostgreSQL errors
- credentials
- filesystem paths
- stack traces
- internal query text

Log internal detail server-side with request context where available.

## Authorization

Authentication proves identity.
Authorization proves the actor may perform the operation.

Do not rely on frontend visibility or client-provided IDs for authorization.

## Dependency injection

Reuse manual wiring in `internal/transport/injector/`.

When adding a repository/service:

1. construct dependency
2. inject dependency explicitly
3. avoid hidden package globals

## Configuration

Use the existing config layer.
Do not hardcode production hosts, secrets, origins, or ports.

## Scope

Do not rewrite adjacent services simply to match a preferred style.

Prefer compatible incremental changes.

## Verification

Targeted first, full second:

```bash
go test ./internal/service/...
go test ./internal/transport/websocket/...
go vet ./...
go test ./...
```

Format changed Go code before completion.
