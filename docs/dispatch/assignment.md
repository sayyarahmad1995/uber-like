# Assignment & Driver Reservation

## 1. Purpose

This document defines how a rider's selected bid becomes a temporary driver
reservation and, ultimately, a confirmed assignment.

The assignment system must guarantee:

> A driver cannot be successfully committed to two conflicting rides at the
> same time.

This is a concurrency-critical part of the system.

---

# 2. Core Distinction

The following concepts are separate:

```text
Bid
    ↓
Driver's offer to perform the ride

Selection
    ↓
Rider chooses a bid

Reservation
    ↓
Driver is temporarily held for this ride

Assignment
    ↓
Driver confirms and becomes committed
```

Therefore:

```text
BID
≠
RESERVATION
≠
ASSIGNMENT
```

---

# 3. Assignment Lifecycle

Initial lifecycle:

```text
ACTIVE BID
     ↓
RIDER SELECTS BID
     ↓
RESERVED
     ↓
confirmation deadline
 ┌───┼───────────┐
 ↓   ↓           ↓
CONFIRMED   REJECTED    EXPIRED
 ↓           ↓           ↓
COMMITTED   RELEASED   RELEASED
```

The reservation exists only during the confirmation window.

---

# 4. Why Reservation Is Required

Consider:

```text
Ride A → Driver X
Ride B → Driver X
```

Both riders select Driver X at nearly the same time.

Without a reservation:

```text
Ride A
  ↓
confirmation request

Ride B
  ↓
confirmation request

Driver X
  ↓
two competing requests
```

This is a bad product experience and a concurrency problem.

Instead:

```text
Ride A
  ↓
reserve Driver X
  ↓
success

Ride B
  ↓
reserve Driver X
  ↓
conflict
  ↓
failure
```

The race is resolved by the backend before the driver receives a competing
confirmation request.

---

# 5. Source of Truth

PostgreSQL is authoritative for:

```text
reservation state
assignment state
ride state
bid state
driver commitment
```

Redis may provide:

```text
fast operational lookup
TTL-based expiry assistance
presence
coordination
```

But Redis must never be the only mechanism preventing double assignment.

If Redis says:

```text
driver = available
```

while PostgreSQL says:

```text
driver = reserved
```

PostgreSQL wins.

---

# 6. Reservation State

Initial reservation states:

```text
ACTIVE
CONFIRMED
REJECTED
EXPIRED
CANCELLED
```

Only `ACTIVE` represents a current temporary reservation.

A terminal reservation must never become active again.

---

# 7. Reservation Record

Conceptually:

```text
assignment_reservations
```

with fields such as:

```text
id
ride_id
driver_id
bid_id
status
reserved_at
expires_at
confirmed_at
released_at
created_at
updated_at
```

The exact schema belongs to the database design.

---

# 8. One Active Reservation Per Driver

The most important database invariant is:

```text
A driver may have at most one active reservation.
```

Conceptually:

```text
driver X
   └── ACTIVE reservation
```

must be unique.

PostgreSQL should enforce this at the database level.

A suitable implementation is a partial unique index over `driver_id` for
active reservations.

Conceptually:

```sql
UNIQUE (driver_id)
WHERE status = 'ACTIVE'
```

The exact migration will be designed later.

---

# 9. Why Application Checks Are Insufficient

This is unsafe:

```text
SELECT active reservation for driver X

if none:
    INSERT reservation
```

Two Go instances can execute:

```text
Go A:
SELECT → none

Go B:
SELECT → none

Go A:
INSERT

Go B:
INSERT
```

Both requests may believe the driver is available.

Therefore:

```text
check
+
insert
```

must be protected by a database constraint/transaction.

---

# 10. Rider Selects Bid

The rider performs:

```text
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

The backend must validate:

```text
authenticated user owns ride
ride is still accepting selection
bid belongs to ride
bid is still selectable
driver is still eligible
driver is not already reserved
driver is not already committed
bidding/selection deadline has not expired
```

The client cannot skip these checks.

---

# 11. Selection Transaction

The selection operation should occur inside one PostgreSQL transaction.

Conceptually:

```text
BEGIN

lock/revalidate ride
lock/revalidate bid
validate driver
create active reservation
update bid
update ride
create outbox events

