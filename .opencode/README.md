# go-chat-system OpenCode Workflow

Project-specific OpenCode setup for the current `go-chat-system` repository.

The workflow is designed around the current architecture:

- Go backend
- Chi HTTP API
- PostgreSQL
- Redis
- Gorilla WebSocket
- React + TypeScript + Vite
- Backend flow: Transport -> Service -> Repository -> Database
- Goose migrations
- Persistent feature plans under `plans/`

This directory intentionally contains specialized roles and reusable knowledge
instead of many feature-specific agents.

## Mental model

| Item | Purpose |
|---|---|
| `AGENTS.md` at repo root | Permanent repository rules and project truth |
| `.opencode/agents/` | Specialized responsibilities |
| `.opencode/skills/` | Reusable implementation knowledge loaded when relevant |
| `.opencode/commands/` | Repeatable workflows |
| `plans/` at repo root | Persistent state for long-running features |

## Included agents

| Agent | Why it exists |
|---|---|
| `planner` | Research a feature and maintain only its persistent plan |
| `backend` | Go HTTP/service/repository implementation |
| `frontend` | React + TypeScript implementation |
| `realtime` | Cross-stack WebSocket/event lifecycle changes |
| `database` | PostgreSQL schema/query/migration work |
| `documentation` | Current-code architecture and feature documentation |
| `reviewer` | Independent read-only production review |
| `security` | Read-only auth/security/abuse review |

## Included skills

- `project-architecture`
- `feature-planning`
- `go-backend`
- `api-contract`
- `websocket-realtime`
- `postgres-migration`
- `react-typescript`
- `testing`
- `security-auth`
- `production-review`

## Included commands

### Large feature workflow

```text
/feature-plan <feature request>
      |
      v
plans/<feature>.plan.md
      |
      v
/feature-build plans/<feature>.plan.md
      |
      v
/review-feature plans/<feature>.plan.md
      |
      +--> /security-review <scope>   when security-sensitive
      |
      v
/verify
```

If work spans multiple sessions:

```text
/resume-feature plans/<feature>.plan.md
```

### Focused changes

```text
/backend-change <change>
/frontend-change <change>
/realtime-change <change>
/database-change <change>
```

## Recommended feature lifecycle

1. Start with `/feature-plan`.
2. Read the generated plan before implementation.
3. Use `/feature-build` for a cross-layer feature.
4. For a narrow change, use the specialized command directly.
5. If the session ends, keep progress and decisions in the plan.
6. Continue with `/resume-feature`.
7. Run `/review-feature`.
8. Run `/security-review` when auth, JWT, WebSocket trust, authorization,
   rate limits, blocking, CORS, secrets, or uploads are affected.
9. Finish with `/verify`.

## Planning rule

The plan is not generic documentation. It is operational state for one feature.

A useful plan must record:

- confirmed current behavior
- requirements
- assumptions
- non-goals
- affected files
- API changes
- WebSocket changes
- database changes
- backend changes
- frontend changes
- tests
- risks
- implementation sequence
- completed work
- remaining work
- decisions that must survive future sessions

## Source-of-truth rule

Do not trust old architecture summaries when the repository disagrees with them.

Before implementation, inspect current code and migrations.

Current high-value paths include:

```text
cmd/server/main.go

internal/domain/model/
internal/repository/
internal/service/
internal/platform/
internal/shared/

internal/transport/injector/
internal/transport/middleware/
internal/transport/routes/
internal/transport/websocket/
internal/transport/wrapper/

migrations/

web/src/api/
web/src/context/
web/src/pages/
```

## Current project-specific invariants

- Authenticated sender identity is server-authoritative.
- Never trust `sender_id` supplied by WebSocket payloads.
- Keep backend dependency direction:
  `transport -> service -> repository`.
- Business rules belong in services.
- SQL belongs in repositories.
- Use new Goose migrations instead of editing deployed migrations.
- Keep REST/WebSocket contracts synchronized with TypeScript client code.
- Do not refactor unrelated code during feature work.
- Verify both backend and frontend before marking a feature complete.

## Repository root expectations

This package assumes your existing root `AGENTS.md` remains present.

Also create this directory once:

```bash
mkdir -p plans
```

Do not move `.opencode/README.md` to the project root.

## Optional root `opencode.json`

OpenCode works without this package providing a root config. If you want a
conservative baseline, create `opencode.json` at repository root:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "skill": {
      "*": "allow"
    },
    "bash": {
      "*": "ask",
      "git status*": "allow",
      "git diff*": "allow",
      "go test*": "allow",
      "go vet*": "allow",
      "gofmt*": "allow",
      "npm run lint*": "allow",
      "npm run build*": "allow",
      "git push*": "deny",
      "git reset --hard*": "deny",
      "git clean*": "deny",
      "rm -rf*": "deny"
    }
  }
}
```

Keep project-level global permissions conservative. Individual agents in this
package add stricter rules where useful.

## Why agents are not feature-specific

Do not create permanent agents such as:

- `read-receipt-agent`
- `typing-agent`
- `friend-request-agent`

Those are features, not responsibilities.

Instead:

- realtime knowledge -> `websocket-realtime` skill
- schema knowledge -> `postgres-migration` skill
- implementation responsibility -> `realtime` / `database` agents
- temporary feature state -> `plans/<feature>.plan.md`

This keeps the workflow reusable as the chat product grows.
