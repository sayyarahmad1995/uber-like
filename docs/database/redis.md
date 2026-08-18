# Redis Design

## 1. Purpose

This document defines the role of Redis in the platform.

Redis is an operational data store for:

- Ephemeral state
- Fast-changing state
- Real-time coordination
- WebSocket connection state
- Rate limiting
- Short-lived caches
- Temporary coordination

Redis is **not** the authoritative source for durable business state.

PostgreSQL remains authoritative for:

- Users
- Drivers
- Vehicles
- Rides
- Bids
- Assignments
- Ride lifecycle
- Bid lifecycle
- Historical records
- Audit events

---

# 2. Redis Design Principle

The system must remain logically correct if Redis is temporarily unavailable.

That means:

```text
PostgreSQL
    ↓
Authoritative business state

Redis
    ↓
Operational / ephemeral state
```

Redis may improve:

- Latency
- Throughput
- Real-time behavior
- Coordination

Redis must not become a hidden second database.

---

# 3. Initial Redis Responsibilities

The initial Redis responsibilities are:

```text
1. Driver operational presence
2. Latest driver location
3. WebSocket connection state
4. Rate limiting
5. Short-lived coordination
6. Temporary caches
7. Real-time fan-out support
```

The following are deliberately excluded from Redis as authoritative data:

```text
Users
Vehicles
Rides
Bids
Assignments
Ride history
Bid history
Audit events
```

---

# 4. Driver Operational Presence

PostgreSQL stores the durable driver availability state:

```text
driver_profiles.availability_status
```

Redis stores the operational presence required by real-time systems.

Example:

```text
driver:{driver_id}:presence
```

Possible values:

```text
ONLINE
OFFLINE
```

The Redis value should contain enough information to determine whether the
driver currently has an active operational connection.

Example conceptual value:

```json
{
  "status": "ONLINE",
  "last_seen_at": "2026-08-18T10:00:00Z"
}
```

This information is ephemeral.

---

# 5. Presence TTL

Presence must have a TTL.

The client or WebSocket connection periodically refreshes the presence.

Conceptually:

```text
Driver connects
    ↓
SET presence
    ↓
TTL = N seconds
    ↓
Heartbeat
    ↓
EXPIRE refreshed
```

If heartbeats stop:

```text
TTL expires
    ↓
Driver considered operationally disconnected
```

The exact TTL and heartbeat interval will be determined during WebSocket
implementation and load testing.

The important principle is:

> Presence must expire automatically.

We must not depend on receiving an explicit "offline" message.

---

# 6. Driver Location

The latest driver location is ephemeral.

Conceptual key:

```text
driver:{driver_id}:location
```

Example value:

```json
{
  "latitude": 33.6844,
  "longitude": 73.0479,
  "accuracy_meters": 8,
  "heading": 120,
  "speed_mps": 11.4,
  "recorded_at": "2026-08-18T10:00:05Z"
}
```

The exact fields will be finalized with the mobile/WebSocket protocol.

---

# 7. Location TTL

Driver location must expire automatically.

Example:

```text
SET driver:{driver_id}:location
EXPIRE driver:{driver_id}:location N
```

If no location update arrives within the TTL:

```text
latest location = unavailable/stale
```

Clients must never assume an expired location is current.

The API should expose freshness information when returning a last-known
location.

---

# 8. Location Updates

The expected flow is:

```text
Flutter Driver App
        ↓
WebSocket
        ↓
API / Realtime Layer
        ↓
Redis
        ↓
latest driver location
        ↓
WebSocket fan-out
        ↓
Rider App
```

The system should not write every GPS update to PostgreSQL.

This avoids turning PostgreSQL into a high-frequency telemetry store.

---

# 9. Location Validation

The backend must validate location updates before storing them.

At minimum:

- Valid latitude
- Valid longitude
- Reasonable timestamp
- Authenticated driver
- Appropriate driver state
- Update frequency
- Payload size

The client is not trusted to determine whether a location update belongs to
the authenticated driver.

---

# 10. Driver Location History

Redis stores only the latest operational location initially.

The platform does not persist every location update.

If future requirements need:

- Route reconstruction
- Driver analytics
- Fraud detection
- Trip replay
- Historical GPS analysis

then a separate location-history architecture should be designed.

It should not be added casually to the initial PostgreSQL schema.

---

# 11. Geographic Driver Discovery

