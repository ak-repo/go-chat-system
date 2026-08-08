---
description: Design and implement a focused PostgreSQL/Goose migration or repository query change.
agent: database
subtask: true
---

Database change:

$ARGUMENTS

Inspect existing migrations and repository queries before editing.

Use the `postgres-migration` skill.

Evaluate:

- existing rows
- backfill/default behavior
- constraints
- indexes
- lock/deployment risk
- rollback truthfulness
- query compatibility

Do not run destructive production database commands.

Run available validation/tests for the changed persistence path.
