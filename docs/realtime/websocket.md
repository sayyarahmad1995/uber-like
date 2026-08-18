# WebSocket Design

## 1. Purpose

This document defines the real-time communication contract between the Flutter
application and the Go backend.

WebSocket is used for:

- Real-time ride state updates
- Live bidding updates
- Driver assignment notifications
- Driver location updates
- Driver arrival notifications
- Trip state updates
- Connection presence
- Real-time recovery notifications

REST remains responsible for authoritative commands and queries.

WebSocket is responsible for delivering real-time information.

---

# 2. REST vs WebSocket

The boundary is intentional.

REST:

```text
Commands
Queries
Authoritative resource state
Historical data
```

WebSocket:

```text
Real-time events
Live location
Bidding updates
Ride state notifications
Assignment notifications
```

Example:

```text
Driver
   │
   │ POST /api/v1/rides/{ride_id}/bids
   ▼
REST API
   │
   ├── PostgreSQL transaction
   │
   └── event
          │
          ▼
       WebSocket
          │
          ▼
        Rider
```

The WebSocket event does not create the bid.

The REST command creates the authoritative state.

---

# 3. Endpoint

Initial WebSocket endpoint:

```text
wss://<api-host>/ws
```

The connection is established over TLS.

Plain:

```text
ws://
```

must not be used in production.

---

# 4. Authentication

The WebSocket connection requires authentication.

The preferred initial mechanism is an access token supplied during the
connection handshake.

The exact transport mechanism will be finalized during implementation,
but the backend must establish the authenticated user before accepting
application-level messages.

The server must never trust a user ID supplied inside a WebSocket message.

The authenticated identity comes from the validated OIDC access token.

---

# 5. Connection Lifecycle

Initial lifecycle:

```text
CONNECTING
    ↓
AUTHENTICATING
    ↓
CONNECTED
    ↓
ACTIVE
    ↓
CLOSING
    ↓
CLOSED
```

If authentication fails:

```text
CONNECTING
    ↓
AUTHENTICATING
    ↓
AUTH_FAILED
    ↓
CLOSED
```

---

# 6. Connection Establishment

After the WebSocket handshake succeeds:

```text
Client
   ↓
WebSocket connection
   ↓
Authentication
   ↓
Server validates OIDC token
   ↓
Connection registered
   ↓
CONNECTED event
```

The server should then send a connection acknowledgment.

Example:

```json
{
  "type": "connection.ready",
  "version": 1,
  "connection_id": "conn_123"
}
```

The `connection_id` is an opaque identifier.

---

# 7. Message Envelope

All application messages use a common envelope.

Example:

```json
{
  "id": "msg_123",
  "type": "ride.state_changed",
  "version": 1,
  "timestamp": "2026-08-18T10:00:00Z",
  "data": {}
}
```

Fields:

| Field | Required | Purpose |
|---|---|---|
| `id` | Yes | Unique message/event ID |
| `type` | Yes | Message type |
| `version` | Yes | Message schema version |
| `timestamp` | Yes | Server-generated event timestamp |
| `data` | Yes | Message payload |

The envelope is intentionally small.

---

# 8. Server vs Client Messages

The protocol distinguishes between:

```text
Client → Server
Server → Client
```

Client messages are commands or subscription requests.

Server messages are notifications/events.

However, authoritative business commands should normally use REST.

Therefore the initial WebSocket protocol has very few client-to-server
application messages.

---

# 9. Server Event Naming

Event names use:

```text
<resource>.<event>
```

Examples:

```text
ride.state_changed
ride.bidding_started
ride.bidding_ended

bid.created
bid.updated
bid.withdrawn
bid.selected

assignment.confirmation_required
assignment.confirmed
assignment.rejected
assignment.expired

driver.location_updated
driver.arrived

trip.started
trip.completed
```

Event names describe what happened.

They do not instruct the client what to do.

---

# 10. Event IDs

Every server event has a unique ID.

Example:

```json
{
  "id": "evt_01J...",
  "type": "ride.state_changed",
  "version": 1,
  "timestamp": "2026-08-18T10:00:00Z",
  "data": {}
}
```

The event ID is useful for:

- Debugging
- Logging
- Observability
- Duplicate detection
- Troubleshooting

The client must not assume that event IDs are sequential.

---

# 11. Event Ordering