Geographic driver discovery is intentionally not committed to a specific Redis
implementation yet.

Possible future approaches include:

```text
Redis GEO
PostGIS
Dedicated geospatial service
Application-level filtering
```

The decision should be based on the actual dispatch query.

For example:

```text
Find eligible online drivers
within X kilometers
for service type Y
who are not already committed
```

This is substantially more complex than:

```text
Find drivers near coordinate X.
```

Therefore the initial design does not make Redis GEO a mandatory architectural
dependency.

---

# 12. Bidding State

The authoritative bid state is PostgreSQL.

Redis may maintain temporary information needed for real-time delivery.

For example:

```text
ride:{ride_id}:realtime
```

could contain temporary information such as:

```text
bidding deadline
connected rider
connected drivers
```

However:

```text
bids.status
bids.amount
rides.status
```

remain authoritative in PostgreSQL.

A Redis key must never be treated as proof that a bid exists or is valid.

---

# 13. Bidding Deadline

The authoritative deadline is:

```text
PostgreSQL:
rides.bidding_ends_at
```

Redis may maintain a temporary timer or sorted-set entry to help trigger
real-time processing.

Example conceptual structure:

```text
bidding:deadlines
```

with:

```text
score = Unix timestamp
member = ride_id
```

This can help a worker identify rides whose bidding window has ended.

However:

```text
Redis timer
    ≠
business authority
```

If the worker runs late, the API must still reject a bid submitted after the
PostgreSQL-defined deadline.

---

# 14. Assignment Confirmation Deadline

The same principle applies to driver confirmation.

Authoritative value:

```text
rides.driver_confirmation_deadline
```

Redis may assist with timeout processing.

Conceptually:

```text
assignment:deadlines
```

with:

```text
score = confirmation deadline
member = ride_id
```

A worker can use this to find expired assignments.

The worker then performs the authoritative state transition in PostgreSQL.

---

# 15. WebSocket Connection State

Redis may store information about active WebSocket connections.

Conceptual key:

```text
ws:user:{user_id}:connections
```

Example conceptual data:

```text
connection_id
server_instance
connected_at
last_seen_at
```

This is useful when multiple Go instances are running.

Example:

```text
                    ┌── Go Instance A
Flutter ── Gateway ─┼── Go Instance B
                    └── Go Instance C
                           │
                           ▼
                         Redis
```

Redis allows instances to coordinate which connection belongs to which user.

---

# 16. WebSocket Connection Ownership

A WebSocket connection belongs to the server instance that accepted it.

Redis should not attempt to become the connection itself.

Conceptually:

```text
Redis
    ↓
"User X has connection Y on server B"
```

not:

```text
Redis
    ↓
actual WebSocket connection
```

The actual socket remains owned by the Go process.

---

# 17. Real-Time Event Fan-Out

When multiple backend instances exist, an event may need to reach a client
connected to a different instance.

Example:

```text
Driver
   ↓
Go Instance A
   ↓
PostgreSQL transaction
   ↓
event
   ↓
Redis Pub/Sub
   ↓
Go Instance B
   ↓
WebSocket
   ↓
Rider
```

Redis Pub/Sub is a candidate mechanism for this.

However, Pub/Sub messages are ephemeral.

If a subscriber is disconnected when the message is published:

```text
message
   ↓
missed
```

Therefore Pub/Sub must not be used as the durable event source.

---

# 18. Event Recovery

A client may disconnect or miss real-time events.

The recovery strategy is:

```text
WebSocket reconnect
       ↓
Fetch authoritative current state
       ↓
Resume real-time subscription
```

For example:

```text
GET /api/v1/rides/{ride_id}
```

can recover the current ride state.

The client does not need every historical WebSocket event to reconstruct
business state.

Durable history is available from PostgreSQL where required.

---

# 19. Redis Pub/Sub

Redis Pub/Sub is appropriate for:

- Short-lived real-time notifications
- Cross-instance WebSocket fan-out
- Operational notifications

Redis Pub/Sub is not appropriate as the sole mechanism for:

- Ride state
- Bid state
- Payment state
- Durable event history
- Guaranteed message delivery

For guaranteed asynchronous delivery, an outbox/stream/queue architecture
should be introduced.

---

# 20. Outbox Integration

The planned reliable event flow is:

```text
BEGIN PostgreSQL transaction
       ↓
Update business state
       ↓
Insert outbox event
       ↓
COMMIT
       ↓
Outbox publisher
       ↓
Redis / messaging layer
       ↓
WebSocket instances
       ↓
Clients
```

This prevents a successful database transaction from being silently detached
from the corresponding asynchronous event.

The outbox design will be defined separately.

---

# 21. Rate Limiting

Redis is a suitable mechanism for distributed rate limiting.

Rate limits may apply to:

- Authentication-related endpoints
- Ride creation
- Bid submission
- Bid modification
- Location updates
- WebSocket connection attempts
- General API requests

Conceptual key:

```text
rate_limit:{scope}:{identifier}
```

The exact algorithm will be selected during implementation.

Possible algorithms:

```text
Fixed window
Sliding window
Token bucket
Leaky bucket
```

We should prefer the simplest algorithm that satisfies the requirement.

---

# 22. Bid Modification Rate Limiting

Bid modification deserves explicit protection.

Without a limit, a malicious or buggy client could repeatedly modify:

```text
1000
999
998
997
...
```

at high frequency.

This creates unnecessary:

- Database writes
- Events
- WebSocket traffic
- CPU usage

Therefore bid modifications should have a rate limit.

The exact threshold is a product/business decision and should not be hard-coded
in this architecture document.

---

# 23. Location Rate Limiting

Location updates are expected to be frequent.

The backend should enforce:

- Maximum update frequency
- Maximum payload size
- Maximum acceptable timestamp drift
- Authentication
- Driver ownership

Redis can support distributed rate limiting.

However, the client should also reduce unnecessary updates before sending them.

The server remains authoritative for abuse protection.

---

# 24. Short-Lived Coordination

Redis may be used for short-lived coordination where losing the state is
acceptable.

Examples:

```text
Distributed worker coordination
Temporary locks
Rate limiting
Presence
Short-lived timers
```

Any lock used to protect a critical business invariant must have a
PostgreSQL-based correctness mechanism as the final authority.

Redis locks are an optimization/coordination mechanism, not a replacement for
database transactions.

---

# 25. Distributed Locks

Distributed locks should not be the primary mechanism for business
correctness.

Bad design:

```text
Acquire Redis lock
    ↓
Assume business state is safe
    ↓
Update PostgreSQL
```

Better:

```text
Redis coordination if useful
        +
PostgreSQL transaction/constraint
        ↓
Business correctness
```

If Redis disappears, the business invariant must still be protected by
PostgreSQL.

---

# 26. Cache Strategy

Redis may cache expensive read results.

Potential candidates:

```text
Driver profile
Vehicle information
Static service configuration
Reference pricing configuration
Google Maps-related short-lived results
```

Cache entries must have:

- Explicit TTL
- Clear ownership
- Defined invalidation behavior
- Safe fallback to PostgreSQL/source service

We should not cache everything.

Caching is justified when measurement shows that it solves a real performance
problem.

---

# 27. Cache-Aside Pattern

The default caching strategy should be cache-aside.

```text
Read
 ↓
Redis
 ↓
hit? ── yes ──→ return
 ↓ no
PostgreSQL
 ↓
populate Redis
 ↓
return
```

On mutation:

```text
PostgreSQL update
       ↓
invalidate/update Redis
```

The PostgreSQL transaction remains authoritative.

---

# 28. Cache Failure

If Redis is unavailable:

```text
cache read
    ↓
failure
    ↓
fallback to PostgreSQL/source
```

The application should not crash simply because an optional cache is
unavailable.

For required operational features such as distributed rate limiting or
real-time fan-out, Redis failure may cause degraded functionality.

Those failure modes must be explicit.

---

# 29. Key Naming Convention

Redis keys should follow a consistent hierarchy.

Initial convention:

```text
{domain}:{entity}:{identifier}:{attribute}
```

Examples:

```text
driver:123:presence
driver:123:location
ws:user:456:connections
ride:789:realtime
rate_limit:bid_submit:driver:123
```

Keys should:

- Be predictable
- Be scoped
- Avoid ambiguous abbreviations
- Avoid embedding sensitive information
- Have documented TTL behavior where applicable

---

# 30. TTL Policy

Every ephemeral Redis key must have an explicit TTL policy.

Examples:

```text
Presence
    → short TTL

Latest location
    → short TTL

Rate-limit counters
    → algorithm-dependent TTL

Temporary coordination
    → short TTL

Cached profile
    → longer TTL
```

