# Event-Driven Architecture

## 1. Purpose

This document defines domain events, the transactional outbox, event publishing,
consumer behavior, retries, ordering, idempotency, observability, and failure
recovery.

The central principle is:

> Events propagate committed business facts and trigger asynchronous work. They do not replace PostgreSQL transactions, domain ownership, or authoritative state.

---

# 2. Core Architecture

```text
                 Command / Use Case
                        │
                        ▼
                  PostgreSQL TX
                 ┌──────┴──────┐
                 │             │
                 ▼             ▼
           Business State    Outbox
                               │
                               ▼
                        Event Publisher
                               │
                 ┌─────────────┼─────────────┐
                 ▼             ▼             ▼
            Realtime         Push        Analytics
```

---

# 3. Domain Events

Events represent facts that already happened.

Examples:

```text
ride.created
ride.bidding_started
bid.created
bid.selected
reservation.created
assignment.created
ride.cancelled
ride.driver_arrived
ride.started
ride.completed
payment.authorized
payment.captured
payment.failed
settlement.created
```

Events should use past-tense business language.

---

# 4. Commands vs Events

A command means:

```text
Do this.
```

An event means:

```text
This happened.
```

Examples:

```text
CancelRide       → command
ride.cancelled   → event
```

Do not use events as disguised commands such as `please_cancel_ride`.

---

# 5. Transactional Outbox

Business state and the corresponding outbox event are written in one PostgreSQL
transaction.

```text
BEGIN
  update business state
  insert outbox event
COMMIT
```

Never rely on:

```text
UPDATE PostgreSQL
COMMIT
publish event
```

because a crash between the two operations can lose the event.

---

# 6. Outbox Record

Conceptually:

```text
outbox_events
-------------
id
event_type
aggregate_type
aggregate_id
payload
created_at
published_at
attempt_count
last_error
```

The final physical schema is an implementation decision.

---

# 7. Publisher

A background publisher reads unpublished events in bounded batches and forwards
them to the appropriate delivery/consumer mechanism.

The publisher must not load the entire outbox into memory.

---

# 8. At-Least-Once Delivery

The system deliberately targets:

```text
at-least-once delivery
```

rather than exactly-once processing.

A crash can occur after a consumer performs work but before it records successful
processing. The same event may therefore be delivered again.

Consumers must be idempotent.

---

# 9. Consumer Idempotency

Consumers must tolerate duplicate events.

Conceptually:

```text
processed_events
----------------
consumer
 event_id
processed_at
```

A unique constraint on the consumer/event identity can prevent duplicate logical
processing where appropriate.

---

# 10. Event Ordering

Do not promise global event ordering.

Ordering is scoped to aggregates where business semantics require it.

Example:

```text
ride_123
  41 ride.created
  42 ride.bidding_started
  43 bid.selected
  44 reservation.created

ride_456
  12 ride.created
  13 ride.cancelled
```

There is no required ordering relationship between `ride_123` and `ride_456`.

---

# 11. Event Sequences

Where ordering matters, events can carry an aggregate-scoped sequence/version.

Example:

```json
{
  "event_id": "evt_123",
  "aggregate_id": "ride_123",
  "sequence": 44,
  "type": "reservation.created"
}
```

Consumers can detect possible gaps such as:

```text
44
46
```

and reconcile against authoritative state.

---

# 12. Event Retention

The outbox is primarily a reliable delivery mechanism, not automatically the
permanent historical event store.

Durable business history remains in domain records such as:

```text
rides
bids
assignments
payments
settlements
```

Published outbox records may eventually be archived or removed according to
retention requirements, provided required consumers have completed processing.

---

# 13. Domain vs Integration Events

A domain event represents an internal business fact.

A client-facing/integration event is an externally shaped representation.

Conceptually:

```text
Domain Event
    ↓
client/integration event model
```

Do not expose internal domain objects directly as public event contracts.

---

# 14. Event Payloads

Keep event payloads reasonably small.

Good:

```json
{
  "event_id": "evt_123",
  "ride_id": "ride_123",
  "reason": "RIDER_CHANGED_MIND",
  "occurred_at": "2026-08-18T10:30:00Z"
}
```

Avoid embedding complete rider, driver, ride, and payment records unless a real
consumer requirement justifies it.

Consumers may retrieve authoritative state when necessary.

---

# 15. Event Schema Versioning

Event contracts evolve.

Use:

```text
event_type
schema_version
```

Do not silently change the meaning of an existing event payload.

---

# 16. Consumer Categories

Initial consumer categories include:

```text
Realtime
Push notifications
Analytics
```

Future consumers may include:

```text
fraud
support
driver metrics
financial reconciliation
```

