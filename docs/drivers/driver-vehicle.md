# Driver and Vehicle Model

## 1. Purpose

This document defines the durable relationship between drivers and vehicles
and how that relationship participates in eligibility, availability, discovery,
bidding, reservation, and assignment.

The core model is:

```text
Driver
  │
  ├── Identity / account
  ├── Eligibility
  └── Active vehicle
          │
          ├── Vehicle identity
          ├── Vehicle category
          ├── Capacity
          ├── Registration / status
          └── Service capabilities
```

The fundamental rule is:

```text
Driver ≠ Vehicle
```

A driver may operate different vehicles over time, while a vehicle has its own
identity, status, and eligibility.

---

# 2. Driver Model

The driver represents the person authorized to perform rides.

Driver data includes durable information such as:

```text
application user identity
driver account status
eligibility-related records
registered vehicles
```

Driver operational presence and location are separate concerns and are defined
in the availability design.

---

# 3. Vehicle Model

A vehicle is an independently identifiable asset that a driver may use to
perform rides.

Conceptually:

```text
Vehicle
├── identity
├── category
├── capacity
├── registration information
├── operational status
└── service capabilities
```

The exact physical/legal attributes will be added only when they support an
actual business rule.

---

# 4. Driver–Vehicle Relationship

A driver may have multiple registered vehicles:

```text
Driver X
   ├── Vehicle A
   ├── Vehicle B
   └── Vehicle C
```

But the driver has only one vehicle active for a given operating context in
the initial system.

The active vehicle is the vehicle associated with discovery and subsequent
assignment.

---

# 5. Active Vehicle

For the initial system:

```text
Driver
   ↓
active vehicle
   ↓
availability / discovery
```

The backend must know which vehicle the driver is currently operating before
that driver can become a candidate for a vehicle-specific ride.

The active vehicle should not be inferred from whichever vehicle was most
recently registered.

It must be explicitly selected or established by the backend.

---

# 6. One Active Vehicle

The initial system should enforce:

```text
one driver
   ↓
one active vehicle
```

This avoids ambiguity such as:

```text
Driver X
   ↓
Vehicle A + Vehicle B
   ↓
Which vehicle is approaching the rider?
```

Multiple registered vehicles remain supported; multiple simultaneously active
vehicles are deferred.

---

# 7. Vehicle Category

The vehicle should have a product-defined category.

Examples may include:

```text
standard
premium
larger vehicle
accessible vehicle
```

The actual category taxonomy is a product decision.

The category participates in eligibility and discovery.

---

# 8. Capacity

Vehicle capacity should be represented explicitly where it affects eligibility.

Example:

```text
vehicle capacity = 4
passenger count = 5
```

The vehicle is not eligible for that request.

Capacity is a vehicle property, not a driver property.

---

# 9. Service Capabilities

A vehicle may support one or more service categories.

Conceptually:

```text
Vehicle A
   ├── standard
   └── larger-vehicle
```

The exact product model may use a category field, capability relation, or both.

Do not introduce a complex capability system until multiple independent
capabilities are actually required.

---

# 10. Driver Eligibility

Driver eligibility answers:

> Is this driver currently authorized to perform the requested service?

Potential factors include:

```text
account status
required documents
license status
service authorization
regional authorization
```

The exact compliance model is a separate concern.

---

# 11. Vehicle Eligibility

Vehicle eligibility answers:

> Is this vehicle currently permitted and suitable for the requested service?

Potential factors include:

```text
vehicle status
registration
inspection/compliance
category
capacity
service capabilities
```

---

# 12. Combined Eligibility

A discovery candidate requires both driver and vehicle eligibility.

Conceptually:

```text
Driver eligible
      +
Vehicle eligible
      +
Vehicle satisfies ride requirements
      ↓
Eligible driver/vehicle pair
```

Examples:

```text
Driver eligible
+
Vehicle inactive
=
not eligible
```

and:

```text
Driver suspended
+
Vehicle valid
=
not eligible
```

---

# 13. Eligibility Is Not Availability

A driver/vehicle pair can be fully eligible but unavailable.

Example:

```text
Driver = eligible
Vehicle = eligible
Commitment = active trip

Result:
not available
```