WebSocket provides ordered delivery over an individual TCP connection.

However, application-level ordering becomes more complicated when:

- Multiple backend instances exist
- Events are generated concurrently
- A client reconnects
- Events are published through Redis
- Messages are delayed or retried

Therefore the client must not rely solely on arrival order for business
correctness.

---

# 12. Resource Version

State-changing resources should expose a version or revision where useful.

Example:

```json
{
  "ride_id": "ride_123",
  "revision": 14,
  "status": "DRIVER_CONFIRMED"
}
```

The revision allows clients to detect stale or unexpected state.

Example:

```text
Client has revision 10

Receives revision 12

revision 11 was not observed
```

The client can request the authoritative ride state through REST.

---

# 13. Ride Subscription

The client may subscribe to real-time events for a ride it is authorized
to observe.

Conceptual client message:

```json
{
  "type": "ride.subscribe",
  "version": 1,
  "data": {
    "ride_id": "ride_123"
  }
}
```

The backend must verify:

- User identity
- Rider ownership, or
- Driver assignment/authorization

The server must not allow a client to subscribe to arbitrary rides.

---

# 14. Ride Unsubscription

Conceptual client message:

```json
{
  "type": "ride.unsubscribe",
  "version": 1,
  "data": {
    "ride_id": "ride_123"
  }
}
```

The server removes the subscription.

Subscriptions are connection-scoped.

They do not become durable database records.

---

# 15. Ride State Event

When a ride's authoritative state changes, the server may publish:

```text
ride.state_changed
```

Example:

```json
{
  "id": "evt_123",
  "type": "ride.state_changed",
  "version": 1,
  "timestamp": "2026-08-18T10:01:00Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 7,
    "status": "DRIVER_CONFIRMED"
  }
}
```

The event contains enough information for the client to update its UI.

For complete resource information, the client may fetch:

```text
GET /api/v1/rides/{ride_id}
```

---

# 16. Bidding Started

When a ride enters the bidding phase:

```text
ride.bidding_started
```

Example:

```json
{
  "id": "evt_124",
  "type": "ride.bidding_started",
  "version": 1,
  "timestamp": "2026-08-18T10:00:00Z",
  "data": {
    "ride_id": "ride_123",
    "ends_at": "2026-08-18T10:00:30Z"
  }
}
```

The client uses the server-provided deadline.

The client timer is presentation logic only.

---

# 17. Bid Created

When a driver's bid is successfully created:

```text
bid.created
```

Example:

```json
{
  "id": "evt_125",
  "type": "bid.created",
  "version": 1,
  "timestamp": "2026-08-18T10:00:05Z",
  "data": {
    "ride_id": "ride_123",
    "bid": {
      "id": "bid_123",
      "amount": 1100,
      "currency": "PKR",
      "status": "ACTIVE"
    }
  }
}
```

The rider receives bid information appropriate to the rider's authorization.

A driver must not receive competing driver's private bid information.

---

# 18. Bid Updated

When a driver changes their bid:

```text
bid.updated
```

Example:

```json
{
  "id": "evt_126",
  "type": "bid.updated",
  "version": 1,
  "timestamp": "2026-08-18T10:00:10Z",
  "data": {
    "ride_id": "ride_123",
    "bid": {
      "id": "bid_123",
      "amount": 1050,
      "currency": "PKR",
      "status": "ACTIVE"
    }
  }
}
```

---

# 19. Bid Withdrawn

When a driver withdraws a bid:

```text
bid.withdrawn
```

Example:

```json
{
  "id": "evt_127",
  "type": "bid.withdrawn",
  "version": 1,
  "timestamp": "2026-08-18T10:00:12Z",
  "data": {
    "ride_id": "ride_123",
    "bid_id": "bid_123"
  }
}
```

---

# 20. Bid Selected

When the rider selects a bid:

```text
bid.selected
```

Example:

```json
{
  "id": "evt_128",
  "type": "bid.selected",
  "version": 1,
  "timestamp": "2026-08-18T10:00:20Z",
  "data": {
    "ride_id": "ride_123",
    "bid_id": "bid_123",
    "driver_id": "driver_123",
    "agreed_fare": {
      "amount": 1100,
      "currency": "PKR"
    }
  }
}
```

This event should only be delivered to authorized participants.

---

