# Ride Request

## 1. Purpose

This document defines the business data and rules for creating a ride request.

A ride request represents the rider's intent to travel from an origin to a
destination under a set of requirements.

It is the input to the ride lifecycle:

```text
Rider
  ↓
Ride Request
  ↓
Ride
  ↓
Discovery
  ↓
Bidding
  ↓
Assignment
  ↓
Trip
```

The ride request is deliberately separated from dispatch, assignment,
payment, and trip-tracking concerns.

---

# 2. Core Principle

A ride request captures **what the rider asked for**.

It should not become a container for every piece of information accumulated
during the ride.

Conceptually:

```text
Ride Request
├── rider
├── pickup
├── destination
├── service requirements
├── passenger requirements
└── request options
```

Later:

```text
Ride
├── request data
├── lifecycle state
├── bids
├── reservation
├── assignment
└── trip data
```

---

# 3. Rider Ownership

A ride request belongs to one authenticated rider.

The rider identity comes from the external OIDC-authenticated identity and is
resolved by the backend to the application's rider record.

The client must not be able to create a ride for an arbitrary rider ID.

Conceptually:

```text
OIDC identity
     ↓
authenticated application user
     ↓
rider
     ↓
ride request
```

---

# 4. Pickup

The request must contain a pickup location.

The authoritative geographic representation should include:

```text
latitude
longitude
```

Coordinates are the durable location primitive used by the backend.

An optional human-readable address may also be stored for display.

---

# 5. Destination

The request must contain a destination location.

Like pickup, the authoritative geographic representation includes:

```text
latitude
longitude
```

An optional human-readable address may be stored for display.

---

# 6. Google Maps Boundary

Google Maps/Routes/Places may be used to provide:

```text
place search
geocoding
reverse geocoding
routing
ETA
route information
```

However, Google Maps is not the authoritative source of the ride record.

The backend should persist the location information required to reproduce the
ride request without requiring Google to return the same result later.

Therefore:

```text
Google Maps
    ↓
location assistance
    ↓
backend normalization
    ↓
ride request
```

not:

```text
ride request
    ↓
Google place ID as the only location data
```

---

# 7. Place IDs

If a Google Place ID is provided, it may be retained as external metadata.

It must not replace latitude/longitude as the core location representation.

Place IDs are external identifiers and their lifecycle is controlled by the
external provider.

---

# 8. Display Address

The client may send a selected display address, or the backend may obtain one
from an approved maps service.

The address is presentation data.

The system must not use a mutable display string as the basis for geographic
matching.

For example:

```text
"123 Main Street, Lahore"
```

is not sufficient as the authoritative location.

Coordinates are.

---

# 9. Location Precision

The request should preserve the precision received from the client/provider
rather than aggressively rounding coordinates.

The system can use a normalized representation for storage and comparison,
but should avoid introducing unnecessary geographic error.

The exact PostgreSQL/PostGIS decision will be made during database design.

---

# 10. Pickup and Destination Are Requested Locations

The request contains the rider's intended locations.

They are not necessarily the actual locations used during trip execution.

Conceptually:

```text
Requested pickup
       ↓
driver approaches
       ↓
actual pickup
```

and:

```text
Requested destination
       ↓
trip execution
       ↓
actual drop-off
```

Actual trip locations belong to trip/telemetry data rather than being silently
mutated into the original request.

---

# 11. Immutability of Request Intent

Once a ride enters active dispatch, core request intent should be immutable.

In particular, the system should not silently change:

```text
pickup coordinates
destination coordinates
service category
passenger requirements
```

after drivers have started bidding.

Changing these values after bidding would invalidate driver offers.

---

# 12. Changes Before Dispatch

If the product eventually supports editing a ride before dispatch, the safer
model is:

```text
REQUESTED
   ↓
modify request
   ↓
DISCOVERY
```

rather than modifying the request after bids have already been submitted.

Any change that materially affects the offer should invalidate/restart the
dispatch process.

---

# 13. Service / Vehicle Requirement

A ride request should specify the requested service or vehicle category.

Examples may eventually include:

```text
standard
premium
larger vehicle
accessible vehicle
```

The exact product categories are not finalized yet.

The request should store the requirement rather than a specific driver or
vehicle.

Conceptually:

```text
Ride Request
   ↓
vehicle/service requirement
   ↓
eligibility
   ↓
discovery
```

---

# 14. Passenger Count

The request should include the number of passengers where the product requires
it.

Example:

```text
passenger_count = 2
```

The backend validates that the requested service/vehicle can support the
passenger count.

The driver should not determine whether the request is structurally valid.

---

# 15. Passenger Requirements

Additional passenger requirements should be explicit rather than encoded in
free-form text where possible.

Potential future examples:

```text
wheelchair_accessible
child_seat
extra_luggage
```

Only requirements that have an actual eligibility rule should become structured
fields.

Do not create dozens of speculative columns before the product requires them.

