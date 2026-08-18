# Ride API

## 1. Purpose

This document defines the initial REST API contract for ride creation,
retrieval, cancellation, bid visibility, and bid selection.

The API is intentionally separated from the internal dispatch implementation.

The rider interacts with the ride API while discovery and bidding operate
asynchronously behind it.

---

# 2. API Base Path

All initial endpoints use:

```text
/api/v1
```

Examples:

```text
POST /api/v1/rides
GET  /api/v1/rides/{ride_id}
```

Common authentication, authorization, idempotency, error, pagination, and
concurrency rules are defined in:

```text
docs/api/api-conventions.md
```

---

# 3. Ride API Surface

The initial ride API consists of:

```text
POST /api/v1/rides
GET  /api/v1/rides/{ride_id}
POST /api/v1/rides/{ride_id}/cancel
GET  /api/v1/rides/{ride_id}/bids
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

The API does not expose internal discovery orchestration.

---

# 4. Ride Creation

```http
POST /api/v1/rides
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

Request example:

```json
{
  "pickup_location": {
    "latitude": 31.5204,
    "longitude": 74.3587
  },
  "pickup_address": "Lahore",
  "destination_location": {
    "latitude": 31.5497,
    "longitude": 74.3436
  },
  "destination_address": "Lahore",
  "service_type": "standard",
  "passenger_count": 1,
  "currency": "PKR"
}
```

The exact request fields will evolve with the ride-request domain.

---

# 5. Ride Creation Semantics

Ride creation performs:

```text
authenticate caller
      ↓
authorize rider
      ↓
validate request
      ↓
create ride
      ↓
set initial lifecycle state
      ↓
create outbox event
      ↓
commit transaction
```

Only after successful commit should discovery begin.

The API must not wait for:

```text
driver discovery
bidding
rider selection
assignment
```

before returning the ride-creation response.

---

# 6. Ride Creation Response

Successful creation returns:

```text
201 Created
```

Conceptually:

```json
{
  "data": {
    "id": "ride_123",
    "status": "REQUESTED",
    "revision": 1,
    "created_at": "2026-08-18T10:30:00Z"
  }
}
```

The actual representation will be finalized when the complete API resource
schema is defined.

---

# 7. Ride Creation Idempotency

Ride creation requires an idempotency key.

Example:

```text
POST /api/v1/rides
Idempotency-Key: 5f3b...
```

If the client retries the same logical request with the same key, the backend
must not create a second ride.

The idempotency key must be associated with the authenticated caller.

A different authenticated user must not be able to reuse another user's key to
retrieve or mutate their operation.

---

# 8. Ride Creation Validation

The backend validates at least:

```text
authenticated rider
pickup coordinates
destination coordinates
service type
passenger count
currency
request options
```

The ride request rules are defined in:

```text
docs/rides/ride-request.md
```

---

# 9. Ride Creation and Discovery

The API does not expose a synchronous driver-search operation.

The flow is:

```text
POST /rides
      ↓
ride committed
      ↓
outbox event
      ↓
discovery worker
      ↓
BIDDING
```

The rider observes discovery/bidding progress through the ride resource and
real-time events.

---

# 10. Get Ride

```http
GET /api/v1/rides/{ride_id}
Authorization: Bearer <token>
```

Returns the authoritative current ride representation.

The caller must be authorized to access the ride.

---

# 11. Get Ride Response

Conceptually:

```json
{
  "data": {
    "id": "ride_123",
    "status": "BIDDING",
    "revision": 3,
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
    "created_at": "2026-08-18T10:30:00Z",
    "updated_at": "2026-08-18T10:31:12Z"
  }
}
```

The exact response fields depend on the finalized ride resource model.

---

# 12. Ride State Authority

The `status` returned by `GET /rides/{ride_id}` is authoritative.

The Flutter application must not assume that its local state is correct when
it differs from the server response.

---

# 13. Ride Retrieval After WebSocket Disconnect

If the client loses its WebSocket connection:

```text
WebSocket disconnect
      ↓
reconnect
      ↓
GET /api/v1/rides/{ride_id}
      ↓
reconcile current state
```

This makes REST the recovery mechanism for missed real-time events.

