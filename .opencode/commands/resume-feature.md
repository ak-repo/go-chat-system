---
description: Resume a long-running feature from its persistent plan and current repository state.
agent: build
---

Resume feature work from:

$ARGUMENTS

Do not restart planning from zero.

Read:

1. root `AGENTS.md`
2. the supplied plan
3. current git status
4. current git diff
5. files already changed
6. relevant tests

Reconcile the plan with current code.

Identify:

- already completed work
- partially completed work
- deviations from plan
- current failures
- next smallest safe implementation step

Continue from that step.

Use specialized agents/skills when appropriate.

Before finishing this session, update the plan's:

- Completed Work
- Decisions
- Remaining Work
- Risks
- Verification

Do not repeat finished work and do not discard uncommitted changes.
