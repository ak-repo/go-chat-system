# Error Tracking Log Feature Plan

## Goal

Add reliable server-side error tracking logs that make production failures diagnosable without exposing internal details to clients. The requested behavior is similar to the example below: normal request logs plus a separate error line containing the internal error and a concise trace such as `HTTP route -> service/repository operation`.

## Confirmed Current Behavior

- The backend already uses Zap via `internal/shared/logger/logger.go`.
- `cmd/server/main.go` calls `logger.Init()` before `config.Load()`, so logging config from `config/config.yaml` is not available when the logger is initialized.
- `internal/transport/routes/routes.go` installs Chi's default `middleware.Logger` and `middleware.RequestID`; it does **not** install the project logger middleware `internal/transport/middleware.Logger()`.
- `internal/transport/middleware/logger.go` can log method, path, status, bytes, latency, remote address, user agent, and Chi request ID, but it is currently unused by the router.
- `internal/transport/wrapper/wrapper.go` catches errors returned by wrapped REST handlers and sends safe client messages through `utils.ErrorResponse`, but it does **not** log the internal error.
- `internal/transport/middleware/recovery.go` catches panics with the standard `log` package and returns `internal server error`; it does **not** use Zap and does **not** log a stack trace.
- Service methods commonly return raw repository/decoder errors to `HTTPResponseWrapper`; examples include `internal/service/users_service.go`, `internal/service/message_service.go`, `internal/service/friend_request_service.go`, `internal/service/friends_service.go`, and `internal/service/block_service.go`.
- Repository methods commonly return raw database errors; examples include `internal/repository/user_repo.go`, `internal/repository/message_repo.go`, `internal/repository/friend_request_repo.go`, `internal/repository/friends_repo.go`, and `internal/repository/blocks_repo.go`.
- The shared error package has `errs.Wrap(op string, err error)` in `internal/shared/errs/errors.go`, but current code only rarely/never uses it for request traces.
- WebSocket code under `internal/transport/websocket/` and `internal/transport/wrapper/ws_handler.go` logs errors using the standard `log` package, not the project Zap logger.
- The REST response contract is currently `{ "status": "error", "message": "safe message" }`; raw SQL/internal errors are not returned by `HTTPResponseWrapper`.
- Current database schema has only users/friends/blocks/friend_requests/messages in `migrations/20260126104003_initial_schema.sql`; there is no persistent error log table.
- Frontend API code (`web/src/api/client.ts`) consumes the existing response envelope and does not require server error trace fields.

## Requirements

- Log every REST handler error returned through `HTTPResponseWrapper` with enough context to diagnose failures:
  - request ID
  - method
  - path and raw query when safe
  - status code
  - authenticated user ID if present
  - internal error string
  - operation trace derived from wrapped errors where available
- Log recovered panics with request context and a stack trace.
- Keep client-facing REST/WebSocket errors safe; do not expose SQL errors, stack traces, filesystem paths, secrets, JWTs, or DB credentials.
- Prefer the smallest backend-only implementation that reuses Zap and existing middleware/wrapper layers.
- Make request logs and error logs share the same request ID.

## Assumptions

- "Error tracking log" means server-side structured logs, not an admin UI or durable database-backed error log.
- It is acceptable for the initial implementation to write logs to stdout/stderr via Zap for container/runtime collection.
- A concise trace based on explicit operation wrapping (`service.UserService.Register`, `repository.UserRepository.CreateUser`, etc.) is sufficient; a full stack trace is only required for panics.
- Adding a request/error ID field to client responses is useful but optional unless the product specifically wants users to report an incident ID.

## Non-Goals

- No database table for error logs in the first implementation.
- No frontend UI for browsing errors.
- No external error tracker integration such as Sentry, Honeycomb, Datadog, or OpenTelemetry in the first implementation.
- No unrelated rewrite of service/repository interfaces.
- No change to WebSocket event contracts for normal chat behavior.

## Affected Files

### Backend

- `cmd/server/main.go`
  - Load config before initializing the logger, or make logger initialization robust to unloaded config.
  - Prefer project Zap logger for startup/shutdown/server errors after initialization.
- `internal/shared/logger/logger.go`
  - Add safe defaults when config is empty.
  - Optionally honor `LoggingConfig.Output` if needed; current plan can keep stdout only.
