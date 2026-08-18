# API Architecture

## 1. Purpose

This document defines the mobile-facing API architecture, transport boundaries,
authentication, authorization, errors, idempotency, pagination, compatibility,
and application-layer responsibilities.

The core architecture is:

```text
Flutter
   ↓
HTTP API / WebSocket
   ↓
Authentication
   ↓
Authorization
   ↓
Application layer
   ↓
Domain
   ↓
PostgreSQL / Redis / external services
```

HTTP handlers are transport adapters. They must not become the application's
business-logic layer.

---

# 2. Core Principles

1. Use REST/JSON over HTTPS for the primary mobile-facing API.
2. Use WebSockets separately for realtime delivery.
3. Version the public API explicitly.
4. Authentication and authorization are separate concerns.
5. Clients request business operations; they do not directly mutate lifecycle state.
6. API handlers invoke application use cases rather than implementing business rules.
7. Domain/application errors map to stable HTTP error codes.
8. Effectful commands use idempotency where appropriate.
9. Public API contracts must not expose database implementation details.
10. PostgreSQL transaction boundaries are defined by business use cases, not HTTP handlers.
11. External dependency calls require bounded timeouts.
12. API compatibility must be intentional.
13. API and WebSocket contracts should use consistent domain vocabulary.
14. The API remains the authoritative fallback for client state reconciliation.
15. OpenAPI will document the concrete HTTP contract.

---

# 3. REST and WebSocket Boundary

Use:

```text
REST
  ↓
commands + queries

WebSocket
  ↓
realtime events
```

The same business operation should not require an open WebSocket connection.

Example:

```text
POST /api/v1/rides/{id}/cancel
      ↓
CancelRide use case
      ↓
PostgreSQL transaction
      ↓
outbox
      ↓
WebSocket/push delivery
```

---

# 4. API Versioning

The initial public API uses:

```text
/api/v1/...
```

Examples:

```text
POST /api/v1/rides
GET  /api/v1/rides/{ride_id}
POST /api/v1/rides/{ride_id}/cancel
```

Breaking API changes should not silently alter the meaning of an existing
version.

---

# 5. Authentication

Authentication uses the external OIDC provider selected by the project.

The API receives:

```http
Authorization: Bearer <access_token>
```

The backend:

```text
verify token
    ↓
extract identity/subject
    ↓
resolve application user
    ↓
authorize operation
```

Client-supplied user IDs are not proof of identity.

---

# 6. Authorization

Authentication answers:

```text
Who are you?
```

Authorization answers:

```text
What are you allowed to do?
```

Every protected resource and command requires an authorization decision.

---

# 7. Resource Ownership

A resource endpoint must verify the requester's relationship to the resource.

For example:

```text
GET /api/v1/rides/{ride_id}
```

may be authorized for:


```text
ride owner
relevant assigned driver
explicitly authorized operational actor
```

A random authenticated user must not gain access merely by knowing the ride ID.

---

# 8. Commands vs Queries

Queries read state:

```text
GET /api/v1/rides/{id}
```

Commands cause business transitions:

```text
POST /api/v1/rides/{id}/cancel
```

Avoid generic status mutation such as:

```http
PATCH /rides/{id}
Content-Type: application/json

{"status":"cancelled"}
```

The client requests `CancelRide`; the backend determines whether that
transition is valid.

---

# 9. Resource URLs

Use nouns for resources:

```text
/rides
/drivers
/bids
/reservations
/payments
```

Use explicit action endpoints when a business command is clearer than generic
CRUD:

```text
POST /rides/{id}/cancel
POST /rides/{id}/start
POST /rides/{id}/complete
```

---

# 10. HTTP Methods

Baseline conventions:

```text
GET    → query
POST   → create/command
PATCH  → partial resource update where appropriate
DELETE → actual resource deletion where appropriate
```

Do not use `DELETE` to represent cancellation.

---

# 11. HTTP Status Codes

Use conventional HTTP semantics.

```text
200 OK
201 Created
202 Accepted
204 No Content

400 Bad Request
401 Unauthorized
403 Forbidden
404 Not Found
409 Conflict
422 Unprocessable Entity
429 Too Many Requests

500 Internal Server Error
502 Bad Gateway
503 Service Unavailable
```

The exact mapping for each application error is standardized in the API
contract.

