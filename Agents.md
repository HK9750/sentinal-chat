# Agents Guide

This document defines the architectural standards and design patterns for the Sentinal Chat codebase. It reflects the current implementation (Service/Repository pattern with Redis signaling) and outlines future architectural goals.

## Codebase Structure Map

```
cmd/
  api/main.go          # HTTP API entry point
  migrate/main.go      # Database migration tool
config/                # Configuration loading (env vars)
internal/
  domain/              # Domain models (structs)
  handler/             # HTTP handlers (GIN)
  middleware/          # HTTP middleware (Auth, RateLimit)
  repository/          # Data access layer (PostgreSQL/GORM)
  server/              # Server setup and route registration
  services/            # Business logic
  redis/               # Redis wrappers (Signaling, RateLimit)
  transport/httpdto/   # Request/Response DTOs
pkg/
  database/            # Database connection helpers
  errors/              # Standardized error definitions
  logger/              # Structured logging
docs/
  database.md          # Database schema documentation
```

## Current Architecture: Service-Repository & Redis

The application currently follows a monolithic **Layered Architecture**:

1.  **Handler**: Receives HTTP request, validates DTO, calls Service.
2.  **Service**: Executes business logic, enforces permissions, calls Repository.
3.  **Repository**: Executes database queries using GORM within a transaction context if needed.
4.  **Redis**: Used for short-lived state (e.g., Signaling, Rate Limiting) and Pub/Sub.

### Transaction Management
Database transactions are managed within the **Service** layer using `gorm.DB` transaction blocks when multiple repository calls must be atomic.

### Real-Time Signaling (Redis)
Real-time features (like WebRTC signaling) use Redis Pub/Sub and Lists.
-   **Files**: `internal/redis/signaling.go`
-   **Mechanism**: Services push signaling messages (offers, answers, ICE candidates) to Redis.
-   **Current State**: Clients poll or subscribe via a separate mechanism (implementation details handled in `call_service.go` and `signaling.go`).

## Future Architecture: Event-Driven Outbox Flow

> **Note**: The following patterns are **aspirational goals** for future iterations to improve scalability and reliability. They are NOT strictly enforced in the current codebase but should be kept in mind for major refactors.

### Transaction Rule (Future)
All state-changing commands should write domain state and an outbox event in the same database transaction.

### Event Emission Pipeline (Future)
1.  Handler receives request.
2.  Service validates and builds a command.
3.  Repository writes data AND outbox record.
4.  Worker publishes to Redis.
5.  WebSocket observers fan-out to clients.

### Redis Channel Taxonomy
```
channel:conversation:{conversation_id}
channel:user:{user_id}
channel:presence:{user_id}
channel:call:{call_id}
channel:system:outbox
```

## Design Patterns Usage

### 1) Service & Repository Pattern (Primary)
**Current Implementation**:
-   **Services** (`internal/services`): Contain all business logic. They accept simple structs or DTOs.
-   **Repositories** (`internal/repository`): Strictly for database access.

### 2) Command Pattern (Implicit)
**Current Implementation**:
-   Service methods accept "Input" structs (e.g., `SendMessageInput`) which act as command objects containing all necessary data and validation logic.

### 3) Observer Pattern (via Redis)
**Current Implementation**:
-   The `SignalingStore` (`internal/redis/signaling.go`) acts as a distributed observer, publishing events to channels that interested parties (e.g., other server instances or connected clients) subscribe to.

### 4) Proxy Pattern
**Current Implementation**:
-   **Middleware** (`internal/middleware`): Acts as a proxy for Auth, Rate Limiting, and Logging before requests reach handlers.

## Transport Layer Standards

All request and response DTOs must live in `internal/transport/httpdto`.

### Standard Response Shape
```go
type Response[T any] struct {
  Success bool   `json:"success"`
  Data    T      `json:"data,omitempty"`
  Error   string `json:"error,omitempty"`
  Code    string `json:"code,omitempty"`
}
```

### Handler Requirements
-   Return `NewSuccessResponse(data)` on success.
-   Return `NewErrorResponse(message, code)` on failure.
-   Use `http.StatusOK` for successful operations, even if the business logic result implies a "soft" failure (unless it's a protocol error).

## Agent Notes & Roadmap

-   **Priority 1**: Maintain consistency with `httpdto` for all new endpoints.
-   **Priority 2**: Use `internal/redis` for any new real-time features instead of local state.
-   **Future Work**:
    -   Implement the "Outbox Pattern" worker to reliably publish DB events to Redis.
    -   Implement a full-fledged WebSocket Hub in `internal/server` for bi-directional real-time communication beyond simple signaling.
