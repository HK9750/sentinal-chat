# API Endpoints

This document is a cleaned-up API build plan based on what is actually present in this repository:

- SQL schema in `migrations/*.sql`
- repository methods in `internal/repository/*.go`
- middleware expectations in `internal/middleware/*.go`
- server bootstrap in `internal/server/server.go`

This file intentionally does not include encryption key endpoints or broadcast endpoints, because those tables and repositories do not exist in the current repo.

## 1. Scope of this file

This file covers only these backend areas:

- utility routes
- auth and sessions
- users, contacts, devices, push tokens
- conversations and participants
- messages and message-related actions
- attachments and uploads
- polls
- calls
- commands

Not included on purpose:

- encryption key endpoints
- broadcast list endpoints

Reason:

- there is no backing SQL schema for those areas in the current repository
- there are no repositories for those areas in the current repository
- documenting them here as real API work would be misleading

## 2. Current backend reality

### Actually implemented right now

Only these routes are currently registered by the Gin server:

- `GET /ping`
- `GET /health`
- `GET /goroutines`

Everything else in this file is the API surface that should be built next because the schema and repositories already support it.

## 3. Global API rules

### Base path

- application routes should live under `/v1`
- diagnostic routes can remain at root

### Success response shape

```json
{
  "success": true,
  "data": {}
}
```

### Error response shape

```json
{
  "success": false,
  "error": "human readable message",
  "code": "ERROR_CODE"
}
```

### Authentication

- protected routes should use `Authorization: Bearer <access_token>`
- auth middleware currently expects JWT parsing to provide:
  - `user_id`
  - `session_id`
  - `device_id`

### Common input rules

- all ids are UUID strings
- timestamps should be ISO-8601 UTC strings
- pagination should use `page` and `limit`
- booleans should be explicit, not string values
- nullable fields should be omitted or set to `null`

### Common service rules

For every write endpoint, the service layer should decide:

- whether current user is allowed to perform the action
- whether the target row exists
- whether the request conflicts with unique constraints
- whether an outbox event should be written in the same transaction
- whether a command log should be written for undoable chat actions

---

## 4. Utility Endpoints

## 4.1 GET `/ping`

- Current status: implemented
- Auth required: no
- Purpose:
  - cheap liveness check
  - useful for load balancers and smoke checks
- Request body: none
- Query params: none
- Success response:

```json
{
  "success": true,
  "data": {
    "message": "pong"
  }
}
```

## 4.2 GET `/health`

- Current status: implemented
- Auth required: no
- Purpose:
  - verifies database connectivity
  - should be used for readiness monitoring
- Request body: none
- Query params: none
- Success response:

```json
{
  "success": true,
  "data": {
    "status": "healthy"
  }
}
```

- Failure response:

```json
{
  "success": false,
  "error": "database error text",
  "code": "UNHEALTHY"
}
```

## 4.3 GET `/goroutines`

- Current status: implemented
- Auth required: no
- Purpose:
  - runtime debugging
  - returns current goroutine count
- Success response example:

```json
{
  "goroutines": 42
}
```

---

## 5. Auth Endpoints

These routes are necessary because the repo already has users, devices, sessions, JWT config, and auth middleware.

## 5.1 POST `/v1/auth/register`

- Auth required: no
- Rate limit: yes, IP-based
- Main purpose:
  - create a new user account
  - optionally register the first device
  - create the initial refresh session
  - return login tokens

### Request body

```json
{
  "display_name": "Hasnain",
  "email": "hasnain@example.com",
  "username": "hasnain",
  "phone_number": "+923001112233",
  "password": "plain-text-password",
  "device": {
    "device_id": "web-123",
    "device_name": "Chrome on Linux",
    "device_type": "web"
  }
}
```

### Field notes

- `display_name`: required
- `email`: optional but recommended
- `username`: optional but recommended
- `phone_number`: optional
- `password`: required; must be hashed before storing
- `device`: optional, but useful for multi-device session tracking

### Backend actions

- validate uniqueness of `email`, `username`, and `phone_number`
- hash the incoming password
- create a row in `users`
- if device payload is present, create a row in `devices`
- create a row in `user_sessions` with hashed refresh token
- return access token and refresh token

### Tables touched

- `users`
- `devices`
- `user_sessions`

### Important DB constraints

- `email`, `username`, and `phone_number` are `CITEXT UNIQUE`
- uniqueness is case-insensitive
- `(user_id, device_id)` in `devices` must be unique