---

# 12. 401 vs 403

Use:

```text
401
→ authentication missing/invalid

403
→ authenticated but not authorized
```

Do not use `403` as a generic authentication failure.

---

# 13. 404 and Resource Privacy

For sensitive resources, the API may intentionally return `404 Not Found` when
the requester is not permitted to discover the resource.

This prevents unauthorized callers from learning that a resource exists.

The behavior should be consistent across resource families.

---

# 14. 409 Conflict

Use `409 Conflict` for valid requests that conflict with authoritative current
state or concurrency.

Examples:

```text
ride already cancelled
bidding already closed
reservation no longer available
bid cannot be accepted in current state
```

---

# 15. Error Envelope

Use one stable error shape.

Example:

```json
{
  "error": {
    "code": "RIDE_ALREADY_CANCELLED",
    "message": "The ride has already been cancelled.",
    "request_id": "req_123"
  }
}
```

The machine-readable contract is `code`.

The `message` is for humans and should not be parsed by clients for behavior.

---

# 16. Validation Errors

Validation errors should identify fields where possible.

Example:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "Request validation failed.",
    "fields": {
      "amount": "must be greater than zero"
    },
    "request_id": "req_123"
  }
}
```

---

# 17. Stable Error Codes

Application errors should use stable codes such as:

```text
INVALID_REQUEST
AUTHENTICATION_REQUIRED
FORBIDDEN
RESOURCE_NOT_FOUND
RIDE_NOT_CANCELLABLE
BIDDING_CLOSED
BID_AMOUNT_INVALID
RESERVATION_EXPIRED
CONFLICT
RATE_LIMITED
DEPENDENCY_UNAVAILABLE
INTERNAL_ERROR
```

Do not expose raw Go error strings as the public contract.

---

# 18. Idempotency

Commands that create business/external effects should support idempotency where
appropriate.

Examples:

```text
POST /rides
POST /rides/{id}/bids
POST /rides/{id}/cancel
POST /payments/{id}/capture
```

Use:

```http
Idempotency-Key: <unique-client-generated-key>
```

where the operation requires it.

---

# 19. Idempotency Semantics

The same idempotency key represents the same logical command.

Example:

```text
request A + key abc123
request A + key abc123
```

must resolve to the same logical operation/result rather than create duplicate
side effects.

---

# 20. Idempotency Key Reuse

If a client reuses a key for a materially different request:

```text
key abc123 → request A
key abc123 → request B
```

the second request should be rejected.

Do not silently reinterpret an idempotency key.

---

# 21. Authentication Token Placement

Access tokens must not be placed in URLs.

Bad:

```text
GET /rides?token=...
```

Use the `Authorization` header instead.

---

# 22. Pagination

Collection endpoints should support pagination.

Prefer cursor-based pagination for large or frequently changing collections.

Example:

```text
GET /api/v1/rides?limit=20&cursor=...
```

Do not rely exclusively on large numeric offsets for operationally large data.

---

# 23. Pagination Response

Conceptual response:

```json
{
  "data": [],
  "pagination": {
    "next_cursor": "abc123",
    "has_more": true
  }
}
```

The exact envelope will be standardized when concrete API contracts are written.

---

# 24. Sorting

Collection endpoints must define explicit ordering.

Do not rely on accidental PostgreSQL row order.

A deterministic ordering may use:

```text
created_at DESC
id DESC
```

or an equivalent stable combination.

---

# 25. Filtering

Only expose explicitly supported filters.

Example:

```text
GET /api/v1/rides?status=completed
```

Do not build a generic SQL-like filtering language into the API.

---

# 26. Request IDs

Every request should receive or propagate a request ID.

Conceptually:

```http
X-Request-ID: req_123
```

The ID should be available in:

```text
response
logs
traces
relevant downstream operations
```

---

# 27. Correlation

Asynchronous workflows should preserve useful correlation identifiers.

Potential identifiers include:

```text
request_id
ride_id
event_id
payment_id
```

These identifiers support debugging across API, domain, event, payment, and
notification workflows.

---

# 28. Time Representation

API timestamps should use RFC 3339/ISO 8601 representations.

Example:

```text
2026-08-18T10:30:00Z
```

Do not expose ambiguous locale-specific timestamps.

---

# 29. Money Representation

Money must follow the exact money model defined by the pricing domain.

Conceptually:

```json
{
  "amount": 1200,
  "currency": "PKR"
}
```

The exact minor-unit convention must remain consistent with the domain money
representation.

Never use floating-point monetary values in the API contract.

---

# 30. Resource Representations

API responses represent business concepts, not database rows.

Bad:

```json
{
  "postgres_row_id": 48392,
  "internal_status_code": 7,
  "driver_fk": 8392
}
```

Better:

```json
{
  "id": "ride_123",
  "status": "driver_en_route",
  "driver": {
    "id": "driver_456"
  }
}
```

---

# 31. Public IDs

Public APIs should use stable public identifiers rather than exposing sequential
database IDs by default.

Conceptually:

```text
ride_...
driver_...
bid_...
```

The exact ID generation strategy is a database/application design decision.

---

# 32. API and WebSocket Vocabulary

REST and WebSocket contracts should use consistent domain terminology.

Example:

```text
REST:
status = driver_en_route