Eligibility answers whether the pair may perform the ride.

Availability answers whether it can receive a new conflicting opportunity now.

---

# 14. Vehicle Status

The vehicle should have a durable operational status.

Initial conceptual states:

```text
ACTIVE
INACTIVE
SUSPENDED
```

The exact vocabulary can evolve.

A vehicle that is inactive or suspended must not be used for new discovery or
assignment.

Existing trips using that vehicle require separate operational handling.

---

# 15. Driver Status

The driver should have a durable account status separate from presence.

Conceptually:

```text
ACTIVE
SUSPENDED
DEACTIVATED
```

This is different from:

```text
ONLINE
OFFLINE
```

Durable account status belongs to the driver domain.

Operational presence belongs to availability.

---

# 16. Vehicle Switching

A driver may switch the active vehicle while not committed to a conflicting
ride.

Conceptually:

```text
Vehicle A active
      ↓
switch
      ↓
Vehicle B active
```

The backend validates Vehicle B before making it active.

---

# 17. Vehicle Switching While Reserved

A driver should not be allowed to switch the active vehicle after a reservation
has been created unless the assignment domain explicitly supports a vehicle
change.

Otherwise the rider may select a bid associated with:

```text
Vehicle A
```

while the driver actually arrives in:

```text
Vehicle B
```

That creates a serious consistency and trust problem.

The initial system should reject active-vehicle changes while reserved,
assigned, or in an active trip.

---

# 18. Vehicle Switching After Trip Completion

After the trip ends, the driver may select another valid vehicle if they remain
online.

Example:

```text
TRIP_ACTIVE
   ↓
TRIP_COMPLETED
   ↓
release commitment
   ↓
switch active vehicle
   ↓
ONLINE_AVAILABLE
```

---

# 19. Vehicle Association With Bids

A bid should identify the active vehicle used to make the offer.

Conceptually:

```text
Bid
 ├── ride
 ├── driver
 └── vehicle
```

This ensures the rider knows which vehicle is being offered and preserves the
historical association.

---

# 20. Vehicle Association With Reservation

When a bid is selected and a reservation is created, the reservation should
retain the selected vehicle.

Conceptually:

```text
Bid
 ↓
Reservation
 ↓
Vehicle
```

The reservation must not silently switch vehicles later.

---

# 21. Vehicle Association With Assignment

The final assignment must preserve the vehicle that was actually committed.

Conceptually:

```text
Ride
 ↓
Bid
 ↓
Reservation
 ↓
Assignment
 ├── Driver
 └── Vehicle
```

This creates an auditable historical record.

---

# 22. Historical Vehicle Association

If a driver later changes vehicles, historical rides must continue to reference
the vehicle actually used for that ride.

For example:

```text
2026 Ride A → Vehicle A
2026 Ride B → Vehicle B
```

Changing the driver's current vehicle must not rewrite Ride A.

---

# 23. Discovery Candidate

Discovery should operate on a driver + active vehicle pair.

Not merely:

```text
nearby driver
```

but:

```text
nearby driver
+
active vehicle
+
valid driver status
+
valid vehicle status
+
combined eligibility
+
fresh location
+
availability
```

This prevents a driver with an unsuitable vehicle from entering the wrong
candidate pool.

---

# 24. Vehicle Category and Bidding

The vehicle category should be known when a driver submits a bid.

The bid should not say only:

```text
Driver X bids PKR 1,200
```

without preserving which eligible vehicle is being offered.

The rider may need to compare:

```text
price
ETA
vehicle category
vehicle information
```

The exact rider-facing information remains a product/UI decision.

---

# 25. Bid Does Not Change Vehicle

Submitting a bid does not permit the driver to change the vehicle associated
with that bid.

If the driver changes vehicles before a reservation is created, the safer
initial behavior is to invalidate the old bid and require a new bid under the
new vehicle.

This prevents the rider from selecting an offer for Vehicle A when the driver
is now operating Vehicle B.

---

# 26. Reservation Revalidation

When a bid is selected, the backend must revalidate:

```text
driver status
vehicle status
active vehicle relationship
combined eligibility
availability/commitment
```

Discovery data may be stale.

The reservation transaction is authoritative.

---

# 27. Assignment Revalidation

