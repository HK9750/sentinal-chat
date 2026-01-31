# Sentinal Chat - Project Plan

## 📋 Project Overview

Build a production-grade, WhatsApp-like chat backend using a monolithic Go service with PostgreSQL, Redis, WebSockets, and event-driven workflows.

**Tech Stack:**
- **Language:** Go 1.21+
- **Database:** PostgreSQL 15
- **Cache/Pub-Sub:** Redis 7
- **ORM:** GORM
- **HTTP Framework:** Gin
- **WebSocket:** Gorilla WebSocket
- **Logging:** Zap
- **Auth:** JWT (Access + Refresh tokens)

---

## ✅ Phase 0: Infrastructure & Foundation (COMPLETED)

### Completed Tasks
- Docker Compose setup with PostgreSQL, Redis, pgAdmin, and RedisInsight
- Go module initialization with all dependencies
- Configuration loader for environment variables
- Database connection package with GORM integration
- Raw SQL migrations for extensions, schema, indexes, and triggers
- GORM entity definitions for all 8 domain packages (user, message, conversation, call, encryption, broadcast, upload, event)
- Migration CLI tool with up, down, status, seed, reset commands
- Database seeding system with admin user, test users, conversations, and messages
- Makefile with all migration targets
- Zap logger package with context support

### Current Project Structure
```
sentinal-chat/
├── cmd/
│   ├── api/main.go           # API server entrypoint
│   └── migrate/main.go       # Migration CLI tool
├── config/
│   └── config.go             # Environment configuration
├── internal/
│   └── domain/               # Domain entities (8 packages)
├── migrations/               # SQL migration files
├── pkg/
│   ├── database/             # Database utilities
│   └── logger/               # Zap logger wrapper
├── docker-compose.yml
├── Makefile
└── go.mod
```

---

## 🚀 Phase 1: Repository Layer

### Goal
Create repository interfaces and implementations for all database operations, following the repository pattern for clean architecture.

### Tasks
1. Define repository interfaces for each domain entity
2. Implement UserRepository with CRUD operations and queries
3. Implement ConversationRepository with participant management
4. Implement MessageRepository with pagination and search
5. Implement CallRepository for call history and participants
6. Implement EncryptionRepository for key management
7. Implement EventRepository for outbox operations
8. Add transaction support across repositories
9. Implement soft delete patterns where applicable
10. Add query optimization with proper indexing usage

### Folder Structure
```
internal/
├── repository/
│   ├── interfaces.go
│   ├── user_repository.go
│   ├── conversation_repository.go
│   ├── message_repository.go
│   ├── call_repository.go
│   ├── encryption_repository.go
│   ├── event_repository.go
│   └── base_repository.go
```

### Estimated Time: 2-3 days

---

## 🔐 Phase 2: Authentication & Authorization

### Goal
Implement secure authentication flows with JWT tokens, session management, and role-based access control.

### Tasks
1. Create auth service with registration and login logic
2. Implement JWT token generation (access + refresh tokens)
3. Implement password hashing with bcrypt
4. Create session management with device tracking
5. Build token refresh mechanism
6. Implement logout with token blacklisting via Redis
7. Add email verification flow
8. Implement password reset flow
9. Create auth middleware for protected routes
10. Implement role-based authorization middleware
11. Add rate limiting per user and IP address

### Auth Endpoints
- POST /api/v1/auth/register - Register new user
- POST /api/v1/auth/login - Login with credentials
- POST /api/v1/auth/refresh - Refresh access token
- POST /api/v1/auth/logout - Logout current session
- POST /api/v1/auth/logout-all - Logout all devices
- GET /api/v1/auth/me - Get current user
- POST /api/v1/auth/verify-email - Verify email address
- POST /api/v1/auth/forgot-password - Request password reset
- POST /api/v1/auth/reset-password - Reset password with token

### Folder Structure
```
internal/
├── service/
│   └── auth/
│       ├── service.go
│       ├── jwt.go
│       ├── password.go
│       └── session.go
├── middleware/
│   ├── auth.go
│   ├── role.go
│   └── rate_limit.go
```

### Estimated Time: 3-4 days

---

## 💬 Phase 3: HTTP API Layer

