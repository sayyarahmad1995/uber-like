# Reservation Domain

## 1. Purpose

This document defines the temporary commitment created when a rider selects a
bid and the lifecycle leading to driver confirmation and assignment.

The core flow is:

```text
Bid selected
     ↓
Reservation created
     ↓
Driver confirmation window
     │
     ├── CONFIRMED → Assignment
     ├── REJECTED  → release
     └── EXPIRED   → release
```

A reservation is a temporary commitment. It is not itself the final assignment.

---

# 2. Core Principles

1. Reservation creation follows successful bid selection.
2. Clients cannot arbitrarily create reservations.
3. A reservation belongs to one ride and one selected bid.
4. The reservation preserves the selected driver and vehicle.
5. A driver cannot have conflicting active reservations/assignments.
6. PostgreSQL is authoritative for reservation correctness.
7. Redis availability is not sufficient to guarantee reservation success.
8. Reservation confirmation must revalidate current state.
9. Expiration must be checked transactionally, not only by a background worker.
10. A reservation must either complete its transition or fail atomically.

---

# 3. Reservation Creation

There is no public endpoint such as:

```text
POST /api/v1/reservations
```

The reservation is created as a consequence of successful bid selection.

Conceptually:

```text
Rider selects bid
       ↓
validate ride
       ↓
validate bid
       ↓
revalidate driver/vehicle
       ↓
reserve driver
       ↓
create reservation
       ↓
close bidding
       ↓
ride → DRIVER_CONFIRMATION_REQUIRED
```

All of the state changes required for correctness should occur in one database
transaction.

---

# 4. Reservation Identity

A reservation requires a stable internal identifier.

Conceptually:

```text
Reservation
├── id
├── ride_id
├── bid_id
├── driver_id
├── vehicle_id
├── amount
├── currency
├── status
├── expires_at
└── timestamps
```

The exact database schema is a later design task.

---

# 5. Reservation Association

The reservation preserves the exact commercial and operational offer selected
by the rider.

It must retain at least:

```text
ride
bid
 driver
vehicle
agreed amount
currency
expiration
```

The association must not silently change after creation.

---

# 6. Reservation Lifecycle

The initial conceptual lifecycle is:

```text
                    ┌─────────────┐
                    │   PENDING   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
          confirm       reject       expire
              │            │            │
              ▼            ▼            ▼
         CONFIRMED      RELEASED     EXPIRED
              │
              ▼
         ASSIGNMENT
```

Some lifecycle information may be represented by assignment relationships
rather than duplicated as reservation status. The persistence model will be
finalized during database design.

---

# 7. PENDING

`PENDING` means:

```text
rider selected the bid
reservation exists
bidding is closed
waiting for driver confirmation
reservation has not expired
```

The driver is temporarily committed for this confirmation window.

---

# 8. CONFIRMED

`CONFIRMED` means the driver explicitly accepted the reservation within the
allowed confirmation window.

The system then proceeds to assignment.

Confirmation does not permit changing the selected vehicle or commercial terms.

---

# 9. RELEASED

`RELEASED` means the temporary commitment has been removed without producing a
confirmed assignment.

This can happen because of:

```text
driver rejection
administrative cancellation
other defined reservation failure
```

---

# 10. EXPIRED

`EXPIRED` means the driver did not confirm before `expires_at`.

Expiration releases the reservation according to the ride lifecycle policy.

An expired reservation must never be confirmed later.

---

# 11. Assignment Boundary

The reservation is not the assignment.

The initial flow is:

```text
Reservation PENDING
      ↓
Driver confirms
      ↓
Reservation CONFIRMED
      ↓
Assignment created
```

Assignment owns the final driver/vehicle commitment for the trip.

---

# 12. Reservation API

Initial public endpoints:

```text
GET  /api/v1/reservations/{reservation_id}
POST /api/v1/reservations/{reservation_id}/confirm
POST /api/v1/reservations/{reservation_id}/reject
```

