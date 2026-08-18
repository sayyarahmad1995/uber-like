# Ride Lifecycle

## 1. Purpose

This document defines the authoritative lifecycle of a ride from request
creation through completion, cancellation, or failure.

The ride lifecycle is the backbone connecting:

- Rider request
- Driver discovery
- Driver bidding
- Rider selection
- Driver reservation
- Driver confirmation
- Pickup
- Trip execution
- Completion
- Cancellation and failure handling

Every ride must always have one authoritative state.

---

# 2. Core Principle

Ride state is business state.

Temporary operational conditions such as:

```text
WebSocket disconnected
Redis unavailable
mobile application backgrounded
notification delivery failed
```

do not independently change the ride state.

The backend and PostgreSQL are authoritative.

---

# 3. Initial State Model

The recommended lifecycle is:

```text
REQUESTED
    ↓
DISCOVERY
    ↓
BIDDING
    ↓
DRIVER_CONFIRMATION_REQUIRED
    ↓
DRIVER_CONFIRMED
    ↓
DRIVER_ARRIVED
    ↓
TRIP_STARTED
    ↓
TRIP_COMPLETED
```

Terminal outcomes include:

```text
CANCELLED
NO_DRIVER_FOUND
ASSIGNMENT_FAILED
TRIP_INTERRUPTED
```

The exact final state vocabulary can evolve, but the responsibilities should
remain stable.

---

# 4. State Definitions

## REQUESTED

The rider has successfully created a ride request.

The ride has not yet entered active driver discovery.

---

## DISCOVERY

The system is finding potentially eligible drivers.

This state covers the operational discovery phase.

```text
Ride
 ↓
Redis GEO
 ↓
candidate drivers
 ↓
eligibility
```

The ride should not remain in discovery indefinitely.

---

## BIDDING

Eligible discovered drivers can submit bids.

The rider can receive and review valid bids.

The ride has a bidding deadline.

```text
BIDDING
 ├── driver submits bid
 ├── driver updates bid
 ├── driver withdraws bid
 └── rider selects bid
```

---

## DRIVER_CONFIRMATION_REQUIRED

The rider has selected a bid and the selected driver has been temporarily
reserved.

The driver must confirm before the reservation expires.

This state means:

```text
rider selected
+
reservation exists
+
driver confirmation pending
```

It does not yet mean the trip is assigned.

---

## DRIVER_CONFIRMED

The selected driver has confirmed the reservation and the assignment is now
committed.

The driver is no longer available for conflicting assignments.

---

## DRIVER_ARRIVED

The assigned driver has arrived at the pickup location according to the
product's arrival criteria.

The exact geofence/distance rule will be defined later.

---

## TRIP_STARTED

The rider and driver have begun the actual trip.

The system can now track trip progress against the active assignment.

---

## TRIP_COMPLETED

The trip has been successfully completed.

This is a terminal ride state for the initial lifecycle.

Payment, receipts, ratings, and other post-trip workflows may occur after this
state without changing the ride's completion state.

---

## CANCELLED

The ride was cancelled before successful completion.

Cancellation may be initiated by:

- Rider
- Driver where product policy permits
- Backend/system policy
- Administrative operation

The exact cancellation rules depend on the current ride state.

---

## NO_DRIVER_FOUND

The system could not obtain a valid driver assignment within the allowed
bidding/discovery policy.

This is a business outcome, not an infrastructure error.

---

## ASSIGNMENT_FAILED

The system reached an assignment failure that could not be recovered through
the normal fallback process.

Examples include persistent assignment conflicts or an unrecoverable failure
in the assignment workflow.

---

## TRIP_INTERRUPTED

The trip began but could not complete normally.

Examples may include:

```text
vehicle breakdown
safety event
forced trip termination
```

The exact business rules for this state will be defined later.

---

# 5. Primary State Transitions

The normal successful flow is:

