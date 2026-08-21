# Local Authentication Stack

The development stack runs Ory Kratos and its own PostgreSQL database alongside the application PostgreSQL database.

## Services

- `kratos`: public Kratos API on `http://localhost:4433`
- `postgres-kratos`: private PostgreSQL database used only by Kratos
- `kratos-migrate`: one-shot Kratos schema migration
- `mailpit`: local SMTP capture with web UI on `http://localhost:8025`
- `api`: application API on `http://localhost:8080`
- `postgres`: application PostgreSQL database

Kratos admin API port 4434 is intentionally not published to the host.

## Start

From WSL:

```bash
docker compose up -d --build
```

Check the stack:

```bash
docker compose ps
docker compose logs kratos --tail=100
```

The Kratos public health endpoint can be checked with:

```bash
curl -i http://localhost:4433/health/ready
```

The local email inbox is available at:

```text
http://localhost:8025
```

## Authentication boundary

Flutter owns the product authentication UI. It talks to the Kratos public API using the supported native/headless flow APIs. Kratos owns authentication, credentials, sessions, verification, recovery, and MFA.

The Go API remains responsible for application authorization and local user mapping. It must not trust a user-provided UUID as proof of identity.

## Current development scope

The initial local stack enables password, code, TOTP, verification, recovery, and settings flows. Email is captured by Mailpit rather than delivered externally.

SMS delivery is deliberately not configured yet. Before enabling production SMS, select the gateway and configure Kratos's SMS delivery integration without coupling the gateway to Flutter or the Go application.

## Important security note

The secrets in `deploy/kratos/kratos.yml` are development-only placeholders. They must be replaced through deployment secrets before any non-local deployment.
