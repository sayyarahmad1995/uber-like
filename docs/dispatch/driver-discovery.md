# Driver Discovery

## 1. Purpose

This document defines how the dispatch system discovers nearby drivers for a
ride bidding opportunity.

Discovery answers:

> Which potentially eligible drivers should receive this ride opportunity?

Discovery is separate from eligibility.

```text
Eligibility
    ↓
Can this driver participate?

Discovery
    ↓
Which candidates should we contact?
```

Discovery must not become the authority for business eligibility.

---

# 2. Core Principle

Driver location is high-frequency, ephemeral operational data.

Therefore the initial architecture uses Redis for live geographic discovery.

```text
Driver location
      ↓
Redis GEO
      ↓
Nearby driver IDs
      ↓
Go eligibility validation
      ↓
Candidate ranking
      ↓
Dispatch batch
```

PostgreSQL remains authoritative for:

- Driver identity
- Driver approval
- Vehicle ownership
- Vehicle status
- Durable availability state
- Ride state
- Bid state
- Assignment state

Redis is used for:

- Current driver location
- Presence
- Fast geographic lookup
- Operational dispatch state
- Real-time coordination

---

# 3. Why Redis GEO

We already use Redis for real-time operational state.

Driver location changes frequently.

Writing every GPS update into PostgreSQL would create unnecessary database
load and would turn PostgreSQL into a high-frequency location store.

Instead:

```text
Flutter
   ↓
WebSocket
   ↓
Go
   ↓
Redis
```

stores the driver's current operational location.

Redis can then efficiently answer:

```text
Which drivers are within radius R of this pickup location?
```

---

# 4. PostgreSQL vs Redis

The division is intentional.

| Data | PostgreSQL | Redis |
|---|---|---|
| Driver profile | Authoritative | Optional cache |
| Driver approval | Authoritative | Optional cache |
| Vehicle | Authoritative | Optional cache |
| Driver availability | Authoritative | Operational mirror |
| Current location | Not primary | Primary operational store |
| Location freshness | Historical/audit if needed | Primary |
| Presence | No | Primary |
| Ride state | Authoritative | Optional cache |
| Bids | Authoritative | Optional operational cache |
| Assignment | Authoritative | Coordination/cache |

Redis failure must not corrupt durable business state.

---

# 5. Redis GEO Structure

The initial design uses a Redis sorted set with geographic coordinates.

Conceptually:

```text
dispatch:drivers:location
```

Each member is a driver ID.

Example:

```text
dispatch:drivers:location

driver_101 → latitude/longitude
driver_102 → latitude/longitude
driver_103 → latitude/longitude
```

Redis maintains the geospatial index.

The driver ID is the member.

---

# 6. Updating Driver Location

When a driver sends a location update:

```text
Flutter
   ↓
WebSocket
   ↓
Go
   ↓
validate driver
   ↓
Redis GEOADD
```

Conceptually:

```text
GEOADD dispatch:drivers:location
       longitude
       latitude
       driver_id
```

The backend should also maintain a location timestamp.

For example:

```text
driver:{driver_id}:location
```

may contain:

```json
{
  "latitude": 33.6844,
  "longitude": 73.0479,
  "accuracy_meters": 8,
  "heading": 120,
  "speed_mps": 11.4,
  "recorded_at": "2026-08-18T10:02:00Z"
}
```

The GEO index alone does not provide sufficient freshness information.

---

# 7. Location Freshness

A driver can appear geographically close while their location is stale.

Example:

```text
10:00:00
Driver location = 33.6844, 73.0479

10:10:00
No updates

10:10:01
Ride requested nearby
```

The driver should not automatically be treated as a valid candidate.

Discovery must apply a location freshness requirement.

Conceptually:

```text
current_time - recorded_at <= freshness_threshold
```

The exact threshold is configuration and should be tuned using real-world
data.

---

# 8. Presence

Location alone is insufficient.

A driver may have a recent location but no active connection.

Redis presence is therefore maintained separately.

Conceptually:

```text
driver:{driver_id}:presence
```

with a TTL.

The driver is considered operationally reachable only when the presence is
valid.

Therefore:

```text
GEO location
    +
fresh location
    +
active presence
```

form the initial operational discovery requirements.

---

# 9. Driver Online State