### Goal
Build RESTful API endpoints using Gin framework with proper request/response handling.

### Tasks
1. Set up Gin HTTP server with middleware chain
2. Create request/response DTOs for all endpoints
3. Implement user endpoints (profile, settings, contacts)
4. Implement conversation endpoints (CRUD, participants)
5. Implement message endpoints (send, edit, delete, reactions)
6. Implement attachment upload endpoints
7. Add request validation with proper error messages
8. Implement pagination for list endpoints
9. Add API versioning (v1 prefix)
10. Create consistent error response format
11. Add CORS configuration

### User Endpoints
- GET /api/v1/users/:id - Get user profile
- PUT /api/v1/users/me - Update own profile
- GET /api/v1/users/me/settings - Get settings
- PUT /api/v1/users/me/settings - Update settings
- GET /api/v1/users/me/contacts - Get contacts
- POST /api/v1/users/me/contacts - Add contact
- DELETE /api/v1/users/me/contacts/:id - Remove contact
- GET /api/v1/users/search - Search users

### Conversation Endpoints
- GET /api/v1/conversations - List conversations
- POST /api/v1/conversations - Create conversation
- GET /api/v1/conversations/:id - Get conversation details
- PUT /api/v1/conversations/:id - Update conversation
- DELETE /api/v1/conversations/:id - Leave or delete conversation
- POST /api/v1/conversations/:id/participants - Add participant
- DELETE /api/v1/conversations/:id/participants/:userId - Remove participant
- PUT /api/v1/conversations/:id/participants/:userId - Update participant role

### Message Endpoints
- GET /api/v1/conversations/:id/messages - Get messages (paginated)
- POST /api/v1/conversations/:id/messages - Send message
- PUT /api/v1/messages/:id - Edit message
- DELETE /api/v1/messages/:id - Delete message
- POST /api/v1/messages/:id/reactions - Add reaction
- DELETE /api/v1/messages/:id/reactions/:code - Remove reaction
- POST /api/v1/messages/:id/star - Star message
- DELETE /api/v1/messages/:id/star - Unstar message
- GET /api/v1/conversations/:id/messages/search - Search messages

### Folder Structure
```
internal/
├── handler/
│   ├── auth_handler.go
│   ├── user_handler.go
│   ├── conversation_handler.go
│   ├── message_handler.go
│   └── attachment_handler.go
├── dto/
│   ├── request/
│   └── response/
├── server/
│   └── server.go
```

### Estimated Time: 4-5 days

---

## ⚡ Phase 4: WebSocket & Real-Time Messaging

### Goal
Implement WebSocket hub for real-time message delivery, presence, and typing indicators.

### Tasks
1. Create WebSocket hub for connection management
2. Implement client connection handling with authentication
3. Define WebSocket message protocol (JSON format)
4. Integrate with Redis Pub/Sub for horizontal scaling
5. Implement message broadcasting to conversation participants
6. Add typing indicators with Redis TTL
7. Implement presence tracking (online/offline status)
8. Handle connection heartbeat and reconnection
9. Support multi-device connections per user
10. Implement message sync on reconnection
11. Add connection-level rate limiting

### WebSocket Events (Server → Client)
- message.new - New message received
- message.updated - Message was edited
- message.deleted - Message was deleted
- message.reaction - Reaction added or removed
- message.receipt - Delivery/read receipt update
- typing.update - User typing status changed
- presence.update - User online status changed
- call.incoming - Incoming call notification
- call.answered - Call was answered
- call.ended - Call ended

### WebSocket Events (Client → Server)
- typing.start - User started typing
- typing.stop - User stopped typing
- message.ack - Acknowledge message received
- presence.ping - Keep connection alive

### Folder Structure
```
internal/
├── websocket/
│   ├── hub.go
│   ├── client.go
│   ├── message.go
│   ├── handler.go
│   └── presence.go
```

### Estimated Time: 4-5 days

---

## 📤 Phase 5: Event-Driven Architecture

### Goal
Implement transactional outbox pattern for reliable event delivery and async processing.

