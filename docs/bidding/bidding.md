# Bidding Domain

## 1. Purpose

This document defines the bidding model used instead of automatic driver
selection.

The core flow is:

```text
Ride
  ↓
BIDDING
  ↓
Eligible drivers discover opportunity
  ↓
Drivers submit bids
  ↓
Rider sees bids
  ↓
Rider selects one
  ↓
Driver reserved
  ↓
Driver confirmation
  ↓
Assignment
```

A bid is an offer from a driver for a specific ride. It is not itself a
commitment to perform the ride.

---

# 2. Core Principles

1. A bid belongs to exactly one ride.
2. A bid belongs to exactly one driver.
3. A bid identifies the vehicle being offered.
4. A driver may have at most one active bid for a ride.
5. Submitting a bid does not make the driver unavailable.
6. Reservation creates the conflicting commitment.
7. Rider selection closes the competition for the ride.
8. Selected commercial terms are immutable.
9. Every important operation is revalidated against authoritative state.
10. Discovery data is never an assignment guarantee.

---

# 3. Bid Ownership

A bid is owned by the authenticated driver who created it.

The driver identity must come from the authenticated application identity.

The client cannot submit a bid on behalf of another driver by supplying an
arbitrary `driver_id`.

Conceptually:

```text
OIDC identity
    ↓
application user
    ↓
driver
    ↓
bid
```

---

# 4. Ride Association

Every bid belongs to exactly one ride.

```text
Ride A
 ├── Bid 1
 ├── Bid 2
 └── Bid 3
```

A bid cannot be moved to another ride.

The ride association is immutable.

---

# 5. Driver/Vehicle Association

A bid identifies both:

```text
driver_id
vehicle_id
```

The vehicle must be the driver's active vehicle at bid creation time.

This means the rider is evaluating an actual driver/vehicle offer rather than a
generic driver.

---

# 6. One Active Bid Per Driver Per Ride

The initial rule is:

```text
one active bid
per driver
per ride
```

If a driver wants to change their offer, they update the existing bid.

They do not create another active bid for the same ride.

This avoids confusing the rider with multiple simultaneous offers from the
same driver.

---

# 7. Bid Lifecycle

The initial conceptual lifecycle is:

```text
             ┌──────────────┐
             │    ACTIVE    │
             └──────┬───────┘
                    │
          ┌─────────┼─────────┐
          │         │         │
       update    withdraw   expire
          │         │         │
          ▼         ▼         ▼
       ACTIVE    WITHDRAWN  EXPIRED
                    
ACTIVE
   │
   │ rider selects
   ▼
SELECTED
   │
   │ reservation created
   ▼
RESERVED
   │
   │ driver confirms
   ▼
ACCEPTED
```

Some of these states may be represented through reservation/assignment state
rather than persisted as independent bid statuses.

The persistence model will be finalized during database design.

---

# 8. Active Bid

An active bid is an offer that:

```text
belongs to the ride
belongs to the driver
has not expired
has not been withdrawn
has not been invalidated
has not been consumed by selection
```

Only active bids are normally shown as selectable rider offers.

---

# 9. Bid Creation

Conceptually:

```http
POST /api/v1/rides/{ride_id}/bids
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
Content-Type: application/json
```

Example request:

```json
{
  "amount": "1200",
  "currency": "PKR"
}
```

The driver, ride, and vehicle identities come from authenticated/backend state
rather than arbitrary client-supplied identifiers.

---

# 10. Bid Creation Validation

The backend validates:

```text
authenticated driver
ride exists
ride is accepting bids
driver is eligible
driver is available
active vehicle exists
vehicle is eligible
vehicle satisfies ride requirements
driver has no active bid for this ride
amount is valid
currency matches ride currency
```

The exact pricing constraints are defined by the pricing/product design.

---

# 11. Bid Creation and Availability

Submitting a bid does not reserve the driver.

Therefore:

```text
ONLINE_AVAILABLE
      ↓
submit bid
      ↓
ONLINE_AVAILABLE
```

The driver can potentially bid on other eligible rides according to dispatch
policy.

The bid itself does not create a commitment.

---

# 12. Bid Creation and Eligibility

Eligibility must be evaluated for the specific ride.

For example:

```text
Driver eligible
+
Vehicle eligible
+
Vehicle capacity sufficient
+
Service category matches
```

must all pass.

A driver who is eligible for one ride is not automatically eligible for every
ride.

---

# 13. Bid Amount

The bid represents the driver's offered price.

Example:

```text
amount = 1200
currency = PKR
```

The system should preserve the exact amount offered.

Floating-point arithmetic must not be used for business money calculations.

