# System Architecture

## 1. Purpose

This document describes the high-level architecture of the ride-hailing
platform.

The system supports two user experiences through a single Flutter application:

- Rider mode
- Driver mode

The backend is responsible for authentication validation, business rules,
ride lifecycle management, dispatch, real-time location handling, and
persistent data.

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
                         │ Authentication       │
                         │ Rate Limiting        │
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