COMMIT
```

If any invariant fails:

```text
ROLLBACK
```

No partial assignment state should remain.

---

# 12. Idempotency

Selection requests must be safe against retries.

Mobile applications can retry requests because of:

```text
network timeout
connection loss
HTTP retry
user double-tap
```

Example:

```text
Request 1
   ↓
reservation succeeds

Client does not receive response

Request 2
   ↓
same selection request
```

The second request must not create a second reservation.

The API should support an idempotency mechanism for this operation.

---

# 13. Selection Result

If selection succeeds:

```text
ACTIVE BID
    ↓
SELECTED
    ↓
ACTIVE RESERVATION
```

The server returns the authoritative result.

Example:

```json
{
  "ride_id": "ride_123",
  "bid_id": "bid_123",
  "reservation_id": "res_123",
  "status": "DRIVER_CONFIRMATION_REQUIRED",
  "expires_at": "2026-08-18T10:01:00Z"
}
```

---

# 14. Driver Notification

Only after the reservation transaction commits should the backend publish:

```text
assignment.confirmation_required
```

The flow is:

```text
PostgreSQL transaction
        ↓
COMMIT
        ↓
Outbox event
        ↓
Redis
        ↓
WebSocket
        ↓
Driver
```

The driver must never receive a confirmation request for an assignment that
was rolled back.

---

# 15. Driver Confirmation

The driver confirms through REST:

```text
POST /api/v1/rides/{ride_id}/assignment/confirm
```

The backend revalidates:

```text
authenticated driver
reservation belongs to driver
reservation is ACTIVE
reservation has not expired
ride is still assignable
```

The driver cannot confirm another driver's reservation.

---

# 16. Confirmation Transaction

Conceptually:

```text
BEGIN

lock reservation
lock ride
validate reservation
validate expiration
validate ride
mark reservation CONFIRMED
mark bid SELECTED/ACCEPTED
assign driver to ride
mark driver committed
update ride state
create outbox events

COMMIT
```

The transaction establishes the final assignment atomically.

---

# 17. Driver Commitment

After successful confirmation:

```text
Driver X
    ↓
COMMITTED
```

The driver must no longer be eligible for conflicting new assignments.

Example:

```text
Driver X
   └── Ride A → DRIVER_CONFIRMED
```

A second ride must not successfully confirm Driver X.

---

# 18. Assignment Invariant

The system must enforce:

```text
A driver cannot have two simultaneously active conflicting assignments.
```

This invariant must be protected at the database level wherever practical.

The exact implementation may use:

- Unique constraints
- Partial unique indexes
- Transactional row locking
- Explicit assignment state

The database design will determine the final combination.

---

# 19. Reservation Expiration

Every active reservation has:

```text
expires_at
```

Example:

```text
reserved_at = 10:00:20
expires_at  = 10:01:00
```

The backend determines whether the reservation is expired.

The client countdown is only UI.

---

# 20. Expiration Race

A critical race exists between:

```text
driver confirms
```

and:

```text
reservation expires
```

Example:

```text
10:00:59.999
Driver confirmation arrives

10:01:00
Reservation expires
```

The system must not rely on application clocks alone.

The database transaction must determine whether the reservation is still valid.

Conceptually:

```text
UPDATE reservation
SET status = 'CONFIRMED'
WHERE id = ?
  AND status = 'ACTIVE'
  AND expires_at > current_timestamp
```

If zero rows are affected:

```text
confirmation failed
```

This makes expiration authoritative.

---

# 21. Expiration Processing

Expiration can be processed by a background worker.

Conceptually:

```text
Worker
   ↓
find expired ACTIVE reservations
   ↓
mark EXPIRED
   ↓
create outbox event
```

The worker is not required to run at exactly `expires_at`.

Correctness comes from the transaction's expiration check.

Therefore:

```text
Worker timing
    ≠
business correctness
```

---

# 22. Redis TTL

Redis may maintain:

```text
dispatch:reservation:{driver_id}
```

with a TTL matching the reservation.

This provides fast operational visibility.

However:

```text
Redis TTL expires
```

must not itself be treated as proof that the PostgreSQL reservation has
expired.

PostgreSQL remains authoritative.

---

# 23. Driver Rejects

The driver can reject:

```text
POST /api/v1/rides/{ride_id}/assignment/reject
```

The backend validates the reservation and then atomically:

```text
ACTIVE
   ↓