The durable driver availability state is stored in PostgreSQL.

Example:

```text
driver_profiles.availability_status = ONLINE
```

Redis presence does not replace this state.

A driver is a discovery candidate only when the relevant conditions are
satisfied:

```text
PostgreSQL:
ONLINE

Redis:
present

Redis:
fresh location
```

---

# 10. Discovery Request

When a ride enters the bidding phase:

```text
Ride
  ↓
BIDDING
  ↓
Dispatch
  ↓
Driver discovery
```

The dispatch service receives:

```text
ride_id
pickup latitude
pickup longitude
service type
```

The pickup coordinates are the center of the initial geographic search.

---

# 11. Geographic Search

The first discovery operation performs a radius search around the pickup.

Conceptually:

```text
pickup
   ●
  /|\
 / | \
driver candidates
```

Example:

```text
pickup:
33.6844, 73.0479

radius:
5 km
```

Redis returns driver IDs within the configured radius.

Important:

```text
nearby
    ≠
eligible
```

The result is only a candidate set.

---

# 12. Discovery Does Not Trust Redis

Redis may contain stale or inconsistent operational information.

Therefore the discovery process must not blindly dispatch a ride to every ID
returned by the GEO query.

Example:

```text
Redis returns:

driver_101
driver_102
driver_103
driver_104
```

Go then evaluates:

```text
driver_101 → eligible
driver_102 → suspended
driver_103 → stale location
driver_104 → wrong vehicle
```

Only:

```text
driver_101
```

continues.

---

# 13. Discovery Pipeline

The complete initial flow is:

```text
Ride enters BIDDING
        ↓
Read pickup coordinates
        ↓
Redis GEO radius search
        ↓
Candidate driver IDs
        ↓
Check presence
        ↓
Check location freshness
        ↓
Check durable eligibility
        ↓
Check service compatibility
        ↓
Remove committed/unavailable drivers
        ↓
Rank candidates
        ↓
Select bounded batch
        ↓
Create dispatch opportunity
        ↓
Notify drivers
```

---

# 14. Candidate Limit

The system should not send every nearby driver a ride opportunity.

Suppose:

```text
1,000 drivers
```

are within the radius.

Sending the ride to all 1,000 drivers would create unnecessary:

- WebSocket traffic
- Notifications
- Bid records
- Database writes
- Rider decision complexity

The discovery process therefore selects a bounded candidate batch.

The exact batch size remains configurable.

---

# 15. Initial Batch Strategy

The initial implementation should favor simplicity.

Example:

```text
Nearby candidates
      ↓
Eligibility filtering
      ↓
Ranking
      ↓
Take first N
```

Where:

```text
N = configurable candidate count
```

Do not build a complex adaptive dispatch algorithm before observing real
system behavior.

---

# 16. Candidate Ranking

Discovery returns candidates.

Ranking decides their order.

Initial ranking can use simple deterministic factors:

```text
1. Distance to pickup
2. Location freshness
3. Driver availability quality
```

The first implementation should avoid machine learning or complex scoring.

For example:

```text
Driver A → 1.2 km
Driver B → 1.8 km
Driver C → 2.1 km
Driver D → 4.7 km
```

could produce:

```text
A
B
C
D
```

before applying the candidate limit.

---

# 17. Why Distance Alone Is Not Enough

Two drivers can be equally close while having very different operational
quality.

Example:

```text
Driver A
distance = 1.0 km
location age = 1 second

Driver B
distance = 0.9 km
location age = 90 seconds
```

Driver A may be the better candidate despite being slightly farther away.

However, the initial ranking should remain simple.

---

# 18. Discovery Radius

The radius should be configurable.

Conceptually:

```text
dispatch.discovery.initial_radius
```

The exact value is intentionally not fixed in this document.

The correct radius depends on:

- City density
- Traffic
- Driver density
- Service type
- Historical acceptance
- Pickup wait expectations

These should be measured rather than guessed permanently.

---

# 19. Radius Expansion

If insufficient candidates are found, the system may later expand the radius.

Example:

```text
5 km
 ↓
not enough candidates
 ↓
8 km
 ↓
not enough candidates
 ↓
12 km
```

However, the initial implementation should not automatically implement an
aggressive expansion algorithm.

Start with:

```text
one configured radius
```

and add controlled expansion once the basic system works.

