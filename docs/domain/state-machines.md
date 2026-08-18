# Domain State Machines

## 1. Purpose

This document defines the explicit lifecycle state machines for the core business
entities.

The purpose is to prevent status columns from becoming undocumented collections of
strings.

For every state machine we define:

```text
states
valid transitions
actors
preconditions
side effects
events
terminal states
invalid transitions
concurrency rules
```

The application/domain layer owns transition rules. PostgreSQL constraints provide
a final integrity boundary where practical.

---

# 2. General Transition Rule

A state transition is an application use case:

```text
request
  ↓
authentication
  ↓
authorization
  ↓
load aggregate
  ↓
validate current state
  ↓
validate actor
  ↓
validate domain preconditions
  ↓
transaction
  ├── state change
  ├── history record where required
  └── outbox event
  ↓
commit
```

No client may directly set a target status.

Bad:

```json
{
  "status": "completed"
}
```

The API exposes commands, not arbitrary state mutation.

---

# 3. Ride State Machine

## States

```text
requested
bidding
reserved
assigned
driver_arrived
in_progress
completed
cancelled
```

`completed` and `cancelled` are terminal states.

---

# 4. Ride Transitions

```text
requested
   │
   ├── start_bidding ─────────→ bidding
   └── cancel ────────────────→ cancelled

bidding
   │
   ├── reserve ───────────────→ reserved
   └── cancel ────────────────→ cancelled

reserved
   │
   ├── assign ────────────────→ assigned
   └── cancel ────────────────→ cancelled

assigned
   │
   ├── driver_arrived ────────→ driver_arrived
   └── cancel ────────────────→ cancelled

driver_arrived
   │
   ├── start_trip ────────────→ in_progress
   └── cancel ────────────────→ cancelled where policy permits

in_progress
   │
   └── complete ──────────────→ completed
```

Exact cancellation permissions are defined by the cancellation policy and may vary
by state and actor.

---

# 5. Ride Preconditions

### `requested → bidding`

Requires:

```text
ride exists
ride is owned by rider/requesting application context
pickup/dropoff valid
ride has not expired/cancelled
```

### `bidding → reserved`

Requires:

```text
valid winning bid
bid belongs to ride
bid is selectable
ride is still open for selection
concurrency protection acquired
```

### `reserved → assigned`

Requires:

```text
valid reservation
reservation not expired
selected driver remains eligible
assignment can be created
```

### `assigned → driver_arrived`

Requires:

```text
assignment active
driver authorized for assignment
```

### `driver_arrived → in_progress`

Requires:

```text
assignment active
ride is in driver-arrived state
start command authorized
```

### `in_progress → completed`

Requires:

```text
active assignment
trip completion command authorized
required completion data valid
```

---

# 6. Ride Side Effects

A successful transition may write:

```text
rides.status
lifecycle timestamp
history record
outbox event
```

Additional domain records may be created where the transition requires them.

Example:

```text
select bid
  ↓
reservation
  ↓
ride state update
  ↓
outbox event
```

These writes belong in one transaction where atomicity is required.

---

# 7. Ride Events

Representative events:

```text
ride.created
ride.bidding_started
ride.reserved
ride.assigned
ride.driver_arrived
ride.started
ride.completed
ride.cancelled
```

Events represent facts after the transition commits.

---

# 8. Ride Invalid Transitions

Examples:

```text
completed → bidding
completed → cancelled
cancelled → requested
cancelled → assigned
in_progress → bidding
```

These must be rejected.

A retry of an already completed command should be handled through idempotency where
appropriate rather than treating an invalid state transition as success.

---

# 9. Ride Concurrency

Critical transitions require database concurrency protection.

Examples:

```text
rider selects bid
        ↔
ride is cancelled

bid A selected
        ↔
 bid B selected
```

Only one operation may win when the invariant requires one winner.

Use transactions, row locking, conditional updates, and database constraints as
appropriate.

---

# 10. Bid State Machine

## States

```text
submitted
active
withdrawn
rejected
selected
expired
```

`selected`, `withdrawn`, `rejected`, and `expired` are terminal states for that bid.