- `internal/transport/routes/routes.go`
  - Replace Chi's default request logger with the project structured request logger.
  - Ensure request ID middleware runs before logging/error middleware.
  - Consider installing `LoggerWithRequestID()` if `middleware.GetReqID` alone is insufficient for non-transport code.
- `internal/transport/middleware/logger.go`
  - Keep structured access logs.
  - Ensure status/bytes are recorded correctly for normal and error responses.
  - Optionally log 5xx request summaries at error level, but avoid duplicating detailed wrapper logs.
- `internal/transport/middleware/recovery.go`
  - Use Zap logger.
  - Include `request_id`, method, path, authenticated user ID if present, panic value, and `runtime/debug.Stack()`.
  - Avoid writing a second response if headers were already written.
- `internal/transport/wrapper/wrapper.go`
  - Log returned handler errors before sending the safe response.
  - Include request context and operation trace.
  - Preserve existing response envelope unless adding a safe `request_id`/`trace_id` is explicitly chosen.
- `internal/shared/errs/errors.go`
  - Extend wrapping support if needed so errors can carry/expose an operation trace without losing `errors.Is` compatibility.
  - Candidate additions: `OpError` type, `Trace(err error) []string`, `Wrap(op string, err error)` returning `OpError`.
- Service files to add operation wrapping around repository/internal failures where useful:
  - `internal/service/users_service.go`
  - `internal/service/friend_request_service.go`
  - `internal/service/friends_service.go`
  - `internal/service/block_service.go`
  - `internal/service/message_service.go`
- Repository files to wrap database/query/scan failures with repository operation names:
  - `internal/repository/user_repo.go`
  - `internal/repository/message_repo.go`
  - `internal/repository/friend_request_repo.go`
  - `internal/repository/friends_repo.go`
  - `internal/repository/blocks_repo.go`
- WebSocket logging cleanup, if included in this feature slice:
  - `internal/transport/wrapper/ws_handler.go`
  - `internal/transport/websocket/client.go`
  - `internal/transport/websocket/hub.go`

### Frontend

- No required frontend change for server-side logging.
- If the REST error response adds a safe `request_id`, update type definitions in `web/src/api/client.ts` to include it.

### Migrations

- No migration required for the recommended first implementation.
- If the product later requires searchable in-app error records, create a new Goose migration for an `error_logs` table; do not modify `20260126104003_initial_schema.sql`.

## REST API Contract Changes

Recommended default: no breaking REST contract change.

Current error response should remain compatible:

```json
{
  "status": "error",
  "message": "an error occurred"
}
```

Optional additive field if desired:

```json
{
  "status": "error",
  "message": "an error occurred",
  "request_id": "<safe request id>"
}
```

Do not include internal `trace`, stack, SQL error strings, or raw wrapped error messages in API responses.

## WebSocket Contract Changes

- No protocol contract changes are required.
- Existing server-to-client error event should remain safe:

```json
{
  "event": "error",
  "sender_id": "system",
  "receiver_id": "<user id>",
  "receiver_type": "user",
  "data": { "message": "message could not be sent" }
}
```

- Log internal WebSocket read/write/persistence/marshal failures server-side with user ID and event metadata, but do not send raw internal details to the socket client.

## Database Changes

- None for the recommended first implementation.
- Rationale: the existing stack already has structured stdout logging; durable error-log persistence would introduce schema, retention, privacy, and write-amplification concerns that were not explicitly requested.

## Security / Authorization

- Do not log Authorization headers, JWT query tokens from `/ws?token=...`, passwords, refresh tokens, password hashes, database credentials, or full request bodies.
- Logging `r.URL.String()` is unsafe for WebSocket routes because it may include `token`; prefer `r.URL.Path` and sanitized query fields only.
- Authenticated user ID can be logged as operational context, but avoid logging email/password values from login/register bodies.
- Client-visible messages must continue to be produced from safe domain mappings in `wrapper.getUserFriendlyMessage`.
- Recovered panic stack traces are server-only.

## Concurrency / Reconnect Behavior

- REST error logging should happen synchronously and must not block on external services in the first implementation.
- Zap logger is concurrency-safe for concurrent requests.
- WebSocket logging must not write to client channels from extra goroutines or alter hub routing semantics.
- Reconnect behavior in `web/src/api/websocket.ts` is unaffected.
- If future durable error logging is added, it must not make request handling fail when the error-log sink is unavailable.

