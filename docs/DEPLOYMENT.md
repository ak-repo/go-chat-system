# Deployment Guide

This document describes how to run and deploy `go-chat-system`. For project architecture, API contracts, implemented features, and current limitations, see `docs/CODEBASE.md`.

## Overview

- Backend: single Go binary serving Chi REST routes and WebSocket traffic. Requires PostgreSQL and Redis.
- Frontend: React TypeScript Vite SPA built to static assets and served by a static host or reverse proxy.
- Local development: root `docker-compose.yml` starts PostgreSQL and Redis only; Make targets run migrations, backend, and frontend.

## Prerequisites

- Go
- Node.js and npm for the frontend
- PostgreSQL
- Redis
- Goose for migrations
- `yq` for Makefile config parsing
- Optional Docker and Docker Compose for local PostgreSQL/Redis

## Backend

### Build

```bash
go build -o bin/server ./cmd/server
```

### Configuration

- Primary config: `config/config.yaml`. Use `config/config.example.yaml` as the reference shape. Do not commit secrets; use environment variables in production.
- Supported environment overrides:

| Env var | Overrides |
| --- | --- |
| `DB_HOST` | `database.host` |
| `DB_PORT` | `database.port` |
| `DB_USER` | `database.user` |
| `DB_PASSWORD` | `database.password` |
| `DB_NAME` | `database.name` |
| `REDIS_HOST` | `redis.host` |
| `REDIS_PORT` | `redis.port` |
| `JWT_SECRET` | `jwt.secret` |
| `PORT` | `server.port` |

- For production, set `JWT_SECRET` and database/Redis credentials through the deployment environment or a secret manager. Configure CORS origins in config for the deployed frontend origin.

### Migrations

```bash
make migrate-up
```

The Makefile builds the Goose DSN from `config/config.yaml` and requires `yq`.

### Run

```bash
./bin/server
# Or with env:
DB_HOST=db.example.com JWT_SECRET=your-secret ./bin/server
```

- The server listens on `PORT` if set, otherwise on `server.port` from config. REST and WebSocket traffic are served by the same process.

### Health endpoints (for orchestrators)

- Liveness: `GET /health/live` returns 200 if the process is up.
- Readiness: `GET /health/ready` returns 200 only if PostgreSQL and Redis are reachable.
- Legacy checks: `GET /db-health`, `GET /redis-health`.

## Frontend

### Build

```bash
cd web
npm ci
npm run lint
npm run build
```

- Output in `dist/`. Serve with any static server (e.g. Nginx, Caddy, or same host as API with a static route).

### Configuration

- REST base URL is currently defined in `web/src/api/client.ts` as `http://localhost:8002/api/v1`.
- WebSocket URL is derived in `web/src/api/websocket.ts` from the REST base URL and switches `http`/`https` to `ws`/`wss`.
- For production, replace hard-coded development URLs with build-time environment configuration before deploying the frontend.

## Docker (optional)

- Use root `docker-compose.yml` for local PostgreSQL and Redis only. The app itself can run on the host or in a separate container.
- Example single-binary container approach: multi-stage build with `go build -o /app/server ./cmd/server`, then run `/app/server` with DB, Redis, and JWT configuration supplied by the environment.

## Production checklist

1. Set `JWT_SECRET` and database/Redis credentials through environment or secret manager; do not commit secrets.
2. Configure CORS origins for the deployed frontend origin. WebSocket origin checks use the same CORS configuration path.
3. Run Goose migrations before starting the new application version.
4. Use `/health/live` and `/health/ready` for liveness and readiness probes.
5. Prefer TLS in front of the app with a reverse proxy such as Nginx, Caddy, or Traefik.
6. Remember WebSocket connection state is process-local; multiple backend instances need additional fan-out/presence design.
