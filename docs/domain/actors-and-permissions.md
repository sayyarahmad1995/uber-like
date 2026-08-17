# Actors and Permissions

## 1. Purpose

This document defines the actors, capabilities, and authorization rules of
the ride-hailing platform.

The backend is authoritative for authorization. The Flutter application may
show or hide functionality based on the user's current mode, but UI state is
never a security boundary.

---

## 2. Actors

The initial system has three application actors and one external dependency:

```text
User
 ├── Rider capability
 └── Driver profile/capability
       └── Vehicle(s)

Platform Administrator

External OIDC Provider
```

The OIDC provider authenticates users but does not receive application-level
permissions over rides, bids, drivers, or vehicles.

---

## 3. User

A User is the application's internal identity associated with an external
OIDC subject.

A user may operate as a rider and may also have a driver profile.

The same User identity is used in both modes.

---

## 4. Rider Capabilities

A rider may:

- View and manage their own profile
- Select pickup and destination
- Request a ride
- View the reference fare
- View valid bids for their ride
- Select one valid bid
- View the selected driver's relevant trip information
- Cancel their own ride when the lifecycle permits it
- View their own ride history

A rider must not be able to:

- Submit a bid as another driver
- Modify or withdraw another driver's bid
- Change the agreed fare directly
- Assign a driver without selecting a valid bid
- Change another user's availability
- Arbitrarily set a ride state
- Access arbitrary drivers' live locations
- Access unrelated riders' private information

---

## 5. Driver Capability

A user must have an approved driver profile to perform driver operations.

An eligible driver may:

- View and manage their driver profile
- Manage their eligible vehicle information
- Go online
- Go offline
- Participate in eligible bidding opportunities
- Submit one active bid per ride
- Modify their active bid while bidding is open
- Withdraw their active bid while bidding is open
- Receive notification when selected
- Confirm a selected ride
- Report arrival at pickup
- Start the assigned trip
- Complete the assigned trip
- Send current operational location
- View their own driver ride history

A driver must not be able to:

- Bid on an ineligible ride
- Submit multiple active bids for the same ride
- Submit or modify a bid after the bidding deadline
- See competing drivers' bid amounts
- Accept a ride that was not selected for them
- Confirm a ride after their confirmation deadline
- Start a ride that is not assigned to them
- Complete a ride that is not assigned to them
- Modify another driver's availability
- Impersonate another driver
- Access unrelated riders' private information
- Access arbitrary users' live locations

---

## 6. Driver Eligibility

A driver may participate in bidding only when the backend confirms the driver
is eligible.

Initial eligibility conditions are:

```text
Authenticated
     ↓
Approved driver profile
     ↓
Not suspended
     ↓
Eligible vehicle
     ↓
Operationally online
     ↓
Geographically eligible
     ↓
Not committed to an active ride
     ↓
Eligible for the ride's requirements
```

Eligibility is checked when the driver submits or changes a bid and is
re-checked when the rider selects the bid.

The client must never be the authority for eligibility.

---

## 7. Driver Availability

Approval and availability are separate concepts.

```text
Driver approval:
PENDING / APPROVED / SUSPENDED / REJECTED

Driver availability:
OFFLINE / ONLINE / BUSY
```

A driver must be approved and operationally eligible before going online.

A driver who becomes committed to an active ride becomes unavailable for
additional assignments, even if the client still displays an online state.

---

## 8. Bidding Permissions

### Submit bid

Allowed for an eligible driver while the ride's bidding window is open.

The backend validates:

- Driver identity
- Driver eligibility
- Ride state
- Bidding deadline
- Existing active bid
- Bid amount limits

### Modify bid

Allowed only for the driver's own active bid and only while bidding is open.

### Withdraw bid

Allowed only for the driver's own active bid and only while bidding is open.

### View bids

A rider may view valid bids for their own ride.

A driver may view their own bid but must not see competing drivers' bid
amounts.

### Select bid

Only the rider associated with the ride may select a bid.

Selection requires a fresh backend eligibility check and must atomically
establish the driver and agreed fare.