---

# 14. Reference Fare

A platform-estimated/reference fare is different from a driver bid.

```text
Reference fare
    ↓
platform estimate

Driver bid
    ↓
driver's offer
```

The rider may use the reference fare when evaluating bids, but selecting a bid
creates the commercial commitment based on that bid's terms.

The complete pricing model is a separate domain.

---

# 15. Currency

A bid must use the ride's currency.

Example:

```text
Ride currency = PKR
Bid currency  = PKR
```

A bid in another currency should be rejected unless a future multi-currency
pricing design explicitly supports it.

---

# 16. Bid Update

A driver may update an active bid.

Conceptually:

```http
PATCH /api/v1/rides/{ride_id}/bids/{bid_id}
Authorization: Bearer <driver-token>
```

Example:

```json
{
  "amount": "1150",
  "revision": 2
}
```

The exact update fields will be finalized with the API contract.

---

# 17. Bid Update Restrictions

A driver cannot update:

```text
ride_id
driver_id
vehicle_id
currency
selected/reserved state
```

through a normal bid update.

If the driver changes vehicle, the old bid must not silently become a bid for
the new vehicle.

---

# 18. Vehicle Change and Bid

Suppose:

```text
Bid → Vehicle A
```

and the driver switches to:

```text
Vehicle B
```

The initial policy is:

```text
invalidate old bid
create new bid if still desired
```

Do not silently mutate the old bid from Vehicle A to Vehicle B.

This preserves what the rider actually evaluated.

---

# 19. Bid Revision

Bid updates should use optimistic concurrency where appropriate.

Example:

```text
Bid revision = 4
```

A client updates revision 4.

If another update already changed the bid to revision 5, the stale request
fails with:

```text
409 Conflict
```

rather than overwriting the newer value.

---

# 20. Bid Withdrawal

A driver may withdraw an active bid.

Conceptually:

```http
POST /api/v1/rides/{ride_id}/bids/{bid_id}/withdraw
Authorization: Bearer <driver-token>
Idempotency-Key: <unique-key>
```

The result is:

```text
ACTIVE
  ↓
WITHDRAWN
```

---

# 21. Withdrawal Restrictions

A driver cannot normally withdraw a bid after it has been selected and a
reservation has been created.

At that point the operation belongs to the reservation/assignment lifecycle.

This prevents a normal bid endpoint from bypassing reservation rules.

---

# 22. Bid Expiration

Bids may have an expiration time.

Conceptually:

```text
ACTIVE
   ↓
expires_at reached
   ↓
EXPIRED
```

The exact expiration duration is configuration/product policy.

The backend must evaluate expiration using authoritative server time.

---

# 23. Expired Bid Visibility

An expired bid should not remain selectable.

The client may retain an old copy in memory, but selection must revalidate:

```text
bid state
expires_at
ride state
```

before creating a reservation.

---

# 24. Bid Selection

The rider selects a bid through:

```http
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
Authorization: Bearer <rider-token>
Idempotency-Key: <unique-key>
```

Only the authorized rider for the ride may perform this operation.

---

# 25. Selection Is a Transactional Boundary

Bid selection should conceptually execute:

```text
BEGIN

authorize rider
validate ride state = BIDDING
validate bid belongs to ride
validate bid = ACTIVE
validate bid not expired
revalidate driver
revalidate vehicle
revalidate driver availability
create reservation
close competing bids
transition ride → DRIVER_CONFIRMATION_REQUIRED
create outbox events

COMMIT
```

The exact PostgreSQL locking/constraint strategy will be defined later.

---

# 26. Selection Closes Bidding

Once a bid is selected:

```text
BIDDING
   ↓
selected bid reserved
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

Other bids become non-selectable.

The system must not leave multiple drivers simultaneously selectable for the
same ride after selection succeeds.

---

# 27. Selection Does Not Immediately Assign

The selected bid creates a reservation first:

```text
Rider selects bid
       ↓
Reservation
       ↓
Driver confirmation
       ↓
Assignment
```

This gives the driver a bounded opportunity to confirm.

Rider selection alone does not force an assignment.

---

# 28. Reservation Association

The reservation should preserve:

```text
ride
bid
 driver
vehicle
agreed amount
currency
expiration
```

The selected commercial terms must not change underneath the reservation.

---

# 29. Driver Confirmation

After selection:

```text
DRIVER_CONFIRMATION_REQUIRED
```

The driver may confirm or reject according to the reservation contract.

Successful confirmation produces:

```text
RESERVED
   ↓
