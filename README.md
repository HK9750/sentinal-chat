# Sentinal Chat Backend – Frontend Integration Guide

This README is written for frontend engineers integrating with the `sentinal-chat` backend.

It documents:

- every HTTP route currently registered by the server
- authentication, access tokens, refresh cookies, and browser/CORS behavior
- exact request and response envelope shapes
- realtime integration over **plain WebSocket**
- every client → server websocket frame you can send
- every server → client websocket event you should listen for
- canonical payload shapes for conversations, messages, uploads, polls, receipts, calls, and commands
- practical frontend flows for auth, messaging, file uploads, typing, receipts, polls, calls, and undo/redo

> Important: this backend uses a **standard WebSocket endpoint**, **not Socket.IO**. Use the browser `WebSocket` API, `ws`, or another standards-compliant websocket client. Do **not** use the Socket.IO client.

---

## 1. Base URLs

### HTTP

Default local base URL:

```text
http://localhost:8080
```

Versioned API base:

```text
http://localhost:8080/v1
```

### WebSocket

Default local websocket base:

```text
ws://localhost:8080/v1/ws
```

For browser clients, the easiest way to authenticate is:

```text
ws://localhost:8080/v1/ws?token=<access_token>
```

The backend also accepts `Authorization: Bearer <access_token>` during websocket upgrade, but that is usually only practical for non-browser websocket clients.

---

## 2. Global behavior you should know first

### 2.1 Response envelope

All HTTP endpoints use the same top-level JSON envelope.

Successful responses:

```json
{
  "success": true,
  "data": {}
}
```

Error responses:

```json
{
  "success": false,
  "error": "message",
  "code": "ERROR_CODE"
}
```

### 2.2 IDs and timestamps

- Most entity IDs are UUID strings.
- All timestamps are UTC timestamps serialized as RFC3339 / ISO-8601 strings.
- Numeric sequence IDs such as `seq_id`, `before_seq`, and `up_to_seq_id` are integers.

### 2.3 Protected routes

All protected HTTP routes require:

```http
Authorization: Bearer <access_token>
```

The public HTTP routes are:

- `POST /v1/auth/register`
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `GET /v1/auth/oauth/:provider/url`
- `POST /v1/auth/oauth/:provider/exchange`
- `GET /ping`
- `GET /health`
- `GET /goroutines`
- `GET /v1/ws` (websocket upgrade, but it still requires a valid access token)

### 2.4 Browser cookie behavior

The refresh token is mainly managed as an **HttpOnly cookie** named `refresh_token`.

That means:

- your frontend **cannot** read it from JavaScript
- your frontend **should** send requests with `credentials: "include"` or `withCredentials: true` if you want the browser to store or send the refresh cookie
- `POST /v1/auth/refresh` can use either:
  - the cookie automatically, or
  - a JSON body containing `refresh_token`

### 2.5 CORS behavior

The backend:

- allows credentials
- allows headers: `Content-Type`, `Authorization`, `X-Request-Id`
- allows methods: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`
- expects the browser origin to exactly match `FRONTEND_URL` (unless the server is configured with `*`)

### 2.6 Canonical frontend setup

Example `axios` setup:

```ts
import axios from "axios";

let accessToken: string | null = null;

export const api = axios.create({
  baseURL: "http://localhost:8080/v1",
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers = config.headers ?? {};
    config.headers.Authorization = `Bearer ${accessToken}`;
  }
  return config;
});

