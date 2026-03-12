# Sentinal Chat Backend

Go backend for auth, conversations, message history, uploads, attachments, and realtime chat delivery.

## Current API Surface

- Diagnostics: `GET /ping`, `GET /health`, `GET /goroutines`
- Auth: register, login, refresh, OAuth URL/exchange, logout, logout-all, sessions
- Conversations: create, list, get, add/remove participants, list participants, clear
- Messages: conversation history and get-by-id
- Uploads: single file upload, bulk upload, attachment create/get/viewed, message attachments
- WebSocket: `GET /v1/ws` with realtime messaging, typing, receipts, polls, and call signaling events

All protected HTTP routes require `Authorization: Bearer <access_token>`.

## Response Shape

Successful responses use:

```json
{
  "success": true,
  "data": {}
}
```

Errors use:

```json
{
  "success": false,
  "error": "message",
  "code": "ERROR_CODE"
}
```

## Local Run

```bash
make up
make migrate-up
make run
```

Default base URL: `http://localhost:8080`

## Auth Notes

- Access tokens are returned in the JSON payload under `data.tokens.access_token`
- Refresh tokens are primarily set as the `refresh_token` HttpOnly cookie
- `POST /v1/auth/refresh` accepts a body token, but Postman can also reuse the cookie jar

## WebSocket Notes

- Connect with `ws://localhost:8080/v1/ws?token=<access_token>` or a Bearer header
- First server frame is `connection:ready`
- Inbound frames use `type`, optional `conversation_id`, optional `call_id`, and `data`
- Event names use colon notation such as `message:send`, `typing:start`, `receipt:read`, `call:offer`

## Postman

Import:

- `postman/Sentinal-Chat-API.postman_collection.json`
- `postman/Sentinal-Chat-Local.postman_environment.json`

The collection is aligned to the currently registered routes in `internal/server/server.go`.