There is intentionally no public reservation-create endpoint.

---

# 13. Get Reservation

```http
GET /api/v1/reservations/{reservation_id}
Authorization: Bearer <token>
```

The caller must be authorized to view the reservation.

A rider may view a reservation associated with their ride.

A driver may view a reservation for which they are the selected driver.

Administrative access requires explicit authorization.

---

# 14. Confirm Reservation

```http
POST /api/v1/reservations/{reservation_id}/confirm
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The authenticated driver must own the reservation.

The server verifies:

```text
reservation = PENDING
not expired
driver still authorized
driver still eligible
vehicle still valid
ride still compatible
```

Only then can confirmation succeed.

---

# 15. Reject Reservation

```http
POST /api/v1/reservations/{reservation_id}/reject
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The authenticated driver must own the reservation.

Successful rejection releases the temporary commitment and moves the ride into
its defined fallback path.

---

# 16. Confirmation Is Not Client-Controlled Assignment

The driver cannot directly request:

```text
POST /assignments
```

as a substitute for confirmation.

Assignment is created by the backend after the reservation confirmation
transition succeeds.

This keeps the lifecycle authoritative and prevents clients from bypassing
reservation rules.

---

# 17. Reservation Creation Transaction

The bid-selection transaction should conceptually be:

```text
BEGIN

authorize rider
lock/revalidate ride
lock/revalidate selected bid
validate ride = BIDDING
validate bid = ACTIVE
validate bid not expired
revalidate driver
revalidate vehicle
verify no conflicting driver commitment
create reservation
close competing bids
transition ride → DRIVER_CONFIRMATION_REQUIRED
create outbox events

COMMIT
```

The exact PostgreSQL locking strategy is a later database-design decision.

---

# 18. Driver Commitment Invariant

The most important reservation invariant is:

```text
A driver cannot have two conflicting active reservations/assignments.
```

Example that must not succeed:

```text
Ride A → Driver X
Ride B → Driver X
```

at the same time when the rides conflict operationally.

This must be enforced by authoritative transactional state, not only by Redis.

---

# 19. Driver Availability Race

Example:

```text
10:00 Driver available
10:01 Bid submitted
10:02 Driver reserved for Ride B
10:03 Rider selects original bid for Ride A
```

The Ride A selection must fail if Driver X cannot be reserved.

The stale bid is not a guarantee of availability.

---

# 20. Two Riders Select Same Driver

Example:

```text
Ride A → Bid A → Driver X
Ride B → Bid B → Driver X
```

Two riders may attempt selection concurrently.

Only one reservation may acquire the conflicting driver commitment.

The other transaction must fail cleanly with a conflict/error and must not
create a partial reservation.

---

# 21. Reservation Expiration

Every pending reservation has an authoritative `expires_at`.

Conceptually:

```text
PENDING
   ↓
expires_at reached
   ↓
EXPIRED
   ↓
release commitment
   ↓
ride fallback policy
```

The exact timeout is configuration, not a hardcoded business assumption.

---

# 22. Expiration Worker

A background worker may process expired reservations:

```text
find pending reservations past expires_at
      ↓
transactionally revalidate
      ↓
mark expired
      ↓
release commitment
      ↓
write outbox event
```

The worker is useful for timely cleanup and notifications.

But it is not the authority for validity.

---

# 23. Expiration at Confirmation Time

A driver may attempt confirmation after `expires_at` but before the expiration
worker runs.

The confirmation transaction must directly evaluate:

```text
now < expires_at
```

If false:

```text
409 Conflict
```

or the standardized reservation-expired error is returned.

The reservation must not become confirmed merely because the worker has not run
yet.

---

# 24. Driver Confirmation Race With Expiration

These operations may race:

```text
Driver confirms
      +
Expiration worker expires
```

Only one valid transition may win according to the transaction and current
server time.

The final state must be unambiguous:

```text
CONFIRMED
```

or:

```text
EXPIRED
```

Never both.

---

