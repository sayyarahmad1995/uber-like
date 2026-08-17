# Bidding Rules

## 1. Purpose

The platform uses a driver fare-bidding marketplace to match riders with
drivers.

When a rider requests a ride:

1. The platform calculates a reference fare.
2. Eligible nearby drivers are allowed to submit bids.
3. Drivers independently choose the fare they are willing to provide the
   ride for.
4. The rider reviews eligible bids.
5. The rider selects one bid.
6. The selected driver must confirm the assignment within a defined timeout.
7. Once confirmed, the ride proceeds through the normal trip lifecycle.

The bidding system is part of the ride-matching process.

---

## 2. Reference Fare

Every ride request receives a platform-calculated reference fare.

The reference fare is based on the pricing system and may consider factors
such as:

- Distance
- Estimated duration
- Vehicle type
- Geographic area
- Other pricing rules

The exact pricing algorithm is defined separately.

Example:

```text
Estimated distance: 12 km
Estimated duration: 30 minutes

Reference fare: PKR 1,200
```

The reference fare is not necessarily the final agreed fare.

The final fare is determined by the selected driver's bid.

---

## 3. Bid

A Bid represents a driver's offer to provide a specific ride for a specific
fare.

Conceptually:

```text
Bid
├── ride
├── driver
├── vehicle
├── amount
├── status
├── submitted_at
└── updated_at
```

A bid belongs to exactly one ride and one driver.

The driver must have an eligible vehicle associated with the bid.

---

## 4. Driver Eligibility

A driver may participate in bidding only if all required eligibility
conditions are satisfied.

At minimum:

```text
Authenticated
    AND
Approved driver
    AND
Not suspended
    AND
Eligible vehicle
    AND
Driver is online
    AND
Driver is geographically eligible
    AND
Driver is not committed to an active trip
```

Additional eligibility rules may be introduced later.

Driver eligibility is checked when the bid is submitted and must be checked
again when the rider selects the bid.

A driver becoming ineligible after submitting a bid does not necessarily
delete the bid immediately, but the bid cannot be selected if the driver is
no longer eligible.

---

## 5. One Active Bid Per Driver Per Ride

A driver may have at most one active bid for a particular ride.

Example:

```text
Ride #123

Driver A
    Bid: PKR 1,100
```

The same driver cannot simultaneously have:

```text
PKR 1,100
PKR 1,150
PKR 1,200
```

as separate active bids for the same ride.

If the driver changes the bid, the existing active bid is updated rather than
creating another active bid.

---

## 6. Bid Creation

A driver may submit a bid while the ride's bidding window is open.

The backend must validate:

- Driver authentication
- Driver approval status
- Driver availability
- Vehicle eligibility
- Geographic eligibility
- Ride eligibility
- Bidding window
- Bid amount
- Existing active bid for the same ride

The client cannot directly establish a bid as valid.

The backend is authoritative.

---

## 7. Bid Amount

The bid amount must satisfy platform-defined limits.

Conceptually:

```text
minimum_bid <= bid_amount <= maximum_bid
```

The limits are derived from applicable pricing rules and the reference fare.

Example:

```text
Reference fare: PKR 1,200
Minimum bid:    PKR 1,000
Maximum bid:    PKR 1,500
```

Therefore:

```text
PKR 999   → rejected
PKR 1,000 → accepted
PKR 1,200 → accepted
PKR 1,500 → accepted
PKR 1,501 → rejected
```

The exact percentage or calculation used to establish the limits is not
defined by this document.

---

## 8. Bid Modification

A driver may modify their active bid while bidding remains open.

Example:

```text
Initial bid:
PKR 1,200

Updated bid:
PKR 1,150
```

There remains only one active bid for the driver and ride.

Bid modifications are subject to the same validation rules as bid creation.

The backend should rate-limit bid modifications to prevent excessive
requests.

The exact rate limit is an implementation/configuration decision.

---

## 9. Bid Withdrawal

A driver may withdraw their active bid while bidding remains open.

The bid becomes:

```text
WITHDRAWN
```

A withdrawn bid cannot be selected.

A driver may not withdraw a bid after the rider has selected it.

Once selected, the driver enters the confirmation process.

---

## 10. Bidding Window

Each ride has a defined bidding period.

Conceptually:

```text
bidding_started_at
bidding_ends_at
```

Bids may be created or modified only while:

```text
current_time < bidding_ends_at
```

The backend owns the authoritative bidding clock.

The Flutter application may display a countdown, but the countdown is not
the source of truth.

---

## 11. Late Bids

Once the bidding window has ended:

- New bids are rejected.
- Existing active bids can no longer be modified.
- Existing active bids can no longer be withdrawn.

The backend determines whether the bidding window is open.

A client request arriving after the deadline must be rejected even if the
client UI still displays bidding as active.

---

## 12. Driver Visibility

Drivers do not see competing bids.

A driver sees information necessary to decide whether to participate,
including:

- Ride pickup
- Ride destination or permitted destination information
- Reference fare
- Applicable bidding limits
- Their own current bid
- Bidding deadline

Drivers must not see:

- Other drivers' identities
- Other drivers' bid amounts
- The number of competing bids
- Other drivers' private information

The exact amount of destination information exposed to drivers may be
refined later based on product and privacy requirements.

---

## 13. Rider Visibility

The rider can see eligible active bids associated with their ride.

A bid presented to the rider may include:

- Driver display name
- Driver rating
- Driver completed-ride count
- Vehicle make/model
- Vehicle type
- Vehicle identifier/plate information
- Estimated arrival time
- Bid amount

The rider must not receive private driver information.

---

## 14. Driver Location Privacy

The rider should not receive the driver's exact live location while the
driver is only participating in bidding.

The rider may receive an estimated arrival time or approximate information
needed to evaluate the bid.

Once a driver has been selected and confirmed, live driver location may be
provided as part of the active ride experience.

---

## 15. Bid Selection

After bidding closes, the rider may select an eligible active bid.

The backend must revalidate the bid and driver before selection.

At minimum:

```text
Bid is still valid
    AND
Driver is still approved
    AND
Driver is still eligible
    AND
Vehicle is still eligible
    AND
Driver is not already committed
    AND
Ride is still selectable
```

The client cannot assume that a bid displayed moments earlier remains
selectable.

---

## 16. Selected Bid

When the rider selects a bid:

```text
Ride
├── driver = selected driver
├── vehicle = selected vehicle
└── agreed_fare = selected bid amount
```

The selected driver enters a confirmation phase.

Conceptually:

```text
DRIVER_SELECTED
```

The selected driver has not yet completed confirmation.

The assignment becomes confirmed only after the driver explicitly confirms.

---

## 17. Driver Confirmation

After selection, the driver receives an assignment confirmation request.

The request includes the agreed fare and the ride information necessary for
the driver to accept the assignment.

The driver must confirm within a configurable timeout.

Conceptually:

```text
driver_selected_at
driver_confirmation_deadline
```

The exact timeout is a configuration/product decision.

---

## 18. Driver Confirmation Success

If the driver confirms before the deadline:

```text
DRIVER_SELECTED
       ↓
DRIVER_CONFIRMED
```

The selected bid becomes the winning bid.

The agreed fare becomes fixed for the ride unless a later business rule
explicitly permits a change.

Other active bids are no longer selectable.

The driver becomes committed to the ride.

---

## 19. Driver Confirmation Failure

If the selected driver:

- Rejects the assignment
- Fails to confirm before the deadline
- Becomes ineligible
- Loses eligibility for another valid reason

then the platform may attempt to select another valid bid.

Example:

```text
Bid A → PKR 1,100  ← selected
Bid B → PKR 1,150
Bid C → PKR 1,200

Driver A fails confirmation

        ↓

Bid B becomes eligible for selection
```

The platform must revalidate the fallback bid and driver before assignment.

The rider should not be required to restart the entire bidding process if a
valid fallback bid exists.

---

## 20. Multiple Active Bidding Rides Per Driver

A driver may submit bids for multiple rides simultaneously while they are
only participating in the bidding stage.

Example:

```text
Driver A

Ride #101 → PKR 1,100
Ride #102 → PKR 900
Ride #103 → PKR 1,200
```

This is permitted.

However, a driver can become committed to only one active trip.

Once the driver is successfully selected and confirmed for a ride:

```text
Driver A
    ↓
COMMITTED
```

Other active bids from that driver must no longer be selectable.

---

## 21. Driver Reservation

Driver selection creates a concurrency-sensitive operation.

The platform must prevent two riders from successfully selecting the same
driver at the same time.

Example:

```text
Rider A selects Driver X
Rider B selects Driver X
```

Exactly one selection may succeed.

Driver reservation must therefore be handled atomically by the backend.

This is a business correctness requirement, not a client-side behavior.

The implementation must use appropriate transactional/concurrency controls.

---

## 22. Bid Lifecycle

The initial bid lifecycle is:

```text
                 ┌──────────────┐
                 │    ACTIVE    │
                 └──────┬───────┘
                        │
            ┌───────────┼────────────┐
            │           │            │
            ▼           ▼            ▼
       WITHDRAWN     SELECTED      EXPIRED
```

