---
description: Run non-destructive backend and frontend verification and report a concise pass/fail matrix.
agent: build
---

Verify the current repository without implementing new features.

Do not auto-fix failures.

Run the checks that exist in this repository.

Backend target:

```text
gofmt check (report changed/unformatted files without rewriting when possible)
go vet ./...
go test ./...
go build ./cmd/server
```

Frontend target from `web/`:

```text
npm run lint
npm run build
```

Run frontend tests only if a test script exists.

If migrations changed, inspect/validate their Goose structure and report whether
a database-backed migration test was actually run.

Return:

```text
Check                     Result
Backend formatting        PASS/FAIL
Backend vet               PASS/FAIL
Backend tests             PASS/FAIL
Backend build             PASS/FAIL
Frontend lint             PASS/FAIL
Frontend build            PASS/FAIL
Frontend tests            PASS/FAIL/N/A
Migration validation      PASS/FAIL/N/A
```

For every failure include the first useful error and affected location.

Do not claim PASS for a command that was not executed.