---

# 20. Candidate Exhaustion

A ride may have fewer candidates than the desired batch size.

Example:

```text
required candidates = 5

eligible candidates = 2
```

The system should dispatch to the 2 available candidates.

It should not invent additional candidates.

The ride can then proceed according to the bidding lifecycle.

---

# 21. Empty Candidate Set

If no eligible drivers are discovered:

```text
Ride
  ↓
BIDDING
  ↓
Discovery
  ↓
0 candidates
```

The system must define a product-level response.

Initial recommendation:

```text
retry discovery
```

after a short controlled interval.

If repeated discovery produces no candidates, the ride should transition to
an appropriate failure/timeout state.

The exact timeout policy belongs to the ride lifecycle design.

---

# 22. Repeated Discovery

Repeated discovery must be bounded.

Bad design:

```text
while no driver:
    search again
```

This can create an uncontrolled loop and unnecessary Redis load.

Instead:

```text
attempt 1
   ↓
wait
   ↓
attempt 2
   ↓
wait
   ↓
attempt 3
   ↓
stop / transition
```

The maximum number of attempts and delays are configuration.

---

# 23. Dispatch Batch

Once candidates are selected, the backend creates a dispatch opportunity.

Conceptually:

```text
Ride 123
   ↓
Dispatch Batch 456
   ├── Driver A
   ├── Driver B
   └── Driver C
```

This gives us an identifiable unit for:

- Logging
- Metrics
- Debugging
- Candidate tracking
- Bid analysis

The exact persistence model will be defined in the database design.

---

# 24. Driver Notification

Selected drivers receive the opportunity through WebSocket.

Conceptually:

```text
Dispatch Batch
      ↓
Driver A WebSocket
Driver B WebSocket
Driver C WebSocket
```

Example event:

```json
{
  "id": "evt_300",
  "type": "ride.bidding_started",
  "version": 1,
  "timestamp": "2026-08-18T10:00:00Z",
  "data": {
    "ride_id": "ride_123",
    "ends_at": "2026-08-18T10:00:30Z"
  }
}
```

The driver receives only information they are authorized to see.

---

# 25. Notification Failure

A driver may be selected but fail to receive the WebSocket notification.

Examples:

```text
WebSocket disconnected
mobile network lost
application suspended
server restart
```

Discovery should not treat WebSocket delivery as proof that the driver
received the opportunity.

The driver must have a way to retrieve active opportunities through REST
after reconnecting.

---

# 26. Active Opportunity Recovery

After reconnecting, a driver may request:

```text
GET /api/v1/me/driver/opportunities
```

The response contains currently valid bidding opportunities.

This prevents WebSocket loss from permanently hiding a ride opportunity.

---

# 27. Bid Submission

Discovery does not reserve the driver.

The driver may submit a bid only if the bid-time validation succeeds.

```text
Driver receives opportunity
        ↓
Driver submits bid
        ↓
Go validates current eligibility
        ↓
Validate ride still accepts bids
        ↓
Create/update bid
```

This protects against stale discovery results.

---

# 28. Multiple Rides

The initial design may allow a driver to bid on multiple rides.

Example:

```text
Driver A
   ├── Bid on Ride 1
   ├── Bid on Ride 2
   └── Bid on Ride 3
```

A bid is not a commitment.

This is useful for maintaining driver utilization when rider demand is high.

However, the assignment system must prevent conflicting confirmed assignments.

---

# 29. Temporary Reservation

When a rider selects a driver's bid:

```text
Rider
  ↓
select bid
  ↓
Driver Selected
```

the system should create a short-lived assignment reservation.

Conceptually:

```text
Driver X
    ↓
RESERVED FOR RIDE A
```

The reservation prevents another ride from successfully reserving the same
driver during the confirmation window.

---

# 30. Reservation Lifecycle

Recommended initial lifecycle:

```text
BID
   ↓
RIDER SELECTS
   ↓
DRIVER_RESERVED
   ↓
confirmation deadline
   ├── CONFIRMED → COMMITTED
   ├── REJECTED  → RELEASED
   └── EXPIRED   → RELEASED
```

This is separate from the original bid.

The bid represents the driver's offer.

The reservation represents temporary assignment ownership.

---

# 31. Reservation Authority

The reservation must be authoritative in PostgreSQL.

