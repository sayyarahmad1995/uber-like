# System Architecture

## 1. Purpose

This document defines the high-level architecture of the ride-hailing platform.

The platform uses a single Flutter application for both riders and drivers.
The backend is responsible for authentication validation, business rules,
ride lifecycle management, driver eligibility, bidding, real-time location,
and durable business state.

The architecture is intentionally designed to start simple while preserving
clear boundaries for future extraction of high-load components.

---

## 2. High-Level Architecture

```text
                         ┌──────────────────────┐
                         │     Flutter App      │
                         │                      │
                         │   Rider / Driver     │
                         │                      │
                         │   Google Maps SDK    │
                         └──────────┬───────────┘
                                    │
                              HTTPS / WSS
                                    │
                         ┌──────────▼───────────┐
                         │     API Gateway      │
                         │                      │
                         │ TLS                  │
                         │ Auth verification    │
                         │ Rate limiting        │
                         │ Routing              │
                         └──────────┬───────────┘
                                    │
                         ┌──────────▼───────────┐
                         │      Go Backend      │
                         │                       │
                         │ Auth                  │
                         │ Users                 │
                         │ Drivers               │
                         │ Vehicles              │
                         │ Rides                 │
                         │ Dispatch              │
                         │ Bidding               │
                         │ Locations             │
                         │ Pricing               │
                         │ Notifications         │
                         └───────┬───────┬───────┘
                                 │       │
                       ┌─────────▼───┐ ┌─▼──────────┐
                       │ PostgreSQL  │ │   Redis    │
                       │             │ │            │
                       │ Durable     │ │ Ephemeral  │
                       │ state       │ │ / realtime │
                       └─────────────┘ └────────────┘

                    ┌────────────────────────────┐
                    │       External Systems      │
                    │                            │
                    │ OIDC Provider              │
                    │ Google Maps                │
                    └────────────────────────────┘
```

---

## 3. Architectural Style

The initial backend will use a **modular monolith**.

The backend contains logical modules within one Go deployment:

```text
internal/
├── auth/
├── users/
├── drivers/
├── vehicles/
├── rides/
├── dispatch/
├── bidding/
├── locations/
├── pricing/
└── notifications/
```

These are domain boundaries, not independent deployments.

The architecture should allow high-load or independently evolving modules,
particularly bidding, dispatch, and location processing, to be extracted later
if actual operational requirements justify doing so.

Microservices will not be introduced merely because the system can be divided
into services conceptually.

---

## 4. Client Responsibilities

The Flutter application is responsible for:

- User interface and navigation
- Rider/driver mode presentation
- Collecting user input
- Displaying maps and location information
- Sending commands to the backend
- Maintaining the real-time connection
- Presenting ride and bid information
- Local transient state

The client is **not authoritative** for:

- Authentication decisions
- Authorization
- Driver eligibility
- Ride state
- Bid validity
- Fare agreement
- Driver assignment
- Current driver availability

A malicious client must not be able to bypass these rules by constructing
requests manually.

---

## 5. API Gateway

The API Gateway is the public entry point to the backend.

Responsibilities include:

- TLS termination
- Authentication/token verification as appropriate
- Rate limiting
- Request identification
- Routing
- WebSocket upgrade/forwarding
- Basic request-size and protocol protection

The gateway should not contain core business logic.

The gateway must not decide:

- Which driver wins a bid
- Whether a ride may transition state
- Whether a driver is eligible for a ride
- What fare a ride ultimately agrees to

Those decisions belong to backend domain modules.

---

## 6. Authentication

Authentication is delegated to an external OIDC provider.

The application does not implement its own identity provider or password
authentication system.

General flow:

```text
Flutter
   │
   ▼
OIDC Provider
   │
   ▼
Access / Identity Token
   │
   ▼
API Gateway / Backend
   │
   ▼
Application User
```

The application maintains its own internal user identity associated with the
external OIDC subject.

---

## 7. Domain Modules

### Users

Owns application-level user identity and profile information.

### Drivers

Owns driver profile, approval state, operational eligibility, and driver
availability.

### Vehicles

Owns driver vehicle information and vehicle eligibility.

### Rides

Owns the ride aggregate and authoritative ride lifecycle.

### Dispatch

Determines which drivers are eligible to participate in a ride opportunity.
Dispatch does not directly choose the final driver merely because a driver is
nearby.

### Bidding

Owns the bidding marketplace for a ride:

- opening and closing the bidding window
- validating bids
- accepting bid changes and withdrawals
- tracking bid state
- exposing valid bids to the rider
- processing rider bid selection
- coordinating driver confirmation

### Locations

Handles real-time driver location and related operational location state.

### Pricing

Calculates the reference fare and applicable bidding boundaries. The exact
pricing algorithm is intentionally deferred.

### Notifications

Handles delivery of user-facing notifications. The concrete provider is
intentionally deferred.

---

## 8. Ride and Bidding Boundary

The Ride module owns the durable ride lifecycle.

The Bidding module owns the bidding process associated with a ride.

Conceptually:

```text
Ride
 │
 └── Bidding
      ├── eligible drivers
      ├── bids
      ├── bidding deadline
      └── driver selection/confirmation
```

A driver submitting a bid does not automatically become the ride's driver.

The rider selects a bid, after which the backend establishes the driver and
agreed fare atomically.

---

## 9. Dispatch vs Bidding

Dispatch and bidding have different responsibilities.

```text
Dispatch
    │
    └── Which drivers may participate?

Bidding
    │
    ├── Which bids are valid?
    ├── What bids are visible to the rider?
    ├── Which bid did the rider select?
    └── Did the selected driver confirm?
```

This is deliberately different from a traditional nearest-driver dispatch
model.

The platform calculates a reference fare and eligible drivers compete by
submitting their own fare bids within platform-defined limits.

---

## 10. PostgreSQL

PostgreSQL is the durable source of truth for business state.

It will contain information such as:

- Users
- Drivers
- Vehicles
- Rides
- Bids
- Ride events
- Durable configuration
- Historical business records

Important ride and bidding decisions must be persisted durably.

---

## 11. Redis

Redis is intended for fast and ephemeral operational data.

Potential uses include:

- Current driver locations
- Nearby-driver lookup
- Online driver state
- Short-lived bidding/dispatch coordination
- Distributed locks
- Rate limiting
- Caching
- Real-time coordination

Redis must not become the authoritative source of important business state.

If Redis data is lost, durable ride and bid state must remain recoverable from
PostgreSQL.

---

## 12. Real-Time Communication

WebSocket will be used where low-latency server-to-client communication is
required.

Potential rider events include:

- New bid available
- Bid changed
- Bid withdrawn
- Bidding closed
- Driver selected
- Driver confirmed
- Driver location updated
- Driver arrived
- Trip started
- Trip completed
- Ride cancelled

Potential driver events include:

- Ride opportunity available
- Bid status changed
- Driver selected
- Confirmation required
- Ride cancelled
- Trip state changed

The exact event contract will be defined separately.

---

## 13. Maps

Google Maps is the initial maps provider.

The backend should access Google services through an internal maps abstraction
where practical.

Conceptually:

```text
MapsProvider
    │
    ├── geocode()
    ├── reverseGeocode()
    ├── route()
    └── distance()
```

This prevents the core domain from becoming tightly coupled to Google-specific
API details.

---

## 14. Infrastructure

The initial deployment will use self-managed infrastructure and containers.

Vertical scaling is the initial scaling strategy.

Kubernetes, distributed service deployment, and more advanced orchestration
are intentionally deferred until actual requirements justify them.

---

## 15. Core Architectural Principles

1. PostgreSQL is the durable source of truth.
2. Redis is fast and ephemeral operational state.
3. The backend is authoritative for business rules and state transitions.
4. The mobile client is untrusted.
5. User identity is separate from rider/driver operating mode.
6. Ride lifecycle is separate from internal dispatch/bidding activity.
7. A bid does not equal a driver assignment.
8. The final driver and agreed fare must be established atomically.
9. Real-time connectivity is an operational mechanism, not a business-state
   transition by itself.
10. Architecture should evolve from measured requirements rather than assumed
    scale.

---

## 16. Decision Status

### Decided

- Flutter single application
- Go backend
- PostgreSQL
- Redis
- API Gateway
- External OIDC
- Google Maps
- Self-managed infrastructure
- Containerized deployment
- Vertical scaling initially
- Modular monolith initially
- Driver fare bidding marketplace

### Deferred

- Payment provider
- Notification provider
- Exact pricing algorithm
- Exact bid bounds
- Exact bidding duration
- Exact driver confirmation timeout
- Message broker
- Kubernetes
- Service extraction strategy
- Advanced administrative system
