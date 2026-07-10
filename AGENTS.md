# Repository Notes

- Keep new Go code under `internal/`; the old root-level `config/`, `database/`, `model/`, `repository/`, `service/`, `transport/`, and `pkg/` paths are stale.
- `cmd/server/main.go` is the only executable entrypoint.
- Current package layout is:
  - `internal/platform/config`
  - `internal/platform/database`
  - `internal/domain/model`
  - `internal/repository`
  - `internal/service`
  - `internal/transport/{injector,routes,middleware,websocket,wrapper}`
  - `internal/shared/{errs,helper,jwt,logger,utils}`

# Commands

- Run from the repo root; `config.Load()` and the Makefile both expect `./config/config.yaml`.
- `make check` is the prerequisite for `make build`, `make run`, and all `make migrate-*` targets.
- `make check` requires `go`, `yq`, and `goose` on `PATH`.
- `make run` starts the server with `go run ./cmd/server`.
- `go test ./...` is the main verification command.
- For a focused package check, use `go test ./internal/<package>/...`.

# Editing Rules

- Update imports after moving files; the module path stays `github.com/ak-repo/go-chat-system`, but app code now imports through `internal/...`.
- Load config before using `internal/shared/logger`, `internal/shared/jwt`, or the DB/Redis setup, because they read `config.Config`.
- Use `internal/transport/middleware.UserIDKey` for authenticated user context in new code.
- `internal/transport/routes.GlobalHub` is stopped during shutdown in `main`; preserve that wiring if you touch WebSocket startup/shutdown.

# Verification Gotcha

- Treat `README.md` as outdated for package paths; verify structure from code and `Makefile` first.