If `submitted` and `active` are operationally identical in implementation, they may
be collapsed into one state. The final API/domain contract should avoid redundant
states.

---

# 11. Bid Transitions

```text
submitted
   │
   ├── activate ──────────────→ active
   ├── withdraw ──────────────→ withdrawn
   ├── reject ────────────────→ rejected
   └── expire ────────────────→ expired

active
   │
   ├── select ────────────────→ selected
   ├── withdraw ──────────────→ withdrawn
   ├── reject ────────────────→ rejected
   └── expire ────────────────→ expired
```

---

# 12. Bid Preconditions

Submitting a bid requires:

```text
driver authenticated
driver authorized as driver
driver eligible
driver operationally available
ride is in bidding state
bid amount valid
driver has no conflicting active bid for the ride
```

The database partial unique index protects against duplicate active bids even when
requests race.

---

# 13. Bid Selection

Selecting a bid requires:

```text
rider authorized for ride
ride currently allows selection
bid belongs to ride
bid is active/selectable
bid has not expired
selected driver remains eligible
```

The selection transaction should protect the ride against concurrent selection or
cancellation.

---

# 14. Bid Events

Representative events:

```text
bid.created
bid.activated
bid.withdrawn
bid.rejected
bid.selected
bid.expired
```

Not every internal transition must necessarily be exposed to clients.

---

# 15. Reservation State Machine

## States

```text
pending
confirmed
expired
cancelled
```

`confirmed`, `expired`, and `cancelled` are terminal states.

---

# 16. Reservation Transitions

```text
pending
   │
   ├── confirm ───────────────→ confirmed
   ├── expire ────────────────→ expired
   └── cancel ────────────────→ cancelled
```

The reservation must reference the selected bid and the same ride/driver.

---

# 17. Reservation Preconditions

Creation requires:

```text
bid selected within a valid transaction
ride still eligible for reservation
selected driver valid
no conflicting active reservation
```

Confirmation requires:

```text
reservation pending
not expired
required driver acceptance/confirmation complete
```

---

# 18. Reservation Events

```text
reservation.created
reservation.confirmed
reservation.expired
reservation.cancelled
```

---

# 19. Assignment State Machine

## States

```text
assigned
driver_arrived
in_progress
completed
cancelled
released
```

`completed`, `cancelled`, and `released` are terminal.

---

# 20. Assignment Transitions

```text
assigned
   │
   ├── driver_arrived ────────→ driver_arrived
   ├── start_trip ────────────→ in_progress where policy permits
   ├── cancel ────────────────→ cancelled
   └── release ───────────────→ released

driver_arrived
   │
   ├── start_trip ────────────→ in_progress
   └── cancel ────────────────→ cancelled where policy permits

in_progress
   │
   └── complete ──────────────→ completed
```

The assignment lifecycle must remain consistent with the parent ride lifecycle.

---

# 21. Assignment Preconditions

Creation requires:

```text
reservation valid
ride still assignable
driver eligible
driver not already conflicting with another active assignment
```

Starting requires:

```text
assignment active
ride in compatible state
required operational checks complete
```

Completion requires:

```text
assignment in progress
required trip completion data valid
```

---

# 22. Assignment Events

```text
assignment.created
assignment.driver_arrived
assignment.started
assignment.completed
assignment.cancelled
assignment.released
```

---

# 23. Driver Eligibility State

Eligibility should be treated as a derived domain decision rather than one permanent
boolean.

Relevant durable dimensions include:

```text
account status
verification status
document validity
vehicle validity
policy restrictions
```

A useful conceptual evaluation result is:

```text
eligible
ineligible
```

but the underlying reasons must remain explainable.

---

# 24. Eligibility Transition Inputs

Eligibility can change when:

```text
account activated/suspended
verification approved/revoked
document expires/rejected
vehicle activated/suspended
policy restriction added/removed
```

The resulting decision may change without the driver explicitly issuing a command.

---

# 25. Eligibility Events

Representative events:

```text
driver.eligibility_changed
driver.verification_changed
driver.vehicle_status_changed
```