REJECTED
```

The driver becomes available for future discovery/assignment, subject to all
other eligibility rules.

An outbox event is created in the same transaction.

---

# 24. Driver Does Not Respond

If the driver does not confirm before `expires_at`:

```text
ACTIVE
   ↓
EXPIRED
```

The driver becomes available again.

The ride does not automatically become assigned to another driver unless the
dispatch/assignment policy explicitly performs fallback selection.

---

# 25. Fallback Selection

After:

```text
REJECTED
```

or:

```text
EXPIRED
```

the backend may return the ride to assignment selection.

Possible strategies:

```text
next eligible bid
```

or:

```text
run another discovery round
```

The exact fallback strategy should be defined by the ride lifecycle.

The important architectural rule is:

> The client does not choose the fallback driver.

The backend remains responsible for assignment.

---

# 26. What Happens to the Other Bids?

Suppose:

```text
Ride A

Bid 1 → Driver X
Bid 2 → Driver Y
Bid 3 → Driver Z
```

Rider selects:

```text
Bid 1
```

Driver X becomes reserved.

The other bids should remain historically recorded but should no longer be
selectable once the ride has entered the confirmation process.

Conceptually:

```text
Bid 1 → SELECTED
Bid 2 → NO_LONGER_SELECTABLE
Bid 3 → NO_LONGER_SELECTABLE
```

The exact bid status vocabulary will be finalized in the bid domain design.

---

# 27. Selection and Ride State

Recommended state transition:

```text
BIDDING
   ↓
DRIVER_SELECTED
   ↓
DRIVER_CONFIRMATION_REQUIRED
   ↓
DRIVER_CONFIRMED
```

If confirmation fails:

```text
DRIVER_CONFIRMATION_REQUIRED
        ↓
REJECTED / EXPIRED
        ↓
fallback assignment
```

The ride must not be considered fully assigned merely because the rider
selected a bid.

---

# 28. Why Selection Is Not Confirmation

The rider selecting a driver does not guarantee that the driver will accept.

Therefore:

```text
Rider selection
    =
temporary reservation

Driver confirmation
    =
final commitment
```

This is important for both system correctness and user experience.

---

# 29. Driver Availability During Reservation

While a driver is:

```text
DRIVER_RESERVED
```

the driver should not receive new assignment opportunities.

The driver may still have other active bids depending on the product policy,
but new conflicting assignment selection must be prevented.

The reservation is therefore an availability boundary.

---

# 30. Multiple Bids During Reservation

Suppose Driver X has:

```text
Bid A
Bid B
Bid C
```

and Ride A selects Bid A.

Driver X becomes:

```text
RESERVED
```

The system should prevent another ride from selecting Driver X during the
reservation window.

Existing bids may be marked unavailable or left historically intact, but they
must not be capable of producing a conflicting confirmed assignment.

---

# 31. Reservation Cancellation

Reservations may also be cancelled by backend operations such as:

```text
ride cancellation
administrative cancellation
system recovery
```

The reservation transitions:

```text
ACTIVE
   ↓
CANCELLED
```

The driver then becomes available again if no other commitment exists.

---

# 32. Ride Cancellation Race

Consider:

```text
Rider cancels ride
        +
Driver confirms assignment
```

at nearly the same time.

Only one valid state transition may win.

Both operations must use transactional validation.

Example:

```text
Transaction A
    lock ride
    cancel ride

Transaction B
    lock ride
    confirm assignment
```

Whichever transaction obtains the required lock first determines the valid
state, and the second transaction must revalidate after acquiring the lock.

The system must never end in:

```text
Ride = CANCELLED
Driver = CONFIRMED
```

unless that combination is explicitly supported by the domain model.

---

# 33. Database Locking

Application-level mutexes are insufficient.

They only protect one Go process.

Our deployment may eventually contain:

```text
Go A
Go B
Go C
```

Therefore synchronization must work across all instances.

PostgreSQL provides the shared concurrency boundary.

---

# 34. Redis Is Not the Assignment Lock

Do not implement:

```text
SETNX driver_lock
```

as the sole correctness mechanism.

Redis locks can be useful for operational coordination, but they should not be
the only protection against double assignment.

The authoritative invariant belongs in PostgreSQL.

---

# 35. Transaction Isolation

The assignment implementation must choose an appropriate PostgreSQL
transaction strategy.

The initial design should prefer explicit row locking and database constraints
over unnecessarily broad serializable transactions.

The goal is:

```text
small transaction
+
clear locking order
+
database-enforced invariants
```

rather than locking large portions of the system.

---

# 36. Lock Ordering

Where multiple rows must be locked, the implementation should use a
consistent ordering.

For example:

```text
ride
  ↓
