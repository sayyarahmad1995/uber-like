# Actors and Permissions

## 1. Purpose

This document defines the actors in the ride-hailing platform and the
permissions required to perform important actions.

The backend is authoritative for authorization. The Flutter application may
show or hide functionality based on the user's current mode, but hiding a UI
control is not a security boundary.

---

## 2. Actors

The initial system has four meaningful actors:

```text
User
 ├── Rider capability
 └── Driver capability

Driver capability
 └── Vehicle

Platform Administrator

External OIDC Provider
```

The OIDC provider authenticates the user but is not an application-level
actor with access to rides or other business operations.

---

## 3. User

A User is the application's internal identity associated with an external
OIDC subject.

A user may have:

- Rider capability
- Driver capability
- Both capabilities

Having a driver capability does not automatically mean that the user may
operate as an active driver. Driver approval and operational availability
are separate concepts.

---

## 4. Rider

A rider may:

- View and manage their own profile
- Select pickup and destination
- Request a ride
- View their own active ride
- View the assigned driver's relevant trip information
- Cancel their own ride when the current ride state permits cancellation
- View their own ride history

A rider must not be able to:

- Modify another user's profile
- Accept a ride for a driver
- Change another driver's availability
- Change ride state on behalf of a driver
- Access another user's location unless explicitly permitted by the active
  ride relationship
- Arbitrarily set a ride's state

---

## 5. Driver

A driver must have an approved driver capability before performing driver
operations.

An eligible driver may:

- View and manage their driver profile
- View and manage their eligible vehicle information
- Go online
- Go offline
- Receive ride offers
- Accept an offered ride
- Reject or allow an offer to expire
- Report arrival at the pickup location
- Start the assigned trip
- Complete the assigned trip
- Send their current location while operationally active
- View their own driver trip history

A driver must not be able to:

- Accept a ride that was not offered to them
- Accept a ride already assigned to another driver
- Start a ride that is not assigned to them
- Complete a ride that is not assigned to them
- Modify another driver's availability
- Impersonate another driver
- Arbitrarily set their driver status to an invalid state
- Access unrelated riders' private information

---

## 6. Platform Administrator

An administrator represents trusted platform operations.

Administrative capabilities are intentionally broad but should be explicitly
scoped as the admin system is designed.

Potential administrative operations include:

- Review driver applications
- Approve or reject drivers
- Suspend users or drivers
- Manage vehicles when required for operations
- Inspect rides and ride events
- Cancel rides for operational reasons
- Investigate disputes and incidents

Administrative permissions should not be implemented as a single unrestricted
boolean if the administration system grows. More granular permissions can be
introduced when administrative requirements are defined.

---

## 7. Rider and Driver Mode

The Flutter application contains a single application with a mode switch.

```text
                 User
                   │
          ┌────────┴────────┐
          │                 │
     Rider Mode        Driver Mode
```

Changing the UI mode does not change the user's identity or grant new
permissions.

When a user selects Driver Mode, the backend must determine whether the user
has an approved and eligible driver capability.

Likewise, entering Rider Mode does not require the user to stop being an
approved driver.

The active application mode is therefore a client experience, while actual
authorization remains server-side.

---

## 8. Core Permission Matrix

| Action | Rider | Driver | Admin |
|---|---:|---:|---:|
| View own profile | Yes | Yes | Yes |
| Request ride | Yes | Yes* | Yes* |
| Cancel own ride | Yes | No** | Yes |
| Go online | No | Yes | Yes |
| Go offline | No | Yes | Yes |
| Receive ride offer | No | Yes | No |
| Accept assigned offer | No | Yes | No |
| Report driver arrival | No | Yes | No |
| Start assigned trip | No | Yes | No |
| Complete assigned trip | No | Yes | No |
| Send driver location | No | Yes | No |
| View own ride history | Yes | Yes | Yes |
| Approve driver | No | No | Yes |
| Suspend user/driver | No | No | Yes |
| Inspect ride events | Own rides | Own rides | Yes |

`*` These permissions are not necessarily part of the initial MVP and must
be explicitly defined before implementation. A driver account may also act as
a rider because the platform uses a single user identity.

`**` Driver cancellation rules will be defined separately. A driver may need
to cancel/reject an assigned ride, but that is a different business action
from a rider cancellation.

---

## 9. Ride Ownership and Assignment

Authorization for ride operations depends on the relationship between the
actor and the ride.

For example:

```text
Rider action
    ↓
Is authenticated user the rider of this ride?
    ↓
Is the requested transition valid?
    ↓
Allow
```

For a driver action:

```text
Driver action
    ↓
Is authenticated user an approved driver?
    ↓
Is this driver assigned/offered this ride?
    ↓
Is the requested transition valid?
    ↓
Allow
```

Being authenticated is never sufficient by itself to perform a ride action.

---

## 10. Location Privacy

Location data requires relationship-based authorization.

A rider may receive the assigned driver's relevant current location during
an active ride relationship.

A driver may receive the rider's relevant pickup/location information needed
to perform the assigned ride.

Users must not be able to query arbitrary users' live locations.

The exact location visibility rules and retention policy will be defined in
the real-time and location design.

---

## 11. Backend Authorization Rules

Every protected operation should be evaluated against at least:

1. Authentication
2. Actor capability
3. Resource ownership or assignment
4. Current resource state
5. Requested action

Conceptually:

```text
Authenticated?
      ↓
Has required capability?
      ↓
Owns or is assigned to resource?
      ↓
Is current state compatible with action?
      ↓
Authorized
```

The backend must reject unauthorized operations even when a malicious client
constructs the request manually.

---

## 12. Authorization vs Application Mode

The application must not treat the following as authorization:

```text
currentMode == DRIVER
```

The backend should instead derive authorization from persistent account state
and the requested operation.

For example:

```text
POST /drivers/online

Authenticated user
    ↓
Has approved driver capability?
    ↓
Has eligible vehicle?
    ↓
Is driver currently allowed to operate?
    ↓
Set driver availability to ONLINE
```

---

## 13. Deferred Decisions

The following authorization details remain intentionally open:

- Exact administrator roles
- Driver onboarding and approval workflow
- Driver suspension rules
- Driver cancellation permissions and penalties
- Whether drivers can request rides while online
- Whether multiple vehicles can be active simultaneously
- Fine-grained administrative permissions
- Location retention and historical-location access
- Payment-related permissions

These should be resolved when the corresponding domains are designed.

---

## 14. Principle

Authorization should be based on **who the user is, what capability they
have, their relationship to the resource, and the current state of that
resource**.

The client controls presentation. The backend controls authorization.