# Cancellation and Policy

## 1. Purpose

This document defines cancellation rules across the ride lifecycle, including
who may cancel, when cancellation is allowed, financial consequences,
reservation/assignment release, and race conditions.

Cancellation is a business command, not an arbitrary lifecycle-state mutation.

---

# 2. Core Principles

1. Cancellation is evaluated against the current authoritative ride state.
2. Rider and driver cancellation are distinct policy cases.
3. Cancellation policy varies by lifecycle stage.
4. Cancellation does not mean payment, refund, or fare calculation; it may trigger those domains.
5. Historical reservations, bids, and assignments must not be deleted merely because a ride is cancelled.
6. Cancellation and confirmation/start/completion races must be resolved transactionally.
7. Cancellation commands must be idempotent.
8. A completed trip is not changed back to cancelled.
9. Post-start abnormal termination should eventually be distinguished from ordinary pre-trip cancellation.
10. Driver availability after cancellation must be re-evaluated rather than blindly set to AVAILABLE.
11. Cancellation policy must be configurable and auditable.
12. Notifications are consequences of cancellation, not the source of truth for it.

---

# 3. Cancellation vs Trip Termination

Ordinary cancellation is primarily a pre-trip concept.

Potential future post-start concepts include:

```text
trip interruption
emergency termination
operational incident
```

Do not make one `cancel` command responsible for every abnormal trip outcome.

For V1, the exact post-start policy must be explicit, but the architecture should
leave room for a separate trip-termination operation.

---

# 4. Cancellation Command

Conceptual API:

```http
POST /api/v1/rides/{ride_id}/cancel
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
```

The backend evaluates:

```text
actor identity
current ride state
actor authorization
cancellation policy
financial consequences
reservation state
assignment state
```

The client must not directly set the ride status.

---

# 5. Rider Cancellation

The rider may request cancellation subject to lifecycle policy.

Conceptually:

```text
Rider
  ↓
cancel command
  ↓
policy evaluation
  ↓
accepted/rejected
```

The policy may change depending on whether a driver has been selected, reserved,
confirmed, or arrived.

---

# 6. Driver Cancellation

The driver may cancel a reservation/assignment where policy permits.

Before trip start this may result in:

```text
reservation/assignment released
      ↓
ride returns to appropriate fallback/discovery state
```

Driver cancellation may also carry operational or marketplace consequences,
such as reliability metrics or future policy decisions.

After trip start, the operation should eventually be modeled as trip termination
rather than ordinary cancellation.

---

# 7. Cancellation by Lifecycle State

Initial conceptual policy:

| Ride State | Rider Cancellation | Driver Cancellation |
|---|---|---|
| BIDDING | Yes | N/A unless opportunity exists |
| RESERVED | Yes, policy-dependent | Yes, policy-dependent |
| DRIVER_CONFIRMED | Yes, policy-dependent | Yes, policy-dependent |
| DRIVER_EN_ROUTE | Yes, likely fee-dependent | Yes, policy-dependent |
| DRIVER_ARRIVED | Yes, likely stronger fee | Yes, policy-dependent |
| TRIP_STARTED | Special termination policy | Special termination policy |
| TRIP_COMPLETED | No | No |

The actual fee and permission rules are configuration/product policy.

---

# 8. Bidding Cancellation

If a ride is still bidding:

```text
BIDDING
   ↓
rider cancels
   ↓
close bidding
   ↓
invalidate outstanding opportunities
```

Drivers who receive stale notifications must not be able to submit valid bids
against a closed ride.

Existing bids remain historical records even after cancellation.

---

# 9. Selected Bid Cancellation

After a rider selects a bid:

```text
bid selected
   ↓
reservation
```

If the rider cancels:

```text
reservation
   ↓
cancelled/released
   ↓
assignment if any is released
```

The selected bid must remain historical data.

---

# 10. Reservation Release

Cancellation before assignment should release the active reservation.

The system must preserve:

```text
reservation id
selected driver
selected bid
created_at
cancelled_at
cancellation reason
```

Do not delete the reservation record.

---

# 11. Assignment Release

If cancellation occurs after assignment exists:

```text
active assignment
      ↓
released/terminated
```

The historical relationship should remain queryable.

Do not use hard deletion as the mechanism for cancellation.

---

# 12. Driver Availability After Cancellation

After reservation/assignment release:

```text
driver state re-evaluation
       ↓
AVAILABLE or OFFLINE or otherwise unavailable
```

Do not blindly set the driver to `AVAILABLE` because the driver may have:

```text
stale heartbeat
stale location
lost eligibility
lost vehicle eligibility
gone offline
```

---

# 13. Cancellation Reasons

Cancellation should use structured reasons rather than relying entirely on free
text.

Initial examples:

```text
RIDER_CHANGED_MIND
WRONG_PICKUP
WAIT_TOO_LONG
DRIVER_REQUESTED_CANCEL
DRIVER_UNABLE_TO_COMPLETE
PAYMENT_PROBLEM
OTHER
```

The exact taxonomy should remain small initially and evolve from real support
and operational data.

An optional note may accompany a structured reason where appropriate.

---

# 14. Cancellation Policy

The policy evaluates at least:

```text
cancelling party
current ride state
time since reservation
time since driver confirmation
driver arrival
trip started status
other explicit product conditions
```

The policy should return an explicit result such as:

```text
allowed
rejected
allowed_with_fee
```

The exact implementation can use a richer policy result object.

---

# 15. Cancellation Fees

Cancellation may result in:

```text
fee = 0
```

or:

```text
fee > 0
```

The fee must be determined by cancellation policy and authoritative ride data.

Do not allow clients to submit their own cancellation fee amount.

---

# 16. Cancellation Fee Boundary

The cancellation domain determines that a fee applies.

The pricing/payment domains determine how the monetary consequence is recorded
and collected.

Conceptually:

```text
Cancellation Policy
      ↓
fee decision
      ↓
Pricing/Financial record
      ↓
Payment/Refund workflow
```

The cancellation endpoint must not directly contain payment-provider logic.

---

# 17. Cancellation After Payment Authorization

If payment has been authorized but the ride is cancelled, the payment workflow
must determine whether to:

```text
void/release authorization
capture a cancellation fee
refund an already captured amount
```

The cancellation domain should publish the business consequence rather than
implement provider-specific operations.

---

# 18. Cancellation After Payment Capture

If money has already been captured:

```text
cancellation
   ↓
financial adjustment/refund policy
```

The original payment remains historical.

Do not mutate the original charge to pretend it never happened.

---

# 19. Cancellation and Refund

Possible flow:

```text
Cancellation
    ↓
fee = 0
    ↓
refund applicable amount
```

or:

```text
Cancellation
    ↓
fee = 300 PKR
    ↓
refund remaining amount
```

The exact calculation belongs to the financial domain and must be auditable.

---

# 20. Cancellation and Bids

Cancellation closes the ride's bidding opportunity.

Outstanding driver opportunities should become invalid for new bid creation.

Existing bids remain immutable historical records.

---

# 21. Cancellation and Discovery

If cancellation occurs while discovery is running:

```text
ride cancelled
      ↓
discovery must stop
```

A stale worker must not notify additional drivers for a closed ride.

Before producing a new opportunity, the discovery worker should verify that the
ride remains open for discovery.

---

# 22. Cancellation and Notification

After a successful cancellation, relevant parties should receive a durable
cancellation event/notification.

Conceptually:

```text
cancel transaction commits
      ↓
outbox event
      ↓
notification delivery
```

The notification is not the source of truth.

---

# 23. Cancellation Race: Rider vs Driver Confirmation

Example:

```text
Rider cancellation
        ↕
Driver confirmation
```

Only one valid state transition may win.

The winning transaction must establish the authoritative state and ensure the
losing operation receives a state/conflict result or idempotent outcome.