export function setAccessToken(token: string | null) {
  accessToken = token;
}
```

Example `fetch` refresh request:

```ts
await fetch("http://localhost:8080/v1/auth/refresh", {
  method: "POST",
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({}),
});
```

---

## 3. Route map

### Diagnostics

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| GET | `/ping` | No | Liveness check |
| GET | `/health` | No | DB-backed health check |
| GET | `/goroutines` | No | Go runtime goroutine count |

### Auth

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| POST | `/v1/auth/register` | No | Register a new user |
| POST | `/v1/auth/login` | No | Login with email, username, or phone |
| POST | `/v1/auth/refresh` | No | Exchange refresh token for new tokens |
| GET | `/v1/auth/oauth/:provider/url` | No | Build Google/GitHub OAuth authorization URL |
| POST | `/v1/auth/oauth/:provider/exchange` | No | Exchange OAuth code for app auth |
| POST | `/v1/auth/logout` | Yes | Logout current session or a specific session |
| POST | `/v1/auth/logout-all` | Yes | Revoke all sessions |
| GET | `/v1/auth/sessions` | Yes | List active sessions |

### Users / Contacts

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| GET | `/v1/users/search` | Yes | Search users |
| GET | `/v1/users/contacts` | Yes | List contacts |
| POST | `/v1/users/contacts` | Yes | Add a contact |
| DELETE | `/v1/users/contacts/:contact_user_id` | Yes | Remove a contact |

### Conversations

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| POST | `/v1/conversations` | Yes | Create DM or group |
| GET | `/v1/conversations` | Yes | List conversations |
| GET | `/v1/conversations/:id` | Yes | Get one conversation |
| POST | `/v1/conversations/:id/participants` | Yes | Add participant to group |
| DELETE | `/v1/conversations/:id/participants/:user_id` | Yes | Remove participant |
| GET | `/v1/conversations/:id/participants` | Yes | List participants |
| POST | `/v1/conversations/:id/clear` | Yes | Clear conversation for current user |

### Messages (read-only over HTTP)

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| GET | `/v1/conversations/:id/messages` | Yes | Message history |
| GET | `/v1/messages/:id` | Yes | Get one message |

### Uploads / Attachments

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| POST | `/v1/uploads` | Yes | Upload a single file |
| POST | `/v1/uploads/bulk` | Yes | Upload multiple files |
| POST | `/v1/attachments` | Yes | Create attachment metadata |
| GET | `/v1/attachments/:id` | Yes | Get attachment details |
| POST | `/v1/attachments/:id/viewed` | Yes | Mark a view-once attachment as viewed |
| GET | `/v1/messages/:id/attachments` | Yes | Get all attachments for a message |

### Realtime

| Method | Path | Auth | Purpose |
|---|---|---:|---|
| GET | `/v1/ws` | Access token required | Websocket realtime chat |

---

## 4. Shared JSON shapes

These are the canonical JSON payloads you will see repeatedly.

## 4.1 Auth payloads

### `AuthPayload`

```json
{
  "user": {
    "id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "display_name": "Hasnain",
    "email": "hasnain@example.com",
    "username": "hasnain",
    "phone_number": "+923001112233",
    "avatar_url": "https://cdn.example.com/avatar.png",
    "is_verified": false
  },
  "session": {
    "id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f",
    "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "device": {
      "id": "59290eb5-e314-49ec-9d74-8f31e953c0cb",
      "device_id": "browser-chrome-macbook",
      "device_name": "Chrome on MacBook",
      "device_type": "web"
    },
    "created_at": "2026-01-01T10:00:00Z",
    "expires_at": "2026-01-15T10:00:00Z",
    "auth_provider": "password",
    "is_current": true
  },
  "tokens": {
    "access_token": "<jwt-access-token>",
    "token_type": "Bearer",
    "expires_in": 3600,
    "expires_at": "2026-01-01T11:00:00Z",
    "refresh_token_expires_at": "2026-01-15T10:00:00Z",
    "refresh_token_set": true
  },
  "auth_provider": "password",
  "is_new_user": false
}
```

Notes:

- `tokens.refresh_token` exists in the DTO definition but is intentionally removed from normal auth HTTP responses after the cookie is set.
- `tokens.refresh_token_set: true` tells you the backend successfully set or issued a refresh token.
- `session.device.id` is the server-side device row UUID.
- `session.device.device_id` is the client-supplied stable device identifier string.

## 4.2 User search / contact payloads

### `UserSearchView`

```json
{
  "id": "11111111-1111-1111-1111-111111111111",
  "display_name": "Ali",
  "username": "ali",
  "email": "ali@example.com",
  "avatar_url": "https://cdn.example.com/users/ali.png",
  "is_online": true,
  "is_contact": false,
  "is_blocked": false,
  "nickname": null
}
```

### `ContactView`

```json
{
  "id": "11111111-1111-1111-1111-111111111111",
  "display_name": "Ali",
  "username": "ali",
  "email": "ali@example.com",
  "avatar_url": "https://cdn.example.com/users/ali.png",
  "is_online": true,
  "is_blocked": false,
  "nickname": "Ali Work",
  "created_at": "2026-01-01T10:00:00Z",
  "last_seen_at": "2026-01-01T09:55:00Z"
}
```

## 4.3 Conversation payloads

### `ConversationView`

```json
{
  "id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "type": "DM",
  "subject": null,
  "description": null,
  "avatar_url": null,
  "disappearing_mode": "OFF",
  "created_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
  "created_at": "2026-01-01T10:00:00Z",
  "updated_at": "2026-01-01T10:15:00Z",
  "last_message_at": "2026-01-01T10:15:00Z",
  "participants": [
    {
      "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "display_name": "Hasnain",
      "username": "hasnain",
      "avatar_url": "https://cdn.example.com/u1.png",
      "role": "OWNER",
      "joined_at": "2026-01-01T10:00:00Z",
      "archived": false,
      "is_online": true,
      "last_read_sequence": 12
    },
    {
      "user_id": "11111111-1111-1111-1111-111111111111",
      "display_name": "Ali",
      "username": "ali",
      "avatar_url": "https://cdn.example.com/u2.png",
      "role": "MEMBER",
      "joined_at": "2026-01-01T10:00:00Z",
      "archived": false,
      "is_online": false,
      "last_read_sequence": 10
    }
  ],
  "last_message": {
    "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "sender_id": "11111111-1111-1111-1111-111111111111",
    "kind": "TEXT",
    "created_at": "2026-01-01T10:15:00Z",
    "seq_id": 12,
    "receipt_status": "READ"
  },
  "unread_count": 2,
  "last_read_sequence": 10
}
```

Notes:

- `type` is `DM` or `GROUP`.
- `disappearing_mode` is normalized by the backend to one of: `OFF`, `24_HOURS`, `7_DAYS`, `90_DAYS`.
- `last_message` is a summary, not the full message.
- `unread_count` and `last_read_sequence` are **current-user-specific** values.

## 4.4 Message payloads

### `MessageView`

```json
{
  "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sender_id": "11111111-1111-1111-1111-111111111111",
  "client_message_id": "web-1740000000-42",
  "seq_id": 12,
  "type": "TEXT",
  "content": "Hello from websocket",
  "is_forwarded": false,
  "reply_to_msg_id": null,
  "mention_count": 0,
  "created_at": "2026-01-01T10:15:00Z",
  "edited_at": null,
  "deleted_at": null,
  "expires_at": null,
  "attachments": [
    {
      "id": "1c0a9d3d-c94d-44ab-b01c-6501279c2f0b",
      "file_url": "https://cdn.example.com/chat/abc.png",
      "filename": "abc.png",
      "mime_type": "image/png",
      "size_bytes": 83021,
      "thumbnail_url": "https://cdn.example.com/chat/abc-thumb.png",
      "width": 1200,
      "height": 900,
      "duration_seconds": null,
      "view_once": false,
      "viewed_at": null
    }
  ],
  "receipts": [
    {
      "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "status": "READ",
      "delivered_at": "2026-01-01T10:15:03Z",
      "read_at": "2026-01-01T10:15:10Z",
      "played_at": null,
      "updated_at": "2026-01-01T10:15:10Z"
    }
  ],
  "reactions": [
    {
      "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "reaction_code": "❤️",
      "created_at": "2026-01-01T10:16:00Z"
    }
  ],
  "poll": null,
  "pinned": false,
  "is_starred": false
}
```

Notes:

- Canonical message types in backend models are `TEXT`, `AUDIO`, `FILE`, `POLL`, and `SYSTEM`.
- The backend currently only enforces that `type` is non-empty, but your frontend should stick to those canonical values.
- `client_message_id` is your best key for optimistic UI reconciliation.
- When a message is resent with the same `client_message_id` in the same conversation, the backend treats it idempotently and returns/broadcasts the existing message.

### `PollView`

```json
{
  "id": "4dc26a59-6a03-4d84-8dc3-f9f66cb83996",
  "question": "Which design should we ship?",
  "allows_multiple": false,
  "closes_at": "2026-01-07T10:00:00Z",
  "closed": false,
  "options": [
    {
      "id": "13b8a8eb-5972-4898-a653-1468b3409ac1",
      "text": "A",
      "position": 1,
      "votes": 2
    },
    {
      "id": "05af7a5a-2a85-4bc3-a303-82620dc5ab77",
      "text": "B",
      "position": 2,
      "votes": 1
    }
  ],
  "my_votes": [
    "13b8a8eb-5972-4898-a653-1468b3409ac1"
  ]
}
```

## 4.5 Upload / attachment payloads

### `UploadFilePayload`

```json
{
  "filename": "voice-note.webm",
  "mime_type": "audio/webm",
  "size_bytes": 142000,
  "object_key": "uploads/8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0/1740000000-voice-note.webm",
  "file_url": "https://cdn.example.com/uploads/.../voice-note.webm"
}
```

### `AttachmentPayload`

```json
{
  "id": "1c0a9d3d-c94d-44ab-b01c-6501279c2f0b",
  "uploader_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
  "file_url": "https://cdn.example.com/chat/abc.png",
  "filename": "abc.png",
  "mime_type": "image/png",
  "size_bytes": 83021,
  "view_once": false,
  "viewed_at": null,
  "thumbnail_url": "https://cdn.example.com/chat/abc-thumb.png",
  "width": 1200,
  "height": 900,
  "duration_seconds": null,
  "created_at": "2026-01-01T10:14:59Z"
}
```

### `AttachmentViewedPayload`

```json
{
  "attachment_id": "1c0a9d3d-c94d-44ab-b01c-6501279c2f0b",
  "viewed": true,
  "viewed_at": "2026-01-01T10:17:00Z"
}
```

## 4.6 Command payloads

### `CommandResult`

```json
{
  "command_id": "c4c4f677-f31c-4174-b457-192db545ba47",
  "type": "EDIT_MESSAGE",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "status": "UNDONE",
  "undone_at": "2026-01-01T10:30:00Z",
  "executed_at": "2026-01-01T10:20:00Z"
}
```

Command types currently supported by undo/redo logic are:

- `DELETE_MESSAGE`
- `EDIT_MESSAGE`
- `REACT_MESSAGE`
- `PIN_MESSAGE`
- `UNPIN_MESSAGE`
- `CLEAR_CHAT`

---

## 5. Auth API

## 5.1 Register

### `POST /v1/auth/register`

Auth required: **No**

Request body:

```json
{
  "display_name": "Hasnain",
  "email": "hasnain@example.com",
  "username": "hasnain",
  "phone_number": "+923001112233",
  "password": "supersecret123",
  "device": {
    "device_id": "browser-chrome-macbook",
    "device_name": "Chrome on MacBook",
    "device_type": "web"
  }
}
```

Rules:

- `display_name` is required.
- `password` is required and must be at least 8 characters.
- At least one of `email`, `username`, or `phone_number` must be present.
- If you include `device`, then `device.device_id` must be non-empty. Sending only `device_name` or `device_type` without `device_id` is rejected.

Success response: `201 Created`

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "display_name": "Hasnain",
      "email": "hasnain@example.com",
      "username": "hasnain",
      "phone_number": "+923001112233",
      "is_verified": false
    },
    "session": {
      "id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f",
      "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "device": {
        "id": "59290eb5-e314-49ec-9d74-8f31e953c0cb",
        "device_id": "browser-chrome-macbook",
        "device_name": "Chrome on MacBook",
        "device_type": "web"
      },
      "created_at": "2026-01-01T10:00:00Z",
      "expires_at": "2026-01-15T10:00:00Z",
      "auth_provider": "password",
      "is_current": true
    },
    "tokens": {
      "access_token": "<jwt-access-token>",
      "token_type": "Bearer",
      "expires_in": 3600,
      "expires_at": "2026-01-01T11:00:00Z",
      "refresh_token_expires_at": "2026-01-15T10:00:00Z",
      "refresh_token_set": true
    },
    "auth_provider": "password"
  }
}
```