WebSocket:
type = ride.driver_en_route
```

They should not invent contradictory names for the same business concept.

---

# 33. API and Domain Events

Internal domain events should not automatically become public API contracts.

Use an explicit mapping:

```text
Domain Event
    ↓
Client-facing event model
```

This prevents internal implementation changes from unnecessarily breaking Flutter.

---

# 34. API and Notifications

Notifications should consume durable events rather than querying arbitrary API
endpoints to discover what happened.

The API is primarily responsible for:

```text
commands
current state queries
historical queries
state reconciliation
```

---

# 35. API and External Services

HTTP handlers should not directly orchestrate every external dependency.

Bad:

```text
HTTP handler
 ↓
Google Maps
 ↓
Redis
 ↓
PostgreSQL
 ↓
payment provider
 ↓
notification provider
```

Better:

```text
HTTP handler
 ↓
application use case
 ↓
domain/services
 ↓
infrastructure adapters
```

---

# 36. Timeouts

Every external dependency call requires a bounded timeout.

Examples:

```text
Google Maps
payment provider
OIDC/JWKS retrieval
notification provider
other external services
```

An HTTP request must not wait indefinitely for a downstream dependency.

---

# 37. Rate Limiting

Rate limits may operate at multiple scopes:

```text
IP
authenticated user
device/session
sensitive operation
```

Protect operations such as:

```text
authentication
ride creation
bid creation
cancellation
payment operations
location updates
```

Do not use one undifferentiated global limit for all operations.

---

# 38. Abuse Protection

Infrastructure rate limiting and business abuse limits are separate.

Business examples:

```text
excessive bids
rapid ride creation
repeated cancellations
excessive location updates
```

The exact policies belong to their respective domains.

---

# 39. Request Size Limits

Endpoints should enforce reasonable request-body and field-size limits.

This prevents malicious or accidental oversized payloads.

The same principle applies to any future file-upload endpoint.

---

# 40. Content Types

The primary API content type is:

```text
application/json
```

Unsupported content types should be rejected consistently.

---

# 41. Compatibility

Breaking API changes must be intentional.

Examples include:

```text
removing a field
changing a field type
changing enum meaning
making an optional field required
changing endpoint semantics
```

Use a new API version or another explicit compatibility mechanism for breaking
changes.

---

# 42. Enum Evolution

Clients should tolerate unknown enum values where practical.

For example, if a new server state appears:

```text
DRIVER_WAITING
```

an older Flutter client should fail gracefully rather than crash because the
value is unfamiliar.

---

# 43. Request Validation Boundary

API-layer validation handles syntax and basic structure:

```text
required fields
types
formats
field sizes
basic ranges
```

Application/domain validation handles business rules:

```text
ride state
ownership
bid eligibility
cancellation policy
payment state
```

Do not move all business validation into HTTP handlers or JSON schemas.

---

# 44. Transaction Boundary

HTTP handlers do not define business transaction boundaries.

Application use cases do.

Example:

```text
CancelRide
  ↓
PostgreSQL transaction
  ├── ride state
  ├── reservation
  ├── assignment
  ├── cancellation record
  └── outbox