---

## 9. Driver Selection and Confirmation

Selecting a bid does not immediately mean the driver has confirmed the ride.

The flow is:

```text
Rider selects driver's bid
        ↓
Backend validates bid and driver
        ↓
Driver is temporarily selected/reserved
        ↓
Driver receives confirmation request
        ↓
Driver confirms within deadline
        ↓
Driver becomes committed to the ride
```

If the driver does not confirm before the deadline, the backend may attempt a
valid fallback bid according to the ride lifecycle rules.

The driver cannot arbitrarily reject a ride after selection without creating a
recorded cancellation/failure event.

---

## 10. Rider and Driver Mode

The Flutter application is a single application with a mode switch.

```text
                 User
                   │
          ┌────────┴────────┐
          │                 │
     Rider Mode        Driver Mode
```

Changing the mode changes the application experience. It does not grant or
remove authorization.

For example:

```text
currentMode == DRIVER
```

is not sufficient to perform driver operations.

The backend determines whether the authenticated User has an approved and
eligible Driver profile.

---

## 11. Simultaneous Rider and Driver Activity

Initially, a user must not be committed to an active driver ride and an active
rider ride simultaneously.

This prevents contradictory operational situations such as:

```text
User is transporting Rider A
        +
Same User requests a ride as Rider B
```

This rule may be revisited if product requirements change.

---

## 12. Platform Administrator

An administrator represents trusted platform operations.

Potential administrative capabilities include:

- Review driver applications
- Approve or reject drivers
- Suspend users or drivers
- Inspect rides and bids
- Inspect ride events
- Cancel rides for operational reasons
- Investigate disputes and incidents

The administrator model is intentionally not fully specified yet.

If the administration system grows, permissions should become granular rather
than being represented by one unrestricted boolean role.

---

## 13. Ride Ownership

A ride operation must consider the authenticated user's relationship to the
ride.

For a rider action:

```text
Authenticated?
      ↓
Is this the ride's rider?
      ↓
Is the requested action valid for the current state?
      ↓
Allow
```

For a driver action:

```text
Authenticated?
      ↓
Approved driver?
      ↓
Is this driver the selected/assigned driver?
      ↓
Is the requested action valid for the current state?
      ↓
Allow
```

Being authenticated alone is never sufficient.

---

## 14. Location Privacy

Location access is relationship-based.

Before a rider selects a bid, the rider may see driver information such as
ETA, but should not receive arbitrary precise live coordinates.

After a driver is selected/confirmed, the rider may receive the driver's
relevant live location for the active ride.

The driver may receive the pickup and other location information required to
perform the assigned ride.

Users must not be able to query arbitrary users' live locations.

Exact location visibility and retention rules will be defined in the
real-time/location design.

---

## 15. Backend Authorization Model

Every protected operation should be evaluated against at least:

1. Authentication
2. Capability or role
3. Resource ownership or assignment
4. Current resource state
5. Requested action
6. Relevant time/deadline constraints

Conceptually:

```text
Authenticated?
      ↓
Has required capability?
      ↓
Owns or is assigned to resource?
      ↓
Within required time window?
      ↓
Is current state compatible with action?
      ↓
Authorized
```

The backend must reject unauthorized operations even when a malicious client
constructs the request manually.

---

## 16. Important Security Principle

The following are not authorization mechanisms:

```text
currentMode == RIDER
currentMode == DRIVER
button is hidden
screen is inaccessible
```

The server must enforce every important rule independently of client UI
behavior.

---

## 17. Deferred Decisions

The following authorization details remain intentionally open:

- Exact administrator roles
- Driver onboarding and approval workflow
- Driver suspension rules
- Driver cancellation penalties
- Detailed vehicle verification
- Exact location retention policy
- Payment-related permissions
- Fine-grained administrative permissions

These will be defined with their respective domains.

---

## 18. Principle

Authorization is based on **who the user is, what capability they have,
their relationship to the resource, the current state of that resource, and
any applicable deadlines**.

The client controls presentation. The backend controls authorization.
