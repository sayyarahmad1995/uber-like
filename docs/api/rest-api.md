# REST API

## 1. Purpose

This document defines the HTTP REST API contract for the platform.

It translates the backend use cases into HTTP resources and operations.

This document defines:

- Base URL structure
- Authentication
- Resource naming
- HTTP methods
- Request and response conventions
- Error responses
- Pagination
- Idempotency
- Concurrency behavior
- Ride and bidding endpoints
- Driver and vehicle endpoints

WebSocket communication is defined separately.

The REST API is responsible for commands and authoritative state queries.
Real-time events are delivered through WebSocket.

---

## 2. API Version

The initial API uses:

```text
/api/v1
```

All application endpoints are versioned under this prefix.

Example:

```text
/api/v1/rides
```

A breaking API change requires a new major API version.

---

## 3. Transport

The API uses HTTPS.

```text
Client
   ↓
HTTPS
   ↓
API Gateway
   ↓
Go Backend
```

The API Gateway is responsible for edge concerns such as:

- TLS termination
- Request routing
- Rate limiting where appropriate
- Request size limits
- Basic protection against abusive traffic

Business authorization remains the responsibility of the backend.

---

# 4. Authentication

Authentication is provided by the external OIDC provider.

The client obtains an access token from the OIDC provider and sends it to
the API using:

```http
Authorization: Bearer <access-token>
```

The backend validates the token and establishes the authenticated internal
User.

The backend must not trust user identity information supplied in request
bodies or query parameters.

For example, this is not an authorization mechanism:

```json
{
  "user_id": "some-user-id"
}
```

The authenticated user comes from the validated access token.

---

# 5. Common Headers

Clients should send:

```http
Authorization: Bearer <token>
Content-Type: application/json
Accept: application/json
```

For mutating operations that support idempotency:

```http
Idempotency-Key: <unique-request-key>
```

The exact key format and retention period are implementation details.

---

# 6. Response Conventions

Successful responses use standard HTTP status codes.

Examples:

```text
200 OK
201 Created
202 Accepted
204 No Content
```

The API should return JSON for responses that contain a body.

Example:

```json
{
  "data": {
    "id": "ride_123",
    "status": "BIDDING"
  }
}
```

The API should not expose internal database models directly.

Responses represent API resources and views appropriate to the requesting
actor.

---

# 7. Error Response

Errors use a consistent structure.

Example:

```json
{
  "error": {
    "code": "BIDDING_CLOSED",
    "message": "Bidding for this ride has ended.",
    "details": {}
  }
}
```

### Error fields

| Field | Required | Description |
|---|---|---|
| `code` | Yes | Stable machine-readable error code |
| `message` | Yes | Human-readable explanation |
| `details` | No | Additional structured information |

Clients should use `code` for programmatic behavior rather than parsing
`message`.

---

# 8. HTTP Status Mapping

Initial mapping:

| HTTP status | Meaning |
|---|---|
| `400` | Invalid request |
| `401` | Authentication required/invalid |
| `403` | Authenticated but not authorized |
| `404` | Resource not found or not visible |
| `409` | State/concurrency conflict |
| `422` | Business validation failure |
| `429` | Rate limit exceeded |
| `500` | Unexpected server error |
| `503` | Service temporarily unavailable |

The API must avoid leaking whether a private resource exists when the actor
is not authorized to access it.

---

# 9. User Endpoints

## GET /api/v1/me

**Use case:** `UC-AUTH-002`

Returns the authenticated user's current account information.

### Response

```json
{
  "data": {
    "id": "user_123",
    "display_name": "User",
    "email": "user@example.com",
    "capabilities": {
      "rider": true,
      "driver": true
    },
    "driver_status": "APPROVED"
  }
}
```

Sensitive OIDC information is not returned.

---

## GET /api/v1/me/profile

**Use case:** `UC-USER-001`

Returns the authenticated user's profile.

---

## PATCH /api/v1/me/profile