Common error codes:

- `AUTH_INVALID_INPUT` → `400`
- `AUTH_IDENTIFIER_TAKEN` → `409`
- `SERVICE_UNAVAILABLE` → `503`

## 5.2 Login

### `POST /v1/auth/login`

Auth required: **No**

Request body:

```json
{
  "identifier": "hasnain@example.com",
  "password": "supersecret123",
  "device": {
    "device_id": "browser-chrome-macbook",
    "device_name": "Chrome on MacBook",
    "device_type": "web"
  }
}
```

Notes:

- `identifier` can be email, username, or phone number.
- Same `device` validation rules as register apply.

Success response: `200 OK`

Returns the same `AuthPayload` shape as register.

Common error codes:

- `AUTH_INVALID_INPUT` → `400`
- `AUTH_INVALID_CREDENTIALS` → `401`
- `SERVICE_UNAVAILABLE` → `503`

## 5.3 Refresh access token

### `POST /v1/auth/refresh`

Auth required: **No**

You can refresh in two ways.

### Option A: use the refresh cookie (recommended in browsers)

Request body can be empty:

```json
{}
```

Send the request with credentials included.

### Option B: send refresh token explicitly

```json
{
  "refresh_token": "<raw-refresh-token>"
}
```

Success response: `200 OK`

Returns the same `AuthPayload` shape as register/login.

Important frontend note:

- in a browser app, the common pattern is to rely on the HttpOnly cookie and call refresh with `credentials: "include"`
- the backend sets a **new** refresh cookie during refresh

Common error codes:

- `AUTH_INVALID_INPUT` → `400`
- `AUTH_UNAUTHORIZED` → `401`

## 5.4 Get OAuth authorization URL

### `GET /v1/auth/oauth/:provider/url`

Auth required: **No**

Supported providers:

- `google`
- `github`

Query params:

- `code_challenge` **required**
- `state` optional
- `redirect_uri` optional, but if supplied it must exactly match the backend provider redirect URI configuration

Example:

```http
GET /v1/auth/oauth/google/url?code_challenge=<pkce-s256-challenge>&state=xyz&redirect_uri=https://app.example.com/auth/google/callback
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "provider": "google",
    "authorization_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "redirect_uri": "https://app.example.com/auth/google/callback",
    "state": "xyz"
  }
}
```

Common error codes:

- `AUTH_INVALID_INPUT` → `400`
- `AUTH_OAUTH_PROVIDER_UNSUPPORTED` → `400`
- `SERVICE_UNAVAILABLE` → `503`

## 5.5 Exchange OAuth code

### `POST /v1/auth/oauth/:provider/exchange`

Auth required: **No**

Request body:

```json
{
  "code": "oauth-authorization-code",
  "code_verifier": "the-original-pkce-code-verifier",
  "redirect_uri": "https://app.example.com/auth/google/callback",
  "device": {
    "device_id": "browser-chrome-macbook",
    "device_name": "Chrome on MacBook",
    "device_type": "web"
  }
}
```

Success response: `200 OK`

Returns the same `AuthPayload` shape as login/register, with possible extra field:

```json
{
  "success": true,
  "data": {
    "auth_provider": "google",
    "is_new_user": true
  }
}
```

Common error codes:

- `AUTH_INVALID_INPUT` → `400`
- `AUTH_OAUTH_PROVIDER_UNSUPPORTED` → `400`
- `AUTH_OAUTH_EMAIL_UNVERIFIED` → `403`
- `SERVICE_UNAVAILABLE` → `503`

## 5.6 Logout current session or specific session

### `POST /v1/auth/logout`

Auth required: **Yes**

Body is optional.

Logout current session:

```json
{}
```

Logout a specific session:

```json
{
  "session_id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f"
}
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "revoked_session_id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f"
  }
}
```

Notes:

- If you revoke the current session, the backend clears the refresh cookie.
- If you revoke another one of your sessions, your current cookie stays intact.

## 5.7 Logout all sessions

### `POST /v1/auth/logout-all`

Auth required: **Yes**

Request body:

```json
{}
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "revoked_all": true
  }
}
```

Notes:

- refresh cookie is cleared
- all sessions for the current user are revoked

## 5.8 List sessions

### `GET /v1/auth/sessions`

Auth required: **Yes**

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f",
        "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
        "device": {
          "id": "59290eb5-e314-49ec-9d74-8f31e953c0cb",
          "device_id": "browser-chrome-macbook",
          "device_name": "Chrome on MacBook",
          "device_type": "web"
        },
        "created_at": "2026-01-01T10:00:00Z",
        "expires_at": "2026-01-15T10:00:00Z",
        "auth_provider": "password",
        "is_current": true
      }
    ]
  }
}
```

---

## 6. Users and contacts API

## 6.1 Search users

### `GET /v1/users/search`

Auth required: **Yes**

Query params:

- `query` optional string
- `page` optional integer, default `1`
- `limit` optional integer, default `15`, max `50`

Example:

```http
GET /v1/users/search?query=ali&page=1&limit=10
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "11111111-1111-1111-1111-111111111111",
        "display_name": "Ali",
        "username": "ali",
        "email": "ali@example.com",
        "avatar_url": "https://cdn.example.com/users/ali.png",
        "is_online": true,
        "is_contact": false,
        "is_blocked": false,
        "nickname": null
      }
    ],
    "total": 1
  }
}
```

Notes:

- the current user is filtered out of results
- `total` is based on the returned items after filtering and should not be treated as a guaranteed global DB total

## 6.2 List contacts

### `GET /v1/users/contacts`

Auth required: **Yes**

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "11111111-1111-1111-1111-111111111111",
        "display_name": "Ali",
        "username": "ali",
        "email": "ali@example.com",
        "avatar_url": "https://cdn.example.com/users/ali.png",
        "is_online": true,
        "is_blocked": false,
        "nickname": "Ali Work",
        "created_at": "2026-01-01T10:00:00Z",
        "last_seen_at": "2026-01-01T09:55:00Z"
      }
    ]
  }
}
```

## 6.3 Add contact

### `POST /v1/users/contacts`

Auth required: **Yes**

Request body:

```json
{
  "contact_user_id": "11111111-1111-1111-1111-111111111111",
  "nickname": "Ali Work"
}
```

Success response: `201 Created`

```json
{
  "success": true,
  "data": {
    "id": "11111111-1111-1111-1111-111111111111",
    "display_name": "Ali",
    "username": "ali",
    "email": "ali@example.com",
    "avatar_url": "https://cdn.example.com/users/ali.png",
    "is_online": true,
    "is_blocked": false,
    "nickname": "Ali Work",
    "created_at": "2026-01-01T10:00:00Z",
    "last_seen_at": "2026-01-01T09:55:00Z"
  }
}
```

## 6.4 Remove contact

### `DELETE /v1/users/contacts/:contact_user_id`

