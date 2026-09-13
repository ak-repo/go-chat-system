# Testing

Unit tests run without external services:

```bash
go test ./...
go vet ./...
```

Repository tests use PostgreSQL and are guarded by the `integration` build tag.
Set `TEST_DATABASE_URL` to an isolated test database. The helper applies the
canonical schema when needed and truncates application tables before each test.

```bash
TEST_DATABASE_URL='postgres://user:password@localhost:5433/chat_test_db?sslmode=disable' \
  go test -tags=integration ./internal/repository/...
```

Do not point `TEST_DATABASE_URL` at a development or production database.
