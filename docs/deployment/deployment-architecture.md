# Deployment Architecture

## 1. Purpose

This document defines the production deployment topology, containers, networking,
configuration, secrets, health checks, deployment process, backups, observability,
failure handling, disaster recovery, and scaling strategy.

The deployment is intentionally simple:

```text
self-managed server
+ containerized workloads
+ vertical scaling
```

The system should not adopt distributed infrastructure merely because it is common
at much larger scale.

---

# 2. Production Topology

```text
                    Internet
                       │
                       ▼
                ┌─────────────┐
                │ Reverse     │
                │ Proxy / TLS │
                └──────┬──────┘
                       │
                       ▼
                ┌─────────────┐
                │ Go API/App  │
                │ Container   │
                └──────┬──────┘
                       │
              ┌────────┴────────┐
              ▼                 ▼
        ┌───────────┐      ┌───────────┐
        │ PostgreSQL│      │   Redis   │
        │ Container │      │ Container │
        └───────────┘      └───────────┘
```

Only the reverse proxy is publicly exposed.

---

# 3. Initial Server Strategy

The initial production deployment uses one self-managed server.

The server should have sufficient:

```text
CPU
RAM
NVMe/storage
network capacity
```

for the expected workload and operational overhead.

A second server is introduced only when a concrete availability or capacity
requirement justifies it.

---

# 4. Container Boundaries

Initial containers should be separated by responsibility:

```text
reverse-proxy
application
postgresql
redis
```

Background workers may initially run from the same Go application image with a
separate process/entrypoint if that simplifies deployment, provided their resource
and failure behavior remain controlled.

---

# 5. Containerization

Docker is the containerization boundary.

Images should be:

```text
minimal
reproducible
versioned
scannable
```

Do not install production dependencies manually on the host when they belong inside
application containers.

---

# 6. Reverse Proxy

The reverse proxy terminates public TLS and forwards traffic to the application.

Responsibilities include:

```text
TLS termination
HTTP routing
WebSocket upgrade
connection limits
request size limits
security headers
basic access logging
```

The exact reverse-proxy product is an implementation decision.

---

# 7. Public Ports

The public interface should expose only required application ports.

Conceptually:

```text
443 → HTTPS / WebSocket
80  → redirect to HTTPS where appropriate
```

PostgreSQL and Redis ports must not be publicly exposed.

---

# 8. Internal Networking

Containers communicate through a private container network.

Conceptually:

```text
reverse-proxy
      │
      ▼
application
   ┌──┴──┐
   ▼     ▼
postgres redis
```

The application should use internal service names rather than public IP addresses
for database/cache connections.

---

# 9. TLS

Production client traffic uses HTTPS.

WebSockets use:

```text
wss://
```

TLS certificates should be renewed automatically where practical.

Certificate expiry must be monitored.

---

# 10. PostgreSQL Deployment

PostgreSQL runs as a persistent container with host-backed storage.

The database is authoritative for durable business state.

The PostgreSQL data directory must not be treated as disposable container storage.

---

# 11. PostgreSQL Storage

Use reliable local persistent storage, preferably fast NVMe where available.

Storage capacity must be monitored.

A full database disk is a production incident, not merely a performance problem.

---

# 12. Redis Deployment

Redis runs as a private container.

Redis stores ephemeral/derived operational state as defined by the database and
Redis architecture.

Redis loss must not corrupt durable business state.

---

# 13. Redis Persistence

Redis persistence is secondary to PostgreSQL durability.

Whether to enable AOF/RDB persistence depends on which ephemeral workloads benefit
from restart recovery.

The architecture must never depend on Redis persistence for authoritative business
history.

---

# 14. Persistent Volumes

Persistent storage should be explicit.

At minimum:

```text
PostgreSQL data
backup staging where required
```

Application containers should remain replaceable.

---

# 15. Configuration

Configuration should be injected at runtime.

Examples:

```text
OIDC issuer URL
database host
Redis host
request timeouts
worker concurrency
feature flags
```

Do not hard-code environment-specific production configuration into application
source code.

---

# 16. Secrets

Secrets are injected at deployment time.

Examples:

```text
OIDC client secrets
database passwords
Redis credentials
payment provider credentials
push provider credentials
TLS private keys
```