Auth required: **Yes**

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "removed": true
  }
}
```

Common user/contact error codes:

- `USER_INVALID_INPUT` → `400`
- `UNAUTHORIZED` → `401`
- `FORBIDDEN` → `403`
- `USER_NOT_FOUND` → `404`
- `USER_CONFLICT` → `409`
- `SERVICE_UNAVAILABLE` → `503`

---

## 7. Conversations API

## 7.1 Create conversation

### `POST /v1/conversations`

Auth required: **Yes**

Request body:

```json
{
  "type": "DM",
  "subject": "",
  "description": "",
  "avatar_url": "",
  "participant_ids": [
    "11111111-1111-1111-1111-111111111111"
  ],
  "disappearing_mode": "OFF"
}
```

Rules:

- `type` must be `DM` or `GROUP`.
- `participant_ids` is required and must contain valid UUIDs.
- For `DM`:
  - it must contain exactly one other user
  - you cannot create a DM with yourself
  - if a DM already exists between the two users, the backend returns that existing conversation instead of creating a duplicate
- For `GROUP`:
  - at least one participant ID is required
  - creator is automatically added as `OWNER`
- `disappearing_mode` is normalized to one of: `OFF`, `24_HOURS`, `7_DAYS`, `90_DAYS`

Example group request:

```json
{
  "type": "GROUP",
  "subject": "Design Team",
  "description": "Internal design chat",
  "avatar_url": "https://cdn.example.com/groups/design.png",
  "participant_ids": [
    "11111111-1111-1111-1111-111111111111",
    "22222222-2222-2222-2222-222222222222"
  ],
  "disappearing_mode": "7_DAYS"
}
```

Success response: `201 Created`

Returns a `ConversationView` wrapped in the normal success envelope.

## 7.2 List conversations

### `GET /v1/conversations`

Auth required: **Yes**

Query params:

- `page` optional integer, default `1`
- `limit` optional integer, default `50`, max `50`

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
        "type": "DM",
        "disappearing_mode": "OFF",
        "created_at": "2026-01-01T10:00:00Z",
        "updated_at": "2026-01-01T10:15:00Z",
        "last_message_at": "2026-01-01T10:15:00Z",
        "participants": [],
        "last_message": {
          "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
          "sender_id": "11111111-1111-1111-1111-111111111111",
          "kind": "TEXT",
          "created_at": "2026-01-01T10:15:00Z",
          "seq_id": 12,
          "receipt_status": "READ"
        },
        "unread_count": 2,
        "last_read_sequence": 10
      }
    ],
    "total": 1
  }
}
```

Sort order:

- conversations are sorted by latest message timestamp descending
- if a conversation has no messages yet, `created_at` is used as fallback

## 7.3 Get one conversation

### `GET /v1/conversations/:id`

Auth required: **Yes**

Success response: `200 OK`

Returns one `ConversationView`.

## 7.4 Add participant

### `POST /v1/conversations/:id/participants`

Auth required: **Yes**

Request body:

```json
{
  "user_id": "22222222-2222-2222-2222-222222222222",
  "role": "ADMIN"
}
```

Rules:

- only `GROUP` conversations can add participants
- actor must be `OWNER` or `ADMIN`
- `role` is normalized as:
  - `OWNER` → `OWNER`
  - `ADMIN` → `ADMIN`
  - anything else (or omitted) → `MEMBER`

Success response: `200 OK`

Returns the updated `ConversationView`.

## 7.5 Remove participant

### `DELETE /v1/conversations/:id/participants/:user_id`

Auth required: **Yes**

Rules:

- if removing another user, actor must be `OWNER` or `ADMIN`
- if removing yourself, being a participant is enough

Success response: `200 OK`

Returns the updated `ConversationView`.

## 7.6 List participants

### `GET /v1/conversations/:id/participants`

Auth required: **Yes**

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
        "display_name": "Hasnain",
        "username": "hasnain",
        "avatar_url": "https://cdn.example.com/u1.png",
        "role": "OWNER",
        "joined_at": "2026-01-01T10:00:00Z",
        "archived": false,
        "is_online": true,
        "last_read_sequence": 12
      }
    ]
  }
}
```

## 7.7 Clear conversation for current user

### `POST /v1/conversations/:id/clear`

Auth required: **Yes**

Request body:

```json
{}
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "cleared": true
  }
}
```

Important behavior:

- this is a **per-user clear**, not a global delete for everybody
- the related realtime event `conversation:cleared` is also **user-targeted**, not broadcast to the whole conversation
- clear operations are undoable through websocket `command:undo`

Common conversation error codes:

- `CONVERSATION_INVALID_INPUT` → `400`
- `UNAUTHORIZED` → `401`
- `FORBIDDEN` → `403`
- `CONVERSATION_NOT_FOUND` → `404`
- `CONVERSATION_CONFLICT` → `409`
- `SERVICE_UNAVAILABLE` → `503`

---

## 8. Messages API (HTTP read side)

> Important: this backend does **not** expose HTTP endpoints to create, edit, delete, react to, pin, or receipt messages. Those write operations happen over websocket.

## 8.1 Message history

### `GET /v1/conversations/:id/messages`

Auth required: **Yes**

Query params:

- `before_seq` optional integer
- `limit` optional integer, default `50`, max `50`

Example:

```http
GET /v1/conversations/c6490b75-d82d-4bdf-b783-6d39fbd4bca4/messages?before_seq=120&limit=20
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
        "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
        "sender_id": "11111111-1111-1111-1111-111111111111",
        "client_message_id": "web-1740000000-42",
        "seq_id": 12,
        "type": "TEXT",
        "content": "Hello from websocket",
        "is_forwarded": false,
        "reply_to_msg_id": null,
        "mention_count": 0,
        "created_at": "2026-01-01T10:15:00Z",
        "attachments": [],
        "receipts": [],
        "reactions": [],
        "pinned": false,
        "is_starred": false
      }
    ]
  }
}
```

Important ordering note:

- history is returned in **descending `seq_id` order** (newest first)
- most chat UIs want oldest → newest within the rendered batch, so you will usually reverse the array client-side before rendering

Important deletion note:

- history excludes messages where `deleted_at` is set
- realtime delete events still include the deleted message state

## 8.2 Get one message

### `GET /v1/messages/:id`

Auth required: **Yes**

Success response: `200 OK`

Returns one `MessageView`.

Common message HTTP error codes:

- `MESSAGE_INVALID_INPUT` → `400`
- `UNAUTHORIZED` → `401`
- `FORBIDDEN` → `403`
- `MESSAGE_NOT_FOUND` → `404`
- `MESSAGE_CONFLICT` → `409`
- `SERVICE_UNAVAILABLE` → `503`

---

## 9. Uploads and attachments API

## 9.1 Recommended frontend file flow

The intended frontend flow for file messages is:

1. Upload the raw file with `POST /v1/uploads` or `POST /v1/uploads/bulk`
2. Create an attachment record with `POST /v1/attachments`
3. Send the chat message over websocket using `message:send` with `attachment_ids`

This is the most important file-sharing flow to implement.

## 9.2 Upload a single file

### `POST /v1/uploads`

Auth required: **Yes**

Content type:

```text
multipart/form-data
```

Form fields:

- `file` → required file blob

Constraints:

- max size: **15 MB**
- MIME type must be a valid content type string, for example `image/png`, `audio/webm`, `application/pdf`
- upload storage (S3-compatible storage) must be configured on the backend; otherwise upload endpoints are unavailable

Example with `fetch`:

```ts
const form = new FormData();
form.append("file", file);

const res = await fetch("http://localhost:8080/v1/uploads", {
  method: "POST",
  credentials: "include",
  headers: {
    Authorization: `Bearer ${accessToken}`,
  },
  body: form,
});
```

Success response: `201 Created`

```json
{
  "success": true,
  "data": {
    "filename": "abc.png",
    "mime_type": "image/png",
    "size_bytes": 83021,
    "object_key": "uploads/.../abc.png",
    "file_url": "https://cdn.example.com/uploads/.../abc.png"
  }
}
```

Important note:

- `file_url` can be omitted if the backend storage is configured without a public base URL
- always keep `object_key` as fallback metadata

## 9.3 Upload multiple files

### `POST /v1/uploads/bulk`

Auth required: **Yes**

Content type:

```text
multipart/form-data
```

Form fields:

- preferred: multiple `files`
- fallback supported by backend: a single `file`

Constraints:

- minimum 1 file
- maximum 20 files
- same 15 MB per-file limit and MIME validation rules

Success response: `201 Created`

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "filename": "a.png",
        "mime_type": "image/png",
        "size_bytes": 83021,
        "object_key": "uploads/.../a.png",
        "file_url": "https://cdn.example.com/uploads/.../a.png"
      },
      {
        "filename": "b.pdf",
        "mime_type": "application/pdf",
        "size_bytes": 120000,
        "object_key": "uploads/.../b.pdf",
        "file_url": "https://cdn.example.com/uploads/.../b.pdf"
      }
    ]
  }
}
```

## 9.4 Create attachment metadata

### `POST /v1/attachments`

Auth required: **Yes**

Request body:

```json
{
  "file_url": "https://cdn.example.com/uploads/.../abc.png",
  "filename": "abc.png",
  "mime_type": "image/png",
  "size_bytes": 83021,
  "view_once": false,
  "thumbnail_url": "https://cdn.example.com/uploads/.../abc-thumb.png",
  "width": 1200,
  "height": 900,
  "duration_seconds": null
}
```

Optional `message_id` variant:

```json
{
  "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
  "file_url": "https://cdn.example.com/uploads/.../abc.png",
  "filename": "abc.png",
  "mime_type": "image/png",
  "size_bytes": 83021,
  "view_once": false
}
```

Rules:

- `file_url`, `filename`, `mime_type`, `size_bytes` are required
- `size_bytes` must be > 0 and <= 15 MB
- `mime_type` must be a valid media type
- if `message_id` is provided, that message must exist and must belong to the current user

Typical frontend usage:

- **normally omit** `message_id`
- create the attachment first
- then send websocket `message:send` with the returned attachment ID

Success response: `201 Created`

Returns an `AttachmentPayload`.

## 9.5 Get attachment

### `GET /v1/attachments/:id`

Auth required: **Yes**

Success response: `200 OK`

Returns an `AttachmentPayload`.

Access rule:

- the current user must be allowed to access the attachment through the linked message/conversation relationship

## 9.6 Mark attachment viewed

### `POST /v1/attachments/:id/viewed`

Auth required: **Yes**

Request body:

```json
{}
```

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "attachment_id": "1c0a9d3d-c94d-44ab-b01c-6501279c2f0b",
    "viewed": true,
    "viewed_at": "2026-01-01T10:17:00Z"
  }
}
```

Important note:

- this endpoint is mainly meaningful for `view_once: true` attachments
- for non-view-once attachments it is effectively a no-op

## 9.7 Get all attachments for a message

### `GET /v1/messages/:id/attachments`

Auth required: **Yes**

Success response: `200 OK`

```json
{
  "success": true,
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "attachments": [
      {
        "id": "1c0a9d3d-c94d-44ab-b01c-6501279c2f0b",
        "uploader_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
        "file_url": "https://cdn.example.com/chat/abc.png",
        "filename": "abc.png",
        "mime_type": "image/png",
        "size_bytes": 83021,
        "view_once": false,
        "created_at": "2026-01-01T10:14:59Z"
      }
    ]
  }
}
```

Common upload/attachment error codes:

- `INVALID_INPUT` → `400`
- `TOO_LARGE` → `413`
- `UNAUTHORIZED` → `401`
- `FORBIDDEN` → `403`
- `NOT_FOUND` → `404`
- `CONFLICT` → `409`
- `SERVICE_UNAVAILABLE` → `503`

---

## 10. Diagnostics endpoints

## 10.1 `GET /ping`

Success response:

```json
{
  "success": true,
  "data": {
    "message": "pong"
  }
}
```

## 10.2 `GET /health`

Healthy response:

```json
{
  "success": true,
  "data": {
    "status": "healthy"
  }
}
```

Unhealthy response:

```json
{
  "success": false,
  "error": "database error text here",
  "code": "UNHEALTHY"
}
```

## 10.3 `GET /goroutines`

Success response:

```json
{
  "goroutines": 37
}
```

---

## 11. WebSocket integration

## 11.1 This is plain websocket, not Socket.IO

Use something like this in the browser:

```ts
const ws = new WebSocket(`ws://localhost:8080/v1/ws?token=${accessToken}`);
```

Do **not** do this:

```ts
// Wrong for this backend
import { io } from "socket.io-client";
const socket = io("http://localhost:8080");
```

## 11.2 Browser connection example

```ts
export function connectChatSocket(accessToken: string) {
  const ws = new WebSocket(`ws://localhost:8080/v1/ws?token=${encodeURIComponent(accessToken)}`);

  ws.onopen = () => {
    console.log("socket connected");
  };

  ws.onmessage = (event) => {
    const envelope = JSON.parse(event.data);
    console.log("socket event", envelope.type, envelope);
  };

  ws.onclose = () => {
    console.log("socket closed");
  };

  ws.onerror = (err) => {
    console.error("socket error", err);
  };

  return ws;
}
```

## 11.3 Authentication rules

Websocket auth is handled by the same access token used for protected HTTP routes.

Accepted ways to authenticate:

### Browser-friendly way

```text
ws://localhost:8080/v1/ws?token=<access_token>
```

### Non-browser client way

Use `Authorization: Bearer <access_token>` during upgrade.

If auth fails, the server responds with HTTP `401` during the upgrade attempt.

## 11.4 Origin rules

Websocket origin checks allow:

- `http://localhost:3000`
- `http://127.0.0.1:3000`
- `https://localhost:3000`
- `https://127.0.0.1:3000`
- plus whatever exact origin is configured in `FRONTEND_URL`

## 11.5 First server frame

Immediately after a successful connection, the server sends:

```json
{
  "type": "connection:ready",
  "sent_at": "2026-01-01T10:00:00Z",
  "data": {
    "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "session_id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f",
    "device_id": "59290eb5-e314-49ec-9d74-8f31e953c0cb"
  }
}
```

## 11.6 Websocket frame shape you send to the backend

Every client → server frame uses this structure:

```json
{
  "type": "message:send",
  "request_id": "optional-client-generated-correlation-id",
  "conversation_id": "optional-conversation-uuid",
  "call_id": "optional-call-uuid",
  "data": {}
}
```

TypeScript shape:

```ts
type WsOutboundFrame = {
  type: string;
  request_id?: string;
  conversation_id?: string;
  call_id?: string;
  data?: any;
};
```

## 11.7 Websocket event envelope you receive from the backend

Every server → client event looks like this:

```json
{
  "type": "message:new",
  "request_id": "optional-request-id-echo",
  "user_id": "optional-routing-user-id",
  "device_id": "optional-routing-device-id",
  "conversation_id": "optional-conversation-id",
  "call_id": "optional-call-id",
  "source": "optional-internal-source-id",
  "sent_at": "2026-01-01T10:15:00Z",
  "data": {}
}
```

TypeScript shape:

```ts
type WsInboundEnvelope = {
  type: string;
  request_id?: string;
  user_id?: string;
  device_id?: string;
  conversation_id?: string;
  call_id?: string;
  source?: string;
  sent_at: string;
  data?: Record<string, any>;
};
```

### Very important metadata note

The top-level fields `user_id`, `device_id`, `conversation_id`, and `call_id` are primarily **routing metadata**.

For business meaning, prefer the `data` payload.

Examples:

- on `presence:update`, the actual user whose presence changed is `data.user_id`
- on `call:offer`, the sender is `data.from_user_id`
- on `message:new`, the actor is inside `data.message.sender_id`

## 11.8 Request correlation

You can send a `request_id` with any outbound frame.

The backend definitely echoes `request_id` in:

- websocket `error` events
- the caller's `call:incoming` echo after `call:start`
- `command:undone`
- `command:redone`

For message sending, the most reliable optimistic-UI correlation field is **`client_message_id` inside `data`**, not `request_id`.

## 11.9 Heartbeats

Two layers exist:

1. The websocket server sends protocol-level ping frames every 30 seconds.
2. The app protocol also supports sending this JSON frame:

```json
{
  "type": "ping"
}
```

Which returns:

```json
{
  "type": "pong",
  "sent_at": "2026-01-01T10:00:10Z"
}
```

---

## 12. WebSocket events you can emit from the frontend

This section answers the core question: **what should the frontend send?**

## 12.1 Typing start

### Send

```json
{
  "type": "typing:start",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4"
}
```

### Server behavior

- validates that you are a participant in the conversation
- broadcasts `typing:started` to the other participants

## 12.2 Typing stop

### Send

```json
{
  "type": "typing:stop",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4"
}
```

### Server behavior

- validates membership
- broadcasts `typing:stopped`

## 12.3 Send message

### Send

