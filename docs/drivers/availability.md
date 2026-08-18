# Driver Availability

## 1. Purpose

This document defines driver presence, availability, location freshness, and
commitment as separate concepts.

These concepts are used by driver discovery and assignment.

The core model is:

```text
Driver
  │
  ├── Eligibility
  │      Can this driver perform this ride?
  │
  ├── Presence
  │      Is the driver's app currently connected/online?
  │
  ├── Location
  │      Where is the driver currently located?
  │
  ├── Availability
  │      Can the driver receive a new ride opportunity?
  │
  └── Commitment
         Is the driver already reserved/assigned?
```

These properties must not be treated as interchangeable.

---

# 2. Core Principle

Being online does not mean being available.

Being available does not mean being eligible.

Being eligible does not mean being assignable.

For example:

```text
ONLINE
+
ELIGIBLE
+
COMMITTED
=
NOT AVAILABLE
```

A driver is a discovery candidate only when the required conditions are all
satisfied.

---

# 3. Driver State Model

The initial availability model should remain small:

```text
OFFLINE
   ↓
ONLINE_AVAILABLE
   ↓
RESERVED
   ↓
ASSIGNED
   ↓
TRIP_ACTIVE
   ↓
ONLINE_AVAILABLE / OFFLINE
```

This is an operational availability model, not a replacement for the ride
lifecycle.

---

# 4. Presence

Presence answers:

> Is the driver application currently connected and communicating with the
> backend?

Presence is short-lived operational state.

It should be represented using:

```text
heartbeat
last_seen_at
connection/session information
TTL
```

Presence should not be stored only in Go process memory.

---

# 5. Availability

Availability answers:

> Can this driver receive a new conflicting ride opportunity right now?

A driver can be:

```text
ONLINE
```

but unavailable because of:

```text
active reservation
active assignment
active trip
manual availability setting
```

---

# 6. Eligibility

Eligibility answers:

> Is this driver/vehicle allowed to perform this particular ride?

Examples:

```text
vehicle category
passenger capacity
account status
required documents
service area
ride-specific requirements
```

Eligibility is evaluated against the ride.

Therefore:

```text
Driver A
```

may be eligible for:

```text
Ride A
```

but not:

```text
Ride B
```

---

# 7. Commitment

Commitment answers:

> Has the driver already been reserved or assigned to another ride?

Commitment is authoritative assignment state.

Examples:

```text
ACTIVE RESERVATION
ACTIVE ASSIGNMENT
ACTIVE TRIP
```

A committed driver must not receive a conflicting assignment.

---

# 8. Discovery Candidate

A driver is a candidate when the relevant conditions are satisfied.

Conceptually:

```text
Presence = ONLINE
        +
Availability = AVAILABLE
        +
Eligibility = TRUE
        +
Location = FRESH
        +
Commitment = NONE
        ↓
DISCOVERY CANDIDATE
```

The exact discovery query is defined in the discovery design.

---

# 9. PostgreSQL vs Redis

The system deliberately splits durable and fast-changing state.

PostgreSQL is authoritative for:

```text
driver account state
vehicle state
eligibility-related durable state
reservation state
assignment state
trip commitment
```

Redis may maintain:

```text
current presence
latest location
heartbeat TTL
fast availability lookup
geospatial discovery index
```

Redis is not authoritative for assignment correctness.

---

# 10. Why Redis Is Appropriate for Presence

Presence changes frequently.

A driver may send heartbeats every few seconds.

Writing every heartbeat to PostgreSQL would create unnecessary database load.

Redis is appropriate for short-lived operational data because it supports:

```text
TTL
fast writes
fast reads
high-frequency updates
geospatial indexes
```

The durable driver record remains in PostgreSQL.

---

# 11. Heartbeat

The Flutter application should periodically communicate that the driver is
still active.

Conceptually:

```text
Flutter Driver App
       ↓
heartbeat
       ↓
Go API
       ↓
Redis
```

The heartbeat refreshes the driver's presence TTL.

The exact interval and TTL are configuration decisions and should not be
hard-coded into the domain model.

---

# 12. Presence TTL

A presence record should expire automatically if heartbeats stop.

Conceptually:

```text
heartbeat
   ↓
TTL refreshed
   ↓
heartbeat stops
   ↓
TTL expires
   ↓
presence becomes stale/offline
```

This protects the system against:

```text
phone crash
network loss
battery depletion
application termination
backend connection loss
```

---

# 13. Presence Is Not a Business Commitment

A Redis TTL expiring must not delete or modify:

```text
reservation
assignment
ride
trip
```

For example:

```text
Driver = ASSIGNED
Redis heartbeat expires
```

The driver remains assigned.

Presence and commitment are separate.

---

# 14. Location Freshness

Location freshness is independent from presence freshness.

A driver can have:

```text
recent heartbeat
stale GPS location
```

