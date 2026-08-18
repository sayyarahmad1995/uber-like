# Trip Lifecycle

## 1. Purpose

This document defines the lifecycle after driver confirmation and assignment,
through driver movement, arrival, trip start, and trip completion.

The pre-trip flow is:

```text
Ride
  ↓
Bidding
  ↓
Bid selected
  ↓
Reservation
  ↓
Driver confirms
  ↓
Assignment
```

The operational trip flow is:

```text
DRIVER_CONFIRMED
       ↓
DRIVER_EN_ROUTE
       ↓
DRIVER_ARRIVED
       ↓
TRIP_STARTED
       ↓
TRIP_COMPLETED
```

---

# 2. Core Principles

1. Assignment is a durable relationship between a ride, driver, and vehicle.
2. Ride lifecycle state is authoritative on the server.
3. Clients send commands; they do not directly set lifecycle state.
4. Driver location is high-frequency operational data and should not be stored as a PostgreSQL write on every update.
5. Redis may hold current driver location and presence; PostgreSQL stores durable milestones.
6. Arrival, start, and completion are explicit business transitions.
7. GPS/geofence information can support validation but does not replace server-side lifecycle commands.
8. Payment and rating are separate domains.
9. Assignment invariants must prevent a driver or vehicle from serving conflicting active rides.
10. WebSocket is for real-time notifications; REST remains authoritative for recovery.

---

# 3. Assignment Model

After reservation confirmation:

```text
Reservation
    ↓
CONFIRMED
    ↓
Assignment created
```

The assignment represents the active operational relationship:

```text
Ride ←→ Driver ←→ Vehicle
```

The assignment should preserve the driver and vehicle selected through the bid
and reservation process.

---

# 4. Assignment Invariants

The system must enforce, where applicable:

```text
one active assignment per ride
one conflicting active assignment per driver
one conflicting active assignment per vehicle
assignment belongs to one ride
assignment belongs to one driver
assignment belongs to one vehicle
```

These are durable business invariants and should not depend solely on Redis.

---

# 5. DRIVER_CONFIRMED

`DRIVER_CONFIRMED` means:

```text
driver confirmed reservation
assignment exists
ride has a selected driver/vehicle
trip has not started
```

The driver is now responsible for proceeding toward the pickup location.

---

# 6. DRIVER_EN_ROUTE

The driver transitions to:

```text
DRIVER_CONFIRMED
      ↓
DRIVER_EN_ROUTE
```

when the driver begins the operational journey to pickup.

The exact trigger can be an explicit driver command, with location data used as
supporting validation.

---

# 7. Driver Location

The driver application sends location updates through the location path.

Conceptually:

```text
Flutter Driver
      ↓
API Gateway
      ↓
Location handling
      ├── Redis → current location/presence
      └── PostgreSQL → durable milestones where needed
```

The system should not write every GPS update to PostgreSQL in the initial
architecture.

---

# 8. Location Freshness

A driver location should have freshness metadata such as:

```text
latitude
longitude
observed_at
received_at
```

The server should distinguish between:

```text
last known location
fresh location
stale location
```

A stale location must not be presented as live positioning.

---

# 9. Location Authority

The driver device is the source of the reported GPS observation, but the server
is authoritative for whether that observation is accepted and how it is exposed
to other users.

Client timestamps must not be trusted for security-sensitive ordering.

---

# 10. DRIVER_ARRIVED

The driver can explicitly signal arrival:

```http
POST /api/v1/rides/{ride_id}/arrived
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The backend validates:

```text
authenticated driver
assignment ownership
ride state
trip not already started
pickup conditions
```

The exact arrival-distance/geofence policy is a product/dispatch decision.

---

# 11. Arrival and Geofencing

GPS/geofence data may be used as supporting evidence that the driver is near
the pickup location.

However, do not make the client authoritative by allowing it to send:

```json
{
  "status": "DRIVER_ARRIVED"
}
```

The command is authoritative; location is validation/supporting data.

---

# 12. Arrival Tolerance

The acceptable distance from pickup should be configurable.

Do not hardcode an assumption such as:

```text
exactly 50 meters forever
```

Urban density, GPS accuracy, and pickup configuration may require tuning.

---

# 13. TRIP_STARTED

The driver starts the trip through an explicit command:

```http
POST /api/v1/rides/{ride_id}/start
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The initial expected transition is:

```text
DRIVER_ARRIVED
      ↓
TRIP_STARTED
```

The backend must validate the assignment and current ride state.

---

# 14. Trip Start Validation

The backend verifies at least:

```text
authenticated driver owns assignment
assignment is active
ride is DRIVER_ARRIVED
ride is not cancelled
assignment driver/vehicle remain valid
```

Additional product-specific requirements may be added later.

---

# 15. Starting From the Wrong State

The backend must reject commands such as:

```text
start while BIDDING
start while DRIVER_EN_ROUTE
start after TRIP_COMPLETED
start after cancellation
```

with an appropriate state/conflict error.

The client cannot skip lifecycle stages merely by sending a different command.

---

# 16. TRIP_STARTED

`TRIP_STARTED` means the passenger journey has begun.

Durable information should include at least:

```text
trip start timestamp
assignment reference
ride revision/state
```

Additional route/distance information can be recorded as it becomes available.

---

# 17. TRIP_COMPLETED

The driver completes the ride through:

```http
POST /api/v1/rides/{ride_id}/complete
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

Expected transition:

```text
TRIP_STARTED
      ↓
TRIP_COMPLETED
```

The backend must reject completion if the trip never started.

---

# 18. Trip Completion Validation

The backend verifies:

```text
authenticated driver owns assignment
assignment is active
ride = TRIP_STARTED
ride is not cancelled
trip has not already completed
```

The exact completion evidence policy can be defined later.

---

# 19. Completion Data

Trip completion should preserve durable facts such as:

```text
completed_at
final assignment reference
final ride state
```

Potential future fields include:

```text
actual distance
actual duration
final route information
```

These should be introduced deliberately rather than making the lifecycle
endpoint a dumping ground for telemetry.

---

# 20. No Direct Status Mutation

Do not expose:

```http
PATCH /api/v1/rides/{ride_id}
```

for lifecycle state mutation.

Use explicit commands:

```text
arrived
start
complete
```

The Go backend owns state transitions.

---

# 21. Cancellation During Trip Lifecycle

Cancellation remains a separate business command:

```text
POST /api/v1/rides/{ride_id}/cancel
```

Whether cancellation is permitted depends on current lifecycle state and
policy.

Examples of potentially cancellable states:

```text
DRIVER_CONFIRMED
DRIVER_EN_ROUTE
DRIVER_ARRIVED
TRIP_STARTED
```

The actual policy, including fees, is a separate cancellation/pricing decision.

---

# 22. Cancellation Must Not Corrupt Assignment

If a rider cancellation wins while a driver command is executing, the system
must not leave an active assignment attached to a cancelled ride unless the
business model explicitly permits it.

The race must be resolved transactionally.

---

# 23. Driver Confirmation Race

A cancellation can race with reservation confirmation before assignment is
created.

The reservation domain owns that boundary.

Once assignment exists, trip lifecycle rules govern the subsequent state.

---

# 24. Arrival Race

Two arrival requests may be received because of mobile retries or multiple
active sessions.

The command should be idempotent and state-aware.

Repeated successful arrival requests must not create duplicate milestones or
advance the ride twice.

---

# 25. Start Race

Two start requests may race.

Only one transition may occur:

```text
DRIVER_ARRIVED
      ↓
TRIP_STARTED
```

A retry should return the authoritative result or the standardized idempotent
outcome rather than starting a second trip.

---

# 26. Completion Race

Two completion requests may race.

Only one transition may occur:

```text
TRIP_STARTED
      ↓
TRIP_COMPLETED
```

Repeated completion attempts must not create duplicate completion records or
trigger duplicate downstream settlement operations.

---

# 27. Driver and Vehicle Availability After Assignment

Once a driver is assigned, the driver should no longer be considered available
for conflicting discovery/reservation operations.

Conceptually:

```text
AVAILABLE
   ↓
RESERVED
   ↓
ASSIGNED
   ↓
TRIP_STARTED
   ↓
TRIP_COMPLETED
   ↓
AVAILABLE
```

The exact driver-presence state machine is defined by the driver domain.

---

# 28. Vehicle Availability

The active vehicle follows the same operational commitment.

A vehicle associated with an active assignment must not be offered for another
conflicting ride.

---

# 29. Driver Location After Assignment

Location updates continue while the driver is operationally active.

The location service may be used to:

```text
show driver to rider
calculate ETA
support arrival validation
support operational monitoring
```

Location data should not itself transition the ride lifecycle without an
explicit domain rule.

---

# 30. Location During Trip

After `TRIP_STARTED`, location can support:

```text
live trip map
ETA updates
route monitoring
future safety/operations features
```

Do not make every GPS point part of the durable ride state.

---

# 31. WebSocket Events

Potential lifecycle events include:

```text
ride.driver_confirmed
ride.driver_en_route
ride.driver_arrived
ride.trip_started
ride.trip_completed
ride.cancelled
```

These are notifications of durable state transitions.

---

# 32. Event Revision

Lifecycle events should carry a ride revision or equivalent monotonic sequence.

Example:

```text
revision 10 → DRIVER_CONFIRMED
revision 11 → DRIVER_EN_ROUTE
revision 12 → DRIVER_ARRIVED
revision 13 → TRIP_STARTED
```

Clients can use the revision to detect stale state/events.

---

# 33. Missed Events

If a client misses events:

```text
WebSocket reconnect
      ↓
