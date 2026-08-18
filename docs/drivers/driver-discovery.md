# Driver Discovery

## 1. Purpose

This document defines how the platform turns a ride request into a bounded set
of eligible driver opportunities.

Discovery answers:

> Which drivers should receive the opportunity to bid on this ride?

Discovery does not select a driver and does not create bids.

The separation is:

```text
Discovery
  ↓
Who should get the opportunity?

Bidding
  ↓
What does the driver offer?

Selection
  ↓
Which offer does the rider choose?
```

---

# 2. Core Principles

1. Discovery is a candidate-generation process, not driver assignment.
2. Discovery should be asynchronous after the ride is committed.
3. Redis supports fast geographic and operational lookup.
4. PostgreSQL remains authoritative for durable business state.
5. Presence and location must be fresh enough for discovery.
6. Driver eligibility and vehicle eligibility are ride-specific.
7. Discovery produces a bounded candidate set.
8. Discovery ranking determines notification priority, not rider selection.
9. A discovered driver is not reserved.
10. A discovered driver is not guaranteed to remain available.
11. Bid creation revalidates relevant driver state.
12. Reservation creation revalidates driver state again.
13. Discovery must be safe to retry.
14. The system must distinguish no eligible drivers from eligible drivers who did not bid.
15. Google Maps should support geographic functionality without becoming a dependency for every candidate.
16. V1 should use deterministic, explainable ranking rather than premature machine learning.

---

# 3. Discovery Trigger

A ride entering the bidding state produces a discovery trigger.

Conceptually:

```text
POST /rides
    ↓
Ride committed
    ↓
outbox: ride.created / ride.bidding_started
    ↓
Discovery worker
```

The discovery workflow should not require the rider HTTP request to wait for
candidate generation.

---

# 4. Asynchronous Architecture

```text
                    Ride Created
                         │
                         ▼
                  Discovery Worker
                         │
                         ▼
                Candidate Generation
                         │
                         ▼
                  Driver Notification
                         │
                         ▼
                    Driver Bids
```

A worker may retry discovery when transient infrastructure failures occur.

---

# 5. Discovery Workflow

The initial conceptual workflow is:

```text
Ride
 ↓
Geographic candidate lookup
 ↓
Presence/location freshness
 ↓
Operational availability
 ↓
Driver eligibility
 ↓
Vehicle eligibility
 ↓
Ride compatibility
 ↓
Candidate ranking
 ↓
Bounded candidate set
 ↓
Driver notifications
```

Each stage reduces the candidate set.

---

# 6. Geographic Candidate Lookup

The first reduction should be geographic.

Conceptually:

```text
Pickup location
      ↓
nearby available drivers
      ↓
candidate set
```

Redis is appropriate for this low-latency lookup.

The initial design should use a configured search radius rather than an
unbounded geographic query.

---

# 7. Search Radius

The search radius should be configuration rather than a permanent hardcoded
business rule.

For example:

```text
initial radius
      ↓
insufficient candidates?
      ↓
expanded radius
      ↓
insufficient candidates?
      ↓
final configured radius
```

The actual distances will be tuned using operational data.

---

# 8. Progressive Radius Expansion

Progressive expansion avoids immediately notifying distant drivers.

Conceptually:

```text
Radius A
  ↓
enough candidates?
  ├── yes → continue
  └── no → Radius B
                 ↓
             enough?
              ├── yes → continue
              └── no → Radius C
```

The workflow should stop once sufficient candidates have been found or the
configured maximum radius has been reached.

---

# 9. Candidate Limit

Discovery should not notify every eligible driver in the surrounding city.

A bounded candidate set reduces:

```text
notification spam
driver fatigue
unnecessary bids
rider bid overload
infrastructure cost
```

The target candidate count should be configurable.

---

# 10. Presence Filter

Only drivers with sufficiently fresh presence should be considered.

Conceptually:

```text
presence fresh
    ↓
pass

presence stale
    ↓
exclude
```

A driver can remain a valid account while being temporarily absent from
discovery.

---

# 11. Location Freshness Filter

Only drivers with sufficiently fresh location should normally be considered for
nearby discovery.

Conceptually:

```text
location_age <= configured threshold
```

A stale location should not be treated as a current position.

---

# 12. Availability Filter

Discovery requires operational availability.

```text
ONLINE ≠ AVAILABLE
```

A driver currently reserved, assigned, or otherwise committed must be excluded
from new conflicting opportunities.

---

# 13. Driver Eligibility

Driver-level eligibility should include relevant current requirements such as:

```text
driver account active
driver verification complete
service authorization valid
no blocking suspension/restriction
```

The complete driver eligibility rules are a separate domain concern, but
discovery must consume their authoritative result.

---

# 14. Vehicle Eligibility