**Use case:** `UC-USER-002`

Updates fields the authenticated user is allowed to modify.

Example:

```json
{
  "display_name": "New Name"
}
```

Protected identity and authorization fields cannot be modified through this
endpoint.

---

# 10. Driver Endpoints

## POST /api/v1/me/driver-application

**Use case:** `UC-DRIVER-001`

Creates or starts the authenticated user's driver application.

### Response

```text
201 Created
```

```json
{
  "data": {
    "driver_status": "PENDING"
  }
}
```

---

## GET /api/v1/me/driver

**Use case:** `UC-DRIVER-006`

Returns the authenticated user's driver profile.

---

## POST /api/v1/me/driver/online

**Use case:** `UC-DRIVER-007`

Makes the approved driver available for ride bidding.

### Response

```json
{
  "data": {
    "availability": "ONLINE"
  }
}
```

The backend must verify driver and vehicle eligibility before changing
availability.

---

## POST /api/v1/me/driver/offline

**Use case:** `UC-DRIVER-008`

Makes the driver unavailable for new bidding opportunities.

---

# 11. Vehicle Endpoints

## GET /api/v1/me/vehicles

Returns the authenticated driver's vehicles.

---

## POST /api/v1/me/vehicles

**Use case:** `UC-VEHICLE-001`

Creates a vehicle.

Example request:

```json
{
  "make": "Toyota",
  "model": "Corolla",
  "year": 2024,
  "plate_number": "ABC-123",
  "vehicle_type": "ECONOMY"
}
```

---

## PATCH /api/v1/me/vehicles/{vehicle_id}

**Use case:** `UC-VEHICLE-002`

Updates permitted vehicle information.

---

## DELETE /api/v1/me/vehicles/{vehicle_id}

**Use case:** `UC-VEHICLE-003`

Deactivates/removes a vehicle when permitted.

---

## POST /api/v1/me/vehicles/{vehicle_id}/activate

**Use case:** `UC-VEHICLE-004`

Sets an eligible vehicle as the driver's active vehicle.

---

# 12. Ride Endpoints

## POST /api/v1/rides

**Use case:** `UC-RIDE-001`

Creates a ride request.

### Request

```json
{
  "pickup": {
    "latitude": 33.6844,
    "longitude": 73.0479
  },
  "destination": {
    "latitude": 33.7294,
    "longitude": 73.0931
  },
  "service_type": "ECONOMY"
}
```

The backend:

1. Validates the request.
2. Resolves/validates the location information.
3. Calculates the reference fare.
4. Creates the ride.
5. Opens bidding.

### Response

```text
201 Created
```

```json
{
  "data": {
    "id": "ride_123",
    "status": "BIDDING",
    "reference_fare": {
      "amount": 1200,
      "currency": "PKR"
    },
    "bidding": {
      "started_at": "2026-08-18T10:00:00Z",
      "ends_at": "2026-08-18T10:00:30Z"
    }
  }
}
```

The exact fare representation will be finalized with the pricing model.

---

## GET /api/v1/rides/active

**Use case:** `UC-RIDE-002`

Returns the authenticated user's relevant active ride.

A rider receives their active rider-side ride.

A driver receives their active assigned ride when applicable.

---

## GET /api/v1/rides/{ride_id}

**Use case:** `UC-RIDE-003`

Returns a ride the authenticated actor is authorized to view.

The response is actor-aware.

For example, a rider may see competing bids while an unrelated driver must
not.

---

## POST /api/v1/rides/{ride_id}/cancel

**Use case:** `UC-RIDE-004`

Requests ride cancellation.

The backend evaluates:

- Actor authorization
- Current ride state
- Cancellation policy
- Assignment state

### Response

```json
{
  "data": {
    "id": "ride_123",
    "status": "CANCELLED"
  }
}
```

---

## GET /api/v1/rides

**Use case:** `UC-RIDE-005`

Returns the authenticated user's ride history.

### Query parameters