GET /api/v1/rides/{ride_id}
      ↓
reconcile authoritative state
```

The system must not require perfect event delivery for correctness.

---

# 34. Outbox Events

Durable lifecycle transitions should create outbox events in the same
transaction.

Examples:

```text
ride.driver_confirmed
ride.driver_en_route
ride.driver_arrived
ride.trip_started
ride.trip_completed
```

The event publication happens after commit and can be retried.

---

# 35. PostgreSQL Responsibilities

PostgreSQL is authoritative for durable trip state such as:

```text
ride lifecycle state
assignment
trip milestones
start timestamp
completion timestamp
revision
```

The exact schema is a later database design task.

---

# 36. Redis Responsibilities

Redis may support:

```text
current driver location
presence
fast operational lookup
real-time fan-out support
```

Redis should not be authoritative for:

```text
trip completion
assignment ownership
trip start
lifecycle correctness
```

---

# 37. Location Persistence Strategy

The initial recommendation is:

```text
high-frequency GPS
      ↓
Redis/current-location path

important trip milestones
      ↓
PostgreSQL
```

If historical GPS tracks become a product requirement, choose a dedicated
storage strategy later rather than filling PostgreSQL with unbounded point
writes.

---

# 38. External Maps Integration

Google Maps may provide:

```text
geocoding
routing
ETA/distance calculations
map rendering support
```

The ride lifecycle should not depend on a Google Maps response for every state
transition.

For example, a temporary Maps outage should not make an already-started trip
impossible to complete.

---

# 39. Maps Failure

If a Maps request fails:

```text
trip state remains authoritative
```

The system can degrade ETA/routing features while preserving lifecycle
correctness.

---

# 40. Driver En-Route Command

The initial API may expose:

```http
POST /api/v1/rides/{ride_id}/en-route
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The exact naming can be adjusted during the final API contract review.

Expected transition:

```text
DRIVER_CONFIRMED
      ↓
DRIVER_EN_ROUTE
```

---

# 41. Arrival Command

```http
POST /api/v1/rides/{ride_id}/arrived
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

Expected transition:

```text
DRIVER_EN_ROUTE
      ↓
DRIVER_ARRIVED
```

The command must be authorized against the assignment.

---

# 42. Start Command

```http
POST /api/v1/rides/{ride_id}/start
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

Expected transition:

```text
DRIVER_ARRIVED
      ↓
TRIP_STARTED
```

---

# 43. Complete Command

```http
POST /api/v1/rides/{ride_id}/complete
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

Expected transition:

```text
TRIP_STARTED
      ↓
TRIP_COMPLETED
```

---

# 44. Idempotency

All driver lifecycle commands should be designed for mobile retries.

Candidates:

```text
en-route
arrived
start
complete
```

A lost HTTP response must not cause duplicate transitions or duplicate
side-effects.

---

# 45. Authorization Matrix

Initial conceptual matrix:

| Operation | Rider | Driver | Admin |
|---|---:|---:|---:|
| Get own ride | Yes | If operationally related | Yes if authorized |
| Mark en route | No | Assigned driver | Policy-dependent |
| Mark arrived | No | Assigned driver | Policy-dependent |
| Start trip | No | Assigned driver | Policy-dependent |
| Complete trip | No | Assigned driver | Policy-dependent |
| Cancel ride | Yes, subject to policy | Policy-dependent | Yes if authorized |

The final matrix will expand with operations/admin APIs.

---

# 46. State Transition Matrix

Initial lifecycle matrix:

| Current State | Command | Result |
|---|---|---|
| DRIVER_CONFIRMED | en-route | DRIVER_EN_ROUTE |
| DRIVER_EN_ROUTE | arrived | DRIVER_ARRIVED |
| DRIVER_ARRIVED | start | TRIP_STARTED |
| TRIP_STARTED | complete | TRIP_COMPLETED |
| Any invalid state | lifecycle command | Conflict/error |

Cancellation is intentionally handled separately because its policy differs by
state.

---

# 47. Invalid Transitions

Examples:

```text
DRIVER_CONFIRMED → start
DRIVER_EN_ROUTE → complete
DRIVER_ARRIVED → complete
TRIP_COMPLETED → start
TRIP_COMPLETED → complete
```

These must be rejected.

The client cannot skip lifecycle states unless a future explicit business rule
allows it.

---

# 48. Completion and Payment

Trip completion produces durable trip facts.

It should not synchronously execute the entire payment lifecycle.

Conceptually:

```text
TRIP_COMPLETED
      ↓
