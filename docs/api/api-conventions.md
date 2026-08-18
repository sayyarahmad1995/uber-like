# API Conventions

## 1. Purpose

This document defines the common API conventions for the Uber-like application.

The goal is to establish consistent rules before individual resource APIs are
designed.

The API serves the Flutter application through the API Gateway and Go backend.

```text
Flutter
   ↓
API Gateway
   ↓
Go application
   ↓
PostgreSQL / Redis / external providers
```

---

# 2. Core Principles

1. The backend is authoritative for business state.
2. Clients send commands; they do not directly mutate business state.
3. Authentication and authorization are separate concerns.
4. Mobile retries must be safe for mutation operations.
5. Concurrency must be explicit and detectable.
6. REST provides commands and authoritative state retrieval.
7. WebSocket provides real-time notifications, not business authority.
8. API conventions should remain simple until actual requirements justify more complexity.

---

# 3. API Versioning

The initial API uses an explicit major version:

```text
/api/v1/
```

Example:

```text
POST /api/v1/rides
```

Breaking changes require a new major API version.

Non-breaking additions should normally remain within the existing version.

---

# 4. Resource Naming

Use plural nouns for resource collections.

Examples:

```text
/rides
/drivers
/vehicles
```

Nested resources may be used when ownership is clear:

```text
/rides/{ride_id}/bids
/drivers/{driver_id}/vehicles
```

Avoid deeply nested paths that encode complicated business workflows.

---

# 5. Commands vs Arbitrary CRUD

Business operations should be represented as explicit commands when they cause
state transitions.

Prefer:

```text
POST /api/v1/rides/{ride_id}/cancel
```

over:

```text
PATCH /api/v1/rides/{ride_id}
{
  "status": "CANCELLED"
}
```

Similarly:

```text
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

rather than allowing the client to directly modify assignment state.

This makes authorization, validation, idempotency, and concurrency explicit.

---

# 6. HTTP Methods

Use methods consistently.

```text
GET
    retrieve state

POST
    create resource or execute a command

PATCH
    partial update where direct mutation is genuinely appropriate

DELETE
    remove a resource where deletion is actually the domain operation
```

Do not use PATCH as a generic state-transition mechanism.

---

# 7. Authentication

Authentication uses the external OIDC provider.

Conceptually:

```text
Flutter
   ↓
OIDC authentication
   ↓
access token
   ↓
API Gateway
   ↓
Go backend
```

The backend must validate the authenticated identity according to the chosen
OIDC integration.

The client must not establish its identity by sending an arbitrary:

```json
{
  "user_id": "..."
}
```

The authenticated identity is authoritative.

---

# 8. Application Identity

The external OIDC subject should be mapped to an internal application user.

Conceptually:

```text
OIDC subject
    ↓
application user
    ↓
rider / driver roles
```

Business resources should reference internal application identities rather than
making external OIDC identifiers the entire domain model.

---

# 9. Authorization

Authentication answers:

> Who is the caller?

Authorization answers:

> Is this caller allowed to perform this operation on this resource?

Examples:

```text
Rider A
   ↓
GET Ride A
   ✓

Rider A
   ↓
GET Ride B
   ✗
```

and:

```text
Driver A
   ↓
submit bid as Driver B
   ✗
```

Authorization must be enforced server-side.

---

# 10. API Gateway Responsibilities

The API Gateway may handle cross-cutting concerns such as:

```text
TLS termination
request routing
OIDC/token validation
request ID propagation
coarse rate limiting
connection management
```

The gateway should not own ride/dispatch business logic.

It should not decide:

```text
which driver wins
whether a bid is eligible
whether a ride can transition state
whether a reservation is valid
```

Those decisions belong to the Go application/domain layer.

---

# 11. Go Application Responsibilities

The Go backend owns:

```text
authorization
business validation
state transitions
concurrency control
idempotency
persistence
transaction boundaries
domain events
```

External provider integrations such as Google Maps remain behind explicit
application boundaries.

---

# 12. REST and WebSocket Boundary

REST is used for:

```text
commands
authoritative state retrieval
state recovery
resource queries
```

WebSocket is used for:

```text
real-time notifications
live ride updates
bid updates
operational events
```

WebSocket messages must not become the authoritative source of business state.

---

# 13. WebSocket Recovery

If a client misses an event:

```text
WebSocket disconnect
    ↓
client reconnects
    ↓
