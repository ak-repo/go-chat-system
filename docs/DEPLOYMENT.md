# Deployment Guide — go-chat-system MVP

This document describes how to run and deploy the backend and frontend for production readiness.

## Overview

- **Backend:** Single Go binary (Chi HTTP API + WebSocket server). Requires PostgreSQL and Redis.
- **Frontend:** React SPA (Vite); build to static assets and serve via any static host or the same origin as the API.
- **Local development:** Docker Compose for Postgres and Redis; Makefile for migrate and run.

## Prerequisites

- Go 1.21+
- Node 18+ (for frontend build)
- PostgreSQL 14+
- Redis 6+
- (Optional) Docker and Docker Compose for local DB/Redis

## Backend

### Build

```bash
cd backend-go
go build -o bin/server ./cmd/server
```

### Configuration

- Primary config: `config/config.yaml`. Do **not** commit secrets; use environment variables in production.
- Env overrides (applied after YAML):

  | Env var              | Overrides              |
  |----------------------|------------------------|
  | `DATABASE_HOST`      | `database.host`        |
  | `DATABASE_PORT`      | `database.port`        |
  | `DATABASE_USER`      | `database.user`        |
  | `DATABASE_PASSWORD`  | `database.password`    |
  | `DATABASE_NAME`      | `database.name`        |
  | `SERVER_PORT`        | `server.port`          |
  | `REDIS_HOST`         | `redis.host`           |
  | `REDIS_PORT`         | `redis.port`           |
  | `JWT_SECRET`         | `jwt.secret`           |
  | `CORS_HOST`          | `CORS.host`            |
  | `CORS_PORT`          | `CORS.port`            |
  | `CORS_ALLOW_ORIGINS` | Comma-separated list   |

- For production: set `JWT_SECRET`, `DATABASE_PASSWORD`, and optionally `CORS_ALLOW_ORIGINS` (e.g. `https://app.example.com`).

### Migrations

```bash
cd backend-go
# Ensure DB is reachable (config or env)
goose -dir migrations postgres "user=... password=... dbname=... sslmode=... host=... port=..." up
# Or use your Makefile target if it reads from config
make migrate-up
```

### Run

```bash
./bin/server
# Or with env:
DATABASE_HOST=db.example.com JWT_SECRET=your-secret ./bin/server
```

- Listens on `SERVER_PORT` (default 8002). Serves HTTP + WebSocket on the same process.

### Health endpoints (for orchestrators)

- **Liveness:** `GET /live` — returns 200 if the process is up. Use for Kubernetes liveness probe.
- **Readiness:** `GET /ready` — returns 200 only if PostgreSQL and Redis are reachable. Use for readiness probe.
- Legacy: `GET /db-health`, `GET /redis-health` — still available.

## Frontend

### Build

```bash
cd frontend-react
npm ci
npm run build
```

- Output in `dist/`. Serve with any static server (e.g. Nginx, Caddy, or same host as API with a static route).

### Configuration

- API base URL and WebSocket URL are in `src/api/api.js` and `Chat.jsx` (e.g. `http://localhost:8002/api/v1`, `ws://localhost:8002/api/v1/ws`). For production, use env at build time (e.g. Vite `import.meta.env.VITE_API_URL`) or a config that matches your backend origin.

## Docker (optional)

- Use `backend-go/docker-compose.yml` for local Postgres and Redis only. The app itself can be run on the host or in a separate container.
- Example single-binary container (Dockerfile not in repo): multi-stage build with `go build -o /app/server ./cmd/server`, then run `/app/server` with env for DB and Redis.

## Production checklist

1. Set `JWT_SECRET` and DB/Redis credentials via env; do not commit secrets.
2. Set `CORS_ALLOW_ORIGINS` (and optionally `CORS_HOST`/`CORS_PORT`) to your frontend origin(s). WebSocket `CheckOrigin` uses the same list.
3. Use `/live` and `/ready` for liveness and readiness probes.
4. Prefer TLS in front of the app (reverse proxy); run the Go server behind Nginx/Caddy/Traefik.
5. WebSocket: config `websocket.max_message_size`, `read_deadline_sec`, and `messages_per_sec` in YAML (or extend config for env override) to limit abuse.

---

*End of deployment guide.*