Driver confirmation must revalidate the reservation and its driver/vehicle
association.

The backend must not create an assignment for:

```text
inactive vehicle
suspended driver
expired reservation
already-committed driver
```

---

# 28. Vehicle Change During Active Trip

The initial system should not support changing vehicles during an active trip.

If the vehicle becomes unusable, the trip enters an operational exception path
rather than silently replacing the vehicle record.

A vehicle replacement feature can be designed later if the product requires it.

---

# 29. Registration and Compliance Boundary

The vehicle domain may eventually include:

```text
registration
inspection
insurance
permit
license plate
```

These should be modeled only to the extent that they support an eligibility or
compliance rule.

The detailed document/verification workflow should be a separate compliance
domain.

---

# 30. Vehicle Identity

A vehicle requires a stable internal identifier.

External identifiers such as registration or license plate are attributes, not
the primary domain identity.

A vehicle record should therefore have an internal ID that remains stable even
if a mutable display attribute changes.

---

# 31. Driver Identity Boundary

The driver domain should reference the application's authenticated user.

The driver record is not the OIDC identity itself.

Conceptually:

```text
External OIDC identity
        ↓
Application user
        ↓
Driver profile
```

This keeps external authentication concerns separate from driver business data.

---

# 32. Privacy

Driver and vehicle information shown to riders should be intentionally limited.

Potential rider-facing data may include:

```text
driver display name
rating
vehicle category
vehicle make/model where appropriate
vehicle color
license plate where operationally necessary
```

Private driver data should never be exposed merely because it exists in the
database.

---

# 33. PostgreSQL Responsibilities

PostgreSQL should be authoritative for durable driver/vehicle relationships:

```text
driver record
vehicle record
driver-vehicle relationship
active vehicle relationship
durable statuses
eligibility/compliance records
assignment vehicle snapshot
```

The exact table structure is a later database-design task.

---

# 34. Redis Responsibilities

Redis should not become the durable driver/vehicle database.

Redis may cache or index:

```text
active driver/vehicle pairing
operational availability
latest location
geospatial discovery data
```

If Redis loses its contents, durable vehicle ownership and eligibility must be
recoverable from PostgreSQL.

---

# 35. Multiple Vehicles

The model supports multiple registered vehicles:

```text
Driver X
 ├── Vehicle A
 ├── Vehicle B
 └── Vehicle C
```

But the initial operational rule remains:

```text
one active vehicle at a time
```

This keeps discovery and assignment deterministic.

---

# 36. Vehicle Deactivation

A vehicle may become inactive because of:

```text
registration issue
maintenance
compliance issue
owner action
administrative suspension
```

The vehicle should immediately stop being eligible for new discovery.

Existing commitments require their own operational policy.

---

# 37. Driver Suspension

If the driver is suspended:

```text
new bids
new discovery opportunities
new reservations
new assignments
```

must be blocked.

Existing active rides should not be silently deleted by changing the driver
status record.

They require an explicit operational transition.

---

# 38. Combined Eligibility Evaluation

The effective candidate decision is conceptually:

```text
Driver ACTIVE
        AND
Vehicle ACTIVE
        AND
Driver eligible for service
        AND
Vehicle eligible for service
        AND
Vehicle satisfies ride requirements
        AND
Driver has active vehicle
        ↓
eligible driver/vehicle pair
```

This is evaluated for the specific ride.

---

# 39. Availability Interaction

Even a fully eligible driver/vehicle pair may be unavailable.

The final discovery candidate therefore requires both:

```text
eligibility
+
availability
```

Availability also requires fresh presence/location where applicable.

The complete model is:

```text
Driver
  ↓
valid status
  ↓
Active Vehicle
  ↓
valid vehicle status
  ↓
Combined Eligibility
  ↓
Availability
  ↓
Fresh Location
  ↓
Discovery Candidate
```

---

# 40. Concurrency

Important races include:

```text
Driver switches vehicle
      +
Rider selects existing bid
```

```text
Vehicle becomes inactive
      +
Reservation is created
```

```text
Driver is suspended
      +
Driver confirms reservation
```

```text
Two sessions attempt to select different active vehicles
```

The backend must resolve these using PostgreSQL transactions and appropriate
version/constraint checks.

