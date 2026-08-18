# Driver Bidding

## 1. Purpose

This document defines the bidding process between eligible drivers and a
rider requesting a ride.

The bidding system answers:

> Which eligible drivers want to perform this ride, and under what offer?

Bidding is intentionally separated from:

- Driver eligibility
- Driver discovery
- Rider selection
- Driver reservation
- Final assignment

The overall flow is:

```text
Ride Request
    ↓
BIDDING
    ↓
Driver Discovery
    ↓
Drivers Receive Opportunity
    ↓
Drivers Submit Bids
    ↓
Rider Reviews Bids
    ↓
Rider Selects Bid
    ↓
Driver Reservation
    ↓
Driver Confirmation
    ↓
Assignment
```

---

# 2. Core Principle

A bid is an offer.

It is not:

```text
an assignment
a reservation
a guarantee
```

Therefore:

```text
Driver submits bid
        ↓
Driver is offering to perform the ride
        ↓
Rider may select the bid
        ↓
Driver becomes temporarily reserved
        ↓
Driver confirms
        ↓
Assignment becomes committed
```

---

# 3. Why Use Bidding?

The system deliberately uses driver bidding instead of automatically selecting
the nearest driver.

This gives the rider a choice among participating drivers.

For example:

```text
Driver A
ETA: 5 min
Bid: PKR 1,200

Driver B
ETA: 7 min
Bid: PKR 1,000

Driver C
ETA: 4 min
Bid: PKR 1,350
```

The rider can select the offer that best fits their preferences.

The exact information shown to the rider is a product/UI decision.

---

# 4. Bidding Is Not a Continuous Auction

The initial system should not implement:

```text
Driver A → PKR 1,200
Driver B → PKR 1,100
Driver A → PKR 1,050
Driver C → PKR 1,000
...
```

with drivers continuously seeing and responding to competing offers.

That creates unnecessary complexity:

- More events
- More database writes
- More race conditions
- More driver notification traffic
- More complicated UI
- Potentially poor driver experience

Instead:

```text
Driver
   ↓
submit offer
   ↓
bid becomes active
```

A driver may update their own bid while bidding remains open.

---

# 5. Bid Lifecycle

Initial bid lifecycle:

```text
NONE
  ↓
ACTIVE
  ↓
 ┌───────────────┬──────────────┐
 ↓               ↓              ↓
UPDATED       WITHDRAWN      SELECTED
                                  ↓
                              ACCEPTED
```

There are also terminal states caused by the ride lifecycle:

```text
ACTIVE
  ↓
EXPIRED
CANCELLED
NO_LONGER_SELECTABLE
```

The exact database status vocabulary will be finalized during schema design.

---

# 6. Recommended Simplification

For the first implementation, a bid should represent the driver's current
offer.

If the driver changes:

```text
PKR 1,200
```

to:

```text
PKR 1,100
```

we do not need to create a completely independent competing bid.

Instead:

```text
Bid
 ├── amount = 1100
 └── revision = 2
```

The revision can be used for optimistic concurrency and auditing.

If detailed bid history becomes important later, bid revisions can be stored
separately.

---

# 7. Bid Ownership

A bid belongs to:

```text
one ride
one driver
one active driver vehicle
```

Conceptually:

```text
Ride A
   │
   ├── Bid 1 → Driver X
   ├── Bid 2 → Driver Y
   └── Bid 3 → Driver Z
```

A driver must not submit a bid on behalf of another driver.

The authenticated identity determines the driver.

---

# 8. One Active Bid Per Driver Per Ride

The initial system should enforce:

```text
One driver
    +
One ride
    ↓
At most one active bid
```

Therefore this should not exist:

```text
Ride A
   ├── Driver X → Bid 1
   └── Driver X → Bid 2
```

Instead, Driver X updates Bid 1.

This keeps rider selection simple.

---

# 9. Bid Creation

The driver submits a bid through an authenticated API.

Conceptually:

```text
POST /api/v1/rides/{ride_id}/bids
```

The backend validates:

```text
driver identity
driver eligibility
ride ownership/state
ride is accepting bids
bidding deadline
driver has an eligible vehicle
driver has access to this opportunity
```

Only then is the bid created.

---

# 10. Bid Amount

The bid should contain an explicit monetary amount.

Conceptually:

```text
amount
currency
```

Example:

```text
amount   = 1200
currency = PKR
```

The backend must never use floating-point arithmetic for monetary values.

The database representation should use an appropriate exact monetary/decimal
representation.

The exact schema will be finalized later.

---

# 11. Currency

The ride should have a defined currency.

A bid must use the ride's currency.

For example:

```text
Ride:
PKR

Bid:
PKR 1,200
```

A driver must not be able to submit:

```text
USD 1,200
```

for a PKR ride.

Currency conversion is outside the initial bidding domain.

---

# 12. Bid Validation

A bid must satisfy product-defined limits.

Examples:

```text
minimum bid
maximum bid
allowed increment
```

However, these values should be configuration rather than hard-coded business
logic.

Example:

```text
minimum = PKR 500
maximum = PKR 10,000
```

The exact limits remain a product decision.

---

# 13. Zero and Negative Bids

Invalid:

```text
amount = 0
amount < 0
```

The API must reject them.

---

# 14. Bid Precision

The API should define the allowed monetary precision.

For PKR, for example, the initial product may operate using whole rupees.

If fractional currency becomes necessary later, the domain should support it
explicitly rather than relying on floating-point values.

---

# 15. Bid Deadline

Bids are accepted only while:

```text
ride.status = BIDDING
```

and:

```text
current database time < bidding_ends_at
```

The backend is authoritative.

The client countdown is informational only.

---

# 16. Late Bid

Consider:

```text
bidding_ends_at = 10:00:30
```

Driver sends a bid at:

```text
10:00:30.500
```

The backend must reject it.

The request's arrival time at the authoritative backend/database determines
whether the operation is valid.

---

# 17. Bid Submission Race

A bid can race with the bidding deadline.

Example:

```text
Driver submits bid
        +
Ride reaches deadline
```

The backend must ensure that a bid cannot become active after bidding has
closed.

This should be enforced transactionally.

---

# 18. Bid Update

A driver may update their active bid while bidding remains open.

Conceptually:

```text
PATCH /api/v1/rides/{ride_id}/bids/{bid_id}
```

Example:

```text
Original:
PKR 1,200

Updated:
PKR 1,100
```

The backend revalidates the operation.

---

# 19. Bid Revision

Every update should advance a revision/version.

Example:

```text
Bid:
revision = 1
amount = PKR 1,200
```

After update:

```text
revision = 2
amount = PKR 1,100
```

This allows the system to detect stale updates.

---

# 20. Optimistic Concurrency

Consider two requests:

```text
Request A:
revision = 1
amount = 1100

Request B:
revision = 1
amount = 1000
```

Only one should successfully update revision 1 → 2.

The second request should receive a conflict/stale-version response.

The client can then refresh the current bid.

This prevents silent lost updates.

---

# 21. Bid Withdrawal

A driver may withdraw their active bid while the ride is still accepting
withdrawals.

Conceptually:

```text
POST /api/v1/rides/{ride_id}/bids/{bid_id}/withdraw
```

The bid becomes:

```text
WITHDRAWN
```

A withdrawn bid cannot be selected.

---

# 22. Withdrawal and Eligibility

A driver who becomes offline or otherwise ineligible should generally still be
able to withdraw an existing bid.

For example:

```text
Driver submits bid
      ↓
Driver loses connection
      ↓
Driver reconnects
      ↓
Driver withdraws bid
```

The system should not reject the withdrawal simply because the driver is no
longer eligible to create a new bid.

---

# 23. Withdrawal and Selection Race

A critical race exists:

```text
Driver withdraws bid
        +
Rider selects same bid
```

Only one valid state transition should win.

The selection transaction must verify that the bid is still selectable.

The withdrawal transaction must verify that the bid is still withdrawable.

PostgreSQL transaction semantics determine the winner.

The losing operation receives a controlled conflict/invalid-state response.

---

# 24. Rider Selection

The rider selects one active bid.

Conceptually:

```text
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

This operation belongs to the assignment domain.

The bidding system provides the candidate bid; assignment creates the
reservation.

Therefore:

```text
Bid selected
    ↓
Assignment reservation
```

---

# 25. Selection Closes Bidding

Once a rider successfully selects a bid:

```text
BIDDING
    ↓
