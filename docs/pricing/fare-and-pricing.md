# Fare and Pricing

## 1. Purpose

This document defines the pricing boundaries between ride pricing, the
reference fare, driver bids, the agreed trip price, final fare, and payment.

The core distinction is:

```text
Reference fare = platform estimate
Bid            = driver's offer
Agreed price   = selected bid's commercial terms
Final fare     = amount ultimately settled
```

These concepts must not be collapsed into one mutable price field.

---

# 2. Core Principles

1. Pricing calculates monetary values; it does not select drivers.
2. A reference fare is an estimate, not a driver bid.
3. A driver bid is an explicit offer from the driver.
4. Rider selection converts a bid into agreed commercial terms.
5. Selected commercial terms become immutable when the reservation is created.
6. Final fare is distinct from the agreed price because legitimate adjustments may exist.
7. V1 should keep final fare equal to the agreed price unless an explicitly defined adjustment applies.
8. Payment is a separate domain from pricing.
9. Money calculations must not use binary floating-point arithmetic.
10. Pricing rules should be versioned for historical reproducibility.
11. Google Maps may provide normalized distance/ETA inputs but should not own pricing logic.
12. Pricing must remain independently testable from payment and ride lifecycle code.

---

# 3. Pricing vs Bidding vs Payment

The boundaries are:

```text
Pricing
  ↓
What is the platform's reference estimate?

Bidding
  ↓
What price does the driver offer?

Selection/Reservation
  ↓
What commercial terms did the rider accept?

Payment
  ↓
How is the money collected/settled?
```

Do not make payment logic part of the pricing engine.

---

# 4. Money Representation

Business money must use an exact representation.

Do not use:

```text
float32
float64
```

for monetary calculations.

Use an integer minor-unit representation or an equivalent exact money value
object with currency-aware precision.

Conceptually:

```text
Money
├── amount
└── currency
```

The exact implementation will be standardized in the Go codebase.

---

# 5. Currency

Every monetary value must carry its currency explicitly.

Example:

```text
amount = 1200
currency = PKR
```

A bid must use the ride's currency unless a future multi-currency pricing model
explicitly permits otherwise.

---

# 6. Currency Precision

The system must respect the currency's actual minor-unit rules.

Do not blindly assume every currency has two decimal places.

The money representation should reject invalid precision for the selected
currency.

The final currency/precision library choice is an implementation decision.

---

# 7. Reference Fare

The platform may calculate a reference fare before driver bidding.

Conceptually:

```text
Reference fare
  = base component
  + distance component
  + time component
  + applicable configured adjustments
```

The exact formula is a product/pricing policy and should remain configurable.

---

# 8. Reference Fare Is an Estimate

The reference fare should communicate an expected platform price, not a
commitment that must equal the final trip price in every future version.

For example:

```text
Reference fare = 1350 PKR
Driver bid     = 1200 PKR
```

The rider may evaluate the driver's offer relative to the reference fare.

---

# 9. Pricing Inputs

Potential reference-fare inputs include:

```text
pickup location
dropoff location
service type
estimated distance
estimated duration
base fare configuration
per-distance rate
per-time rate
configured adjustments
```

Only inputs justified by the actual product should be introduced.

---

# 10. Pricing Version

Pricing rules change over time.

A reference fare should therefore carry a pricing/rule version.

Conceptually:

```text
pricing_version = 3
```

This allows the system to answer later:

```text
Which pricing rules produced this reference fare?
```

The exact versioning format is an implementation detail.

---

# 11. Driver Bid

A driver explicitly submits an offer.

Example:

```json
{
  "amount": "1200",
  "currency": "PKR"
}
```

The backend validates the amount before accepting the bid.

The bid is not the same object as the platform reference fare.

---

# 12. Bid Amount Validation

At minimum, validate:

```text
amount > 0
currency = ride currency
precision valid
amount within configured limits
ride still accepts bids
```

The exact minimum/maximum rules are configuration/product policy.

---

# 13. Bid vs Reference Fare

The initial model should not require the bid to equal the reference fare.

Do not automatically enforce:

```text
bid >= reference fare
```

or:

```text
bid <= reference fare
```

unless the product explicitly decides to impose such constraints.

The purpose of the bidding model is to allow independent driver offers.

---

# 14. Invalid Bid Values

Reject clearly invalid values such as:

```text
zero
negative amount
unsupported currency
invalid precision
unreasonably large configured-over-limit value
```

The backend, not Flutter, is authoritative for validation.

---

# 15. Agreed Price

When the rider selects a bid:

```text
Driver bid
    ↓
Rider selection
    ↓
Reservation
```

The selected bid becomes the agreed commercial offer.

Example:

```text
Reference fare: 1350 PKR
Driver bid:     1200 PKR
Rider selects:  1200 PKR
```

The agreed price is 1200 PKR.

---

# 16. Agreed Price Immutability

Once a reservation is created, the selected commercial terms are immutable.

At minimum:

```text
amount
currency
driver
vehicle
ride
```

must not silently change through ordinary bid or reservation updates.

---

# 17. Pricing Snapshot

The reservation should preserve the commercial information needed to reproduce
what the rider selected.

Conceptually:

```text
bid amount
currency
reference fare where applicable
pricing version/reference
selected timestamp
```

The exact snapshot schema is a database design task.

---

# 18. Why Snapshot Pricing

Pricing configuration may change after selection.

Example:

```text
10:00 → reference fare = 1350
10:01 → driver bids = 1200
10:02 → rider selects
10:03 → pricing configuration changes
```

The reservation must still preserve the 1200 PKR agreed price.

Historical records must not be recalculated from current pricing configuration.

---

# 19. Final Fare

Final fare is the amount ultimately settled for the trip.

V1 recommendation:

```text
Final fare = agreed bid
```

unless a specifically defined adjustment applies.

Do not introduce arbitrary dynamic repricing after selection.

---

# 20. Fare Adjustments

Future legitimate adjustments may include:

```text
tolls
approved waiting charges
authorized route changes
additional stops
cancellation fees where applicable
other explicit product rules
```

Each adjustment should have its own defined rule and audit trail.

Do not allow generic clients to submit arbitrary fare adjustments.

---

# 21. No Silent Repricing

After selection, the system must not silently change:

```text
1200 PKR
```

to:

```text
1450 PKR
```

because current pricing configuration changed.

Any increase must be caused by an explicit business rule that the product
supports and that can be audited.

---

# 22. Pricing Recalculation

The system may calculate a new reference fare while a ride is still in an
appropriate pre-selection state if the product later requires it.

But once a bid is selected and reserved, the agreed price is locked.

The reservation must not call the pricing engine to discover a new price.

---

# 23. Google Maps Boundary

Google Maps may provide normalized inputs such as:

```text
estimated distance
estimated duration
route information
geocoding
```

The pricing engine consumes these inputs.

It should not contain Google Maps API calls throughout its calculation logic.

Conceptually:

```text
Google Maps
    ↓
normalized route input
    ↓
Pricing Engine
    ↓
Reference Fare
```

---

# 24. Maps Failure

A temporary Google Maps failure must not corrupt durable pricing or trip state.

Depending on the operation, the system may:

```text
retry
use an available cached/previous estimate where policy permits
fail the reference-fare calculation safely
```

The system must not invent inaccurate monetary values simply to return success.

---

# 25. Pricing and Reservation

Reservation consumes the selected commercial terms.

It does not independently calculate a new price.

The boundary is:

```text
Pricing
  ↓
Reference fare

Bidding
  ↓
Driver offer

Selection
  ↓
Agreed price

Reservation
  ↓
Immutable commercial snapshot
```

---

# 26. Pricing and Payment

Keep these domains separate:

```text
Pricing
  ↓
amount owed

Payment
  ↓
collection/authorization/capture/refund
```

Payment providers, payment methods, payment authorization, capture, refunds,
and driver payouts should not be implemented inside the pricing engine.

---

# 27. Cancellation Fees

Cancellation fees should be governed by cancellation policy.

Conceptually:

```text
Ride cancellation
      ↓
Cancellation policy
      ↓
fee calculation
```

Do not bury cancellation behavior inside the base ride-fare formula.

---

# 28. Rounding

Rounding must happen according to currency rules and at defined calculation
boundaries.

Do not repeatedly round intermediate values in ways that create cumulative
errors.

The pricing engine should define where rounding occurs and preserve sufficient
precision internally when necessary.

---

# 29. Determinism

Given the same:

```text
pricing version
pricing inputs
currency
```

the reference-fare calculation should produce the same result.

This makes pricing testable and auditable.

---

# 30. Pricing Configuration

Pricing configuration may include:

```text
base fare
per-distance rate
per-time rate
service-specific rules
minimum fare
maximum fare
currency
version
```

Configuration changes should be versioned or otherwise auditable.

Do not make pricing constants scattered throughout Go source code.

---

# 31. Pricing Configuration Changes

A new pricing configuration should receive a new version.

Conceptually:

```text
Version 3
   ↓
Version 4
```

New rides use the applicable current version according to policy.

Historical rides retain their recorded pricing context.

---

# 32. API Boundary

Potential pricing API responsibilities include:

```text
calculate reference fare
return pricing metadata/version
validate bid amount
```

Pricing should not expose internal configuration mutation directly to normal
mobile clients.

---

# 33. Reference Fare API

A conceptual internal operation might be:

```text
calculateReferenceFare(rideInput)
```

The exact public/private API boundary will be decided during API design.

The Flutter client should receive the resulting reference fare rather than
implementing pricing calculations itself.

---

# 34. Bid Validation API Boundary

Bid validation belongs to the backend bidding workflow.

Conceptually:

```text
POST /rides/{ride_id}/bids
       ↓
Bidding domain
       ↓
pricing validation/value object
       ↓
Bid accepted/rejected
```

Do not make Flutter responsible for enforcing business pricing constraints.

---

# 35. Idempotency

Pricing calculation itself should ideally be deterministic and side-effect
free.

Commands that persist pricing-related business state must use the relevant
idempotency rules of their owning domains.

For example:

```text
bid creation → bidding idempotency
reservation → reservation/selection idempotency
payment → payment idempotency
```

Do not create one giant cross-domain idempotency mechanism.

---

# 36. Concurrency

Pricing calculations should be safe to execute concurrently.

A reference-fare calculation should not mutate shared global pricing state.

Pricing configuration changes must use an explicit versioned configuration
model.

---

# 37. PostgreSQL Responsibilities

PostgreSQL may persist:

```text
pricing configuration/version
reference fare snapshot where required
bid amount
agreed price
final fare
fare adjustments
currency
```

The exact tables and constraints are a later database-design task.

---

# 38. Redis Responsibilities

Redis may cache:

```text
current pricing configuration
short-lived fare calculation inputs/results
operational lookup data
```

Cached pricing data must be invalidated or versioned safely when configuration
changes.

Redis is not the durable historical pricing authority.

---

# 39. Failure Handling

If pricing calculation cannot safely produce a reference fare:

```text
no fabricated price
no false success
```

If a cached value is used, it must comply with explicit freshness/version policy.

If PostgreSQL cannot persist a required pricing state, the owning transaction
must fail rather than claiming the operation succeeded.

---

# 40. Security

Pricing configuration is privileged operational data.

Normal rider/driver clients should not be able to modify:

```text
rates
pricing formulas
pricing versions
currency rules
fare limits
```

Administrative pricing changes require explicit authorization and auditability.

---

# 41. Auditability

Important pricing records should allow reconstruction of:

```text
pricing version
reference fare inputs/result
driver bid
selected/agreed price
final fare
adjustments
```

This is important for support, disputes, accounting, and debugging.

---

# 42. Observability

Useful metrics include:

```text
reference_fare_calculation_total
reference_fare_calculation_failure_total
bid_validation_failure_total
fare_adjustment_total
pricing_calculation_latency
```

Useful tracing/logging fields include:

```text
request_id
ride_id
pricing_version
currency
```