Redis may be used for fast coordination or expiry handling, but a Redis-only
lock is insufficient.

The system must remain correct if Redis becomes unavailable.

Conceptually:

```text
PostgreSQL
    ↓
reservation state
```

Redis:

```text
operational acceleration
```

not:

```text
business truth
```

---

# 32. Concurrent Reservation

Consider:

```text
Ride A selects Driver X
Ride B selects Driver X
```

Two requests execute concurrently.

The database must guarantee:

```text
only one reservation succeeds
```

Conceptually:

```text
Transaction A
    ↓
reserve Driver X
    ↓
COMMIT

Transaction B
    ↓
attempt reserve Driver X
    ↓
conflict
    ↓
FAIL
```

The exact PostgreSQL constraint/locking strategy belongs in the assignment
design.

---

# 33. Why We Need a Reservation

Without a reservation:

```text
Ride A selects Driver X
Ride B selects Driver X
```

Both could send:

```text
assignment.confirmation_required
```

to Driver X.

The driver would receive two competing assignment requests.

That creates an unnecessary race at the user interface level.

The reservation moves the race into the backend, where it can be controlled.

---

# 34. Reservation Expiration

The reservation must have an expiration timestamp.

Example:

```text
reserved_at
expires_at
```

The driver must confirm before:

```text
expires_at
```

The client countdown is informational.

The backend timestamp is authoritative.

---

# 35. Expiration Processing

The reservation may expire because:

```text
driver rejects
driver does not respond
network failure
confirmation timeout
```

The backend then releases the reservation.

Possible flow:

```text
DRIVER_RESERVED
       ↓
expiration reached
       ↓
reservation released
       ↓
ride returns to assignment selection
```

The exact fallback behavior belongs to the assignment design.

---

# 36. Redis Role in Reservation

Redis may maintain an operational representation such as:

```text
dispatch:reservation:{driver_id}
```

with a TTL.

This can help with:

- Fast checks
- Expiration detection
- Operational visibility

But correctness must still be enforced by PostgreSQL.

If Redis says:

```text
driver is free
```

while PostgreSQL says:

```text
driver is reserved
```

PostgreSQL wins.

---

# 37. Location Removal

When a driver goes offline:

```text
PostgreSQL → OFFLINE
```

the driver should be removed from the active discovery GEO set or otherwise
excluded from discovery.

Conceptually:

```text
ZREM dispatch:drivers:location driver_123
```

However, cleanup must be defensive.

A stale GEO member must not cause a correctness problem because discovery
still validates:

```text
availability
presence
location freshness
eligibility
```

---

# 38. Presence Expiration

Redis presence uses TTL.

When the TTL expires:

```text
driver presence
      ↓
expired
```

the driver should no longer be considered operationally reachable.

The GEO entry may remain temporarily.

This is acceptable because discovery applies freshness/presence filtering.

A background cleanup process may remove stale GEO members later.

---

# 39. Location Update Frequency

Location updates should be rate-controlled.

Sending GPS coordinates every few milliseconds is unnecessary and harmful.

The Flutter implementation should use sensible movement/time thresholds.

The backend should also protect itself with rate limits.

The exact frequency should be determined through performance testing and
real-world behavior.

---

# 40. Location During Active Ride

The driver's location becomes especially important after assignment.

The same Redis operational location can serve:

```text
Pre-assignment discovery
+
Post-assignment live tracking
```

But authorization changes.

Before assignment:

```text
location → discovery infrastructure
```

After assignment:

```text
location → authorized rider
```

The backend controls who receives the location.

---

# 41. Geospatial Accuracy

Redis GEO gives us a fast geographic index.

It does not solve:

- Road distance
- Actual travel time
- Traffic
- Route restrictions
- Pickup ETA

Therefore:

```text
geographic distance
    ≠
driving distance
    ≠
ETA
```

Initial discovery should use geographic proximity.

Google Maps/Routes functionality can later provide richer ETA/routing signals.

Do not call Google Maps for every candidate during the first discovery stage.

That would increase latency and external API cost unnecessarily.

---

# 42. Google Maps Role

Google Maps should primarily handle:

- Map rendering
- Geocoding where needed
- Route calculations
- ETA
- Navigation-related functionality

Redis handles:

```text
"Which drivers are geographically nearby right now?"
```