### Success response example

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "uuid",
      "display_name": "Hasnain",
      "email": "hasnain@example.com",
      "username": "hasnain",
      "phone_number": "+923001112233"
    },
    "access_token": "jwt",
    "refresh_token": "opaque-refresh-token"
  }
}
```

## 5.2 POST `/v1/auth/login`

- Auth required: no
- Rate limit: yes, IP-based
- Main purpose:
  - authenticate existing user
  - create a new session
  - update or create device info

### Request body

```json
{
  "identifier": "hasnain@example.com",
  "password": "plain-text-password",
  "device": {
    "device_id": "web-123",
    "device_name": "Chrome on Linux",
    "device_type": "web"
  }
}
```

### Field notes

- `identifier` should accept email, username, or phone number
- `device` should be used to map the login session to a physical or logical client

### Backend actions

- find user by identifier
- verify password hash
- if provided, add or update device information
- create new refresh session
- optionally update `users.last_seen_at`
- optionally set `users.is_online = true`

### Tables touched

- `users`
- `devices`
- `user_sessions`

## 5.3 POST `/v1/auth/refresh`

- Auth required: no
- Rate limit: yes, IP-based
- Main purpose:
  - rotate access token and refresh token pair

### Request body

```json
{
  "refresh_token": "opaque-refresh-token"
}
```

### Backend actions

- hash refresh token
- find matching valid session
- ensure session is not revoked
- ensure session is not expired
- issue new access token
- rotate refresh token hash in `user_sessions`

### Tables touched

- `user_sessions`

## 5.4 POST `/v1/auth/logout`

- Auth required: yes
- Main purpose:
  - revoke the current session or a specific session

### Request body

```json
{
  "session_id": "uuid-optional"
}
```

### Behavior

- if `session_id` is omitted, revoke current session from auth context
- if `session_id` is provided, only revoke that session if it belongs to current user

### Tables touched

- `user_sessions`

## 5.5 POST `/v1/auth/logout-all`

- Auth required: yes
- Main purpose:
  - revoke every session of current user

### Backend actions

- set `is_revoked = true` for all rows in `user_sessions` for current user

## 5.6 GET `/v1/auth/sessions`

- Auth required: yes
- Main purpose:
  - list all sessions of current user
  - include device information if available

### Query params

- none required

### Response should include

- session id
- user id
- device id
- expiry time
- revoked state
- created time
- optional device name and type

## 5.7 POST `/v1/auth/oauth/:provider/exchange`

- Auth required: no
- Rate limit: yes, IP-based
- Main purpose:
  - exchange OAuth authorization code for first-party app session
  - link provider identity to user
  - return app access token and session metadata

### Path params

- `provider`: `google` or `github`

### Request body

```json
{
  "code": "provider-oauth-code",
  "code_verifier": "pkce-code-verifier",
  "redirect_uri": "https://app.example.com/auth/callback/google",
  "device": {
    "device_id": "web-123",
    "device_name": "Chrome on Linux",
    "device_type": "web"
  }
}
```

### Backend actions

- validate provider and PKCE payload
- exchange code with provider token endpoint
- fetch provider identity profile
- find existing `oauth_identities(provider, provider_user_id)` mapping
- if no mapping:
  - require verified email
  - find or create user by email
  - create identity mapping
- upsert device (if provided)
- create `user_sessions` row with hashed refresh token
- issue first-party JWT access token

### Tables touched

- `oauth_identities`
- `users`
- `devices`
- `user_sessions`

---

## 6. User Endpoints

The repo has a strong user repository, so these should exist.

## 6.1 GET `/v1/users/me`

- Auth required: yes
- Main purpose:
  - fetch current user's own profile

### Reads from

- `users`

### Response fields

- `id`
- `phone_number`
- `username`
- `email`
- `display_name`
- `bio`
- `avatar_url`
- `is_online`
- `last_seen_at`
- `is_active`
- `is_verified`
- `created_at`
- `updated_at`

## 6.2 PUT `/v1/users/me`

- Auth required: yes
- Main purpose:
  - update current user's profile

### Request body

```json
{
  "display_name": "Hasnain Ali",
  "bio": "Builder",
  "avatar_url": "https://cdn.example.com/avatar.jpg",
  "email": "hasnain@example.com",
  "username": "hasnain",
  "phone_number": "+923001112233"
}
```

### Backend actions

- load current user
- patch editable fields
- enforce uniqueness for email, username, phone number
- update row in `users`

## 6.3 GET `/v1/users/:id`

- Auth required: yes
- Main purpose:
  - fetch another user by id for contacts, DM creation, mentions, and participant rendering

## 6.4 GET `/v1/users/search?q=<term>&page=1&limit=20`

- Auth required: yes
- Main purpose:
  - user discovery

### Search behavior from repository

- matches against:
  - `display_name`
  - `username`
  - `email`

### Response should include

- result list
- total count
- page
- limit

## 6.5 GET `/v1/users/contacts`

- Auth required: yes
- Main purpose:
  - list current user's contacts

### Reads from

- `user_contacts`

### Suggested response fields

- `contact_user_id`
- `nickname`
- `is_blocked`
- `created_at`
- joined profile summary for the contact user

## 6.6 POST `/v1/users/contacts`

- Auth required: yes
- Main purpose:
  - add a user as contact

### Request body

```json
{
  "contact_user_id": "uuid",
  "nickname": "Work"
}
```

### Backend actions

- verify target user exists
- insert into `user_contacts`

## 6.7 DELETE `/v1/users/contacts/:contact_user_id`

- Auth required: yes
- Main purpose:
  - remove a contact entry

## 6.8 POST `/v1/users/contacts/:contact_user_id/block`

- Auth required: yes
- Main purpose:
  - block a contact

### Backend behavior from repository

- if contact row exists, set `is_blocked = true`
- if contact row does not exist, create it as blocked

## 6.9 POST `/v1/users/contacts/:contact_user_id/unblock`

- Auth required: yes
- Main purpose:
  - unblock a contact

## 6.10 GET `/v1/users/contacts/blocked`

- Auth required: yes
- Main purpose:
  - list blocked contacts only

## 6.11 POST `/v1/users/devices`

- Auth required: yes
- Main purpose:
  - register a device for current user

### Request body

```json
{
  "device_id": "ios-abc-123",
  "device_name": "Hasnain's iPhone",
  "device_type": "ios"
}
```

### Backend actions

- create device row
- set `is_active = true`
- set `registered_at`

### Tables touched

- `devices`

## 6.12 GET `/v1/users/devices`

- Auth required: yes
- Main purpose:
  - list all devices for current user

## 6.13 DELETE `/v1/users/devices/:device_uuid`

- Auth required: yes
- Main purpose:
  - deactivate a device instead of physically deleting it

### Backend behavior

- set `devices.is_active = false`

## 6.14 POST `/v1/users/devices/:device_uuid/push-tokens`

- Auth required: yes
- Main purpose:
  - attach an FCM token to a device

### Request body

```json
{
  "platform": "android",
  "token": "push-token"
}
```

### Tables touched

- `fcm_tokens`

## 6.15 GET `/v1/users/push-tokens`

- Auth required: yes
- Main purpose:
  - list push tokens for current user

## 6.16 DELETE `/v1/users/push-tokens/:token_id`

- Auth required: yes
- Main purpose:
  - deactivate a push token

---

## 7. Conversation Endpoints

The conversation repository and schema are strong enough that this should be one of the first major handler sets you build.

## 7.1 POST `/v1/conversations`

- Auth required: yes
- Main purpose:
  - create a DM or group conversation

### Request body for DM

```json
{
  "type": "DM",
  "participant_ids": ["other-user-uuid"]
}
```

### Request body for group

```json
{
  "type": "GROUP",
  "subject": "Core Team",
  "description": "Planning room",
  "avatar_url": "https://cdn.example.com/group.png",
  "disappearing_mode": "OFF",
  "participant_ids": ["uuid-1", "uuid-2", "uuid-3"]
}
```

### Important fields

- `type`: must be `DM` or `GROUP`
- `subject`: mainly for groups
- `description`: mainly for groups
- `avatar_url`: optional
- `disappearing_mode`: one of:
  - `OFF`
  - `24_HOURS`
  - `7_DAYS`
  - `90_DAYS`
- `participant_ids`: target user ids to include in the conversation

### Backend actions

- create row in `conversations`
- for DM:
  - set `dm_user_id_a` and `dm_user_id_b`
  - DB trigger normalizes pair ordering automatically
  - DB unique index prevents duplicate DM per user pair
- insert creator and target users into `participants`
- optionally create initial outbox event such as `conversation:created`

### Tables touched

- `conversations`
- `participants`

### Important DB behavior

- DM pair normalization is enforced by trigger
- duplicate DM pairs are prevented by unique partial index

## 7.2 GET `/v1/conversations?page=1&limit=20`

- Auth required: yes
- Main purpose:
  - list all conversations of current user

### Backend behavior from repository

- fetches conversations where current user exists in `participants`
- computes `last_message_at` using a subquery against `messages`
- sorts by latest message activity first
- loads participant list for each conversation

### Response should include

- conversation metadata
- participant summaries
- last message time
- pagination info

## 7.3 GET `/v1/conversations/:id`

- Auth required: yes
- Main purpose:
  - fetch one conversation with participant list

### Important service rule

- before returning, verify current user is a participant

## 7.4 PUT `/v1/conversations/:id`

- Auth required: yes
- Main purpose:
  - update conversation metadata

### Request body

```json
{
  "subject": "New name",
  "description": "New description",
  "avatar_url": "https://cdn.example.com/new.png",
  "disappearing_mode": "24_HOURS"
}
```

### Backend actions

- load conversation
- patch mutable fields
- update row
- optionally create outbox event

## 7.5 DELETE `/v1/conversations/:id`

- Auth required: yes
- Main purpose:
  - permanently delete a conversation

### Important note

- current schema uses hard delete here
- related rows cascade because of foreign keys
- in many chat products this should be restricted or replaced with leave/archive behavior

## 7.6 GET `/v1/conversations/direct?user_id=<uuid>`

- Auth required: yes
- Main purpose:
  - fetch the DM shared by current user and another user

### Backend behavior

- repository queries the normalized DM pair directly

## 7.7 GET `/v1/conversations/search?q=<text>`

- Auth required: yes
- Main purpose:
  - search current user's conversations by subject

### Important limitation

- current repository search only matches `subject`
- that means this endpoint is mostly useful for group conversations

## 7.8 GET `/v1/conversations/type?type=DM|GROUP`

- Auth required: yes
- Main purpose:
  - filter conversations by type

## 7.9 GET `/v1/conversations/invite?link=<invite_link>`

- Auth required: yes
- Main purpose:
  - resolve an invite link to a conversation

### Backend behavior

- repository only returns rows where:
  - `invite_link = provided_link`
  - and `invite_link_revoked_at IS NULL OR invite_link_revoked_at > NOW()`

## 7.10 POST `/v1/conversations/:id/invite`

- Auth required: yes
- Main purpose:
  - regenerate invite link

### Backend actions

- generate new UUID string as invite link
- set `invite_link`
- clear `invite_link_revoked_at`

### Response example

```json
{
  "success": true,
  "data": {
    "invite_link": "new-uuid-link"
  }
}
```

## 7.11 POST `/v1/conversations/:id/participants`

- Auth required: yes
- Main purpose:
  - add user to conversation

### Request body

```json
{
  "user_id": "uuid",
  "role": "MEMBER"
}
```

### Backend actions

- verify current user has permission to add participants
- verify target user exists
- insert into `participants`

## 7.12 DELETE `/v1/conversations/:id/participants/:user_id`

- Auth required: yes
- Main purpose:
  - remove a participant from conversation

## 7.13 GET `/v1/conversations/:id/participants`

- Auth required: yes
- Main purpose:
  - list participants

### Joined profile fields already available from repository

- `display_name`
- `username`
- `avatar_url`
- `is_online`

## 7.14 PUT `/v1/conversations/:id/participants/:user_id/role`

- Auth required: yes
- Main purpose:
  - change participant role

### Request body

```json
{
  "role": "ADMIN"
}
```

## 7.15 POST `/v1/conversations/:id/mute`

- Auth required: yes
- Main purpose:
  - mute a conversation for current user

### Request body

```json
{
  "until": "2026-03-11T15:00:00Z"
}
```

### Backend behavior

- update `participants.muted_until`

## 7.16 POST `/v1/conversations/:id/unmute`

- Auth required: yes
- Main purpose:
  - clear mute state for current user

## 7.17 POST `/v1/conversations/:id/archive`

- Auth required: yes
- Main purpose:
  - archive conversation for current user only

### Backend behavior

- update `participants.archived = true`

## 7.18 POST `/v1/conversations/:id/unarchive`

- Auth required: yes
- Main purpose:
  - unarchive conversation for current user only

## 7.19 POST `/v1/conversations/:id/read-sequence`

- Auth required: yes
- Main purpose:
  - update current user's highest read sequence

### Request body

```json
{
  "seq_id": 144
}
```

### Backend behavior

- update `participants.last_read_sequence`
- should also emit realtime read progress event

## 7.20 GET `/v1/conversations/:id/sequence`

- Auth required: yes
- Main purpose:
  - get conversation sequence state from `conversation_sequences`

## 7.21 POST `/v1/conversations/:id/clear`

- Auth required: yes
- Main purpose:
  - clear chat history for the current user without deleting global messages

### Backend behavior from repository

- upsert row in `conversation_clears`
- update `cleared_at = NOW()` on repeat clears

### Why this endpoint matters

- schema already supports per-user clear chat
- SQL comments mention `CLEAR_CHAT` as a command type

---

## 8. Message Endpoints

Messages are the most important API area in the project.

## 8.1 POST `/v1/messages`

- Auth required: yes
- Rate limit: yes, user-based
- Main purpose:
  - create a message inside a conversation

### Request body

```json
{
  "conversation_id": "uuid",
  "client_message_id": "client-idempotency-key",
  "type": "TEXT",
  "encrypted_content": "base64-ciphertext",
  "is_forwarded": false,
  "reply_to_msg_id": null,
  "expires_at": null,
  "mentions": [
    {
      "user_id": "uuid",
      "offset": 0,
      "length": 8
    }
  ],
  "attachment_ids": ["uuid-1", "uuid-2"],
  "poll": null
}
```

### Important fields

- `conversation_id`: required
- `client_message_id`: optional but strongly recommended for idempotency
- `type`: one of:
  - `TEXT`
  - `IMAGE`
  - `VIDEO`
  - `AUDIO`
  - `FILE`
  - `GIF`
  - `EMOJI`
  - `POLL`
  - `SYSTEM`
- `encrypted_content`: opaque encrypted payload stored by server as text
- `reply_to_msg_id`: optional reply link
- `expires_at`: optional disappearing message deadline
- `mentions`: optional mention metadata
- `attachment_ids`: optional pre-created attachments
- `poll`: optional poll definition if `type = POLL`

### Backend actions

- verify current user is participant in conversation
- insert message row
- let DB trigger assign `seq_id`
- add mentions if provided
- link attachments if provided
- create poll and poll options if needed
- create outbox event for fan-out

### Tables touched

- `messages`
- `message_mentions`
- `message_attachments`
- `polls`
- `poll_options`
- `outbox_events`

### Important DB constraints

- `(conversation_id, client_message_id)` must be unique
- `seq_id` is assigned by DB trigger, not by handler logic

## 8.2 GET `/v1/messages?conversation_id=<uuid>&before_seq=<n>&limit=50`

- Auth required: yes
- Main purpose:
  - load message history for a conversation

### Query params

- `conversation_id`: required
- `before_seq`: optional; if present, load older messages only
- `limit`: required or defaulted by server

### Backend behavior from repository

- only returns messages with `deleted_at IS NULL`
- sorts by `seq_id DESC`

## 8.3 GET `/v1/messages/:id`

- Auth required: yes
- Main purpose:
  - fetch one message by id

### Note

- repo supports this cleanly
- product response should likely include attachments, mentions, receipts, and reactions by composition in service layer

## 8.4 PUT `/v1/messages/:id`

- Auth required: yes
- Main purpose:
  - edit a message

### Request body

```json
{
  "encrypted_content": "new-base64-ciphertext",
  "expires_at": null
}
```

### Backend actions that should happen together

- verify current user is allowed to edit the message
- create row in `message_edits` holding previous encrypted content
- update `messages.encrypted_content`
- update `messages.edited_at`
- create command log for `EDIT_MESSAGE`
- create outbox event

## 8.5 DELETE `/v1/messages/:id`

- Auth required: yes
- Main purpose:
  - soft delete a message

### Backend behavior

- update `messages.deleted_at = now`
- create command log for `DELETE_MESSAGE`
- create outbox event

## 8.6 DELETE `/v1/messages/:id/hard`

- Auth required: yes
- Main purpose:
  - permanently delete a message row

### Important note

- should probably be restricted to admins, system tasks, or moderation flows
- hard delete removes auditability compared to soft delete

## 8.7 POST `/v1/messages/:id/delivered`

- Auth required: yes
- Main purpose:
  - mark message delivered for current user

### Backend behavior from repository

- if receipt exists, update it to `DELIVERED`
- if receipt does not exist, create it

### Tables touched

- `message_receipts`

## 8.8 POST `/v1/messages/:id/read`

- Auth required: yes
- Main purpose:
  - mark message read for current user

### Backend behavior from repository

- if receipt exists, update it to `READ`
- if receipt does not exist, create it
- service should also consider updating `participants.last_read_sequence`

## 8.9 POST `/v1/messages/:id/played`

- Auth required: yes
- Main purpose:
  - mark media message as played

### Why include this endpoint

- repository already supports `MarkAsPlayed`
- schema already supports `PLAYED` in delivery lifecycle

## 8.10 POST `/v1/messages/bulk-delivered`

- Auth required: yes
- Main purpose:
  - bulk delivery acknowledgement for mobile sync or reconnect flows

### Request body

```json
{
  "message_ids": ["uuid-1", "uuid-2", "uuid-3"]
}
```

## 8.11 POST `/v1/messages/bulk-read`

- Auth required: yes
- Main purpose:
  - bulk read acknowledgement for current user

### Request body

```json
{
  "message_ids": ["uuid-1", "uuid-2", "uuid-3"]
}
```

---

## 9. Message Feature Endpoints

These are all backed by existing schema and repository methods.

## 9.1 POST `/v1/messages/:id/reactions`

- Auth required: yes
- Main purpose:
  - add reaction to a message

### Request body

```json
{
  "reaction_code": ":thumbsup:"
}
```

### Backend actions

- create row in `message_reactions`
- optionally create command log for `REACT_MESSAGE`
- create outbox event for realtime fan-out

## 9.2 DELETE `/v1/messages/:id/reactions/:reaction_code`

- Auth required: yes
- Main purpose:
  - remove current user's reaction from a message

## 9.3 GET `/v1/messages/:id/reactions`

- Auth required: yes
- Main purpose:
  - list all reactions for a message

## 9.4 POST `/v1/messages/:id/star`

- Auth required: yes
- Main purpose:
  - star a message for the current user

### Tables touched

- `starred_messages`

## 9.5 DELETE `/v1/messages/:id/star`

- Auth required: yes
- Main purpose:
  - remove star from a message for the current user

## 9.6 GET `/v1/messages/starred?page=1&limit=20`

- Auth required: yes
- Main purpose:
  - list current user's starred messages

## 9.7 POST `/v1/messages/:id/pin`

- Auth required: yes
- Main purpose:
  - pin a message inside a conversation

### Request body

```json
{
  "conversation_id": "uuid"
}
```

### Backend actions

- insert into `pinned_messages`
- create command log for `PIN_MESSAGE`
- create outbox event

## 9.8 DELETE `/v1/messages/:id/pin?conversation_id=<uuid>`

- Auth required: yes
- Main purpose:
  - unpin a message from a conversation

### Backend actions

- delete from `pinned_messages`
- create command log for `UNPIN_MESSAGE`
- create outbox event

## 9.9 GET `/v1/conversations/:id/pinned-messages`

- Auth required: yes
- Main purpose:
  - list pinned messages for one conversation

## 9.10 GET `/v1/messages/:id/edits`

- Auth required: yes
- Main purpose:
  - list edit history of a message

## 9.11 GET `/v1/messages/:id/receipts`

- Auth required: yes
- Main purpose:
  - list receipt rows for a message

## 9.12 GET `/v1/users/me/mentions?page=1&limit=20`

- Auth required: yes
- Main purpose:
  - list messages where current user was mentioned

## 9.13 GET `/v1/conversations/:id/messages/by-type?type=IMAGE&limit=50`

- Auth required: yes
- Main purpose:
  - filter recent messages in one conversation by type

## 9.14 GET `/v1/conversations/:id/messages/range?start_seq=1&end_seq=100`

- Auth required: yes
- Main purpose:
  - fetch an inclusive sequence range

## 9.15 GET `/v1/conversations/:id/messages/unread`

- Auth required: yes
- Main purpose:
  - fetch unread messages for current user in a conversation

---

## 10. Attachment and Upload Endpoints

The active upload flow is direct multipart upload to the backend, followed by optional attachment creation.

## 10.1 POST `/v1/uploads`

- Auth required: yes
- Main purpose:
  - accept one real file upload from frontend/Postman via `multipart/form-data`
  - upload the file to S3
  - return the final file metadata and URL

### Request body

- Content type: `multipart/form-data`
- Field: `file`

### Backend actions

- validate `size_bytes <= 15728640`
- create object key
- stream upload body to S3

### Tables touched

- none

### Success response should include

- filename
- mime type
- size bytes
- object key
- final file url

## 10.2 POST `/v1/uploads/bulk`

- Auth required: yes
- Main purpose:
  - accept many real file uploads from frontend/Postman via `multipart/form-data`
  - upload files concurrently to S3
  - return a list of file metadata and URLs

### Request body

- Content type: `multipart/form-data`
- Repeated field: `files`

### Backend actions

- validate each file size and mime type
- upload files concurrently with goroutines
- return one result item per uploaded file

## 10.3 POST `/v1/attachments`

- Auth required: yes
- Main purpose:
  - create attachment metadata for an already uploaded file
  - optionally link the attachment to an existing message sent by the same user

### Request body

```json
{
  "message_id": "uuid-optional",
  "file_url": "https://cdn.example.com/uploads/photo.jpg",
  "filename": "photo.jpg",
  "mime_type": "image/jpeg",
  "size_bytes": 1048576,
  "view_once": false,
  "thumbnail_url": "https://cdn.example.com/thumb.jpg",
  "width": 1280,
  "height": 720,
  "duration_seconds": null
}
```

### Important DB constraint

- attachment size must be `<= 15 MB`

### Backend behavior

- `file_url`, `filename`, `mime_type`, and `size_bytes` must describe the uploaded file
- if `message_id` is provided, it must belong to a message sent by the current user and the attachment is linked atomically through `message_attachments`

## 10.4 GET `/v1/attachments/:id`

- Auth required: yes
- Main purpose:
  - fetch one attachment metadata row

## 10.5 POST `/v1/attachments/:id/viewed`

- Auth required: yes
- Main purpose:
  - mark a view-once attachment as viewed

### Important DB behavior

- when `view_once = true` and `viewed_at` gets set, DB trigger clears `encrypted_url`

## 10.6 GET `/v1/messages/:id/attachments`

- Auth required: yes
- Main purpose:
  - list attachments linked to a message

---

## 11. Poll Endpoints

The poll schema exists and is message-linked.

## 11.1 POST `/v1/polls`

- Auth required: yes
- Main purpose:
  - create a poll and options
  - usually as part of message creation flow

### Request body

```json
{
  "message_id": "uuid",
  "question": "Where should we meet?",
  "allows_multiple": false,
  "closes_at": null,
  "options": [
    {"option_text": "Office", "position": 1},
    {"option_text": "Cafe", "position": 2}
  ]
}
```

### Important application rule

- DB does not enforce single-choice voting across whole poll
- service layer must enforce it when `allows_multiple = false`

## 11.2 GET `/v1/polls/:id`

- Auth required: yes
- Main purpose:
  - fetch poll metadata

## 11.3 GET `/v1/polls/:id/options`

- Auth required: yes
- Main purpose:
  - list options ordered by `position`

## 11.4 POST `/v1/polls/:id/votes`

- Auth required: yes
- Main purpose:
  - vote on a poll option

### Request body

```json
{
  "option_id": "uuid"
}
```

### Tables touched

- `poll_votes`

## 11.5 DELETE `/v1/polls/:id/votes/:option_id`

- Auth required: yes
- Main purpose:
  - remove one vote by current user

## 11.6 GET `/v1/polls/:id/votes`

- Auth required: yes
- Main purpose:
  - list all votes on a poll

## 11.7 GET `/v1/polls/:id/my-votes`

- Auth required: yes
- Main purpose:
  - fetch current user's votes for one poll

## 11.8 POST `/v1/polls/:id/close`

- Auth required: yes
- Main purpose:
  - close a poll immediately by setting `closes_at = now`

---

## 12. Call Endpoints

The repository already supports a useful call API even though websocket signaling still needs to be built.

## 12.1 POST `/v1/calls`

- Auth required: yes
- Rate limit: yes, user-based
- Main purpose:
  - create a call record
  - add initial participants

### Request body

```json
{
  "conversation_id": "uuid",
  "type": "AUDIO",
  "is_group_call": false,
  "participant_ids": ["uuid-1"]
}
```

### Backend actions

- verify current user belongs to conversation
- create row in `calls`
- create rows in `call_participants`
- optionally create outbox event for call start

### Important enum note

- SQL uses `CONNECTED`
- current repository incorrectly uses `JOINED` in some places
- fix that before implementing handlers

## 12.2 GET `/v1/calls/:id`

- Auth required: yes
- Main purpose:
  - fetch one call by id

## 12.3 GET `/v1/calls?conversation_id=<uuid>&page=1&limit=20`

- Auth required: yes
- Main purpose:
  - list calls for a conversation

## 12.4 GET `/v1/calls/me?page=1&limit=20`

- Auth required: yes
- Main purpose:
  - list calls where current user is initiator or participant

## 12.5 GET `/v1/calls/active`

- Auth required: yes
- Main purpose:
  - list active calls for current user

### Important fix needed first

- current repository query should be aligned with SQL enum values

## 12.6 GET `/v1/calls/missed?since=<timestamp>`

- Auth required: yes
- Main purpose:
  - list missed calls since a given point in time

## 12.7 POST `/v1/calls/:id/connect`

- Auth required: yes
- Main purpose:
  - mark call connected

### Backend behavior

- sets `connected_at = now`

## 12.8 POST `/v1/calls/:id/end`

- Auth required: yes
- Main purpose:
  - end a call and compute duration

### Request body

```json
{
  "reason": "COMPLETED"
}
```

### Allowed reasons from schema

- `COMPLETED`
- `MISSED`
- `DECLINED`
- `FAILED`
- `TIMEOUT`

### Backend behavior

- set `ended_at`
- set `end_reason`
- if `connected_at` exists, compute `duration_seconds`

## 12.9 GET `/v1/calls/:id/duration`

- Auth required: yes
- Main purpose:
  - fetch stored call duration

## 12.10 GET `/v1/calls/:id/participants`

- Auth required: yes
- Main purpose:
  - list all call participants

## 12.11 POST `/v1/calls/:id/participants`

- Auth required: yes
- Main purpose:
  - add participant to a call

### Request body

```json
{
  "user_id": "uuid",
  "status": "INVITED"
}
```

## 12.12 DELETE `/v1/calls/:id/participants/:user_id`

- Auth required: yes
- Main purpose:
  - mark participant as left

## 12.13 PATCH `/v1/calls/:id/participants/:user_id/status`

- Auth required: yes
- Main purpose:
  - update participant call state

### Request body

```json
{
  "status": "CONNECTED"
}
```

### Valid values from SQL

- `INVITED`
- `RINGING`
- `CONNECTED`
- `LEFT`
- `DECLINED`

## 12.14 PATCH `/v1/calls/:id/participants/:user_id/mute`

- Auth required: yes
- Main purpose:
  - update audio and video mute state for one participant

### Request body

```json
{
  "audio_muted": true,
  "video_muted": false
}
```

## 12.15 GET `/v1/calls/:id/participants/active-count`

- Auth required: yes
- Main purpose:
  - get current connected participant count

---

## 13. Command Endpoints

Command logs already have a table and repository, so they should be exposed clearly.

## 13.1 GET `/v1/commands/:id`

- Auth required: yes
- Main purpose:
  - fetch a single command log

### Response should include

- `id`
- `command_type`
- `user_id`
- `conversation_id`
- `status`
- `payload`
- `undo_payload`
- `error_message`
- `execution_time_ms`
- `created_at`
- `executed_at`
- `undone_at`

## 13.2 GET `/v1/commands?limit=50`

- Auth required: yes
- Main purpose:
  - fetch recent command logs for current user

## 13.3 POST `/v1/commands/:id/undo`

- Auth required: yes
- Main purpose:
  - reverse an undoable chat action

### What it should check

- command exists
- command belongs to current user
- command status is undoable
- command has not already been undone
- undo window has not expired

### What it should do

- inspect `command_type`
- load `undo_payload`
- reverse the original action
- mark command as undone
- create outbox event for clients

### Important repo issue

- Go command status constants do not match SQL enum values yet
- fix that before implementing this route

---

## 14. Endpoints that should not be in this API doc right now

These were intentionally removed from this file:

### Encryption key endpoints

Removed because there is no backing schema or repository for:

- identity keys
- signed prekeys
- one-time prekeys
- key bundle serving

### Broadcast endpoints

Removed because there is no backing schema or repository for:

- broadcast lists
- broadcast recipients

If those features are added later, they should be documented in a separate section only after:

- SQL tables exist
- repositories exist
- handlers/services are planned

---

## 15. Recommended implementation order

If you want to build this backend cleanly, implement the endpoints in this order:

1. auth and sessions
2. users, contacts, devices
3. conversations and participants
4. messages and receipts
5. reactions, stars, pins, mentions
6. uploads and attachments
7. polls
8. calls
9. commands and undo

That order matches the actual structure already present in the repository.