reservation/driver
  ↓
assignment
```

or another documented ordering.

All code paths that participate in the same concurrency domain must follow the
same ordering.

This reduces deadlock risk.

The exact order will be finalized during implementation.

---

# 37. Stale Client Requests

A client may send a request based on stale UI state.

Example:

```text
Client believes:
reservation ACTIVE

Server:
reservation EXPIRED
```

The server must reject the stale command.

The client then refreshes authoritative state.

Never trust the client's local state for assignment decisions.

---

# 38. Duplicate Confirmation

A driver may tap confirm twice.

Example:

```text
Request 1 → confirms
Request 2 → arrives immediately after
```

The second request should be idempotent or return the already-confirmed state
rather than creating another assignment.

There must be exactly one assignment.

---

# 39. Duplicate Rejection

The same principle applies to rejection.

If:

```text
Request 1 → reservation RELEASED
Request 2 → same request
```

the second request should not corrupt the state.

The API should return an appropriate idempotent result or a clear terminal-state
response.

---

# 40. Outbox Integration

Assignment state changes must use the outbox mechanism.

Example:

```text
BEGIN
   ↓
update reservation
update ride
update bid
insert outbox event
   ↓
COMMIT
```

After commit:

```text
Outbox
   ↓
Publisher
   ↓
Redis
   ↓
WebSocket
```

This prevents the system from publishing:

```text
assignment.confirmed
```

when the PostgreSQL transaction actually failed.

---

# 41. Confirmation Event

After successful confirmation:

```text
assignment.confirmed
```

is published to authorized participants.

Example:

```json
{
  "id": "evt_400",
  "type": "assignment.confirmed",
  "version": 1,
  "timestamp": "2026-08-18T10:00:30Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 8
  }
}
```

---

# 42. Reservation Events

Useful events include:

```text
assignment.confirmation_required
assignment.confirmed
assignment.rejected
assignment.expired
```

The exact event names remain aligned with the WebSocket design.

---

# 43. Failure Handling

If reservation creation fails because another driver reservation already
exists:

```text
reservation conflict
```

the backend should return a controlled domain error.

The client should not retry blindly forever.

The ride can proceed to fallback selection.

---

# 44. Example Race

Two riders select the same driver.

```text
                Driver X
                   │
          ┌────────┴────────┐
          │                 │
       Ride A            Ride B
          │                 │
          ▼                 ▼
     Transaction A      Transaction B
          │                 │
          ▼                 ▼
     INSERT ACTIVE      INSERT ACTIVE
      reservation        reservation
          │                 │
          ▼                 ▼
       SUCCESS             CONFLICT
          │                 │
          ▼                 ▼
      Driver X          fallback
      reserved
```

The database constraint determines the winner.

This remains correct even when requests arrive at different Go instances.

---

# 45. Example Confirmation Race

```text
Reservation
expires at 10:01:00
```

At approximately the same time:

```text
Driver confirms
```

and:

```text
Expiration worker runs
```

Both attempt state transitions.

Only one transition from:

```text
ACTIVE
```

should succeed.

The other sees the reservation is no longer active.

---

# 46. State Transition Rules

Initial rules:

```text
ACTIVE
  → CONFIRMED
  → REJECTED
  → EXPIRED
  → CANCELLED

CONFIRMED
  → terminal

REJECTED
  → terminal

EXPIRED
  → terminal

CANCELLED
  → terminal
```

Terminal states cannot transition back to `ACTIVE`.

---

# 47. Assignment Record

A successful confirmation should create or finalize an assignment record.

Conceptually:

```text
assignments
```

may contain:

```text
id
ride_id
driver_id
vehicle_id
reservation_id
assigned_at
status
created_at
updated_at
```

The exact schema belongs to the database design.

The assignment is the durable record that the driver was actually committed
to the ride.

---

# 48. Historical Integrity

Once a driver has been assigned:

```text
ride
+
driver
+
vehicle
```

must remain historically traceable.

Later changes such as:

```text
driver goes offline
vehicle becomes inactive
driver becomes suspended
```

must not rewrite the historical assignment.

---

# 49. Assignment Completion

After assignment:

```text
DRIVER_CONFIRMED
```

the ride lifecycle continues:

```text
DRIVER_CONFIRMED
      ↓
