---
description: Implement a focused Go backend change without unrelated frontend or architectural refactoring.
agent: backend
subtask: true
---

Implement this backend change:

$ARGUMENTS

Inspect the existing path first.

Use relevant project skills.

Preserve transport -> service -> repository boundaries.

If the requested change actually requires a WebSocket protocol change,
cross-stack frontend change, or migration with non-trivial design, report that
dependency clearly instead of hiding it.

Run targeted tests and full Go verification appropriate to the scope.