---

# 16. Scheduled Rides

The initial system should distinguish an immediate ride from a future ride if
scheduled rides are eventually supported.

For an immediate ride:

```text
requested_at = now
```

For a scheduled ride:

```text
requested_at = creation time
scheduled_for = future time
```

Scheduled dispatch is a separate concern and should not complicate the initial
real-time flow.

---

# 17. Immediate Ride Assumption

The initial MVP should assume:

```text
ride is requested now
```

and enters discovery shortly after creation.

Scheduled rides can be introduced later without changing the fundamental ride
request model.

---

# 18. Time Fields

The ride request should retain authoritative server timestamps such as:

```text
created_at
updated_at
```

If scheduling is supported later:

```text
scheduled_for
```

Client-provided timestamps must not be treated as authoritative event times.

---

# 19. Time Zone

Timestamps should be stored in a timezone-safe representation, preferably
UTC-based timestamps in PostgreSQL.

User-facing formatting occurs at the application/UI boundary.

A future scheduled ride may additionally retain the relevant local timezone
identifier if the product requires local-time semantics.

---

# 20. Currency

The request/ride should have a defined currency context.

For the initial Pakistan-oriented deployment this may be:

```text
PKR
```

However, currency should remain an explicit domain value rather than being
implicitly hard-coded throughout the application.

The driver bid must use the ride's currency.

---

# 21. Reference Fare vs Driver Bid

A rider request may eventually have an estimated/reference fare.

That value is different from a driver bid.

```text
Reference fare
    ↓
platform estimate / informational value

Driver bid
    ↓
individual driver's offer
```

The rider may use the reference fare when evaluating bids, but it must not be
confused with the selected driver's actual offer.

The detailed pricing model will be designed separately.

---

# 22. No Driver Information in the Request

The ride request should not contain:

```text
driver_id
vehicle_id
reservation_id
assignment_id
```

Those relationships are created by dispatch and assignment.

This keeps request intent independent from who eventually performs the trip.

---

# 23. No Payment Data in the Request

The ride request should not contain payment transaction state.

Payment belongs to a separate payment domain.

The request may eventually reference a selected payment method, but payment
authorization/capture/failure should not be represented as ride-request
fields.

---

# 24. No Live Location in the Request

The request's pickup and destination are static intent.

Live driver/rider locations are dynamic telemetry.

Do not continually overwrite the ride's pickup or destination with GPS
updates.

Conceptually:

```text
Ride Request
   ├── requested pickup
   └── requested destination

Location Telemetry
   ├── driver positions
   └── rider position where required
```

---

# 25. Request Validation

Ride creation must validate at least:

```text
authenticated rider
valid pickup coordinates
valid destination coordinates
supported service category
valid passenger count
valid request options
valid currency context
```

The backend must perform validation even if Flutter already validates the same
fields.

Client validation is for UX.

Server validation is for correctness and security.

---

# 26. Coordinate Validation

The backend should reject invalid geographic coordinates.

Latitude must be within:

```text
-90 ≤ latitude ≤ 90
```

Longitude must be within:

```text
-180 ≤ longitude ≤ 180
```

The system should also reject obviously malformed or missing coordinates.

---

# 27. Pickup Equals Destination

A request where pickup and destination are identical or effectively identical
should normally be rejected or explicitly handled by product policy.

Do not silently create a normal trip that has no meaningful route.

The exact minimum-distance threshold is a product decision.

---

# 28. Service Eligibility

The request's service requirements feed driver eligibility.

Example:

```text
Ride requests 6 passengers
        ↓
standard 4-seat vehicle
        ↓
not eligible
```

Eligibility remains a driver/vehicle domain concern.

The ride request only expresses the requirement.

---

# 29. Request Idempotency

Ride creation must tolerate mobile retries.

Example:

```text
Flutter sends create request
        ↓
server creates ride
        ↓
network response is lost
        ↓
Flutter retries
```

The retry must not create two rides.

Ride creation should therefore support an idempotency key supplied by the
client and enforced by the backend.

The exact API contract will be finalized during API design.

---

# 30. Request Creation Transaction

Ride creation should be transactional.

Conceptually:

```text
BEGIN

validate rider
validate request
create ride
create initial lifecycle state
create outbox event

COMMIT
```

Only after successful commit should downstream discovery processing begin.

---

# 31. Discovery Trigger

Ride creation should not depend on the client to manually start discovery.

After the ride is committed:

```text
ride created
   ↓
outbox event
   ↓
discovery worker/service
   ↓
DISCOVERY
```

This makes dispatch server-driven and resilient to client disconnects.

---

# 32. Request State vs Ride State

The request contains intent.

The ride contains lifecycle state.

Therefore avoid duplicating state such as:

```text
request_status
ride_status
```

unless there is a clearly distinct business meaning.

The authoritative lifecycle state belongs to the ride.

---

# 33. Request Snapshot

