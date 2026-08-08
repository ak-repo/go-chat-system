---
description: Perform independent production review of a feature plan and/or current diff without editing code.
agent: reviewer
subtask: true
---

Review scope:

$ARGUMENTS

Inspect current git diff and surrounding code.

If a feature plan path is provided, compare the implementation against it.

Return only actionable findings classified as Critical, High, Medium, or Low,
followed by verification status and residual risks.

Do not edit files and do not request unrelated refactors.