outbox event
      ↓
payment/fare domain
```

This prevents trip completion from becoming a distributed transaction across
multiple external systems.

---

# 49. Completion and Rating

Rating is similarly separate:

```text
TRIP_COMPLETED
      ↓
rating becomes eligible
```

The trip API should not directly create rider/driver ratings.

---

# 50. Completion and History

A completed ride becomes durable historical data.

History/query APIs can later expose completed rides without making the
completion command responsible for history-specific behavior.

---

# 51. Failure Handling

If PostgreSQL cannot commit a lifecycle transition:

```text
no successful transition
no fabricated response
```

If an outbox publication fails after commit:

```text
ride state remains committed
outbox retries
```

If Redis fails:

```text
current location may become temporarily unavailable/stale
ride lifecycle remains authoritative
```

---

# 52. Idempotent Event Consumers

Downstream consumers must tolerate duplicate lifecycle events.

For example:

```text
ride.trip_completed
ride.trip_completed
```

must not cause two payment captures or two rating eligibility records.

Downstream domains should use their own idempotency/uniqueness guarantees.

---

# 53. Security

Driver lifecycle commands require:

```text
authenticated driver
active assignment ownership
valid ride state
```

Do not trust:

```text
client-supplied driver_id
client-supplied assignment_id without authorization
client status fields
client timestamps for lifecycle ordering
```

---

# 54. Observability

Useful metrics include:

```text
trip_started_total
trip_completed_total
trip_cancellation_total
arrival_latency
en_route_duration
trip_duration
lifecycle_conflict_total
stale_location_total
```

Useful trace/log fields include:

```text
request_id
ride_id
assignment_id
driver_id
vehicle_id
```

Avoid using high-cardinality identifiers as metric labels.

---

# 55. Complete Lifecycle

```text
                    BIDDING
                       │
                       ▼
                  RESERVATION
                       │
                       ▼
              DRIVER_CONFIRMED
                       │
                       ▼
               DRIVER_EN_ROUTE
                       │
                       ▼
                DRIVER_ARRIVED
                       │
                       ▼
                 TRIP_STARTED
                       │
                       ▼
                TRIP_COMPLETED
                       │
              ┌────────┴────────┐
              ▼                 ▼
           PAYMENT           RATING
           domain             domain
```

The assignment is the durable driver/vehicle relationship spanning the
operational portion of the ride.

---

# 56. What We Should Not Build Yet

Do not build:

```text
full GPS history storage in PostgreSQL
complex geofencing engine
automatic lifecycle transitions from GPS alone
payment processing inside trip commands
rating processing inside trip commands
route optimization engine
multi-stop trip lifecycle
shared rides
scheduled rides
complex driver navigation logic
```

These can be added when product requirements justify them.

---

# 57. Design Principles

1. Assignment is the durable ride-driver-vehicle relationship created after confirmation.
2. The operational lifecycle is explicit: DRIVER_CONFIRMED → DRIVER_EN_ROUTE → DRIVER_ARRIVED → TRIP_STARTED → TRIP_COMPLETED.
3. Clients issue commands; servers own lifecycle transitions.
4. Lifecycle state is authoritative in PostgreSQL.
5. Driver and vehicle assignment invariants must prevent conflicting active rides.
6. Driver location is high-frequency operational data, not a PostgreSQL write on every GPS update.
7. Redis may hold current location and presence but does not own trip correctness.
8. Arrival, start, and completion are explicit commands.
9. GPS/geofence data can support validation but does not directly control lifecycle state.
10. Lifecycle commands must validate the current state and assignment ownership.
11. Driver lifecycle commands must be safe against mobile retries.
12. Concurrent lifecycle commands must produce one authoritative transition.
13. WebSocket events notify clients; REST provides authoritative recovery.
14. Durable lifecycle transitions produce outbox events.
15. Google Maps failures must not corrupt durable ride state.
16. Trip completion records trip facts but does not synchronously own payment or rating.
17. Payment, rating, and history remain separate domains.
18. Avoid turning the trip lifecycle into a GPS, routing, or payment subsystem.