## Implementation Steps

- [ ] Fix logger initialization order in `cmd/server/main.go` so config is loaded before `logger.Init()` or logger has safe defaults.
- [ ] Switch `internal/transport/routes/routes.go` from Chi default logger to `mdware.Logger()` and confirm middleware order: request ID, CORS, recovery, access logging as appropriate.
- [ ] Update `internal/transport/wrapper/wrapper.go` to log every non-nil returned error with request context and safe metadata before writing the API error response.
- [ ] Update `internal/transport/middleware/recovery.go` to use Zap and include panic stack trace plus request context.
- [ ] Extend `internal/shared/errs/errors.go` with trace extraction if the existing `Wrap` string-only behavior is not enough for structured `trace: [route/service/repository]` fields.
- [ ] Add targeted `errs.Wrap("<package>.<operation>", err)` calls at repository boundaries for DB/query/scan failures.
- [ ] Add targeted `errs.Wrap("<service>.<operation>", err)` calls at service boundaries for unexpected internal failures, while preserving sentinel/domain errors for `errors.Is` checks.
- [ ] Replace standard `log.Printf` error paths in WebSocket transport with structured Zap logs where low risk.
- [ ] Keep REST/WebSocket client messages unchanged and safe.
- [ ] Run targeted tests and full Go verification.

## Tests

- Add/update tests for `internal/shared/errs/errors.go`:
  - wrapping preserves `errors.Is` for sentinel errors.
  - trace extraction returns ordered operation names.
- Add tests for `internal/transport/wrapper/wrapper.go`:
  - handler error returns safe response.
  - internal error is logged with request ID/method/path/status.
  - raw internal error is not included in response body.
- Add tests for `internal/transport/middleware/recovery.go`:
  - panic returns HTTP 500.
  - log includes stack/request context.
  - response does not expose stack.
- Existing service tests may need small updates if wrapped errors affect direct equality; prefer `errors.Is` assertions.
- WebSocket tests in `internal/transport/websocket/hub_test.go` should still pass; add tests only if WebSocket error behavior changes.

## Verification

Backend:

```bash
go fmt ./...
go test ./internal/shared/...
go test ./internal/transport/middleware/...
go test ./internal/transport/wrapper/...
go test ./internal/service/...
go test ./internal/transport/websocket/...
go vet ./...
go test ./...
```

Frontend only if an error response type is changed:

```bash
cd web
npm run lint
npm run build
```

Manual smoke checks:

- Trigger a validation error and verify the client receives only a safe message.
- Trigger or fake a repository/internal error and verify a structured error log includes request ID, path, status, error, and trace.
- Trigger a test panic route in a non-production/dev-only setup or unit test and verify stack trace is logged server-side only.

## Recommendations

- Start with structured stdout logs rather than database persistence.
- Use operation wrapping for durable logical traces instead of relying on runtime stack traces for all errors.
- Use full stack traces only for panics to avoid high log volume and accidental sensitive context capture.
- Add a safe request ID to client responses only if support/debug workflows need users to report it.
- Avoid logging raw query strings on `/api/v1/ws` because the current WebSocket client sends the JWT in the `token` query parameter.

## Unknowns That Materially Affect Implementation

- Whether logs must be searchable inside the application UI or only through deployment log aggregation.
- Whether the app will run under a log collector that expects JSON fields and stdout only.
- Whether client responses should include a safe `request_id`/incident ID.
- Required retention period and privacy policy if durable database error logs are later requested.
- Whether startup/shutdown logs should be fully converted from standard `log` to Zap in the same feature slice.

## Decisions

- Pending: choose whether to add `request_id` to REST error response bodies.
- Pending: choose whether WebSocket standard-log cleanup is included in the first implementation or a follow-up hardening task.

## Completed Work

- Current repository behavior inspected.
- Existing example in this file incorporated into the focused plan.

## Remaining Work

- Implement the backend logging changes in the order above.
- Add tests for wrapper/recovery/error trace behavior.
- Run verification commands.

## Risks

- Logging raw URLs can leak WebSocket JWT query tokens.
- Excessive wrapping can make logs noisy; use operation names at service/repository boundaries rather than every small helper.
- Changing error wrapping can break tests that compare errors directly instead of using `errors.Is`.
- Durable database logging, if added later, can fail recursively during database outages and should be isolated from request success/failure.