---

# 41. Active Vehicle Change Race

Suppose:

```text
Bid → Vehicle A
```

and simultaneously:

```text
Driver changes active vehicle → Vehicle B
```

The system must not allow an assignment to silently use B when the rider
selected an offer for A.

The reservation transaction should either:

```text
retain Vehicle A
```

or reject the operation if Vehicle A is no longer valid.

---

# 42. Vehicle Status Change Race

Suppose:

```text
Vehicle A = ACTIVE
```

and then becomes:

```text
INACTIVE
```

while a rider selects a bid using Vehicle A.

The reservation transaction must revalidate the vehicle status.

If invalid, reservation fails and the ride follows its fallback path.

---

# 43. Driver Status Change Race

Likewise:

```text
Driver = ACTIVE
```

then:

```text
SUSPENDED
```

before confirmation.

The assignment must not commit simply because discovery previously found the
driver eligible.

---

# 44. API Boundary

The driver/vehicle domain should expose commands such as:

```text
POST /api/v1/driver/vehicles
POST /api/v1/driver/vehicles/{vehicle_id}/activate
POST /api/v1/driver/vehicles/{vehicle_id}/deactivate
GET  /api/v1/driver/vehicles
```

Exact API naming and authorization will be finalized during API design.

Clients should not directly patch arbitrary eligibility or status fields.

---

# 45. Observability

Useful metrics include:

```text
drivers_active_total
vehicles_active_total
drivers_with_active_vehicle_total
vehicle_activation_failures_total
eligibility_failures_total
vehicle_status_conflicts_total
```

Useful trace/log fields include:

```text
driver_id
vehicle_id
ride_id
bid_id
reservation_id
assignment_id
```

Avoid high-cardinality IDs as metric labels.

---

# 46. Complete Relationship Diagram

```text
                  ┌──────────────────┐
                  │ Application User │
                  └────────┬─────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │    Driver    │
                    └──────┬───────┘
                           │
                    registered vehicles
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         ┌─────────┐  ┌─────────┐  ┌─────────┐
         │Vehicle A│  │Vehicle B│  │Vehicle C│
         └────┬────┘  └─────────┘  └─────────┘
              │
         active vehicle
              │
              ▼
      ┌───────────────────┐
      │ Driver/Vehicle    │
      │ Eligibility       │
      └─────────┬─────────┘
                │
                ▼
          Availability
                │
                ▼
           Discovery
                │
                ▼
              Bid
                │
                ▼
          Reservation
                │
                ▼
           Assignment
                │
                ▼
              Ride
```

---

# 47. What We Should Not Build Yet

Do not build:

```text
Multiple simultaneously active vehicles per driver
Complex vehicle capability matrices
Automatic vehicle replacement during trips
Full compliance/document management system
Vehicle marketplace
Fleet-management subsystem
Vehicle maintenance management
Deep external registry integrations
```

Add these only when the product creates a concrete requirement.

---

# 48. Design Principles

1. Driver and vehicle are separate domain entities.
2. A driver may own/register multiple vehicles.
3. The initial system supports one active vehicle per driver.
4. Driver eligibility and vehicle eligibility are separate checks.
5. Both driver and vehicle must be valid for a discovery candidate.
6. Eligibility is distinct from operational availability.
7. The active vehicle must be explicit, not inferred from registration order.
8. A bid identifies the driver and vehicle making the offer.
9. Reservation preserves the selected vehicle.
10. Assignment preserves the actual driver/vehicle association historically.
11. Changing the current vehicle must never rewrite historical rides.
12. Vehicle changes are blocked during reservation, assignment, and active trip in the initial system.
13. Material vehicle changes before reservation should invalidate/recreate the bid rather than silently mutate it.
14. Assignment revalidates driver and vehicle state because discovery can become stale.
15. Durable driver/vehicle state belongs in PostgreSQL.
16. Redis may accelerate operational discovery but is not the durable vehicle database.
17. External OIDC identity remains separate from driver business identity.
18. Sensitive driver and vehicle information must be role-protected.
19. New vehicle attributes should be introduced when they support actual business rules.
20. Multiple active vehicles and complex fleet behavior are explicitly deferred.