DRIVER_SELECTED
```

No new bids should be accepted.

Existing competing bids become non-selectable.

For example:

```text
Bid A → SELECTED
Bid B → CLOSED
Bid C → CLOSED
```

This prevents the rider from selecting multiple drivers for the same ride.

---

# 26. Why Bidding Must Close

Without closing bidding:

```text
Rider selects Driver A
        ↓
Driver A confirmation pending
        ↓
Rider selects Driver B
```

The system now has two assignment paths competing for the same ride.

That is unnecessary complexity.

The initial system should have exactly one active assignment path at a time.

---

# 27. Failed Reservation

There is an important exception.

Suppose the rider selects Bid A, but reservation creation fails because Driver A
was concurrently reserved by another ride.

The ride should not become permanently stuck.

The transaction must fail cleanly.

The backend may then:

```text
return ride to BIDDING
```

or:

```text
select another valid bid
```

according to the assignment fallback policy.

The client should never be responsible for repairing the state.

---

# 28. Multiple Bids

A ride may receive:

```text
0 bids
1 bid
5 bids
20 bids
```

The system should support multiple bids.

There should be a configurable upper bound if operational testing shows that
unbounded bids create excessive load.

Do not impose an arbitrary small limit before we have evidence that one is
needed.

---

# 29. Zero Bids

If bidding ends with:

```text
0 valid bids
```

the ride cannot be assigned.

The ride lifecycle should transition to the appropriate no-driver outcome.

For example:

```text
BIDDING
   ↓
BIDDING_TIMEOUT
   ↓
NO_DRIVER_FOUND
```

The exact ride state vocabulary will be defined in the ride lifecycle
document.

---

# 30. Bid Ranking

The rider should receive bids in a useful order.

Initial ranking can use:

```text
ETA
bid amount
driver distance
bid creation/update time
```

However, the ranking must not silently determine the winner.

The rider chooses.

Therefore:

```text
ranking
    ≠
selection
```

---

# 31. ETA

ETA should not be calculated solely from geographic distance.

We already distinguish:

```text
Redis GEO
    ↓
nearby candidates
```

from:

```text
Google Maps / Routes
    ↓
driving ETA
```

The initial system may use geographic distance as a fallback.

Google Maps/Routes should be used when richer ETA information is actually
needed.

Do not make an expensive external routing request for every possible bid
unless the product requires it.

---

# 32. Driver Information

A bid response may expose information such as:

```text
driver display name
driver rating
vehicle information
vehicle category
ETA
bid amount
```

Only information explicitly allowed by the product/privacy model should be
returned.

The driver should never receive private information about competing drivers.

---

# 33. Bid Visibility

Driver sees:

```text
their own bid
```

Rider sees:

```text
bids for their ride
```

A driver should not automatically see:

```text
other drivers' bid amounts
other drivers' private information
```

This is important because otherwise the bidding system becomes a live
price-war mechanism.

---

# 34. No Bid-to-Bid Visibility

The initial product should not expose:

```text
Driver B bid = PKR 1,000
```

to Driver A.

Drivers only know:

```text
their own bid
whether their bid is active
whether they have been selected
whether the opportunity is closed
```

This keeps the model simple.

---

# 35. Driver Bid Status

The driver should be able to determine:

```text
ACTIVE
WITHDRAWN
SELECTED
CLOSED
EXPIRED
```

through authoritative server responses.

WebSocket events can provide real-time updates.

REST remains available for recovery after connection loss.

---

# 36. Bid Notifications

Useful driver events:

```text
ride.bidding_started
bid.created
bid.updated
bid.withdrawn
bid.selected
bid.closed
```

However, the driver does not need a notification for every competing driver's
bid.

The driver only needs information relevant to their own bid and the ride
opportunity.

---

# 37. Rider Bid Updates

The rider may receive:

```text
new bid
bid updated
bid withdrawn
bidding closed
```

through WebSocket.

Example:

```text
Ride
  ↓
BIDDING
  ↓
Driver A submits
  ↓
Rider UI updates

Driver B submits
  ↓
Rider UI updates

Driver A updates
  ↓
