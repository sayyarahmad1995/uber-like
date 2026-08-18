# Database Architecture

## 1. Purpose

This document defines durable persistence boundaries, PostgreSQL responsibilities,
Redis responsibilities, aggregate ownership, transactions, constraints, indexing,
and operational data rules.

The central rule is:

> If losing Redis would corrupt business correctness, that data is in the wrong place.

---

# 2. Core Architecture

```text
                    PostgreSQL
                 source of truth
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
     Identity        Rides        Financial
     domain          domain         domain
        │              │              │
        └──────────────┼──────────────┘
                       │
                    Outbox
                       │
                       ▼
                 Event delivery

                     Redis
               ephemeral state
                       │
       ┌───────────────┼───────────────┐
       ▼               ▼               ▼
    Presence        Location        Rate limits
    /sessions       /realtime        /locks
```

PostgreSQL is the authoritative durable store. Redis is operational acceleration
and ephemeral state.

---

# 3. PostgreSQL Responsibilities

PostgreSQL owns durable business records such as:

```text
users
driver_profiles
vehicles
rides
bids
reservations
assignments
cancellations
payments
settlements
payouts
outbox events
```

The exact table set will evolve as the remaining domain documents are finalized.

---

# 4. Redis Responsibilities

Redis may hold:

```text
driver presence
current driver location
WebSocket connection/routing state
short-lived discovery state
deadlines and TTL state
rate-limit counters
short-lived locks
cache entries
```

Redis must not become authoritative for:

```text
ride status
bid history
assignment history
payment state
driver earnings
cancellation history
```

---

# 5. Ride Aggregate

Ride is the central aggregate for lifecycle operations.

Conceptually:

```text
Ride
 ├── lifecycle state
 ├── pickup/dropoff
 ├── rider
 ├── bidding
 ├── reservation
 ├── assignment
 ├── cancellation
 └── trip lifecycle
```

The aggregate boundary describes ownership of lifecycle invariants; it does not
require every related historical record to be stored in one physical row.

---

# 6. Aggregate Boundaries

Avoid one giant aggregate for the entire system.

Conceptual aggregates include:

```text
Ride
Driver
Payment
Settlement
```

Each aggregate owns its own invariants.

For example:

```text
Ride
 → whether bidding is open

Driver
 → whether the driver is operationally eligible

Payment
 → whether a payment operation is valid
```

Application use cases coordinate across aggregates where required.

---

# 7. Users

A user represents application identity.

Conceptually:

```text
users
-----
id
oidc_subject
created_at
updated_at
```

The external OIDC subject should be unique.

The OIDC identity is not the platform's primary business identifier.

---

# 8. Driver Profiles

Driver-specific information belongs separately from generic user identity.

Conceptually:

```text
driver_profiles
---------------
id
user_id
status
created_at
updated_at
```

This separates identity from driver operational/business data.

---

# 9. Vehicles

Vehicles are durable entities.

Conceptually:

```text
vehicles
--------
id
driver_id
vehicle_type
make/model metadata
registration metadata
status
created_at
updated_at
```

Exact fields depend on finalized driver eligibility requirements.

---

# 10. Rides

A ride contains durable information required to reconstruct its lifecycle.

Conceptually:

```text
rides
-----
id
rider_id
status
pickup
destination
pricing reference
selected bid reference where appropriate
timestamps
```

Avoid turning the ride table into a 100-column record containing every historical
detail.

---

# 11. Bids

Bids are durable historical records.

Conceptually:

```text
bids
----
id
ride_id
driver_id
amount
status
created_at
updated_at
```

A bid remains historical after it is rejected, expired, withdrawn, selected, or
associated with a cancelled ride.

---

# 12. Bid History

If multiple bid state transitions need to be recorded explicitly, use an
append-oriented history/event table.

Conceptually:

```text
bid_events
----------
id
bid_id
event_type
occurred_at
metadata
```

This allows reconstruction of important bid history without overloading the
current bid row.

---

# 13. Ride History

The ride's current state belongs on the ride record.

Historical transitions can be recorded separately:

```text
ride_events
-----------
id
ride_id
event_type
occurred_at
metadata
```

This provides both current state and historical timeline.

---

# 14. Reservations

Reservations require durable history.

Conceptually:

```text
reservations
------------
id
ride_id
driver_id
bid_id
status
created_at
expires_at
cancelled_at
```

This is particularly important for the bidding flow:

```text
bid selected
      ↓
reservation
      ↓
confirmation
```

---

# 15. Assignments

