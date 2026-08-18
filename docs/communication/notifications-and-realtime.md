# Notifications and Realtime Communication

## 1. Purpose

This document defines how durable domain changes become realtime application
updates and push notifications.

The core architecture is:

```text
Domain Transaction
      ↓
PostgreSQL
      ↓
Outbox
      ↓
Event Consumers
   ┌──┴──┐
   ▼     ▼
Realtime Push
   │     │
   ▼     ▼
WebSocket APNs/FCM
   └──┬──┘
      ▼
   Flutter
```

WebSocket and push delivery are not sources of truth. PostgreSQL/domain state
remains authoritative.

---

# 2. Core Principles

1. Domain state is authoritative; delivery mechanisms are not.
2. Domain transactions publish durable outbox events.
3. WebSocket provides low-latency in-app updates.
4. Push notifications attract user attention when the app may be inactive.
5. Clients must tolerate duplicate, delayed, or missed delivery.
6. Reconnection must perform authoritative state synchronization.
7. Realtime ordering is scoped rather than globally guaranteed.
8. Event delivery should target at-least-once semantics.
9. Driver location is high-frequency operational data and should not be written to PostgreSQL for every GPS update.
10. WebSocket connectivity does not by itself determine driver availability.
11. Channels must enforce authorization.
12. Event payloads should be small and versioned.
13. Push/WebSocket duplication is acceptable when they serve different purposes.
14. Notification failure must not roll back a committed business transaction.

---

# 3. WebSocket Responsibility

WebSockets should provide low-latency updates such as:

```text
new driver bid
bid changes/expiration
ride state changes
driver confirmation
driver en route
driver arrival
trip start
trip completion
cancellation
```

The business command itself should not depend on a live WebSocket.

Example:

```text
Driver submits bid
      ↓
HTTP API
      ↓
PostgreSQL transaction
      ↓
outbox
      ↓
realtime event
      ↓
Rider WebSocket
```

---

# 4. Push Notification Responsibility

Push notifications are primarily for user attention when the application is
backgrounded or inactive.

Potential examples:

```text
new bid received
driver confirmed
driver arriving
ride cancelled
payment issue
trip completed
```

A push notification is not authoritative state.

---

# 5. Push as a Wake-Up Mechanism

Where appropriate, the client should use a push notification to trigger a
refresh of authoritative state.

Conceptually:

```text
Push
 ↓
open/wake application
 ↓
fetch authoritative state
 ↓
render current state
```

Do not make critical correctness depend on the exact push payload arriving.

---

# 6. WebSocket Reconnection

Mobile connections can disappear because of:

```text
network changes
poor connectivity
backgrounding
OS process termination
server restart
```

The recovery flow is:

```text
WebSocket disconnect
      ↓
reconnect
      ↓
authenticate
      ↓
authorize
      ↓
state synchronization
      ↓
resume realtime updates
```

Do not attempt to guarantee perfect event delivery to a disconnected mobile
client.

---

# 7. State Synchronization

After reconnect, the client should retrieve authoritative current state.

Conceptually:

```text
GET /rides/{ride_id}
```

or an equivalent synchronization operation.

The client then resumes realtime events from the synchronized state.

---

# 8. Snapshot + Events

The intended client model is:

```text
Initial snapshot
      ↓
Realtime events
      ↓
Local state updates
      ↓
Disconnect
      ↓
Reconnect
      ↓
New snapshot
      ↓
Realtime events continue
```

This is simpler and more resilient than requiring mobile clients to replay every
missed event.

---

# 9. Event Identity

Realtime events should have a unique event ID.

Example:

```json
{
  "event_id": "evt_123",
  "type": "ride.driver_arrived",
  "ride_id": "ride_123"
}
```

The event ID supports deduplication, tracing, and debugging.

Clients should tolerate receiving the same event more than once.

---

# 10. Event Sequence

Where ordering matters, events should carry a sequence/version within a defined
scope.

Example:

```json
{
  "event_id": "evt_123",
  "sequence": 42,
  "type": "ride.status_changed",
  "ride_id": "ride_123"
}
```

If a client observes:

```text
42
44
```

it can detect a possible gap and request authoritative synchronization.

---

# 11. Ordering Scope

Do not promise global ordering across the entire system.