Secrets must not be committed to Git or baked into container images.

---

# 17. Environment Separation

At minimum distinguish:

```text
development
staging
production
```

Production credentials and data must never be reused in development.

If staging is not initially affordable, a production-like local/staging workflow
should still exist before major production releases.

---

# 18. Health Checks

Containers should expose health/readiness information appropriate to their role.

Application health should distinguish:

```text
process alive
ready to serve traffic
critical dependencies available
```

Do not make a simple process-liveness check depend on every external service.

---

# 19. Application Readiness

Readiness should verify the dependencies required to safely serve requests.

Conceptually:

```text
application
   ├── PostgreSQL reachable
   └── required local dependencies ready
```

A temporary external API outage should not necessarily make the entire application
unhealthy if unrelated requests can continue safely.

---

# 20. Graceful Shutdown

The Go application must handle termination signals and gracefully stop accepting
new work.

Shutdown should allow active requests and workers to finish within a bounded grace
period.

WebSocket connections should close cleanly and clients should reconnect.

---

# 21. Database Migrations

Database migrations are an explicit deployment step.

Conceptually:

```text
build image
    ↓
backup/checkpoint as required
    ↓
run migration
    ↓
start/replace application
    ↓
health checks
```

Destructive schema changes require a deliberate migration strategy.

---

# 22. Backward-Compatible Migrations

Prefer expand/contract migrations for production changes that could overlap with
old and new application versions.

Example:

```text
add new column
      ↓
application starts using it
      ↓
backfill
      ↓
remove old column later
```

Do not combine incompatible schema deletion with an application rollout unless the
deployment guarantees no old application process remains.

---

# 23. CI Pipeline

The CI pipeline should perform at least:

```text
go test
go vet/static analysis
format checks
dependency checks
secret scanning
container build
container vulnerability scanning where available
```

The exact CI platform is an implementation decision.

---

# 24. Image Management

Production should deploy immutable image versions.

Prefer:

```text
uber-like-api:<git-sha>
```

rather than relying only on mutable tags such as:

```text
latest
```

This makes rollback and debugging deterministic.

---

# 25. Deployment Process

A normal deployment should be:

```text
commit
  ↓
CI
  ↓
image build
  ↓
security/tests
  ↓
publish image
  ↓
production pull
  ↓
migration
  ↓
application restart/recreate
  ↓
health checks
  ↓
traffic
```

---

# 26. Deployment Verification

After deployment verify:

```text
application health
API endpoints
WebSocket connectivity
PostgreSQL connectivity
Redis connectivity
background workers
outbox processing
critical business flows
error rates
```

A successful container start is not sufficient evidence of a successful deployment.

---

# 27. Rollback

Rollback must be possible for application images.

Conceptually:

```text
bad image
   ↓
previous known-good image
```

Database rollback is harder and should not depend on reversing arbitrary migrations.

Prefer forward-compatible migrations and corrective migrations.

---

# 28. Logs

Application logs should be structured.

Useful fields include:

```text
time
level
service
request_id
route
status
duration
error code
aggregate IDs where appropriate
```

Avoid logging credentials or unnecessary sensitive data.

---

# 29. Log Storage

Logs must survive enough of the application container lifecycle to support
incident investigation.

Container logs should be collected/rotated with bounded disk usage.

Never allow logs to fill the production disk.

---

# 30. Metrics

Monitor at least:

```text
CPU
memory
disk usage
disk I/O
network
HTTP request rate
HTTP latency
HTTP error rate
WebSocket connections
PostgreSQL health
PostgreSQL connections
Redis health
Redis memory
outbox backlog
worker failures
backup status
```

---

# 31. Application Metrics

Useful application metrics include:

```text
http_requests_total
http_request_duration
http_request_errors
websocket_connections
outbox_pending_events
outbox_processing_failures
background_job_failures
```

Avoid high-cardinality IDs as metric labels.

---

# 32. Database Monitoring

Monitor:

```text
connection count
query latency
locks
transaction failures
disk usage
WAL growth
checkpoint behavior
slow queries
backup health
```

Database performance problems should be diagnosed from actual measurements rather
than speculation.

---

# 33. Redis Monitoring

Monitor:

```text
memory usage
connected clients
command latency
evictions
key expiration behavior
availability
```

