# API Contract

## 1. Purpose

This document turns the API architecture and domain state machines into a concrete
mobile-facing contract.

The API is a transport boundary over application use cases. It does not expose
PostgreSQL tables or permit arbitrary lifecycle mutation.

Primary transport:

```text
HTTPS + JSON
```

Realtime transport:

```text
WSS
```

Base path:

```text
/api/v1
```

---

# 2. Contract Principles

1. Clients issue commands; they do not set arbitrary statuses.
2. Authentication uses the external OIDC provider.
3. Authorization is evaluated server-side for every protected resource/command.
4. HTTP handlers invoke application use cases.
5. Public responses represent domain resources, not database rows.
6. Machine-readable error codes are stable API contracts.
7. Effectful commands use idempotency where retries can create duplicate effects.
8. The API remains authoritative for state reconciliation.
9. WebSocket events are delivery mechanisms, not command mechanisms.
10. Breaking changes require explicit versioning/compatibility decisions.

---

# 3. Authentication

Protected endpoints require:

```http
Authorization: Bearer <oidc-access-token>
```

The backend validates:

```text
signature
issuer
audience
expiration
required claims
```

Then resolves:

```text
OIDC subject → internal user_id
```

Client-provided user IDs never establish identity.

---

# 4. Common Headers

Recommended request headers:

```http
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/json
X-Request-ID: <optional-client-request-id>
Idempotency-Key: <required-for-selected-commands>
```

The server generates a request ID if the client does not provide one.

Request IDs must appear in logs and error responses.

---

# 5. Common Response Envelope

Single resources:

```json
{
  "data": {}
}
```

Collections:

```json
{
  "data": [],
  "pagination": {
    "next_cursor": null,
    "has_more": false
  }
}
```

The envelope is intentionally thin. Domain resources remain explicit.

---

# 6. Error Envelope

All API errors use:

```json
{
  "error": {
    "code": "RIDE_NOT_CANCELLABLE",
    "message": "The ride cannot be cancelled in its current state.",
    "request_id": "req_123",
    "fields": {}
  }
}
```

`code` is machine-readable.

`message` is informational and must not be parsed by clients.

`fields` is present for field-level validation errors where useful.

---

# 7. HTTP Error Mapping

```text
400 Bad Request
→ malformed request / invalid syntax

401 Unauthorized
→ missing or invalid authentication

403 Forbidden
→ authenticated but not authorized

404 Not Found
→ resource does not exist or intentionally hidden

409 Conflict
→ valid request conflicts with current state/concurrency

422 Unprocessable Entity
→ structurally valid request fails validation/domain input rules

429 Too Many Requests
→ rate limit exceeded

500 Internal Server Error
→ unexpected server failure

502 Bad Gateway
→ required upstream dependency failed through the gateway boundary

503 Service Unavailable
→ service/dependency temporarily unavailable
```

Exact mapping should remain consistent once implementation begins.

---

# 8. API Error Codes

Initial stable codes:

```text
INVALID_REQUEST
VALIDATION_FAILED
AUTHENTICATION_REQUIRED
FORBIDDEN
RESOURCE_NOT_FOUND
CONFLICT
RATE_LIMITED
DEPENDENCY_UNAVAILABLE
INTERNAL_ERROR

RIDE_NOT_CANCELLABLE
RIDE_INVALID_STATE
BIDDING_CLOSED
BID_INVALID
BID_NOT_SELECTABLE
RESERVATION_EXPIRED
RESERVATION_INVALID_STATE
ASSIGNMENT_INVALID_STATE
DRIVER_NOT_ELIGIBLE
DRIVER_NOT_AVAILABLE
PAYMENT_INVALID_STATE
PAYMENT_FAILED
SETTLEMENT_INVALID_STATE
PAYOUT_INVALID_STATE
IDEMPOTENCY_KEY_REUSED
```

The list can grow, but existing codes should not casually change meaning.

---

# 9. Resource Identifier Format

Public IDs should be opaque.

Examples:

```text
usr_...
ride_...
driver_...
vehicle_...
bid_...
reservation_...
assignment_...
payment_...
settlement_...
payout_...
```

The exact encoding is an implementation decision.

Do not expose sequential PostgreSQL IDs as public identifiers by default.

---

# 10. Ride Commands

## Create Ride

```http
POST /api/v1/rides
```

Request:

```json
{
  "pickup": {
    "latitude": 31.5204,
    "longitude": 74.3587
  },
  "dropoff": {
    "latitude": 31.5497,
    "longitude": 74.3436
  }
}
```

Authorization:

```text
authenticated rider
```

Result:

```text
201 Created
```

Creates the initial `requested` ride state.

The operation should support idempotency because clients may retry after network
failures.

---

# 11. Get Ride

```http
GET /api/v1/rides/{ride_id}
```

Authorization:

```text
ride owner
relevant assigned/associated driver
explicitly authorized operational actor
```

Returns current ride state and only the data the caller is entitled to see.

---

# 12. List My Rides

```http
GET /api/v1/rides?status=&limit=&cursor=
```

The server derives the caller from authentication.

The client does not submit `rider_id` to define ownership.

Ordering must be deterministic, normally by:

```text
created_at DESC, id DESC
```

---

# 13. Start Bidding

```http
POST /api/v1/rides/{ride_id}/start-bidding
```

Authorization:

```text
ride owner / authorized system workflow
```

Transition:

```text
requested → bidding
```

The endpoint is a command and does not accept a target status.

---

# 14. Cancel Ride

```http
POST /api/v1/rides/{ride_id}/cancel
```

Request:

```json
{
  "reason": "changed_plans"
}
```

Authorization:

```text
actor authorized by cancellation policy
```

Possible result:

```text
200 OK
```

Transition:

```text
active state → cancelled
```

The use case also records cancellation history, releases applicable operational
state, and emits the appropriate event/outbox record.

---

# 15. Get Ride History

```http
GET /api/v1/rides/{ride_id}/events?limit=&cursor=
```

Returns authorized historical ride events.

Sensitive internal metadata should not automatically be exposed.

---

# 16. Submit Bid

```http
POST /api/v1/rides/{ride_id}/bids
```

Request:

```json
{
  "amount": {
    "minor": 1250,
    "currency": "PKR"
  }
}
```

Authorization:

```text
authenticated driver
```

Domain requirements:

```text
ride is bidding
caller has a driver profile
driver is eligible
driver is operationally available
amount is valid
driver has no conflicting active bid
```

Result:

```text
201 Created
```

Idempotency should be supported.

---

# 17. List Ride Bids

```http
GET /api/v1/rides/{ride_id}/bids?limit=&cursor=
```

Authorization:

```text
ride owner
authorized driver for their own bid
operational actor with explicit permission
```

A rider should not receive driver-private operational information that is not part
of the client contract.

---

# 18. Withdraw Bid

```http
POST /api/v1/bids/{bid_id}/withdraw
```

Authorization:

```text
owning driver
```

Valid transitions are defined by the bid state machine.

---

# 19. Select Bid

```http
POST /api/v1/bids/{bid_id}/select
```

Authorization:

```text
ride owner
```

The server verifies that the bid belongs to the ride and is selectable.

The transaction coordinates:

```text
bid selection
ride state
reservation creation
outbox events
```

Concurrent selection must produce one authoritative winner.

---

# 20. Get Bid

```http
GET /api/v1/bids/{bid_id}
```

Authorization is based on the bid's ride/driver relationship.

Do not allow arbitrary authenticated users to enumerate bids.

---

# 21. Reservation Endpoints

Get reservation:

```http
GET /api/v1/reservations/{reservation_id}
```

Confirm reservation where the workflow requires explicit driver confirmation:

```http
POST /api/v1/reservations/{reservation_id}/confirm
```

Cancel reservation where permitted:

```http
POST /api/v1/reservations/{reservation_id}/cancel
```

The API must reject expired reservations rather than treating them as active.

---

# 22. Assignment Endpoints

Get assignment:

```http
GET /api/v1/assignments/{assignment_id}
```

Driver arrival:

```http
POST /api/v1/assignments/{assignment_id}/arrive
```

Start trip:

```http
POST /api/v1/assignments/{assignment_id}/start
```

Complete trip:

```http
POST /api/v1/assignments/{assignment_id}/complete
```

Cancel/release where allowed:

```http
POST /api/v1/assignments/{assignment_id}/cancel
POST /api/v1/assignments/{assignment_id}/release
```

The server validates the assignment, parent ride, driver identity, and state.

---

# 23. Driver Profile

Get current driver profile:

```http
GET /api/v1/me/driver
```

Create/activate driver profile where product onboarding permits:

```http
POST /api/v1/me/driver
```

The backend associates the profile with the authenticated user.