# 25. Driver Rejection Race With Expiration

Likewise:

```text
Driver rejects
      +
expiration worker expires
```

Both operations are terminal for the pending reservation.

Exactly one transition wins transactionally.

The losing request observes the already-terminal state.

---

# 26. Rider Cancellation Race

Rider cancellation can race with driver confirmation.

Example:

```text
Rider cancels
      +
Driver confirms
```

The system must serialize the competing lifecycle transitions according to the
ride state machine.

It must not produce:

```text
ride CANCELLED
and
assignment CONFIRMED
```

as an inconsistent final state.

---

# 27. Reservation and Ride State

Creating a reservation causes:

```text
BIDDING
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

Confirmation then causes the next ride/assignment transition according to the
ride lifecycle.

Reservation status must not independently invent a conflicting ride state.

---

# 28. Reservation and Competing Bids

When reservation creation succeeds:

```text
selected bid → reserved
other active bids → closed/non-selectable
```

The system must prevent another rider selection from creating a second
reservation for the same ride.

---

# 29. Selected Bid Immutability

Once a reservation exists, the selected bid's commercial terms are fixed:

```text
amount
currency
driver
vehicle
ride
```

The driver cannot update the bid amount after selection.

The rider cannot modify the selected bid's price through the ride API.

---

# 30. Vehicle Immutability

The reservation must preserve the selected vehicle.

If the driver changes vehicles before confirmation, the initial policy is to
reject confirmation and follow the reservation/ride fallback path rather than
silently replacing the vehicle.

This protects the rider from receiving a different vehicle than the one they
selected.

---

# 31. Driver Eligibility Revalidation

At confirmation time, the backend should verify relevant current eligibility.

Examples:

```text
driver suspended
service authorization revoked
vehicle compliance invalid
vehicle inactive
```

If confirmation is no longer valid, the reservation must not become confirmed.

---

# 32. Driver Identity and Authorization

Confirmation and rejection use authenticated driver identity.

The request must not trust a client-provided:

```json
{
  "driver_id": "..."
}
```

The server determines the driver from the authenticated application identity
and verifies ownership of the reservation.

---

# 33. Idempotent Confirmation

Confirmation must support idempotency.

Example:

```text
confirm reservation
      ↓
server succeeds
      ↓
response lost
      ↓
client retries same key
```

The retry must not create a second assignment or duplicate lifecycle transition.

---

# 34. Idempotent Rejection

Rejection should likewise support idempotency.

Repeated requests with the same idempotency key represent one logical rejection
operation.

---

# 35. Confirmation After Success

If the driver retries confirmation after the reservation has already been
confirmed, the response should reflect the already-established outcome when the
request is recognized as the same idempotent operation.

A different confirmation attempt after the operation is already terminal should
return the appropriate state/conflict response.

---

# 36. Rejection After Confirmation

A normal rejection endpoint must not reverse a confirmed reservation.

Once:

```text
PENDING → CONFIRMED
```

post-confirmation cancellation/release belongs to the appropriate trip/
assignment lifecycle.

---

# 37. Reservation Response

A successful confirmation should return authoritative state sufficient for the
client to proceed.

Conceptually:

```json
{
  "data": {
    "id": "reservation_123",
    "status": "CONFIRMED",
    "ride_id": "ride_123",
    "driver_id": "driver_123",
    "vehicle_id": "vehicle_123",
    "expires_at": "2026-08-18T10:32:00Z"
  }
}
```

The final public representation will be refined with the assignment API.

---

# 38. Reservation Visibility

A rider may see their reservation and relevant selected driver/vehicle details.

The selected driver may see their own reservation details.

Internal operational information must not be exposed unnecessarily.

---

# 39. Reservation Events

Important transitions should produce durable outbox events.

Examples:

```text
reservation.created
reservation.confirmed
reservation.rejected
reservation.expired
reservation.released
```

The exact event names will be finalized in the event contract.

---

# 40. Event Delivery

Reservation events should use the same at-least-once delivery assumptions as
other domain events.

Consumers must tolerate duplicates.

A duplicate event must not create duplicate assignments or reservations.

---

# 41. PostgreSQL Responsibilities

PostgreSQL is authoritative for:

```text
reservation identity
ride association
bid association
driver association
vehicle association
status
expiration
commercial terms
concurrency/version information
created/updated timestamps
```

The exact tables and indexes will be designed later.

---

# 42. Redis Responsibilities

Redis may support:

```text
fast availability lookup
presence
notification fan-out
short-lived operational state
```

Redis must not be the final authority for:

```text
reservation ownership
driver commitment
reservation confirmation
assignment correctness
```

---

# 43. Database Invariants

The database design should enforce, where practical:

```text
reservation belongs to one ride
reservation belongs to one bid
reservation preserves one driver and vehicle
selected bid cannot produce multiple active reservations
one ride cannot have multiple active reservation commitments
one driver cannot have conflicting active reservations/assignments
```

The exact PostgreSQL constraints/indexes are a later database design task.

---

# 44. Transaction Boundaries

The following operations must be transactionally authoritative:

```text
bid selection + reservation creation
reservation confirmation + assignment creation
reservation expiration/release
reservation rejection/release
```

External notification delivery should not be required to complete the database
transaction.

Use the outbox pattern for durable event publication.

---

# 45. Assignment Creation

After confirmation:

```text
BEGIN