No ephemeral key should depend on manual deletion for correctness.

---

# 31. Redis Serialization

Application data stored in Redis should use a clearly defined serialization
format.

For simple values:

```text
string
integer
timestamp
```

For structured values:

```text
JSON
```

For performance-sensitive structures, Redis-native data types may be used.

The application must version structured values if their schema can change.

---

# 32. Redis Data Ownership

Each Redis key must have a clear owner.

Example:

```text
driver:{id}:location
    Owner: realtime/location subsystem

driver:{id}:presence
    Owner: realtime/presence subsystem

rate_limit:*
    Owner: API middleware

ws:user:{id}:connections
    Owner: WebSocket subsystem
```

A subsystem must not silently repurpose another subsystem's keys.

---

# 33. Redis Failure Modes

## Redis unavailable

Expected behavior:

```text
PostgreSQL operations
    ↓
continue where Redis is optional
```

Potentially degraded:

```text
Real-time fan-out
Presence
Rate limiting
Live location
```

The exact behavior depends on the feature.

---

## Redis data loss

Redis data may be reconstructed when it represents ephemeral state.

Examples:

```text
Presence → rebuilt on connection/heartbeat
Location → rebuilt on next GPS update
WebSocket state → rebuilt on connection
Cache → rebuilt from source
```

Durable business state must not require Redis recovery.

---

# 34. Redis Persistence

Redis persistence settings should be selected according to the role of the
specific data.

Because the initial Redis design primarily contains ephemeral state, Redis
must not be treated as a disaster-recovery copy of PostgreSQL.

PostgreSQL backups are responsible for durable business recovery.

If Redis later contains durable coordination or stream data, its persistence
requirements must be reconsidered.

---

# 35. Security

Redis must not be exposed directly to the public internet.

Architecture:

```text
Internet
   ↓
API Gateway
   ↓
Go Application
   ↓
Redis
```

Redis should be accessible only from trusted application infrastructure.

Authentication, network restrictions, and encryption requirements will be
defined in the deployment design.

---

# 36. Initial Key Catalog

| Key | Purpose | TTL | Authoritative? |
|---|---|---|---|
| `driver:{id}:presence` | Driver operational presence | Short | No |
| `driver:{id}:location` | Latest driver location | Short | No |
| `ws:user:{id}:connections` | WebSocket connection metadata | Short | No |
| `ride:{id}:realtime` | Temporary ride real-time state | Short | No |
| `rate_limit:*` | Rate limiting | Algorithm-dependent | No |
| `bidding:deadlines` | Temporary deadline scheduling | Until processed | No |
| `assignment:deadlines` | Temporary confirmation scheduling | Until processed | No |
| `cache:*` | Read cache | Resource-dependent | No |

---

# 37. What Redis Must Never Become

Redis must not become the only place where the system knows:

```text
Who the rider is
Who owns a ride
Which driver won a bid
Which fare was agreed
Whether a trip was completed
Whether a ride was cancelled
Which bids exist
```

If losing Redis would permanently lose any of the above information, the
architecture is wrong.

---

# 38. Initial Architecture

```text
                       ┌─────────────────┐
                       │   PostgreSQL    │
                       │                 │
                       │ Durable Truth   │
                       └────────┬────────┘
                                │
                                │
                    ┌───────────▼───────────┐
                    │       Go Backend      │
                    │                       │
                    │ Domain / Application  │
                    └───────────┬───────────┘
                                │
                         ┌──────▼──────┐
                         │    Redis    │
                         │             │
                         │ Ephemeral   │
                         │ Operational │
                         └──────┬──────┘
                                │
                         ┌──────▼──────┐
                         │  WebSocket  │
                         │   Clients   │
                         └─────────────┘
```

---

# 39. Design Principles

1. PostgreSQL remains the source of truth.
2. Redis stores operational and ephemeral state.
3. Every ephemeral key has a TTL.
4. Redis failure must not destroy business state.
5. Redis locks do not replace PostgreSQL transactions.
6. Redis Pub/Sub does not provide durable event delivery.
7. Driver location is not persisted to PostgreSQL by default.
8. Presence is heartbeat/TTL based.
9. Cache is optional and measurable.
10. Geographic driver discovery remains an explicit future decision.
11. Redis must not become a hidden second database.
12. Durable asynchronous events should use an outbox-based architecture.
13. Redis is private infrastructure and must not be publicly exposed.