Assignments must remain queryable after cancellation.

Conceptually:

```text
assignments
-----------
id
ride_id
driver_id
status
created_at
started_at
ended_at
reason
```

Cancellation terminates/releases the assignment; it does not delete the record.

---

# 16. Active Bid Constraint

A critical invariant is:

```text
one active bid
per driver
per ride
```

This should be enforced at the database level with an appropriate partial unique
index once the exact active statuses are finalized.

---

# 17. Database Constraints

Use PostgreSQL constraints for invariants that can safely be expressed there:

```text
NOT NULL
UNIQUE
FOREIGN KEY
CHECK
partial UNIQUE INDEX
```

Application logic still enforces domain rules, while PostgreSQL provides a final
correctness boundary.

---

# 18. Foreign Keys

Use foreign keys where referential integrity matters.

Examples:

```text
rides.rider_id
    → users.id

bids.ride_id
    → rides.id

bids.driver_id
    → driver_profiles.id
```

Do not remove foreign keys merely to simplify application code.

---

# 19. Deletion Policy

Ride and financial history should generally not be hard deleted.

Examples:

```text
rides
bids
assignments
payments
settlements
cancellations
```

must preserve business history.

Privacy deletion requirements should use an explicit retention/anonymization
policy rather than blindly cascading deletes through business history.

---

# 20. Identifiers

UUID-based identifiers are preferred.

UUIDv7 is the current recommended direction because it retains UUID semantics while
providing useful time-ordered characteristics.

Conceptually:

```text
ride_id = UUIDv7
```

The final identifier implementation should be locked before migrations are built.

---

# 21. Public IDs

Public APIs should use stable opaque identifiers rather than sequential database
IDs by default.

Sequential IDs make enumeration and information leakage easier.

---

# 22. Timestamps

Use PostgreSQL:

```text
TIMESTAMPTZ
```

Store timestamps in UTC.

API responses use RFC 3339/ISO 8601 representations.

Example:

```text
2026-08-18T10:30:00Z
```

---

# 23. Money

Never use PostgreSQL floating-point types for monetary values.

Use integer minor units plus currency, consistently with the pricing/payment
money model.

Conceptually:

```text
amount_minor
currency = PKR
```

The exact minor-unit convention must be consistent throughout the system.

---

# 24. Coordinates

For V1, geographic coordinates can remain explicit latitude/longitude fields.

Conceptually:

```text
latitude
longitude
```

PostGIS is intentionally deferred until actual spatial-query requirements justify
its additional complexity.

---

# 25. Driver Location Storage

Current driver location is operational state and belongs primarily in Redis.

Conceptually:

```text
driver:{id}:location

latitude
longitude
updated_at
```

The exact Redis data structure is an implementation decision.

PostgreSQL should not receive every GPS update.

---

# 26. Durable Location History

If later requirements need:

```text
trip replay
route analysis
dispute investigation
analytics
```

introduce sampled or aggregated durable location history.

Do not prematurely persist every GPS coordinate.

---

# 27. Presence

Driver presence is ephemeral operational state.

Redis may hold:

```text
online/offline
last heartbeat
current operational presence
```

Presence should remain reconstructible from sessions/heartbeats rather than being
considered permanent business history.

---

# 28. Redis Is Not a Second Database

Do not maintain a complete duplicate domain database in Redis.

Bad:

```text
PostgreSQL → durable data
Redis      → second copy of entire domain
```

Better:

```text
PostgreSQL
   ↓
authoritative state

Redis
   ↓
derived/ephemeral acceleration
```

---

# 29. Cache Invalidation

Every cache entry needs an explicit TTL or invalidation strategy.

Do not assume a PostgreSQL write automatically keeps Redis correct.

If stale Redis data could cause an unsafe business decision, revalidate against
PostgreSQL.

---

# 30. Transactions

PostgreSQL transactions protect business invariants.

Examples include:

```text
selecting a bid
creating a reservation
assigning a driver
cancelling a ride
starting a trip
completing a trip
creating a settlement
```

The application use case determines the appropriate transaction boundary.

---

# 31. Concurrent Bid Selection

The database must ensure that concurrent rider selection cannot produce multiple
successful selections.

Conceptually:

```text
BEGIN
lock ride
validate bidding state
validate bid
select bid
create reservation/assignment
update ride
write outbox
COMMIT
```

The exact SQL locking strategy is an implementation detail.

---

# 32. Concurrent Driver Actions

Database constraints and transactions must protect races such as:

```text
Driver A submits duplicate bid
Driver accepts ↔ ride cancelled
Driver accepts ↔ another driver selected
```

Application code alone is not sufficient for these invariants.

---

# 33. Row Locking

Use PostgreSQL row-level locking where required.

Potential candidates:

```text
ride
bid
reservation
assignment
driver operational record
```

Do not lock entire tables for ordinary ride operations.

---

# 34. Transaction Isolation

Start with PostgreSQL's normal transaction isolation and use explicit row locking
and constraints for the invariants that require them.

Do not default to serializable transactions everywhere.

---

# 35. Deadlocks

Multi-row transactions must acquire locks in a consistent order.

For example, a documented order could be:

```text
ride
 ↓
bid
 ↓
driver
```

The exact order can change, but different code paths must not acquire the same
resources in conflicting orders.

---

# 36. Outbox

The outbox belongs in PostgreSQL and is written in the same transaction as the
business state transition.

Conceptually:

```text
BEGIN
  update business state
  insert outbox event
COMMIT
```

Then:

```text
outbox publisher
      ↓
realtime
push
analytics
other consumers
```

---

# 37. Outbox Delivery

Outbox processing uses at-least-once delivery.

Consumers must tolerate duplicate events.

If a consumer fails, the event remains available for retry.

Do not introduce system-wide exactly-once processing without a demonstrated need.

---

# 38. Outbox Event IDs

Outbox records should have stable identifiers.

Conceptually:

```text
event_id
aggregate_id
event_type
payload
created_at
published_at
```

Consumers can use `event_id` for deduplication and observability.

---

# 39. JSONB

JSONB is appropriate for genuinely flexible metadata, especially event payloads.

It should not become the default storage format for core domain fields.

Bad:

```text
rides.data JSONB
```

Better:

```text
rides
  → strongly typed core fields

event.metadata
  → flexible auxiliary information
```

---

# 40. Schema Ownership

Each table should have a clear owning domain/application area.

Examples:

```text
users             → identity
driver_profiles   → driver
vehicles          → driver
rides             → ride
bids              → bidding
payments          → payment
settlements       → settlement
outbox            → event infrastructure
```

Avoid multiple modules independently modifying the same table without a defined
ownership boundary.

---

# 41. Data Access Layer

Go application code should not scatter raw SQL through HTTP handlers and domain
logic.

A reasonable boundary is:

```text
HTTP
 ↓
Application
 ↓
Repository/data access
 ↓
PostgreSQL
```

Repositories should expose meaningful persistence operations rather than becoming
a generic SQL abstraction that hides important database semantics.

---

# 42. SQL-Oriented Data Access

Keep PostgreSQL semantics visible rather than designing the system around a
heavy ORM abstraction.

The exact Go SQL library/tooling is an implementation decision.

The principle is that relational constraints and transactional semantics should
remain easy to use and understand.

---

# 43. Migrations

All schema changes should use versioned migrations.

Conceptually:

```text
001_initial_schema
002_add_bids
003_add_reservations
004_add_assignments
...
```

Migrations must be deterministic and reviewable.

Do not rely on manually editing production databases.

---

# 44. Indexing

Indexes should follow real query patterns.

Likely early candidates include:

```text
rides by rider
rides by status
bids by ride
bids by driver
assignments by ride
assignments by driver
driver eligibility/status
pending outbox events
```

Do not index every column preemptively.

---

# 45. Partial Indexes

Partial indexes are useful for active/state-dependent records.

Examples:

```text
one active bid per driver/ride
pending outbox events
active assignments
```

The exact predicates should follow finalized lifecycle states.

---

# 46. Query Design

Queries should be organized around application use cases.

Examples:

```text
GetRideForRider
GetActiveBidsForRide
GetActiveAssignment
GetPendingOutboxEvents
```

Avoid a generic repository method such as:

```text
GetEverything(filters...)
```

---

# 47. N+1 Queries

Avoid loading related records one row at a time.

For example:

```text
20 bids
 ↓
20 driver queries
```

should not happen accidentally.

Use appropriate joins, batching, or explicit query plans.

---

# 48. Read/Write Strategy

Do not introduce CQRS infrastructure or read replicas prematurely.

The initial system can use PostgreSQL for both reads and writes.

Given the explicit vertical-scaling strategy, additional read infrastructure should
be introduced only when actual workload requires it.

---

# 49. Redis Failure

The system must define safe behavior if Redis disappears.

Possible consequences include:

```text
temporary loss of fast driver location
presence reconstruction
cache misses
rate-limit fallback behavior
```