```text
?page=1&page_size=20
```

Cursor pagination may replace page-based pagination if required by the final
implementation.

---

# 13. Bidding Endpoints

## POST /api/v1/rides/{ride_id}/bids

**Use case:** `UC-BID-002`

Submits a driver's bid.

### Request

```json
{
  "amount": 1100
}
```

### Response

```text
201 Created
```

```json
{
  "data": {
    "id": "bid_123",
    "ride_id": "ride_123",
    "amount": 1100,
    "currency": "PKR",
    "status": "ACTIVE"
  }
}
```

The backend verifies all driver and ride eligibility rules.

---

## PATCH /api/v1/rides/{ride_id}/bids/{bid_id}

**Use case:** `UC-BID-003`

Updates the driver's active bid.

### Request

```json
{
  "amount": 1050
}
```

The driver may only modify their own active bid.

The backend revalidates:

- Bid ownership
- Bidding deadline
- Bid limits
- Ride state
- Driver eligibility

---

## DELETE /api/v1/rides/{ride_id}/bids/{bid_id}

**Use case:** `UC-BID-004`

Withdraws the driver's active bid.

The driver may only withdraw their own bid while bidding remains open.

---

## GET /api/v1/rides/{ride_id}/bids

**Use case:** `UC-BID-005`

Returns bids visible to the rider who owns the ride.

### Example response

```json
{
  "data": [
    {
      "id": "bid_123",
      "amount": 1100,
      "currency": "PKR",
      "driver": {
        "id": "driver_123",
        "display_name": "Ahmed",
        "rating": 4.8,
        "completed_rides": 324
      },
      "vehicle": {
        "make": "Toyota",
        "model": "Corolla",
        "vehicle_type": "ECONOMY",
        "plate_number": "ABC-123"
      },
      "estimated_arrival_minutes": 5,
      "status": "ACTIVE"
    }
  ]
}
```

A driver must not be allowed to use this endpoint to inspect competing bids.

Authorization is based on the ride relationship.

---

## POST /api/v1/rides/{ride_id}/selection

**Use case:** `UC-BID-007`

Selects a bid for the rider's ride.

### Request

```json
{
  "bid_id": "bid_123"
}
```

The rider must own the ride.

The backend atomically:

1. Verifies the ride is selectable.
2. Verifies the bid is selectable.
3. Revalidates driver eligibility.
4. Reserves the driver.
5. Establishes the selected assignment.
6. Establishes the agreed fare.
7. Starts the driver confirmation period.

### Response

```json
{
  "data": {
    "ride_id": "ride_123",
    "status": "DRIVER_SELECTED",
    "driver": {
      "id": "driver_123"
    },
    "vehicle": {
      "id": "vehicle_123"
    },
    "agreed_fare": {
      "amount": 1100,
      "currency": "PKR"
    },
    "confirmation_deadline": "2026-08-18T10:01:00Z"
  }
}
```

A successful response means the selection transaction succeeded. It does
not mean the driver has confirmed.

---

# 14. Assignment Endpoints

## POST /api/v1/rides/{ride_id}/assignment/confirm

**Use case:** `UC-BID-008`

Confirms an assignment for the authenticated selected driver.

The backend verifies:

- Driver identity
- Assignment
- Deadline
- Ride state
- Driver eligibility

### Response

```json
{
  "data": {
    "ride_id": "ride_123",
    "status": "DRIVER_CONFIRMED"
  }
}
```

This operation must be idempotent.

A retry after successful confirmation should not create another assignment.

---

## POST /api/v1/rides/{ride_id}/assignment/reject

**Use case:** `UC-BID-009`

Rejects the assignment for the authenticated selected driver.

The backend may attempt fallback selection.

---

# 15. Trip Endpoints

## POST /api/v1/rides/{ride_id}/arrival

**Use case:** `UC-TRIP-001`

The assigned driver reports arrival.

### Result

```text
DRIVER_ARRIVED
```

