# Driver Eligibility

## 1. Purpose

This document defines the rules determining whether a driver is eligible to
participate in a ride's bidding process.

Eligibility answers:

> Can this driver receive and submit a bid for this ride?

Eligibility is separate from driver discovery.

```text
Eligibility
    ↓
Is the driver allowed?

Discovery
    ↓
Which eligible drivers should be contacted?
```

A driver may be eligible but not selected for discovery because the driver is:

- Too far away
- Outside the discovery radius
- Already handling another ride
- Not part of the current dispatch batch
- Lower priority than other eligible drivers

---

# 2. Core Principle

Driver eligibility is a business rule.

It must not depend solely on Redis.

The authoritative eligibility checks must be performed by the Go application
using authoritative data from PostgreSQL where necessary.

Redis may provide fast operational information, but Redis is not the final
authority for whether a driver is eligible.

---

# 3. Eligibility Pipeline

A driver's eligibility should be evaluated in stages.

```text
Driver candidate
      ↓
Authenticated?
      ↓
Approved?
      ↓
Online?
      ↓
Vehicle eligible?
      ↓
Service eligible?
      ↓
Not already committed?
      ↓
Operationally reachable?
      ↓
Eligible
```

A driver failing any required condition is excluded.

---

# 4. Driver Must Exist

The driver must have a valid:

```text
driver_profiles
```

record.

The driver profile must belong to the authenticated platform user.

The backend must never trust:

```text
driver_id
```

supplied by the client without validating ownership/authorization.

---

# 5. Driver Approval

The driver must have an approved status.

Initial approval states:

```text
PENDING
APPROVED
SUSPENDED
REJECTED
```

Only:

```text
APPROVED
```

drivers are eligible for normal ride bidding.

Therefore:

```text
PENDING   → not eligible
APPROVED  → potentially eligible
SUSPENDED → not eligible
REJECTED  → not eligible
```

---

# 6. Driver Availability

The driver's durable availability state is stored in PostgreSQL.

Initial states:

```text
OFFLINE
ONLINE
```

Only:

```text
ONLINE
```

drivers may participate in normal bidding.

However, `ONLINE` alone does not guarantee eligibility.

A driver may be online but temporarily unavailable because of an existing
assignment.

---

# 7. Operational Presence

The driver should also have an active operational presence.

Redis maintains ephemeral presence information.

Conceptually:

```text
driver:{driver_id}:presence
```

The presence record has a TTL.

A driver may therefore have:

```text
PostgreSQL:
ONLINE

Redis:
presence expired
```

In this situation the driver should not be considered currently reachable
for a new bidding opportunity.

This distinction is intentional:

```text
ONLINE
    =
driver intends to be available

PRESENT
    =
driver is currently connected/reachable
```

Both may be required for discovery.

---

# 8. Vehicle Eligibility

A driver must have an eligible vehicle.

The vehicle must:

- Belong to the driver
- Be active
- Be approved/eligible
- Match the requested service type

Example:

```text
Ride:
service_type = STANDARD

Driver:
vehicle_type = STANDARD
status = ACTIVE

→ eligible
```

But:

```text
Ride:
service_type = PREMIUM

Driver:
vehicle_type = STANDARD

→ not eligible
```

The exact vehicle/service compatibility matrix will be defined separately.

---

# 9. Active Vehicle

A driver may have multiple vehicles.

Only the driver's selected/active vehicle should be considered for a new ride.

Example:

```text
Driver
 ├── Vehicle A → inactive
 ├── Vehicle B → active
 └── Vehicle C → inactive
```

The ride should be associated with:

```text
Vehicle B
```

if the driver wins the bid.

The backend must verify that:

```text
vehicle.driver_id = driver.id
```

before accepting the bid.

---

# 10. Vehicle Status

Initial vehicle states may include:

```text
ACTIVE
INACTIVE
SUSPENDED
```

Only:

```text
ACTIVE
```

vehicles are eligible.

Historical rides retain their original vehicle relationship even if the vehicle
later becomes inactive.

---

# 11. Service Compatibility

A ride has a requested service type.

Example:

```text
STANDARD
PREMIUM
XL
```

The exact service catalog is deferred.

The eligibility layer should evaluate:

```text
ride.service_type
        ↓
vehicle capabilities
        ↓
compatible?
```