# 21. Assignment Confirmation Required

When a rider selects a driver, the selected driver receives:

```text
assignment.confirmation_required
```

Example:

```json
{
  "id": "evt_129",
  "type": "assignment.confirmation_required",
  "version": 1,
  "timestamp": "2026-08-18T10:00:20Z",
  "data": {
    "ride_id": "ride_123",
    "confirmation_deadline": "2026-08-18T10:01:00Z"
  }
}
```

The driver confirms through:

```text
POST /api/v1/rides/{ride_id}/assignment/confirm
```

The WebSocket event does not perform the confirmation.

---

# 22. Assignment Confirmed

After the REST command successfully changes PostgreSQL state:

```text
assignment.confirmed
```

is delivered to authorized participants.

Example:

```json
{
  "id": "evt_130",
  "type": "assignment.confirmed",
  "version": 1,
  "timestamp": "2026-08-18T10:00:30Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 8
  }
}
```

---

# 23. Assignment Rejected

If the selected driver rejects the assignment:

```text
assignment.rejected
```

Example:

```json
{
  "id": "evt_131",
  "type": "assignment.rejected",
  "version": 1,
  "timestamp": "2026-08-18T10:00:35Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 9
  }
}
```

Fallback selection is handled by the backend.

The client does not select the fallback driver itself.

---

# 24. Assignment Expired

If the driver does not confirm before the deadline:

```text
assignment.expired
```

The timeout is authoritative only when the backend changes the ride/assignment
state in PostgreSQL.

Redis timers or background workers may trigger the operation.

---

# 25. Driver Location

Driver location is a high-frequency real-time message.

Example:

```json
{
  "id": "evt_200",
  "type": "driver.location_updated",
  "version": 1,
  "timestamp": "2026-08-18T10:02:00Z",
  "data": {
    "ride_id": "ride_123",
    "latitude": 33.6844,
    "longitude": 73.0479,
    "accuracy_meters": 8,
    "heading": 120,
    "speed_mps": 11.4,
    "recorded_at": "2026-08-18T10:02:00Z"
  }
}
```

Location updates should not be treated as durable business events.

The latest location is operational state.

---

# 26. Driver Location Direction

The driver publishes location:

```text
Driver Flutter
      ↓
WebSocket
      ↓
Go
      ↓
Redis
```

The backend may then fan out the location to the authorized rider:

```text
Redis
   ↓
Go
   ↓
Rider WebSocket
```

The driver must not publish directly to another client.

The backend remains the authorization boundary.

---

# 27. Driver Arrival

When the driver reports arrival through REST:

```text
POST /api/v1/rides/{ride_id}/arrival
```

and PostgreSQL transitions the ride:

```text
DRIVER_CONFIRMED
       ↓
DRIVER_ARRIVED
```

the backend publishes:

```text
driver.arrived
```

Example:

```json
{
  "id": "evt_210",
  "type": "driver.arrived",
  "version": 1,
  "timestamp": "2026-08-18T10:10:00Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 10
  }
}
```

---

# 28. Trip Started

After successful REST processing:

```text
POST /api/v1/rides/{ride_id}/start
```

the server publishes:

```text
trip.started
```

Example:

```json
{
  "id": "evt_220",
  "type": "trip.started",
  "version": 1,
  "timestamp": "2026-08-18T10:12:00Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 11
  }
}
```

---

# 29. Trip Completed

After successful REST processing:

```text
POST /api/v1/rides/{ride_id}/complete
```

the server publishes:

```text
trip.completed
```

Example:

```json
{
  "id": "evt_230",
  "type": "trip.completed",
  "version": 1,
  "timestamp": "2026-08-18T10:30:00Z",
  "data": {
    "ride_id": "ride_123",
    "revision": 12
  }
}
```

---

# 30. Client-to-Server Messages

The initial protocol should keep client-to-server WebSocket messages minimal.

Allowed initial messages:

```text
ride.subscribe
ride.unsubscribe
ping
```

Business commands remain REST operations.

This gives us a clear boundary:

```text
WebSocket
    ↓
real-time transport

REST
    ↓
business commands
```

---

# 31. Ping/Pong

The protocol should support heartbeat detection.

The server may send:

```json
{
  "type": "ping"
}
```

The client responds:

```json
{
  "type": "pong"
}
```