```json
{
  "type": "message:send",
  "request_id": "req-001",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "client_message_id": "web-1740000000-42",
    "type": "TEXT",
    "content": "Hello from websocket",
    "reply_to_msg_id": null,
    "expires_at": null,
    "attachment_ids": [],
    "mention_user_ids": []
  }
}
```

### Optional poll message variant

```json
{
  "type": "message:send",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "client_message_id": "web-1740000000-poll-1",
    "type": "POLL",
    "content": "Vote below",
    "poll": {
      "question": "Which design should we ship?",
      "allows_multiple": false,
      "closes_at": "2026-01-07T10:00:00Z",
      "options": ["A", "B", "C"]
    }
  }
}
```

### Validation/behavior

- `conversation_id` is required
- `data.type` is required
- at least one of these must exist:
  - non-empty `content`
  - non-empty `attachment_ids`
  - `poll`
- `reply_to_msg_id` must be a valid UUID if provided
- `expires_at` must be RFC3339 if provided
- `attachment_ids` must be an array of UUID strings if provided
- `mention_user_ids` must be an array of UUID strings if provided
- `poll.closes_at` must be RFC3339 if provided
- `poll.options` should contain at least 2 non-empty options
- if `client_message_id` already exists for that conversation, the backend returns/broadcasts the existing message instead of creating a duplicate

### Success event you should listen for

There is no dedicated `message:send:ack` event.

Instead, the sender receives the normal broadcast:

- `message:new`

That broadcast contains the authoritative `MessageView`.

## 12.4 Edit message

### Send

```json
{
  "type": "message:edit",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "content": "Edited message text",
    "expires_at": "2026-01-07T10:00:00Z"
  }
}
```

Rules:

- only the original sender can edit
- `content` must be non-empty
- results in `message:edited`
- edit actions are undoable

## 12.5 Delete message

### Send

```json
{
  "type": "message:delete",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0"
  }
}
```

Rules:

- only the original sender can delete
- results in `message:deleted`
- delete actions are undoable

## 12.6 Add reaction

### Send

```json
{
  "type": "message:reaction:add",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "reaction_code": "❤️"
  }
}
```

Results in:

- `message:reaction`

## 12.7 Remove reaction

### Send

```json
{
  "type": "message:reaction:remove",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "reaction_code": "❤️"
  }
}
```

Results in:

- `message:reaction`

## 12.8 Pin message

### Send

```json
{
  "type": "message:pin",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0"
  }
}
```

Results in:

- `message:pinned`

## 12.9 Unpin message

### Send

```json
{
  "type": "message:unpin",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0"
  }
}
```

Results in:

- `message:unpinned`

## 12.10 Delivered receipt

### Send

```json
{
  "type": "receipt:delivered",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_ids": [
      "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
      "43effa89-856b-4725-b59c-4f5815b03e51"
    ]
  }
}
```

Rules:

- `message_ids` must be an array of UUID strings
- your own messages are filtered out by the backend
- invalid/non-member messages are ignored or rejected depending on context

Results in:

- `receipt:update`

## 12.11 Read receipt

### Send

```json
{
  "type": "receipt:read",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_ids": [
      "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
      "43effa89-856b-4725-b59c-4f5815b03e51"
    ],
    "up_to_seq_id": 120
  }
}
```

Behavior:

- marks the given messages read
- updates your participant `last_read_sequence`
- if `up_to_seq_id` is greater than the max valid message `seq_id` in the batch, the backend uses your `up_to_seq_id`

Results in:

- `receipt:update`

## 12.12 Played receipt

### Send

```json
{
  "type": "receipt:played",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "message_ids": [
      "34616ddd-4f53-4b2b-a33d-7277d248f0e0"
    ]
  }
}
```

Important:

- the backend only supports **one playable message** at a time for `PLAYED`

Results in:

- `receipt:update`

## 12.13 Vote in poll

### Send

```json
{
  "type": "poll:vote",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "poll_id": "4dc26a59-6a03-4d84-8dc3-f9f66cb83996",
    "option_ids": [
      "13b8a8eb-5972-4898-a653-1468b3409ac1"
    ]
  }
}
```

Behavior:

- existing votes from that user are replaced
- if the poll does not allow multiple votes, sending more than one `option_id` is invalid
- closed polls cannot be voted on

Results in:

- `poll:update`

## 12.14 Close poll

### Send

```json
{
  "type": "poll:close",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "poll_id": "4dc26a59-6a03-4d84-8dc3-f9f66cb83996"
  }
}
```

Results in:

- `poll:update`

## 12.15 Undo latest command

### Send

Undo latest command globally for this user:

```json
{
  "type": "command:undo",
  "request_id": "undo-001"
}
```

Undo latest command scoped to one conversation:

```json
{
  "type": "command:undo",
  "request_id": "undo-002",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4"
}
```

Behavior:

- returns a direct `command:undone` event to the requester
- also triggers the actual state-change event such as:
  - `message:edited`
  - `message:deleted`
  - `message:reaction`
  - `message:pinned`
  - `message:unpinned`
  - `conversation:cleared`

## 12.16 Redo command

### Send

```json
{
  "type": "command:redo",
  "request_id": "redo-001",
  "data": {
    "command_id": "c4c4f677-f31c-4174-b457-192db545ba47"
  }
}
```

Behavior:

- returns direct `command:redone` to the requester
- also replays the corresponding state-change event

## 12.17 Start call

### Send

```json
{
  "type": "call:start",
  "request_id": "call-start-1",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "data": {
    "type": "VIDEO"
  }
}
```

Rules:

- calls currently only work for `DM` conversations
- call type must be `AUDIO` or `VIDEO`

Behavior:

- other participant(s) receive `call:incoming`
- the caller also receives a direct `call:incoming` echo with the same `request_id`

## 12.18 Send WebRTC offer

### Send

```json
{
  "type": "call:offer",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "data": {
    "to_user_id": "11111111-1111-1111-1111-111111111111",
    "sdp": "v=0...",
    "kind": "offer"
  }
}
```

Behavior:

- backend requires both users to be conversation participants
- backend forwards the entire `data` object to the target user inside `data.payload`
- target user receives `call:offer`

## 12.19 Send WebRTC answer

### Send

```json
{
  "type": "call:answer",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "data": {
    "to_user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "sdp": "v=0...",
    "kind": "answer"
  }
}
```

Behavior:

- marks the answering participant as connected in the backend
- receiver gets `call:answer`

## 12.20 Send ICE candidate

### Send

```json
{
  "type": "call:ice",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "data": {
    "to_user_id": "11111111-1111-1111-1111-111111111111",
    "candidate": "candidate:...",
    "sdpMid": "0",
    "sdpMLineIndex": 0
  }
}
```

Behavior:

- target user receives `call:ice`

## 12.21 End call

### Send

```json
{
  "type": "call:end",
  "request_id": "call-end-1",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "data": {
    "reason": "hangup"
  }
}
```

Important:

- `conversation_id` is **not required** for `call:end`
- `call_id` is required
- backend normalizes reason to uppercase internally for persistence, but emits your reason text back in realtime payloads

## 12.22 Ping

### Send

```json
{
  "type": "ping"
}
```

### Receive

```json
{
  "type": "pong",
  "sent_at": "2026-01-01T10:00:10Z"
}
```

---

## 13. WebSocket events you should listen for on the frontend

This section answers the other core question: **what can the server emit?**

## 13.1 `connection:ready`

```json
{
  "type": "connection:ready",
  "sent_at": "2026-01-01T10:00:00Z",
  "data": {
    "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "session_id": "8f6f4ad9-f8b6-4bc4-8828-c6bb6e5fcd4f",
    "device_id": "59290eb5-e314-49ec-9d74-8f31e953c0cb"
  }
}
```

Use this as your initial ready signal.

## 13.2 `presence:update`

```json
{
  "type": "presence:update",
  "sent_at": "2026-01-01T10:05:00Z",
  "data": {
    "user_id": "11111111-1111-1111-1111-111111111111",
    "is_online": false,
    "last_seen_at": "2026-01-01T10:05:00Z"
  }
}
```

Notes:

- sent to the user themselves and their contacts
- use `data.user_id`, not top-level `user_id`, to know whose presence changed
- `last_seen_at` appears when user goes offline

