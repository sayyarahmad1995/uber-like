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
11. Ride requests use a driver fare-bidding marketplace rather than
    automatic driver assignment.

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
| Ride matching | Driver fare bidding | Decided |
| Bid visibility | Riders see competing bids; drivers do not | Decided |
| Backend architecture | Modular monolith | Proposed |
| Payments | TBD | Deferred |
| Notifications | TBD | Deferred |
| Message broker | TBD | Deferred |
| Kubernetes | TBD | Deferred |
| Microservices | TBD | Deferred |

## Local Development

The backend can be run locally with Docker Compose. PostgreSQL is started
first, the migration service applies any pending SQL migrations, and the API
starts only after migrations complete successfully.

```bash
docker compose up --build
```

The API exposes:

- `http://localhost:8080/healthz`
- `http://localhost:8080/readyz`

To inspect migration state:

```bash
docker compose logs migrate
docker compose exec postgres psql -U uber_like -d uber_like -c '\\dt'
```

The migration runner records applied versions and SHA-256 checksums in
`schema_migrations`. Existing migration files are never silently re-applied,
and changing an already-applied migration causes the migration step to fail.

## Documentation

Architecture decisions are documented under `docs/`.

Important architectural decisions should be recorded as ADRs rather than
silently changing the architecture.