The WebSocket implementation may also use the underlying WebSocket protocol
ping/pong mechanism.

The application-level protocol should not duplicate functionality unless
needed by the client.

The exact heartbeat mechanism will be selected during implementation.

---

# 32. Connection Heartbeat

A connection is considered healthy only while heartbeats continue.

Conceptually:

```text
Client
   ↓
heartbeat
   ↓
Server
   ↓
refresh presence
```

If the connection stops responding:

```text
heartbeat timeout
       ↓
close connection
       ↓
presence expires
```

---

# 33. Reconnection

Mobile networks are unreliable.

The Flutter application must expect:

```text
Wi-Fi → mobile
mobile → Wi-Fi
network loss
backgrounding
OS suspension
server restart
```

The client should automatically reconnect using bounded exponential backoff.

Example conceptual sequence:

```text
1 second
2 seconds
4 seconds
8 seconds
16 seconds
...
maximum delay
```

The exact values belong to the Flutter implementation.

---

# 34. Reconnection Recovery

After reconnecting:

```text
CONNECT
   ↓
AUTHENTICATE
   ↓
CONNECTION READY
   ↓
RESTORE SUBSCRIPTIONS
   ↓
FETCH CURRENT AUTHORITATIVE STATE
   ↓
RESUME REAL-TIME EVENTS
```

The client should not assume that events sent while disconnected were
received.

---

# 35. State Recovery

For an active ride:

```text
GET /api/v1/rides/{ride_id}
```

should be used to recover authoritative state.

For active bidding:

```text
GET /api/v1/rides/{ride_id}
GET /api/v1/rides/{ride_id}/bids
```

may be used as required.

This is intentionally simpler and safer than attempting to reconstruct state
from missed WebSocket events.

---

# 36. Duplicate Events

The client should tolerate duplicate events.

For example:

```text
event A
event A
```

may occur because of retries or reconnect behavior.

Events should therefore be safely processable more than once where practical.

For state events, the client can use:

```text
ride_id
revision
```

to avoid applying an older state over a newer state.

---

# 37. Stale Events

Suppose the client has:

```text
revision = 10
```

and receives:

```text
revision = 9
```

The client should not roll the UI backward.

It should ignore the stale state and retain revision 10.

If a gap is detected:

```text
10 → 12
```

the client should recover authoritative state through REST.

---

# 38. Event Delivery Guarantee

Initial WebSocket delivery is:

```text
at-most-once transport delivery
+
state recovery through REST
```

We are deliberately not building a fully reliable event queue into the
WebSocket protocol.

Why?

Because the authoritative state exists in PostgreSQL.

A missed notification is recoverable.

For important asynchronous backend processing, the outbox mechanism provides
durable event publication.

---

# 39. Event Privacy

Events must be filtered according to authorization.

Example:

```text
Rider
  ↓
may receive all bids for their ride

Driver A
  ↓
may receive their own bid state

Driver A
  ↓
must not receive Driver B's private information
```

The backend determines event recipients.

Clients cannot request arbitrary event streams.

---

# 40. Driver Mode

The Flutter application uses one application for both rider and driver.

The mode switch is a UI concern.

The backend still determines capabilities.

Example:

```text
Flutter
   │
   ├── Rider Mode
   │
   └── Driver Mode
```

Switching the UI mode does not change authentication identity.

The same authenticated User may have:

```text
rider capability
driver capability
```

The server authorizes each operation independently.

---

# 41. Driver Online State

When the driver switches to online mode:

```text
Flutter
   ↓
REST
   ↓
POST /api/v1/me/driver/online
   ↓
PostgreSQL availability state
   ↓
WebSocket presence
```

The UI should not assume that switching the button means the driver is
successfully online.

The server response is authoritative.

---

# 42. Security

The WebSocket layer must enforce:

- Authentication
- Authorization
- Message size limits
- Subscription authorization
- Connection limits
- Rate limiting
- Heartbeat timeout
- Input validation
- Origin policy where applicable
- TLS

A WebSocket connection is not trusted merely because the TCP connection was
successfully established.

---

# 43. Message Size

The server must enforce a maximum WebSocket message size.

Location messages should remain small.

Large payloads should use REST or another appropriate transport.

The exact maximum size will be defined during implementation.

---

# 44. Rate Limits

WebSocket-specific limits should protect against:

- Excessive connection attempts
- Excessive subscriptions
- Excessive messages
- Excessive location updates
- Malformed message floods

Redis may provide distributed rate limiting.

---

# 45. Observability

Every connection should be observable.

Useful metrics include:

```text
active_connections
connections_opened_total
connections_closed_total
authentication_failures_total
messages_received_total
messages_sent_total
message_processing_errors_total
subscription_count
reconnect_count
```

For latency:

```text
event_publish_latency
event_delivery_latency
websocket_message_processing_latency
```

Logs should include:

```text
connection_id
user_id
event_id
event_type
ride_id where applicable
```

Sensitive tokens must never be logged.

---

# 46. Backpressure

A slow client must not be allowed to consume unlimited server memory.

The WebSocket layer should maintain bounded outbound queues.

Conceptually:

```text
Go Backend
    ↓
bounded queue
    ↓
WebSocket
```

If a client cannot consume messages quickly enough:

```text
queue fills
    ↓
connection degraded
    ↓
disconnect/recovery
```

The client can reconnect and recover state through REST.

This is preferable to allowing unbounded memory growth.

---

# 47. Multiple Backend Instances

The architecture must support multiple Go instances.

Example:

```text
                    ┌── Go A ── WebSocket clients
API Gateway ────────┼── Go B ── WebSocket clients
                    └── Go C ── WebSocket clients
                         │
                         ▼
                       Redis
                         │
                         ▼
                    PostgreSQL
```

Redis may be used for:

- Presence
- Connection metadata
- Pub/Sub fan-out
- Operational coordination

PostgreSQL remains authoritative.

---

# 48. Server Restart

If a Go instance restarts:

```text
connections lost
      ↓
clients reconnect
      ↓
subscriptions restored
      ↓
state recovered through REST
```

The server does not need to persist WebSocket connections.

WebSocket connection state is ephemeral.

---

# 49. Event Publication Flow

The preferred flow for business events is:

```text
REST command
    ↓
Go domain/application
    ↓
PostgreSQL transaction
    ├── update business state
    └── insert outbox event
            ↓
         COMMIT
            ↓
      Outbox publisher
            ↓
      Redis Pub/Sub
            ↓
      Go WebSocket instances
            ↓
          clients
```

The event should only be published after the database transaction commits.

This prevents clients from receiving a state change that was ultimately
rolled back.

---

# 50. Initial Event Catalog

| Event | Recipient |
|---|---|
| `connection.ready` | Connected client |
| `ride.bidding_started` | Authorized ride participants |
| `ride.state_changed` | Authorized ride participants |
| `bid.created` | Rider / appropriate participant |
| `bid.updated` | Rider / appropriate participant |
| `bid.withdrawn` | Rider / appropriate participant |
| `bid.selected` | Authorized participants |
| `assignment.confirmation_required` | Selected driver |
| `assignment.confirmed` | Authorized ride participants |
| `assignment.rejected` | Authorized ride participants |
| `assignment.expired` | Authorized ride participants |
| `driver.location_updated` | Authorized rider/participants |
| `driver.arrived` | Authorized ride participants |
| `trip.started` | Authorized ride participants |
| `trip.completed` | Authorized ride participants |

---

# 51. Deferred WebSocket Features

The initial protocol does not define:

- Chat
- Voice/video
- Support messaging
- Push notification integration
- Group rides
- Multi-stop rides
- Scheduled rides
- Admin live dashboards

These can be added later without changing the fundamental REST/WebSocket
boundary.

---

# 52. Design Principles

1. REST performs authoritative business commands.
2. WebSocket delivers real-time information.
3. PostgreSQL remains the source of truth.
4. Redis supports operational real-time infrastructure.
5. Clients must tolerate disconnection.
6. Clients must recover state through REST.
7. Events are not guaranteed to be delivered exactly once.
8. Event IDs are unique and opaque.
9. Resource revisions protect against stale state.
10. Authorization is enforced server-side.
11. WebSocket subscriptions are connection-scoped.
12. Location is ephemeral operational data.
13. Outbox publication occurs after successful database commit.
14. Slow clients must not cause unbounded server memory growth.
15. One Flutter application can support both rider and driver capabilities.
16. Switching UI mode does not change authentication identity.
17. Missed events are recoverable from authoritative state.