```text
REQUESTED
   ↓
DISCOVERY
   ↓
BIDDING
   ↓
DRIVER_CONFIRMATION_REQUIRED
   ↓
DRIVER_CONFIRMED
   ↓
DRIVER_ARRIVED
   ↓
TRIP_STARTED
   ↓
TRIP_COMPLETED
```

---

# 6. Discovery Transitions

Normal transition:

```text
REQUESTED
    ↓
DISCOVERY
```

If eligible candidates are found:

```text
DISCOVERY
    ↓
BIDDING
```

If no candidates are found after the configured retry policy:

```text
DISCOVERY
    ↓
NO_DRIVER_FOUND
```

If the rider cancels:

```text
DISCOVERY
    ↓
CANCELLED
```

---

# 7. Bidding Transitions

Normal flow:

```text
BIDDING
   ↓
rider selects valid bid
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

If bidding expires without a usable assignment:

```text
BIDDING
   ↓
NO_DRIVER_FOUND
```

If the rider cancels:

```text
BIDDING
   ↓
CANCELLED
```

---

# 8. Selection and Reservation

Rider selection is not by itself a final assignment.

The transition must be atomic with reservation creation.

Conceptually:

```text
BIDDING
   ↓
select bid
   ↓
create driver reservation
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

If reservation creation fails:

```text
BIDDING
   ↓
reservation conflict
   ↓
fallback selection / bidding
```

The ride must not move to confirmation-required unless the reservation was
successfully created.

This intentionally does not introduce a durable `DRIVER_SELECTED` ride state.
The selected bid and reservation provide the intermediate assignment context.

---

# 9. Driver Confirmation

Normal transition:

```text
DRIVER_CONFIRMATION_REQUIRED
          ↓
      driver confirms
          ↓
DRIVER_CONFIRMED
```

If the driver rejects:

```text
DRIVER_CONFIRMATION_REQUIRED
          ↓
reservation released
          ↓
fallback selection / bidding
```

If the reservation expires:

```text
DRIVER_CONFIRMATION_REQUIRED
          ↓
reservation EXPIRED
          ↓
fallback selection / bidding
```

If the rider cancels:

```text
DRIVER_CONFIRMATION_REQUIRED
          ↓
CANCELLED
```

---

# 10. Driver Arrival

Normal transition:

```text
DRIVER_CONFIRMED
       ↓
driver reaches pickup
       ↓
DRIVER_ARRIVED
```

Arrival should be determined by backend rules, not merely by the driver
pressing a button.

The initial implementation may combine:

```text
driver location
+
pickup geofence
+
explicit driver action
```

The exact policy will be designed later.

---

# 11. Trip Start

Normal transition:

```text
DRIVER_ARRIVED
       ↓
trip starts
       ↓
TRIP_STARTED
```

The system should define a clear authoritative trip-start event.

The rider or driver client must not be able to create an impossible state such
as:

```text
TRIP_STARTED
```

without a committed assignment.

---

# 12. Trip Completion

Normal transition:

```text
TRIP_STARTED
       ↓
trip completed
       ↓
TRIP_COMPLETED
```

Completion should be authoritative on the server.

The final trip location, timestamps, and assignment association should be
persisted as appropriate.

---

# 13. Cancellation Rules

Cancellation is state-dependent.

The system should not implement:

```text
any state → CANCELLED
```

without defining the business consequences.

For example, cancellation during:

```text
BIDDING
```

is very different from cancellation during:

```text
TRIP_STARTED
```

The latter may require cancellation fees, driver compensation, or other
business rules.

Those rules are intentionally deferred.

---

# 14. Rider Cancellation

The rider may request cancellation through an authenticated API.

Conceptually:

```text
POST /api/v1/rides/{ride_id}/cancel
```

The backend validates whether cancellation is allowed in the current state.

The backend then performs the state transition transactionally.

---

# 15. Driver Cancellation

Driver cancellation must be restricted by policy.

The driver cannot arbitrarily cancel every ride state without consequences.

Examples:

```text
before confirmation
possibly allowed as rejection

after confirmation
may be treated as cancellation

after trip start
may require special interruption handling
```

The exact rules remain a product decision.

---

# 16. No Driver Found

A ride can reach `NO_DRIVER_FOUND` when the dispatch policy cannot produce a
usable assignment.

Example:

```text
DISCOVERY
   ↓
insufficient candidates
   ↓
retry
   ↓
BIDDING
   ↓
zero valid bids
   ↓
NO_DRIVER_FOUND
```

The system should distinguish this business outcome from infrastructure
failure.

---

# 17. Assignment Failure

Assignment failure occurs when the normal selection/reservation process cannot
produce a valid assignment and recovery is exhausted.

Example:

```text
BIDDING
   ↓
select bid
   ↓
reservation conflict
   ↓
fallback
   ↓
all viable options exhausted
   ↓
ASSIGNMENT_FAILED
```

Whether this ultimately maps to `NO_DRIVER_FOUND` or remains a separate
operational state can be revisited after implementation experience.

---

# 18. Trip Interruption

Once the trip starts, cancellation semantics change.

A trip interruption may occur because of:

```text
vehicle problem
safety event
forced termination
system-defined exceptional condition
```

The ride should not be falsely marked `TRIP_COMPLETED` when the trip did not
complete normally.

---

# 19. Invalid Transitions

Examples of invalid transitions include:

```text
REQUESTED → TRIP_STARTED
BIDDING → TRIP_STARTED
BIDDING → TRIP_COMPLETED
DRIVER_CONFIRMATION_REQUIRED → TRIP_STARTED
TRIP_COMPLETED → BIDDING
TRIP_COMPLETED → DRIVER_CONFIRMED
```

The backend must reject invalid state transitions.

The client must never be able to force a state transition by sending an
arbitrary state value.

---

# 20. State Transition Authority

Only the backend may change ride state.

Clients send commands such as:

```text
request ride
cancel ride
submit bid
select bid
confirm assignment
reject assignment
arrived
start trip
complete trip
```

The backend decides whether each command is valid and performs the resulting
state transition.

A client must never send:

```text
PATCH /ride
{
  "status": "TRIP_COMPLETED"
}
```

as a generic state mutation API.

---

# 21. Atomic State Changes

A state transition and its related domain changes must occur in one
transaction whenever they are logically inseparable.

Example: rider selects a bid.

```text
BEGIN

lock/revalidate ride
validate bid
create reservation
close bidding
transition ride state
create outbox event

COMMIT
```

The system must not produce:

```text
ride = DRIVER_CONFIRMATION_REQUIRED
reservation = missing
```

---

# 22. Assignment Confirmation Transaction

Driver confirmation should atomically update:

```text
reservation
assignment
ride state
bid state
outbox events
```

Conceptually:

```text
BEGIN

validate active reservation
validate expiration
create/finalize assignment
mark reservation confirmed
transition ride
create outbox event

COMMIT
```

---

# 23. Expiration and State

The backend determines reservation expiration.

If the confirmation deadline passes:

```text
DRIVER_CONFIRMATION_REQUIRED
          ↓
reservation expires
          ↓
fallback selection / bidding
```

The ride must not be considered assigned merely because the client timer has
not updated yet.

---

# 24. Idempotency

Ride commands must tolerate mobile retries.

Examples:

```text
cancel request retried
bid selection retried
assignment confirmation retried
trip completion retried
```

Repeated valid requests should not create duplicate domain records or invalid
state transitions.

The API should use idempotency keys where appropriate and state-aware handling
for commands that naturally become no-ops after the first successful request.

---

# 25. Concurrency

Important concurrent operations include:

```text
rider selects bid
      +
other ride reserves same driver
```

```text
driver confirms
      +
reservation expires
```

```text
rider cancels
      +
driver confirms
```

```text
driver rejects
      +
rider selects another bid
```

PostgreSQL transactions, constraints, and consistent locking rules must resolve
these races.

---

# 26. State Version

