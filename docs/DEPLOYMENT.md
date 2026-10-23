# Deployment and image storage

The Go API, React frontend, PostgreSQL and Redis run separately. Apply Goose
migrations before starting the API; the server does not migrate automatically.
Keep real credentials in environment variables or a secret manager, never in
`config/config.yaml` or committed files. Use `config/config.example.yaml` as the
configuration reference.

## Start and migrate

Start PostgreSQL and Redis with `docker compose up -d postgres redis`. Set the
application database, Redis, JWT, SMTP and CORS configuration for your environment.
Use `goose -dir migrations postgres "$DATABASE_URL" up`, then `go run ./cmd/server`.
The environment variable in that migration command is supplied by the operator;
application configuration uses the variables documented in the example config
and `internal/platform/config/config.go`.

Build the frontend with `cd web && npm ci && npm run build`; serve `web/dist` with
SPA fallback. Set `VITE_API_ORIGIN` at build time to the API origin. API endpoints
use `/api/v1`; proxy WebSocket upgrades for `/api/v1/ws`. Configure allowed browser
origins and use HTTPS/WSS in production. Check `/health/live` and `/health/ready`.

## Optional private image storage

Images are disabled by default. For local storage run:

```sh
docker compose --profile media up -d minio minio-init
```

The profile builds pinned MinIO server and client sources. The first build needs
network access and can take several minutes. The initialization job creates the
private `chat-media` bucket. Ports 9000 and 9001 bind to localhost. Compose's
default credentials are for local development only; override them through
`MINIO_ROOT_USER` and `MINIO_ROOT_PASSWORD` before using a shared environment.

Set these API environment variables:

- `MEDIA_ENABLED=true`
- `S3_ENDPOINT=http://localhost:9000` for a host-run API and local browser
- `S3_REGION=us-east-1`, `S3_BUCKET=chat-media`, `S3_PATH_STYLE=true`
- `S3_ACCESS_KEY` and `S3_SECRET_KEY` from your storage credential source

For AWS S3 leave `S3_ENDPOINT` empty, use the actual region and bucket, and set
`S3_PATH_STYLE=false`. The AWS default credential chain is supported when both
explicit credential variables are empty. Never expose storage credentials to the
frontend. Give the API identity object Get/Put/Delete permissions for its bucket;
keep anonymous access disabled and enable S3 Block Public Access.

The configured storage endpoint must be reachable by both the API and browser.
A container-only hostname such as `minio` cannot be used in URLs opened by a
host browser. Use one resolvable endpoint or a reverse proxy preserving the signed
host/path. Use HTTPS for public endpoints.

Set `MEDIA_CORS_ORIGINS` for local MinIO to the exact frontend origins. For S3,
configure bucket CORS to allow those origins, POST and GET/HEAD, and the required
request headers. Do not send API bearer tokens or cookies with storage requests.
Direct multipart POST uploads must preserve every returned policy field and put
the file field last. Verify CORS from the deployed browser origin.

## Lifecycle and operation

The authenticated `/api/v1/media/capabilities` endpoint reports availability and
limits. The UI hides image selection when images are disabled or capability
lookup fails. Existing text messaging remains available.

One JPEG/PNG/WebP image of at most 10 MiB and 25 megapixels is allowed per message.
Upload policies expire after ten minutes. A PostgreSQL-backed worker polls every
two seconds, decodes one image at a time per API process, strips metadata, applies
orientation, and stores a maximum 2048px image and 320px thumbnail. Opaque images
become JPEG; transparent images become PNG. Animated images are rejected.

Worker leases expire after five minutes. At most three processing attempts are
allowed, including lease recovery after crashes. Processing and cleanup each
have a one-minute context; individual storage HTTP requests time out after twenty
seconds. Publication holds an attachment row lock across bounded writes and
readiness finalization. Cancellation and cleanup use that same lock. Avoid
configuring a database transaction timeout shorter than these operations.

Unlinked pending, failed or ready attachments expire after 24 hours. Cancelled
attachments and media whose final live message has been deleted are reclaimed
after upload policy expiry. Live forwards keep shared objects available. Cleanup
leases retry failed deletions after five minutes. Originals are removed after
successful processing and checked again after policy expiry to handle POST replay.
As an additional safeguard against abandoned in-flight direct uploads, configure
a bucket lifecycle expiration for the `staging/` prefix (for example, one day).
Do not apply that expiration to `media/`.

Downloads require current message visibility, conversation authorization and a
linked ready attachment. Signed URLs last five minutes; already-issued URLs can
remain usable until expiry after membership or message changes. The UI renews a
failed image URL once, then offers manual retry.

Monitor worker errors, storage failures, database lock waits, oldest queued work,
expired processing leases and attachments awaiting deletion. With images disabled
the worker does not run; keep storage enabled until outstanding cleanup finishes.
Shutdown cancels and joins the worker before closing the application.

## Migration and rollback

The image migration adds attachment metadata, message links and message types.
No later schema change is required by the completion fixes. Never edit a migration
that has already been deployed.

The image migration's Down operation intentionally rejects rollback while any
image message exists (including soft-deleted messages) or any attachment is not
in `deleted` state. UI deletion alone therefore does not make a used database
rollback-ready. Prefer a forward fix. If rollback is necessary, stop writers,
back up the database, complete object cleanup, and use an explicitly reviewed
maintenance procedure to purge retained image records and their dependent data
before running Goose Down. Do not disable the guard or silently discard media.

See `docs/TESTING.md` for isolated PostgreSQL and MinIO verification. Never run
truncating repository tests against a development or production database.