The client never supplies another user's `user_id` as the ownership mechanism.

---

# 24. Driver Eligibility

Get current eligibility decision:

```http
GET /api/v1/me/driver/eligibility
```

Response concept:

```json
{
  "data": {
    "eligible": true,
    "reasons": []
  }
}
```

When ineligible, reasons should be actionable but must not expose sensitive internal
security information.

Eligibility is derived from durable account, verification, document, vehicle, and
policy state.

---

# 25. Driver Availability

Set operational availability:

```http
POST /api/v1/me/driver/availability
```

Request:

```json
{
  "available": true
}
```

The server must reject availability if the driver is not eligible to operate.

Availability is operational state, not durable driver eligibility.

---

# 26. Driver Location

Update current location:

```http
POST /api/v1/me/driver/location
```

Request:

```json
{
  "latitude": 31.5204,
  "longitude": 74.3587,
  "occurred_at": "2026-08-18T10:30:00Z"
}
```

The server validates coordinate ranges and freshness.

High-frequency location state should primarily use Redis rather than PostgreSQL.

The endpoint must have dedicated rate/abuse limits.

---

# 27. Vehicle Endpoints

List driver's vehicles:

```http
GET /api/v1/me/driver/vehicles
```

Create vehicle:

```http
POST /api/v1/me/driver/vehicles
```

Update vehicle where allowed:

```http
PATCH /api/v1/me/driver/vehicles/{vehicle_id}
```

The update contract must whitelist editable fields. It must never support arbitrary
status/eligibility mutation by the client.

---

# 28. Payment Endpoints

Get payment:

```http
GET /api/v1/payments/{payment_id}
```

Payment commands are intentionally narrow.

Capture where the product workflow requires the client to initiate it:

```http
POST /api/v1/payments/{payment_id}/capture
```

Refund where the actor and business policy permit:

```http
POST /api/v1/payments/{payment_id}/refund
```

Financial commands require idempotency.

---

# 29. Settlement Endpoints

Settlement is primarily a backend workflow.

Operationally authorized users may query it:

```http
GET /api/v1/settlements/{settlement_id}
```

Settlement creation/finalization should normally be driven by application workers or
explicitly authorized financial workflows rather than rider/driver endpoints.

---

# 30. Payout Endpoints

Driver payout history:

```http
GET /api/v1/me/driver/payouts?limit=&cursor=
```

Request payout where supported:

```http
POST /api/v1/me/driver/payouts
```

Request:

```json
{
  "amount": {
    "minor": 5000,
    "currency": "PKR"
  }
}
```

The server verifies available settled balance and payout eligibility.

Payout creation is idempotent.

---

# 31. Realtime WebSocket

Endpoint:

```text
GET /api/v1/ws
```

Authentication occurs during the WebSocket handshake using the same OIDC identity
model as HTTP.

After authentication, the server authorizes subscriptions.

---

# 32. WebSocket Subscription

Conceptual client message:

```json
{
  "type": "subscribe",
  "channel": "ride",
  "resource_id": "ride_123"
}
```

The server checks authorization before establishing the subscription.

The client cannot subscribe to arbitrary resource channels merely by guessing IDs.

---

# 33. WebSocket Events

Conceptual event envelope:

```json
{
  "type": "ride.updated",
  "event_id": "evt_123",
  "occurred_at": "2026-08-18T10:30:00Z",
  "data": {}
}
```

Representative client-facing events:

```text
ride.updated
ride.cancelled
bid.created
bid.updated
reservation.updated
assignment.updated
payment.updated
```

Internal domain events are mapped explicitly into client-facing events.

---

# 34. WebSocket Reconnection

WebSocket delivery is not assumed to be perfectly reliable.

After reconnect, the client must reconcile authoritative state through REST queries.

Conceptually:

```text
WebSocket disconnect
      ↓
reconnect
      ↓
GET current ride state
      ↓
resume realtime subscription
```

Do not make correctness depend on receiving every WebSocket message.

---

# 35. Pagination Contract

Collection endpoints use:

```text
limit
cursor
```

Example:

```http
GET /api/v1/rides?limit=20&cursor=abc
```

Response:

```json
{
  "data": [],
  "pagination": {
    "next_cursor": "def",
    "has_more": true
  }
}
```

Limits must have server-defined maximums.

---

# 36. Sorting and Filtering

Every collection defines supported filters and deterministic ordering.

Do not expose arbitrary SQL-like query parameters.

Example:

```http
GET /api/v1/rides?status=completed&limit=20
```

Unsupported filters return `400 Bad Request` rather than being silently ignored.

---

# 37. Idempotency Contract

Use:

```http
Idempotency-Key: <key>
```

for commands where retrying can duplicate business effects.

The server stores enough information to distinguish:

```text
same key + same logical request
```

from:

```text
same key + different request
```

The latter returns:

```text
409 Conflict
IDEMPOTENCY_KEY_REUSED
```

---

# 38. Commands That Require Idempotency

At minimum:

```text
ride creation
bid submission
ride cancellation where duplicate effects are possible
payment capture
payment refund
payout creation
external webhook processing
```

Read operations do not require idempotency keys.

---

# 39. Validation

Transport validation covers:

```text
required fields
types
formats
field sizes
coordinate ranges
basic numeric ranges
enum syntax
```

Domain validation covers:

```text
state transitions
ownership
eligibility
availability
cancellation policy
financial state
concurrency
```

---

# 40. Authorization Matrix

Initial high-level matrix:

| Operation | Rider | Driver | Admin/Ops |
|---|---:|---:|---:|
| Create ride | Yes | No | Policy-dependent |
| View own ride | Yes | Assigned/relevant only | Yes |
| Cancel own ride | Yes, policy | Assigned/relevant, policy | Yes |
| Submit bid | No | Yes, eligible | No/ops workflow |
| View ride bids | Yes for own ride | Own bid | Yes |
| Select bid | Yes for own ride | No | Policy-dependent |
| Confirm reservation | No | Assigned driver | Policy-dependent |
| Start trip | No | Assigned driver | Policy-dependent |
| Complete trip | No | Assigned driver | Policy-dependent |
| View own payment | Yes | Relevant only | Finance |
| Request payout | No | Eligible driver | Finance |
| View settlement | No | Relevant only | Finance |

This matrix is a baseline. The final authorization policy must be implemented in
application use cases, not inferred from Flutter screens.

---

# 41. Resource Privacy

A protected resource may return `404` instead of `403` when revealing its existence
would leak sensitive information.

This is particularly relevant for:

```text
other users
private driver records
unrelated rides
payment records
payout records
```

The behavior must be consistent within each resource family.

---

# 42. Sensitive Data

API responses should minimize exposure of:

```text
identity documents
payment credentials
private driver information
unrelated locations
internal provider metadata
security/audit internals
```

Pickup/dropoff information is exposed only to actors who need it for the ride.

---

# 43. Location API Rules

Location endpoints require:

```text
authenticated driver identity
coordinate validation
freshness validation
rate limiting
authorization
```

The server must not accept an arbitrary `driver_id` in the request as the authority
for whose location is being updated.

---

# 44. Payment API Rules

Payment APIs must:

```text
never accept raw payment credentials unnecessarily
validate amount/currency
validate payment state
use provider idempotency where available
record provider identifiers
verify webhooks
```

A client cannot set `payment.status = captured`.

---

# 45. Webhook Endpoint

External payment/provider webhooks use a dedicated endpoint such as:

```http
POST /api/v1/webhooks/{provider}
```

Authentication is provider-specific signature verification rather than rider/driver
OIDC authentication.

The handler must:

```text
verify signature
validate event schema
check provider event ID
apply idempotent state transition
write outbox event if needed
return provider-compatible response
```

---

# 46. API Timeouts

Every request path must have bounded execution time.

External dependency calls must use shorter child timeouts than the overall request
where appropriate.

Do not hold mobile connections open indefinitely waiting for third-party services.

---

# 47. Rate Limits

Initial limits should be defined per operation after load/abuse testing.

The contract requires the existence of limits for high-risk/high-volume endpoints,
not one arbitrary universal number.

When exceeded:

```text
429 Too Many Requests
```

with a machine-readable error code.

---

# 48. Request Size Limits

The API rejects oversized bodies and fields before expensive processing.

Each endpoint defines its own maximums where necessary.

---

# 49. API Compatibility

Breaking changes include:

```text
removing fields
changing field types
changing enum semantics
making optional data required
changing command meaning
```

Breaking changes require a new API version or explicit compatibility mechanism.

Additive fields should be preferred where practical.

---

# 50. Client Compatibility

Flutter clients should:

```text
ignore unknown response fields
handle unknown enum values safely
handle new event types safely
reconcile state through REST after reconnect
```

The server must not assume every installed mobile client updates immediately.

