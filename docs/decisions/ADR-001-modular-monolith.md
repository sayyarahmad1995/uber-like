# ADR-001: Start With a Modular Monolith

## Status

Accepted

## Context

The platform will initially run on self-managed infrastructure and the
requirements, traffic volume, and operational characteristics are not yet
known.

An Uber-like system contains domains that could eventually become
independent services, particularly dispatch and real-time location.

However, deploying every domain as an independent microservice from the
beginning would introduce unnecessary distributed-system complexity.

## Decision

Start with a modular monolith implemented in Go.

The backend will contain clear logical modules:

```text
auth
users
drivers
vehicles
rides
dispatch
locations
pricing
notifications
```