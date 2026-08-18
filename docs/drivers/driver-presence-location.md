# Driver Presence and Location

## 1. Purpose

This document defines driver presence, availability, heartbeat, location
freshness, and the boundary between Redis operational state and PostgreSQL
durable state.

Discovery depends on this contract. It must not invent its own definition of an
available driver.

---

# 2. Core Principle

The system distinguishes:

```text
Presence
  ↓
is the driver/app alive and connected?

Location
  ↓
where was the driver last observed?

Eligibility
  ↓
is the driver/vehicle allowed to serve this ride?

Availability
  ↓
can this driver currently participate in discovery?
```

These are related but different concepts.

---

# 3. Durable vs Operational State

The initial architecture is:

```text
PostgreSQL
    ↓
durable truth

Redis
    ↓
fast operational state
```

PostgreSQL owns durable driver/account/vehicle/business state.

Redis may hold short-lived presence, location, and availability information
needed for fast discovery.

Redis is not the final authority for assignment or reservation correctness.

---

# 4. Driver Mode

The Flutter application supports both rider and driver modes.

Conceptually:

```text
RIDER MODE
    ↕
DRIVER MODE
```

Switching into driver mode does not automatically make the driver available.

The recommended flow is:

```text
Switch to Driver Mode
       ↓
Driver Mode
       ↓
Go Online
       ↓
availability evaluation
       ↓
AVAILABLE
```

---

# 5. Presence States

Initial conceptual presence states:

```text
OFFLINE
ONLINE
```

Presence describes whether the driver application is actively connected/alive.

It does not by itself mean the driver can receive ride opportunities.

---

# 6. Operational Availability

Availability is derived from multiple conditions.

Conceptually:

```text
account active
+
driver eligible
+
active vehicle valid
+
presence fresh
+
location fresh
+
not reserved
+
not assigned
       ↓
AVAILABLE
```

A driver can therefore be:

```text
ONLINE but unavailable
```

For example, the driver may be online while already assigned to a trip.

---

# 7. Recommended Operational States

The broader driver availability lifecycle may conceptually be:

```text
OFFLINE
   ↓
ONLINE
   ↓
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

The exact driver domain state model will be finalized separately.

These states must not be duplicated unnecessarily across ride, reservation,
and driver tables.

---

# 8. Going Online

The driver explicitly chooses to go online.

Conceptually:

```http
POST /api/v1/driver/presence/online
Authorization: Bearer <driver-token>
```

The backend should validate prerequisites before treating the driver as
available for discovery.

---

# 9. Going Offline

The driver can explicitly go offline:

```http
POST /api/v1/driver/presence/offline
Authorization: Bearer <driver-token>
```

A driver who is already reserved or assigned cannot use offline state to bypass
an active ride commitment.

The operational lifecycle takes precedence.

---

# 10. Heartbeat

The driver app should periodically send a heartbeat.

Conceptually:

```text
Flutter Driver
      ↓
heartbeat
      ↓
Go backend
      ↓
Redis
```

The heartbeat refreshes a short-lived presence record.

The exact interval and expiry window should be configuration rather than a
permanent hardcoded business rule.

---

# 11. Presence Freshness

A driver should be considered operationally connected only while their presence
is fresh.

Conceptually:

```text
heartbeat received
      ↓
expires_at = now + configured TTL
```

If the TTL expires:

```text
ONLINE
  ↓
stale
  ↓
not discoverable
```

The durable driver account remains intact.

---

# 12. Location Data

A current location record should contain at least:

```text
latitude
longitude
observed_at
received_at
```

Potential future fields include:

```text
accuracy
heading
speed
altitude
```

Only fields required by product and discovery should be added initially.

---

# 13. Location Updates

The driver application sends location updates through the backend.

Conceptually:

```http
POST /api/v1/driver/location
Authorization: Bearer <driver-token>
Content-Type: application/json
```

Example:

```json
{
  "latitude": 31.5204,
  "longitude": 74.3587,
  "observed_at": "2026-08-18T10:30:00Z"
}
```

The server records receipt time independently.

---

# 14. Server Time Authority

Client timestamps are useful for understanding when the device observed a
location, but server receipt time remains authoritative for freshness and
ordering decisions.

Do not trust a manipulated client clock to keep a location artificially fresh.

---

# 15. Location Freshness

Discovery should consider location freshness explicitly.

Conceptually:

```text
location_age = server_now - observed/accepted location time
```

A driver whose location exceeds the configured freshness threshold should not be
considered a valid nearby candidate for location-sensitive discovery.

The threshold should be configurable.

---

# 16. Presence and Location Are Independent

A driver may have:

```text
fresh heartbeat
+
stale location
```

In that case the driver is online but may not be suitable for nearby-driver
discovery.

Conversely, an old location without a fresh heartbeat must not make a driver
appear currently available.

---

# 17. Availability Is Not Client-Controlled

Do not trust a client request such as:

```json
{
  "available": true
}
```

as sufficient evidence.

The backend derives operational availability from authoritative conditions.

The client can request:

```text
GO ONLINE
GO OFFLINE
```

but the backend decides whether the driver is actually discoverable.

---

# 18. Driver Eligibility

Availability also requires current eligibility.

Examples include:

```text
driver account active
required driver verification complete
service authorization valid
vehicle active
vehicle compliance valid
vehicle supports requested service
```

The complete eligibility model belongs to the driver eligibility/discovery
domain.

---

# 19. Active Vehicle Requirement

A driver should have an active eligible vehicle before becoming discoverable for
vehicle-dependent ride requests.

The selected vehicle must remain associated with the driver throughout bidding,
reservation, and assignment.

---

# 20. Availability and Bidding

Submitting a bid does not reserve the driver.

The intended flow is:

```text
AVAILABLE
   ↓