The exact event granularity will be finalized with the driver eligibility contract.

---

# 26. Operational Availability vs Eligibility

These are intentionally separate.

```text
Eligibility
→ may this driver receive/perform rides?

Availability
→ is this eligible driver currently available?
```

A driver can be:

```text
eligible + offline
eligible + online
ineligible + online
```

Being online does not make an ineligible driver discoverable.

---

# 27. Operational Presence

Current presence is primarily Redis state.

Conceptually:

```text
offline
online
stale
```

Presence is ephemeral and heartbeat-driven.

A driver should not be considered available indefinitely after the last heartbeat.

---

# 28. Payment State Machine

## States

```text
pending
authorized
captured
failed
refunded
partially_refunded
cancelled
```

The exact provider lifecycle may require additional states, but these are the core
business concepts.

---

# 29. Payment Transitions

```text
pending
   │
   ├── authorize ─────────────→ authorized
   ├── fail ──────────────────→ failed
   └── cancel ────────────────→ cancelled

authorized
   │
   ├── capture ───────────────→ captured
   ├── fail ──────────────────→ failed
   └── cancel ────────────────→ cancelled

captured
   │
   ├── refund ────────────────→ refunded
   └── partial_refund ────────→ partially_refunded

partially_refunded
   │
   └── refund_remaining ──────→ refunded
```

Provider-specific intermediate states may be mapped into this domain model.

---

# 30. Payment Preconditions

Authorization/capture/refund operations require:

```text
payment belongs to authorized ride/user context
amount valid
currency valid
payment state permits operation
provider operation is idempotent
```

Do not rely solely on provider response state without recording the local durable
state transition.

---

# 31. Payment Events

```text
payment.authorized
payment.captured
payment.failed
payment.cancelled
payment.refunded
payment.partially_refunded
```

External provider webhook events must be deduplicated.

---

# 32. Payment Concurrency

Payment operations must protect against duplicate commands and provider retries.

Examples:

```text
capture request
     ↔
retry of same capture

provider webhook
     ↔
local payment worker
```

Use durable idempotency and provider event identifiers where available.

---

# 33. Settlement State Machine

## States

```text
pending
calculated
finalized
reversed
```

`finalized` and `reversed` are terminal for the represented settlement version.

---

# 34. Settlement Transitions

```text
pending
   │
   └── calculate ─────────────→ calculated

calculated
   │
   ├── finalize ──────────────→ finalized
   └── reverse ───────────────→ reversed

finalized
   │
   └── reverse ───────────────→ reversed where policy permits
```

Settlement should normally depend on a completed trip and finalized fare/payment
inputs.

---

# 35. Settlement Preconditions

Finalization requires:

```text
ride/trip completed
fare finalized
required payment state known
settlement calculation reproducible
no conflicting finalized settlement
```

The financial record must preserve enough information to explain the historical
calculation.

---

# 36. Settlement Events

```text
settlement.calculated
settlement.finalized
settlement.reversed
```

---

# 37. Payout State Machine

## States

```text
pending
submitted
processing
paid
failed
cancelled
```

---

# 38. Payout Transitions

```text
pending
   │
   ├── submit ────────────────→ submitted
   └── cancel ────────────────→ cancelled

submitted
   │
   ├── process ───────────────→ processing
   ├── fail ──────────────────→ failed
   └── cancel ────────────────→ cancelled where provider/policy permits

processing
   │
   ├── succeed ───────────────→ paid
   └── fail ──────────────────→ failed

failed
   │
   └── retry ─────────────────→ submitted
```

A payout must not be retried as a new financial operation if the provider may have
already executed the original operation. Provider idempotency is mandatory where
supported.

---

# 39. Payout Preconditions

Submission requires:

```text
settlement valid/finalized
payout amount valid
currency valid
driver eligible for payout
no conflicting successful payout
```

---

# 40. Payout Events

```text
payout.submitted
payout.processing
payout.paid
payout.failed
payout.cancelled
```

---

# 41. Cross-Aggregate Transitions

Some business operations affect multiple aggregates.

Example:

```text
select bid
   ↓
Ride state
   +
Bid state
   +
Reservation
   +
Outbox events
```

The application use case coordinates these changes inside the appropriate
transaction.

Do not attempt to solve required atomicity by chaining asynchronous events.

---

# 42. Ride Completion Workflow

A typical completion flow is:

```text
assignment in_progress
        ↓
ride in_progress
        ↓
complete trip
        ↓
ride completed
        ↓
assignment completed
        ↓
outbox events
        ↓
financial workflow
```

The exact ordering of assignment/ride updates must be fixed in the implementation
contract so that every code path follows the same invariant order.

---

# 43. Cancellation Workflow

Cancellation is not simply:

```text
UPDATE rides SET status = 'cancelled'
```

A valid cancellation may require:

```text
authorization
cancellation policy evaluation
assignment/reservation release
cancellation record
ride state update
outbox event
financial consequences where applicable
```

The exact financial consequences belong to the pricing/payment/settlement contracts.

---

# 44. Invalid Transition Policy

Invalid transitions should return a domain-level error.

Examples:

```text
ride already completed
bid already selected
reservation already expired
assignment already completed
payment already captured
payout already paid
```

Do not silently convert invalid commands into unrelated state changes.

---

# 45. Idempotent Commands

Some commands should be idempotent by design.

Examples:

```text
retrying payment capture
retrying webhook processing
retrying notification processing
```

Idempotency means repeating the same logical operation does not create an additional
business effect.

It does not mean every invalid state transition should return success.

---

# 46. Concurrency Rules

Critical state transitions require consistent lock/constraint strategy.

At minimum protect:

```text
bid selection
ride cancellation vs selection
reservation expiration vs confirmation
assignment creation
trip completion
payment capture
payout submission
```

Where multiple rows are locked, use a consistent lock acquisition order to reduce
deadlock risk.

---

# 47. Event Atomicity

Every transition that publishes a business event should follow:

```text
BEGIN
  validate transition
  update current state
  write history
  insert outbox event
COMMIT
```

The event is published only after the transaction commits.

---

# 48. State Machine Testing

For every state machine test:

```text
initial state
valid transition
invalid transition
actor authorization
preconditions
side effects
emitted event
terminal state
concurrent transition
retry/idempotency behavior
```

State transition tables should become executable tests during implementation.

---

# 49. State Machine Summary

```text
Ride
requested → bidding → reserved → assigned → driver_arrived → in_progress → completed
   └────────────────────────────────────────────────────────────────────────→ cancelled

Bid
submitted → active → selected
             ├──────→ withdrawn
             ├──────→ rejected
             └──────→ expired

Reservation
pending → confirmed
   ├──→ expired
   └──→ cancelled

Assignment
assigned → driver_arrived → in_progress → completed
    ├────────────────────────────────────→ cancelled
    └────────────────────────────────────→ released

Payment
pending → authorized → captured → refunded
   ├──────────────→ failed
   └──────────────→ cancelled

Settlement
pending → calculated → finalized
              └──────→ reversed

Payout
pending → submitted → processing → paid
   │          └──────→ failed → submitted
   └──────────────────────────→ cancelled
```

---

# 50. Design Principles

1. Clients issue commands; they do not set arbitrary statuses.
2. Authentication and authorization happen before transition evaluation.
3. Domain preconditions determine whether an authorized command is valid.
4. Current state is stored explicitly.
5. Important history is recorded separately.
6. Terminal states cannot transition back into active lifecycle states.
7. Database constraints protect race-sensitive invariants.
8. Critical transitions occur inside transactions.
9. State changes and outbox events commit atomically.
10. Ride, bid, reservation, assignment, payment, settlement, and payout lifecycles
    have explicit states and transitions.
11. Driver eligibility is derived from multiple durable facts rather than a single
    permanent boolean.
12. Operational availability is distinct from eligibility.
13. Redis presence does not become durable business state.
14. External financial operations require idempotency.
15. Invalid transitions fail explicitly.
16. Retrying a command is not the same as bypassing a state invariant.
17. State machine rules should become executable tests before implementation is
    considered complete.