Avoid high-cardinality monetary values or raw customer identifiers as metric
labels.

---

# 43. Testing Requirements

Pricing should have deterministic unit tests for:

```text
base fare
zero/negative inputs
currency precision
minimum fare
maximum fare
distance component
time component
rounding
pricing version changes
boundary values
```

Property-based tests may become useful for money arithmetic later.

---

# 44. Pricing and Trip Completion

Trip completion should produce the facts required to determine final fare.

Conceptually:

```text
TRIP_COMPLETED
      ↓
final fare calculation/confirmation
      ↓
Payment domain
```

The exact sequencing is a later payment/fare-settlement design decision.

---

# 45. No Arbitrary Client Fare

A mobile client must never be able to submit:

```json
{
  "final_fare": 999
}
```

and have the backend accept it as authoritative.

Final fare is calculated or confirmed from authoritative domain data and rules.

---

# 46. No Pricing Logic in Flutter

Flutter may display:

```text
reference fare
bid amount
agreed price
final fare
```

but it should not own authoritative fare formulas.

The same pricing logic must not be independently reimplemented in Dart.

---

# 47. No Pricing Logic in Reservation

Reservation should preserve selected commercial terms.

It should not calculate:

```text
base fare
per-km price
surge
waiting fee
```

unless a future explicitly defined reservation-time rule requires it.

---

# 48. No Pricing Logic in Payment

Payment should consume an authoritative amount.

It should not independently recalculate the fare and potentially disagree with
the pricing domain.

---

# 49. Historical Integrity

Once a ride has completed, its historical monetary records must remain
reproducible even if current pricing configuration changes.

Historical records should therefore retain sufficient snapshots/version
references.

---

# 50. Complete Pricing Flow

```text
                     RIDE
                       │
                       ▼
                 PRICING ENGINE
                       │
                       ▼
                 REFERENCE FARE
                       │
                       ▼
                  DISCOVERY
                       │
                       ▼
                    BIDDING
                       │
                       ▼
                  DRIVER BID
                       │
                       ▼
                 RIDER SELECTS
                       │
                       ▼
                  RESERVATION
                       │
                       ▼
              AGREED PRICE LOCKED
                       │
                       ▼
                 TRIP COMPLETED
                       │
                       ▼
                  FINAL FARE
                       │
                 ┌─────┴─────┐
                 ▼           ▼
              PAYMENT     HISTORY
```

---

# 51. What We Should Not Build Yet

Do not build:

```text
complex dynamic surge pricing
machine-learning fare prediction
multi-currency settlement
arbitrary post-selection repricing
payment provider integration inside pricing
complex loyalty/discount engine
subscription pricing
promo-code engine
auction pricing logic
```

Those are separate product decisions and should be introduced only when
requirements justify them.

---

# 52. Design Principles

1. Reference fare, bid, agreed price, and final fare are distinct concepts.
2. Money uses exact currency-aware arithmetic, never binary floating point.
3. Every monetary value carries explicit currency.
4. Currency precision must follow the actual currency rules.
5. Reference fare is an estimate, not a guaranteed driver offer.
6. Driver bids are explicit offers and may differ from the reference fare.
7. Bid validation belongs to the backend.
8. Rider selection converts a bid into agreed commercial terms.
9. Agreed commercial terms are immutable once reservation is created.
10. Final fare is separate from agreed price and starts equal to it in V1 unless an explicit adjustment applies.
11. Pricing rules are versioned for historical reproducibility.
12. Google Maps provides normalized route inputs; pricing owns the calculation.
13. Pricing should not synchronously depend on external map services for every operation.
14. Payment is a separate domain that consumes authoritative amounts.
15. Cancellation fees belong to cancellation policy rather than base fare logic.
16. Pricing calculations should be deterministic and independently testable.
17. Pricing configuration changes must be auditable.
18. Historical rides must retain sufficient pricing context.
19. Flutter displays pricing but does not own pricing formulas.
20. Reservation preserves pricing but does not independently recalculate it.
21. Payment does not independently recalculate the fare.
22. Avoid sophisticated pricing systems until actual product requirements justify them.