receive opportunity
   ↓
submit bid
   ↓
still operationally available
```

The reservation created after rider selection changes the driver's commitment
state.

---

# 21. Availability and Reservation

Once a bid is selected and a reservation is created:

```text
AVAILABLE
   ↓
RESERVED
```

The driver must no longer be treated as freely available for conflicting new
reservations.

---

# 22. Availability and Assignment

After confirmation and assignment:

```text
RESERVED
   ↓
ASSIGNED
```

The driver must not be returned as an eligible candidate for conflicting rides.

---

# 23. Availability After Trip Completion

After trip completion:

```text
TRIP_COMPLETED
      ↓
release assignment
      ↓
re-evaluate presence/eligibility/location
      ↓
AVAILABLE or OFFLINE
```

Completion does not automatically mean the driver is available again.

If the driver has gone offline, become stale, or is otherwise ineligible, the
result remains unavailable.

---

# 24. Going Offline During a Trip

A driver cannot use the normal offline command to invalidate an active trip.

For example:

```text
TRIP_STARTED
    ↓
POST /presence/offline
```

must not simply make the assignment disappear.

The trip lifecycle owns the active commitment.

---

# 25. Network Disconnect

A driver may disappear without explicitly going offline.

The heartbeat mechanism handles this:

```text
network lost
    ↓
heartbeat stops
    ↓
presence TTL expires
    ↓
driver becomes stale
    ↓
removed from discovery
```

The durable driver account and historical state remain intact.

---

# 26. Reconnection

When the driver reconnects:

```text
connect
  ↓
authenticate
  ↓
refresh presence
  ↓
send current location
  ↓
re-evaluate availability
```

The backend should not blindly restore the previous operational state without
revalidating current assignment/reservation/eligibility state.

---

# 27. Multi-Device Consideration

The same driver account may eventually be signed in on multiple devices.

The initial system should avoid assuming:

```text
one account = one connection
```

Presence should represent the driver's operational session rather than blindly
trusting one arbitrary connection.

The exact multi-device policy can be finalized before implementation.

---

# 28. Location Privacy

Driver location is sensitive operational information.

Access should be limited by role and relationship.

Conceptually:

```text
Driver
  → own current location

Rider
  → assigned driver's relevant location

Operations
  → explicitly authorized operational visibility

Unrelated user
  → no access
