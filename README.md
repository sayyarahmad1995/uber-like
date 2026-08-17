# Uber-Like Ride-Hailing Platform

A ride-hailing platform inspired by Uber, consisting of a single Flutter
mobile application supporting both rider and driver modes, and a Go backend.

## Technology Stack

### Mobile
- Flutter
- Single application for riders and drivers
- Rider/Driver mode switching

### Backend
- Go
- PostgreSQL
- Redis
- API Gateway
- WebSocket for real-time communication

### Authentication
- External OIDC provider
- The application does not implement its own authentication server

### Maps
- Google Maps

### Infrastructure
- Self-managed server
- Containerized deployment
- Vertical scaling initially

## Architecture Status

The project is currently in the architecture and design phase.

Some decisions are finalized while others are intentionally deferred until
their requirements become clear.

## Initial Architectural Principles

1. PostgreSQL is the durable source of truth.
2. Redis is used for ephemeral and high-speed data.
3. The backend is initially a modular monolith.
4. Business logic belongs in the backend, not the mobile application.
5. The mobile application is treated as an untrusted client.
6. Real-time communication uses WebSocket where appropriate.
7. External OIDC handles authentication.
8. Google Maps is accessed through an internal abstraction rather than
   coupling the entire application directly to Google APIs.
9. Important ride state transitions are controlled by the backend.
10. Architecture should allow high-load components such as dispatch and
    location processing to be extracted later if necessary.

## Current Decisions

| Area | Decision | Status |
|---|---|---|
| Mobile | Flutter | Decided |
| Mobile applications | One app for rider + driver | Decided |
| Backend | Go | Decided |
| Database | PostgreSQL | Decided |
| Cache / ephemeral state | Redis | Decided |
| Authentication | External OIDC | Decided |
| Maps | Google Maps | Decided |
| API Gateway | Yes | Decided |
| Deployment | Self-managed server | Decided |
| Initial scaling strategy | Vertical | Decided |
| Backend architecture | Modular monolith | Proposed |
| Payments | TBD | Deferred |
| Notifications | TBD | Deferred |
| Message broker | TBD | Deferred |
| Kubernetes | TBD | Deferred |
| Microservices | TBD | Deferred |

## Documentation

Architecture decisions are documented under `docs/`.

Important architectural decisions should be recorded as ADRs rather than
silently changing the architecture.