Once dispatch begins, the system should retain a snapshot of the request data
used to evaluate drivers and bids.

This protects against accidental mutation of the rider's original intent.

For example:

```text
Ride
 ├── pickup snapshot
 ├── destination snapshot
 ├── service requirement
 └── passenger requirement
```

The exact normalization/relational structure is a database design decision.

---

# 34. Privacy

Ride request data can be sensitive.

Access should be limited according to role:

```text
Rider
  → own ride request

Assigned/eligible driver
  → only information needed to perform the ride

Operations/admin
  → according to explicit authorization
```

A driver should not receive unnecessary rider information.

---

# 35. Address Privacy

The platform should avoid exposing the rider's full address to drivers before
it is operationally necessary.

The exact disclosure point depends on the product and safety requirements.

Coordinates and address data should therefore be treated as protected ride
information, not generic public data.

---

# 36. External Provider Data

External maps data should be treated as provider data, not blindly copied into
the domain model.

Store only information needed for:

```text
ride execution
user experience
routing
support
compliance
```

Avoid storing large provider responses or unnecessary metadata in the ride
record.

---

# 37. Route Information

The initial ride request does not need to store the complete route geometry.

A route can be recalculated when required using the configured maps provider.

If route snapshots become necessary for pricing, navigation, or audit, that
should be a separate design decision.

---

# 38. Request Versioning

If the request contains mutable pre-dispatch options, a request revision can
be used to detect stale updates.

However, once bidding starts, material request changes should generally be
rejected rather than creating complicated mid-dispatch synchronization.

---

# 39. API Boundary

The initial API should expose commands rather than arbitrary model mutation.

Conceptually:

```text
POST /api/v1/rides
```

creates a ride request.

After creation, commands operate on the ride lifecycle:

```text
POST /api/v1/rides/{ride_id}/cancel
GET  /api/v1/rides/{ride_id}
```

Dispatch-specific APIs remain separate:

```text
GET  /api/v1/rides/{ride_id}/bids
POST /api/v1/rides/{ride_id}/bids/{bid_id}/select
```

The exact API specification will be created later.

---

# 40. Error Handling

Ride creation should return explicit validation errors for invalid requests.

Examples:

```text
invalid coordinates
unsupported service
invalid passenger count
request already processed
unauthorized rider
```

The API should not return success while silently modifying the request into a
different product request.

---

# 41. Cancellation Before Discovery

If the rider cancels immediately after creation and before discovery begins,
the backend must resolve the race transactionally.

Possible outcome:

```text
Ride created
   ↓
cancel command wins
   ↓
CANCELLED
```

Discovery must then observe that the ride is no longer eligible for dispatch.

It must not start bidding for a cancelled ride.

---

# 42. Cancellation During Discovery

If cancellation races with discovery:

```text
rider cancels
      +
discovery worker starts
```

both operations must revalidate the authoritative ride state.

A cancelled ride must not enter bidding.

---

# 43. Cancellation During Bidding

If the rider cancels while drivers are bidding:

```text
BIDDING
   ↓
CANCELLED
```

Active bids become non-selectable.

Drivers must receive the appropriate event so they stop acting on the
opportunity.

---

# 44. Completion of Request Data

The request's core fields should remain available after completion for:

```text
receipts
support
analytics
trip history
rating context
```

Historical data should not depend on the continued availability of a Google
Place result.

---

# 45. What We Should Not Build Yet

Do not add these to the ride request prematurely:

```text
full route geometry
payment transaction state
rating state
live GPS history
driver assignment state
complex pricing rules
fraud score
large Google Maps response payloads
scheduled-ride machinery if MVP is immediate-only
```

Those belong to their respective domains.

---

# 46. Future Extensions

The request model should leave room for:

```text
scheduled rides
multi-stop rides
special accessibility requirements
multiple service categories
corporate/business rides
advanced ride options
```

But future compatibility does not justify implementing these now.

---

# 47. Design Principles

1. A ride request represents rider intent.
2. The request belongs to one authenticated rider.
3. Pickup and destination coordinates are authoritative geographic primitives.
4. Human-readable addresses are presentation/support data.
5. Google Maps is an external provider, not the ride's source of truth.
6. Place IDs must not replace coordinates.
7. Core request intent becomes immutable once dispatch begins.
8. Actual trip locations are separate from requested locations.
9. Service requirements belong to the request; driver eligibility evaluates them.
10. Driver, reservation, and assignment data do not belong in the request.
11. Payment state does not belong in the request.
12. Live location telemetry does not belong in the request.
13. Server-side validation is authoritative.
14. Ride creation must be idempotent.
15. Ride creation and initial state creation are transactional.
16. Discovery is triggered server-side after successful creation.
17. PostgreSQL remains the durable source of truth.
18. Sensitive location data must be role-protected.
19. Do not store external provider payloads unnecessarily.
20. Do not introduce speculative fields or subsystems before the product requires them.