## 13.3 `typing:started`

```json
{
  "type": "typing:started",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:15:00Z",
  "data": {
    "user_id": "11111111-1111-1111-1111-111111111111"
  }
}
```

## 13.4 `typing:stopped`

```json
{
  "type": "typing:stopped",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:15:05Z",
  "data": {
    "user_id": "11111111-1111-1111-1111-111111111111"
  }
}
```

## 13.5 `conversation:created`

```json
{
  "type": "conversation:created",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:00:00Z",
  "data": {
    "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
    "created_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "type": "GROUP"
  }
}
```

Recommended frontend action:

- refresh or insert the conversation in your conversation list
- if you need full details, call `GET /v1/conversations/:id`

## 13.6 `conversation:participant_added`

```json
{
  "type": "conversation:participant_added",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:20:00Z",
  "data": {
    "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
    "user_id": "22222222-2222-2222-2222-222222222222",
    "added_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "role": "MEMBER"
  }
}
```

## 13.7 `conversation:participant_removed`

```json
{
  "type": "conversation:participant_removed",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:21:00Z",
  "data": {
    "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
    "user_id": "22222222-2222-2222-2222-222222222222",
    "removed_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0"
  }
}
```

## 13.8 `conversation:cleared`

```json
{
  "type": "conversation:cleared",
  "sent_at": "2026-01-01T10:25:00Z",
  "data": {
    "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
    "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "cleared": true,
    "cleared_at": "2026-01-01T10:25:00Z"
  }
}
```

Important:

- this is user-targeted and represents the current user's clear state
- it is not a global conversation wipe event

## 13.9 `message:new`

```json
{
  "type": "message:new",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:15:00Z",
  "data": {
    "message": {
      "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
      "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
      "sender_id": "11111111-1111-1111-1111-111111111111",
      "client_message_id": "web-1740000000-42",
      "seq_id": 12,
      "type": "TEXT",
      "content": "Hello from websocket",
      "created_at": "2026-01-01T10:15:00Z",
      "attachments": [],
      "receipts": [],
      "reactions": [],
      "pinned": false,
      "is_starred": false
    }
  }
}
```

Recommended frontend action:

- use `data.message.client_message_id` to reconcile optimistic placeholders
- append or insert by `seq_id`
- update the conversation preview / last message / unread count

## 13.10 `message:edited`

```json
{
  "type": "message:edited",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:20:00Z",
  "data": {
    "message": {
      "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
      "edited_at": "2026-01-01T10:20:00Z",
      "content": "Edited message text"
    }
  }
}
```

Backend actually sends the full `MessageView`; the snippet above is shortened for readability.

## 13.11 `message:deleted`

```json
{
  "type": "message:deleted",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:22:00Z",
  "data": {
    "message": {
      "id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
      "deleted_at": "2026-01-01T10:22:00Z"
    }
  }
}
```

Backend actually sends the full `MessageView`.

## 13.12 `message:reaction`

```json
{
  "type": "message:reaction",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:23:00Z",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "reactions": [
      {
        "user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
        "reaction_code": "❤️",
        "created_at": "2026-01-01T10:23:00Z"
      }
    ]
  }
}
```

## 13.13 `message:pinned`

```json
{
  "type": "message:pinned",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:24:00Z",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "pinned": true
  }
}
```

## 13.14 `message:unpinned`

```json
{
  "type": "message:unpinned",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:24:30Z",
  "data": {
    "message_id": "34616ddd-4f53-4b2b-a33d-7277d248f0e0",
    "pinned": false
  }
}
```

## 13.15 `receipt:update`

```json
{
  "type": "receipt:update",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:26:00Z",
  "data": {
    "message_ids": [
      "34616ddd-4f53-4b2b-a33d-7277d248f0e0"
    ],
    "user_id": "11111111-1111-1111-1111-111111111111",
    "status": "READ",
    "up_to_seq_id": 120
  }
}
```

Recommended frontend action:

- update receipt state for all listed messages
- if `up_to_seq_id` is present, you can also mark everything up to that sequence as read for that user

## 13.16 `poll:update`

```json
{
  "type": "poll:update",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "sent_at": "2026-01-01T10:27:00Z",
  "data": {
    "poll": {
      "id": "4dc26a59-6a03-4d84-8dc3-f9f66cb83996",
      "question": "Which design should we ship?",
      "allows_multiple": false,
      "closed": false,
      "options": [
        {
          "id": "13b8a8eb-5972-4898-a653-1468b3409ac1",
          "text": "A",
          "position": 1,
          "votes": 2
        }
      ],
      "my_votes": [
        "13b8a8eb-5972-4898-a653-1468b3409ac1"
      ]
    }
  }
}
```

## 13.17 `call:incoming`

### Event sent to callees

```json
{
  "type": "call:incoming",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:28:00Z",
  "data": {
    "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
    "type": "VIDEO",
    "initiated_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "participant_ids": [
      "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "11111111-1111-1111-1111-111111111111"
    ]
  }
}
```

### Event echoed back to the caller

```json
{
  "type": "call:incoming",
  "request_id": "call-start-1",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:28:00Z",
  "data": {
    "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
    "type": "VIDEO",
    "initiated_by": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "started_at": "2026-01-01T10:28:00Z",
    "participant_ids": [
      "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "11111111-1111-1111-1111-111111111111"
    ]
  }
}
```

Note:

- `started_at` is available on the caller echo and may be absent on callee delivery, so treat it as optional in frontend types.

## 13.18 `call:offer`

```json
{
  "type": "call:offer",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:28:10Z",
  "data": {
    "from_user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
    "payload": {
      "to_user_id": "11111111-1111-1111-1111-111111111111",
      "sdp": "v=0...",
      "kind": "offer"
    }
  }
}
```

## 13.19 `call:answer`

```json
{
  "type": "call:answer",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:28:20Z",
  "data": {
    "from_user_id": "11111111-1111-1111-1111-111111111111",
    "payload": {
      "to_user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "sdp": "v=0...",
      "kind": "answer"
    }
  }
}
```

## 13.20 `call:ice`

```json
{
  "type": "call:ice",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:28:25Z",
  "data": {
    "from_user_id": "11111111-1111-1111-1111-111111111111",
    "payload": {
      "to_user_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0",
      "candidate": "candidate:...",
      "sdpMid": "0",
      "sdpMLineIndex": 0
    }
  }
}
```

## 13.21 `call:ended`

```json
{
  "type": "call:ended",
  "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
  "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
  "sent_at": "2026-01-01T10:30:00Z",
  "data": {
    "call_id": "0c3e98c0-f3b3-499e-9fdd-cfbe90d1db19",
    "reason": "hangup",
    "actor_id": "8f4be5bd-3e45-49ff-bf0e-d9da23b24ea0"
  }
}
```

## 13.22 `command:undone`

```json
{
  "type": "command:undone",
  "request_id": "undo-002",
  "sent_at": "2026-01-01T10:31:00Z",
  "data": {
    "command": {
      "command_id": "c4c4f677-f31c-4174-b457-192db545ba47",
      "type": "EDIT_MESSAGE",
      "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
      "status": "UNDONE",
      "undone_at": "2026-01-01T10:31:00Z",
      "executed_at": "2026-01-01T10:20:00Z"
    }
  }
}
```

## 13.23 `command:redone`

```json
{
  "type": "command:redone",
  "request_id": "redo-001",
  "sent_at": "2026-01-01T10:32:00Z",
  "data": {
    "command": {
      "command_id": "c4c4f677-f31c-4174-b457-192db545ba47",
      "type": "EDIT_MESSAGE",
      "conversation_id": "c6490b75-d82d-4bdf-b783-6d39fbd4bca4",
      "status": "EXECUTED",
      "undone_at": null,
      "executed_at": "2026-01-01T10:32:00Z"
    }
  }
}
```

## 13.24 `error`

```json
{
  "type": "error",
  "request_id": "req-001",
  "sent_at": "2026-01-01T10:15:01Z",
  "data": {
    "code": "MESSAGE_SEND_FAILED",
    "message": "forbidden"
  }
}
```