```

The API must not provide arbitrary driver-location lookup by ID to normal
clients.

---

# 29. Rider Location Visibility

Before assignment, a rider should not receive arbitrary real-time locations of
candidate drivers.

During an active assigned ride, the rider may receive the selected driver's
relevant current location and ETA.

The exact privacy/product policy will be finalized separately.

---

# 30. Redis Responsibilities

Redis is appropriate for short-lived operational state such as:

```text
current driver location
presence TTL
heartbeat freshness
operational availability
spatial candidate lookup
real-time fan-out support
```

The exact Redis data structures will be decided in the discovery design.

---

# 31. PostgreSQL Responsibilities

PostgreSQL remains authoritative for durable state such as:

```text
driver account
eligibility/compliance
vehicle ownership
active vehicle
assignment
reservation
ride lifecycle
important operational milestones
```

High-frequency transient GPS should not be written to PostgreSQL by default.

---

# 32. Redis Failure

If Redis becomes unavailable:

```text
current presence/location may become unavailable or stale
```

The system must not fabricate availability.

Discovery should fail safely or degrade according to the dispatch failure policy.

Existing durable reservations and assignments remain authoritative in
PostgreSQL.

---

# 33. PostgreSQL Failure

If PostgreSQL is unavailable, the system must not perform operations requiring
durable business state changes as if they succeeded.

Examples:

```text
reservation
assignment
eligibility changes
trip lifecycle transitions
```

may not be safely committed without PostgreSQL.

---

# 34. Location Update Failure

If a location update fails temporarily, the driver application can retry.

The backend should not interpret one failed GPS update as an immediate permanent
account state change.

Freshness naturally determines whether the driver remains discoverable.

---

# 35. Battery and Network Considerations

The Flutter client should not send GPS at maximum possible frequency without
regard for battery and network cost.

Location policy should vary by operational state where justified:

```text
AVAILABLE
DRIVER_EN_ROUTE
TRIP_STARTED
```

The exact intervals should be configuration and implementation details, tuned
using real operational data.

---

# 36. Location Accuracy

A location with extremely poor accuracy should not automatically be treated as
precise.

If the platform supplies accuracy information, the backend may use it in
candidate-quality and ETA decisions.

Do not reject all imperfect GPS readings by default; urban environments can
produce noisy measurements.

---

# 37. Location Ordering

The backend should guard against out-of-order updates.

Example:

```text
update A observed_at = 10:01:10
update B observed_at = 10:01:08
```

An older observation should not overwrite a newer accepted location merely
because it arrived later.

The exact ordering policy should account for clock skew and server receipt time.

---

# 38. Driver Presence API

Initial conceptual endpoints:

```text
POST /api/v1/driver/presence/online
POST /api/v1/driver/presence/offline
POST /api/v1/driver/heartbeat
POST /api/v1/driver/location
GET  /api/v1/driver/presence
```

The final endpoint naming will be standardized during the driver API design.

---

# 39. Heartbeat Semantics

Heartbeat should be lightweight.

It should refresh presence without requiring the driver application to resend
full driver profile or eligibility data.

Conceptually:

```json
{
  "session_id": "session_123"
}
```

The exact request format is an API design detail.

---

# 40. Session Identity

Presence should be associated with an authenticated driver session where useful.

A session can help distinguish:

```text
current connection
stale connection
reconnected connection
```

The server remains responsible for deciding the driver's effective operational
presence.

---

# 41. Presence Events

Potential events include:

```text
driver.online
driver.offline
driver.stale
driver.available
driver.unavailable
```

These should be used carefully. Internal presence transitions do not all need
to be exposed to riders.

---

# 42. Driver-Facing Events

The driver should receive relevant operational events such as:

```text
reservation.created
reservation.released
assignment.created
ride.cancelled
```

Location/presence acknowledgements should not generate unnecessary event volume.

---

# 43. Discovery Boundary

Discovery should consume a clear candidate contract such as:

```text
candidate driver
current accepted location
location freshness
presence freshness
operational availability
vehicle reference
```

Discovery should then apply ride-specific eligibility rules.

It should not need to infer whether a heartbeat is stale from arbitrary Redis
keys.

---

# 44. No Direct Discovery Queries From Clients

The Flutter client must not query Redis or internal presence structures.

All discovery and availability decisions go through the Go backend.

---

# 45. Availability Does Not Guarantee Assignment

Even a driver marked `AVAILABLE` can become unavailable before selection.

Therefore:

```text
AVAILABLE
   ≠
RESERVABLE
```

The reservation transaction must revalidate the driver's current state.

---

# 46. Availability Does Not Guarantee Bid Eligibility

A nearby available driver may still be unable to bid because of ride-specific
constraints.

For example:

```text
service category
vehicle capacity
vehicle requirements
compliance
ride constraints
```

Discovery and bidding must retain their separate responsibilities.

---

# 47. Observability

Useful metrics include:

```text
driver_online_total
driver_offline_total
driver_stale_total
location_updates_total
stale_location_total
heartbeat_expiration_total
availability_transition_total
```

Useful trace/log fields include:

```text
driver_id
session_id
ride_id where relevant
vehicle_id
request_id
```

Avoid high-cardinality identifiers as metric labels.

---

# 48. Security

Presence and location endpoints require authenticated driver identity.

Do not trust client-supplied:

```text
driver_id
availability state
assignment ownership
server timestamps
```

The server derives identity from the authenticated principal and authoritative
application records.

---

# 49. What We Should Not Build Yet

Do not build:

```text
PostgreSQL writes for every GPS point
full historical GPS tracking system
complex geofencing engine
automatic availability based only on client flags
public arbitrary driver-location lookup
multi-device fleet coordination
advanced battery optimization algorithms
custom spatial indexing before discovery requirements are defined
```

Keep this layer focused on presence, current location, freshness, and
availability inputs for discovery.

---

# 50. Design Principles

1. Presence, location, eligibility, and availability are separate concepts.
2. Switching to driver mode does not automatically make the driver available.
3. The driver explicitly goes online/offline; the backend derives effective availability.
4. Heartbeats provide short-lived presence freshness.
5. Location freshness is independent from heartbeat freshness.
6. Stale drivers must disappear from discovery without deleting durable driver records.
7. PostgreSQL owns durable driver/business state.
8. Redis owns short-lived operational state where low latency is required.
9. Redis is not authoritative for reservation or assignment correctness.
10. A bid does not reserve a driver.
11. Reservation changes the driver's operational commitment state.
12. Assignment prevents conflicting ride commitments.
13. Driver availability never guarantees eventual assignment.
14. Discovery consumes a well-defined availability/location contract.
15. Clients cannot directly control authoritative availability or lifecycle state.
16. Server time controls freshness and expiration decisions.
17. Location updates must handle out-of-order and stale observations.
18. Driver location is privacy-sensitive and access-controlled.
19. High-frequency GPS should not become an unbounded PostgreSQL workload.
20. Presence/location design should remain independent from ride-specific discovery rules.