### Tasks
1. Define event types and payload structures
2. Implement event publishing within database transactions
3. Create outbox worker for polling pending events
4. Integrate with Redis Pub/Sub for event distribution
5. Implement retry logic with exponential backoff
6. Add dead-letter handling for failed events
7. Create event subscription management
8. Implement event delivery tracking
9. Add event correlation for tracing
10. Create cleanup job for old processed events

### Event Types
- message.created, message.updated, message.deleted
- reaction.added, reaction.removed
- receipt.sent, receipt.delivered, receipt.read
- user.registered, user.updated, user.deleted
- conversation.created, conversation.updated
- participant.added, participant.removed, participant.role_changed
- call.started, call.answered, call.ended

### Folder Structure
```
internal/
├── events/
│   ├── types.go
│   ├── publisher.go
│   ├── consumer.go
│   ├── outbox_worker.go
│   └── subscription.go
```

### Estimated Time: 2-3 days

---

## 📞 Phase 6: Calls & WebRTC Support

### Goal
Implement voice/video call signaling and management infrastructure.

### Tasks
1. Create call initiation flow
2. Implement call answer/reject handling
3. Add call end and cleanup logic
4. Track call participants and their status
5. Store call quality metrics
6. Implement TURN credential generation
7. Add SFU server selection logic
8. Create call history endpoints
9. Handle group call participant management
10. Implement call recording metadata (optional)

### Call Endpoints
- POST /api/v1/calls - Initiate a call
- POST /api/v1/calls/:id/answer - Answer incoming call
- POST /api/v1/calls/:id/reject - Reject incoming call
- POST /api/v1/calls/:id/end - End active call
- GET /api/v1/calls/:id - Get call details
- GET /api/v1/calls/history - Get call history
- POST /api/v1/calls/turn-credentials - Get TURN server credentials
- POST /api/v1/calls/:id/participants - Add participant to group call
- DELETE /api/v1/calls/:id/participants/:userId - Remove participant

### Call Flow
1. Caller initiates call via REST API
2. Server sends WebSocket event to callee(s)
3. Callee answers or rejects via REST API
4. Server notifies caller via WebSocket
5. Clients exchange WebRTC signaling via WebSocket
6. Call quality metrics stored periodically
7. Call ends via REST API or disconnect
8. Server updates call record and notifies participants

### Estimated Time: 3-4 days

---

## 🔒 Phase 7: E2E Encryption Support

### Goal
Implement Signal Protocol key management endpoints for client-side encryption.

### Tasks
1. Create identity key upload endpoint
2. Implement signed prekey management
3. Add one-time prekey batch upload
4. Create key bundle retrieval endpoint
5. Track prekey consumption
6. Implement prekey count monitoring
7. Add low prekey warning system
8. Create encrypted session storage (optional)
9. Implement key rotation reminders
10. Add device key verification flow

### Encryption Endpoints
- POST /api/v1/keys/identity - Upload identity key
- GET /api/v1/keys/identity - Get own identity keys
- POST /api/v1/keys/signed-prekey - Upload signed prekey
- GET /api/v1/keys/signed-prekey - Get current signed prekey
- POST /api/v1/keys/one-time-prekeys - Upload batch of one-time prekeys
- GET /api/v1/keys/one-time-prekeys/count - Get remaining prekey count
- GET /api/v1/keys/bundle/:userId/:deviceId - Get user's key bundle
- DELETE /api/v1/keys/device/:deviceId - Remove device keys

### Estimated Time: 2-3 days

---

## 📊 Phase 8: Observability & Monitoring

### Goal
Add comprehensive logging, metrics, and health checks for production readiness.

### Tasks
1. Enhance structured logging with request context
2. Add request ID propagation through all layers
3. Implement Prometheus metrics endpoints
4. Create custom metrics for business operations
5. Add distributed tracing preparation
6. Implement health check endpoints
7. Create readiness and liveness probes
8. Add database connection monitoring
9. Implement Redis connection monitoring
10. Create alerting thresholds documentation

### Metrics to Track
- HTTP request duration and count by endpoint
- HTTP error rates by status code
- WebSocket active connections
- WebSocket messages per second
- Message send throughput
- Outbox queue depth and processing time
- Database query latency
- Redis operation latency
- Authentication success/failure rates
- Active user sessions

