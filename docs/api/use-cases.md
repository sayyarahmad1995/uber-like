# Backend Use Cases

## 1. Purpose

This document defines the application's backend use cases before defining
HTTP endpoints, request/response schemas, database tables, or WebSocket
protocols.

A use case represents a meaningful operation that the backend performs for an
actor.

The API layer will later expose these use cases through REST, WebSocket, or
other appropriate mechanisms.

---

## 2. Use Case Categories

The initial backend is divided into:

```text
Authentication
Users
Drivers
Vehicles
Rides
Bidding
Trips
Locations
```

Payments, ratings, promotions, and other future domains are intentionally
excluded until their requirements are defined.

---

# 3. Authentication

Authentication is provided by the external OIDC provider.

The backend does not implement password authentication or act as an identity
provider.

## UC-AUTH-001 — Authenticate User

**Actor:** User

The user authenticates through the external OIDC provider.

The backend validates the resulting identity/token and associates the external
OIDC subject with an internal User.

### Preconditions

- OIDC provider is available.
- The supplied identity/token is valid.

### Result

- The backend can identify the internal User.
- The user may access authorized application functionality.

---

## UC-AUTH-002 — Get Current User

**Actor:** Authenticated User

Returns the authenticated user's internal identity and relevant account
information.

### Result

The backend returns:

- Internal user ID
- Profile information
- Available capabilities
- Driver status, if applicable
- Other information required by the application

The response must not expose private/internal authentication information.

---

# 4. User

## UC-USER-001 — Get Own Profile

**Actor:** Authenticated User

Returns the user's own profile.

A user may only retrieve their own private profile through this use case.

---

## UC-USER-002 — Update Own Profile

**Actor:** Authenticated User

Updates fields the user is permitted to modify.

The backend validates all supplied fields.

Users cannot modify protected fields such as:

- Internal user ID
- OIDC subject
- Driver approval status
- Administrative permissions

---

# 5. Driver

## UC-DRIVER-001 — Apply as Driver

**Actor:** Authenticated User

Creates or starts a driver application.

The application enters a pending state until the platform approves or rejects
the driver.

### Result

```text
Driver approval status = PENDING
```

---

## UC-DRIVER-002 — Review Driver Application

**Actor:** Administrator

Retrieves driver application information for operational review.

---

## UC-DRIVER-003 — Approve Driver

**Actor:** Administrator

Approves an eligible driver application.

### Result

```text
Driver approval status = APPROVED
```

The user may now perform approved driver operations.

---

## UC-DRIVER-004 — Reject Driver

**Actor:** Administrator

Rejects a driver application.

The reason and audit requirements will be defined later.

---

## UC-DRIVER-005 — Suspend Driver

**Actor:** Administrator

Suspends a driver.

A suspended driver cannot participate in bidding or operate as an active
driver.

Existing active assignments require separate cancellation/reassignment
handling.

---

## UC-DRIVER-006 — Get Own Driver Profile

**Actor:** Approved or pending Driver

Returns the authenticated user's driver profile and current driver status.

---

## UC-DRIVER-007 — Go Online

**Actor:** Approved Driver

Makes the driver available for eligible ride bidding.

### Preconditions

- Driver is approved.
- Driver is not suspended.
- Driver has an eligible vehicle.
- Driver is permitted to operate.

### Result

```text
Driver availability = ONLINE
```

---

## UC-DRIVER-008 — Go Offline

**Actor:** Driver

Stops the driver from receiving new bidding opportunities.

A driver with an active ride must follow the applicable operational rules
rather than arbitrarily becoming unavailable.

---

# 6. Vehicle

## UC-VEHICLE-001 — Add Vehicle

**Actor:** Approved Driver

Adds a vehicle associated with the driver's account.

The vehicle must satisfy the platform's required information and eligibility
rules.

---

## UC-VEHICLE-002 — Update Own Vehicle

**Actor:** Driver

Updates permitted vehicle information.

Changes affecting eligibility may require revalidation.

---

## UC-VEHICLE-003 — Remove Vehicle

**Actor:** Driver

Removes or deactivates a vehicle that is no longer being used.

A vehicle currently associated with an active trip cannot simply be removed.

---

## UC-VEHICLE-004 — Set Active Vehicle

**Actor:** Driver

Selects which eligible vehicle is used for operational driver activity.

The exact multiple-vehicle rules will be refined later.

---

# 7. Ride

## UC-RIDE-001 — Request Ride

**Actor:** Rider

Creates a ride request containing at minimum:

- Pickup
- Destination
- Requested vehicle/service type, if applicable

The backend:

1. Validates the request.
2. Calculates a reference fare.
3. Creates the Ride.
4. Opens the bidding process.

### Result

```text
Ride status = BIDDING
```

The rider receives the information necessary to participate in the bidding
process.

This operation should be idempotent where appropriate to prevent accidental
duplicate ride requests.

---

## UC-RIDE-002 — Get Active Ride