---

# 14. Ride Authorization

The API must authorize ride access by the caller's role and relationship.

At minimum:

```text
Rider
  → own rides

Driver
  → rides for which the driver has an authorized operational relationship

Operations/admin
  → according to explicit administrative permissions
```

The exact driver visibility rules will be finalized with the driver APIs.

---

# 15. Ride Not Found

If the ride cannot be returned, the API may return:

```text
404 Not Found
```

For private rides, the implementation may intentionally avoid revealing
whether another user's ride exists.

---

# 16. Cancel Ride

```http
POST /api/v1/rides/{ride_id}/cancel
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

The request body is initially empty unless a later cancellation policy needs
structured information.

---

# 17. Cancellation Semantics

Cancellation is a business command.

The backend decides whether cancellation is allowed in the current lifecycle
state.

It must not be implemented as:

```http
PATCH /rides/{ride_id}
```

with an arbitrary status field.

---

# 18. Cancellation Flow

Conceptually:

```text
cancel request
      ↓
authenticate
      ↓
authorize
      ↓
lock/revalidate ride state
      ↓
validate cancellation policy
      ↓
transition ride
      ↓
release applicable dispatch resources
      ↓
create outbox event
      ↓
commit
```

The exact cancellation fee/compensation policy is a separate domain decision.

---

# 19. Cancellation State Restrictions

Cancellation is state-dependent.

Potentially cancellable states include:

```text
REQUESTED
DISCOVERY
BIDDING
DRIVER_CONFIRMATION_REQUIRED
DRIVER_CONFIRMED
DRIVER_ARRIVED
TRIP_STARTED
```

Whether cancellation is permitted in each state, and its financial
consequences, will be defined by cancellation policy.

The API must not assume all states are always cancellable.

---

# 20. Cancellation Race

A cancellation can race with another lifecycle command.

Example:

```text
rider cancels
      +
driver confirms
```

The backend must resolve the race transactionally against authoritative state.

Exactly one valid transition wins according to the defined transition rules.

The losing command receives an appropriate conflict/state error.

---

# 21. Cancellation Response

If cancellation succeeds and a representation is useful:

```text
200 OK
```

may return the authoritative ride state.

If no response body is needed:

```text
204 No Content
```

may be used.

The exact choice should be standardized when the complete API response
conventions are finalized.

---

# 22. Get Ride Bids

```http
GET /api/v1/rides/{ride_id}/bids
Authorization: Bearer <token>
```

This endpoint is intended primarily for the rider's bidding view.

The caller must be authorized to view bids for the ride.

---

# 23. Bid Visibility

The rider should receive only information needed to evaluate a bid.

Potential fields include:

```text
bid_id
driver display information
driver rating where applicable
vehicle information
bid amount
currency
estimated arrival information
bid status
created_at
expires_at
```

The API must not expose internal fields such as:

```text
eligibility evaluation details
internal scoring
fraud signals
Redis keys
internal candidate ranking
private driver data
```

---

# 24. Bid Pagination

The bids endpoint should use the common cursor pagination convention.

Example:

```text
GET /api/v1/rides/{ride_id}/bids?limit=20&cursor=<cursor>
```

The server controls maximum page size.

---

# 25. Bid Ordering

The API should define a deterministic ordering.

The initial ordering may prioritize recent active bids while preserving stable
ordering for pagination.

The exact product ranking of bids is not an API convention and belongs to the
bidding/product design.

---

# 26. Bid Selection

```http
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
```

Only the authorized rider for the ride may select a bid.

---

# 27. Bid Selection Transaction

Bid selection is one of the most concurrency-sensitive ride operations.

The backend should conceptually perform:

```text
BEGIN

authenticate rider
lock/revalidate ride
validate ride = BIDDING
validate bid belongs to ride
validate bid selectable
revalidate driver
revalidate active vehicle
revalidate driver availability
create reservation
close bidding
transition ride → DRIVER_CONFIRMATION_REQUIRED
create outbox event

COMMIT
```

The actual PostgreSQL locking strategy will be defined during database design.

---

# 28. Bid Selection and Stale Discovery

Discovery information can become stale.

For example:

```text
Redis:
Driver A available

