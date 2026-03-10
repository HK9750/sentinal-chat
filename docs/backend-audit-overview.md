# Backend Audit Overview

This document summarizes what is actually present in the repository and what still needs to be built.

## What exists today

- HTTP server bootstrap exists in `cmd/api/main.go` and `internal/server/server.go`.
- Only these routes are actually registered today:
  - `GET /ping`
  - `GET /health`
  - `GET /goroutines`
- PostgreSQL schema is substantial and covers users, devices, sessions, contacts, conversations, participants, messages, receipts, reactions, mentions, pins, stars, attachments, uploads, polls, calls, command logs, conversation clears, and outbox events.
- Repository layer exists for:
  - users
  - conversations
  - messages
  - calls
  - uploads
  - commands
  - outbox

## What is missing today

- No `internal/handler` package.
- No `internal/services` package.
- No `internal/redis` package.
- No websocket implementation.
- No outbox worker.
- No Redis Pub/Sub publisher/subscriber.
- No actual `/v1/...` API route registration.

## Important mismatch notes

### README vs codebase

- `README.md` describes many `/v1/...` endpoints, websocket flows, Redis usage, and internal packages that do not exist yet.
- `README.md` claims Gorilla WebSocket and Redis-backed realtime delivery, but there is no websocket code and no Redis client code.
- `README.md` claims per-device message ciphertext storage, but the current schema stores `messages.encrypted_content` directly and does not create a `message_ciphertexts` table.

### SQL vs Go mismatches

- `command_logs.status` SQL enum is:
  - `PENDING`
  - `EXECUTED`
  - `FAILED`
  - `UNDONE`
- Go command constants are:
  - `PENDING`
  - `EXECUTING`
  - `COMPLETED`
  - `FAILED`
  - `UNDONE`
- This must be fixed before command execution is implemented.

- `call_participants.status` SQL enum is:
  - `INVITED`
  - `RINGING`
  - `CONNECTED`
  - `LEFT`
  - `DECLINED`
- Call repository uses `JOINED` in a few places, which is not a valid SQL enum value.

## What the other docs in this folder contain

- `docs/api-endpoints.md`
  - Full endpoint plan for the HTTP API.
  - Separates implemented routes from repo-backed routes and recommended new routes.
- `docs/websockets.md`
  - Full websocket contract, event envelopes, authentication, subscriptions, typing, receipts, presence, and call signaling.
- `docs/redis-outbox.md`
  - Full outbox table behavior, worker flow, Redis channel taxonomy, publish envelope, retry rules, and failure handling.
- `docs/commands.md`
  - Full command model for chat actions, payload and undo payload shapes, statuses, execution flow, and undo rules.

## Source basis used for these docs

- `README.md`
- `Agents.md`
- `cmd/api/main.go`
- `internal/server/server.go`
- `internal/middleware/auth_middleware.go`
- `internal/middleware/ratelimit_middleware.go`
- `internal/repository/*.go`
- `internal/domain/*.go`
- `migrations/*.sql`
- `config/config.go`
- `internal/storage/s3_client.go`