---

## POST /api/v1/rides/{ride_id}/start

**Use case:** `UC-TRIP-002`

Starts the trip.

### Result

```text
TRIP_STARTED
```

---

## POST /api/v1/rides/{ride_id}/complete

**Use case:** `UC-TRIP-003`

Completes the trip.

### Result

```text
TRIP_COMPLETED
```

Payment settlement is not part of this endpoint's initial responsibility.

---

# 16. Location API

Driver location is primarily a real-time operation and should not be sent
through a conventional REST endpoint for every GPS update.

The primary transport will be WebSocket.

A REST endpoint may still exist for:

- Last-known location
- Recovery after reconnect
- Debugging/administrative purposes
- Initial state retrieval

For example:

```text
GET /api/v1/rides/{ride_id}/location
```

should return the current location only to authorized participants.

Continuous location publishing will be defined by the WebSocket contract.

---

# 17. Pagination

Collection endpoints must support pagination.

Initial candidates:

```text
GET /api/v1/rides?page=1&page_size=20
GET /api/v1/rides/{ride_id}/bids?page=1&page_size=20
```

The server must enforce maximum page size.

For high-volume collections, cursor pagination should be preferred.

Example future form:

```text
?cursor=<opaque-cursor>&limit=20
```

Cursors must be opaque to clients.

---

# 18. Idempotency

Mutating endpoints that can create or transition business state should
support idempotency where retrying the request could otherwise create
duplicate effects.

Initial idempotent candidates:

```text
POST /api/v1/rides
POST /api/v1/rides/{ride_id}/bids
PATCH /api/v1/rides/{ride_id}/bids/{bid_id}
DELETE /api/v1/rides/{ride_id}/bids/{bid_id}
POST /api/v1/rides/{ride_id}/selection
POST /api/v1/rides/{ride_id}/assignment/confirm
POST /api/v1/rides/{ride_id}/assignment/reject
POST /api/v1/rides/{ride_id}/cancel
POST /api/v1/rides/{ride_id}/arrival
POST /api/v1/rides/{ride_id}/start
POST /api/v1/rides/{ride_id}/complete
```

The final idempotency policy will specify:

- Header format
- Key scope
- Retention
- Request fingerprinting
- Behavior for reused keys with different payloads

---

# 19. Concurrency

REST requests may arrive concurrently.

The API must not assume that:

```text
request A
    ↓
request B
```

will be processed in that order.

The following operations are particularly concurrency-sensitive:

- Bid submission
- Bid modification
- Bid selection
- Driver reservation
- Assignment confirmation
- Trip start
- Trip completion

The backend must enforce state transitions atomically.

Example:

```text
Rider A selects Driver X
Rider B selects Driver X

           ↓

      PostgreSQL
        transaction

           ↓

Exactly one selection succeeds.
```

The losing request receives an appropriate conflict/business error.

---

# 20. State Conflicts

A request may be syntactically valid but invalid because the resource has
changed since the client last observed it.

Example:

```text
Flutter sees:
Bid status = ACTIVE

Meanwhile:
Another request selects the driver.

Flutter submits:
Select bid
```

The backend must reject the stale operation rather than applying it blindly.

Possible response:

```text
409 Conflict
```

with:

```json
{
  "error": {
    "code": "BID_NOT_SELECTABLE",
    "message": "This bid is no longer available.",
    "details": {}
  }
}
```

---

# 21. API Security Rules

The API must enforce:

- Authentication on protected endpoints
- Resource-level authorization
- Input validation
- Request size limits
- Rate limiting
- Abuse prevention
- Secure transport
- No trust in client-supplied identity
- No exposure of internal infrastructure errors
- No exposure of private driver information

Authorization must be evaluated on every request.

A valid OIDC token does not automatically grant access to a resource.

---

# 22. Resource Ownership

Initial ownership rules:

| Resource | Rider | Driver | Admin |
|---|---|---|---|
| Own profile | Read/Write | Read/Write | Authorized |
| Own driver profile | — | Read/Write | Authorized |
| Own vehicles | — | Read/Write | Authorized |
| Own ride | Read/Manage | Assigned ride | Authorized |
| Ride bids | View own ride | Own bid only | Authorized |
| Assignment | Rider selects | Selected driver confirms | Authorized |
| Live driver location | Assigned ride | Own location | Authorized |

The API must enforce these rules rather than relying on Flutter UI restrictions.

---

# 23. REST vs WebSocket Boundary

REST is responsible for:

```text
Commands
Queries
Resource state
Historical data
```

WebSocket is responsible for:

```text
Real-time events
Live bidding updates
Assignment notifications
Ride state notifications
Live location
```

Example:

```text
Driver
   │
   │ POST /bids
   ▼
REST API
   │
   ├── PostgreSQL
   │
   └── publish event
           │
           ▼
        WebSocket
           │
           ▼
        Rider
```

The WebSocket event does not replace the REST command.

The REST command changes authoritative state.

---

# 24. Deferred API Areas

The following are intentionally excluded from the initial API:

- Payments
- Driver payouts
- Ratings
- Reviews
- Promotions
- Scheduled rides
- Multi-stop rides
- Corporate accounts
- Subscriptions
- Referral systems
- Customer support

They will receive API contracts when their domain requirements are defined.

---

# 25. Initial Endpoint Summary

| Method | Endpoint | Use Case |
|---|---|---|
| GET | `/api/v1/me` | UC-AUTH-002 |
| GET | `/api/v1/me/profile` | UC-USER-001 |
| PATCH | `/api/v1/me/profile` | UC-USER-002 |
| POST | `/api/v1/me/driver-application` | UC-DRIVER-001 |
| GET | `/api/v1/me/driver` | UC-DRIVER-006 |
| POST | `/api/v1/me/driver/online` | UC-DRIVER-007 |
| POST | `/api/v1/me/driver/offline` | UC-DRIVER-008 |
| GET | `/api/v1/me/vehicles` | Vehicle queries |
| POST | `/api/v1/me/vehicles` | UC-VEHICLE-001 |
| PATCH | `/api/v1/me/vehicles/{vehicle_id}` | UC-VEHICLE-002 |
| DELETE | `/api/v1/me/vehicles/{vehicle_id}` | UC-VEHICLE-003 |
| POST | `/api/v1/me/vehicles/{vehicle_id}/activate` | UC-VEHICLE-004 |
| POST | `/api/v1/rides` | UC-RIDE-001 |
| GET | `/api/v1/rides/active` | UC-RIDE-002 |
| GET | `/api/v1/rides/{ride_id}` | UC-RIDE-003 |
| POST | `/api/v1/rides/{ride_id}/cancel` | UC-RIDE-004 |
| GET | `/api/v1/rides` | UC-RIDE-005 |
| POST | `/api/v1/rides/{ride_id}/bids` | UC-BID-002 |
| PATCH | `/api/v1/rides/{ride_id}/bids/{bid_id}` | UC-BID-003 |
| DELETE | `/api/v1/rides/{ride_id}/bids/{bid_id}` | UC-BID-004 |
| GET | `/api/v1/rides/{ride_id}/bids` | UC-BID-005 |
| POST | `/api/v1/rides/{ride_id}/selection` | UC-BID-007 |
| POST | `/api/v1/rides/{ride_id}/assignment/confirm` | UC-BID-008 |
| POST | `/api/v1/rides/{ride_id}/assignment/reject` | UC-BID-009 |
| POST | `/api/v1/rides/{ride_id}/arrival` | UC-TRIP-001 |
| POST | `/api/v1/rides/{ride_id}/start` | UC-TRIP-002 |
| POST | `/api/v1/rides/{ride_id}/complete` | UC-TRIP-003 |
| GET | `/api/v1/rides/{ride_id}/location` | Location recovery/query |