lock/revalidate reservation
validate reservation = PENDING or valid confirmable state
validate not expired
validate driver/vehicle state
transition reservation → CONFIRMED
create assignment
transition ride according to lifecycle
create outbox events

COMMIT
```

The exact assignment lifecycle is the next domain design task.

---

# 46. No Partial Confirmation

The backend must not produce:

```text
reservation CONFIRMED
but
assignment missing
```

as the result of a successful confirmation transaction if assignment creation is
part of that transition.

If assignment creation is deliberately separated, the system must have an
explicit durable intermediate state and recovery mechanism.

The initial recommendation is to create the assignment within the same
transaction as confirmation.

---

# 47. Reservation Failure Handling

If PostgreSQL cannot commit reservation creation:

```text
no successful selection
no reservation
no assignment
```

The API must not return a false success.

If event publication fails after commit:

```text
reservation remains committed
outbox retries publication
```

---

# 48. Redis Failure During Selection

If Redis is unavailable during bid selection, the system should not blindly
assume availability.

The final reservation decision must use authoritative transactional state.

If the system cannot safely determine whether the driver can be reserved, it
should fail safely rather than create a potentially conflicting reservation.

---

# 49. Driver Confirmation Failure

If confirmation fails because the reservation is no longer valid:

```text
no assignment
reservation remains/enters terminal state according to transaction
ride follows fallback policy
```

The API should expose a stable application error code such as:

```text
RESERVATION_EXPIRED
RESERVATION_NOT_CONFIRMABLE
DRIVER_NOT_ELIGIBLE
VEHICLE_NOT_ELIGIBLE
```

---

# 50. Fallback After Release

When a reservation is released because of rejection or expiration, the ride
enters the fallback policy defined by the ride/dispatch lifecycle.

The reservation domain should not independently decide how many new drivers to
search for or how long the ride remains open.

That belongs to dispatch/ride lifecycle policy.

---

# 51. Notifications

The rider should receive events for:

```text
reservation.created
reservation.confirmed
reservation.rejected
reservation.expired
```

The driver should receive events for:

```text
reservation.created
reservation.cancelled/released where relevant
```

Clients can recover authoritative state through REST if an event is missed.

---

# 52. Timeout Configuration

The confirmation timeout should be configurable.

Do not hardcode business assumptions such as:

```text
exactly 30 seconds forever
```

Configuration should allow controlled tuning after observing real system
behavior.

---

# 53. Clock Authority

Expiration uses server-side time.

The Flutter client's clock must never determine whether a reservation is valid.

For example:

```text
client says reservation has 5 seconds left
```

is not authoritative.

The backend compares current server time with `expires_at`.

---

# 54. Reservation and Payment

Payment should not be embedded into the initial reservation state machine.

The reservation can preserve agreed commercial terms, but payment authorization
and capture belong to a separate payment domain.

This prevents the reservation model from becoming an oversized transaction
coordinator.

---

# 55. Reservation and Cancellation

Rider cancellation may affect a pending reservation.

The cancellation transaction must coordinate with reservation state.

For example:

```text
rider cancels
   ↓