---

# 51. API Testing

Contract tests must cover:

```text
valid authentication
invalid authentication
wrong audience/issuer
resource authorization
cross-user access
cross-driver access
validation errors
state-transition conflicts
idempotency
pagination
rate limits
WebSocket authorization
WebSocket reconnect/reconciliation
payment webhook verification
```

Every state-machine transition should have at least one API-level test where a public
command exposes it.

---

# 52. OpenAPI Mapping

The next implementation artifact should be an OpenAPI specification containing:

```text
schemas
security schemes
paths
parameters
request bodies
responses
error models
pagination
WebSocket documentation where supported by the chosen tooling
```

The OpenAPI document should be generated/maintained from this contract rather than
used as a substitute for domain design.

---

# 53. Initial Endpoint Map

```text
Authentication
  external OIDC

Rides
  POST   /api/v1/rides
  GET    /api/v1/rides
  GET    /api/v1/rides/{ride_id}
  POST   /api/v1/rides/{ride_id}/start-bidding
  POST   /api/v1/rides/{ride_id}/cancel
  GET    /api/v1/rides/{ride_id}/events

Bids
  POST   /api/v1/rides/{ride_id}/bids
  GET    /api/v1/rides/{ride_id}/bids
  GET    /api/v1/bids/{bid_id}
  POST   /api/v1/bids/{bid_id}/withdraw
  POST   /api/v1/bids/{bid_id}/select

Reservations
  GET    /api/v1/reservations/{reservation_id}
  POST   /api/v1/reservations/{reservation_id}/confirm
  POST   /api/v1/reservations/{reservation_id}/cancel

Assignments
  GET    /api/v1/assignments/{assignment_id}
  POST   /api/v1/assignments/{assignment_id}/arrive
  POST   /api/v1/assignments/{assignment_id}/start
  POST   /api/v1/assignments/{assignment_id}/complete
  POST   /api/v1/assignments/{assignment_id}/cancel
  POST   /api/v1/assignments/{assignment_id}/release

Driver
  GET    /api/v1/me/driver
  POST   /api/v1/me/driver
  GET    /api/v1/me/driver/eligibility
  POST   /api/v1/me/driver/availability
  POST   /api/v1/me/driver/location

Vehicles
  GET    /api/v1/me/driver/vehicles
  POST   /api/v1/me/driver/vehicles
  PATCH  /api/v1/me/driver/vehicles/{vehicle_id}

Payments
  GET    /api/v1/payments/{payment_id}
  POST   /api/v1/payments/{payment_id}/capture
  POST   /api/v1/payments/{payment_id}/refund

Settlements
  GET    /api/v1/settlements/{settlement_id}

Payouts
  GET    /api/v1/me/driver/payouts
  POST   /api/v1/me/driver/payouts

Realtime
  GET    /api/v1/ws

Webhooks
  POST   /api/v1/webhooks/{provider}
```

This is the baseline endpoint surface. Endpoints should not be implemented until
its authorization and domain use case are defined.

---

# 54. Deliberately Excluded

Do not add generic endpoints such as:

```text
PATCH /rides/{id}
PATCH /bids/{id}
PATCH /payments/{id}
```

when they permit clients to mutate state-machine fields directly.

Also avoid:

```text
GET /users/{id}/everything
GET /drivers/all-locations
POST /internal/run-any-command
```

These create security and coupling problems.

---

# 55. Design Principles

1. `/api/v1` is the initial public version.
2. REST handles commands and queries; WebSocket handles realtime delivery.
3. Authentication uses external OIDC.
4. Authorization is evaluated server-side.
5. Clients never set arbitrary lifecycle statuses.
6. Public API resources are domain representations, not database rows.
7. Stable error codes are part of the contract.
8. Request IDs support operational tracing.
9. Cursor pagination is preferred for mutable collections.
10. Deterministic ordering is mandatory for collections.
11. Idempotency protects effectful/retriable commands.
12. Payment/webhook operations require stronger idempotency guarantees.
13. WebSocket subscriptions require resource authorization.
14. REST is the reconciliation authority after realtime disconnects.
15. Sensitive data is minimized in responses.
16. Location updates have dedicated validation and abuse controls.
17. External dependency calls use bounded timeouts.
18. API handlers do not own business transaction boundaries.
19. OpenAPI will become the machine-readable HTTP contract.
20. The API surface remains intentionally narrow; new endpoints require a concrete
    business/use-case reason.