The driver's active vehicle must satisfy the ride's requirements.

Potential checks include:

```text
vehicle active
vehicle compliant
service category compatible
capacity sufficient
required features available
```

Example:

```text
Ride requires 5 seats
Vehicle has 4 seats
→ exclude candidate
```

---

# 15. Ride Compatibility

Discovery must evaluate:

```text
Driver
+
Vehicle
+
Ride
```

rather than simply finding nearby drivers.

Potential ride-specific constraints include:

```text
service type
passenger count
vehicle class
accessibility requirements
other future ride constraints
```

---

# 16. Candidate Exclusion

The initial exclusion list is:

```text
offline
presence stale
location stale
not available
reserved
assigned
conflicting active trip
ineligible driver
invalid vehicle
incompatible vehicle
```

The exact implementation should keep these rules testable and explicit.

---

# 17. Candidate Snapshot

Discovery operates on rapidly changing state.

A candidate is a snapshot:

```text
Driver A
location = accepted position at discovery time
availability = valid at discovery time
eligibility = valid at discovery time
```

It is not a guarantee of future availability.

For example:

```text
10:00 → Driver A discovered
10:01 → Driver A becomes reserved
10:02 → Driver A submits bid
```

Bid creation must revalidate current state.

---

# 18. Bid Revalidation

When a driver submits a bid:

```text
Discovery candidate
       ↓
Driver opens opportunity
       ↓
POST /bids
       ↓
backend revalidation
       ↓
accept/reject bid
```

Relevant checks include:

```text
ride still accepting bids
driver still eligible
driver still available
vehicle still valid
vehicle still compatible
```

Discovery does not bypass these checks.

---

# 19. Reservation Revalidation

When the rider selects a bid, reservation creation revalidates again.

Therefore:

```text
Discovery validation
        ↓
Bid validation
        ↓
Reservation validation
```

This is intentional because state may change between each stage.

---

# 20. Candidate Ranking

After filtering, candidates should be ranked before notification.

V1 should use deterministic, explainable factors.

Potential factors:

```text
distance to pickup
estimated arrival time where available
location freshness
operational quality signals where justified
```

The initial ranking should remain simple.

---

# 21. Ranking Is Not Driver Selection

A higher-ranked candidate does not become the selected driver.

Ranking only controls opportunity/notification priority.

The final decision remains:

```text
Driver submits bid
      ↓
Rider evaluates bids
      ↓
Rider selects bid
```

---

# 22. Distance

Geographic distance is an appropriate initial ranking/filtering signal.

It is cheap to calculate and works without external routing requests for every
candidate.

Distance should normally be measured from the candidate's accepted current
location to the ride pickup location.

---

# 23. ETA

Distance and ETA are not equivalent.

Example:

```text
Driver A: 1.5 km / 8 minutes
Driver B: 2.0 km / 5 minutes
```

Traffic and road topology can make the farther driver faster.

ETA can therefore become a useful ranking signal later.

---

# 24. Google Maps Boundary

Google Maps may provide:

```text
routing
distance
ETA
geocoding
```

But discovery should not synchronously call Google Maps for every candidate in
V1.

That would introduce:

```text
external latency
external failure dependency
API cost
rate-limit pressure
```

into the hottest dispatch path.

---

# 25. Initial Geographic Strategy

V1 recommendation:

```text
Redis spatial lookup
      ↓
cheap geographic filtering
      ↓
deterministic candidate ranking
```

Use routing/ETA selectively where the additional accuracy is justified.

---

# 26. Notification Strategy

The initial design should notify a bounded set of candidates rather than every
eligible driver.

Conceptually:

```text
rank candidates
      ↓
select bounded set
      ↓
notify
```

Progressive expansion can be used if the initial candidate set produces
insufficient bids.

---

# 27. Candidate Batches

A future optimization may use batches:

```text
Candidate batch 1
      ↓
observe bids
      ↓
insufficient bids?
      ↓
Candidate batch 2
```

The initial design should support bounded discovery without requiring a complex
auction/round system.

---

# 28. No Automatic Bid

Discovery must never create a bid on behalf of a driver.

The driver explicitly chooses:

```text
submit bid
```

or does nothing.

---

# 29. No Driver-to-Driver Visibility

Discovery should not expose the candidate list to drivers.

A driver sees their own opportunity, not:

```text
other candidates
other drivers' locations
other drivers' ranking
other drivers' bids
```

---

# 30. Discovery Deadline

The ride's bidding lifecycle should have a configured deadline.

Conceptually:

```text
Ride enters BIDDING
      ↓
discovery attempts
      ↓
bidding remains open
      ↓
deadline reached
      ↓
stop new discovery
      ↓
close bidding/fallback
```

The exact timeout is product configuration.