A ride should have a monotonically increasing state revision/version.

Example:

```text
revision 1 → REQUESTED
revision 2 → DISCOVERY
revision 3 → BIDDING
revision 4 → DRIVER_CONFIRMATION_REQUIRED
revision 5 → DRIVER_CONFIRMED
```

The revision helps with:

- WebSocket event ordering
- Optimistic concurrency
- Client state reconciliation
- Debugging

Clients should not invent revisions.

---

# 27. Events

Important lifecycle events may include:

```text
ride.requested
ride.discovery_started
ride.bidding_started
ride.bidding_closed
ride.driver_confirmation_required
ride.driver_confirmed
ride.driver_arrived
ride.trip_started
ride.trip_completed
ride.cancelled
ride.no_driver_found
ride.assignment_failed
ride.trip_interrupted
```

Events are notifications of committed state changes.

They are not the source of truth.

---

# 28. Outbox Integration

Lifecycle events should use the transactional outbox pattern.

Example:

```text
BEGIN
   ↓
change ride state
   ↓
insert outbox event
   ↓
COMMIT
```

After commit:

```text
Outbox publisher
      ↓
Redis
      ↓
WebSocket
      ↓
clients
```

This prevents an event from being published when the corresponding state
change was rolled back.

---

# 29. WebSocket Recovery

WebSocket delivery is not guaranteed.

If a client misses:

```text
ride.driver_confirmed
```

it can retrieve the authoritative ride state through REST.

Conceptually:

```text
GET /api/v1/rides/{ride_id}
```

The response includes the current ride state and revision.

---

# 30. Client State Handling

The Flutter client should treat the backend state as authoritative.

The client may optimistically update UI for responsiveness, but it must
reconcile with the server response.

For example:

```text
Client shows:
"Cancelling..."

Server responds:
CANCELLED
```

The final state comes from the server.

---

# 31. Recovery After Backend Restart

Ride state must survive Go process restarts.

The backend can reconstruct the current state from PostgreSQL.

No critical ride state should exist only in:

```text
Go memory
Redis
WebSocket connections
```

---

# 32. Recovery After Redis Failure

Redis may affect:

```text
real-time delivery
presence
location discovery
operational coordination
```

but PostgreSQL must continue to contain the authoritative ride state.

Redis recovery must not cause the ride to move backward or produce duplicate
state transitions.

---

# 33. Historical State

The current ride state is not enough for operational debugging.

The system should retain enough transition history or event/audit data to
answer:

```text
When did the ride enter each state?
Why did it change?
Which command caused it?
Which actor caused it?
What was the previous revision?
```

The exact audit schema will be designed later.

---

# 34. Assignment History

The ride should retain the relationship between:

```text
ride
bid
reservation
assignment
```

Example:

```text
Ride 123
   ↓
Bid 456
   ↓
Reservation 789
   ↓
Assignment 101
```

If the reservation fails, the historical bid and failure should remain
traceable.

---

# 35. Driver Availability

Ride state and driver availability are related but separate.

For example:

```text
Ride = TRIP_COMPLETED
Driver = OFFLINE
```

is perfectly valid.

Completing a ride does not automatically mean the driver is online.

Likewise:

```text
Driver = ONLINE
```

does not mean the driver is available for assignment if the driver is currently
committed to another ride.

---

# 36. Payment Boundary

Payment processing should not be embedded into the ride state machine unless
necessary.

The initial ride lifecycle ends at:

```text
TRIP_COMPLETED
```

Payment may have its own lifecycle:

```text
PENDING
AUTHORIZED
CAPTURED
FAILED
REFUNDED
```

The payment domain should be designed separately.

---

# 37. Rating Boundary

Ratings should also remain outside the ride state machine.

After completion:

```text
TRIP_COMPLETED
    ↓
rating workflow
```

A missing rating must never make the ride incomplete.

---

# 38. Cancellation Policy Boundary

The lifecycle defines where cancellation can occur.

It does not yet define every financial consequence.

For example:

```text
cancel before driver confirmation
cancel after driver confirmation
cancel after driver arrival
cancel after trip start
```

may have different fees and compensation.

Those policies belong in a separate cancellation/pricing design.

---

# 39. Observability

Useful metrics:

```text
rides_created_total
rides_cancelled_total
rides_completed_total
rides_no_driver_found_total
rides_assignment_failed_total
ride_state_transition_total
ride_state_transition_failures_total
ride_lifecycle_duration
```

Useful dimensions:

```text
service_type
city/region
```

Avoid high-cardinality metric labels such as:

```text
ride_id
driver_id
rider_id
```

Use logs and traces instead.

---

# 40. Complete State Diagram

```text
                         ┌──────────────┐
                         │   REQUESTED  │
                         └──────┬───────┘
                                │
                                ▼
                         ┌──────────────┐
                         │  DISCOVERY   │
                         └──────┬───────┘
                                │
                  ┌─────────────┴─────────────┐
                  │                           │
          candidates found              no candidates
                  │                           │
                  ▼                           ▼
           ┌──────────────┐           ┌────────────────┐
           │   BIDDING    │           │ NO_DRIVER_FOUND│
           └──────┬───────┘           └────────────────┘
                  │
             bid selected
                  │
                  ▼
     ┌──────────────────────────────┐
     │ DRIVER_CONFIRMATION_REQUIRED │
     └──────────────┬───────────────┘
                    │
             ┌──────┼────────┐
             │      │        │
             ▼      ▼        ▼
         confirmed rejected expired
             │      │        │
             ▼      └────┬───┘
     ┌────────────────┐  │
     │DRIVER_CONFIRMED│  │
     └───────┬────────┘  │
             │           │
             ▼           │
     ┌────────────────┐  │
     │DRIVER_ARRIVED  │  │
     └───────┬────────┘  │
             │           │
             ▼           │
     ┌────────────────┐  │
     │  TRIP_STARTED  │  │
     └───────┬────────┘  │
             │           │
             ▼           │
     ┌────────────────┐  │
     │ TRIP_COMPLETED │  │
     └────────────────┘  │
                         │
                         ▼
                 fallback selection

Cancellation may branch from the states where policy permits:

REQUESTED
DISCOVERY
BIDDING
DRIVER_CONFIRMATION_REQUIRED
DRIVER_CONFIRMED
DRIVER_ARRIVED
TRIP_STARTED
        │
        ▼
   CANCELLED

TRIP_STARTED may alternatively end in:

TRIP_INTERRUPTED
```

---

# 41. What We Should Not Build Yet

Do not build:

```text
Payment state inside the ride state machine
Rating state inside the ride state machine
Complex cancellation fee logic
Automatic state changes based only on client events
Client-controlled arbitrary state updates
Distributed state machines across Redis and PostgreSQL
Separate timers for every ride state
```

Keep the ride lifecycle authoritative, transactional, and understandable.

---

# 42. Design Principles

1. Every ride has one authoritative business state.
2. PostgreSQL is the source of truth for ride state.
3. Clients send commands, not arbitrary state mutations.
4. Every state transition must be validated by the backend.
5. Invalid state transitions must be rejected.
6. Selection and reservation must succeed together before confirmation-required state is entered.
7. Driver confirmation creates the committed assignment.
8. Temporary operational failures do not automatically change business state.
9. State-changing commands must tolerate mobile retries.
10. Concurrency-sensitive transitions must be transactional.
11. Ride state should have a monotonically increasing revision.
12. Lifecycle events are emitted only after successful state persistence.
13. WebSocket is a delivery mechanism, not the source of truth.
14. REST provides state recovery after connection loss.
15. Ride state must survive Go and Redis failures.
16. Payment and rating have independent lifecycles.
17. Cancellation rules are state-dependent.
18. Historical transitions should remain auditable.
19. The lifecycle should remain small enough to reason about.
20. New states should be introduced only when they represent a genuinely distinct business condition.