Redis memory exhaustion should produce an explicit operational alert.

---

# 34. Alerting

Alerts should focus on conditions requiring action.

Examples:

```text
PostgreSQL unavailable
disk nearly full
backup failed
outbox backlog growing
high API 5xx rate
high latency
Redis unavailable
TLS certificate approaching expiry
container repeatedly restarting
```

Do not create alerts for every low-level metric fluctuation.

---

# 35. Backups

PostgreSQL backups are mandatory.

The backup strategy should eventually define:

```text
frequency
retention
off-server storage
encryption
restore testing
point-in-time recovery
```

Backups must not exist only on the production server.

---

# 36. Restore Testing

A backup that has never been restored is an assumption, not a recovery strategy.

Regularly test restoration into an isolated environment.

Verify:

```text
backup integrity
schema
business data
application startup
critical queries
```

---

# 37. Disaster Recovery

At minimum define:

```text
backup location
server replacement procedure
application image recovery
configuration recovery
secret recovery
PostgreSQL restore
DNS/TLS recovery
```

Recovery procedures should be documented rather than kept only in the operator's
memory.

---

# 38. Recovery Priorities

A reasonable recovery order is:

```text
1. infrastructure/server
2. persistent storage
3. PostgreSQL
4. Redis
5. application
6. workers
7. reverse proxy
8. traffic
```

Redis can be reconstructed after PostgreSQL because it is not authoritative.

---

# 39. Vertical Scaling

Vertical scaling is the primary scaling strategy.

Scale in this order based on observed bottlenecks:

```text
CPU
RAM
NVMe/storage
network capacity
```

Do not scale components blindly.

---

# 40. Capacity Planning

Measure before scaling.

Track:

```text
requests/sec
peak concurrent connections
average/p95/p99 latency
PostgreSQL CPU/memory
PostgreSQL I/O
Redis memory
outbox throughput
worker throughput
```

Capacity decisions should use measured production-like workloads.

---

# 41. Application Resource Limits

Containers should have sensible CPU and memory expectations.

Resource limits prevent one workload from consuming the entire server.

The exact values are deployment-specific and should be established through load
testing.

---

# 42. Database Resource Planning

PostgreSQL requires enough:

```text
RAM
CPU
storage IOPS
connection capacity
```

Connection pooling should be used appropriately.

Do not create one PostgreSQL connection per request without bounds.

---

# 43. Redis Resource Planning

Redis is memory-sensitive.

Plan explicitly for:

```text
working set
TTL behavior
maximum memory
eviction policy
connection count
```

Do not let cache growth consume all host memory.

---

# 44. Worker Resource Planning

Background workers must not starve the API.

For example:

```text
outbox worker
notification worker
analytics worker
```

should use bounded concurrency.

If worker load grows significantly, separate worker processes/containers can be
introduced without changing the domain architecture.

---

# 45. Failure: Application Container

If the application crashes:

```text
container restarts
health checks run
traffic resumes when ready
```

Durable state remains in PostgreSQL.

In-flight asynchronous work must be recoverable through the database/outbox design.

---

# 46. Failure: Redis

If Redis fails:

```text
cache misses
presence/location degradation
possible temporary realtime limitations
```

But:

```text
rides
bids
assignments
payments
settlements
```

remain durable in PostgreSQL.

---

# 47. Failure: PostgreSQL

If PostgreSQL fails, durable commands should fail safely.

The application must not pretend business state changed merely because Redis or
another dependency accepted a request.

---

# 48. Failure: External Dependency

External dependency failures should be isolated through:

```text
timeouts
bounded retries
circuit/availability policy where justified
idempotency
asynchronous processing where appropriate
```

Do not allow one slow external provider to consume all application workers.

---

# 49. Server Failure

A single-server deployment has a real availability limitation:

```text
server dies
   ↓
application unavailable
```

Backups and documented recovery reduce data-loss risk but do not create high
availability.

This limitation is accepted for the initial deployment.

---

# 50. Future Horizontal Scaling

The architecture should avoid blocking future horizontal scaling.

The application should not rely on:

```text
local process memory as durable state
local filesystem as shared state
single-process timers for business correctness
```

Redis/PostgreSQL/outbox provide shared durable or operational coordination where
needed.