or:

```text
recent GPS location
stale presence
```

Discovery should require a location freshness threshold appropriate to the
product.

---

# 15. Latest Location

Redis can maintain the driver's latest operational location.

Conceptually:

```text
location:{driver_id}
    ↓
latitude
longitude
timestamp
```

The location record should contain a server-observed timestamp or an
appropriately validated source timestamp.

---

# 16. Client Location Cannot Determine Availability

The Flutter client may report:

```text
available = true
```

but the backend must still determine whether the driver is actually available.

Client state is input, not authority.

For example:

```text
Flutter:
AVAILABLE

PostgreSQL:
ACTIVE RESERVATION

Result:
NOT AVAILABLE
```

---

# 17. Going Online

A driver may request to become online.

Conceptually:

```text
POST /api/v1/driver/availability/online
```

The backend validates:

```text
authenticated driver
account active
required driver eligibility
active vehicle
vehicle eligible for service
```

If valid:

```text
OFFLINE
   ↓
ONLINE_AVAILABLE
```

The exact driver onboarding/eligibility checks are defined elsewhere.

---

# 18. Going Offline

A driver may request:

```text
POST /api/v1/driver/availability/offline
```

The backend must handle this according to current commitment.

If the driver is free:

```text
ONLINE_AVAILABLE
   ↓
OFFLINE
```

If the driver has an active assignment, the system should not blindly remove
the driver's operational commitment.

The product must define whether going offline is allowed while assigned.

---

# 19. Offline During Reservation

If a driver goes offline while a reservation is awaiting confirmation, the
reservation policy must decide whether this causes immediate rejection or
allows the confirmation window to continue.

The initial recommendation is:

```text
reservation remains authoritative
```

and the driver can confirm/reject if still within the valid reservation window.

However, if the driver becomes unreachable, the reservation will eventually
expire.

---

# 20. Offline During Trip

A driver cannot simply become operationally offline and thereby erase an
active trip.

The ride/assignment remains authoritative.

If the driver's connection disappears:

```text
presence = stale/offline
assignment = still active
trip = still active
```

Recovery logic must handle reconnection.

---

# 21. Reconnection

When the driver reconnects:

```text
connection restored
      ↓
authenticate
      ↓
refresh presence
      ↓
retrieve authoritative state
```

The driver should not rely on locally cached availability state after a long
disconnection.

The server should return the current:

```text
availability
reservation
assignment
active ride
```

as appropriate.

---

# 22. Multiple Devices

A driver may potentially authenticate from multiple devices.

The system should not allow two devices to independently create conflicting
availability state.

The backend should maintain one authoritative driver availability state.

Conceptually:

```text
Device A ─┐
          ├── Driver X
Device B ─┘
```

Both devices operate on the same driver record.

The exact multi-device session policy will be defined later.

---

# 23. Multiple Connections

Multiple WebSocket connections may exist due to:

```text
reconnect
network transition
application restart
multiple devices
```

A connection disconnecting must not automatically mark the driver unavailable
if another valid connection is still active.

Presence should therefore be modeled carefully enough to avoid a stale
connection clearing a newer session.

---

# 24. Session Identity

Presence records should include enough information to distinguish current and
stale sessions.

Conceptually:

```text
presence
├── driver_id
├── session_id
├── last_seen_at
└── expires_at / TTL
```

A stale session must not be allowed to overwrite a newer session's state.

---

# 25. Availability and Bidding

Submitting a bid does not automatically make the driver unavailable.

Example:

```text
Driver X
   ↓
ONLINE_AVAILABLE
   ↓
submits bid
   ↓
still eligible for other opportunities
```

This is intentional.

A bid is an offer, not a commitment.

---

# 26. Availability and Reservation

When a rider selects a driver's bid and reservation succeeds:

```text
ONLINE_AVAILABLE
       ↓
ACTIVE RESERVATION
       ↓
RESERVED
```

At this point the driver must not be selected for another conflicting ride.

PostgreSQL enforces the critical assignment invariant.

---

# 27. Availability and Assignment

When the driver confirms:

```text
RESERVED
   ↓
ASSIGNED
```

The driver remains unavailable for conflicting work.

The assignment is durable in PostgreSQL.

---

# 28. Availability and Trip

When the trip begins:

```text
ASSIGNED
   ↓
TRIP_ACTIVE
```

The driver remains committed until the trip completes or is otherwise
terminated.

---

# 29. Availability After Completion

When the ride completes:

```text
TRIP_ACTIVE
   ↓
release commitment
```

Then:

```text
if driver wants to remain online
    ↓
ONLINE_AVAILABLE

if driver was offline
    ↓
OFFLINE
```

Completion does not automatically force the driver online.

---

# 30. Reservation Expiration

If a reservation expires before confirmation:

```text
RESERVED
   ↓
reservation released
   ↓
ONLINE_AVAILABLE / OFFLINE
```

The resulting presence/availability depends on the driver's actual online
state.

The driver should not remain blocked indefinitely because of an expired
reservation.

---

# 31. Driver Rejection

If the driver rejects an assignment reservation:

```text
RESERVED
   ↓
reservation released
   ↓
ONLINE_AVAILABLE / OFFLINE
```

Again, the result depends on whether the driver is still online.

---

# 32. Availability Is Not Stored as One Boolean

Avoid a model such as:

```text
is_available = true
```

as the only source of truth.

Availability is derived from multiple conditions:

```text
presence
+
eligibility
+
location freshness
+
commitment
+
manual availability preference
```

A boolean can be exposed by an API as a convenient projection, but it should
not erase the underlying state distinctions.

---

# 33. Discovery Read Path

Discovery can use Redis for fast candidate lookup:

```text
Ride request
   ↓
Redis GEO
   ↓
nearby online drivers
   ↓
freshness filter
   ↓
availability filter
   ↓
PostgreSQL eligibility validation where needed
   ↓
candidates
```

The exact discovery algorithm is defined separately.

---

# 34. Redis Staleness

Redis data can become stale.

Therefore discovery results are candidates, not assignments.

A driver returned by discovery may become unavailable before the rider selects
their bid.

The assignment transaction must revalidate the driver at reservation time.

This is expected behavior, not an exceptional bug.

---

# 35. PostgreSQL Revalidation

At assignment time:

```text
Redis says:
Driver X available

PostgreSQL says:
Driver X reserved
```

The assignment must fail for Driver X.

PostgreSQL wins.

This is why discovery and assignment must remain separate stages.

---

# 36. Redis Failure

If Redis becomes unavailable:

```text
real-time presence/discovery may degrade
```

but PostgreSQL remains authoritative for:

```text
driver account
eligibility
reservation
assignment
trip commitment
```

The system must not invent availability from missing Redis data.

The safe behavior may be to stop new discovery/assignment until the required
operational dependency is healthy.

---

# 37. PostgreSQL Failure

If PostgreSQL is unavailable:

```text
assignment correctness cannot be guaranteed
```

The system should not create new reservations or assignments using Redis alone.

Operational availability is secondary to avoiding double assignment.

---

# 38. Go Process Restart

No critical availability commitment should exist only in Go memory.

After restart:

```text
PostgreSQL
   ↓
recover durable commitment state
```

Redis presence may need to be refreshed when drivers reconnect.

---

# 39. Presence vs Location Recovery

After reconnect:

```text
presence refreshed
```

and separately:

```text
location refreshed
```

The driver should not become a high-quality discovery candidate until both
presence and location freshness satisfy the required thresholds.

---

# 40. Heartbeat Failure

If heartbeats stop:

```text
TTL expires
   ↓
presence becomes stale
```

The driver should no longer be considered an online discovery candidate.

This does not automatically cancel:

```text
reservation
assignment
trip
```

Those require their own domain rules.

---

# 41. Location Failure

If location updates stop but heartbeat continues:

```text
presence = online
location = stale
```

The driver should normally be excluded from location-sensitive discovery.

This prevents dispatching a driver based on a dangerously outdated position.

---

# 42. Manual Availability

The driver may explicitly choose:

```text
available
not accepting rides
```

The exact UI labels are product decisions.

The backend should represent the driver's intent separately from automatic
commitment state.

For example:

```text
manual_mode = ACCEPTING
```

combined with:

```text
active reservation
```

still produces:

```text
not available for new assignments
```

---

# 43. Automatic Availability

The backend should derive effective availability from:

```text
manual availability intent
+
presence
+
eligibility
+
location freshness
+
commitment
```

Conceptually:

```text
effective_available =
    manual_accepting
    AND presence_fresh
    AND location_fresh
    AND eligible
    AND no_conflicting_commitment
```

The exact implementation may be distributed across PostgreSQL and Redis.

---

# 44. State Transition Authority

Availability commands are validated by the backend.

The client cannot directly mutate:

```text
reserved
assigned
trip_active
```

Those states result from ride/assignment lifecycle operations.

The driver can request changes to their own manual availability, but cannot
self-declare an assignment state.

---

# 45. Availability and Vehicle Selection

A driver may have multiple vehicles in the future.

The effective discovery candidate should be based on the vehicle that is
actually active for the driver's session/service.

The system must not assume:

```text
one driver = one permanently active vehicle
```

The exact vehicle/session model will be designed in the driver/vehicle domain.

---

# 46. Availability and Eligibility Changes

A driver's eligibility can change while online.

Examples:

```text
document expires
vehicle becomes inactive
account suspended
service category removed
```

The driver should immediately stop being a candidate for affected rides.

Existing assignments require separate policy and must not be silently deleted
because a later eligibility record changed.

