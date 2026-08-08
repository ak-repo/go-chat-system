---
description: Implement a planned cross-layer feature while keeping its persistent plan updated.
agent: build
---

Implement the feature described by:

$ARGUMENTS

First read:

- root `AGENTS.md`
- the supplied plan
- current git status/diff
- current code for affected paths

Treat the plan as execution guidance, not as a substitute for current code.

Use the relevant specialized agents/skills where they materially improve the
work:

- backend
- frontend
- realtime
- database

Implementation rules:

1. Work in the plan's dependency order.
2. Keep contracts synchronized.
3. Avoid unrelated refactoring.
4. Run targeted verification after each coherent stage.
5. Update the supplied plan with completed work, decisions, deviations, failed
   checks, and remaining work.
6. Do not mark a step complete unless it is actually implemented/verified.

Finish with a concise summary of changed files, tests run, and remaining risk.
