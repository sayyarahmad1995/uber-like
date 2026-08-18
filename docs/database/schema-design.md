# Database Schema Design

## 1. Purpose

This document turns the database architecture into a concrete relational model for
PostgreSQL.

The schema is designed around durable business state, explicit relationships,
strong constraints, query-driven indexes, and historical records where business
history matters.

Redis remains operational/ephemeral state and is not authoritative for business
correctness.

---

# 2. Core Rules

1. PostgreSQL is authoritative for durable business state.
2. Money is stored as integer minor units, never floating point.
3. Foreign keys protect relational integrity.
4. Current state and historical state are separate concepts.
5. Status transitions are enforced by application/domain logic and supported by
   database constraints where practical.
6. Client-provided identifiers never establish ownership.
7. Sensitive data is minimized.
8. Outbox insertion occurs in the same transaction as the business state change.
9. IDs should be opaque and non-sequential at the API boundary; UUIDs are the
   default identifier strategy.
10. Timestamps use `TIMESTAMPTZ` and are stored in UTC.

---

# 3. PostgreSQL Types

The initial schema should use PostgreSQL-native types where they improve correctness:

```text
UUID
TEXT
BOOLEAN
BIGINT
INTEGER
NUMERIC where fractional arithmetic is genuinely required
TIMESTAMPTZ
JSONB
```

For business state machines, explicit PostgreSQL enums may be used where the set of
values is stable. If frequent evolution is expected, constrained text values may be
preferable to avoid migration friction.

The final implementation should choose one approach consistently per bounded domain.

---

# 4. `users`

Application identity mapped from the external OIDC provider.

```text
users
-----
id                  UUID PK
 oidc_subject        TEXT NOT NULL UNIQUE
status              TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

Rules:

- `oidc_subject` is unique.
- The application derives identity from a verified OIDC token.
- The OIDC subject is not treated as a public API identifier.
- A user can exist without a driver profile.

Recommended initial status values:

```text
active
suspended
deactivated
```

---

# 5. `driver_profiles`

Represents the driver's application-level identity and account state.

```text
driver_profiles
---------------
id                  UUID PK
user_id             UUID NOT NULL UNIQUE FK users(id)
status              TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

The one-to-one relationship means one application user has at most one driver
profile.

Do not put all eligibility state into this table.

Driver account status and operational eligibility are separate concepts.

---

# 6. Driver Eligibility Boundary

Eligibility should not be represented by a single permanent boolean such as:

```text
is_eligible
```

Eligibility can depend on multiple durable facts:

```text
account status
verification status
document validity
vehicle status
policy restrictions
```

The initial schema should therefore keep eligibility evaluation as a domain concern
that can derive a decision from authoritative records.

If a cached/materialized eligibility decision is introduced later, it must be
rebuildable from durable state.

---

# 7. `vehicles`

Represents a vehicle associated with a driver.

```text
vehicles
--------
id                  UUID PK
driver_id           UUID NOT NULL FK driver_profiles(id)
vehicle_type        TEXT NOT NULL
make                TEXT
model               TEXT
registration_number TEXT NOT NULL
status              TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

Recommended initial status values:

```text
active
inactive
suspended
```

Registration uniqueness should be scoped according to the legal/operational rules
of the deployment. If registrations are globally unique in the target market, use a
unique constraint; otherwise define the appropriate scope explicitly before adding
it.

---

# 8. `rides`

The central durable lifecycle record.

```text
rides
-----
id                  UUID PK
rider_id            UUID NOT NULL FK users(id)
status              TEXT NOT NULL
pickup_latitude     NUMERIC(...) NOT NULL
pickup_longitude    NUMERIC(...) NOT NULL
dropoff_latitude    NUMERIC(...) NOT NULL
dropoff_longitude   NUMERIC(...) NOT NULL
requested_at        TIMESTAMPTZ NOT NULL
bidding_started_at  TIMESTAMPTZ
reserved_at         TIMESTAMPTZ
started_at          TIMESTAMPTZ
completed_at        TIMESTAMPTZ
cancelled_at        TIMESTAMPTZ
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

The exact coordinate precision must be fixed during migration implementation. It
must be sufficient for mapping without implying false precision.

Current lifecycle state belongs in `rides.status`.

Historical lifecycle facts belong in `ride_events` and related records.

---

# 9. Ride Status

The initial state machine should support the lifecycle already defined by the ride
architecture.

Conceptually:

```text
requested
  ↓
bidding
  ↓
reserved
  ↓
assigned
  ↓
driver_arrived
  ↓
in_progress
  ↓
completed
```

Cancellation can occur only from states where cancellation is legally/business-wise
permitted.

Exact transition rules belong in the ride state-machine contract, not in arbitrary
SQL updates.

---

# 10. `bids`

Durable driver offers against a ride.