**Actor:** Rider or assigned Driver

Returns the authenticated user's currently relevant active ride.

Authorization depends on the user's relationship to the ride.

---

## UC-RIDE-003 — Get Ride

**Actor:** Rider, assigned Driver, or authorized Administrator

Returns a ride according to authorization rules.

A rider can access their own rides.

A driver can access rides assigned to them.

Administrators may access rides according to administrative permissions.

---

## UC-RIDE-004 — Cancel Ride

**Actor:** Rider, Driver, or Administrator

Requests cancellation of a ride.

The backend verifies:

- Actor authorization
- Current ride state
- Cancellation rules
- Assignment relationship

Cancellation policy and fees are defined separately.

---

## UC-RIDE-005 — Get Ride History

**Actor:** Rider or Driver

Returns historical rides associated with the authenticated user.

The user must not be able to retrieve unrelated users' ride history.

Pagination will be required when the REST API is designed.

---

# 8. Bidding

## UC-BID-001 — Open Bidding

**Actor:** Backend

Opens bidding after a valid ride request has been created.

The backend establishes:

- Bidding start time
- Bidding deadline
- Applicable reference fare
- Applicable bidding constraints

Drivers then become eligible to participate.

---

## UC-BID-002 — Submit Bid

**Actor:** Eligible Driver

Submits a fare offer for a ride.

The backend validates:

- Authentication
- Driver approval
- Driver availability
- Vehicle eligibility
- Geographic eligibility
- Ride state
- Bidding deadline
- Bid amount
- Existing active bid

A driver may have only one active bid for a particular ride.

---

## UC-BID-003 — Modify Bid

**Actor:** Driver who owns the bid

Changes the driver's active bid while bidding remains open.

The new amount must pass the same pricing and eligibility validation as a
new bid.

The backend may rate-limit repeated modifications.

---

## UC-BID-004 — Withdraw Bid

**Actor:** Driver who owns the bid

Withdraws an active bid while bidding remains open.

A withdrawn bid cannot later be selected.

---

## UC-BID-005 — List Ride Bids

**Actor:** Rider who owns the ride

Returns the bids that the rider is permitted to see.

The response may include:

- Driver information
- Driver rating
- Vehicle information
- Estimated arrival
- Bid amount

The API must not expose competing-bid information to drivers.

---

## UC-BID-006 — Close Bidding

**Actor:** Backend

Closes bidding when the bidding deadline is reached or when another valid
business condition requires bidding to close.

After bidding closes:

- New bids are rejected.
- Bid modifications are rejected.
- Bid withdrawals are rejected.

The backend is authoritative for the deadline.

---

## UC-BID-007 — Select Bid

**Actor:** Rider who owns the ride

Selects one eligible bid after bidding closes.

The backend revalidates:

- Bid state
- Driver approval
- Driver availability
- Vehicle eligibility
- Geographic eligibility
- Driver commitment status
- Ride state

The operation must atomically prevent another rider or concurrent request
from successfully selecting the same driver.

### Result

```text
Ride
├── selected driver
├── selected vehicle
└── agreed fare
```

The selected driver enters the confirmation phase.

---

## UC-BID-008 — Confirm Assignment

**Actor:** Selected Driver

Confirms the assignment offered by the rider.

The backend verifies:

- Driver identity
- Ride assignment
- Confirmation deadline
- Ride state
- Driver eligibility

### Result

```text
Ride status = DRIVER_CONFIRMED
```

The driver becomes committed to the ride.

---

## UC-BID-009 — Reject Assignment

**Actor:** Selected Driver

Rejects the selected assignment.

The backend invalidates the current assignment and determines whether another
valid bid can be selected.

The rider should not need to restart the entire bidding process if a valid
fallback bid exists.

---

## UC-BID-010 — Handle Assignment Timeout

**Actor:** Backend

Handles a selected driver's failure to confirm before the confirmation
deadline.

The backend:

1. Invalidates the expired selection.
2. Revalidates remaining bids.
3. Attempts fallback selection when permitted.
4. Otherwise ends the ride as unsuccessful.

---

# 9. Trip

Once a driver has confirmed the assignment, the ride proceeds into the
operational trip lifecycle.

## UC-TRIP-001 — Report Arrival

**Actor:** Assigned Driver

Reports that the driver has arrived at the pickup location.

The backend verifies:

- Driver assignment
- Current ride state
- Driver authorization
- Required arrival conditions

### Result

```text
Ride status = DRIVER_ARRIVED
```

---

## UC-TRIP-002 — Start Trip

**Actor:** Assigned Driver

Starts the trip after the required pickup conditions have been satisfied.

### Result

```text
Ride status = TRIP_STARTED
```

---

## UC-TRIP-003 — Complete Trip

**Actor:** Assigned Driver

Completes the trip after the required completion conditions have been
satisfied.

### Result

```text
Ride status = TRIP_COMPLETED
```

Payment settlement is a separate domain and is not defined by this use case.

---

# 10. Location