ACCEPTED
```

and the ride proceeds toward assignment according to the ride lifecycle.

---

# 30. Confirmation Timeout

If the driver does not confirm before the reservation expires:

```text
reservation expires
       ↓
release driver commitment
       ↓
selection fails
       ↓
ride follows fallback policy
```

The initial recommendation is to return the ride to bidding when the product
still has time and viable candidates.

The exact fallback policy will be defined by the ride lifecycle.

---

# 31. Driver Rejection

If the selected driver rejects:

```text
reservation released
       ↓
ride follows fallback policy
```

The driver is no longer committed to that reservation.

The bid must not become selectable again merely because the driver rejected the
reservation.

---

# 32. Bid Status After Selection

The selected bid should retain historical information that it was selected.

Other bids should become non-selectable.

The exact persisted status model may be:

```text
selected bid → SELECTED / RESERVED
other bids    → CLOSED / NOT_SELECTED
```

or equivalent derived state.

The database design should avoid unnecessary duplicated lifecycle state.

---

# 33. Competing Bids

Suppose:

```text
Ride A
 ├── Bid 1
 ├── Bid 2
 └── Bid 3
```

The rider selects Bid 2.

The result is:

```text
Bid 2 → selected/reserved
Bid 1 → closed
Bid 3 → closed
```

The system should emit appropriate events so drivers stop treating the ride as
an open opportunity.

---

# 34. Driver Availability Revalidation

A driver may become committed after submitting a bid.

Example:

```text
10:00 Driver available
10:01 Driver submits bid
10:02 Driver reserved elsewhere
10:03 Rider selects original bid
```

The selection must fail if the driver can no longer be reserved.

The existence of the bid does not override current availability.

---

# 35. Vehicle Revalidation

The selected vehicle must still be valid.

Selection must revalidate:

```text
vehicle active
vehicle still associated with driver
vehicle satisfies ride requirements
```

A stale bid must not create an assignment with a different or invalid vehicle.

---

# 36. Driver Eligibility Revalidation

Eligibility may change after bid creation.

Examples:

```text
account suspended
vehicle compliance expires
service authorization removed
```

Selection must revalidate relevant eligibility before reservation.

---

# 37. Bid and Discovery Separation

Discovery produces candidate opportunities.

Bidding produces driver offers.

Reservation produces commitment.

These are distinct stages:

```text
Discovery
   ↓
Bid
   ↓
Reservation
   ↓
Assignment
```

Do not collapse them into one operation.

---

# 38. Discovery Does Not Create Bids

Finding an eligible driver does not automatically create a bid on their behalf.

The driver must explicitly submit the offer.

Conceptually:

```text
Discovery
   ↓
notify driver
   ↓
driver evaluates ride
   ↓
submit bid
```

This preserves the driver's agency and makes the bid an actual offer.

---

# 39. Driver Bid Visibility

A driver should see enough ride information to decide whether to bid.

The system should avoid exposing unnecessary rider data.

The exact driver-facing ride representation is defined by the driver API.

---

# 40. Rider Bid Visibility

The rider sees valid bids for their ride.

Potential information:

```text
driver display information
rating
vehicle information
bid amount
currency
ETA where available
bid expiration
```

Internal candidate scoring and private driver data remain hidden.

---

# 41. Bid Notifications

The rider should receive real-time notification when a new bid becomes
available.

Conceptually:

```text
driver submits bid
      ↓
commit bid
      ↓
outbox event
      ↓
WebSocket event
      ↓
rider UI
```

The WebSocket event is a notification, not the source of truth.

---

# 42. Driver Notifications

When the rider selects another bid, non-selected drivers should receive an
event indicating that the opportunity is closed where appropriate.

When a driver's bid is selected:

```text
bid selected
   ↓
reservation created
   ↓
driver notification
```

The driver then retrieves authoritative reservation state if necessary.

---

# 43. Event Ordering

Bid-related events should carry a ride revision, bid revision, or equivalent
sequence where necessary.

Example:

```text
ride revision 8 → bid received
ride revision 9 → bid selected
```

Clients must be able to detect stale events.

---

# 44. Missed Bid Events

If a rider misses WebSocket events, the client retrieves:

```text
GET /api/v1/rides/{ride_id}
GET /api/v1/rides/{ride_id}/bids
```

and reconciles current state.

The system must not depend on the client receiving every WebSocket event.

---

# 45. Idempotent Bid Creation

Bid creation requires an idempotency key because mobile networks can lose
responses.

Example:

```text
submit bid
   ↓
server commits
   ↓
response lost
   ↓