```text
bids
----
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
driver_id           UUID NOT NULL FK driver_profiles(id)
amount_minor        BIGINT NOT NULL
currency            TEXT NOT NULL
status              TEXT NOT NULL
expires_at          TIMESTAMPTZ
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

Rules:

- `amount_minor` must be greater than zero where a positive fare is required.
- Currency must use a controlled representation.
- A bid belongs to exactly one ride and one driver.
- Historical bids are not deleted as a normal lifecycle operation.

---

# 11. Active Bid Uniqueness

A driver should not have multiple simultaneously active bids for the same ride.

Implement this with a partial unique index, conceptually:

```text
UNIQUE (ride_id, driver_id)
WHERE status IN ('submitted', 'active')
```

The exact active status set must match the bidding state machine.

This is a database invariant, not merely a Go validation.

---

# 12. Bid History

If detailed bid state transitions are required for audit/debugging, use:

```text
bid_events
----------
id                  UUID PK
bid_id              UUID NOT NULL FK bids(id)
event_type          TEXT NOT NULL
occurred_at         TIMESTAMPTZ NOT NULL
metadata            JSONB
```

`bids.status` remains the current state.

`bid_events` preserves the historical timeline.

This is not full event sourcing.

---

# 13. `reservations`

Represents the durable reservation/hold between bid selection and assignment.

```text
reservations
------------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
bid_id              UUID NOT NULL FK bids(id)
driver_id           UUID NOT NULL FK driver_profiles(id)
status              TEXT NOT NULL
expires_at          TIMESTAMPTZ
created_at          TIMESTAMPTZ NOT NULL
confirmed_at        TIMESTAMPTZ
cancelled_at        TIMESTAMPTZ
```

The reservation should reference the selected bid explicitly so the system can
explain how the reservation originated.

---

# 14. Reservation Integrity

Application/domain logic must verify that:

```text
reservation.ride_id == bid.ride_id
reservation.driver_id == bid.driver_id
```

The database cannot conveniently express every cross-row domain invariant using a
simple foreign key, so these checks belong in the transaction/use case as well.

At most one reservation may be active for a ride unless the reservation state
machine explicitly permits another model.

---

# 15. `assignments`

Represents the durable driver assignment to a ride.

```text
assignments
-----------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
driver_id           UUID NOT NULL FK driver_profiles(id)
status              TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
started_at          TIMESTAMPTZ
ended_at            TIMESTAMPTZ
ended_reason        TEXT
```

Assignments should not be deleted merely because the ride is cancelled or the
assignment ends.

Historical assignment records are useful for support, reconciliation, and audit.

---

# 16. Active Assignment Uniqueness

The system should normally have at most one active assignment per ride.

Use a partial unique index conceptually:

```text
UNIQUE (ride_id)
WHERE status IN ('assigned', 'driver_arrived', 'in_progress')
```

The exact active statuses must match the assignment state machine.

A corresponding driver-side uniqueness constraint may be required if a driver is
forbidden from holding multiple simultaneous assignments.

---

# 17. `cancellations`

Cancellation is historical business data, not merely a field overwrite.

```text
cancellations
-------------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
actor_type          TEXT NOT NULL
actor_id            UUID NOT NULL
reason              TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
```

`actor_id` must correspond to the authenticated/business actor represented by
`actor_type`.

The ride's current state remains in `rides.status`.

---

# 18. `ride_events`

Durable ride lifecycle history.

```text
ride_events
-----------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
event_type          TEXT NOT NULL
occurred_at         TIMESTAMPTZ NOT NULL
metadata            JSONB
```

This table is for business history and operational reconstruction, not for making
the entire application event-sourced.

Useful events include:

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

---

# 19. `payments`

Payment state is separate from ride state.

```text
payments
--------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
status              TEXT NOT NULL
amount_minor        BIGINT NOT NULL
currency            TEXT NOT NULL
provider             TEXT
provider_payment_id TEXT
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

Raw payment credentials must not be stored here.

Provider identifiers and statuses may be stored where required for reconciliation.

---

# 20. Payment Events

Detailed financial history belongs in:

```text
payment_events
--------------
id                  UUID PK
payment_id          UUID NOT NULL FK payments(id)
event_type          TEXT NOT NULL
provider_event_id   TEXT
occurred_at         TIMESTAMPTZ NOT NULL
metadata            JSONB
```

Provider event IDs should be unique where the provider guarantees their uniqueness.

This supports webhook idempotency and financial reconciliation.

---

# 21. `settlements`

Settlement represents the durable allocation/reconciliation of completed financial
obligations.