GET authoritative resource state
```

The client should reconcile its local state with the REST response.

---

# 14. Request ID

Every API request should have a request/correlation identifier.

Conceptually:

```text
X-Request-ID: <request-id>
```

If the client provides a valid request ID, the gateway may propagate it.

Otherwise the gateway/backend should generate one.

The identifier should be included in logs and traces.

The request ID is for observability and is not an idempotency key.

---

# 15. Idempotency

Mutation operations that may be retried by mobile clients should support
idempotency.

The client may send:

```text
Idempotency-Key: <unique-key>
```

The key identifies one logical client operation.

Typical candidates include:

```text
create ride
create bid
select bid
cancel ride
confirm reservation
complete trip
```

Not every GET request requires an idempotency key.

---

# 16. Idempotency Semantics

Example:

```text
Flutter
   ↓
create ride + key K
   ↓
server creates Ride 123
   ↓
response lost
   ↓
Flutter retries key K
```

The second request must not create Ride 124.

It should return the result associated with the original operation where
appropriate.

An idempotency key must not be reused for a different logical operation.

---

# 17. Idempotency Storage

The exact implementation will be defined during infrastructure/database
design.

The important requirement is that idempotency state survives the relevant
application retry window and is consistent with the business operation.

Do not rely only on in-memory Go state.

---

# 18. Concurrency and Revisions

Resources that can be concurrently modified should expose a revision/version
where useful.

Example:

```text
bid revision = 4
```

A client updates revision 4.

If another request already changed it to revision 5, the stale request must not
silently overwrite revision 5.

---

# 19. Conflict Response

A stale or conflicting mutation should normally return:

```text
409 Conflict
```

Example:

```json
{
  "error": {
    "code": "BID_REVISION_CONFLICT",
    "message": "The bid has been modified.",
    "details": {}
  }
}
```

The client should retrieve current state and reconcile.

---

# 20. State Transition Errors

A command can also fail because the resource is in an incompatible business
state.

For example:

```text
select bid
```

when the ride is already completed.

This should use a stable application error code and an appropriate HTTP status,
usually `409 Conflict` when the conflict is caused by current resource state.

---

# 21. Error Envelope

Errors should use one consistent structure:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable description",
    "details": {}
  }
}
```

The `code` is intended for client logic.

The `message` is intended for diagnostics and safe presentation where
appropriate.

The `details` object is optional and must not expose sensitive internal data.

---

# 22. Error Codes

Error codes should describe stable application meaning.

Examples:

```text
INVALID_REQUEST
UNAUTHORIZED
FORBIDDEN
RESOURCE_NOT_FOUND
RIDE_NOT_BIDDING
BID_REVISION_CONFLICT
DRIVER_NOT_ELIGIBLE
DRIVER_NOT_AVAILABLE
RESERVATION_EXPIRED
RATE_LIMITED
```

Do not create hundreds of codes before the API actually needs them.

---

# 23. HTTP Status Codes

Initial conventions:

```text
200 OK
    successful retrieval or command returning a response body

201 Created
    resource successfully created

204 No Content
    successful operation with no response body

400 Bad Request
    malformed request syntax or invalid request structure

401 Unauthorized
    missing/invalid authentication

403 Forbidden
    authenticated caller lacks permission

404 Not Found
    requested resource does not exist or is intentionally undiscoverable

409 Conflict
    current resource state conflicts with the requested operation

422 Unprocessable Content
    syntactically valid request with domain validation failure

429 Too Many Requests
    rate limit exceeded

500 Internal Server Error
    unexpected server failure

503 Service Unavailable
    required service temporarily unavailable
```

The exact choice between `400` and `422` should remain consistent across the
API rather than being decided endpoint by endpoint.

---

# 24. Resource Not Found and Authorization

The API should avoid leaking sensitive resource existence when appropriate.

For example, an unauthorized user attempting to access another user's private
ride may receive `404 Not Found` rather than information revealing that the ride
exists.

The exact policy depends on resource sensitivity.

---

# 25. Validation

Client-side validation improves UX.

Server-side validation is authoritative.

The server validates:

```text
types
required fields
ranges
enums
business rules
authorization
current resource state
```

Never assume Flutter validation is sufficient.

---

# 26. Request Body Naming

Use consistent `snake_case` JSON field names.

Example:

```json
{
  "pickup_location": {
    "latitude": 31.5204,
    "longitude": 74.3587
  },
  "passenger_count": 2
}
```

The exact serialization convention can be revisited if another standard is
adopted across the entire project, but it must remain consistent.

---

# 27. Timestamps

API timestamps use timezone-safe ISO 8601/RFC 3339 representations.

Example:

```text
2026-08-18T10:30:00Z
```

Server-generated timestamps are authoritative.

Client timestamps must not be trusted for lifecycle ordering.

---

# 28. IDs

Public API resources should use stable identifiers.

The exact identifier format will be finalized during database/API design.

Clients should treat IDs as opaque strings.

Clients must not infer database structure from an ID.

---

# 29. Money

