---
name: feature-planning
description: Plan multi-step go-chat-system features so work can continue safely across multiple OpenCode sessions.
compatibility: opencode
metadata:
  project: go-chat-system
---

# Feature Planning

Use this for features expected to touch multiple layers or last more than one
working session.

The plan is persistent execution state, not a speculative design essay.

## Planning workflow

1. Restate the requested outcome.
2. Inspect current implementation.
3. Identify confirmed facts.
4. Identify assumptions.
5. Define non-goals.
6. Map affected layers/files.
7. Define contracts before implementation.
8. Determine persistence changes.
9. Determine security/concurrency risks.
10. Define tests.
11. Order the implementation.
12. Record verification commands.
13. Keep progress updated as work completes.

## Plan structure

Use:

```markdown
# <Feature>

## Goal

## Confirmed Current Behavior

## Requirements

## Assumptions

## Non-Goals

## Affected Files

### Backend
### Frontend
### Migrations

## REST API Contract Changes

## WebSocket Contract Changes

## Database Changes

## Security / Authorization

## Concurrency / Reconnect Behavior

## Implementation Steps

- [ ] ...

## Tests

## Verification

## Decisions

## Completed Work

## Remaining Work

## Risks
```

Omit irrelevant sections rather than filling them with noise.

## Affected-file discipline

List exact current files when they can be determined.

Do not invent a new file if an existing module clearly owns the behavior.

## Contract-first rule

When backend and frontend both change, define the contract before writing either
side.

For realtime events define:

```text
event name
client -> server payload
server -> client payload
authorization
persistence
ack/error
duplicate behavior
offline behavior
```

## Resume rule

At the end of each implementation session update:

- completed checkboxes
- important decisions
- deviations from original plan
- known failures
- remaining next step

A future session should be able to continue by reading:

```text
AGENTS.md
plan file
current git diff/status
current implementation
relevant skills
```

without depending on lost chat context.

## Planning quality test

A good plan answers:

- What exactly changes?
- Where does it change?
- Why does it belong there?
- What contract must remain synchronized?
- What can fail?
- How will we prove it works?
- What should the next session do?