```text
settlements
-----------
id                  UUID PK
ride_id             UUID NOT NULL FK rides(id)
status              TEXT NOT NULL
gross_amount_minor  BIGINT NOT NULL
platform_fee_minor  BIGINT NOT NULL
driver_amount_minor BIGINT NOT NULL
currency            TEXT NOT NULL
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

The monetary invariant should hold:

```text
gross_amount_minor
= platform_fee_minor + driver_amount_minor + other explicitly modeled amounts
```

If additional components are introduced, model them explicitly rather than hiding
arithmetic in arbitrary JSON.

---

# 22. `payouts`

Represents movement of settled driver earnings toward the driver.

```text
payouts
-------
id                  UUID PK
driver_id           UUID NOT NULL FK driver_profiles(id)
settlement_id       UUID FK settlements(id)
status              TEXT NOT NULL
amount_minor        BIGINT NOT NULL
currency            TEXT NOT NULL
provider            TEXT
provider_payout_id  TEXT
created_at          TIMESTAMPTZ NOT NULL
updated_at          TIMESTAMPTZ NOT NULL
```

Payouts must be idempotent against provider operations.

---

# 23. `outbox_events`

Transactional outbox records are durable PostgreSQL rows.

```text
outbox_events
-------------
id                  UUID PK
event_type          TEXT NOT NULL
schema_version      INTEGER NOT NULL
aggregate_type      TEXT NOT NULL
aggregate_id        UUID NOT NULL
payload             JSONB NOT NULL
occurred_at         TIMESTAMPTZ NOT NULL
created_at          TIMESTAMPTZ NOT NULL
published_at        TIMESTAMPTZ
attempt_count       INTEGER NOT NULL DEFAULT 0
last_error          TEXT
```

The outbox insert occurs in the same transaction as the business change that
produced the event.

---

# 24. Outbox Indexing

The publisher's hot query should be supported by an index over unpublished events.

Conceptually:

```text
(published_at, created_at)
WHERE published_at IS NULL
```

If row claiming uses additional state such as `locked_at` or `status`, the index
should reflect the actual publisher query rather than being designed abstractly.

---

# 25. Outbox Cleanup

Published events can eventually be archived/deleted according to retention policy.

Do not delete records merely because they were published if required downstream
processing/recovery still depends on them.

Cleanup must be bounded and observable.

---

# 26. Identity and Ownership

Ownership is always expressed through foreign keys and server-side authorization.

Examples:

```text
rides.rider_id → users.id
bids.driver_id → driver_profiles.id
vehicles.driver_id → driver_profiles.id
assignments.driver_id → driver_profiles.id
```

A client-supplied ID never overrides these relationships.

---

# 27. Soft Deletes

Do not introduce soft deletion everywhere by default.

For records where historical existence matters, prefer explicit status/lifecycle
state.

Examples:

```text
users.status
vehicles.status
rides.status
bids.status
assignments.status
```

Use a deletion marker only where the product genuinely needs logical deletion and
retention semantics.

---

# 28. Timestamps

Use `TIMESTAMPTZ` for business and operational timestamps.

At minimum, durable entities should normally have:

```text
created_at
updated_at
```

Lifecycle-specific timestamps should exist only when they provide a meaningful
query/audit boundary.

---

# 29. Money

All monetary values use integer minor units:

```text
BIGINT amount_minor
TEXT currency
```

Examples:

```text
1250 PKR → amount_minor = 1250
```

Do not use floating-point values for fares, fees, payments, settlements, or payouts.

---

# 30. Currency

Currency must be an explicit field on monetary records.

Do not infer currency from server locale, user profile, or deployment environment.

The allowed currency set should be controlled by application/domain configuration.

---

# 31. Coordinates

Pickup/dropoff coordinates should be stored with a numeric type whose precision is
sufficient for mapping.

The implementation must define a fixed precision/scale rather than leaving it to
unbounded numeric values.

Latitude/longitude should also have range checks:

```text
latitude  ∈ [-90, 90]
longitude ∈ [-180, 180]
```

---

# 32. JSONB Usage

JSONB is appropriate for metadata that is:

```text
optional
schema-flexible
non-authoritative
not frequently queried as a primary business key
```

Do not put core relational state into JSONB merely to avoid defining columns.

Examples of appropriate JSONB:

```text
provider metadata
historical event metadata
diagnostic context
```

---

# 33. Foreign Keys

Foreign keys should be used for durable relationships unless there is a documented
performance/partitioning reason not to use them.

Default behavior should favor referential integrity over application-only checks.

---

# 34. Delete Behavior

Default to restrictive deletion for business records.

Do not cascade-delete an entire ride history merely because a parent row is removed.

Where lifecycle history must remain, use status changes rather than destructive
cascades.

---

# 35. Indexing Strategy

Initial indexes should match known access patterns.

Recommended starting set:

```text
users(oidc_subject)

rides(rider_id, created_at)
r​​ides(status, created_at)

bids(ride_id, created_at)
bids(driver_id, created_at)