Ordering should be defined where business semantics require it.

For example:

```text
ride_123 → 40, 41, 42
ride_456 → 18, 19
```

No ordering guarantee is required between different rides.

---

# 12. Ride Channel

A conceptual rider channel is:

```text
ride:{ride_id}
```

Potential externally visible events include:

```text
ride.bidding_started
ride.bid_received
ride.bid_updated
ride.bid_expired
ride.reserved
ride.driver_confirmed
ride.driver_en_route
ride.driver_arrived
ride.trip_started
ride.trip_completed
ride.cancelled
```

Not every internal domain event needs to be exposed to clients.

---

# 13. Driver Channel

A driver may have a user/driver-specific channel such as:

```text
driver:{driver_id}
```

Potential events:

```text
ride.opportunity.created
ride.opportunity.expired
ride.opportunity.cancelled
ride.reservation.created
ride.reservation.cancelled
```

The server must enforce authorization for subscriptions.

---

# 14. Location Updates

Driver location is high-frequency operational data.

It should not be persisted to PostgreSQL on every GPS update.

Conceptually:

```text
Driver Flutter
      ↓
location update
      ↓
Redis operational state
      ↓
realtime location stream
      ↓
relevant rider
```

Durable location history, if required later, should be sampled or aggregated.

---

# 15. Location Frequency

Do not initially send GPS coordinates every few hundred milliseconds.

That increases:

```text
battery usage
network traffic
Redis writes
WebSocket load
client rendering load
```

Use configurable thresholds based on:

```text
distance moved
time elapsed
trip state
```

Exact values should be determined through testing and operational data.

---

# 16. Location Freshness

Location data should include its update time.

Conceptually:

```text
latitude
longitude
updated_at
```

If location becomes stale, the client should not present it as live.

---

# 17. Push and WebSocket Duplication

The same business event may produce both:

```text
WebSocket event
+
push notification
```

This is acceptable.

For example:

```text
WebSocket → update active UI
Push      → alert inactive user
```

The client should handle duplicate underlying events safely.

---

# 18. Push Token Lifecycle

The backend should model devices/tokens rather than a single user-level push
token.

A device token can:

```text
be registered
change
expire
be invalidated
```

The system should support token refresh and removal of invalid tokens.

---

# 19. Multiple Devices

A user may have multiple active devices.

Conceptually:

```text
User
 ├── Device A → token A
 ├── Device B → token B
 └── Device C → token C
```

Notification delivery should target valid registered devices according to the
account/device policy.

---

# 20. Notification Preferences

Eventually users may control categories such as:

```text
ride updates
promotional messages
payment notifications
system notifications
```

Critical transactional notifications may remain mandatory according to product
policy.

Preference management should not accidentally disable essential operational
communication.

---

# 21. Authentication

WebSocket authentication should use the existing external OIDC identity model.

Conceptually:

```text
Flutter
  ↓
OIDC authentication
  ↓
access token
  ↓
WebSocket connection
  ↓
backend validates token
```

The backend derives identity from the verified token.

---

# 22. WebSocket Authorization

Authentication answers:

```text
Who are you?
```

Authorization answers:

```text
What may you subscribe to?
```

Examples:

```text
Rider A → ride A       ✓
Rider A → ride B       ✗

Driver A → driver A   ✓
Driver A → driver B   ✗
```

The server must enforce this regardless of client claims.

---

# 23. Connection Lifecycle

Conceptually:

```text
CONNECT
  ↓
AUTHENTICATE
  ↓
AUTHORIZE
  ↓
SUBSCRIBE
  ↓
ACTIVE
  ↓
DISCONNECT
```

Subscriptions and connection resources must be cleaned up when the connection
closes.

---

# 24. Heartbeats

WebSocket connections should use ping/pong or an equivalent heartbeat mechanism
to detect dead connections.

Conceptually:

```text
ping
 ↓
pong
 ↓
healthy
```

Dead connections should eventually be removed.

---

# 25. WebSocket Health vs Driver Availability

A live WebSocket does not automatically mean:

```text
driver = AVAILABLE
```

Connection health and operational availability are separate concepts.

Driver availability remains governed by the driver presence/availability domain.

---

# 26. Event Delivery Semantics

V1 should target:

```text
at-least-once delivery
```

rather than exactly-once delivery.

Consumers must tolerate:

```text
duplicates
delayed delivery
missing events after disconnection
reordering where not guaranteed
```

Authoritative state synchronization handles gaps.

---

# 27. Durable Domain Events vs Delivery Events

A domain event represents a durable business fact.

A delivery event represents an attempt to communicate that fact to a client.

```text
Domain Event
   ↓
durable business fact

Delivery Event
   ↓
client communication
```

If delivery fails, the business transaction remains committed.

---

# 28. Outbox

Domain transactions should use the outbox pattern:

```text
PostgreSQL transaction
      │
      ├── business state
      │
      └── outbox event
              ↓
        event publisher
              ↓
     realtime / push consumers
```

This prevents the business state from committing while the corresponding event
is silently lost.

---

# 29. Push Retry

Push delivery failures should be classified.

```text
retryable failure
permanent failure
invalid token
```

Retry transient failures according to a bounded policy.

Do not retry permanently invalid device tokens forever.

---

# 30. Realtime Gateway Failure

If the WebSocket/realtime gateway crashes:

```text
clients reconnect
      ↓
authoritative state synchronization
      ↓
realtime resumes
```

A realtime gateway failure must not corrupt ride, bid, reservation, or payment
state.

---

# 31. Event Consumer Failure

If a consumer fails after the domain transaction commits:

```text
outbox remains available
      ↓
consumer retries
      ↓
event delivered
```

The outbox provides durable recovery for downstream delivery.

---

# 32. Privacy

Realtime channels must expose only data the authenticated user is authorized
to receive.

Do not expose:

```text
other drivers' identities
other drivers' locations
candidate ranking
internal eligibility decisions
provider secrets
internal operational metadata
```

---

# 33. Rider Location

Rider location is sensitive operational data.

It should only be exposed to the relevant driver when the ride state and product
policy permit it.

Rider location must not become a globally discoverable resource.

---

# 34. Driver Location

Driver location should be exposed to the rider only when appropriate, generally
after a relevant driver has been selected/confirmed.

Discovery's internal driver-location inventory must never become a public rider
feed.

---

# 35. Event Payload Design

Events should contain enough information for the UI without becoming giant
domain objects.

Example:

```json
{
  "event_id": "evt_123",
  "sequence": 42,
  "type": "ride.driver_arrived",
  "ride_id": "ride_123",
  "occurred_at": "2026-08-18T10:30:00Z"
}
```

For complex state, the client can fetch authoritative details through the API.

---

# 36. Schema Versioning

Realtime event schemas will evolve.

Events should therefore carry an explicit schema version where appropriate:

```text
event_type
schema_version
```

Do not silently change the meaning of an existing event payload.

---

# 37. API Fallback

Realtime is an optimization for responsiveness.

The API remains the fallback for:

```text
initial state
reconnect synchronization
manual refresh
historical data
state reconciliation
```

The mobile application should remain functionally correct even if realtime
communication is temporarily unavailable.

---

# 38. Notifications vs UI State

A notification should not be treated as the application's database.

Bad:

```text
push says "Driver arrived"
→ permanently set local state to ARRIVED
```

Better:

```text
push says "Driver arrived"
→ wake/update app
→ fetch authoritative state
→ render current state
```

The same principle applies to realtime events where practical.

---

# 39. Event Visibility

Internal events and client-facing events should be distinct concepts.

For example, an internal discovery event may contain operational information
that should never be sent to a rider.

A delivery mapper/adapter should expose only the approved client contract.

---

# 40. Notification Content

Push payloads should contain minimal sensitive information.

Prefer:

```text
ride_id
notification type
short display-safe message
```

rather than embedding complete ride, payment, or identity records.

The app can fetch details after opening when necessary.

---

# 41. Background Behavior

Mobile operating systems may restrict realtime connections while the app is in
the background.

Therefore the architecture must not assume a permanently connected WebSocket.

Push notifications and subsequent state synchronization provide the recovery
path.

---

# 42. Reconnection Backoff

Clients should use bounded exponential/backoff-style reconnect behavior rather
than reconnecting in a tight loop.

This prevents a server/network outage from creating a reconnect storm.

Exact client parameters are an implementation detail.