pending reservation released
   ↓
driver no longer committed
```

The exact cancellation fees/policies belong to the cancellation/pricing domain.

---

# 56. Reservation and Trip Start

Once the driver confirms and assignment is created, reservation responsibilities
shrink.

The trip lifecycle owns subsequent states such as:

```text
DRIVER_CONFIRMED
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
```

Do not keep extending reservation status to represent the entire ride lifecycle.

---

# 57. Observability

Useful metrics include:

```text
reservations_created_total
reservations_confirmed_total
reservations_rejected_total
reservations_expired_total
reservation_conflicts_total
reservation_confirmation_latency
```

Useful tracing/logging fields include:

```text
request_id
reservation_id
ride_id
bid_id
driver_id
vehicle_id
```

Avoid using high-cardinality IDs as metric labels.

---

# 58. Security

Reservation operations must enforce:

```text
authenticated identity
resource ownership
role permissions
state-transition permissions
```

Do not expose reservation details to unrelated users.

Do not allow a driver to confirm another driver's reservation.

---

# 59. What We Should Not Build Yet

Do not build:

```text
public reservation creation endpoint
multiple simultaneous reservations per driver
automatic reservation reassignment inside the reservation domain
payment processing inside reservation logic
complex reservation auctions
vehicle replacement during pending confirmation
long-running synchronous reservation requests
```

Keep reservation focused on temporary commitment and transition to assignment.

---

# 60. Complete Flow

```text
                    RIDER
                      │
                      ▼
                 Select Bid
                      │
                      ▼
              ┌───────────────┐
              │ PostgreSQL TX │
              │               │
              │ validate bid  │
              │ validate ride │
              │ validate      │
              │ driver/vehicle│
              │ reserve driver│
              └───────┬───────┘
                      │
                      ▼
                RESERVATION
                  PENDING
                      │
             ┌────────┼────────┐
             │        │        │
          confirm   reject    expire
             │        │        │
             ▼        ▼        ▼
        CONFIRMED   RELEASED  EXPIRED
             │
             ▼
         ASSIGNMENT
             │
             ▼
            RIDE
```

---

# 61. Design Principles

1. A reservation is a temporary commitment, not the final assignment.
2. Reservation creation follows successful rider bid selection.
3. Clients cannot arbitrarily create reservations.
4. A reservation preserves the selected ride, bid, driver, vehicle, and commercial terms.
5. A driver cannot hold conflicting active reservations/assignments.
6. PostgreSQL is authoritative for reservation correctness.
7. Redis is an accelerator, not the final commitment authority.
8. Reservation creation and competing-bid closure must be transactionally consistent.
9. Driver confirmation revalidates reservation, eligibility, vehicle, and expiration.
10. Reservation expiration is based on server time.
11. Background expiration workers are cleanup/notification mechanisms, not validity authorities.
12. Confirmation and expiration races must produce exactly one terminal outcome.
13. Rider cancellation and driver confirmation must be serialized against the ride lifecycle.
14. Selected commercial terms are immutable after reservation creation.
15. Vehicle replacement is not silently allowed during pending confirmation.
16. Confirmation should create assignment transactionally in the initial design.
17. Important reservation transitions use durable outbox events.
18. Event delivery is at-least-once and consumers must tolerate duplicates.
19. Payment remains a separate domain.
20. Reservation logic stops once assignment owns the active trip lifecycle.