later:
Driver A receives another reservation
```

The rider may still see Driver A's bid.

When the rider selects it, the backend must revalidate availability.

The API must never assume the bid remains assignable merely because it was
previously displayed.

---

# 29. Driver Unavailable During Selection

If the selected driver is no longer available:

```text
POST /rides/{id}/bids/{bid}/select
       ↓
409 Conflict
```

Example:

```json
{
  "error": {
    "code": "DRIVER_NOT_AVAILABLE",
    "message": "The selected driver is no longer available.",
    "details": {}
  }
}
```

The ride remains under the appropriate bidding/fallback lifecycle policy.

---

# 30. Bid Selection Result

Successful selection produces:

```text
BIDDING
   ↓
reservation created
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

The response should contain authoritative state sufficient for the Flutter
client to render the confirmation-pending state.

Conceptually:

```json
{
  "data": {
    "ride_id": "ride_123",
    "status": "DRIVER_CONFIRMATION_REQUIRED",
    "revision": 4,
    "reservation": {
      "id": "reservation_123",
      "expires_at": "2026-08-18T10:32:00Z"
    }
  }
}
```

The exact reservation representation belongs to the reservation API design.

---

# 31. Bid Selection Idempotency

Bid selection requires an idempotency key.

A retry after a lost response must not create:

```text
second reservation
second assignment
second lifecycle transition
```

If the original selection succeeded, a retry with the same idempotency key
should return the result associated with that operation where appropriate.

---

# 32. Invalid Ride State

Examples:

```text
select bid while REQUESTED
select bid while DRIVER_CONFIRMED
select bid after TRIP_COMPLETED
```

These are state conflicts and should normally return:

```text
409 Conflict
```

with a stable application error code.

---

# 33. Invalid Bid

Examples:

```text
bid belongs to another ride
bid expired
bid withdrawn
bid already selected
bid not visible to caller
```

The API should return an appropriate `404`, `409`, or `422` depending on the
specific semantic case and the common API error policy.

The distinction should remain consistent across the API.

---

# 34. Ride API and Discovery

The ride API does not expose discovery internals.

There is no initial endpoint such as:

```text
POST /rides/{id}/search-drivers
```

Instead:

```text
POST /rides
      ↓
ride committed
      ↓
outbox
      ↓
discovery
      ↓
bids
      ↓
WebSocket notifications
```

This keeps dispatch independent from the public ride command API.

---

# 35. Ride API and WebSocket

The Flutter application can subscribe to ride-related events through the
WebSocket connection.

Potential events include:

```text
ride.discovery_started
ride.bidding_started
ride.bid_received
ride.bid_updated
ride.bid_withdrawn
ride.bidding_closed
ride.driver_confirmation_required
ride.driver_confirmed
ride.driver_arrived
ride.trip_started
ride.trip_completed
ride.cancelled
ride.no_driver_found
```

The event list is illustrative and will be finalized in the event contract.

---

# 36. WebSocket Is Not the Command Path

Do not make rider actions depend on sending business commands through the
WebSocket.

For example:

```text
REST POST /rides/{id}/cancel
```

is preferred over:

```text
WebSocket: {"command":"cancel_ride"}
```

This keeps authorization, retries, idempotency, and observability consistent.

---

# 37. Event Ordering

Ride events should include enough information to detect stale or missing
updates.

At minimum, lifecycle-related events should carry a ride revision or equivalent
monotonic sequence.

Example:

```text
revision 4 → DRIVER_CONFIRMATION_REQUIRED
revision 5 → DRIVER_CONFIRMED
```

A client receiving revision 5 after revision 4 can advance safely.

---

# 38. Missed Events

If a client receives revision 8 after its local state is revision 5, it may
need to retrieve authoritative state rather than assuming it received every
intermediate event.

The API therefore guarantees state recovery through:

```text
GET /api/v1/rides/{ride_id}
```

---

# 39. Redis Failure

Ride creation should still use PostgreSQL for durable state.

If Redis is unavailable, the system must not fabricate successful discovery or
assignment results.

The API should return the ride state that was actually committed.

Discovery can fail/retry independently according to the dispatch design.

---