DRIVER_ARRIVED
      ↓
TRIP_STARTED
      ↓
TRIP_COMPLETED
```

The assignment remains associated with the ride throughout the trip.

---

# 50. Driver Availability After Trip

Once the trip completes, the driver may become eligible for new rides.

Conceptually:

```text
TRIP_COMPLETED
      ↓
release commitment
      ↓
driver available
```

The exact availability transition must be coordinated with the driver's
offline state.

For example:

```text
ONLINE + trip completed
    → available

OFFLINE + trip completed
    → remains offline
```

---

# 51. Recovery After Backend Restart

Suppose a Go instance crashes during assignment.

The durable state is in PostgreSQL.

On restart:

```text
PostgreSQL
    ↓
read active reservations
    ↓
resume expiration processing
```

The system must not rely on in-memory Go state to know which drivers are
reserved.

---

# 52. Recovery After Redis Failure

If Redis becomes unavailable:

```text
WebSocket/pub-sub functionality may degrade
```

but PostgreSQL must still know:

```text
which driver is reserved
which driver is assigned
which ride is assigned
```

Redis recovery must not create duplicate assignments.

---

# 53. Observability

Assignment metrics should include:

```text
reservation_attempts_total
reservation_success_total
reservation_conflicts_total
reservation_expired_total
reservation_rejected_total
assignment_confirmed_total
assignment_confirmation_failures_total
assignment_latency
```

Logs/traces should include:

```text
ride_id
driver_id
bid_id
reservation_id
assignment_id
```

Tokens and sensitive authentication data must never be logged.

---

# 54. Recommended Initial Architecture

```text
                     Rider
                       │
                       │ select bid
                       ▼
                Go Assignment API
                       │
                       ▼
                  PostgreSQL
                       │
              ┌────────┼────────┐
              │        │        │
              ▼        ▼        ▼
           Ride       Bid   Reservation
                                │
                         unique active
                           driver
                                │
                                ▼
                         Outbox Event
                                │
                                ▼
                             Redis
                                │
                                ▼
                           WebSocket
                                │
                                ▼
                             Driver
                                │
                         confirm/reject
                                │
                                ▼
                         PostgreSQL
                                │
                       ┌────────┴────────┐
                       ▼                 ▼
                   CONFIRMED          RELEASED
                       │                 │
                       ▼                 ▼
                  Assignment          Fallback
```

---

# 55. What We Should Not Do

Do not:

```text
Use Redis as the assignment source of truth
Use an in-memory Go mutex for assignment locking
Trust client reservation state
Trust client countdowns
Create assignment before transaction commit
Publish confirmation before database commit
Allow duplicate active reservations
Use a background worker as the only expiration mechanism
Perform a full-table PostgreSQL driver scan as a discovery fallback
```

---

# 56. Design Principles

1. A bid is not an assignment.
2. Rider selection creates a temporary reservation.
3. Driver confirmation creates the final commitment.
4. PostgreSQL is authoritative for reservation and assignment state.
5. A driver may have at most one active reservation.
6. Database constraints must protect critical invariants.
7. Application-level pre-checks are not sufficient for concurrency safety.
8. Selection must be transactional.
9. Confirmation must be transactional.
10. Reservation expiration must be server-authoritative.
11. Confirmation and expiration must safely race against each other.
12. Redis may accelerate coordination but cannot provide assignment correctness.
13. WebSocket notifications occur after database commit.
14. Outbox events provide durable event publication.
15. Commands must tolerate mobile retries and duplicate requests.
16. Terminal reservation states cannot become active again.
17. Driver fallback selection remains a backend responsibility.
18. Historical assignments must remain immutable.
19. Backend restart must not lose reservation/assignment state.
20. Redis failure must not corrupt assignment state.
21. Consistent database locking order should reduce deadlocks.
22. The implementation should prefer narrow transactions and explicit invariants.