But durable business history remains available:

```text
ride history
bid history
assignment history
payment history
```

Loss of Redis must not destroy business correctness.

---

# 50. PostgreSQL Failure

If PostgreSQL is unavailable, durable business commands must fail safely.

Do not claim a ride was cancelled, assigned, completed, or paid merely because a
cache operation succeeded.

---

# 51. Backups

PostgreSQL backups are mandatory.

Operational documentation must eventually define:

```text
backup frequency
retention
restore testing
point-in-time recovery
disaster recovery
```

A backup that has never been restored is an assumption, not a recovery strategy.

---

# 52. Data Retention

Retention requirements differ by domain.

Examples:

```text
financial records
ride history
operational logs
location history
outbox records
```

should not automatically share one retention period.

Exact requirements are deferred until product and compliance needs are known.

---

# 53. Financial Data

Financial records require stronger durability and auditability than transient
ride UI state.

Do not treat a final fare field on the ride as the complete financial history.

Payments, refunds, settlements, fees, and payouts require their own durable
records as defined by the payment/settlement architecture.

---

# 54. Historical Integrity

Historical records should remain explainable after configuration changes.

Examples include:

```text
old commission rate
old cancellation policy
old pricing rule
old bid
old assignment
```

Store policy/rule version or other references where necessary to explain historical
outcomes.

---

# 55. Database and API IDs

API resource identifiers should map cleanly to durable database entities without
exposing database implementation details.

Clients should not depend on:

```text
indexes
query plans
foreign-key implementation
internal database layout
```

---

# 56. Database and Domain Events

A state-changing transaction may write:

```text
business state
+
domain/outbox event
```

in one PostgreSQL transaction.

The event then bridges durable state to:

```text
realtime
notifications
analytics
other consumers
```

---

# 57. Testing Requirements

Database/application tests should cover:

```text
foreign-key integrity
unique constraints
state-transition concurrency
bid uniqueness
reservation races
cancellation races
transaction rollback
outbox atomicity
migration correctness
query performance for critical paths
```

Concurrency tests are particularly important for bidding and ride assignment.

---

# 58. What We Should Not Build Yet

Do not build:

```text
PostGIS
CQRS infrastructure
read-replica architecture
sharded PostgreSQL
multi-database domain storage
full event-sourcing architecture
complete Redis mirror of PostgreSQL
per-GPS-update durable storage
heavy ORM-driven architecture
```

These add complexity before the workload requires them.

---

# 59. Complete Database Boundary

```text
                    APPLICATION
                         │
              ┌──────────┴──────────┐
              ▼                     ▼
         PostgreSQL               Redis
       durable truth          ephemeral state
              │                     │
      ┌───────┼───────┐       ┌─────┼─────┐
      ▼       ▼       ▼       ▼     ▼     ▼
     Ride    Bids   Finance Presence Location Cache
      │
      ├── Reservation
      ├── Assignment
      ├── Cancellation
      └── Events
              │
              ▼
            Outbox
              │
              ▼
       Realtime / Push
```

---

# 60. Design Principles

1. PostgreSQL is the authoritative durable store.
2. Redis contains ephemeral or derived operational state.
3. Redis is never a shadow database for the domain.
4. Ride is the central lifecycle aggregate without becoming a giant physical row.
5. Driver, payment, and settlement domains retain their own aggregate boundaries.
6. Bids, reservations, assignments, and cancellations preserve history.
7. Database constraints enforce invariants that PostgreSQL can express.
8. Foreign keys protect durable relationships.
9. UUID-based public identifiers are preferred, with UUIDv7 currently recommended.
10. Timestamps use `TIMESTAMPTZ` and UTC.
11. Money uses integer minor units plus currency.
12. PostGIS is deferred for V1.
13. Current driver location and presence remain operational Redis state.
14. PostgreSQL transactions protect concurrent business operations.
15. Lock ordering must be consistent across multi-row transactions.
16. The outbox is written atomically with business state.
17. Outbox consumers use at-least-once processing.
18. JSONB is for flexible metadata, not core domain modeling.
19. Each table has a clear owning domain.
20. Migrations are versioned and reviewable.
21. Indexes follow actual query patterns.
22. Read/write separation is deferred until workload justifies it.
23. Redis failure must not corrupt durable business state.
24. PostgreSQL failure must fail durable commands safely.
25. Backups must be restorable, not merely created.
26. Historical financial and lifecycle outcomes must remain explainable.
27. Avoid database infrastructure complexity until real requirements justify it.