---

# 31. No Eligible Drivers

The system should distinguish:

```text
NO_ELIGIBLE_DRIVERS
```

from:

```text
ELIGIBLE_DRIVERS_FOUND_NO_BIDS
```

These represent different marketplace outcomes.

---

# 32. No Eligible Drivers

If geographic expansion reaches the configured maximum and no eligible
candidate exists:

```text
candidate search exhausted
      ↓
NO_ELIGIBLE_DRIVERS
      ↓
ride fallback policy
```

The ride should not remain indefinitely in bidding.

---

# 33. Eligible Drivers but No Bids

A different outcome is:

```text
eligible candidates found
      ↓
drivers notified
      ↓
no bids
      ↓
bidding deadline
```

This should remain distinguishable from having no eligible drivers.

It may indicate future marketplace/product problems such as unattractive
pricing or poor opportunity quality.

---

# 34. Discovery Exhaustion

Discovery is exhausted when:

```text
maximum configured search radius reached
AND
candidate expansion has no viable additional candidates
```

or when the ride's bidding deadline has been reached.

The workflow must stop generating new opportunities after the ride closes.

---

# 35. Discovery Idempotency

The same ride may trigger discovery more than once because of:

```text
worker retry
outbox retry
process crash
network failure
```

Discovery must therefore be safe to execute repeatedly.

It must not create duplicate logical candidate opportunities or uncontrolled
notification storms.

---

# 36. Concurrent Discovery

Multiple workers may attempt to process the same ride.

The implementation must provide either:

```text
one active discovery workflow per ride
```

or a design where concurrent processing is safely idempotent.

The exact locking/coordination mechanism is an implementation decision.

---

# 37. Discovery and Ride Closure

A discovery worker may be processing candidates while the rider selects a bid
or cancels the ride.

Before notifying a candidate, the worker should verify that the ride is still
open for discovery.

A stale discovery worker must not create opportunities for a closed ride.

---

# 38. Discovery and Reservation Race

A driver may be discovered while another workflow reserves that driver.

Example:

```text
Discovery sees Driver A
      ↓
Driver A reserved elsewhere
      ↓
Driver A receives stale opportunity
```

This is acceptable if bid creation revalidates availability.

The system must not attempt to maintain a perfect instantaneous candidate
snapshot.

---

# 39. Redis Responsibilities

Redis may provide:

```text
current location
presence freshness
operational availability
spatial lookup
fast candidate filtering
```

Redis is an operational acceleration layer.

---

# 40. PostgreSQL Responsibilities

PostgreSQL remains authoritative for:

```text
driver account
eligibility
vehicle records
reservation
assignment
ride lifecycle
bid state
```

Discovery may read durable information as necessary, but high-frequency
candidate location lookup should not require scanning PostgreSQL.

---

# 41. Redis Failure

If Redis cannot provide trustworthy location/presence information:

```text
Discovery cannot safely establish current candidates
```

The system should fail safely, retry, or follow a defined degradation policy.

It should not fabricate candidate availability from stale data.

---

# 42. PostgreSQL Failure

If PostgreSQL is unavailable, discovery should not claim successful durable ride
state transitions.

The worker can retry after database recovery.

---

# 43. Notification Delivery

Candidate notification should be based on durable discovery decisions where
appropriate and should tolerate duplicate delivery.

The driver application must treat the notification as an opportunity, not an
assignment.

---

# 44. Notification Event

A conceptual event is:

```text
ride.opportunity.created
```

Potential payload:

```json
{
  "ride_id": "ride_123",
  "pickup": {
    "latitude": 31.5204,
    "longitude": 74.3587
  },
  "service_type": "standard",
  "expires_at": "2026-08-18T10:32:00Z"
}
```

The final payload should minimize unnecessary rider/private data exposure.

---

# 45. Opportunity Expiration

A driver may receive an opportunity after discovery but open it later.

The driver-facing API must verify that:

```text
ride still accepts bids
opportunity is still valid
```

The client notification itself is not authoritative.

---

# 46. Candidate Ranking Explainability

V1 ranking should be explainable enough to debug.

For example:

```text
Driver A
  distance: 1.2 km
  location freshness: 2 seconds

Driver B
  distance: 2.1 km
  location freshness: 1 second
```

Avoid opaque scoring systems until there is evidence that simple ranking is
insufficient.

---

# 47. Ranking and Driver Quality

Driver quality signals may eventually influence ranking.

Potential examples:

```text
service reliability
cancellation rate
acceptance/bidding behavior
completed-trip history
```

Do not introduce these into V1 without a clear product reason and a defensible
policy.

Ranking affects opportunity distribution and therefore can materially affect
driver earnings, so it must remain observable and reviewable.

---

# 48. Discovery Fairness