Consumers should be independently retryable.

---

# 17. Realtime Consumer

Realtime consumes domain facts and projects them into client-facing WebSocket
events.

```text
ride.cancelled
      ↓
realtime consumer
      ↓
ride.cancelled WebSocket event
```

The realtime consumer does not modify authoritative ride state.

---

# 18. Notification Consumer

Notifications consume relevant events and produce push notifications.

```text
ride.driver_arrived
      ↓
notification consumer
      ↓
push notification
```

Push failure must not roll back the ride transaction.

---

# 19. Analytics Consumer

Analytics is downstream of business correctness.

```text
ride.completed
      ↓
PostgreSQL commit
      ↓
event
      ↓
analytics
```

Analytics failure must not prevent a trip from completing.

---

# 20. Financial Consumers

Financial workflows require durable domain state and stronger idempotency.

An event may trigger a payment use case:

```text
trip.completed
      ↓
payment workflow
      ↓
payment domain/use case
```

A consumer must not bypass payment-domain validation merely because it received an
event.

---

# 21. Event-Triggered Workflows

Asynchronous workflows should maintain durable state at each important business
step.

Example:

```text
trip.completed
      ↓
fare finalization
      ↓
payment workflow
      ↓
settlement
```

Do not create one opaque asynchronous chain whose state exists only in logs.

---

# 22. Retry Strategy

Classify failures as:

```text
transient
permanent
unknown
```

Transient failures are retried.

Permanent failures move toward explicit failure/dead-letter handling.

Unknown failures receive bounded retries before escalation.

---

# 23. Retry Backoff

Retries use bounded backoff rather than immediate infinite retry loops.

Conceptually:

```text
1s
2s
4s
8s
...
max delay
```

Exact parameters are operational configuration.

---

# 24. Dead-Letter Handling

Repeatedly failing events eventually leave the normal retry loop.

```text
outbox
  ↓
consumer
  ↓
retry
  ↓
retry
  ↓
dead-letter
```

Dead-letter records must be observable and recoverable.

A dead-letter mechanism that nobody monitors is a failure sink, not reliability.

---

# 25. Poison Events

Malformed or permanently invalid events must not retry forever.

Consumers should classify such failures and move them to dead-letter handling.

---

# 26. Consumer Isolation

A broken consumer must not block unrelated consumers.

Preferred model:

```text
             event
        ┌─────┼─────┐
        ▼     ▼     ▼
    realtime push analytics
```

Each consumer has independent processing and retry behavior.

---

# 27. Backpressure

Consumers should use:

```text
bounded fetch
bounded worker concurrency
bounded retries
```

Do not allow event backlog processing to create unbounded memory usage.

---

# 28. Worker Concurrency

Concurrency should be configurable and constrained by ordering requirements.

Events for different aggregates can generally process independently.

Events for the same aggregate may require ordered processing for a particular
consumer.

---

# 29. Per-Aggregate Ordering

A practical ordering key is:

```text
aggregate_id
```

For example:

```text
ride_123 → ordered
ride_456 → independent
```

This provides useful business ordering without globally serializing all events.

---

# 30. Event Duplication

Duplicate delivery is expected.

Example:

```text
consumer processes event
      ↓
external side effect succeeds
      ↓
consumer crashes before recording success
      ↓
event is retried
```

The consumer and external operation must be designed to tolerate this where
possible.

---

# 31. External Side Effects

External integrations require their own idempotency strategy where supported.

This is particularly important for:

```text
payment providers
push providers
external APIs
```

Do not assume the local consumer's database transaction can atomically include an
external provider.

---

# 32. Event Publication Failure

If publishing fails after the business transaction commits:

```text
PostgreSQL state remains committed
outbox remains pending
publisher retries later
```

The outbox is the recovery mechanism.

---

# 33. Transactions vs Events

Events must not replace transactions for operations that require atomicity.

If two changes must happen together to preserve a business invariant, they belong
in the same application use case and database transaction.

Events decouple downstream work after the fact is durably committed.

---

# 34. Domain Ownership

Each event has an owning domain.

Examples:

```text
ride.cancelled
    → Ride domain

bid.selected
    → Bidding/Ride domain

payment.captured
    → Payment domain

settlement.created
    → Settlement domain
```

Other consumers may react to the event but do not become owners of that business
fact.

---

# 35. Event Naming

Use clear past-tense facts:

```text
ride.created
ride.cancelled
bid.created
bid.selected
reservation.created
assignment.created
trip.started
trip.completed
payment.authorized
payment.captured
payment.failed
```

Avoid ambiguous names such as:

```text
ride.update
ride.process
ride.change
```

---