---

# 43. Server Backpressure

The realtime system must avoid allowing one fast producer or slow client to
consume unbounded memory.

Potential controls include:

```text
bounded per-connection buffers
message coalescing for high-frequency location updates
disconnect slow consumers when necessary
```

A slow mobile client should not threaten gateway stability.

---

# 44. Location Coalescing

High-frequency location updates may be coalesced because intermediate positions
can become irrelevant.

For example:

```text
location 1
location 2
location 3
location 4
```

may become simply:

```text
latest valid location
```

for a slow consumer.

Do not apply this blindly to ordered business events.

---

# 45. Authorization Revocation

If a user's authorization changes, active realtime subscriptions should no
longer provide access to resources the user is no longer entitled to see.

The exact token/session revocation mechanism is part of the security design.

---

# 46. PostgreSQL Responsibilities

PostgreSQL remains authoritative for durable business state such as:

```text
ride lifecycle
bids
reservations
assignments
cancellations
payment state
```

The realtime system should derive client updates from these authoritative
changes.

---

# 47. Redis Responsibilities

Redis may support:

```text
current driver location
presence
short-lived connection/routing state
realtime fan-out coordination
```

Redis does not become the authoritative record of ride or payment state.

---

# 48. Observability

Useful metrics include:

```text
websocket_connections_active
websocket_connect_total
websocket_disconnect_total
websocket_auth_failure_total
realtime_event_published_total
realtime_event_delivery_total
realtime_event_delivery_failure_total
push_notification_sent_total
push_notification_failure_total
push_invalid_token_total
realtime_reconnect_total
```

Useful trace/log fields include:

```text
request_id
connection_id
event_id
ride_id
user_id
event_type
```

Avoid high-cardinality identifiers as metric labels.

---

# 49. Testing Requirements

Realtime testing should cover:

```text
connect/authenticate/authorize
subscription authorization
reconnect
state synchronization
duplicate events
out-of-order events where applicable
event gaps
slow consumers
push token invalidation
webhook/outbox consumer retries
```

The core correctness test is that a disconnected client can reconnect and recover
its authoritative state.

---

# 50. What We Should Not Build Yet

Do not build:

```text
custom event-streaming platform
exactly-once mobile delivery
full event replay system for mobile clients
persistent per-client event history
complex presence federation
high-frequency GPS persistence
custom push provider
multiple realtime protocols
```

The initial architecture only needs reliable outbox-driven delivery, WebSockets,
push, and API-based state reconciliation.

---

# 51. Complete Communication Flow

```text
                    DOMAIN TRANSACTION
                           │
                           ▼
                       PostgreSQL
                           │
                           ▼
                         Outbox
                           │
                           ▼
                    Event Publisher
                           │
                ┌──────────┴──────────┐
                ▼                     ▼
         Realtime Gateway        Push Service
                │                     │
                ▼                     ▼
            WebSocket              APNs/FCM
                │                     │
                └──────────┬──────────┘
                           ▼
                         Flutter
                           │
                     disconnected?
                           │
                           ▼
                  API synchronization
                           │
                           ▼
                 authoritative state
```

---

# 52. Design Principles

1. PostgreSQL/domain state is authoritative.
2. WebSocket and push are delivery mechanisms, not sources of truth.
3. Domain transactions publish durable outbox events.
4. WebSocket optimizes active UI responsiveness.
5. Push alerts inactive users.
6. Mobile clients must tolerate duplicate and missed delivery.
7. Reconnection always has an authoritative state synchronization path.
8. At-least-once delivery is preferred over fragile exactly-once delivery.
9. Event ordering is scoped rather than globally guaranteed.
10. Event IDs support deduplication and debugging.
11. Location is high-frequency operational data and should not be persisted on every update.
12. WebSocket connectivity does not imply driver availability.
13. Channels enforce authentication and authorization.
14. Event payloads should be small, privacy-aware, and versioned.
15. Push payloads should contain minimal sensitive information.
16. Slow consumers must not cause unbounded server memory growth.
17. Realtime failure must not corrupt business state.
18. Redis may support operational realtime state but never owns durable business truth.
19. API synchronization is the recovery mechanism for disconnected clients.
20. Avoid building a custom event-replay platform before actual requirements justify it.