The websocket `error` event is your universal negative-ack frame.

---

## 14. WebSocket error codes you should handle

These codes are emitted inside:

```json
{
  "type": "error",
  "data": {
    "code": "...",
    "message": "..."
  }
}
```

Common codes currently used by the websocket handler include:

- `INVALID_FRAME`
- `UNKNOWN_EVENT`
- `INVALID_CONVERSATION`
- `TYPING_FAILED`
- `INVALID_ATTACHMENTS`
- `INVALID_MENTIONS`
- `INVALID_REPLY`
- `INVALID_EXPIRY`
- `INVALID_POLL`
- `MESSAGE_SEND_FAILED`
- `INVALID_MESSAGE`
- `MESSAGE_EDIT_FAILED`
- `MESSAGE_DELETE_FAILED`
- `REACTION_FAILED`
- `PIN_FAILED`
- `INVALID_MESSAGE_IDS`
- `INVALID_SEQUENCE`
- `RECEIPT_FAILED`
- `INVALID_OPTIONS`
- `POLL_CLOSE_FAILED`
- `POLL_VOTE_FAILED`
- `INVALID_DATA`
- `CALL_START_FAILED`
- `INVALID_CALL`
- `CALL_END_FAILED`
- `INVALID_TARGET`
- `CALL_SIGNAL_FAILED`
- `COMMAND_UNDO_FAILED`
- `INVALID_COMMAND`
- `COMMAND_REDO_FAILED`

Frontend recommendation:

- if you included `request_id`, match the error back to the pending action
- otherwise fall back to action-specific optimistic rollback logic

---

## 15. End-to-end frontend flows

## 15.1 Login flow

1. `POST /v1/auth/login` with `credentials: "include"`
2. Store `data.tokens.access_token` in memory
3. Use that access token for:
   - protected HTTP routes
   - websocket connect query param `?token=`
4. When access token expires:
   - call `POST /v1/auth/refresh` with `credentials: "include"`
   - replace in-memory access token with the new one
5. On logout:
   - call `POST /v1/auth/logout`
   - clear local access token
   - close websocket

## 15.2 Open chat screen flow

1. `GET /v1/conversations?page=1&limit=50`
2. User picks a conversation
3. `GET /v1/conversations/:id/messages?limit=50`
4. Reverse messages client-side if your UI renders oldest first
5. Open websocket if not already open
6. Start listening for `message:new`, `message:edited`, `message:deleted`, `message:reaction`, `message:pinned`, `message:unpinned`, `receipt:update`, `typing:*`

## 15.3 Send text message flow

1. Generate `client_message_id` on frontend
2. Optimistically insert local pending message
3. Send websocket `message:send`
4. Wait for `message:new`
5. Match the returned message by `client_message_id`
6. Replace optimistic message with authoritative message payload

## 15.4 Send file message flow

1. `POST /v1/uploads` with multipart file
2. `POST /v1/attachments` using returned `file_url` / `filename` / `mime_type` / `size_bytes`
3. Send websocket `message:send` with:

```json
{
  "type": "FILE",
  "content": "optional caption",
  "attachment_ids": ["<attachment-id>"]
}
```

4. Wait for `message:new`

## 15.5 Typing indicator flow

1. When user starts typing, send `typing:start`
2. Debounce or throttle on frontend
3. When input clears, message sends, or user stops typing, send `typing:stop`
4. Render remote typing state from `typing:started` and `typing:stopped`

## 15.6 Read receipt flow

1. When messages become visible, gather unread message IDs from others
2. Send `receipt:read` with `message_ids`
3. Include `up_to_seq_id` if you track “read up to message X” in your UI
4. Update local UI when `receipt:update` arrives

## 15.7 Poll flow

1. Sender creates a poll message using `message:send` with `poll`
2. Everyone receives `message:new` containing `message.poll`
3. Voters send `poll:vote`
4. Everyone receives `poll:update`
5. Poll creator or participant can send `poll:close`
6. Everyone receives another `poll:update`

## 15.8 Call flow (WebRTC signaling)

1. Caller sends `call:start`
2. Callee receives `call:incoming`
3. Caller also receives `call:incoming` echo with `request_id`
4. Caller sends `call:offer`
5. Callee receives `call:offer`
6. Callee responds with `call:answer`
7. Caller receives `call:answer`
8. Both sides exchange `call:ice`
9. Either side can send `call:end`
10. Participants receive `call:ended`

Important note:

- the backend is a **signaling layer**, not a media relay
- your frontend still needs normal WebRTC peer connection handling

## 15.9 Undo / redo flow

1. User edits / deletes / reacts / pins / clears chat
2. Backend records the action as a command
3. User sends `command:undo`
4. Backend emits:
   - a direct `command:undone` result to the requester
   - the actual state change event to affected clients
5. User can later send `command:redo` with `command_id`
6. Backend emits:
   - `command:redone`
   - the actual redone state change event

---

## 16. Frontend implementation recommendations

## 16.1 Keep access token in memory

Because refresh token is already in an HttpOnly cookie, a common browser approach is:

- keep access token in memory
- refresh on app startup / 401 / token expiry
- do not persist refresh token in localStorage

## 16.2 Normalize incoming message updates

For best results, key messages by:

- primary key: `id`
- optimistic key: `client_message_id`

When `message:new` arrives:

- if it matches a pending optimistic message by `client_message_id`, replace that item
- otherwise insert by `seq_id`

## 16.3 Treat websocket events as source of truth for writes

There is no separate HTTP mutation API for chat actions.

So for chat state changes:

- websocket is the write channel
- websocket events are the authoritative write result channel
- HTTP is mostly for initial loading and recovery

## 16.4 Re-hydrate after reconnect

On websocket reconnect:

1. reconnect with a fresh access token if needed
2. call message history endpoints for active conversations if you think events were missed
3. re-send local presence/typing state only if appropriate for your UX

## 16.5 Reverse message history batches if needed

`GET /v1/conversations/:id/messages` returns newest-first.

Most chat UIs render oldest-first inside a batch, so do:

```ts
const items = response.data.data.items;
const ordered = [...items].reverse();
```

---

## 17. Minimal TypeScript helper types

```ts
export type ApiSuccess<T> = {
  success: true;
  data: T;
};

export type ApiError = {
  success: false;
  error: string;
  code: string;
};

export type ApiResponse<T> = ApiSuccess<T> | ApiError;

export type WsOutboundFrame = {
  type: string;
  request_id?: string;
  conversation_id?: string;
  call_id?: string;
  data?: any;
};

export type WsInboundEnvelope = {
  type: string;
  request_id?: string;
  user_id?: string;
  device_id?: string;
  conversation_id?: string;
  call_id?: string;
  source?: string;
  sent_at: string;
  data?: Record<string, any>;
};
```

---

## 18. Final integration checklist

Before calling your frontend integration complete, make sure you have all of these:

- [ ] use `Authorization: Bearer <access_token>` for protected HTTP routes
- [ ] use `withCredentials: true` / `credentials: "include"` for login/register/refresh/logout flows
- [ ] connect to websocket using plain `WebSocket`, not Socket.IO
- [ ] send websocket access token via query param in the browser
- [ ] handle `connection:ready`
- [ ] handle websocket `error`
- [ ] use `client_message_id` for optimistic message reconciliation
- [ ] reverse message history batches if your UI is oldest-first
- [ ] implement upload → attachment → message flow for file messages
- [ ] listen for `message:new`, `message:edited`, `message:deleted`, `message:reaction`, `message:pinned`, `message:unpinned`
- [ ] implement `receipt:update` handling
- [ ] implement `typing:started` and `typing:stopped`
- [ ] implement `presence:update`
- [ ] implement `poll:update`
- [ ] implement `call:incoming`, `call:offer`, `call:answer`, `call:ice`, `call:ended`
- [ ] implement `command:undone` and `command:redone` if your UX exposes undo/redo

---

## 19. One-sentence summary

If you only remember one thing: **load data over HTTP, mutate chat state over plain websocket, treat realtime events as the authoritative write result, and use `client_message_id` plus `request_id` for frontend correlation.**
