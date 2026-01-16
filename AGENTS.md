# AGENTS.md — Sentinel Chat

This repository currently contains only `docs/` (no application source yet). Guidance below is oriented around the intended stack described in `docs/database.md` (Go + PostgreSQL + Redis + WebSockets). Update this file once a real build system (Makefile, Go module, CI) lands.

## Repo Map

- `docs/database.md`: database design, triggers, indexes, and architecture notes
- `.git/`: git metadata (no project tooling configs found)

## Commands (Current State)

There is no `go.mod`, `Makefile`, `package.json`, or CI config in this repo right now, so there are no verified build/lint/test commands to run.

### What to add (Recommended)

When code is added, prefer a single entrypoint (Makefile or Taskfile) so agents can reliably run everything.

**Makefile conventions (suggested):**

- `make setup` — install toolchain, dev deps
- `make build` — compile all binaries
- `make lint` — run lints/formatters
- `make test` — run all unit tests
- `make test-one PKG=./path TEST=TestName` — run one test

**Go module conventions (suggested):**

- Build: `go build ./...`
- Lint (golangci-lint): `golangci-lint run ./...`
- Format:
  - `gofmt -w .`
  - `goimports -w .` (preferred for import grouping)
- Unit tests: `go test ./...`
- Single package tests: `go test ./internal/foo -run TestSomething`
- Single test by regex: `go test ./... -run '^TestSomething$'`
- Verbose: `go test -v ./...`
- Race detector: `go test -race ./...`

**Database / migrations (suggested):**

- If using `golang-migrate`:
  - `migrate -path db/migrations -database "$DATABASE_URL" up`
  - One migration step: `migrate ... up 1`

**Redis / Postgres dev stack (suggested via docker compose):**

- `docker compose up -d postgres redis`
- `docker compose logs -f`

## Code Style & Conventions (For Future Code)

Until the Go services exist, treat these as the baseline style rules for all new code.

### General

- Keep changes minimal and focused; match existing patterns once code exists.
- Prefer simple, explicit code over cleverness.
- Avoid introducing new dependencies unless necessary.

### Project Structure (Suggested)

- `cmd/<service>/main.go` — service entrypoints
- `internal/` — private application modules
- `pkg/` — reusable packages (only if truly shared)
- `db/` — migrations, SQL snippets, seeds
- `docs/` — design docs (kept as-is unless requested)

### Imports

- Use `goimports` formatting.
- Group imports in this order:
  1. standard library
  2. third-party
  3. local module imports
- No dot-imports; avoid blank identifier imports unless required for side-effects.

### Formatting

- Run `gofmt` on all Go files.
- Keep line length readable; break long function signatures and chained calls.
- Prefer early returns to reduce nesting.

### Naming

- Go naming: `CamelCase` for exported, `camelCase` for unexported.
- Use domain-language names from the docs (`Conversation`, `Participant`, `Message`).
- Avoid generic names (`data`, `item`, `stuff`) unless tightly scoped.
- Prefer `ID` not `Id` (`UserID`, `ConversationID`).

### Types & Interfaces

- Prefer concrete types; introduce interfaces at boundaries (DB, cache, external APIs).
- Keep interfaces small (“accept interfaces, return structs”).
- Avoid `interface{}`; use generics or explicit types.

### Error Handling

- Always wrap errors with context: `fmt.Errorf("...: %w", err)`.
- Don’t log and return the same error at multiple layers.
- Treat user-caused errors as typed/validated errors (HTTP 4xx) vs system errors (5xx).
- Validate inputs at handler boundaries; return structured errors.

### Logging

- Use structured logging (e.g., `slog` or `zap`) once chosen.
- Include key fields: `user_id`, `conversation_id`, `message_id`, `seq_id` when relevant.
- Avoid logging secrets (passwords, tokens, auth headers).

### Database Access

- Follow the schema in `docs/database.md`.
- Prefer transactions for multi-step writes.
- Keep SQL close to repositories; avoid sprinkling query strings in handlers.
- Ensure indexes align with query patterns.
- Use context timeouts on queries.

### Concurrency & Context

- Every request path accepts `context.Context`.
- Don’t store request contexts globally.
- Use `errgroup` for fanout with cancellation.

### API / HTTP

- Prefer explicit request/response structs with JSON tags.
- Version API routes early (`/v1/...`).
- Return consistent error envelopes:
  - `code` (stable string)
  - `message` (human readable)
  - `details` (optional)

### Testing

- Table-driven tests for pure functions.
- Use `t.Helper()` in test helpers.
- Prefer fakes over heavy mocks.
- Integration tests (DB/Redis) should be tagged and skippable:
  - via build tags (`//go:build integration`), or
  - via env var (`INTEGRATION=1`).

**Suggested test commands:**

- Unit only: `go test ./...`
- Single test: `go test ./... -run '^TestName$'`
- Single subtest: `go test ./... -run '^TestName$/Subcase$'`
- Integration (build tag): `go test -tags=integration ./...`

## Cursor / Copilot Rules

- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` were found in this repository at the time this file was generated.
- If those are added later, mirror their requirements here and treat them as authoritative.

## Notes for Agents

- The repo currently lacks executable code and tool configuration.
- Before adding substantial code, introduce:
  - `go.mod`
  - `Makefile` (or Taskfile)
  - `docker-compose.yml` for Postgres/Redis
  - linter config (e.g., `.golangci.yml`)
- Keep `docs/database.md` as the current design source of truth.