## UC-LOCATION-001 — Publish Driver Location

**Actor:** Active Driver

Publishes the driver's current location while the driver is operationally
active.

The backend validates:

- Driver authentication
- Driver availability
- Driver assignment/context
- Location format
- Update frequency/rate limits

Current operational location is primarily ephemeral state.

Redis is a candidate storage mechanism for this information.

---

## UC-LOCATION-002 — Subscribe to Ride Location

**Actor:** Rider or assigned Driver

Receives location updates that the actor is authorized to see.

Examples:

```text
Rider → assigned driver's location

Driver → relevant rider/pickup location
```

Users must not be able to subscribe to arbitrary users' live locations.

The exact WebSocket contract will be defined separately.

---

# 11. Real-Time Operations

Real-time communication is not itself the source of truth.

The backend persists authoritative business state separately.

Examples of server events include:

```text
BIDDING_STARTED
BID_RECEIVED
BID_UPDATED
BID_WITHDRAWN
BIDDING_CLOSED
DRIVER_SELECTED
DRIVER_CONFIRMED
DRIVER_CONFIRMATION_FAILED
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
DRIVER_LOCATION_UPDATED
```

A client reconnecting after missing an event must be able to recover current
state through normal backend queries.

---

# 12. Authorization Model

Every protected use case must evaluate:

```text
Authentication
      ↓
Capability
      ↓
Resource relationship
      ↓
Current resource state
      ↓
Requested operation
```

Examples:

### Submit bid

```text
Authenticated
    AND
Approved driver
    AND
Eligible vehicle
    AND
Online
    AND
Ride is accepting bids
```

### Select bid

```text
Authenticated
    AND
User is rider of ride
    AND
Ride is selectable
    AND
Bid is selectable
    AND
Driver is eligible
```

### Complete trip

```text
Authenticated
    AND
User is assigned driver
    AND
Ride is in valid pre-completion state
```

---

# 13. Idempotency

Operations that create or transition important business state should be
designed with retry behavior in mind.

Potentially idempotent operations include:

- Request ride
- Submit bid
- Modify bid
- Withdraw bid
- Select bid
- Confirm assignment
- Cancel ride
- Start trip
- Complete trip

The exact idempotency-key mechanism will be defined in the API contract.

The goal is to prevent network retries from producing duplicate or conflicting
business operations.

---

# 14. Concurrency-Sensitive Operations

The following operations require special concurrency handling:

- Creating a ride
- Submitting/updating a bid
- Selecting a bid
- Reserving a driver
- Confirming an assignment
- Going online/offline
- Starting a trip
- Completing a trip

The backend must not rely on Flutter or WebSocket message ordering to maintain
business correctness.

PostgreSQL transactions and appropriate locking/concurrency controls will be
used where durable state must change atomically.

Redis may assist with ephemeral coordination, but PostgreSQL remains the
authoritative source for durable business state.

---

# 15. Errors

Use cases should return domain-level errors rather than exposing database or
infrastructure errors directly.

Examples:

```text
UNAUTHORIZED
FORBIDDEN
RIDE_NOT_FOUND
RIDE_NOT_ACTIVE
INVALID_RIDE_STATE
BIDDING_CLOSED
BID_NOT_FOUND
BID_NOT_SELECTABLE
BID_LIMIT_EXCEEDED
DRIVER_NOT_ELIGIBLE
VEHICLE_NOT_ELIGIBLE
DRIVER_ALREADY_COMMITTED
ASSIGNMENT_EXPIRED
ASSIGNMENT_REJECTED
INVALID_STATE_TRANSITION
```

The API layer will later map these domain errors to appropriate HTTP or
WebSocket responses.

---

# 16. Synchronous vs Real-Time

The distinction is:

```text
Command
    ↓
Client explicitly asks backend to perform an operation.

Event
    ↓
Backend informs clients that something happened.
```

Examples:

```text
Submit Bid
    → Command

Bid Received
    → Event

Select Bid
    → Command

Driver Selected
    → Event

Confirm Assignment
    → Command

Driver Confirmed
    → Event
```

A WebSocket event must never be treated as proof that a command succeeded.

The command response or subsequent authoritative state query determines the
result.

---

# 17. Deferred Use Cases

The following are intentionally excluded:

- Payment authorization
- Payment capture
- Refunds
- Driver payouts
- Ratings
- Reviews
- Promotions
- Scheduled rides
- Multi-stop rides
- Corporate accounts
- Subscriptions
- Referral systems
- Customer support workflows

They will be introduced when their domain requirements are defined.

---

# 18. Design Principle

Use cases describe **what the system must accomplish**, not how it is
implemented.

The following layers should remain separate:

```text
Transport
    ↓
Application / Use Cases
    ↓
Domain
    ↓
Infrastructure
```

REST, WebSocket, PostgreSQL, Redis, Google Maps, and the OIDC provider are
implementation/integration concerns.

The domain use cases must not depend directly on transport-specific details.
