# Sentinal Chat Backend - Complete API Documentation

## Table of Contents

1. [Overview](#overview)
2. [Technology Stack](#technology-stack)
3. [Base URL & Environment](#base-url--environment)
4. [Authentication](#authentication)
5. [Standard Response Format](#standard-response-format)
6. [Error Handling](#error-handling)
7. [Rate Limiting](#rate-limiting)
8. [API Endpoints](#api-endpoints)
   - [Health & Utilities](#health--utilities)
   - [Authentication](#authentication-endpoints)
   - [Users](#users)
   - [Conversations](#conversations)
   - [Messages](#messages)
   - [Calls](#calls)
   - [Encryption](#encryption)
   - [Uploads](#uploads)
   - [Broadcasts](#broadcasts)
9. [WebSocket](#websocket)
   - [Connection](#websocket-connection)
   - [Events](#websocket-events)
10. [Data Models](#data-models)
11. [End-to-End Encryption](#end-to-end-encryption)
12. [Call Signaling (WebRTC)](#call-signaling-webrtc)

---

## Overview

Sentinal Chat is a secure, real-time messaging platform with end-to-end encryption (E2EE), voice/video calling, and support for multiple devices. This documentation provides everything frontend developers need to integrate with the backend API.

**Key Features:**
- End-to-End Encryption using Signal Protocol (X3DH)
- Real-time messaging via WebSocket
- Voice and video calls with WebRTC
- Multi-device support
- Group conversations
- Broadcast lists
- File uploads
- Message reactions, replies, and mentions

---

## Technology Stack

| Component | Technology |
|-----------|------------|
| **Language** | Go 1.25.5 |
| **HTTP Framework** | Gin |
| **Database** | PostgreSQL 15 |
| **ORM** | GORM |
| **Cache** | Redis 7 |
| **WebSocket** | Gorilla WebSocket |
| **Authentication** | JWT (JSON Web Tokens) |
| **Password Hashing** | bcrypt |
| **UUIDs** | Google UUID |

---

## Base URL & Environment

### Environment Variables

Create a `.env` file in your project root:

```env
# Application
APP_PORT=8080
APP_MODE=debug  # debug, release, test

# Database
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=sentinal_chat
DB_PORT=5432

# Authentication
JWT_SECRET=your-super-secret-key
JWT_EXPIRY_HOURS=12
REFRESH_EXPIRY_DAYS=14

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
```

### Base URLs

- **Development:** `http://localhost:8080`
- **Staging:** `https://api-staging.sentinal-chat.com`
- **Production:** `https://api.sentinal-chat.com`

All API endpoints are prefixed with `/v1/` (e.g., `http://localhost:8080/v1/auth/login`)

---

## Authentication

The API uses **JWT Bearer tokens** for authentication.

### Getting a Token

1. Register or login to receive an `access_token` and `refresh_token`
2. Include the access token in all protected requests:
   ```
   Authorization: Bearer <access_token>
   ```

### Token Lifecycle

| Token Type | Expiration | Purpose |
|------------|------------|---------|
| **Access Token** | 12 hours | Authenticate API requests |
| **Refresh Token** | 14 days | Get new access tokens |

### Token Refresh

When your access token expires, use the `/v1/auth/refresh` endpoint with your refresh token to get a new access token.

---

## Standard Response Format

All API responses follow a consistent format:

### Success Response

```json
{
  "success": true,
  "data": { ... }
}
```

### Error Response

```json
{
  "success": false,
  "error": "Human-readable error message",
  "code": "ERROR_CODE"
}
```

---

## Error Handling

### HTTP Status Codes

| Status Code | Meaning |
|-------------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Invalid or missing token |
| 403 | Forbidden - No permission |
| 404 | Not Found |
| 409 | Conflict - Resource already exists |
| 422 | Unprocessable Entity - Validation error |
| 429 | Too Many Requests - Rate limit exceeded |
| 500 | Internal Server Error |
| 503 | Service Unavailable |

### Common Error Codes

| Code | Description |
|------|-------------|
| `UNAUTHORIZED` | Invalid or expired token |
| `UNHEALTHY` | Service unavailable |
| `VALIDATION_ERROR` | Invalid request data |
| `RATE_LIMIT_EXCEEDED` | Too many requests |
| `NOT_FOUND` | Resource not found |

---

## Rate Limiting

The API implements rate limiting to prevent abuse:

| Endpoint Type | Limit |
|---------------|-------|
| Authentication | 5 requests per minute |
| Messages | 30 messages per minute |
| Calls | 10 calls per minute |
| General API | 100 requests per minute |

When rate limited, the API returns:
```json
{
  "success": false,
  "error": "Rate limit exceeded",
  "code": "RATE_LIMIT_EXCEEDED"
}
```

---

## API Endpoints

### Health & Utilities

#### Ping - Health Check
```http
GET /ping
```

**Response:**
```json
{
  "success": true,
  "data": {
    "message": "pong"
  }
}
```

#### Health - Database Status
```http
GET /health
```

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "healthy"
  }
}
```

#### Goroutines - Debug Info
```http
GET /goroutines
```

**Response:**
```json
{
  "goroutines": 42
}
```

---

### Authentication Endpoints

#### Register
```http
POST /v1/auth/register
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "username": "johndoe",
  "phone_number": "+1234567890",
  "password": "securePassword123",
  "display_name": "John Doe",
  "device_id": "device-uuid-123",
  "device_name": "iPhone 15 Pro",
  "device_type": "mobile"
}
```

**Required Fields:** `email`, `username`, `password`, `device_id`

**Response:**
```json
{
  "success": true,
  "data": {
    "user_id": "uuid",
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "dGhpcyBpcyBhIHJlZnJlc2ggdG9rZW4...",
    "session_id": "uuid",
    "expires_at": "2025-01-01T12:00:00Z"
  }
}
```

---

#### Login
```http
POST /v1/auth/login
```

**Request Body:**
```json
{
  "identity": "user@example.com",
  "password": "securePassword123",
  "device_id": "device-uuid-123",
  "device_name": "iPhone 15 Pro",
  "device_type": "mobile"
}
```

**Note:** `identity` can be email, username, or phone number

**Response:** Same as Register

---

#### Refresh Token
```http
POST /v1/auth/refresh
```

**Request Body:**
```json
{
  "session_id": "uuid",
  "refresh_token": "dGhpcyBpcyBhIHJlZnJlc2ggdG9rZW4..."
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "new-refresh-token...",
    "expires_at": "2025-01-01T12:00:00Z"
  }
}
```

---

#### Logout
```http
POST /v1/auth/logout
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "session_id": "uuid"
}
```

**Response:**
```json
{
  "success": true,
  "data": null
}
```

---

#### Logout All Sessions
```http
POST /v1/auth/logout-all
Authorization: Bearer <token>
```

Logs out from all devices.

---

#### List Sessions
```http
GET /v1/auth/sessions
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "sessions": [
      {
        "id": "uuid",
        "device_id": "device-uuid",
        "device_name": "iPhone 15 Pro",
        "device_type": "mobile",
        "ip_address": "192.168.1.1",
        "last_active": "2025-01-01T10:00:00Z",
        "created_at": "2025-01-01T08:00:00Z",
        "is_current": true
      }
    ]
  }
}
```

---

#### Forgot Password
```http
POST /v1/auth/password/forgot
```

**Request Body:**
```json
{
  "identity": "user@example.com"
}
```

---

#### Reset Password
```http
POST /v1/auth/password/reset
```

**Request Body:**
```json
{
  "identity": "user@example.com",
  "new_password": "newSecurePassword123"
}
```

---

### Users

#### Get Current User Profile
```http
GET /v1/users/me
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "email": "user@example.com",
    "username": "johndoe",
    "display_name": "John Doe",
    "avatar_url": "https://...",
    "status": "Hey there!",
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

---

#### Update Profile
```http
PUT /v1/users/me
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "display_name": "John Doe Updated",
  "avatar_url": "https://...",
  "bio": "Software developer",
  "status": "Available"
}
```

---

#### Delete Account
```http
DELETE /v1/users/me
Authorization: Bearer <token>
```

---

#### Get User Settings
```http
GET /v1/users/me/settings
Authorization: Bearer <token>
```

---

#### Update Settings
```http
PUT /v1/users/me/settings
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "notifications_enabled": true,
  "theme": "dark",
  "language": "en"
}
```

---

#### List Contacts
```http
GET /v1/users/me/contacts
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "contacts": [
      {
        "id": "uuid",
        "email": "friend@example.com",
        "username": "frienduser",
        "display_name": "Friend Name",
        "avatar_url": "https://...",
        "status": "Busy",
        "created_at": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```

---

#### Add Contact
```http
POST /v1/users/me/contacts
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "contact_user_id": "uuid-of-user-to-add"
}
```

---

#### Remove Contact
```http
DELETE /v1/users/me/contacts/:id
Authorization: Bearer <token>
```

---

#### Block/Unblock Contact
```http
POST /v1/users/me/contacts/:id/block
POST /v1/users/me/contacts/:id/unblock
Authorization: Bearer <token>
```

---

#### List Blocked Contacts
```http
GET /v1/users/me/contacts/blocked
Authorization: Bearer <token>
```

---

#### List Devices
```http
GET /v1/users/me/devices
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "devices": [
      {
        "id": "uuid",
        "device_name": "iPhone 15 Pro",
        "device_type": "mobile",
        "last_active": "2025-01-01T10:00:00Z",
        "is_active": true
      }
    ]
  }
}
```

---

#### Deactivate Device
```http
DELETE /v1/users/me/devices/:id
Authorization: Bearer <token>
```

---

#### Revoke Session
```http
DELETE /v1/users/me/sessions/:id
Authorization: Bearer <token>
```

---

#### Revoke All Other Sessions
```http
DELETE /v1/users/me/sessions
Authorization: Bearer <token>
```

---

### Conversations

#### Create Conversation
```http
POST /v1/conversations
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "type": "GROUP",
  "subject": "Team Chat",
  "description": "Discussion group for the team",
  "participants": ["user-uuid-1", "user-uuid-2"]
}
```

**Types:** `DM` (Direct Message) or `GROUP`

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "type": "GROUP",
    "subject": "Team Chat",
    "description": "Discussion group for the team",
    "creator_id": "uuid",
    "participant_count": 3,
    "created_at": "2025-01-01T00:00:00Z"
  }
}
```

---

#### List Conversations
```http
GET /v1/conversations?page=1&limit=20
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "conversations": [
      {
        "id": "uuid",
        "type": "GROUP",
        "subject": "Team Chat",
        "description": "Discussion group",
        "avatar_url": "https://...",
        "creator_id": "uuid",
        "invite_link": "https://...",
        "participant_count": 5,
        "last_message_at": "2025-01-01T12:00:00Z",
        "created_at": "2025-01-01T00:00:00Z"
      }
    ],
    "total": 42
  }
}
```

---

#### Get Conversation by ID
```http
GET /v1/conversations/:id
Authorization: Bearer <token>
```

---

#### Update Conversation
```http
PUT /v1/conversations/:id
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "subject": "Updated Subject",
  "description": "Updated description",
  "avatar_url": "https://..."
}
```

---

#### Delete Conversation
```http
DELETE /v1/conversations/:id
Authorization: Bearer <token>
```

---

#### Get Direct Conversation Between Users
```http
GET /v1/conversations/direct?user_id_1=uuid1&user_id_2=uuid2
Authorization: Bearer <token>
```

---

#### Search Conversations
```http
GET /v1/conversations/search?query=team
Authorization: Bearer <token>
```

---

#### Get by Invite Link
```http
GET /v1/conversations/invite?link=invite-code
Authorization: Bearer <token>
```

---

#### Regenerate Invite Link
```http
POST /v1/conversations/:id/invite
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "invite_link": "https://app.sentinal-chat.com/invite/abc123"
  }
}
```

---

#### Add Participant
```http
POST /v1/conversations/:id/participants
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid",
  "role": "member"
}
```

**Roles:** `member`, `admin`, `owner`

---

#### Remove Participant
```http
DELETE /v1/conversations/:id/participants/:user_id
Authorization: Bearer <token>
```

---

#### List Participants
```http
GET /v1/conversations/:id/participants
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "participants": [
      {
        "user_id": "uuid",
        "username": "johndoe",
        "role": "admin",
        "joined_at": "2025-01-01T00:00:00Z"
      }
    ]
  }
}
```

---

#### Update Participant Role
```http
PUT /v1/conversations/:id/participants/:user_id/role
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "role": "admin"
}
```

---

#### Mute/Unmute Conversation
```http
POST /v1/conversations/:id/mute
POST /v1/conversations/:id/unmute
Authorization: Bearer <token>
```

**Mute Request Body:**
```json
{
  "until": "2025-01-02T00:00:00Z"
}
```

---

#### Pin/Unpin Conversation
```http
POST /v1/conversations/:id/pin
POST /v1/conversations/:id/unpin
Authorization: Bearer <token>
```

---

#### Archive/Unarchive Conversation
```http
POST /v1/conversations/:id/archive
POST /v1/conversations/:id/unarchive
Authorization: Bearer <token>
```

---

#### Get Sequence Number
```http
GET /v1/conversations/:id/sequence
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "sequence": 42
  }
}
```

---

#### Increment Sequence
```http
POST /v1/conversations/:id/sequence
Authorization: Bearer <token>
```

---

### Messages

#### Send Message
```http
POST /v1/messages
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "conversation_id": "uuid",
  "ciphertexts": [
    {
      "recipient_device_id": "device-uuid-1",
      "ciphertext": "base64-encoded-encrypted-message",
      "header": {
        "ephemeral_key": "base64-key",
        "counter": 1
      }
    }
  ],
  "message_type": "text",
  "client_message_id": "client-generated-uuid",
  "idempotency_key": "unique-key-for-dedup"
}
```

**Message Types:** `text`, `image`, `video`, `audio`, `file`, `location`, `contact`, `poll`

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "conversation_id": "uuid",
    "sender_id": "uuid",
    "client_message_id": "client-generated-uuid",
    "sequence_number": 42,
    "created_at": "2025-01-01T12:00:00Z"
  }
}
```

---

#### List Messages
```http
GET /v1/messages?conversation_id=uuid&before_seq=100&limit=50
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "messages": [
      {
        "id": "uuid",
        "conversation_id": "uuid",
        "sender_id": "uuid",
        "client_message_id": "client-uuid",
        "sequence_number": 41,
        "is_deleted": false,
        "is_edited": false,
        "ciphertext": "base64-encrypted-content",
        "header": "base64-header",
        "recipient_device_id": "device-uuid",
        "created_at": "2025-01-01T12:00:00Z",
        "updated_at": "2025-01-01T12:00:00Z"
      }
    ]
  }
}
```

---

#### Get Message by ID
```http
GET /v1/messages/:id
Authorization: Bearer <token>
```

---

#### Update Message (Edit)
```http
PUT /v1/messages/:id
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "ciphertext": "new-encrypted-content"
}
```

---

#### Delete Message (Soft Delete)
```http
DELETE /v1/messages/:id
Authorization: Bearer <token>
```

---

#### Hard Delete Message
```http
DELETE /v1/messages/:id/hard
Authorization: Bearer <token>
```

---

#### Mark as Read
```http
POST /v1/messages/:id/read
Authorization: Bearer <token>
```

---

#### Mark as Delivered
```http
POST /v1/messages/:id/delivered
Authorization: Bearer <token>
```

---

### Calls

#### Create Call
```http
POST /v1/calls
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "conversation_id": "uuid",
  "type": "VIDEO",
  "initiator_id": "uuid"
}
```

**Call Types:** `AUDIO`, `VIDEO`

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "conversation_id": "uuid",
    "type": "VIDEO",
    "status": "INITIATED",
    "initiator_id": "uuid",
    "created_at": "2025-01-01T12:00:00Z"
  }
}
```

---

#### Get Call by ID
```http
GET /v1/calls/:id
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "conversation_id": "uuid",
    "type": "VIDEO",
    "status": "ACTIVE",
    "initiator_id": "uuid",
    "started_at": "2025-01-01T12:00:00Z",
    "ended_at": null,
    "duration": 0
  }
}
```

---

#### List Calls
```http
GET /v1/calls?conversation_id=uuid&page=1&limit=20
GET /v1/calls/user?user_id=uuid
GET /v1/calls/active
GET /v1/calls/missed
Authorization: Bearer <token>
```

---

#### Add Call Participant
```http
POST /v1/calls/:id/participants
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid"
}
```

---

#### Remove Call Participant
```http
DELETE /v1/calls/:id/participants/:user_id
Authorization: Bearer <token>
```

---

#### List Call Participants
```http
GET /v1/calls/:id/participants
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "participants": [
      {
        "user_id": "uuid",
        "status": "JOINED",
        "audio_muted": false,
        "video_muted": false,
        "joined_at": "2025-01-01T12:00:00Z",
        "left_at": null
      }
    ]
  }
}
```

---

#### Update Participant Status
```http
PUT /v1/calls/:id/participants/:user_id/status
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "status": "JOINED"
}
```

**Statuses:** `INVITED`, `JOINED`, `LEFT`

---

#### Update Participant Mute State
```http
PUT /v1/calls/:id/participants/:user_id/mute
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "audio_muted": true,
  "video_muted": false
}
```

---

#### Mark Call Connected
```http
POST /v1/calls/:id/connected
Authorization: Bearer <token>
```

---

#### End Call
```http
POST /v1/calls/:id/end
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "reason": "COMPLETED"
}
```

**Reasons:** `COMPLETED`, `MISSED`, `DECLINED`, `FAILED`, `TIMEOUT`, `NETWORK_ERROR`

---

#### Get Call Duration
```http
GET /v1/calls/:id/duration
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "duration": 3600
  }
}
```

---

#### Record Quality Metric
```http
POST /v1/calls/quality
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "call_id": "uuid",
  "user_id": "uuid",
  "timestamp": "2025-01-01T12:00:00Z",
  "packet_loss": 0.01,
  "jitter": 15.5,
  "latency": 45.2,
  "bitrate": 2500000,
  "frame_rate": 30,
  "resolution": "1920x1080",
  "audio_level": -20.5
}
```

---

### Encryption

These endpoints are used for the Signal Protocol implementation (X3DH) for end-to-end encryption.

#### Upload Identity Key
```http
POST /v1/encryption/identity
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid",
  "device_id": "device-uuid",
  "public_key": "base64-public-key"
}
```

---

#### Get Identity Key
```http
GET /v1/encryption/identity?user_id=uuid&device_id=device-uuid
Authorization: Bearer <token>
```

---

#### Deactivate Identity Key
```http
PUT /v1/encryption/identity/:id/deactivate
Authorization: Bearer <token>
```

---

#### Delete Identity Key
```http
DELETE /v1/encryption/identity/:id
Authorization: Bearer <token>
```

---

#### Upload Signed PreKey
```http
POST /v1/encryption/signed-prekeys
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid",
  "device_id": "device-uuid",
  "key_id": 1,
  "public_key": "base64-public-key",
  "signature": "base64-signature"
}
```

---

#### Get Signed PreKey
```http
GET /v1/encryption/signed-prekeys?user_id=uuid&device_id=device-uuid&key_id=1
Authorization: Bearer <token>
```

---

#### Get Active Signed PreKey
```http
GET /v1/encryption/signed-prekeys/active
Authorization: Bearer <token>
```

---

#### Rotate Signed PreKey
```http
POST /v1/encryption/signed-prekeys/rotate
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid",
  "device_id": "device-uuid",
  "key": {
    "key_id": 2,
    "public_key": "base64-public-key",
    "signature": "base64-signature"
  }
}
```

---

#### Upload One-Time PreKeys
```http
POST /v1/encryption/onetime-prekeys
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "keys": [
    {
      "user_id": "uuid",
      "device_id": "device-uuid",
      "key_id": 1,
      "public_key": "base64-public-key"
    }
  ]
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "uploaded": 100
  }
}
```

---

#### Consume One-Time PreKey
```http
POST /v1/encryption/onetime-prekeys/consume?user_id=uuid&device_id=device-uuid&consumed_by=uuid&consumed_device_id=device-uuid
Authorization: Bearer <token>
```

---

#### Get PreKey Count
```http
GET /v1/encryption/onetime-prekeys/count?user_id=uuid&device_id=device-uuid
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "count": 50
  }
}
```

---

#### Get Key Bundle
```http
GET /v1/encryption/bundles?user_id=uuid&device_id=device-uuid&consumer_device_id=device-uuid
Authorization: Bearer <token>
```

Gets a complete key bundle (identity key + signed prekey + one-time prekey) for initializing a session.

---

#### Check Active Keys
```http
GET /v1/encryption/keys/active?user_id=uuid&device_id=device-uuid
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "has_active_keys": true
  }
}
```

---

### Uploads

#### Create Upload Session
```http
POST /v1/uploads
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "file_name": "photo.jpg",
  "file_size": 2048576,
  "content_type": "image/jpeg",
  "uploader_id": "uuid"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "file_name": "photo.jpg",
    "file_size": 2048576,
    "content_type": "image/jpeg",
    "uploader_id": "uuid",
    "status": "PENDING",
    "upload_url": "https://storage.sentinal-chat.com/upload/uuid",
    "created_at": "2025-01-01T12:00:00Z"
  }
}
```

---

#### Get Upload Status
```http
GET /v1/uploads/:id
Authorization: Bearer <token>
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "file_name": "photo.jpg",
    "file_size": 2048576,
    "content_type": "image/jpeg",
    "uploader_id": "uuid",
    "status": "COMPLETED",
    "uploaded_bytes": 2048576,
    "file_url": "https://cdn.sentinal-chat.com/files/uuid.jpg",
    "created_at": "2025-01-01T12:00:00Z",
    "completed_at": "2025-01-01T12:01:00Z"
  }
}
```

**Statuses:** `PENDING`, `IN_PROGRESS`, `COMPLETED`, `FAILED`, `CANCELLED`

---

#### Update Upload Progress
```http
POST /v1/uploads/:id/progress
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "uploaded_bytes": 1024000
}
```

---

#### Mark Upload Complete
```http
POST /v1/uploads/:id/complete
Authorization: Bearer <token>
```

---

#### Mark Upload Failed
```http
POST /v1/uploads/:id/fail
Authorization: Bearer <token>
```

---

#### List User Uploads
```http
GET /v1/uploads?uploader_id=uuid&page=1&limit=20
Authorization: Bearer <token>
```

---

#### List Completed Uploads
```http
GET /v1/uploads/completed
Authorization: Bearer <token>
```

---

#### List In-Progress Uploads
```http
GET /v1/uploads/in-progress
Authorization: Bearer <token>
```

---

#### List Stale Uploads
```http
GET /v1/uploads/stale?older_than_sec=3600
Authorization: Bearer <token>
```

---

#### Delete Stale Uploads
```http
DELETE /v1/uploads/stale?older_than_sec=3600
Authorization: Bearer <token>
```

---

#### Delete Upload
```http
DELETE /v1/uploads/:id
Authorization: Bearer <token>
```

---

### Broadcasts

Broadcasts allow sending messages to multiple users without creating a group.

#### Create Broadcast
```http
POST /v1/broadcasts
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "name": "Marketing List",
  "description": "List for marketing updates"
}
```

---

#### Get Broadcast
```http
GET /v1/broadcasts/:id
Authorization: Bearer <token>
```

---

#### Update Broadcast
```http
PUT /v1/broadcasts/:id
Authorization: Bearer <token>
```

---

#### Delete Broadcast
```http
DELETE /v1/broadcasts/:id
Authorization: Bearer <token>
```

---

#### List My Broadcasts
```http
GET /v1/broadcasts
Authorization: Bearer <token>
```

---

#### Search Broadcasts
```http
GET /v1/broadcasts/search?query=marketing
Authorization: Bearer <token>
```

---

#### Add Recipient
```http
POST /v1/broadcasts/:id/recipients
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_id": "uuid"
}
```

---

#### Remove Recipient
```http
DELETE /v1/broadcasts/:id/recipients/:user_id
Authorization: Bearer <token>
```

---

#### List Recipients
```http
GET /v1/broadcasts/:id/recipients
Authorization: Bearer <token>
```

---

#### Get Recipient Count
```http
GET /v1/broadcasts/:id/recipients/count
Authorization: Bearer <token>
```

---

#### Check if User is Recipient
```http
GET /v1/broadcasts/:id/recipients/:user_id
Authorization: Bearer <token>
```

---

#### Bulk Add Recipients
```http
POST /v1/broadcasts/:id/recipients/bulk
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_ids": ["uuid-1", "uuid-2", "uuid-3"]
}
```

---

#### Bulk Remove Recipients
```http
DELETE /v1/broadcasts/:id/recipients/bulk
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "user_ids": ["uuid-1", "uuid-2"]
}
```

---

## WebSocket

WebSocket connections provide real-time updates for messages, typing indicators, presence, and call signaling.

### WebSocket Connection

#### Connection URL
```
ws://localhost:8080/v1/ws?token=<access_token>
```

Or use the Authorization header:
```javascript
const ws = new WebSocket('ws://localhost:8080/v1/ws');
ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'auth',
    token: 'Bearer <access_token>'
  }));
};
```

#### Connection Limitations

- Maximum 10 connections per user
- Rate limited to 10 new connections per minute per user
- Connections are automatically closed after inactivity

### WebSocket Events

#### Incoming Events (Server → Client)

##### New Message
```json
{
  "type": "message:new",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "sender-uuid",
  "conversation_id": "conv-uuid",
  "message_id": "msg-uuid",
  "sender_id": "sender-uuid",
  "content": "encrypted-content"
}
```

##### Message Read
```json
{
  "type": "message:read",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "reader-uuid",
  "conversation_id": "conv-uuid",
  "message_id": "msg-uuid",
  "reader_id": "reader-uuid"
}
```

##### Message Delivered
```json
{
  "type": "message:delivered",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "recipient-uuid",
  "conversation_id": "conv-uuid",
  "message_id": "msg-uuid",
  "recipient_id": "recipient-uuid"
}
```

##### Typing Started
```json
{
  "type": "typing:started",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "typer-uuid",
  "conversation_id": "conv-uuid",
  "display_name": "John Doe",
  "is_typing": true
}
```

##### Typing Stopped
```json
{
  "type": "typing:stopped",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "typer-uuid",
  "conversation_id": "conv-uuid",
  "display_name": "John Doe",
  "is_typing": false
}
```

##### Presence Online
```json
{
  "type": "presence:online",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "user-uuid",
  "is_online": true,
  "status": "online"
}
```

##### Presence Offline
```json
{
  "type": "presence:offline",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "user-uuid",
  "is_online": false,
  "status": "offline"
}
```

#### Outgoing Events (Client → Server)

##### Send Typing Indicator
```json
{
  "type": "typing:started",
  "conversation_id": "conv-uuid"
}
```

```json
{
  "type": "typing:stopped",
  "conversation_id": "conv-uuid"
}
```

##### Update Presence
```json
{
  "type": "presence:online",
  "status": "online"
}
```

---

## Data Models

### User

```typescript
interface User {
  id: string;           // UUID
  email: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  status?: string;      // Custom status message
  created_at: string;   // ISO 8601 timestamp
}
```

### Device

```typescript
interface Device {
  id: string;           // UUID
  device_name: string;
  device_type: string;  // mobile, desktop, web, tablet
  last_active?: string;
  is_active: boolean;
}
```

### Conversation

```typescript
interface Conversation {
  id: string;           // UUID
  type: 'DM' | 'GROUP';
  subject?: string;     // Group name
  description?: string;
  avatar_url?: string;
  creator_id: string;
  invite_link?: string;
  participant_count: number;
  last_message_at?: string;
  created_at: string;
}
```

### Participant

```typescript
interface Participant {
  user_id: string;
  username?: string;
  role: 'member' | 'admin' | 'owner';
  joined_at: string;
}
```

### Message

```typescript
interface Message {
  id: string;                    // UUID
  conversation_id: string;
  sender_id: string;
  client_message_id?: string;    // Client-generated ID for deduplication
  sequence_number: number;       // Message ordering within conversation
  is_deleted: boolean;
  is_edited: boolean;
  ciphertext?: string;           // Encrypted content (base64)
  header?: string;               // Encryption header (base64)
  recipient_device_id?: string;  // Target device for this ciphertext
  created_at: string;
  updated_at?: string;
}
```

### Call

```typescript
interface Call {
  id: string;
  conversation_id: string;
  type: 'AUDIO' | 'VIDEO';
  status: 'INITIATED' | 'RINGING' | 'CONNECTED' | 'ENDED';
  initiator_id: string;
  started_at?: string;
  ended_at?: string;
  duration?: number;  // Seconds
}
```

### Call Participant

```typescript
interface CallParticipant {
  user_id: string;
  status: 'INVITED' | 'JOINED' | 'LEFT';
  audio_muted: boolean;
  video_muted: boolean;
  joined_at?: string;
  left_at?: string;
}
```

### Upload

```typescript
interface Upload {
  id: string;
  file_name: string;
  file_size: number;        // Bytes
  content_type: string;
  uploader_id: string;
  status: 'PENDING' | 'IN_PROGRESS' | 'COMPLETED' | 'FAILED' | 'CANCELLED';
  uploaded_bytes: number;
  file_url?: string;
  created_at: string;
  completed_at?: string;
}
```

### Broadcast

```typescript
interface Broadcast {
  id: string;
  name: string;
  description?: string;
  owner_id: string;
  created_at: string;
}
```

---

## End-to-End Encryption

Sentinal Chat implements the **Signal Protocol (X3DH)** for end-to-end encryption.

### Key Types

1. **Identity Key** - Long-term ECDH key pair
2. **Signed PreKey** - Medium-term ECDH key pair (signed by identity key)
3. **One-Time PreKeys** - Short-term ECDH key pairs (deleted after use)

### Encryption Flow

#### Initial Session Setup (Alice → Bob)

1. Alice requests Bob's key bundle:
   ```http
   GET /v1/encryption/bundles?user_id=bob&device_id=bob-device&consumer_device_id=alice-device
   ```

2. Alice generates an ephemeral key pair and computes shared secrets using X3DH

3. Alice sends initial message with encrypted payload for each of Bob's devices

4. Bob receives message and completes X3DH to derive the same shared secrets

#### Sending Messages

After initial setup, use the Double Ratchet algorithm for forward secrecy:

1. Each message uses a new encryption key derived from the root chain
2. Keys are never reused
3. Messages include a counter for ordering

#### Multi-Device Support

- Each device has its own identity key
- Messages are encrypted separately for each recipient device
- Sender includes `ciphertexts` array with one entry per device

#### Key Rotation

- **Signed PreKeys**: Should be rotated every 1-3 months
- **One-Time PreKeys**: Upload 100 new keys when count drops below 10

---

## Call Signaling (WebRTC)

WebSocket events are used for WebRTC signaling:

### Call Offer
```json
{
  "type": "call:offer",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "caller-uuid",
  "call_id": "call-uuid",
  "from_id": "caller-uuid",
  "to_id": "callee-uuid",
  "signal_type": "offer",
  "data": "base64-encoded-SDP-offer"
}
```

### Call Answer
```json
{
  "type": "call:answer",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "callee-uuid",
  "call_id": "call-uuid",
  "from_id": "callee-uuid",
  "to_id": "caller-uuid",
  "signal_type": "answer",
  "data": "base64-encoded-SDP-answer"
}
```

### ICE Candidate
```json
{
  "type": "call:ice",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "sender-uuid",
  "call_id": "call-uuid",
  "from_id": "sender-uuid",
  "to_id": "recipient-uuid",
  "signal_type": "ice",
  "data": "base64-encoded-ICE-candidate"
}
```

### Call Ended
```json
{
  "type": "call:ended",
  "timestamp": "2025-01-01T12:00:00Z",
  "user_id": "ender-uuid",
  "call_id": "call-uuid",
  "conversation_id": "conv-uuid",
  "ended_by": "ender-uuid",
  "reason": "completed",
  "duration_seconds": 3600
}
```

### WebRTC Flow

1. **Create Call** - Caller creates call via REST API
2. **Add Participants** - Add callee(s) to the call
3. **Send Offer** - Caller sends WebSocket event with SDP offer
4. **Send Answer** - Callee sends WebSocket event with SDP answer
5. **Exchange ICE** - Both parties exchange ICE candidates via WebSocket
6. **Mark Connected** - Call REST API when connection established
7. **End Call** - Either party can end via REST API

---

## Quick Reference

### HTTP Methods

| Method | Usage |
|--------|-------|
| GET | Retrieve resources |
| POST | Create resources, actions |
| PUT | Update resources (full) |
| DELETE | Remove resources |

### Common Headers

```http
Authorization: Bearer <access_token>
Content-Type: application/json
Accept: application/json
X-Request-ID: <uuid>
```

### UUID Format

All UUIDs use the standard format:
```
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

Example: `550e8400-e29b-41d4-a716-446655440000`

### Date Format

All dates use ISO 8601 format:
```
2025-01-01T12:00:00Z
```

### Base64 Encoding

Encrypted content and cryptographic keys are base64-encoded strings.

---

## Development Setup

### Prerequisites

- Go 1.25.5+
- PostgreSQL 15+
- Redis 7+

### Run with Docker

```bash
# Start infrastructure services
make up

# Run migrations
make migrate-up

# Run the server
make run
```

### API Testing

Use the provided endpoints to test:

1. Health check: `GET http://localhost:8080/ping`
2. Register a user: `POST http://localhost:8080/v1/auth/register`
3. Connect WebSocket: `ws://localhost:8080/v1/ws?token=<token>`

---

*This documentation covers the complete Sentinal Chat API. For questions or issues, please refer to the backend codebase or contact the development team.*