```

The handler invokes the use case and translates its result into HTTP.

---

# 45. Long-Running Operations

Do not hold HTTP connections open indefinitely for slow external workflows.

Where appropriate, asynchronous operations may return:

```text
202 Accepted
```

and expose a way to observe the resulting state.

Do not make ordinary ride commands asynchronous without a real requirement.

---

# 46. API Caching

Cache only responses whose consistency model permits it.

Potentially cacheable:

```text
static metadata
stable configuration
```

Require careful handling:

```text
ride status
bid status
payment status
driver availability
```

Do not blindly cache mutable lifecycle or financial state.

---

# 47. Conditional Requests

ETags or conditional requests may be introduced for suitable read-heavy
resources later.

They are an optimization, not a replacement for authoritative state handling.

---

# 48. OpenAPI

Concrete HTTP endpoints should eventually be documented using OpenAPI.

OpenAPI becomes the contract for:

```text
Flutter integration
API integration tests
external clients where applicable
```

OpenAPI should describe the API contract rather than become the location of
business architecture.

---

# 49. Testing

API tests should cover:

```text
authentication
authorization
validation
success responses
error responses
idempotency
concurrency conflicts
pagination
rate limits
resource ownership
state-transition conflicts
```

Important business rules must also be tested directly at application/domain
layers rather than relying only on HTTP integration tests.

---

# 50. API Error Mapping

The application layer should expose typed domain/application errors.

The API layer maps them to HTTP responses.

Example:

```text
RideAlreadyCancelled
        ↓
409 Conflict
        ↓
RIDE_ALREADY_CANCELLED
```

This prevents transport-specific concepts from leaking into the domain.

---

# 51. Logging

Do not log sensitive authentication or payment information.

Never log:

```text
access tokens
raw payment credentials
unnecessarily complete sensitive payloads
```

Useful structured fields include:

```text
request_id
route
method
status
duration
ride_id where appropriate
```

---

# 52. Security

The production API should require HTTPS/TLS.

Appropriate security headers and transport configuration belong to the
reverse-proxy/deployment design.

The application should assume authenticated production traffic is encrypted in
transit.

---

# 53. Observability

Useful metrics include:

```text
http_requests_total
http_request_duration
http_request_errors
http_authentication_failures
http_authorization_failures
http_rate_limited
http_idempotency_replays
```

Useful tracing fields include:

```text
request_id
route
method
status
ride_id
user_id where appropriate
```

Avoid high-cardinality identifiers as metric labels.

---

# 54. What We Should Not Build Yet

Do not build:

```text
GraphQL API
public API marketplace
generic query language
automatic API generation from database tables
multiple API versions before V1 exists
custom RPC protocol for mobile clients
complex API gateway platform
```

The initial system needs a clear REST API, WebSockets, authentication,
authorization, and reliable contracts.

---

# 55. Complete API Architecture

```text
                       Flutter
                          │
              ┌───────────┴───────────┐
              ▼                       ▼
           REST API               WebSocket
              │                       │
              └───────────┬───────────┘
                          ▼
                 Authentication
                          │
                          ▼
                  Authorization
                          │
                          ▼
                 Application Layer
                          │
                          ▼
                      Domain
                          │
             ┌────────────┼────────────┐
             ▼            ▼            ▼
        PostgreSQL      Redis      External APIs
```

---

# 56. Design Principles

1. REST/JSON over HTTPS is the primary mobile API.
2. WebSocket is a separate realtime delivery mechanism.
3. Public APIs are explicitly versioned.
4. Authentication uses the external OIDC provider.
5. Authorization is enforced for every protected resource and command.
6. Clients request business operations rather than directly mutating lifecycle state.
7. HTTP handlers remain thin transport adapters.
8. Application use cases own business orchestration and transaction boundaries.
9. Domain errors map to stable API error codes.
10. Effectful commands use idempotency where required.
11. Public API representations do not expose database internals.
12. Cursor pagination is preferred for large/changing collections.
13. Request IDs and correlation identifiers support observability.
14. Money and timestamps use explicit, unambiguous representations.
15. External calls use bounded timeouts.
16. Rate limiting and domain abuse protection are separate concerns.
17. API compatibility changes must be intentional.
18. REST and WebSocket contracts share domain vocabulary.
19. API state queries provide the recovery path for realtime disconnection.
20. OpenAPI documents the concrete contract.
21. Avoid adding API complexity until the product requires it.
