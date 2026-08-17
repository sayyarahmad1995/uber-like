# Ride Lifecycle

## 1. Purpose

This document defines the lifecycle of a ride and the valid state
transitions.

The backend is authoritative over ride state.

The Flutter application may request a transition but cannot directly set
arbitrary ride states.

---

## 2. Initial States

```text
REQUESTED
SEARCHING_DRIVER
DRIVER_ASSIGNED
DRIVER_ARRIVING
DRIVER_ARRIVED
TRIP_STARTED
TRIP_COMPLETED
CANCELLED
```