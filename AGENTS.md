# go-chat-system

Backend-first real-time chat application.

Canonical project and feature reference: docs/CODEBASE.md.
Deployment/runtime reference: docs/DEPLOYMENT.md.

## Stack

Backend:
- Go
- Chi
- PostgreSQL
- Redis
- Gorilla WebSocket
- Goose migrations

Frontend:
- React
- TypeScript
- Vite

## Architecture

Backend request flow:

Transport
→ Service
→ Repository
→ PostgreSQL/Redis

Do not bypass layers without an explicit architectural reason.

Primary directories:

- cmd/server/          application bootstrap
- internal/domain/    domain models
- internal/repository persistence
- internal/service/   business logic
- internal/transport/ HTTP/WS transport
- internal/platform/  database/config infrastructure
- internal/shared/    shared helpers
- migrations/         Goose PostgreSQL migrations
- web/                React TypeScript application

## Realtime

WebSocket implementation lives under:

internal/transport/websocket/

Message business logic belongs in:

internal/service/message_service.go

Persistence belongs in:

internal/repository/message_repo.go

Never trust sender_id supplied by the WebSocket client.
Sender identity must come from authenticated context.

## Database

PostgreSQL migrations use Goose.

Never modify an already-deployed migration.
Create a new migration.

Migrations must include production-safe Up/Down behavior unless rollback
is intentionally impossible and documented.

## Backend Development

Before completing backend changes:

go fmt ./...
go vet ./...
go test ./...

Keep handlers thin.

Business rules belong in service layer.
SQL belongs in repository layer.

## Frontend Development

Frontend is under web/.

Use TypeScript.
Keep API calls under web/src/api/.
WebSocket transport belongs in the socket/API layer rather than UI components.

Before completing frontend changes:

cd web
npm run lint
npm run build

## Feature Changes

For non-trivial features:

1. inspect existing implementation
2. create/update a plan under plans/
3. identify API/database/WebSocket impact
4. implement smallest coherent backend change
5. test backend
6. implement frontend integration
7. run full verification
8. perform review

Do not refactor unrelated code while implementing a feature.

## Safety

Never expose:
- passwords
- JWT secrets
- config secrets
- database credentials

Do not commit config/config.yaml secrets.
