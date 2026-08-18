# Payment and Settlement

## 1. Purpose

This document defines the boundaries between pricing, customer payment,
platform fees, driver earnings, settlement, payouts, refunds, and external
payment providers.

The core distinction is:

```text
Pricing
  ↓
what is owed

Payment
  ↓
collect what is owed

Settlement
  ↓
determine who gets what

Payout
  ↓
transfer money to the recipient
```

---

# 2. Core Principles

1. Payment is separate from pricing.
2. Payment is separate from driver settlement.
3. Payment consumes an authoritative amount and does not recalculate fares.
4. Payment operations require idempotency.
5. Provider webhooks are asynchronous and may be duplicated.
6. PostgreSQL is authoritative for durable financial state.
7. Redis is never the financial source of truth.
8. Customer payment success does not imply driver payout completion.
9. Trip completion does not imply payment success.
10. Payment failure does not mean the trip failed.
11. Refunds are separate financial operations and must preserve original history.
12. Platform commission and driver earnings are explicit financial records.
13. Financial records must retain sufficient information for reconciliation and audit.
14. Provider-specific details must stay behind a payment-provider adapter boundary.
15. Sensitive payment credentials must not be stored unnecessarily by the platform.

---

# 3. Payment Lifecycle

Initial conceptual lifecycle:

```text
PAYMENT_PENDING
      ↓
PAYMENT_AUTHORIZED
      ↓
PAYMENT_CAPTURED
      ↓
SETTLEMENT_PENDING
      ↓
SETTLED
```

Failure paths include:

```text
PAYMENT_PENDING
      ↓
PAYMENT_FAILED
```

and:

```text
PAYMENT_CAPTURED
      ↓
REFUND_PENDING
      ↓
REFUNDED
```

The exact states will be refined after payment-provider selection.

---

# 4. Payment Does Not Start During Bidding

The initial flow is:

```text
Ride
 ↓
Reference fare
 ↓
Bidding
 ↓
Driver bid
 ↓
Rider selects
 ↓
Reservation
 ↓
Trip
 ↓
Final fare
 ↓
Payment
```

The system should not authorize or capture payment for every driver bid.

---

# 5. Payment Authorization Timing

For payment methods supporting authorization, the system may authorize around
reservation or before trip start.

Conceptually:

```text
Reservation
    ↓
Payment authorization
    ↓
Driver proceeds
```

The exact timing is a payment-method/provider policy and should not be hardcoded
into the ride lifecycle.

---

# 6. Payment Capture

A clean initial model is:

```text
TRIP_COMPLETED
      ↓
Final Fare
      ↓
Capture payment
```

Example:

```text
Agreed price = 1200 PKR
Final fare   = 1200 PKR
Payment      = 1200 PKR
```

If an explicitly supported fare adjustment changes the final fare, payment uses
the authoritative final amount.

---

# 7. Payment Amount Authority

The payment service consumes:

```text
ride_id
payment_id
amount
currency
```

It must not independently calculate:

```text
distance
ETA
base fare
per-km rate
commission
```

Correct boundary:

```text
Pricing/Fare
      ↓
authoritative final amount
      ↓
Payment
      ↓
provider
```

---

# 8. Payment Provider Boundary

The architecture should isolate provider-specific behavior:

```text
Our Payment Domain
        ↓
Provider Adapter
        ↓
External Payment Provider
```

Conceptually:

```text
PaymentService
      ↓
PaymentProvider interface
      ↓
Provider implementation
```

Provider-specific request/response types must not leak throughout the domain.

---

# 9. Provider Selection

Do not choose a payment provider as part of this domain document.

Provider selection depends on:

```text
launch geography
supported payment methods
local availability
provider fees
payout capabilities
compliance requirements
cash support
```

The internal payment contract should be defined first.

---

# 10. Payment Methods

The domain should support explicit payment method types.

Initial conceptual examples:

```text
CASH
CARD
DIGITAL_WALLET
```

Not every method must be implemented in V1.

The payment method type must remain distinct from the provider implementation.

---

# 11. Cash

Cash has a different financial lifecycle:

```text
Trip completed
      ↓
Driver collects cash
      ↓
Platform records amount owed/collected
      ↓
Settlement
```

There is no card-style authorization/capture flow.

This is why the payment domain should model financial obligations and
transactions rather than assuming every payment is an external card charge.

---

# 12. Payment Identity

A payment has its own durable identity.

Conceptually:

```text
Payment
├── payment_id
├── ride_id
├── customer_id
├── amount
├── currency
├── status
├── payment_method
├── provider
├── provider_reference
└── timestamps
```

The provider's transaction ID must not become the platform's primary business
identifier.

---

# 13. Payment Operations

Effectful operations should have their own durable identity or equivalent
idempotency key.

Potential operations include:

```text
authorization
capture
refund
```

The exact persistence model is a database design task.

---

# 14. Payment Idempotency

Payment operations must be safe against retries.

Example:

```text
backend requests capture
      ↓
provider captures
      ↓
network timeout
      ↓
backend retries
```

The retry must resolve to the existing capture operation rather than create a
second charge.

---

# 15. Provider Idempotency

Where the provider supports idempotency keys, the platform should use a stable
operation-specific key.

The same logical operation must not produce multiple external charges.

---

# 16. Payment Webhooks

External providers may asynchronously report state changes.

Conceptual events:

```text
payment.authorized
payment.captured
payment.failed
payment.refunded
```

Webhook processing should:

```text
receive
  ↓
verify authenticity
  ↓
deduplicate
  ↓
persist/update payment state
  ↓
publish internal event
```

---

# 17. Webhook Authentication

Webhook requests must be verified using the provider's documented signature or
authentication mechanism.

Reject:

```text
invalid signature
unknown event
malformed payload
```

Never trust a webhook merely because it reached the endpoint.

---

# 18. Webhook Idempotency

Provider events may be delivered more than once.

Track a unique provider event identifier where available.

Example:

```text
event_123
event_123
```

must produce one logical state transition.

---

# 19. Payment Failure

Payment failure is independent from trip lifecycle.

Example:

```text
TRIP_COMPLETED
      ↓
Payment capture fails
      ↓
PAYMENT_FAILED
      ↓
retry/recovery policy
```

The ride remains completed because the trip actually happened.

---

# 20. Payment Recovery

Payment recovery may include:

```text
provider retry
alternative payment method
customer action
outstanding balance
manual review
```

The exact recovery policy is a later product/payment decision.

---

# 21. Refund Lifecycle

Refunds are separate operations:

```text
REFUND_PENDING
      ↓
REFUNDED
```

A refund must reference the original payment.

---

# 22. Partial Refund

Partial refunds must be representable.

Example:

```text
Original charge = 1200 PKR
Refund         = 300 PKR
Net charge     = 900 PKR
```

Do not mutate the original payment amount from 1200 to 900.

Record the 300 PKR refund explicitly.

---

# 23. Refund Idempotency

A refund operation must be idempotent.

Retries must not create multiple refunds for the same logical operation.

Provider refund references should be stored where applicable.

---

# 24. Driver Settlement

Customer payment and driver settlement are separate workflows:

```text
Customer
   ↓
Payment
   ↓
Platform
   ↓
Settlement
   ↓
Driver
```

A successful customer payment does not necessarily mean the driver has already
received the money.

---

# 25. Platform Commission

Settlement calculates the distribution of the final fare.

Example:

```text
Final fare          = 1200 PKR
Platform commission = 200 PKR
Driver earnings     = 1000 PKR
```

The amounts should be represented explicitly rather than hidden in scattered
percentage calculations.

---

# 26. Commission Rules

Commission is a financial rule and may change over time.

Settlement records should retain enough information to identify the applicable
commission rule/version.

Conceptually:

```text
commission_rule_version
platform_fee
 driver_earning
```

The exact rule model is a later settlement-design task.

---

# 27. Driver Earnings

A completed trip creates a durable earning record.

Conceptually:

```text
Final Fare
    ↓
Settlement Calculation
    ├── Platform Fee
    └── Driver Earnings
```

Driver earnings must not simply be derived on the fly from the current
commission configuration because historical rules may differ.

---

# 28. Settlement Lifecycle

Initial conceptual lifecycle:

```text
SETTLEMENT_PENDING
       ↓
SETTLED
```

Failure:

```text
SETTLEMENT_PENDING
       ↓
SETTLEMENT_FAILED
       ↓
RETRY
```

Settlement status is independent from customer payment status.

---

# 29. Customer Payment vs Driver Payout

These are separate:

```text
PAYMENT_CAPTURED
      ≠
DRIVER_PAID
```

There may be:

```text
pending driver balance
settlement period
payout schedule
provider transfer
failed payout
```

Do not promise instant payout merely because customer payment succeeded.

---

# 30. Payout

Payout represents transfer of an accumulated or settled driver balance to a
driver's external payout destination.

Conceptually:

```text
Driver earnings
      ↓
settled balance
      ↓
payout request
      ↓
external transfer
      ↓
PAYOUT_COMPLETED
```

The payout provider may be different from the customer payment provider.

---

# 31. Ledger Mindset

The platform should retain explicit financial records such as:

```text
customer charge
platform fee
driver earning
refund
adjustment
payout
```

A sophisticated double-entry ledger may be introduced later, but the initial
schema should not make financial history dependent on mutable ride fields.

---

# 32. Financial Transaction Identity

Every important financial operation should have a durable platform identifier.

Provider references are additional external identifiers.

Conceptually:

```text
platform_transaction_id
provider_transaction_id
ride_id
payment_id
```

This allows provider replacement and reconciliation without changing the
platform's domain identity.

---

# 33. Payment and Trip Completion

These states are intentionally independent:

```text
TRIP_COMPLETED
      │
      └── payment workflow
```

A payment failure does not roll the ride back from `TRIP_COMPLETED`.

---

# 34. Payment and Pricing

Payment consumes the authoritative final fare.

It must not independently recalculate:

```text
distance
pricing formula
commission
fare adjustments
```

The payment amount should come from the pricing/fare domain.

---

# 35. Payment and Reservation

Reservation locks the agreed commercial terms.

Payment may use those terms when authorization is required, but payment does not
change reservation pricing.

---

# 36. Outbox Events

Durable payment and settlement transitions should publish events through the
outbox pattern.

Examples:

```text
payment.authorized
payment.captured
payment.failed
payment.refunded
settlement.created
settlement.completed
settlement.failed
payout.completed
```

The exact event contracts will be defined later.

---

# 37. External Provider Boundary

The provider adapter should translate between internal domain operations and
provider-specific APIs.

Conceptually:

```text
Internal Payment Command
        ↓
Provider Adapter
        ↓
Provider API
        ↓
Provider Adapter
        ↓
Internal Payment Result
```

Provider-specific status values must not leak directly into the rest of the
application.

---

# 38. PostgreSQL Responsibilities

PostgreSQL is authoritative for durable financial state including:

```text
payment
payment operations
provider references
refunds
financial transactions
settlement records
driver earnings
payout records
commission rule references
```

The exact tables, indexes, and constraints are later database work.

---

# 39. Redis Responsibilities

Redis may support:

```text
short-lived idempotency acceleration
rate limiting
short-lived locks/retry coordination
```

Redis must never be the authoritative financial record.

If Redis is lost, financial history must remain intact in PostgreSQL.

---

# 40. External Transaction Consistency

A PostgreSQL transaction cannot atomically include an external payment provider.

Therefore payment operations require an explicit retry/reconciliation model.

Conceptually:

```text
PostgreSQL
    ↓
record payment operation
    ↓
provider request
    ↓
provider result/webhook
    ↓
PostgreSQL state update
```

Do not assume a distributed transaction exists where it does not.

---

# 41. Unknown Provider Outcome

A dangerous case is:

```text
provider receives capture
      ↓
provider succeeds
      ↓
network fails
      ↓
platform does not know result
```

The system must not immediately create a second capture.

Instead:

```text
operation = UNKNOWN/PENDING
      ↓
query provider or await webhook
      ↓
resolve authoritative outcome
```

The exact state names depend on provider capabilities.

---

# 42. Reconciliation

A reconciliation process should eventually compare:

```text
Platform payment records
        ↕
Provider records
```

and identify:

```text
missing payment
unexpected payment
amount mismatch
currency mismatch
status mismatch
missing refund
missing payout
```

This is essential for operating a real financial system.

---

# 43. Reconciliation Is Not the Primary Flow

Reconciliation is a safety net, not a replacement for correct transactional
state management and idempotent provider operations.

Normal operations should be correct without waiting for reconciliation.

---

# 44. Security

Never store raw payment credentials such as card numbers or CVVs unless there
is an exceptional, explicitly justified compliance architecture.

Prefer provider tokenization/payment-method references.

Store only the information necessary for business operations and reconciliation.

---

# 45. PCI/Sensitive Data Boundary

The platform should minimize the payment-card data it handles directly.

The final integration should be designed around the chosen provider's secure
hosted/tokenized payment flow and applicable compliance requirements.

Provider-specific compliance decisions are deferred until provider selection.

---

# 46. Webhook Security

Webhook processing must verify provider authenticity before changing financial
state.

Do not allow an unauthenticated request to claim:

```text
payment captured
payment refunded
payout completed
```

---

# 47. Authorization