Repeatedly favoring the same drivers can create marketplace imbalance.

V1 should prioritize ride suitability and proximity, but the architecture should
leave room for future fairness/marketplace constraints.

Do not introduce arbitrary fairness penalties before we have data showing a
problem.

---

# 49. Candidate Deduplication

A driver must appear at most once in the candidate set for a given discovery
attempt.

If multiple data sources produce the same driver:

```text
Driver A
Driver A
Driver A
```

collapse to one candidate.

---

# 50. Candidate Ordering Stability

Where candidates have equivalent ranking values, use a deterministic tie-breaker
or stable ordering.

This makes behavior easier to test and debug.

---

# 51. Security and Authorization

Discovery is a backend-controlled operation.

Drivers must not be able to request arbitrary candidate lists.

Riders must not be able to retrieve the platform's full nearby-driver inventory.

Candidate information is internal operational data.

---

# 52. Privacy

Do not expose candidate driver identities or exact locations to unrelated users.

The driver receives only the ride opportunity information required to decide
whether to bid.

The rider receives bids, not the platform's internal candidate pool.

---

# 53. Observability

Useful metrics include:

```text
discovery_started_total
discovery_completed_total
discovery_failed_total
discovery_candidates_total
discovery_eligible_candidates_total
discovery_notified_total
discovery_no_candidates_total
discovery_no_bids_total
discovery_latency
```

Useful tracing/logging fields include:

```text
request_id
ride_id
discovery_id
candidate_count
radius
```

Avoid high-cardinality driver IDs as metric labels.

---

# 54. Discovery State

The discovery workflow may need an internal operational state separate from the
ride state.

Conceptually:

```text
NOT_STARTED
RUNNING
EXHAUSTED
COMPLETED
CANCELLED
FAILED_RETRYABLE
```

The exact persistence model is an implementation decision.

Do not unnecessarily expose discovery internals as rider-facing ride states.

---

# 55. API Boundary

Discovery itself should initially be an internal service/workflow rather than a
public client API.

Public clients interact with:

```text
Ride APIs
Bid APIs
Reservation APIs
```

The discovery worker operates behind those boundaries.

---

# 56. Complete Discovery Flow

```text
                     RIDE CREATED
                          │
                          ▼
                    ENTER BIDDING
                          │
                          ▼
                  DISCOVERY WORKER
                          │
                          ▼
                REDIS SPATIAL LOOKUP
                          │
                          ▼
                 NEARBY DRIVER SET
                          │
             ┌────────────┴────────────┐
             ▼                         ▼
      Presence/Freshness          Availability
             │                         │
             └────────────┬────────────┘
                          ▼
                     ELIGIBILITY
                          │
                          ▼
                  VEHICLE COMPATIBILITY
                          │
                          ▼
                       RANKING
                          │
                          ▼
                  BOUNDED CANDIDATES
                          │
                          ▼
                 DRIVER NOTIFICATION
                          │
                          ▼
                     DRIVER BIDS
                          │
                          ▼
                    RIDER SELECTS
                          │
                          ▼
                     RESERVATION
```

---

# 57. What We Should Not Build Yet

Do not build:

```text
machine-learning candidate ranking
complex auction rounds
full-city candidate notification
Google Maps routing calls for every candidate
custom geospatial engine
automatic bid generation
opaque candidate scoring
complex fairness optimization
real-time driver-to-driver competition
```

Start with deterministic geographic filtering, eligibility, compatibility, and
bounded ranking.

---

# 58. Design Principles

1. Discovery determines who receives an opportunity; it does not select the driver.
2. Discovery runs asynchronously after the ride enters bidding.
3. Geographic lookup should be the first major candidate reduction.
4. Redis is appropriate for low-latency spatial and operational state.
5. Presence and location freshness are mandatory candidate considerations.
6. Online status alone does not make a driver discoverable.
7. Driver eligibility and vehicle eligibility are ride-specific.
8. Candidate sets must be bounded.
9. Ranking controls notification priority, not rider selection.
10. V1 ranking should be deterministic and explainable.
11. Discovery candidates are snapshots, not availability guarantees.
12. Bid creation revalidates candidate state.
13. Reservation creation revalidates candidate state again.
14. Discovery must distinguish no eligible drivers from no bids.
15. Discovery must stop when the ride closes or its configured deadline is reached.
16. Discovery must be safe to retry and safe against concurrent workers.
17. Candidate notifications must tolerate duplicate delivery.
18. Redis failure must not result in fabricated availability.
19. Google Maps should not sit in the synchronous hot path for every candidate.
20. Candidate identity and location are internal/private operational information.
21. Discovery should remain simpler than the bidding and reservation domains.
22. Add sophisticated ranking only when production data justifies it.