---

# 24. Cancellation Race: Rider vs Driver Start

The same principle applies to:

```text
cancel ↔ start
```

If cancellation commits first, the start must fail.

If start commits first, ordinary cancellation may no longer be valid and the
request must follow the post-start policy.

---

# 25. Cancellation Race: Cancellation vs Completion

A completion request racing with cancellation must be serialized by the
authoritative ride state transaction.

Only one terminal transition may win.

Do not allow:

```text
CANCELLED
and
TRIP_COMPLETED
```

as competing terminal states for the same ride.

---

# 26. Cancellation Race: Reservation

Cancellation can race with reservation creation.

The reservation transaction must revalidate that the ride is still open and
eligible for reservation.

A cancellation committed first must prevent a later reservation from becoming
active.

---

# 27. Atomic State Change

Cancellation should update all required durable business state within an
appropriate PostgreSQL transaction.

For example:

```text
ride state
reservation state
assignment state where applicable
cancellation record
outbox event
```

must be made consistent before the transaction commits.

---

# 28. Idempotency

Mobile clients may retry cancellation after a lost response.

Therefore:

```text
cancel request
cancel request again
```

must not produce duplicate cancellation records or duplicate financial effects.

The response should resolve to the authoritative cancellation outcome where
possible.

---

# 29. Already Cancelled Ride

If the same actor retries after cancellation has already committed, the backend
should return an idempotent success/authoritative result where the request
semantically matches the existing cancellation.

A conflicting second cancellation should not create a second state transition.

---

# 30. Already Completed Ride

A cancellation request after:

```text
TRIP_COMPLETED
```

must be rejected as an invalid lifecycle operation.

If money needs to be returned later, use refund/dispute/adjustment mechanisms
rather than rewriting trip history.

---

# 31. Driver Reliability Data

Driver cancellations may eventually contribute to operational metrics such as:

```text
cancellation rate
cancellation timing
completed-trip rate
```

These metrics should be derived from durable cancellation records rather than
maintaining a single mutable counter as the source of truth.

---

# 32. Rider Cancellation Data

Similarly, rider cancellations should remain queryable for:

```text
support
fee disputes
analytics
fraud/abuse analysis
product decisions
```

Do not throw away the reason and actor information after changing ride state.

---

# 33. Historical Cancellation Record

A cancellation record should capture enough information to answer:

```text
who cancelled?
when?
which ride?
which state?
why?
was a fee applied?
what policy/rule version applied?
```

The exact schema is a later database design task.

---

# 34. Policy Versioning

Cancellation policies can change.

Important cancellation decisions should retain a policy/rule version where
practical.

Conceptually:

```text
cancellation_policy_version = 4
```

This allows historical explanation of why a fee or permission decision occurred.

---

# 35. PostgreSQL Responsibilities

PostgreSQL should persist durable cancellation information such as:

```text
ride cancellation state
cancellation record
reason
actor
timestamps
policy version
fee decision/reference
reservation/assignment terminal state
```

The exact schema and constraints are a later database task.

---

# 36. Redis Responsibilities

Redis may support short-lived operational acceleration such as:

```text
cancellation notification coordination
short-lived locks where appropriate
fast discovery invalidation
```

Redis must not be authoritative for whether a ride is cancelled.

---

# 37. Outbox Events

A committed cancellation should produce a durable outbox event.

Conceptual event:

```text
ride.cancelled
```

Potential payload fields:

```text
ride_id
actor_type
actor_id
reason
cancelled_at
policy_version
```

Consumers can then update notifications, analytics, discovery, and other
non-authoritative workflows.

---

# 38. Failure Handling

If PostgreSQL cannot commit cancellation:

```text
cancellation did not succeed
```

Do not send a success notification before the durable transaction commits.

If notification delivery fails after commit:

```text
ride remains cancelled
notification retries
```

---

# 39. Discovery Failure After Cancellation