The driver should not be able to bid with a vehicle that does not satisfy
the ride's requirements.

---

# 12. Driver Commitment

A driver must not be simultaneously committed to conflicting rides.

This is one of the most important invariants in the system.

Invalid state:

```text
Driver X
   ├── Ride A → DRIVER_CONFIRMED
   └── Ride B → DRIVER_CONFIRMED
```

The system must prevent this.

---

# 13. Bidding vs Commitment

A critical distinction:

```text
Bid
    ≠
Assignment
```

A driver may potentially submit bids for multiple rides if the product rules
allow it.

But once the driver becomes committed to a ride, conflicting assignments
must be prevented.

Therefore:

```text
BIDDING
    ↓
not necessarily committed

DRIVER_SELECTED
    ↓
assignment pending

DRIVER_CONFIRMED
    ↓
committed
```

The exact point at which the driver becomes unavailable for other rides must
be explicitly defined.

---

# 14. Recommended Commitment Rule

The initial design should treat a driver as committed when the assignment is
successfully confirmed.

```text
BID
   ↓
DRIVER_SELECTED
   ↓
DRIVER_CONFIRMATION_REQUIRED
   ↓
DRIVER_CONFIRMED
   ↓
DRIVER COMMITTED
```

Before confirmation:

```text
driver is not yet fully committed
```

However, the system must prevent two riders from successfully assigning the
same driver concurrently.

This requires transactional coordination.

---

# 15. Concurrent Assignment

Consider:

```text
Ride A → Driver X
Ride B → Driver X
```

Both riders select bids at almost exactly the same time.

Both requests reach different Go instances:

```text
Go A
  ↓
Ride A

Go B
  ↓
Ride B
```

The system must not allow both assignments to succeed.

The authoritative protection must exist in PostgreSQL.

Redis may assist with coordination but must not be the only protection.

---

# 16. Assignment Transaction

Conceptually:

```text
BEGIN

lock relevant driver state
lock relevant ride
validate eligibility
validate driver commitment
assign driver
update ride
update bid
create events

COMMIT
```

If another transaction has already committed the driver:

```text
second transaction
    ↓
detect conflict
    ↓
reject/fallback
```

The exact locking strategy will be finalized in the assignment design.

---

# 17. Driver Suspension

A driver who becomes suspended must immediately become ineligible for new
bids.

Example:

```text
APPROVED
   ↓
SUSPENDED
```

The driver must no longer receive new bidding opportunities.

Existing rides require separate handling.

Suspension must not silently rewrite historical ride records.

---

# 18. Driver Going Offline

When a driver switches offline:

```text
POST /api/v1/me/driver/offline
```

the backend updates:

```text
driver_profiles.availability_status
```

to:

```text
OFFLINE
```

The driver should then stop receiving new bidding opportunities.

Existing active rides are handled according to their current lifecycle.

Going offline must not automatically cancel a confirmed ride unless an explicit
business rule says so.

---

# 19. Connection Loss

A driver may disappear without explicitly switching offline.

Example:

```text
phone loses network
       ↓
WebSocket disconnects
       ↓
Redis presence TTL expires
```

The driver should stop being considered operationally reachable.

However, the durable PostgreSQL availability state may remain:

```text
ONLINE
```

until the system reconciles it.

This is why:

```text
availability
+
presence
```

are separate concepts.

---

# 20. Eligibility During Bid Submission

Eligibility must be checked again when the driver submits a bid.

It is not sufficient to check eligibility when the driver is discovered.

Example:

```text
10:00:00
Driver discovered

10:00:10
Driver becomes suspended

10:00:11
Driver submits bid
```

The bid must be rejected.

The client cannot rely on the earlier discovery result.

---

# 21. Eligibility During Bid Update

The same principle applies when updating a bid.

A driver must still satisfy all required rules when modifying a bid.

Example:

```text
Driver submits:
PKR 1200

Driver becomes unavailable

Driver attempts:
PKR 1100
```

The backend must determine whether the bid modification is still allowed.

---

# 22. Eligibility During Bid Withdrawal

Withdrawal is different.

A driver who is no longer eligible should generally still be allowed to
withdraw an existing bid, subject to the ride's lifecycle rules.

Example:

```text
Driver becomes offline
       ↓
existing ACTIVE bid
       ↓
driver withdraws bid
```

This should not be rejected merely because the driver is now offline.

