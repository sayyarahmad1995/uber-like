# Ride Lifecycle

## 1. Purpose

This document defines the authoritative lifecycle of a Ride and the valid
business transitions.

The platform uses a driver fare-bidding marketplace rather than automatically
assigning the nearest driver.

The backend is authoritative over Ride state. The Flutter application may
request actions but cannot directly set arbitrary states.

---

## 2. Ride Lifecycle States

Initial durable Ride states:

```text
REQUESTED
BIDDING
DRIVER_SELECTED
DRIVER_CONFIRMED
DRIVER_ARRIVING
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
NO_DRIVER_FOUND
CANCELLED
```

The bidding process has its own internal data such as bidding deadlines and
bid states. Those details should not be confused with the Ride's authoritative
lifecycle.

---

## 3. Primary Flow

```text
REQUESTED
    │
    ▼
BIDDING
    │
    │ rider selects a valid bid
    ▼
DRIVER_SELECTED
    │
    │ selected driver confirms
    ▼
DRIVER_CONFIRMED
    │
    ▼
DRIVER_ARRIVING
    │
    ▼
DRIVER_ARRIVED
    │
    ▼
TRIP_STARTED
    │
    ▼
TRIP_COMPLETED
```

---

## 4. Request Creation

A rider creates a Ride request containing at least:

- Pickup location
- Destination location
- Required ride/vehicle characteristics where applicable

The backend validates the request and calculates a reference fare before
opening bidding.

The lifecycle then becomes:

```text
REQUESTED → BIDDING
```

---

## 5. Bidding

During `BIDDING`:

- Eligible drivers may submit one active bid for the Ride.
- Drivers may modify their own active bid.
- Drivers may withdraw their own active bid.
- Drivers cannot see competing bid amounts.
- The rider can see valid bids for their own Ride.
- The rider cannot select an invalid, withdrawn, or expired bid.
- New bids and bid modifications are rejected after the bidding deadline.

The bidding deadline is controlled by the backend.

The exact duration is configurable and intentionally not fixed in this
architecture document.

---

## 6. Bid Selection

The rider selects one valid bid.

The backend must re-check:

- The Ride is still selectable.
- The bid is still valid.
- The driver is still eligible.
- The driver's vehicle is still eligible.
- The relevant deadline has not expired.

The backend must atomically establish the selected driver and agreed fare.

The transition is:

```text
BIDDING → DRIVER_SELECTED
```

A bid selection is not equivalent to driver confirmation.

---

## 7. Driver Confirmation

After selection, the driver receives a confirmation request.

```text
DRIVER_SELECTED
       │
       ├── confirms before deadline → DRIVER_CONFIRMED
       │
       └── timeout / failure → fallback selection
```

The exact confirmation timeout is configurable.

The selected driver is temporarily reserved while confirmation is pending so
that another rider cannot successfully commit the same driver at the same
time.

---

## 8. Fallback After Confirmation Failure

If the selected driver does not confirm within the deadline, the platform may
attempt another still-valid bid from the same Ride.

Conceptually:

```text
DRIVER_SELECTED
       │
       ▼
confirmation fails
       │
       ▼
select next valid bid
       │
       ▼
DRIVER_SELECTED
```

A fallback driver must pass a fresh eligibility check.

If no valid bids remain:

```text
DRIVER_SELECTED → NO_DRIVER_FOUND
```

The rider should not be required to restart the entire request merely because
the first selected driver failed to confirm.

---

## 9. Driver Arrival

Once the driver has confirmed the Ride, the driver may report progress toward
the pickup location.

```text
DRIVER_CONFIRMED
       │
       ▼
DRIVER_ARRIVING
       │
       ▼
DRIVER_ARRIVED
```

Only the selected/confirmed driver may perform these driver actions.

---

## 10. Trip Start and Completion

The trip begins when the selected driver starts the ride:

```text
DRIVER_ARRIVED → TRIP_STARTED
```

The trip ends when the driver completes the ride:

```text
TRIP_STARTED → TRIP_COMPLETED
```

The backend must validate that the driver is the assigned driver and that the
current Ride state permits the requested transition.

---

## 11. Rider Cancellation

The rider may request cancellation from eligible pre-completion states.

Initial cancellation paths are:

```text
REQUESTED       → CANCELLED
BIDDING         → CANCELLED
DRIVER_SELECTED → CANCELLED
DRIVER_CONFIRMED → CANCELLED
DRIVER_ARRIVING → CANCELLED
DRIVER_ARRIVED  → CANCELLED
```

Whether a cancellation creates a fee is a future pricing/payment policy and
is not part of the Ride state transition itself.

A completed Ride cannot be cancelled through the normal rider cancellation
operation.

---

## 12. Driver Cancellation

