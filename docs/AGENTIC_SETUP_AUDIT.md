# Agentic Setup Audit — go-chat-system

Audit date: 2026-08-23
Scope: `.opencode/`, root `AGENTS.md`, `opencode.json`, `plans/`, global `~/.config/opencode/`

## Inventory (what exists)

| Component | Count | Location |
|---|---|---|
| Agents | 8 | `.opencode/agents/` (backend, database, documentation, frontend, planner, realtime, reviewer, security) |
| Commands | 11 | `.opencode/commands/` (feature lifecycle + focused-change workflows) |
| Skills | 11 (~1,400 lines) | `.opencode/skills/` |
| Project config | 1 | `opencode.json` (permissions) |
| Rules | 1 | root `AGENTS.md` |
| Plans | 1 | `plans/error_tracing_feature.md` |
| Local deps | @opencode-ai/plugin | `.opencode/package.json` + 62 MB `node_modules` (untracked) |
| Global config | plugin + MCP | `~/.config/opencode/opencode.jsonc` |

**Verdict:** The skeleton is genuinely good — role separation (not feature-agents), persistent plans, security invariants, read-only reviewer/security, conservative permissions. But it is **not fully optimized**: there are 2 critical operational defects, several permission/doc drift issues, and dead weight.

---

## Critical findings

### C1. Secrets are committed despite the #1 safety rule — no agentic guardrail catches it

- `config/config.yaml` **is tracked in git** (`git ls-files` confirms), containing DB credentials/JWT secret.
- `.gitignore` lists `./config/config.yaml` — the leading `./` makes the gitignore pattern **invalid**, so it never matched.
- `AGENTS.md` says "Do not commit config/config.yaml secrets", but nothing enforces it: no plugin, no hook, no check in any of the 11 skills, and `/review-feature` only inspects diffs, not tracked state. `todo.md` flags `.opencode/node_modules` as a bug but misses this bigger one.
- **Impact:** every agent instruction about secret safety is undermined; history leak persists even after fixing the ignore rule.

### C2. Canonical documentation is stale — and agents are instructed to treat it as truth

- `AGENTS.md` declares `docs/CODEBASE.md` the "canonical project and feature reference".
- Reality drifted: `migrations/20260822090000_phase1_core_chat.sql` (landed Aug 22) adds delivery/read tracking (`DeliveredAt`, `ReadAt` in `internal/domain/model/message.go:35-36`, `internal/repository/message_repo.go` marks delivered), plus `conversation_service.go`, `mailer.go`, and `session.go` exist.
- `CODEBASE.md` §6 still claims *"The only Goose migration is 20260126104003_initial_schema.sql"* and read receipts aren't stored.
- The correct tool for this exists (`/codebase-doc`) but was not re-run after phase 1. Agents loading `project-architecture`/domain skills will reason from wrong facts.

---

## High findings

### H1. Permission rules don't match how verification is actually run

In `opencode.json`:

- `"gofmt*": allow` never matches `go fmt ./...` (the AGENTS.md-mandated command) → prompts every time.
- `"npm run lint*"` / `"npm run build*"` don't match `cd web && npm run lint` when run as a compound command → prompt friction on every frontend verification.
- No allows for `make migrate-status`, `npm install`, `npx tsc`; no deny for `make docker-clean` / `docker compose down -v` (volume destruction).

### H2. Dead plugin dependency + fragile global config

- `.opencode/package.json` pins `@opencode-ai/plugin@1.18.18` but **no plugin file exists anywhere in `.opencode/`** — 62 MB of unused `node_modules`.
- Global config loads `file:///home/ak/Downloads/codex-status.ts` — breaks on any other machine or Downloads cleanup.
- Remote MCP `Vision` (third-party Render-hosted server) conflicts with the project's own data-exposure posture.

### H3. `AGENTS.md` itself has structural drift

It's the first thing every agent reads, yet:

- Says models live in `internal/domain/` — actual: `internal/domain/model/`.
- Omits `injector/`, `middleware/`, `routes/`, `wrapper/` under transport (which `.opencode/README.md` correctly lists).
- Doesn't mention Makefile targets, docker ports (5433/6380), or that the `plans/` workflow exists.

---

## Medium / Low findings

| # | Finding |
|---|---|
| M1 | `docs/` has 5 overlapping files (`CODEBASE.md`, `CURRENT_REPOSITORY_DOCUMENTATION.md`, `PHASE1_COMPLETION_PLAN.md`, roadmap, `DEPLOYMENT.md`) — only 2 declared canonical; the rest pollute agent retrieval. |
| M2 | No frontend test infrastructure at all — `/verify` will report `N/A` forever; the `testing` skill has nothing frontend-concrete to anchor to. |
| M3 | No CI — all verification relies on the agent honestly running `/verify`. |
| L1 | Event-lifecycle checklist duplicated in the `realtime` agent body *and* `websocket-realtime` skill — double maintenance. |
| L2 | `opencode.json` sets no `small_model`/provider tuning for cheap subagent work. |
| L3 | `todo.md` mixes roadmap, bugs, and infra notes — belongs in `plans/` or issues. |

---

## Proposed remediation plan (in order)

1. **Secrets (C1):** fix `.gitignore` → `config/config.yaml`; `git rm --cached config/config.yaml`; **rotate DB password + JWT secret** (already in history); decide on history purge (`git filter-repo`). Add enforcement (see 4).
2. **Docs resync (C2):** run `/codebase-doc` to regenerate `CODEBASE.md` post-phase-1; archive/mark-superseded the 3 non-canonical docs.
3. **Permissions (H1):** add `"go fmt*"`, `"npm ci*"`, `"npm install*"`, `"make migrate-status*"`, `"npx tsc*"` to allow; add `"docker compose down -v*"` / `"make docker-clean*"` deny.
4. **Plugin decision (H2):** either delete the unused `@opencode-ai/plugin` dep + `node_modules`, **or** (better) write a small real plugin: secret-pattern guard before edit/commit + auto-`gofmt` hook. Recommend the latter — it turns C1 from a manual rule into a mechanical guarantee.
5. **AGENTS.md refresh (H3):** correct paths, add transport subdirs, make targets, plans-workflow pointer.
6. **Optional hardening:** vitest scaffold for `web`, CI workflow mirroring `/verify`, dedupe realtime checklist, `small_model` tuning, fold `todo.md` into `plans/`.

## Open decisions

1. **Secrets:** was `config/config.yaml` committed intentionally? If not — rotation + history rewrite, or just untrack going forward?
2. **Plugins:** remove the unused dependency, or build the real guard plugin (secret-scan + gofmt)?