Payment operations require appropriate backend authorization.

Examples:

```text
customer → own payment methods/payment information
customer → own charges/refunds where policy permits
platform → settlement operations
admin → authorized financial operations
provider webhook → verified provider identity
```

Drivers must not be able to manipulate their own earnings through client
requests.

---

# 48. Auditability

Important financial records should allow reconstruction of:

```text
what happened
when it happened
which ride
which customer
which driver
amount
currency
provider
provider reference
pricing version
commission version
reason/source
```

Financial records should be append-oriented wherever practical rather than
silently overwriting history.

---

# 49. Observability

Useful metrics include:

```text
payment_authorization_total
payment_capture_total
payment_failure_total
refund_total
settlement_total
settlement_failure_total
payout_total
payment_webhook_total
payment_reconciliation_mismatch_total
```

Useful tracing/logging fields include:

```text
request_id
payment_id
operation_id
ride_id
provider_reference
```

Avoid high-cardinality financial/customer identifiers as metric labels.

---

# 50. Failure Handling

If PostgreSQL cannot commit a required financial transition:

```text
operation does not succeed
```

If the provider succeeds but the response is lost:

```text
do not blindly repeat the charge
resolve operation using idempotency/provider lookup/webhook
```

If webhook processing fails:

```text
provider event remains recoverable
retry processing
reconcile if necessary
```

---

# 51. Concurrency

Concurrent operations must not produce:

```text
duplicate charges
duplicate refunds
duplicate payouts
duplicate settlement records
```

Use database uniqueness/idempotency constraints and provider idempotency where
available.

---

# 52. Historical Integrity

Do not mutate historical financial records merely because current configuration
changed.

Historical records should retain sufficient snapshots/references for:

```text
pricing rule
commission rule
currency
amount
provider operation
```

---

# 53. Payment State vs Ride State

The following combinations are valid:

```text
TRIP_COMPLETED + PAYMENT_PENDING
TRIP_COMPLETED + PAYMENT_FAILED
TRIP_COMPLETED + PAYMENT_CAPTURED
```

The ride lifecycle and payment lifecycle are separate state machines.

---

# 54. Settlement State vs Payment State

Likewise:

```text
PAYMENT_CAPTURED + SETTLEMENT_PENDING
PAYMENT_CAPTURED + SETTLED
PAYMENT_FAILED + no settlement
```

Settlement must consume authoritative financial records rather than infer state
from a ride status alone.

---

# 55. Complete Flow

```text
                     TRIP COMPLETED
                           │
                           ▼
                       FINAL FARE
                           │
                           ▼
                        PAYMENT
                           │
                 ┌─────────┴─────────┐
                 ▼                   ▼
          Payment Provider       Payment Record
                 │
                 ▼
              CAPTURED
                 │
                 ▼
             SETTLEMENT
                 │
          ┌──────┴──────┐
          ▼             ▼
    Platform Fee    Driver Earnings
                        │
                        ▼
                      PAYOUT
```

---

# 56. What We Should Not Build Yet

Do not build:

```text
specific payment-provider integration
multi-provider routing
full double-entry accounting platform
instant driver payouts
complex wallet system
credit system
loans/advances
subscription billing
complex tax engine
multi-currency settlement
advanced fraud platform
```

Those require separate product, geographic, compliance, and operational
requirements.

---

# 57. Design Principles

1. Pricing determines what is owed; payment collects it; settlement determines distribution; payout transfers funds.
2. Do not authorize/capture payment for every bid.
3. Payment consumes an authoritative final fare and never recalculates pricing.
4. Customer payment and driver payout are separate workflows.
5. Trip completion and payment success are independent states.
6. Payment failure does not undo a completed trip.
7. Every effectful payment operation must be idempotent.
8. Provider webhooks must be authenticated and deduplicated.
9. Unknown provider outcomes must be resolved rather than blindly retried.
10. Refunds are explicit operations and do not mutate original payment history.
11. Platform commission and driver earnings are explicit financial records.
12. Commission rules should be versioned/auditable.
13. PostgreSQL is the durable financial authority.
14. Redis is never the financial source of truth.
15. Provider-specific APIs remain behind an adapter boundary.
16. Sensitive payment credentials should not be stored unnecessarily.
17. Financial records must support reconciliation and audit.
18. External payment operations require retry and reconciliation strategies because they are not part of PostgreSQL transactions.
19. Settlement is independent of customer payment status.
20. Avoid building sophisticated accounting/payment infrastructure until the actual launch requirements justify it.