Google Maps handles:

```text
"How long would it actually take to travel there?"
```

These are different questions.

---

# 43. Discovery Failure

If Redis is unavailable:

```text
Redis GEO unavailable
```

the system cannot perform normal fast driver discovery.

The system should not silently treat all drivers as eligible.

Possible behavior:

```text
discovery unavailable
      ↓
retry with controlled backoff
      ↓
if persistent
      ↓
fail/timeout according to ride policy
```

A fallback to a full PostgreSQL driver scan should not be the default.

That would create unpredictable database load during exactly the kind of
incident when another infrastructure component is already failing.

---

# 44. Scaling

Redis GEO discovery is compatible with multiple Go backend instances.

```text
                    ┌── Go A
API Gateway ────────┼── Go B
                    └── Go C
                         │
                         ▼
                       Redis
```

All instances query the same operational location index.

PostgreSQL continues to hold durable state.

---

# 45. Discovery Observability

Useful metrics:

```text
discovery_requests_total
discovery_success_total
discovery_empty_total
discovery_latency
candidates_found
candidates_after_eligibility
candidates_dispatched
candidate_rejection_count
location_stale_count
presence_missing_count
```

Useful dimensions include:

```text
service_type
city/region
discovery_radius
```

Avoid high-cardinality labels such as:

```text
driver_id
ride_id
```

in metrics.

Those belong in logs/traces.

---

# 46. Dispatch Trace

A ride should be traceable through the discovery pipeline.

Example:

```text
ride_123
   ↓
discovery_456
   ↓
Redis search
   ↓
100 candidates
   ↓
42 eligible
   ↓
ranked
   ↓
5 dispatched
   ↓
3 bids
   ↓
1 selected
   ↓
1 reservation
```

This will be extremely useful when debugging real-world dispatch behavior.

---

# 47. Initial Architecture

```text
                         PostgreSQL
                    durable driver state
                              │
                              │
                              ▼
Ride ───────────────→ Dispatch Service
                              │
                              ▼
                        Redis GEO
                              │
                    nearby driver IDs
                              │
                              ▼
                       Eligibility
                              │
                              ▼
                         Ranking
                              │
                              ▼
                     Candidate Batch
                              │
                              ▼
                         WebSocket
                              │
                ┌─────────────┼─────────────┐
                ▼             ▼             ▼
             Driver A      Driver B      Driver C
                │             │             │
                └─────────────┼─────────────┘
                              ▼
                             Bids
                              │
                              ▼
                       Rider selection
                              │
                              ▼
                       Reservation
                              │
                              ▼
                         Assignment
```

---

# 48. What We Should Not Build Yet

Do not build these in the first implementation:

```text
Machine-learning dispatch
Predictive driver acceptance
Complex driver scoring
Dynamic surge-aware ranking
Traffic-aware ranking for every candidate
Full PostGIS dispatch engine
Sophisticated fraud detection
Global driver optimization
Auction theory optimization
```

These are optimization problems.

First prove:

```text
ride requested
→ drivers discovered
→ drivers bid
→ rider selects
→ driver confirms
```

works reliably.

---

# 49. Design Principles

1. Discovery and eligibility are separate concerns.
2. Redis GEO is the initial live geographic discovery mechanism.
3. PostgreSQL remains the durable source of truth.
4. Redis location is operational and ephemeral.
5. Redis presence is separate from durable driver availability.
6. Location freshness must be validated.
7. Proximity does not establish eligibility.
8. Discovery returns candidates; Go applies business rules.
9. Candidate count is bounded.
10. Ranking should initially remain simple.
11. Discovery should not call Google Maps for every candidate.
12. Geographic distance is not equivalent to driving ETA.
13. WebSocket delivers opportunities but does not establish business state.
14. Bid submission revalidates eligibility.
15. Bid selection creates a temporary driver reservation.
16. Reservation correctness belongs to PostgreSQL.
17. Redis may accelerate reservation handling but cannot be the sole authority.
18. Reservation expiration must be server-authoritative.
19. Stale Redis GEO entries must not create correctness problems.
20. Redis failure must not corrupt durable ride or assignment state.
21. Discovery retries must be bounded.
22. Discovery must be observable end-to-end.
23. Complex dispatch optimization is deferred until the basic flow is proven.