The command is changing an existing relationship rather than creating a new
opportunity.

---

# 23. Bidding Deadline

A driver is not eligible to submit or modify a bid after:

```text
rides.bidding_ends_at
```

The backend must check the authoritative PostgreSQL timestamp.

Client countdowns are not authoritative.

Redis timers are not authoritative.

---

# 24. Ride State

The ride must be in a state that accepts bids.

Initially:

```text
BIDDING
```

is the only state in which new bids can be submitted.

Therefore:

```text
BIDDING
    → bid allowed

DRIVER_SELECTED
    → bid rejected

DRIVER_CONFIRMED
    → bid rejected

TRIP_STARTED
    → bid rejected

TRIP_COMPLETED
    → bid rejected

CANCELLED
    → bid rejected
```

---

# 25. Driver Eligibility vs Bid Eligibility

These are related but not identical.

Driver eligibility:

```text
Can this driver participate in this ride?
```

Bid eligibility:

```text
Can this driver perform this specific bid operation right now?
```

For example:

```text
Driver is approved
Driver is online
Driver has eligible vehicle
Driver is nearby

→ driver eligible
```

But:

```text
bidding deadline expired

→ bid not eligible
```

The implementation should keep these checks conceptually separate.

---

# 26. Discovery Candidate Set

The discovery system should only operate on potentially eligible drivers.

Conceptually:

```text
All drivers
    ↓
Basic eligibility filtering
    ↓
Potential candidates
    ↓
Geographic discovery
    ↓
Operational filtering
    ↓
Dispatch candidates
```

This prevents geographic search from becoming responsible for business rules.

---

# 27. Geographic Discovery

Geographic proximity is a discovery concern.

For example:

```text
Ride pickup:
33.6844, 73.0479

Search radius:
5 km
```

The discovery system finds nearby candidates.

But proximity alone does not establish eligibility.

A nearby driver may be:

```text
Suspended
Wrong vehicle
Offline
Already committed
Not present
```

Therefore:

```text
nearby
    ≠
eligible
```

---

# 28. Discovery Radius

The initial system should support a configurable discovery radius.

Example:

```text
initial radius = R
```

The actual value is a product/operations decision and should not be
hard-coded in this architecture document.

The system may later support:

```text
small city
    → smaller radius

low driver density
    → larger radius
```

However, dynamic radius expansion should be introduced only after the basic
dispatch behavior is validated.

---

# 29. Number of Drivers Discovered

The system should not necessarily notify every eligible driver in the city.

Sending every ride to every nearby driver creates:

- Excessive notifications
- Excessive bids
- More database writes
- More WebSocket traffic
- Poor driver experience
- More difficult rider decision-making

The dispatch system should therefore operate on a bounded candidate set.

The exact candidate count will be determined during the discovery/dispatch
design.

---

# 30. Candidate Refresh

Driver eligibility is dynamic.

A candidate may become:

```text
offline
suspended
busy
unreachable
```

after discovery.

Therefore the system should not assume that a candidate remains eligible
until the bidding deadline.

Eligibility is revalidated at important operations.

---

# 31. Driver Location Freshness

A driver's location must be sufficiently fresh to participate in geographic
discovery.

Example:

```text
location timestamp
      ↓
current time - timestamp
      ↓
fresh enough?
```

If the location is too old:

```text
candidate rejected
```

The exact freshness threshold belongs to dispatch configuration.

---

# 32. Driver Location Accuracy

Location accuracy may also influence discovery.

Example:

```text
accuracy = 5m
```

is substantially different from:

```text
accuracy = 500m
```

The initial implementation should at least preserve the reported accuracy.

Whether accuracy becomes a hard eligibility requirement should be determined
after real-world testing.

We should not over-engineer this rule initially.

---

# 33. GPS Manipulation

Driver-provided GPS cannot be treated as inherently trustworthy.

Potential issues include:

- Mock locations
- GPS spoofing
- Stale timestamps
- Impossible movement
- Excessive speed
- Location jumps

The initial architecture should record sufficient metadata to support future
fraud detection.

It should not attempt to build a sophisticated fraud engine before the core
ride flow exists.

---

# 34. Driver Eligibility Snapshot

When a driver is selected for discovery, the system may capture the relevant
candidate information:

```text
driver_id
vehicle_id
location
location timestamp
eligibility information
```

