# Domain Model

## 1. Purpose

This document defines the initial business entities of the ride-hailing
platform.

The model is intentionally minimal. New entities should be introduced only
when a real business requirement requires them.

---

## 2. User

A User represents the application's primary identity.

A user may have rider capabilities, driver capabilities, or both.

```text
User
 ├── Rider capability
 └── Driver capability
       │
       └── Driver
             │
             └── Vehicle(s)
```