# 36. Event Metadata

Useful metadata includes:

```text
event_id
event_type
schema_version
aggregate_type
aggregate_id
occurred_at
producer
correlation_id
causation_id
```

Metadata should remain purposeful rather than becoming arbitrary application state.

---

# 37. Correlation and Causation

Use correlation identifiers to connect a workflow and causation identifiers to
explain which event caused another event/workflow.

Example:

```text
ride.completed
    ↓
payment workflow
    ↓
payment.captured
```

This is especially useful for debugging asynchronous workflows.

---

# 38. Event Observability

Events should be traceable through:

```text
event_id
aggregate_id
event_type
correlation_id
```

Metrics should avoid using raw aggregate/event IDs as labels because of high
cardinality.

---

# 39. Event Security

Do not publish sensitive information unnecessarily.

Events should not casually contain:

```text
OIDC access tokens
payment credentials
complete user profiles
unnecessary private location/address data
```

Publish the minimum information required by consumers.

---

# 40. Client Event Authorization

Internal events are not automatically client-visible.

For example:

```text
driver.eligibility.evaluated
```

may remain internal.

Client-facing events are explicitly projected by the realtime/notification layer.

---

# 41. Outbox Cleanup

Published outbox records should eventually be archived or removed according to
retention policy.

Cleanup must not occur before required consumers have completed processing.

Avoid indefinite outbox growth.

---

# 42. Event Contract Testing

Event contracts should have tests covering:

```text
serialization
deserialization
required fields
schema versions
backward compatibility
consumer behavior
duplicate delivery
```

---

# 43. Failure Recovery

Recovery should be possible without manually mutating business state.

Conceptually:

```text
consumer failed
     ↓
inspect event
     ↓
retry/replay
     ↓
consumer succeeds
```

Manual production table edits should not be the normal recovery mechanism.

---

# 44. Operational Replay

V1 should support replay/retry of failed outbox events where necessary.

This is operational recovery, not a full event-sourcing system.

---

# 45. Event Sourcing Boundary

Full event sourcing is explicitly out of scope for V1.

The system uses:

```text
current domain state
+
selected historical records
+
outbox events
```

rather than reconstructing all state exclusively from events.

---

# 46. Testing Requirements

Event-driven tests should cover:

```text
outbox atomicity
publisher retries
consumer idempotency
duplicate delivery
ordering constraints
dead-letter handling
poison events
external side-effect retries
schema compatibility
recovery/replay
```

---

# 47. What We Should Not Build Yet

Do not build:

```text
Kafka-sized custom event platform
full event sourcing
system-wide exactly-once delivery
global event ordering
per-client event replay history
complex event mesh
unbounded event retention
multiple broker technologies
```

The initial architecture only needs a durable PostgreSQL outbox, reliable
publishers/consumers, bounded retries, idempotency, and clear event contracts.

---

# 48. Complete Architecture

```text
                         COMMAND
                            │
                            ▼
                      APPLICATION
                            │
                            ▼
                    PostgreSQL TX
                   ┌────────┴────────┐
                   ▼                 ▼
             Domain State          Outbox
                                     │
                                     ▼
                              Event Publisher
                                     │
                  ┌──────────────────┼──────────────────┐
                  ▼                  ▼                  ▼
              Realtime             Push             Analytics
                  │                  │                  │
                  ▼                  ▼                  ▼
             WebSocket             APNs/FCM         Data pipeline

       Each consumer:
       ┌───────────────────────────────────────┐
       │ receive → validate → process → record │
       │             │                         │
       │             └→ retry → DLQ            │
       └───────────────────────────────────────┘
```

---

# 49. Design Principles

1. Events represent committed business facts.
2. Commands request actions; events describe completed facts.
3. Business state and outbox events commit atomically.
4. PostgreSQL remains authoritative for durable business state.
5. Delivery targets at-least-once semantics.
6. Consumers must be idempotent.
7. Event ordering is aggregate-scoped rather than global.
8. Event sequences can detect gaps where required.
9. Domain events and client-facing events are separate contracts.
10. Event payloads remain small and privacy-aware.
11. Event schemas are explicitly versioned.
12. Consumers operate independently and fail independently.
13. Retries are bounded and use backoff.
14. Poison events move to dead-letter handling.
15. Slow consumers must not create unbounded memory growth.
16. External side effects require their own idempotency strategy.
17. Events do not replace database transactions.
18. Events do not bypass domain ownership.
19. Correlation and causation metadata support distributed debugging.
20. Operational replay is supported without adopting event sourcing.
21. Event retention is finite unless a concrete requirement justifies otherwise.
22. Full event sourcing and system-wide exactly-once processing are deferred.