---

# 51. Future Multi-Node Application

If load later requires multiple Go application instances:

```text
                 Reverse Proxy
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
       Go App 1    Go App 2    Go App 3
          │           │           │
          └───────────┼───────────┘
                      ▼
               PostgreSQL/Redis
```

The application architecture should continue to work without major domain changes.

---

# 52. WebSocket Scaling

A future multi-node deployment requires connection-aware routing and/or shared
realtime coordination.

The initial single-node deployment avoids this complexity.

Redis may eventually support cross-node realtime coordination if needed.

---

# 53. Deployment Security

Production deployment should include:

```text
firewall
SSH key-based administration
restricted management ports
regular OS security updates
container image updates
secret protection
TLS
least-privilege database roles
```

Do not expose administrative services to the public internet unnecessarily.

---

# 54. Host Security

The self-managed server must be treated as production infrastructure.

At minimum:

```text
minimal installed services
firewall
patched OS
restricted SSH
monitoring
backup access
separate operator credentials
```

The exact hardening checklist belongs in operational runbooks.

---

# 55. Time Synchronization

Production infrastructure should maintain accurate system time.

Time correctness matters for:

```text
OIDC token validation
TLS
logs
metrics
scheduled jobs
event timestamps
financial workflows
```

Use a reliable time synchronization mechanism on the host.

---

# 56. Deployment Runbooks

Document runbooks for:

```text
normal deployment
rollback
database migration failure
PostgreSQL recovery
Redis recovery
backup restore
TLS renewal
server replacement
outbox backlog
high API error rate
full disk
```

The goal is to make incident response executable rather than dependent on memory.

---

# 57. What We Should Not Build Yet

Do not build:

```text
Kubernetes
service mesh
multi-region deployment
active-active database
PostgreSQL sharding
managed cloud migration
complex autoscaling platform
multi-cluster Redis
elaborate deployment orchestrator
```

These may become appropriate later, but they are not justified by the current
self-managed vertically scaled architecture.

---

# 58. Complete Deployment Architecture

```text
                    Internet
                       │
                       ▼
                ┌─────────────┐
                │ Reverse     │
                │ Proxy / TLS │
                └──────┬──────┘
                       │
                       ▼
              ┌─────────────────┐
              │ Go Application  │
              │ + Workers       │
              └───────┬─────────┘
                      │
             ┌────────┴────────┐
             ▼                 ▼
       ┌───────────┐      ┌───────────┐
       │ PostgreSQL│      │   Redis   │
       │ persistent│      │ ephemeral │
       └─────┬─────┘      └───────────┘
             │
             ▼
          Backups
             │
             ▼
       Separate Storage
```

---

# 59. Design Principles

1. The initial production deployment uses one self-managed server.
2. Workloads are containerized.
3. The reverse proxy is the only public application entry point.
4. PostgreSQL and Redis remain private.
5. PostgreSQL uses persistent host-backed storage.
6. Redis remains operational/ephemeral rather than authoritative.
7. Production secrets are injected at deployment time.
8. Secrets never enter Git or container images.
9. Development, staging, and production environments are separated.
10. Health and readiness checks are explicit.
11. Go processes shut down gracefully.
12. Database migrations are controlled deployment steps.
13. Production images are immutable/versioned.
14. CI runs tests and security checks before deployment.
15. Deployment verification checks the running system, not just container startup.
16. Application rollback is supported; database rollback favors forward-compatible migrations.
17. Logs are structured and bounded.
18. Metrics cover application, database, Redis, infrastructure, workers, and backups.
19. Alerts focus on actionable failures.
20. PostgreSQL backups are stored separately from the production server.
21. Backups are periodically restored and tested.
22. Disaster recovery procedures are documented.
23. Vertical scaling is the first scaling strategy.
24. Capacity decisions are based on measured workload.
25. Worker concurrency is bounded.
26. Redis failure must not corrupt durable business state.
27. PostgreSQL failure must fail durable operations safely.
28. External dependency failures use timeouts and bounded recovery mechanisms.
29. The single-server availability limitation is explicitly accepted for V1.
30. The application avoids local-state assumptions that would prevent future horizontal scaling.
31. Kubernetes, multi-region, sharding, and service meshes are deferred until justified by actual requirements.