Rider UI updates
```

---

# 38. WebSocket Is Not the Source of Truth

If a rider misses:

```text
bid.created
```

because of a network problem, the ride should not become inconsistent.

The rider can retrieve current bids through REST.

Conceptually:

```text
GET /api/v1/rides/{ride_id}/bids
```

WebSocket provides low-latency updates.

REST provides state recovery.

---

# 39. Bid Event Ordering

Events may arrive late or out of order.

For example:

```text
bid.updated
```

may arrive before:

```text
bid.created
```

due to reconnects or message delivery behavior.

Events should therefore include enough metadata to detect stale state.

A revision number is useful:

```text
bid_id
revision
```

The client should ignore an event with an older revision than the state it
already has.

---

# 40. Bid Idempotency

Bid creation should tolerate mobile retries.

Example:

```text
Driver submits bid
      ↓
server creates bid
      ↓
network response lost
      ↓
client retries
```

The system must not create two active bids.

An idempotency key should be supported for bid creation.

The exact API contract will be finalized during API design.

---

# 41. Bid Update Idempotency

Bid updates should use:

```text
bid revision
```

or an equivalent concurrency mechanism.

A repeated update should not accidentally apply twice.

Example:

```text
revision 3
amount 1100
```

A stale request targeting revision 2 should fail rather than overwrite revision
3.

---

# 42. Bid Amount Changes

A driver may update their bid amount while:

```text
ride.status = BIDDING
```

and the bid remains active.

Example:

```text
10:00:05
PKR 1,200

10:00:12
PKR 1,150

10:00:20
PKR 1,100
```

The rider sees the current valid offer.

Whether the product should restrict excessive updates can be decided later.

---

# 43. Anti-Abuse Considerations

Bidding can be abused through:

```text
rapid bid updates
spam bid creation
fake GPS
automated clients
extreme bid values
```

The initial system should implement basic protections:

```text
authentication
authorization
rate limiting
input validation
server-side eligibility checks
bid amount limits
```

A sophisticated fraud system is deferred.

---

# 44. Bid Expiration

Bids are inherently tied to the ride's bidding lifecycle.

When:

```text
bidding_ends_at
```

is reached, active bids become non-selectable.

The system does not need a separate timer per bid.

The ride's authoritative bidding deadline controls them.

This avoids unnecessary timer infrastructure.

---

# 45. Bid History

The system should preserve enough information to answer:

```text
Who bid?
When?
For how much?
What happened to the bid?
Was it selected?
Was it withdrawn?
Was it superseded?
```

This is important for:

- Support
- Disputes
- Analytics
- Debugging
- Future pricing analysis

The exact history schema is a database design concern.

---

# 46. Selected Bid

When the rider selects a bid:

```text
Bid
   ↓
SELECTED
```

The bid should reference the resulting reservation/assignment where appropriate.

This gives us a traceable chain:

```text
Ride
 ↓
Bid
 ↓
Reservation
 ↓
Assignment
```

---

# 47. Selected Bid Is Not Yet Confirmed

The bid being selected means:

```text
rider chose this driver's offer
```

It does not mean:

```text
driver accepted
```

Therefore the UI should represent:

```text
Waiting for driver confirmation
```

rather than:

```text
Driver assigned
```

until the confirmation transaction succeeds.

---

# 48. Driver Confirmation Failure

If the driver:

```text
rejects
```

or:

```text
times out
```

the selected bid becomes unsuccessful.

The ride may then:

```text
return to bidding
```

or:

```text
move to another valid bid
```

according to the assignment fallback policy.

The original bid should remain historically visible as unsuccessful.

---

# 49. Bid Cancellation After Selection

Once the rider has selected a bid:

```text
DRIVER_CONFIRMATION_REQUIRED
```

the driver cannot simply update the bid amount.

The offer has moved into the reservation/assignment process.

At this point the allowed operations are limited to:

```text
confirm
reject
timeout
backend cancellation
```

---

# 50. State Machine

The recommended initial state machine is:

```text
                     ┌─────────────┐
                     │   BIDDING   │
                     └──────┬──────┘
                            │
                  ┌─────────┼─────────┐
                  │         │         │
                  ▼         ▼         ▼
               CREATE     UPDATE   WITHDRAW
                  │
                  ▼
                ACTIVE
                  │
          ┌───────┼────────┐
          │       │        │
          ▼       ▼        ▼
       UPDATE  WITHDRAW  SELECTED
                             │
                             ▼
                         RESERVATION
```

After selection, the bid lifecycle is controlled by assignment.

---

# 51. Bidding and Assignment Boundary

The responsibility boundary should be:

```text
Bidding domain
    ↓
Maintain valid driver offers

Assignment domain
    ↓
