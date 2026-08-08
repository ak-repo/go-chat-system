---
name: postgres-migration
description: Create production-safe PostgreSQL and Goose schema changes, indexes, constraints, backfills, and repository queries for go-chat-system.
compatibility: opencode
metadata:
  project: go-chat-system
---

# PostgreSQL / Goose Migrations

## First rule

Do not edit an already-deployed migration to change production schema.

Create a new versioned Goose migration.

## Before writing SQL

Inspect:

- current migrations
- current repository queries
- foreign keys
- existing indexes
- nullable/default conventions
- application deployment behavior

## Migration checklist

For every change answer:

```text
Can current rows satisfy the new schema?
Is a default needed?
Is the default semantically correct?
Is a backfill needed?
Can the backfill be separated from the DDL?
Will an index build be expensive?
Can the change lock a hot table?
What happens to concurrent old application instances?
Can Down safely reverse it?
Would rollback destroy newer data?
```

## Constraints

Use database constraints for invariants that must remain true regardless of
application bugs, where practical.

Examples:

- self-relationship prevention
- uniqueness
- foreign keys
- valid status values

Keep service validation too when a client-friendly error is required.

## Index design

Create indexes from query shapes, not guesses.

For chat history consider the actual conversation filter and ordering.

Ask whether queries filter by:

- sender
- receiver
- both participants
- conversation/room
- created timestamp
- message ID/cursor

Match indexes to the real repository query.

## Group-chat evolution

When designing DM-only persistence, consider whether the chosen schema blocks
future group membership/read receipts.

Do not over-engineer future groups, but avoid an irreversible dead end when a
small generalization is clearly justified.

## Up / Down

Down should be truthful.

If rollback is intentionally destructive or unsafe, document the limitation
rather than pretending it is reversible.

Never silently drop user data in a rollback without an explicit requirement.

## Repository SQL

- parameterize input
- use deterministic ordering
- limit unbounded reads
- handle no-row behavior explicitly
- avoid N+1 patterns for conversation lists
- verify indexes with the intended query

## Validation

When local tooling permits:

- validate Goose syntax/order
- apply to an isolated development/test database
- migrate down/up when rollback is intended
- run repository/service tests
