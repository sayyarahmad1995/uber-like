# Domain Model

## 1. Purpose

This document defines the initial business entities and relationships of the
ride-hailing platform.

The model is intentionally minimal. New entities should be introduced when a
real business requirement requires them rather than being added speculatively.

---

## 2. Core Model

```text
                         USER
                          │
              ┌───────────┴───────────┐
              │                       │
         Rider capability        DRIVER PROFILE
                                      │
                              ┌───────┴───────┐
                              │               │
                         Approval       Availability
                                      │
                                   Vehicle(s)

USER ────────────────► RIDE as rider
DRIVER ──────────────► RIDE as driver after bid selection
RIDE ────────────────► BIDS
```

A user has one application identity and may have both rider and driver
capabilities.

Rider is a capability of a User rather than a separate identity entity.

Driver is a separate domain profile associated with the User because driver
operations require additional state such as approval, availability, and
vehicles.

---

## 3. User

A User represents the application's internal identity associated with an
external OIDC subject.

A User owns:

- Identity/profile information
- Rider capability by default
- Optional Driver profile
- Their ride history as rider

The internal user ID is the application's stable identifier. The external
OIDC subject is used to associate the external identity with that user.

---

## 4. Driver

A Driver represents the driver's operational profile associated with a User.

Driver approval and operational availability are separate concepts.

### Approval status

```text
PENDING
APPROVED
SUSPENDED
REJECTED
```

### Availability status

```text
OFFLINE
ONLINE
BUSY
```

An approved driver is not necessarily online.

A suspended or rejected driver cannot become operationally available.

The exact onboarding and approval workflow is deferred.

---

## 5. Vehicle

A Vehicle belongs to a Driver.

The model should support multiple vehicles even if the MVP initially permits
only one active vehicle at a time.

Potential attributes include:

```text
id
driver_id
type
make
model
plate_number
status
```

The exact vehicle categories and verification requirements are deferred.

For an active ride, the selected driver/vehicle relationship must be valid at
the time of driver selection and confirmation.

---

## 6. Ride

A Ride is the single domain aggregate representing the customer's
transportation request and its resulting trip.

The initial model does not split the concept into separate `RideRequest` and
`Trip` aggregates.

A Ride contains concepts such as:

```text
id
rider_id
driver_id              nullable until a bid is selected
vehicle_id              nullable until a bid is selected
pickup
 destination
reference_fare
agreed_fare             nullable until a bid is selected
status
requested_at
selected_at
confirmed_at
started_at
completed_at
cancelled_at
```

The exact database representation will be defined separately.

---

## 7. Bid

A Bid is an offer made by an eligible Driver to provide a specific Ride at a
specific fare.

Conceptually:

```text
Bid
├── ride_id
├── driver_id
├── vehicle_id
├── amount
├── status
├── submitted_at
└── updated_at
```

A driver may have only one active bid for a ride.

The driver may modify or withdraw that bid while bidding is open.

A bid does not assign the driver to the ride.

The driver becomes associated with the ride only after the rider selects the
bid and the backend successfully establishes the assignment.

### Bid status

Initial persisted states:

```text
ACTIVE
WITHDRAWN
SELECTED
```

Bid expiry is primarily determined by the ride's bidding deadline rather than
requiring every bid to be individually updated when the deadline passes.

The final persistence behavior will be defined during database design.

---

## 8. Reference Fare and Agreed Fare

The platform calculates a reference fare before bidding begins.

Example:

```text
Reference fare: PKR 1,200
```

Eligible drivers submit bids within platform-defined minimum and maximum
limits.

The rider's selected bid determines the agreed fare:

```text
reference_fare = PKR 1,200
selected_bid   = PKR 1,100
agreed_fare    = PKR 1,100
```

The exact pricing algorithm and bid boundaries are deferred.

---

## 9. Ride Event

A Ride Event records important historical actions or state transitions
associated with a Ride.

Examples include:

```text
RIDE_REQUESTED
BIDDING_OPENED
BID_SUBMITTED
BID_CHANGED
BID_WITHDRAWN
BIDDING_CLOSED
DRIVER_SELECTED
DRIVER_CONFIRMED
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
RIDER_CANCELLED
DRIVER_CANCELLED
NO_DRIVER_FOUND
```

The event history provides an audit trail while the Ride's current status
represents the current business state.

```text
current status = current truth
events         = historical truth
```

The exact event schema is deferred until event and API design.

---

## 10. Location

A Location represents geographical coordinates.

At minimum:

```text
latitude
longitude
```

Pickup and destination locations are durable Ride information.

Current driver location is operational, high-frequency data and is primarily
handled through Redis rather than being written to PostgreSQL for every GPS
update.

Location visibility is governed by the actor's relationship to an active
Ride.

---

## 11. Driver Availability

Driver availability is distinct from driver approval.

```text
Approval:    APPROVED
Availability: OFFLINE
```

means the driver is eligible but not currently offering rides.

```text
Approval:    APPROVED
Availability: ONLINE
```

means the driver may participate in eligible bidding opportunities.

```text
Availability: BUSY
```

means the driver is committed to an active ride and should not be considered
for additional assignments.

The exact transition rules will be defined with the driver and dispatch
design.

---

## 12. Dispatch Relationship

Dispatch is not the owner of the Ride lifecycle.

Its responsibility is to determine which drivers are eligible to participate
in the bidding opportunity.

Conceptually:

```text
Ride
  │
  ▼
Dispatch
  │
  ├── geographic eligibility
  ├── driver approval
  ├── availability
  ├── vehicle eligibility
  └── other platform rules
          │
          ▼
      Eligible drivers
          │
          ▼
       Bidding
```

The final driver is selected through the bidding process rather than simply
being chosen by dispatch.

---

## 13. Relationships

```text
User 1 ──── 0..1 Driver
Driver 1 ──── N Vehicle
User 1 ──── N Ride (as rider)
Driver 1 ──── N Ride (historical/assigned rides)
Ride 1 ──── N Bid
Ride 1 ──── N RideEvent
```

A Ride has at most one selected Driver and one selected Vehicle at a time.

A Ride may have many bids before selection.

---

## 14. Active Bid Rules

The initial domain rules are:

1. A driver may submit only one active bid per ride.
2. A driver may modify the bid while bidding is open.
3. A driver may withdraw the bid while bidding is open.
4. A bid must satisfy platform-defined minimum and maximum limits.
5. Drivers do not see competing bids.
6. Riders can compare eligible active bids.
7. A bid cannot be submitted or modified after the bidding deadline.
8. A bid does not become an assignment until selected by the rider and
   successfully confirmed by the backend.

---

## 15. Driver Concurrency

A driver may participate in multiple open bidding opportunities while not
committed to an active ride.

Once a driver is successfully selected and committed to one ride, their other
active bids must no longer be capable of producing another assignment.

The backend must enforce this atomically.

Two riders selecting the same driver at nearly the same time must result in
exactly one successful assignment.

This is a domain-level concurrency requirement and will influence the
transaction design.

---

## 16. Deferred Domains

The following are intentionally not part of the initial core model:

- Payment
- Wallet
- Ratings implementation details
- Promotions
- Corporate accounts
- Scheduled rides
- Multi-stop rides
- Subscriptions
- Referral systems
- Detailed administration model

They may be introduced when their requirements are defined.

---

## 17. Modeling Principles

1. One User identity may have both rider and driver capabilities.
2. Rider is a capability, not a separate identity entity.
3. Driver has additional domain state and therefore has its own profile.
4. Ride is the initial aggregate for the transportation lifecycle.
5. Bid is a first-class entity associated with a Ride.
6. A bid is not an assignment.
7. PostgreSQL owns durable domain truth.
8. Redis owns high-speed ephemeral operational state.
9. State transitions should be explicit rather than represented by unrelated
   boolean flags.