---

# 47. Availability and Account Suspension

If a driver account is suspended:

```text
new discovery
new bids
new assignments
```

should be blocked.

Existing assignment/trip handling must follow operational policy rather than
blindly deleting active commitments.

---

# 48. Bid Visibility and Availability

A driver may remain available while they have submitted bids.

Therefore:

```text
has_bid = true
```

does not imply:

```text
available = false
```

The driver becomes unavailable when an actual reservation or commitment is
created.

---

# 49. Concurrency

Important races include:

```text
driver goes offline
      +
rider selects driver's bid
```

```text
driver receives reservation
      +
second ride selects same driver
```

```text
heartbeat expires
      +
driver submits bid
```

```text
driver reconnects
      +
stale session disconnects
```

These must be resolved by authoritative state and version/session checks.

---

# 50. Stale Session Protection

Suppose:

```text
Session A
   ↓
old connection

Session B
   ↓
new connection
```

If Session A disconnects after Session B connects, the disconnect must not
clear Session B's presence.

Presence updates should therefore be associated with a session identifier or
another monotonic mechanism.

---

# 51. Stale Availability Command

A client may reconnect with an old local state such as:

```text
AVAILABLE
```

while the backend knows:

```text
RESERVED
```

The backend must reject the stale command or return the current authoritative
state.

The client then reconciles.

---

# 52. Observability

Useful metrics include:

```text
driver_online_total
driver_offline_total
driver_heartbeat_total
driver_presence_expired_total
driver_location_stale_total
driver_discovery_candidates_total
driver_availability_conflicts_total
```

Useful logs/traces should include:

```text
driver_id
session_id
ride_id
reservation_id
assignment_id
```

Do not put high-cardinality driver IDs into normal metric labels.

---

# 53. Security

Availability commands must require authenticated driver identity.

A driver must not be able to modify another driver's availability.

The backend must also validate that the driver owns or is authorized to use
the selected vehicle.

Location data must be treated as sensitive operational data.

---

# 54. Complete Availability Diagram

```text
                         ┌─────────────┐
                         │   OFFLINE   │
                         └──────┬──────┘
                                │
                         go online + valid
                                │
                                ▼
                   ┌────────────────────────┐
                   │   ONLINE_AVAILABLE     │
                   └───────────┬────────────┘
                               │
                         reservation
                               │
                               ▼
                   ┌────────────────────────┐
                   │       RESERVED         │
                   └───────────┬────────────┘
                               │
                           confirm
                               │
                               ▼
                   ┌────────────────────────┐
                   │       ASSIGNED         │
                   └───────────┬────────────┘
                               │
                         trip starts
                               │
                               ▼
                   ┌────────────────────────┐
                   │      TRIP_ACTIVE       │
                   └───────────┬────────────┘
                               │
                        trip completes
                               │
                               ▼
                  ┌──────────────────────────┐
                  │ ONLINE_AVAILABLE / OFFLINE│
                  └──────────────────────────┘

Other paths:

RESERVED
   ↓
rejected / expired / cancelled
   ↓
ONLINE_AVAILABLE / OFFLINE

Any non-committed online state
   ↓
manual offline
   ↓
OFFLINE

ONLINE_AVAILABLE
   ↓
heartbeat/location becomes stale
   ↓
not a discovery candidate
```

---

# 55. What We Should Not Build Yet

Do not build:

```text
Distributed Redis locks as assignment authority
Permanent driver availability in a single boolean
Per-heartbeat PostgreSQL writes
Complex multi-device synchronization
Automatic cancellation of trips because a heartbeat expired
Location-based assignment without freshness checks
Client-controlled assignment states
Sophisticated driver presence analytics
```

---

# 56. Design Principles

1. Presence, availability, eligibility, location, and commitment are separate concepts.
2. Online does not automatically mean available.
3. Available does not automatically mean eligible.
4. A bid does not make a driver unavailable.
5. A reservation makes a driver unavailable for conflicting assignments.
6. Assignment commitment is authoritative in PostgreSQL.
7. Redis is appropriate for fast-changing presence and location data.
8. Redis is not the source of truth for assignment correctness.
9. Heartbeats should use TTL-based operational presence.
10. Presence freshness and location freshness are separate.
11. Stale location should exclude a driver from location-sensitive discovery.
12. Client availability state is not authoritative.
13. Discovery results are candidates, not guarantees.
14. Assignment must revalidate driver availability transactionally.
15. Stale sessions must not overwrite newer sessions.
16. Redis failure must not create duplicate assignments.
17. PostgreSQL failure must block correctness-sensitive assignment operations.
18. Driver reconnection must reconcile against authoritative server state.
19. Existing commitments survive presence loss until their own domain rules resolve them.
20. Effective availability is derived from multiple conditions rather than stored as a single authoritative boolean.