Turn one selected offer into a driver reservation
```

This prevents bidding logic from becoming responsible for concurrency around
driver assignment.

---

# 52. Database Invariants

The eventual database design should enforce at least:

```text
one active bid per driver per ride
bid belongs to the specified ride
bid belongs to the specified driver
bid references a valid vehicle
bid amount is valid
bid cannot be selected after closure
```

Application logic handles dynamic business rules.

Database constraints protect structural invariants.

---

# 53. Transaction Boundaries

Bid creation:

```text
BEGIN
  validate ride
  validate driver
  validate deadline
  create/update bid
COMMIT
```

Bid withdrawal:

```text
BEGIN
  lock/revalidate bid
  validate ride state
  withdraw bid
COMMIT
```

Bid selection:

```text
BEGIN
  lock/revalidate ride
  lock/revalidate bid
  close bidding
  create reservation
  create outbox event
COMMIT
```

The final selection transaction belongs to the assignment design.

---

# 54. Failure Handling

If bid creation fails:

```text
no partial bid
```

If bid update fails:

```text
previous valid bid remains intact
```

If withdrawal fails:

```text
previous bid state remains intact
```

If selection fails:

```text
ride must remain in a valid recoverable state
```

No operation should leave the ride in an impossible intermediate state.

---

# 55. Observability

Useful metrics:

```text
bids_created_total
bids_updated_total
bids_withdrawn_total
bids_selected_total
bid_creation_failures_total
bid_update_conflicts_total
bid_submission_latency
bids_per_ride
time_to_first_bid
time_to_first_selection
```

Useful dimensions:

```text
service_type
city/region
```

Avoid putting:

```text
ride_id
driver_id
bid_id
```

into metric labels because of high cardinality.

Use logs/traces instead.

---

# 56. Example End-to-End Flow

```text
Rider requests ride
        ↓
Ride enters BIDDING
        ↓
Redis discovers nearby candidates
        ↓
Driver A receives opportunity
Driver B receives opportunity
Driver C receives opportunity
        ↓
Driver A bids PKR 1,200
Driver B bids PKR 1,000
Driver C bids PKR 1,350
        ↓
Rider receives bids
        ↓
Rider selects Driver B
        ↓
Bidding closes
        ↓
Assignment creates reservation
        ↓
Driver B receives confirmation request
        ↓
Driver B confirms
        ↓
Assignment committed
```

---

# 57. Example Failure Flow

```text
Rider selects Driver B
        ↓
Reservation attempt
        ↓
Driver B already reserved by another ride
        ↓
Reservation conflict
        ↓
Selection fails
        ↓
Ride remains recoverable
        ↓
Fallback selection / bidding
```

The system must not produce:

```text
Ride = assigned to Driver B
```

when the reservation transaction failed.

---

# 58. What We Should Not Build Yet

Do not build:

```text
Continuous driver-to-driver price wars
Real-time bid leaderboard for drivers
Automatic winner selection
Machine-learning bid ranking
Dynamic auction algorithms
Complex bid negotiation
Currency conversion
Sophisticated fraud scoring
```

First prove the basic flow:

```text
discover
→ bid
→ rider selects
→ reserve
→ driver confirms
```

---

# 59. Design Principles

1. A bid is an offer, not an assignment.
2. A driver may have at most one active bid per ride.
3. Bid creation is authenticated and server-authorized.
4. Bid creation revalidates driver eligibility.
5. Bid creation revalidates ride state and deadline.
6. Bid updates are revision-controlled.
7. Stale bid updates must fail rather than overwrite newer state.
8. Drivers may withdraw active bids subject to ride state.
9. Withdrawal does not require the driver to remain eligible for new bids.
10. Bidding closes when the rider successfully selects a bid.
11. Other bids become non-selectable after selection.
12. Bid selection creates a reservation; it does not create final assignment.
13. Driver confirmation creates final commitment.
14. Drivers do not see competing drivers' bid amounts.
15. Riders see only bids belonging to their ride.
16. WebSocket provides real-time updates; REST provides state recovery.
17. Events need revision/version information for stale-event handling.
18. Bid creation must tolerate mobile retries.
19. The backend is authoritative for all bid state.
20. PostgreSQL protects structural invariants.
21. The bidding domain does not own assignment concurrency.
22. Sophisticated auction behavior is explicitly deferred.
