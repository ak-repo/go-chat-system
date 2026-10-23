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

## Image milestone

The image regression suite includes processing validation, upload/URL authorization,
HTTP errors/capabilities, WebSocket sender identity, navigation cancellation,
image-only composition, URL renewal and storage credential separation.

```sh
cd web
npm test
npm run lint
npm run build
```

Repository integration tests additionally cover publication/cancellation locks,
stale attempts, exhausted leases, concurrent client IDs, transactional links,
forward retention and final cleanup. They truncate application tables; the
`TEST_DATABASE_URL` above must identify a disposable database.

Real storage integration tests use a separate disposable MinIO service. Supply
credentials through the environment, without putting them in shell history or
committed configuration:

```sh
TEST_S3_ENDPOINT=http://localhost:59000 \
  go test -tags=integration ./internal/platform/storage/... -v
```

Also set `TEST_S3_ACCESS_KEY` and `TEST_S3_SECRET_KEY`. The test creates its own
`chat-media-test` bucket, checks presigned multipart POST size policy, CORS for
`http://localhost:5173`, anonymous denial, signed downloads, URL expiry and deletion,
then removes the bucket. Use only an isolated MinIO server with that bucket absent.

For the complete HTTP/service/repository/worker/storage flow, create a second,
empty disposable database and apply Goose migrations, then run:

```sh
TEST_MEDIA_DATABASE_URL="$ISOLATED_MEDIA_DATABASE_URL" \
  TEST_S3_ENDPOINT=http://localhost:59000 \
  go test -tags=integration ./internal/transport/http/... -v
```

The HTTP test creates unique users, conversations, attachment records and a unique
bucket. It does not truncate or remove retained database records; drop its database
afterward. It injects authenticated identities at the HTTP boundary; production
JWT/session middleware is covered separately. It verifies direct upload,
processing, owner status, concurrent idempotent image sends, forged sender rejection,
caption removal, history hydration, recipient-only download, forwarding and cleanup.

Migration verification should use another empty database: apply all migrations,
roll back the image migration, and reapply it. After creating image messages,
confirm Down refuses rollback and leaves schema/data intact. See
`DEPLOYMENT.md` for the guard's retained-data implications.

On 2026-10-23, Go formatting/vet/unit tests, repository integration tests, the real
MinIO lifecycle test, the HTTP image lifecycle test, Goose Up/Down/reapply and
populated-data rollback protection passed. Frontend lint/build and image regressions
also passed (33 tests across 11 files). Repository, storage and HTTP integration
suites also passed with the Go race detector. A real Chromium smoke test against the isolated API, React dev server,
PostgreSQL, Redis and MinIO passed direct upload, WebSocket image send/reply and rejected-send retry, captions,
history reload, viewing/downloading, navigation cleanup, disabled capabilities,
text send and storage bearer-token separation. The browser runtime was installed
under `/tmp` for verification; no browser dependency was added to the application.
Repeat the smoke check from the actual deployment origin before release, including
network interruption and URL renewal under your proxy/storage configuration.