# 40. PostgreSQL Failure

If PostgreSQL cannot commit a state-changing operation:

```text
ride creation
cancellation
bid selection
```

must not return a false success.

The API should return an appropriate temporary failure such as:

```text
503 Service Unavailable
```

when the failure is a known dependency outage.

---

# 41. Authorization Matrix

Initial conceptual matrix:

| Operation | Rider | Driver | Admin |
|---|---:|---:|---:|
| Create ride | Yes | Policy-dependent | Yes if authorized |
| Get own ride | Yes | If operationally related | Yes if authorized |
| Cancel own ride | Yes | No | Yes if authorized |
| Get ride bids as rider | Yes | No | Yes if authorized |
| Select bid | Yes, own ride | No | Policy-dependent |

The final matrix will expand when driver and operations APIs are defined.

---

# 42. No Generic Ride Update Endpoint

Do not introduce:

```text
PATCH /api/v1/rides/{ride_id}
```

as a generic endpoint for lifecycle state.

If a future ride field genuinely needs direct modification, it should receive an
explicitly defined mutation contract with its own authorization and concurrency
rules.

---

# 43. API and Domain Boundaries

The API should map to domain commands rather than database operations.

For example:

```text
POST /rides/{id}/cancel
```

means:

```text
CancelRide
```

not:

```text
UPDATE rides SET status = 'CANCELLED'
```

The Go application owns the business operation.

---

# 44. Long-Running Dispatch

The API must return without waiting for the complete ride lifecycle.

For example:

```text
POST /rides
   ↓
201 Created
```

The rider may then receive:

```text
BIDDING
```

seconds later through REST/WebSocket.

The API must not impose an artificial synchronous timeout on driver discovery.

---

# 45. Security Boundaries

The API must protect:

```text
rider identity
pickup/destination data
driver identity
vehicle information
bid information
assignment information
```

Do not return more information merely because the caller is authenticated.

Authorization is resource- and operation-specific.

---

# 46. API Contract and Database Independence

The API contract must not expose PostgreSQL implementation details.

For example, do not expose:

```text
PostgreSQL row IDs with implied semantics
internal table names
lock versions as implementation details
Redis keys
```

A public revision field may exist because it has API semantics, but it is not a
database implementation leak.

---

# 47. Future Ride Endpoints

Potential future endpoints include:

```text
GET  /api/v1/rides
GET  /api/v1/rides/{ride_id}/events
POST /api/v1/rides/{ride_id}/stop
POST /api/v1/rides/{ride_id}/rating
```

These should be added only when their domain behavior is defined.

Do not reserve endpoint names for speculative features unnecessarily.

---

# 48. What We Should Not Build Yet

Do not build:

```text
synchronous driver search endpoint
client-controlled ride status
WebSocket business commands
payment operations in the ride API
rating operations in the ride API
route recalculation endpoints
full ride history API
administrative ride mutation APIs
OpenAPI for undefined endpoints
```

Those belong to later domain/API work.

---

# 49. Design Principles

1. Ride creation is synchronous only for validating and committing the request.
2. Discovery and bidding happen asynchronously after ride creation.
3. `GET /rides/{id}` is the authoritative ride-state recovery endpoint.
4. Cancellation is an explicit business command.
5. Bid selection is an explicit business command.
6. State-changing endpoints require server-side authorization.
7. Ride creation and bid selection require idempotency.
8. Bid selection must revalidate driver and vehicle availability.
9. Redis discovery results are never treated as assignment guarantees.
10. Reservation and lifecycle changes must be transactionally consistent.
11. WebSocket delivers events but does not own business state.
12. Ride events should carry a monotonic revision/sequence where applicable.
13. Clients recover from missed events through REST.
14. API responses expose deliberate domain contracts, not database rows.
15. The ride API must remain independent of internal dispatch orchestration.
16. Long-running discovery must never block the ride-creation HTTP request.
17. Business state changes belong to domain commands rather than generic PATCH operations.
18. Payment, rating, and other adjacent domains should remain outside the initial ride API.
19. The API should fail rather than claim success when a required durable transaction cannot commit.
20. New endpoints should be added only after their underlying domain behavior is defined.