### Health Endpoints
- GET /health - Basic health check
- GET /health/ready - Readiness probe (DB + Redis connected)
- GET /health/live - Liveness probe
- GET /metrics - Prometheus metrics

### Estimated Time: 2 days

---

## 🧪 Phase 9: Testing

### Goal
Comprehensive test coverage for reliability and maintainability.

### Tasks
1. Write unit tests for all repository methods
2. Write unit tests for all service methods
3. Write unit tests for handlers with mocked dependencies
4. Create integration tests with test database
5. Implement WebSocket connection tests
6. Create API endpoint integration tests
7. Add authentication flow tests
8. Implement message flow end-to-end tests
9. Create load testing scripts
10. Add CI pipeline with test automation

### Test Categories
- Unit Tests: Repository, Service, Handler layers
- Integration Tests: Database operations, Redis operations
- API Tests: Full endpoint testing with test server
- WebSocket Tests: Connection, messaging, presence
- Load Tests: Concurrent users, message throughput

### Test Structure
```
internal/
├── repository/
│   └── *_test.go
├── service/
│   └── *_test.go
├── handler/
│   └── *_test.go
tests/
├── integration/
│   ├── auth_test.go
│   ├── message_test.go
│   └── websocket_test.go
└── load/
    └── scenarios/
```

### Estimated Time: 3-4 days

---

## 🚢 Phase 10: Deployment & Operations

### Goal
Production-ready containerization, CI/CD, and operational documentation.

### Tasks
1. Create multi-stage Dockerfile for production build
2. Create production docker-compose configuration
3. Document all environment variables
4. Create secrets management strategy
5. Set up GitHub Actions CI/CD pipeline
6. Implement database migration in deployment
7. Create backup and restore procedures
8. Document scaling strategies
9. Create runbook for common operations
10. Set up log aggregation strategy

### Deployment Artifacts
- Dockerfile (multi-stage, minimal image)
- docker-compose.prod.yml
- .env.example with all variables documented
- GitHub Actions workflow for CI/CD
- Kubernetes manifests (optional)
- Database backup scripts
- Operational runbook

### CI/CD Pipeline Stages
1. Lint (golangci-lint)
2. Test (unit + integration)
3. Build (Go binary)
4. Docker Build
5. Push to Registry
6. Deploy to Staging
7. Deploy to Production (manual approval)

### Estimated Time: 2 days

---

## 📅 Timeline Summary

| Phase | Description | Duration | Priority |
|-------|-------------|----------|----------|
| 0 | Infrastructure & Foundation | ✅ DONE | - |
| 1 | Repository Layer | 2-3 days | 🔴 High |
| 2 | Authentication | 3-4 days | 🔴 High |
| 3 | HTTP API | 4-5 days | 🔴 High |
| 4 | WebSocket | 4-5 days | 🔴 High |
| 5 | Event System | 2-3 days | 🟡 Medium |
| 6 | Calls | 3-4 days | 🟡 Medium |
| 7 | E2E Encryption | 2-3 days | 🟡 Medium |
| 8 | Observability | 2 days | 🟢 Low |
| 9 | Testing | 3-4 days | 🟡 Medium |
| 10 | Deployment | 2 days | 🟢 Low |

**Total Estimated Time: 4-6 weeks**

---

## 🎯 Immediate Next Steps

1. Create repository interfaces in `internal/repository/interfaces.go`
2. Implement UserRepository with all CRUD operations
3. Implement ConversationRepository with participant management
4. Implement MessageRepository with pagination
5. Set up Gin HTTP server in `internal/server/`
6. Create auth service with JWT token handling
7. Build registration and login endpoints
8. Add auth middleware for protected routes

---

## 📝 Development Guidelines

### Code Standards
- Follow Go naming conventions (CamelCase for exports)
- Use domain language from database schema
- Prefer explicit types over `interface{}`
- Always wrap errors with context
- Use structured logging with relevant fields

### Git Workflow
- Feature branches from `main`
- Pull requests with code review
- Squash merge to keep history clean
- Semantic commit messages

### API Conventions
- RESTful resource naming
- Consistent error response format
- Proper HTTP status codes
- Request validation at handler layer
- Pagination with cursor-based approach