`ACTIVE` means the bid can potentially be selected.

`WITHDRAWN` means the driver voluntarily removed the bid.

`SELECTED` means the rider selected the bid and the driver entered the
confirmation process.

`EXPIRED` means the bidding window ended before the bid was selected.

Expiration may be derived from the ride's bidding deadline rather than
requiring an individual expiration operation for every bid.

---

## 23. Bid History

The current bid amount represents the driver's current offer.

If bid modifications need to be auditable, the platform should record bid
change events separately.

Example:

```text
Bid
    current amount = PKR 1,150

Bid events:
    PKR 1,200 submitted
    PKR 1,150 updated
```

The initial domain model should not require a full versioned bid object unless
there is a business requirement for it.

An event/audit mechanism can provide the history.

---

## 24. No Bids

If the bidding window closes and there are no selectable bids:

```text
BIDDING
   ↓
NO_DRIVER_FOUND
```

The ride becomes a terminal unsuccessful ride request.

The rider may be offered the ability to request another ride, but that is a
product/UI decision rather than part of the bidding lifecycle.

---

## 25. Driver Becomes Unavailable During Bidding

A driver may become unavailable after submitting a bid.

Examples:

- Driver goes offline
- Driver's vehicle becomes unavailable
- Driver is suspended
- Driver becomes committed to another ride
- Driver leaves the eligible geographic area

The bid does not necessarily need to be immediately deleted.

However, when the rider attempts to select it, the backend must perform a
fresh eligibility check.

If the driver is no longer eligible, the bid cannot be selected.

---

## 26. Driver Cancellation After Confirmation

A driver cancellation after confirmation is different from bid withdrawal.

Before selection:

```text
ACTIVE → WITHDRAWN
```

After selection and confirmation:

```text
DRIVER_CONFIRMED
       ↓
driver cancellation
```

The driver has already committed to the ride.

The platform may attempt reassignment using still-valid bids or another
matching mechanism.

The exact driver cancellation policy, penalties, and reassignment strategy
will be defined separately.

---

## 27. Rider Cancellation

The rider may cancel the ride while cancellation is permitted by the ride
lifecycle and applicable cancellation policy.

Cancelling a ride is separate from withdrawing a bid.

The cancellation policy may later determine:

- Whether cancellation is allowed
- Whether a fee applies
- Whether the driver receives compensation
- Whether the ride can be resumed

These rules are outside the bidding domain.

---

## 28. Notifications and Real-Time Updates

Important bidding events should be delivered to clients through the
real-time communication system.

Potential rider events:

```text
BIDDING_STARTED
BID_RECEIVED
BID_UPDATED
BID_WITHDRAWN
BIDDING_CLOSED
DRIVER_SELECTED
DRIVER_CONFIRMED
DRIVER_CONFIRMATION_FAILED
```

Potential driver events:

```text
RIDE_AVAILABLE_FOR_BIDDING
BID_ACCEPTED
BID_UPDATED
BID_WITHDRAWN
DRIVER_SELECTED
CONFIRMATION_DEADLINE
BID_NO_LONGER_AVAILABLE
```

The exact WebSocket event contract will be designed separately.

Real-time delivery is not the authoritative business state. The backend and
database remain authoritative.

---

## 29. Business Rules vs Implementation

This document defines business behavior.

It does not prescribe:

- PostgreSQL table structure
- Redis key structure
- REST endpoint names
- WebSocket implementation
- Lock implementation
- Background worker implementation
- Exact pricing algorithm
- Exact rate limits
- Infrastructure topology

Those decisions will be made in the appropriate technical design
documents.

---

## 30. Initial Rules Summary

| Rule | Decision |
|---|---|
| Active bids per driver/ride | One |
| Bid modification | Allowed while bidding is open |
| Bid withdrawal | Allowed while bidding is open |
| Competing bids visible to drivers | No |
| Competing bids visible to rider | Yes |
| Exact driver location before selection | No |
| Reference fare | Required |
| Bid minimum/maximum | Platform-defined |
| Bidding deadline | Required |
| Late bid | Rejected |
| Multiple rides per driver during bidding | Allowed |
| Driver committed to multiple trips | No |
| Driver selection | Rider chooses |
| Driver confirmation | Required |
| Confirmation timeout | Configurable |
| Failed confirmation | Try another valid bid |
| No valid bids | `NO_DRIVER_FOUND` |
| Bid negotiation/chat | Not part of initial bidding system |
| Backend authorization | Required for every operation |
| Driver reservation | Atomic |
| Client as source of truth | Never |