Money must not be represented using binary floating-point values in business
logic.

A monetary value should explicitly identify:

```text
amount
currency
```

Example:

```json
{
  "amount": "1200",
  "currency": "PKR"
}
```

The exact amount representation, including whether the API uses decimal strings
or minor units, will be finalized with the pricing/database design.

---

# 30. Location

Locations use a consistent geographic representation.

Example:

```json
{
  "latitude": 31.5204,
  "longitude": 74.3587
}
```

External Google Maps identifiers may be included as optional provider metadata,
but provider-specific objects should not become the core API location model.

---

# 31. Pagination

Collection endpoints should use cursor-based pagination where result sets can
be large or change while the client is paging.

Example:

```text
GET /api/v1/rides?limit=20&cursor=<cursor>
```

A response may contain:

```json
{
  "data": [],
  "pagination": {
    "next_cursor": "..."
  }
}
```

The exact envelope will be standardized during endpoint design.

---

# 32. Pagination Limits

The server controls maximum page size.

For example:

```text
requested limit = 5000
server maximum = 100
```

The server should cap or reject unreasonable values consistently.

Do not allow clients to turn collection endpoints into unbounded database
queries.

---

# 33. Filtering

Filtering uses explicit named parameters.

Example:

```text
GET /api/v1/rides?status=completed
```

Do not expose arbitrary SQL-like filtering expressions.

---

# 34. Sorting

Sorting should use a documented allowlist.

Example:

```text
?sort=created_at_desc
```

The API must not pass arbitrary client-provided sort expressions directly to
the database.

---

# 35. Rate Limiting

Rate limiting should exist at multiple levels where appropriate.

The API Gateway can provide coarse protection.

Application-level limits should protect expensive or abuse-prone operations.

Examples:

```text
ride creation
bid creation/update
availability changes
location updates
authentication-related endpoints
```

Exact limits should be based on product behavior, load testing, and abuse
analysis rather than guessed prematurely.

---

# 36. Location Update Rate

Driver location updates may be high-frequency.

They should not necessarily use the same API path and persistence strategy as
normal business commands.

The implementation should optimize for:

```text
low latency
controlled bandwidth
Redis writes
location freshness
```

while avoiding unnecessary PostgreSQL writes.

The exact transport and frequency will be designed separately.

---

# 37. Authentication Rate Limits

Authentication and token-related endpoints must have protection against abuse.

OIDC provider interactions should follow the provider's security and rate
limits.

The application should not implement a second incompatible authentication
system merely to compensate for poor API design.

---

# 38. Response Structure

Individual resource endpoints should return predictable JSON objects.

Example:

```json
{
  "data": {
    "id": "ride_123",
    "status": "BIDDING"
  }
}
```

Whether every endpoint uses a `data` wrapper or only collection endpoints will
be standardized before the OpenAPI contract is written.

The important requirement is consistency.

---

# 39. No Arbitrary Internal Fields

API responses should not expose database rows directly.

Do not return internal fields merely because they exist in PostgreSQL.

The API response is a deliberate contract.

For example, internal fields such as:

```text
internal_version
internal_flags
database timestamps used only for maintenance
internal provider metadata
```

should not automatically become public API fields.

---

# 40. Authorization Before Sensitive Data

The backend should authorize access before returning sensitive information.

Examples include:

```text
rider location
pickup address
driver location
vehicle registration information
internal operational state
```

Do not rely on the Flutter client to hide sensitive response fields.

---

# 41. Command Responses

Commands should return enough authoritative state for the client to update
itself without immediately making another request when practical.

For example:

```text
cancel ride
   ↓
200/204 + authoritative cancellation result
```

The exact response depends on whether the command creates or changes a resource
whose current representation is useful to the client.

---

# 42. Long-Running Operations

Operations that may take significant time should not block HTTP requests while
waiting for unrelated external processing.

For example:

```text
ride creation
   ↓
commit transaction
   ↓
return success
   ↓
background discovery
```

The API should not keep the rider request open while waiting for drivers to be
found.

---

# 43. External Provider Calls

The API should avoid coupling critical database transactions to long-running
external provider calls when possible.

For example, Google Maps routing should not unnecessarily hold a PostgreSQL
transaction open.

Prefer:

```text
validate
commit domain state
call external provider when appropriate
store/use result
```

The exact sequence depends on the business operation.

---

# 44. Retries

Clients and internal services should retry only operations that are safe to
retry.

Retry behavior must respect:

```text
idempotency
current resource state
rate limits
external provider limits
```

Blindly retrying every `500` response can create duplicate business operations
if the endpoint is not idempotent.

---

# 45. Timeouts

Every external request should have bounded timeouts.

This includes calls to:

```text
Google Maps
Redis
PostgreSQL
other internal services
```

Timeout values are implementation/configuration decisions.

An API request must not remain blocked indefinitely because an external
dependency stopped responding.

---

# 46. API Gateway Failure

The gateway is an infrastructure dependency, not the business source of truth.

If the gateway is unavailable, clients may be unable to reach the API, but
business state remains durable in PostgreSQL.

---

# 47. Security Headers and Transport

Production API traffic must use TLS.

The gateway should enforce appropriate transport/security policies.

Sensitive tokens must not be placed in URLs.

Do not put access tokens, secrets, or private location data into query
parameters merely for convenience.

---

# 48. Logging

Logs should include:

```text
request_id
operation
HTTP method/path
status
latency
actor/application identity where appropriate
```

Do not log:

```text
access tokens
OIDC credentials
passwords
secrets
unnecessary sensitive location data
```

---

# 49. Tracing

Distributed traces should propagate the request/correlation ID across:

```text
API Gateway
   ↓
Go application
   ↓
PostgreSQL / Redis / external APIs
```

Trace attributes should avoid unnecessary personal or sensitive information.

---

# 50. API Evolution

Non-breaking changes may include:

```text
new optional response field
new endpoint
new supported filter
```

Breaking changes may include:

```text
removing a field
changing field meaning
changing required input
changing an existing enum incompatibly
changing response semantics
```

Breaking changes require an explicit versioning strategy.

---

# 51. Deprecation

Deprecated API behavior should have a defined migration path.

Do not silently change the meaning of an existing field.

When an API is deprecated, document:

```text
replacement
migration period
removal/version plan
```

---

# 52. OpenAPI

An OpenAPI specification should be created after the domain APIs and common
conventions are sufficiently stable.

It should become the executable API contract for:

```text
backend validation
documentation
client generation where useful
integration testing
```

Do not generate a large OpenAPI specification before the domain model is ready.

---

# 53. Example Ride Creation

Conceptually:

```http
POST /api/v1/rides
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

```json
{
  "pickup_location": {
    "latitude": 31.5204,
    "longitude": 74.3587
  },
  "destination_location": {
    "latitude": 31.5497,
    "longitude": 74.3436
  },
  "service_type": "standard",
  "passenger_count": 1,
  "currency": "PKR"
}
```

The backend validates and creates the ride.

It does not wait for driver discovery before responding.

---

# 54. Example Bid Selection

Conceptually:

```http
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
```

The backend atomically validates:

```text
rider authorization
ride state
bid state
bid ownership
reservation availability
```

If successful:

```text
bidding closes
reservation created
ride → DRIVER_CONFIRMATION_REQUIRED
```

---

# 55. Example Conflict

```http
409 Conflict
Content-Type: application/json
```

```json
{
  "error": {
    "code": "DRIVER_NOT_AVAILABLE",
    "message": "The selected driver is no longer available.",
    "details": {}
  }
}
```

The client should refresh the relevant ride/bid state and follow the fallback
flow.

---

# 56. What We Should Not Build Yet

Do not build:

```text
OpenAPI for every future endpoint
GraphQL
arbitrary query languages
API gateway business workflows
client-controlled state mutation
custom authentication protocol
multiple API versions without need
complex generic error frameworks
```

The API should remain boring, predictable, and explicit.

---

# 57. Design Principles

1. Use `/api/v1` for the initial API.
2. Use resource-oriented paths for retrieval and collections.
3. Use explicit commands for meaningful business state transitions.
4. Never allow clients to arbitrarily mutate business state.
5. External OIDC authenticates identity; application authorization remains server-side.
6. The API Gateway handles cross-cutting infrastructure concerns, not domain decisions.
7. Go owns business validation and state transitions.
8. REST is authoritative for commands and state retrieval.
9. WebSocket is for real-time notifications and recovery still uses REST.
10. Mobile mutations must support idempotency where retries can duplicate operations.
11. Revision/version checks protect concurrent updates.
12. `409 Conflict` represents meaningful state/concurrency conflicts.
13. Error responses use stable machine-readable error codes.
14. Timestamps are timezone-safe and server-authoritative.
15. Money must not rely on floating-point business arithmetic.
16. Locations use a stable core representation independent of Google Maps provider objects.
17. Collection endpoints use bounded pagination.
18. Filtering and sorting use explicit allowlists.
19. Rate limits protect both infrastructure and business operations.
20. API responses are deliberate contracts, not raw database rows.
21. Sensitive data is protected by server-side authorization.
22. External dependencies require bounded timeouts.
23. Retries must respect idempotency and resource state.
24. Breaking API changes require explicit versioning.
25. OpenAPI should be finalized after domain APIs stabilize.
