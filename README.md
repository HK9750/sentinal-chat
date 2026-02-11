# Sentinal Chat API

Sentinal Chat is a Go backend for real-time chat with end-to-end encrypted messaging, conversations, calls, uploads, and broadcast lists. It uses Gin for HTTP, GORM for data access, PostgreSQL for persistence, and Redis for rate limiting, caching, and Pub/Sub event delivery.

**Highlights**
- JWT-based auth with refresh tokens and device-aware sessions.
- E2EE message storage with per-device ciphertexts.
- Conversations, participants, receipts, reactions, mentions, and starred messages.
- P2P DM calls with call quality metrics and WebRTC signaling state in Redis.
- Redis-backed outbox worker for reliable event publishing.
- WebSocket hub for live events (typing, message read/delivered, call events).

**Tech Stack**
- Go `1.25.5` (per `go.mod`)
- Gin HTTP framework
- GORM + PostgreSQL
- Redis (rate limiting, cache, signaling, Pub/Sub)
- Gorilla WebSocket
- Zap logging

**Architecture**
- Layered layout: `handler -> service -> repository` with domain entities in `internal/domain`.
- Transactions are handled in services when multiple writes must be atomic.
- Events are written to the `outbox_events` table in the same transaction and later published by the outbox worker.
- Redis Pub/Sub distributes events to connected WebSocket clients.

**Project Layout**
```text
cmd/
  api/              # HTTP API entry point
  migrate/          # Migration + seed CLI
config/             # Environment config
internal/
  commands/         # Command pattern (send/edit/delete/bulk archive)
  domain/           # Domain models
  events/           # Event types + Redis event bus
  handler/          # HTTP handlers (Gin)
  middleware/       # Auth, logging, rate limit, CORS
  redis/            # Redis stores (cache, rate limit, signaling, presence)
  repository/       # GORM repositories
  server/           # Router + WebSocket hub
  services/         # Business logic + outbox worker
  transport/httpdto # HTTP request/response DTOs
migrations/         # SQL migrations
pkg/                # logger, database, errors
```

**Configuration**
All config is read from environment variables (with defaults). The API will load a `.env` file if present.

Required variables:
- `APP_PORT` (default `8080`)
- `APP_MODE` (`debug` | `release` | `test`)
- `DB_HOST`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_PORT`
- `JWT_SECRET`, `JWT_EXPIRY_HOURS`, `REFRESH_EXPIRY_DAYS`
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`

Docker extras (used by `docker-compose.yml`):
- `PGADMIN_EMAIL`, `PGADMIN_PASSWORD`

Example `.env`:
```env
APP_PORT=8080
APP_MODE=debug

DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=sentinal_chat
DB_PORT=5432

JWT_SECRET=change-me
JWT_EXPIRY_HOURS=12
REFRESH_EXPIRY_DAYS=14

REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

PGADMIN_EMAIL=admin@sentinal.chat
PGADMIN_PASSWORD=Admin@123!
```

**Running Locally**
```bash
make up
make run
```

Or run directly:
```bash
go run cmd/api/main.go
```

**Migrations and Seeding**
The API runs SQL migrations on startup. You can also use the CLI:
```bash
go run cmd/migrate/main.go up
go run cmd/migrate/main.go status
go run cmd/migrate/main.go seed
```

Make targets:
- `make migrate-up`
- `make migrate-down`
- `make migrate-status`
- `make migrate-seed`
- `make migrate-seed-dev`
- `make migrate-reset`
- `make migrate-truncate`

**HTTP API Overview**
All responses use a common envelope:
```json
{"success": true, "data": {}}
```
Errors return:
```json
{"success": false, "error": "message", "code": "ERROR_CODE"}
```

Main routes (prefixes):
- `/v1/auth`: `POST /register`, `POST /login`, `POST /refresh`, `POST /logout`, `POST /logout-all`, `GET /sessions`, `POST /password/forgot`, `POST /password/reset`
- `/v1/messages`: `POST /`, `GET /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `DELETE /:id/hard`, `POST /:id/read`, `POST /:id/delivered`
- `/v1/conversations`: `POST /`, `GET /`, `GET /:id`, `PUT /:id`, `DELETE /:id`, `GET /direct`, `GET /search`, `GET /type`, `GET /invite`, `POST /:id/invite`, `POST /:id/participants`, `DELETE /:id/participants/:user_id`, `GET /:id/participants`, `PUT /:id/participants/:user_id/role`, `POST /:id/mute`, `POST /:id/unmute`, `POST /:id/pin`, `POST /:id/unpin`, `POST /:id/archive`, `POST /:id/unarchive`, `POST /:id/read-sequence`, `GET /:id/sequence`, `POST /:id/sequence`
- `/v1/users`: profile, settings, contacts, devices, sessions
- `/v1/calls`: create/list/participants/quality metrics (DM calls only)
- `/v1/uploads`: upload sessions and progress tracking
- `/v1/encryption`: identity keys, signed prekeys, one-time prekeys, key bundle
- `/v1/broadcasts`: broadcast lists and recipients

Utility routes:
- `GET /ping`
- `GET /health`
- `GET /goroutines`

**Authentication**
- Access tokens are JWTs signed with `JWT_SECRET`.
- Refresh tokens are hashed (SHA-256) and stored in `user_sessions`.
- Auth middleware validates JWT + session + device ID (if present).

**E2EE Messaging**
- Messages store per-recipient ciphertexts in `message_ciphertexts`.
- `POST /v1/messages` expects base64 ciphertexts per device.
- Sequence numbers are assigned by a Postgres trigger on insert.

**WebSocket**
Endpoint:
- `GET /v1/ws?token=...` or `Authorization: Bearer <token>`

Inbound client messages:
- `typing:start`, `typing:stop`, `read`, `ping`

Outbound events (from Redis Pub/Sub):
- `message:new`, `message:read`, `message:delivered`
- `typing:started`, `typing:stopped`
- `call:offer`, `call:answer`, `call:ice`, `call:ended`

**Rate Limiting and Cache**
- Auth, message, and call endpoints are rate limited via Redis.
- Redis cache store exists for sessions, users, and conversations (not wired into handlers yet).

**Operational Notes**
- SQL migrations define enums, triggers, and indexes for core tables.
- `cmd/migrate` supports `seed` and `seed-dev` for demo data.

**Known Limitations**
- Message detail and update endpoints return `NOT_SUPPORTED` for E2EE flows.
- Message search is disabled (`SearchMessages` returns forbidden).
- Call signaling helpers exist in services/Redis store but no HTTP or WS routes currently expose offer/answer/ICE endpoints.
- Some encryption session routes are intentionally disabled in the handler.
- Command undo paths are stubbed (no-op) and message versioning uses a fixed version number.

**License**
This repository does not include a license file. Add one if you plan to distribute.