Driver cancellation is different from rider cancellation.

A driver who has merely submitted a bid can withdraw the bid while bidding is
open. This does not cancel the Ride.

After driver selection/confirmation, a driver cancellation becomes a ride
assignment failure and must be recorded as an event.

The platform may attempt to use another still-valid bid where possible.

If no valid alternative exists, the Ride may transition to `NO_DRIVER_FOUND`
or `CANCELLED` according to the point in the lifecycle and the applicable
business policy.

The exact cancellation/penalty policy is deferred.

---

## 13. No Driver Found

The Ride enters `NO_DRIVER_FOUND` when the platform cannot establish a valid
driver assignment from the available bidding opportunity.

Examples:

```text
BIDDING
   │
   └── bidding deadline reached with no valid bids
           ↓
      NO_DRIVER_FOUND
```

or:

```text
DRIVER_SELECTED
   │
   └── selected driver fails
       and no valid fallback bids remain
           ↓
      NO_DRIVER_FOUND
```

`NO_DRIVER_FOUND` is a terminal Ride state for that request.

A future product flow may allow the rider to create a new request.

---

## 14. Bidding Deadline

The backend owns the bidding clock.

Once the deadline has passed:

- New bids are rejected.
- Bid modifications are rejected.
- Bid withdrawals may be restricted according to the exact point in the
  closing process.
- The rider can no longer select a newly submitted bid.

The system must not rely on the Flutter client's clock to enforce these rules.

---

## 15. Connectivity Failure

Network or WebSocket failure does not automatically change the Ride state.

For example:

```text
TRIP_STARTED
     │
     └── driver loses connectivity
            ↓
       TRIP_STARTED
```

Connectivity is an operational condition, not a business lifecycle state.

Recovery, stale location handling, and reconnection behavior will be defined
in the real-time/location design.

---

## 16. Driver Concurrency

A driver may participate in multiple open bidding opportunities while not
committed to an active Ride.

Once a driver is successfully selected and committed to a Ride, other active
bids must no longer be capable of producing another assignment.

Two riders selecting the same driver concurrently must result in exactly one
successful assignment.

The backend must enforce this with appropriate transactional/concurrency
controls. The client must not attempt to coordinate this itself.

---

## 17. State Transition Authority

The backend controls every Ride state transition.

The client may request commands such as:

```text
request ride
select bid
confirm ride
arrived
start
complete
cancel
```

but the backend must validate:

- Authentication
- Authorization
- Resource ownership/assignment
- Current Ride state
- Bid validity where applicable
- Driver eligibility where applicable
- Relevant deadlines

---

## 18. Ride Events

Important lifecycle and bidding actions should produce durable Ride Events.

Examples:

```text
RIDE_REQUESTED
BIDDING_OPENED
BID_SUBMITTED
BID_CHANGED
BID_WITHDRAWN
BIDDING_CLOSED
DRIVER_SELECTED
DRIVER_CONFIRMATION_REQUESTED
DRIVER_CONFIRMED
DRIVER_CONFIRMATION_EXPIRED
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
RIDER_CANCELLED
DRIVER_CANCELLED
NO_DRIVER_FOUND
```

The current Ride status represents current business truth. Ride Events
provide historical truth.

---

## 19. State Transition Summary

```text
REQUESTED
    ├── BIDDING
    └── CANCELLED

BIDDING
    ├── DRIVER_SELECTED
    ├── NO_DRIVER_FOUND
    └── CANCELLED

DRIVER_SELECTED
    ├── DRIVER_CONFIRMED
    ├── DRIVER_SELECTED  (fallback to another valid bid)
    ├── NO_DRIVER_FOUND
    └── CANCELLED

DRIVER_CONFIRMED
    ├── DRIVER_ARRIVING
    └── CANCELLED

DRIVER_ARRIVING
    ├── DRIVER_ARRIVED
    └── CANCELLED

DRIVER_ARRIVED
    ├── TRIP_STARTED
    └── CANCELLED

TRIP_STARTED
    └── TRIP_COMPLETED

TRIP_COMPLETED
    └── terminal

NO_DRIVER_FOUND
    └── terminal

CANCELLED
    └── terminal
```

The repeated `DRIVER_SELECTED → DRIVER_SELECTED` transition represents a
logical fallback to another valid bid. The implementation may represent this
as an internal assignment change while keeping the Ride in the same state.

---

## 20. Deferred Decisions

The following remain intentionally open:

- Exact bidding duration
- Exact driver confirmation timeout
- Minimum and maximum bid formulas
- Exact driver cancellation penalties
- Whether bidding can reopen after all fallback bids fail
- Payment/cancellation fees
- Detailed stale-driver handling
- Emergency and administrative cancellation states

These should be defined when the corresponding domains are designed.
