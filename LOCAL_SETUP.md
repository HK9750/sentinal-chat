# Sentinal Chat Backend - Local Setup Manual

This guide explains how to run the backend locally so another developer can get started without guesswork.

## 1) Prerequisites

- Docker + Docker Compose plugin (recommended for local infrastructure)
- Go `1.25.x` (only needed if you want to run the API outside Docker)
- Git
- `curl` (optional, for health checks)

## 2) Clone and prepare environment

```bash
git clone https://github.com/HK9750/sentinal-chat.git
cd sentinal-chat
cp .env.example .env
```

Edit `.env` and set at least these values for local development:

```env
APP_PORT=5000
APP_MODE=debug

DB_HOST=localhost
DB_PORT=5432
DB_USER=sentinal_user
DB_PASSWORD=change-me
DB_NAME=sentinal_chat

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=change-me

JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRY_HOURS=12
REFRESH_EXPIRY_DAYS=14
COOKIE_SECURE=false
COOKIE_DOMAIN=
FRONTEND_URL=http://localhost:3000
```

Notes:

- `COOKIE_SECURE=false` is important for local HTTP (`http://localhost`) so refresh cookies can be set.
- Keep `FRONTEND_URL` exactly equal to your frontend origin to avoid CORS/WebSocket origin issues.
- Do not commit `.env`.

## 3) Run the backend

You have two good options.

### Option A (recommended): Run everything with Docker Compose

```bash
docker compose up -d --build
```

This starts:

- API service
- PostgreSQL
- Redis
- pgAdmin (`http://localhost:5050`)
- RedisInsight (`http://localhost:5540`)

### Option B: Run DB/Redis in Docker, run API with Go

```bash
docker compose up -d postgres redis
go mod download
go run cmd/api/main.go
```

## 4) Migrations and seed data

Migrations run automatically when the API starts.

Useful manual commands:

```bash
go run cmd/migrate/main.go status
go run cmd/migrate/main.go up
go run cmd/migrate/main.go seed-dev
```

Or with `make`:

```bash
make migrate-status
make migrate-up
make migrate-seed-dev
```

## 5) Verify local backend is healthy

```bash
curl http://localhost:5000/ping
curl http://localhost:5000/health
```

Expected:

- `/ping` returns `{"success":true,"data":{"message":"pong"}}`
- `/health` returns `{"success":true,"data":{"status":"healthy"}}`

## 6) Connect frontend to this backend

Use these values in the frontend env file:

```env
NEXT_PUBLIC_API_URL=http://localhost:5000
NEXT_PUBLIC_SOCKET_URL=ws://localhost:5000
```

The backend websocket endpoint is plain WebSocket at `/v1/ws`.

## 7) Optional features

- OAuth login requires valid Google/GitHub credentials in `.env`.
- File uploads require valid S3 configuration (`S3_*` env vars).
- If S3 is missing, chat still works, but upload endpoints will not be usable.

## 8) Stop services

```bash
docker compose down
```

To remove volumes as well:

```bash
docker compose down -v
```

## 9) Common local issues

- `401` after refresh/login: check `COOKIE_SECURE=false` for local HTTP.
- CORS errors in browser: check `FRONTEND_URL` exactly matches frontend origin.
- WebSocket closes immediately: verify token is passed and origin is allowed.
- DB connection refused: ensure Postgres container is up and `.env` DB values match.