If a discovery worker does not immediately observe cancellation:

```text
worker reads stale state
      ↓
rechecks authoritative ride state
      ↓
stops before creating a valid opportunity
```

The system should rely on authoritative state checks rather than requiring
perfect cache invalidation.

---

# 40. Authorization

The backend derives actor identity from authentication.

Do not trust client-supplied:

```text
rider_id
driver_id
actor_id
```

The backend verifies that the authenticated actor is allowed to cancel the
specific ride/reservation/assignment.

---

# 41. Driver Authorization

A driver may cancel only when they are the relevant assigned/reserved driver.

A random authenticated driver must never be able to cancel another driver's
ride by supplying its ID.

---

# 42. Rider Authorization

A rider may cancel only rides belonging to that rider, subject to lifecycle and
policy restrictions.

---

# 43. Admin/Operations Cancellation

Authorized operational users may eventually have cancellation capabilities for
support or safety reasons.

These actions should be explicitly authorized and auditable rather than treated
as ordinary rider/driver cancellation.

---

# 44. Notifications

Potential cancellation events include:

```text
ride.cancelled
reservation.released
assignment.released
```

Not every internal event needs to become a separate rider-facing push
notification.

The notification policy belongs to the real-time communication design.

---

# 45. Observability

Useful metrics include:

```text
ride_cancellation_total
rider_cancellation_total
driver_cancellation_total
cancellation_fee_total
cancellation_conflict_total
cancellation_failure_total
```

Useful trace/log fields include:

```text
request_id
ride_id
reservation_id
assignment_id
actor_id
cancellation_reason
policy_version
```

Avoid high-cardinality actor identifiers as metric labels.

---

# 46. What We Should Not Build Yet

Do not build:

```text
complex cancellation scoring
automatic driver penalties
automatic rider fraud decisions
post-trip cancellation rewriting
complex dispute system
full support tooling
ML-based cancellation policy
provider-specific refund logic inside cancellation
```

Those require separate product, financial, and operational requirements.

---

# 47. Complete Cancellation Flow

```text
                 CANCEL COMMAND
                       │
                       ▼
                AUTHORIZATION
                       │
                       ▼
               POLICY EVALUATION
                       │
              ┌────────┴────────┐
              ▼                 ▼
           REJECTED           ALLOWED
                                │
                                ▼
                         ATOMIC TRANSACTION
                                │
                 ┌──────────────┼──────────────┐
                 ▼              ▼              ▼
             Ride state    Reservation/    Financial
                           Assignment       consequence
                              release
                 └──────────────┼──────────────┘
                                ▼
                          OUTBOX EVENT
                                │
                    ┌───────────┴───────────┐
                    ▼                       ▼
               Discovery              Notifications
                 stops
```

---

# 48. Design Principles

1. Cancellation is an explicit command governed by policy.
2. Rider and driver cancellation are distinct actor cases.
3. Cancellation permission depends on the current lifecycle state.
4. Cancellation does not mean payment, refund, or fare calculation.
5. Cancellation may trigger a financial consequence handled by financial domains.
6. Bidding closes when the ride is cancelled.
7. Outstanding driver opportunities become invalid after cancellation.
8. Existing bids remain historical records.
9. Reservations and assignments are released/terminated without deleting history.
10. Driver availability is re-evaluated after release.
11. Cancellation and confirmation/start/completion/reservation races must be resolved transactionally.
12. Cancellation commands must be idempotent.
13. Completed trips cannot be rewritten as cancelled.
14. Post-start abnormal termination should eventually be a separate concept from ordinary cancellation.
15. Cancellation reasons should be structured and auditable.
16. Cancellation policy versions should be retained where needed for historical explanation.
17. PostgreSQL is authoritative for cancellation state.
18. Redis may accelerate operational workflows but never owns cancellation truth.
19. Cancellation notifications are downstream effects of committed state.
20. Keep cancellation policy configurable and simpler than the financial/provider systems it may trigger.
