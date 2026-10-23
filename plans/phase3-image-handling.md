# Phase 3 image handling

Completion work: serialize processed writes/finalization with cancellation and cleanup using repository row locks; reject stale attempts and cap lease recovery at three attempts. Add authenticated media capabilities, clear drafts on navigation, bound URL renewal, and cover lifecycle failures with regression tests. No schema changes are needed for these fixes. The HTTP base path is `/api/v1`; WebSocket image metadata remains unchanged.

Implement one JPEG/PNG/WebP image (10 MiB, 25 megapixels) per message with an optional caption. Use private MinIO/S3 storage and direct presigned POST uploads to staging. Validate and normalize images in a bounded, PostgreSQL-backed worker; serve a 2048px image and 320px thumbnail through short-lived authorized URLs. Discard originals after processing.

Add attachment metadata and message links in a new Goose migration. Link ready owner/conversation-bound attachments atomically with messages, deliveries, mentions and notifications. Preserve client-ID retries, support image replies/caption edits, and forward by sharing processed media. Cleanup must lock against new links and preserve live forwards.

Add authenticated initiate/complete/status/cancel/download-URL endpoints. Keep SQL in repositories, policy in services, framing in transport, and S3 operations in the platform adapter. Image bytes never travel through WebSockets.

Extend the React API/socket types, composer, message rendering and previews with progress, processing, cancellation, retry, viewing and downloading. Keep tokens out of storage requests.

Verification: Go formatting/vet/unit tests, PostgreSQL migration/repository tests, MinIO lifecycle checks, frontend tests/lint/build, and final review. Cover MIME spoofing, corrupt/animated/oversized images, orientation/transparency, membership boundaries, concurrent retries, forwarding/deletion races, worker recovery and URL expiry. Update CODEBASE, deployment instructions and roadmap. Uploads default to disabled; other media, avatars and multi-image sends remain deferred.

## Completion implementation

- Publication locks the attachment row, checks the worker attempt and performs
  bounded storage writes before committing readiness. Cancellation and cleanup use
  the same row lock. Stale finalization returns a conflict. Three attempts include
  expired-lease recovery; cleanup gets a separate deadline from processing.
- `GET /api/v1/media/capabilities` reports availability and limits. The frontend
  hides unavailable uploads, clears/cancels drafts on navigation, and bounds image
  URL renewal with manual retry. Send retries preserve attachment and client IDs.
- No existing migration was altered. Goose Up/Down/reapply succeeded against a
  disposable database; rollback correctly rejects retained image records.
- Added service, handler, WebSocket, repository concurrency, frontend and real S3
  integration coverage. Real MinIO and the HTTP upload/process/send/history/edit/
  forward/download/delete lifecycle passed against disposable infrastructure.
- Updated CODEBASE, deployment, testing and the roadmap. Video/audio/voice/files,
  avatars and multiple image sends remain deferred. Uploads default to disabled.
- A real Chromium smoke test passed against isolated API/React/PostgreSQL/Redis/MinIO:
  upload, WebSocket image send/reply/retry, caption, history reload, view/download, navigation
  cleanup, disabled capabilities and text send. Storage requests carried no bearer
  token. Repeat smoke checks from the actual deployment origin before release.

Final verification: Go formatting/vet/unit tests passed; race-enabled repository,
storage and HTTP lifecycle integration tests passed; frontend lint/build passed
and all 33 tests across 11 files passed. Both staged and working-tree diffs passed
whitespace checks. Temporary runtime credentials and disposable services were
removed after verification.