retry same key
```

The retry must not create a second active bid.

---

# 46. Idempotent Bid Withdrawal

Withdrawal should also support idempotency.

Repeated requests with the same key must resolve to the same logical
withdrawal operation rather than creating multiple transitions or errors that
misrepresent the outcome.

---

# 47. Idempotent Bid Selection

Selection requires idempotency.

A lost response must not result in:

```text
second reservation
second assignment
second lifecycle transition
```

The idempotency record must be tied to the authenticated rider and operation.

---

# 48. Concurrent Bid Updates

Two driver sessions may attempt to update the same bid.

Use revision/concurrency checks:

```text
revision 4
   ↓
update A → revision 5

update B using revision 4
   ↓
409 Conflict
```

The stale client retrieves current state before retrying.

---

# 49. Concurrent Rider Selection

Two rider requests may attempt to select different bids for the same ride.

Example:

```text
Select Bid A
      +
Select Bid B
```

Only one selection may succeed.

The ride state and selection operation must be protected transactionally.

The losing request receives a conflict/state error.

---

# 50. Concurrent Driver Reservation

A driver may be selected for another ride while a rider is selecting their bid.

The reservation transaction must enforce:

```text
one conflicting active reservation/assignment per driver
```

PostgreSQL is authoritative for this invariant.

---

# 51. Redis Role

Redis may support:

```text
fast candidate lookup
presence
location freshness
notification fan-out support
```

Redis must not be the authoritative store for:

```text
bid ownership
bid amount
reservation
assignment
selection correctness
```

---

# 52. PostgreSQL Role

PostgreSQL is authoritative for durable bidding data:

```text
ride association
driver association
vehicle association
bid amount
currency
bid lifecycle information
revision
created/updated timestamps
reservation association where applicable
```

The exact schema will be designed later.

---

# 53. Outbox Events

Important bid state changes should produce durable outbox events in the same
transaction as the state change.

Examples:

```text
bid.created
bid.updated
bid.withdrawn
bid.expired
bid.selected
bid.closed
```

This prevents the following failure:

```text
bid committed
   ↓
server crashes before WebSocket publication
   ↓
rider never learns about the bid
```

The outbox allows event delivery to be retried.

---

# 54. Event Delivery Is At-Least-Once

The event system should assume at-least-once delivery.

Consumers must tolerate duplicate events.

For example:

```text
bid.created
bid.created
```

must not create two bids.

The event is a notification of durable state, not a second command to execute
blindly.

---

# 55. Expiration Processing

Bid expiration may be handled asynchronously.

Conceptually:

```text
expires_at reached
      ↓
expiration worker
      ↓
validate bid still active
      ↓
mark expired
      ↓
outbox event
```

The selection transaction must still check `expires_at` directly.

An expiration worker being delayed must not make an already-expired bid
selectable.

---

# 56. No Automatic Highest-Bid Selection

The initial product does not automatically choose:

```text
highest bid
lowest bid
fastest ETA
```

The rider makes the selection.

Any future automated recommendation should remain separate from the
fundamental selection mechanism.

---

# 57. No Driver-to-Driver Bid Visibility

Drivers should not see competing drivers' prices in the initial system.

They receive the ride opportunity and decide their own offer.

This avoids turning the system into a real-time auction or price war.

---

# 58. No Continuous Auction Rounds

The initial bidding model is one independent offer per driver.

Do not introduce:

```text
multiple bidding rounds
minimum increments
automatic counter-bids
real-time auction timers
```

unless the product later demonstrates a real need for them.

---

# 59. API Surface

Initial bidding endpoints:

```text
POST   /api/v1/rides/{ride_id}/bids
GET    /api/v1/rides/{ride_id}/bids
PATCH  /api/v1/rides/{ride_id}/bids/{bid_id}
POST   /api/v1/rides/{ride_id}/bids/{bid_id}/withdraw
POST   /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

The final API schemas are subject to the common conventions in
`docs/api/api-conventions.md`.

---

# 60. Bid Creation Response

A successful bid creation should return the authoritative bid representation.

Conceptually:

```json
{
  "data": {
    "id": "bid_123",
    "ride_id": "ride_123",
    "amount": "1200",
    "currency": "PKR",
    "status": "ACTIVE",
    "revision": 1,
    "created_at": "2026-08-18T10:30:00Z",
    "expires_at": "2026-08-18T10:32:00Z"
  }
}
```

The driver and vehicle representation in the response is deliberately omitted
from this conceptual example and will be defined by the driver-facing API.

---

# 61. Bid Update Response

A successful update returns the new authoritative revision and relevant bid
state.

Example:

```json
{
  "data": {
    "id": "bid_123",
    "amount": "1150",
    "currency": "PKR",
    "status": "ACTIVE",
    "revision": 2
  }
}
```

---

# 62. Common Errors

Examples:

```text
RIDE_NOT_BIDDING
BID_NOT_FOUND
BID_EXPIRED
BID_WITHDRAWN
BID_ALREADY_SELECTED
DUPLICATE_ACTIVE_BID
DRIVER_NOT_AVAILABLE
DRIVER_NOT_ELIGIBLE
VEHICLE_NOT_ELIGIBLE
BID_REVISION_CONFLICT
RESERVATION_CONFLICT
```

The API should expose only the stable error semantics needed by clients.

---

# 63. Security and Authorization

Driver operations require:

```text
authenticated driver
```

and the driver must own the bid being modified or withdrawn.

Rider selection requires:

```text
authenticated rider
```

and the rider must own the ride.

No client-provided driver or rider ID may override authenticated identity.

---

# 64. Privacy

Bids can reveal information about both sides of the marketplace.

The API must limit visibility to the information required for the current role.

For example, the rider may see driver/vehicle presentation data, while drivers
should not see private rider information that is unnecessary for evaluating or
performing the ride.

---

# 65. Failure Handling

If PostgreSQL cannot commit a bid:

```text
no success response
no fabricated bid
```

If event publication fails after the bid commits:

```text
bid remains committed
outbox retries publication
```

If Redis fails:

```text
bid persistence remains PostgreSQL-backed
```

Discovery/availability-dependent validation may temporarily degrade according
to the broader dispatch failure policy.

---

# 66. Selection Failure Handling

If selection fails because the driver is no longer available:

```text
no reservation
no assignment
bid remains non-selected according to current state
ride follows bidding/fallback policy
```

The API must not partially create a reservation and then return an error as if
nothing happened.

Transactional integrity is required.

---

# 67. Complete State Flow

```text
                    ┌───────────────┐
                    │     ACTIVE    │
                    └───────┬───────┘
                            │
             ┌──────────────┼──────────────┐
             │              │              │
          update         withdraw        expire
             │              │              │
             ▼              ▼              ▼
          ACTIVE         WITHDRAWN       EXPIRED
             │
             │ rider selects
             ▼
         SELECTED
             │
             │ reservation
             ▼
          RESERVED
             │
             │ driver confirms
             ▼
          ACCEPTED
             │
             │ assignment
             ▼
          ASSIGNED
```

The exact persistent bid status model may collapse some states into
reservation/assignment relationships.

---

# 68. Relationship to Ride Lifecycle

The important ride transition is:

```text
BIDDING
   ↓
rider selects valid bid
   ↓
DRIVER_CONFIRMATION_REQUIRED
```

The bidding domain must not independently assign the driver.

Assignment occurs only after the reservation/confirmation lifecycle succeeds.

---

# 69. What We Should Not Build Yet

Do not build:

```text
automatic highest/lowest bid selection
real-time price auctions
multiple bidding rounds
driver-to-driver bid visibility
complex bidding strategies
algorithmic rider recommendation as a requirement
auction-style price increments
bid negotiation chat
```

The initial product needs a marketplace of independent offers, not a full
auction engine.

---

# 70. Design Principles

1. A bid is an offer, not a driver commitment.
2. A bid belongs to one ride, one driver, and one vehicle.
3. A driver has at most one active bid per ride.
4. Bid creation requires authenticated driver identity.
5. Bid creation validates ride state, driver eligibility, vehicle eligibility, and availability.
6. Bid updates cannot change ride, driver, vehicle, or currency identity.
7. Vehicle changes invalidate the old bid rather than silently mutating it.
8. Bid expiration is based on authoritative server time.
9. Expired bids are never selectable even if the client displays stale data.
10. Rider selection is a transactional operation.
11. Selection revalidates driver and vehicle state.
12. Selection creates a reservation before assignment.
13. Selection closes competing bids.
14. A selected bid's commercial terms are immutable.
15. Driver confirmation and timeout belong to the reservation lifecycle.
16. Discovery, bidding, reservation, and assignment are distinct stages.
17. Redis can accelerate discovery but PostgreSQL owns durable bidding state.
18. Important bid changes use durable outbox events.
19. Event delivery is treated as at-least-once.
20. WebSocket events are notifications; REST remains authoritative.
21. Bid creation, withdrawal, and selection support idempotency where retries can duplicate operations.
22. Concurrent bid updates use revision/concurrency checks.
23. Concurrent rider selections must allow exactly one winner.
24. Driver commitment invariants are enforced transactionally.
25. The initial model is independent offers, not an automated auction engine.