reservations(ride_id)
reservations(driver_id)

assignments(ride_id)
assignments(driver_id)

ride_events(ride_id, occurred_at)
bid_events(bid_id, occurred_at)

payments(ride_id)
payment_events(payment_id, occurred_at)

settlements(ride_id)
payouts(driver_id, created_at)

outbox_events(published_at, created_at)
```

These are starting indexes, not a claim that every index must exist forever.

---

# 36. Index Discipline

Every index has a write and storage cost.

Do not create indexes simply because a column exists.

After real query patterns are known, use PostgreSQL query plans and production-like
benchmarks to validate index usefulness.

---

# 37. Transaction Boundaries

The database schema supports transactions around business invariants.

Examples:

```text
select winning bid
+ create reservation
+ update ride
+ write outbox event
```

or:

```text
complete ride
+ create payment state transition
+ write outbox event
```

The exact transaction scope belongs to the application use case.

---

# 38. Concurrency Control

Critical state transitions must protect against concurrent requests.

Depending on the operation, use:

```text
row-level locking
conditional UPDATE
unique constraints
transaction isolation
idempotency keys
```

Do not rely on an in-memory Go mutex for correctness across requests/processes.

---

# 39. Idempotency Keys

Important externally retried commands may require durable idempotency records.

Potential operations include:

```text
ride creation
payment commands
payout commands
external webhook processing
```

The exact idempotency table should be introduced when the API command contracts are
finalized, rather than adding a generic mechanism to every endpoint.

---

# 40. Operational Availability Data

The following should not be stored as authoritative durable state merely because it
is useful operationally:

```text
driver online presence
current driver location
WebSocket connection state
short-lived discovery membership
```

These belong primarily in Redis and can be reconstructed/refreshed from durable
records where necessary.

---

# 41. What Is Intentionally Missing

The following are intentionally not finalized in this schema document yet:

```text
exact eligibility/document tables
exact pricing/fare breakdown tables
exact payment-provider schema
exact API idempotency table
geospatial indexing implementation
partitioning strategy
multi-tenant schema
```

They should be introduced only after their corresponding contracts are finalized.

---

# 42. Migration Order

The initial migration sequence should follow dependency order:

```text
001_extensions_and_types
002_users
003_driver_profiles
004_vehicles
005_rides
006_bids
007_bid_events
008_reservations
009_assignments
010_cancellations
011_ride_events
012_payments
013_payment_events
014_settlements
015_payouts
016_outbox_events
017_indexes_and_constraints
```

This is a proposed implementation sequence, not a promise that every item will be
one physical migration file.

---

# 43. Migration Rules

Migrations must be:

```text
versioned
ordered
reviewable
repeatable in fresh environments
safe to execute exactly once
```

Never modify an already-applied migration to change production history.

Create a new migration for subsequent changes.

---

# 44. Schema Testing

Tests should verify at least:

```text
foreign-key integrity
unique constraints
partial unique indexes
check constraints
money precision
coordinate ranges
outbox atomicity
critical concurrent transitions
migration from empty database
migration from representative previous version
```

---

# 45. Final Relational Model

```text
users
  │
  └── driver_profiles
          │
          └── vehicles

users
  │
  └── rides
        │
        ├── bids ────────── driver_profiles
        │    └── bid_events
        │
        ├── reservations
        │
        ├── assignments ── driver_profiles
        │
        ├── cancellations
        │
        └── ride_events

rides
  │
  └── payments
        └── payment_events

rides
  │
  └── settlements
        └── payouts ───── driver_profiles

all durable business transactions
              │
              ▼
        outbox_events
```

---

# 46. Design Principles

1. PostgreSQL is the source of truth for durable business state.
2. Redis is never authoritative for business correctness.
3. Users and driver profiles are separate identities/capabilities.
4. Driver eligibility is not collapsed into one permanent boolean.
5. Rides own lifecycle state; history is represented separately.
6. Bids are durable and historically preserved.
7. Active bid uniqueness is enforced at the database level.
8. Reservations and assignments are explicit durable states.
9. Cancellation is preserved as business history.
10. Financial records are separate from ride lifecycle state.
11. Money uses integer minor units.
12. Currency is explicit.
13. Coordinates have bounded valid ranges and defined precision.
14. JSONB is for flexible metadata, not core relational state.
15. Foreign keys protect durable relationships.
16. Destructive cascading is avoided for business history.
17. Indexes are driven by access patterns and measured query plans.
18. Critical transitions use database concurrency controls rather than process-local locks.
19. Outbox events commit atomically with business state.
20. Operational availability/location state remains primarily in Redis.
21. Schema evolution uses forward migrations rather than rewriting applied history.
22. Exact contracts for eligibility, pricing, payment providers, and idempotency are finalized before implementation of those subsystems.