This is useful for debugging and dispatch analysis.

However, the snapshot does not replace final validation.

The driver must be revalidated when submitting a bid and during assignment.

---

# 35. Eligibility Failure Reasons

The backend should use internal machine-readable failure reasons.

Examples:

```text
DRIVER_NOT_APPROVED
DRIVER_OFFLINE
DRIVER_NOT_PRESENT
DRIVER_SUSPENDED
NO_ELIGIBLE_VEHICLE
SERVICE_NOT_SUPPORTED
DRIVER_ALREADY_COMMITTED
RIDE_NOT_ACCEPTING_BIDS
BIDDING_DEADLINE_PASSED
LOCATION_UNAVAILABLE
LOCATION_TOO_STALE
```

These reasons are useful for:

- Logs
- Metrics
- Debugging
- Operations

Not every internal reason should be exposed directly to clients.

---

# 36. Security Boundary

The client must never determine:

```text
"I am eligible for this ride."
```

The client may display:

```text
Eligible
```

based on server responses.

But the backend is authoritative.

A malicious client must not be able to bypass eligibility by modifying its
Flutter application or sending a custom HTTP/WebSocket request.

---

# 37. Eligibility Evaluation

Conceptually:

```text
isEligible(driver, ride)
```

should evaluate:

```text
1. Driver exists
2. Driver approved
3. Driver not suspended
4. Driver operationally online
5. Driver presence is valid
6. Eligible vehicle exists
7. Vehicle belongs to driver
8. Vehicle supports service type
9. Driver is not committed to a conflicting ride
10. Ride accepts bidding
11. Bidding deadline has not passed
12. Required location information is available
```

Not every check must execute against PostgreSQL individually.

The implementation should avoid unnecessary database round trips.

---

# 38. Separation of Concerns

The dispatch architecture should separate:

```text
Eligibility
    ↓
Business rules

Discovery
    ↓
Candidate retrieval

Ranking
    ↓
Candidate ordering

Notification
    ↓
Deliver opportunity

Bid validation
    ↓
Final authorization

Assignment
    ↓
Concurrency-safe commitment
```

These are different responsibilities.

They should not become one large function.

---

# 39. Recommended Initial Architecture

```text
                         Ride
                          │
                          ▼
                  ┌───────────────┐
                  │  Eligibility  │
                  └───────┬───────┘
                          │
                   eligible drivers
                          │
                          ▼
                  ┌───────────────┐
                  │   Discovery   │
                  └───────┬───────┘
                          │
                    nearby candidates
                          │
                          ▼
                  ┌───────────────┐
                  │    Ranking    │
                  └───────┬───────┘
                          │
                    selected batch
                          │
                          ▼
                  ┌───────────────┐
                  │ Notification  │
                  └───────┬───────┘
                          │
                          ▼
                       Drivers
                          │
                          ▼
                         Bids
                          │
                          ▼
                  Final eligibility
                          │
                          ▼
                     Assignment
```

---

# 40. What We Should Not Decide Yet

The following are intentionally deferred:

```text
Exact discovery radius
Exact candidate count
Candidate ranking algorithm
Dynamic radius expansion
Driver scoring
Surge dispatch
Fraud scoring
Advanced geospatial indexing
Predictive ETA ranking
Machine-learning dispatch
```

These should not be invented before we have the basic system working.

---

# 41. Design Principles

1. Eligibility and discovery are separate concerns.
2. Proximity does not imply eligibility.
3. PostgreSQL remains authoritative for durable business eligibility.
4. Redis provides operational information but is not the final authority.
5. Driver approval is required.
6. Driver availability is required.
7. Operational presence is required for active discovery.
8. An eligible vehicle is required.
9. Vehicle/service compatibility must be enforced.
10. Conflicting assignments must be prevented transactionally.
11. Eligibility must be revalidated at bid submission.
12. Eligibility must be revalidated at assignment.
13. Bidding deadlines are authoritative in PostgreSQL.
14. Client state never establishes eligibility.
15. Discovery should operate on a bounded candidate set.
16. Location freshness matters.
17. Location accuracy should initially be recorded rather than over-engineered.
18. Fraud detection is deferred.
19. Eligibility failure reasons should be observable internally.
20. Dispatch should be composed of eligibility, discovery, ranking, notification,
    bid validation, and assignment rather than one monolithic operation.
