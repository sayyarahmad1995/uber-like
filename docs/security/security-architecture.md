# Security Architecture

## 1. Purpose

This document defines authentication, authorization, service trust, secrets,
network security, data protection, abuse controls, auditing, and security testing.

The central principle is:

> Authentication establishes identity; authorization establishes permission; domain rules establish whether an otherwise authorized action is valid.

---

# 2. Security Boundary

```text
                    External OIDC
                         │
                         ▼
                    Access Token
                         │
                         ▼
                  API Authentication
                         │
                         ▼
                    User Identity
                         │
                         ▼
                   Authorization
                         │
                         ▼
                 Application Use Case
                         │
                         ▼
                   Domain Rules
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
         PostgreSQL    Redis    External APIs
```

---

# 3. External OIDC

The application uses an external OIDC provider for authentication.

The project will not implement its own OIDC provider.

The backend validates provider-issued access tokens and maps the external subject
to an internal application user.

```text
OIDC subject
     ↓
users.oidc_subject
     ↓
internal user_id
```

The internal user identifier remains the application's stable business identity.

---

# 4. Client Identity

Never trust a client-supplied user ID as proof of identity.

Bad:

```json
{
  "user_id": "user_123"
}
```

The backend derives identity from the verified access token.

---

# 5. Authentication vs Authorization vs Domain Rules

These are separate checks:

```text
Authentication
→ Who are you?

Authorization
→ Are you allowed to access this resource?

Domain rule
→ Is this operation valid right now?
```

Example:

```text
Driver is authenticated
        ↓
Driver is authorized to operate
        ↓
Driver is eligible for this ride
        ↓
Driver may submit bid
```

Passing one layer does not imply the others.

---

# 6. Resource-Level Authorization

Every protected resource must verify the requester's relationship to it.

For example:

```text
Rider A
   ↓
GET ride B
   ↓
authenticated ✓
authorized ✗
```

Knowing a resource ID is never sufficient to access the resource.

---

# 7. Rider and Driver Capabilities

The Flutter application serves both riders and drivers, but backend authorization
remains explicit.

```text
User
 ├── Rider capabilities
 └── Driver capabilities
```

The client UI is not a security boundary.

Every sensitive operation is authorized server-side.

---

# 8. Driver Eligibility

Being a driver does not automatically mean the driver may receive rides.

Eligibility can depend on:

```text
account status
verification
vehicle status
required documents
operational status
policy restrictions
```

Driver eligibility/discovery owns these business decisions.

---

# 9. OIDC Token Validation

The backend should validate at minimum:

```text
signature
issuer
audience
expiration
not-before where applicable
token type/required claims
```

Do not merely decode a JWT and trust its contents.

---

# 10. JWKS

OIDC signing keys should be obtained through the provider's JWKS mechanism and
cached appropriately.

The backend must support signing-key rotation.

Signing keys must not be hard-coded into application source code.

---

# 11. Token Lifetime

Access tokens should use the identity provider's supported short-lived token
model.

The backend should not create an unnecessary long-lived substitute token merely
to avoid OIDC validation.

---

# 12. Refresh Tokens

Refresh-token handling primarily belongs to the OIDC/client architecture.

The backend should not store refresh tokens unless a concrete architecture
requires it.

If backend-controlled refresh tokens are ever introduced, they require stronger
protection and lifecycle controls.

---

# 13. WebSocket Authentication

WebSockets use the same identity model as the HTTP API.

```text
OIDC access token
       ↓
WebSocket handshake
       ↓
token validation
       ↓
authenticated identity
       ↓
subscription authorization
```

A WebSocket connection does not bypass authorization.

---

# 14. WebSocket Subscription Authorization

Subscriptions must verify access to the requested resource.

For example:

```text
ride:{ride_id}
```

requires authorization to view that ride.

Likewise:

```text
driver:{driver_id}
```

requires authorization to access that driver's operational information.

---

# 15. Service-to-Service Authentication

Internal network placement is not treated as sufficient trust.

Service interactions should have explicit credentials/trust boundaries where
required.

The initial vertically scaled deployment can keep this model simple, but internal
services must not silently assume universal trust.

---

# 16. Secrets

Never commit secrets to Git.

Examples:

```text
OIDC client secrets
database passwords
Redis credentials
payment provider credentials
push provider credentials
encryption keys
API credentials
```

Use injected environment/secrets-management mechanisms.

---

# 17. Configuration vs Secrets

Separate normal configuration from secrets.

Configuration examples:

```text
OIDC issuer URL
database host
Redis host
timeouts
feature flags
```

Secrets examples:

```text
passwords
private keys
API credentials
tokens
```

Do not place secrets in ordinary configuration files or container images.

---

# 18. Database Credentials

The application should use a dedicated PostgreSQL role with only required
permissions.

Do not run the application as PostgreSQL superuser.

Migrations should use a separately privileged role where practical.

---

# 19. Redis Security

Redis should not be exposed publicly.

Restrict Redis network access to trusted application infrastructure.

Authentication and TLS should be used where appropriate to the deployment trust
boundary, but network isolation remains important.

---

# 20. Network Boundary

Production topology is approximately:

```text
Internet
   │
   ▼
Reverse Proxy / TLS
   │
   ▼
Go Application
   ├──── PostgreSQL
   ├──── Redis
   └──── external services
```

PostgreSQL and Redis must not be directly reachable from the public internet.

---

# 21. TLS

Production client traffic uses HTTPS.

WebSockets use secure WebSockets:

```text
wss://
```

Production authentication and payment flows must not use plaintext HTTP.

---

# 22. Sensitive Commands

Every sensitive command requires server-side authorization and domain validation.

Examples:

```text
cancel ride
submit bid
select bid
accept reservation
start trip
complete trip
capture payment
request payout
```

---

# 23. Mass Assignment Protection

Do not bind arbitrary JSON fields directly onto domain/database models.

Bad:

```text
request JSON
   ↓
database model
```

A malicious client could attempt to set fields such as:

```text
status
driver_id
rider_id
payment_status
commission
```

Use explicit request/command DTOs.

---

# 24. Input Validation

Validate at the API boundary:

```text
type
length
format
range
enum values
required fields
```

Then validate business rules in application/domain layers.

Validation does not replace authorization.

---

# 25. SQL Injection

All database access must use parameterized queries.

Never concatenate untrusted request data into SQL.

---

# 26. Output Filtering

Do not return database records wholesale.

Responses should expose only fields intended for the requesting actor.

Internal security flags, provider identifiers, operational notes, and other
sensitive fields should remain internal unless explicitly required.

---

# 27. Location Privacy

Location is sensitive data.

Do not expose arbitrary driver locations, historical driver movement, or unrelated
rider locations.

Location visibility follows ride state and authorization requirements.

---

# 28. Address Privacy

Pickup/dropoff information should only be exposed to actors who need it.

Avoid unnecessarily putting address information into:

```text
logs
events
push payloads
analytics
```

---

# 29. Push Notification Privacy

Push payloads should contain minimal sensitive information.

Prefer:

```text
ride_id
notification type
safe short message
```

rather than complete personal, payment, or location records.

---

# 30. Logging Security

Never log:

```text
access tokens
refresh tokens
passwords
payment credentials
private keys
complete sensitive payloads
```

Use structured logs with enough context for diagnosis without turning logs into a
data-leak source.

---

# 31. Audit Logging

Security-sensitive and important business actions should be auditable.

Examples:

```text
authentication failures
authorization failures
role/status changes
driver eligibility changes
payment actions
payout actions
administrative actions
```

Audit records should be append-oriented and protected from ordinary application
mutation.

---

# 32. Business Audit vs Technical Logs

Technical logs describe system behavior:

```text
request failed
database timeout
WebSocket disconnected
```

Business audit records describe important business facts:

```text
driver eligibility changed
payment captured
payout requested
ride cancelled
```

Business audit records should not disappear merely because normal logs rotate.

---

# 33. Rate Limiting

Rate limiting protects the system from abuse.

Sensitive operations require tighter limits where appropriate:

```text
authentication
ride creation
bid creation
cancellation
payment operations
payout operations
```

Do not rely exclusively on IP limits because legitimate users may share IPs.

---

# 34. Brute-Force Protection

Authentication is primarily delegated to the external OIDC provider.

The application must still protect sensitive endpoints from repeated abuse.

Do not accidentally introduce a second password-authentication system.

---

# 35. Idempotency as Security

Idempotency prevents retries/replays from creating duplicate business effects.

Important examples:

```text
payment capture
payout request
ride creation
```

---

# 36. Replay Protection

Commands that support idempotency should recognize repeated requests.

Where signed/nonce-based mechanisms are required, use established cryptographic
patterns rather than inventing custom schemes.

---

# 37. CSRF

The mobile API primarily uses bearer-token authentication rather than browser
cookies, so traditional cookie-based CSRF is not the primary mobile threat model.

If browser-based administrative interfaces are introduced, they require their own
CSRF protection design.

---

# 38. CORS

Browser-facing endpoints should use restrictive CORS policy.

Do not use wildcard origins with credentialed browser authentication.

The Flutter mobile client itself does not require browser CORS semantics.

---

# 39. File Uploads

Future driver/vehicle/identity document uploads require a dedicated security
design covering:

```text
size limits
content validation
malware scanning where appropriate
private storage
access-controlled retrieval
retention/deletion
```

Uploaded documents should not become public by default.

---

# 40. Encryption at Rest

Sensitive databases and storage should use appropriate encryption-at-rest controls
for the self-managed infrastructure.

Encryption at rest does not replace authorization, network isolation, secret
management, or access control.

---

# 41. Encryption in Transit

Sensitive communication should use encrypted transport:

```text
Flutter ↔ API
API ↔ external services
```

Internal PostgreSQL/Redis TLS should be considered whenever infrastructure crosses
trust boundaries.

---

# 42. Payment Security

Do not store raw payment credentials unless absolutely required and explicitly
architected for the associated compliance obligations.

Prefer provider-hosted/tokenized payment mechanisms.

Store provider identifiers and statuses rather than sensitive payment credentials.

---

# 43. Webhook Security

External webhooks must be cryptographically verified according to the provider's
supported mechanism.

Never trust a payment status merely because a request reached a webhook endpoint.

---

# 44. Webhook Idempotency

External webhook deliveries may be retried.

The same provider event may therefore arrive multiple times.

Persist provider event IDs where appropriate and make processing idempotent.

---

# 45. External API Trust

Treat external service responses as untrusted input.

Validate relevant:

```text
status
schema
IDs
amounts
currency
signatures where applicable
```

before applying business effects.

---

# 46. SSRF Protection

If the application ever fetches remote resources based on user-supplied URLs, it
must defend against SSRF.

Do not permit arbitrary server-side HTTP requests based on untrusted URLs.

---

# 47. Administrative Access

Administrative functionality must not be mixed casually with rider/driver
permissions.

Use explicit administrative roles and authorization policies.

Potential roles include:

```text
support
operations
finance
security/admin
```

The exact role matrix is a later authorization-policy decision.

---

# 48. Least Privilege

Every component should receive only the permissions it needs.

Examples:

```text
API → application database role
migration → migration role
analytics → read-only access where possible
worker → only required resources
```

Do not give every component unrestricted database access.

---

# 49. Security Boundaries in Go

Centralize transport/security concerns such as:

```text
authentication middleware
authorization policy evaluation
request validation
security headers
audit logging
secret/config loading
```

But keep resource-specific/domain authorization inside application use cases where
business context is required.

For example:

```text
middleware
→ authenticated user

use case
→ user may cancel this specific ride
```

---

# 50. Dependency Security

Go dependencies should be:

```text
versioned
reviewed
updated regularly
```

Avoid unnecessary dependencies because each dependency increases attack surface
and maintenance burden.

---

# 51. Container Security

Production containers should, where practical:

```text
run as non-root
use minimal images
avoid unnecessary packages
keep secrets out of images
use reviewed dependency versions
```

Never bake credentials into Docker images.

---

# 52. Supply-Chain Security

The build process should eventually include:

```text
dependency vulnerability scanning
container image scanning
secret scanning
static analysis
```

The objective is to catch dependency and credential problems before production.

---

# 53. Reverse Proxy Security

The reverse proxy should provide appropriate:

```text
TLS termination
security headers
request limits
connection limits
```

Exact deployment configuration belongs in deployment documentation.

---

# 54. Data Minimization

Only collect and retain information the product actually needs.

This is especially important for:

```text
location
identity documents
addresses
payment information
device information
audit data
```

More data creates more security responsibility.

---

# 55. Narrow Data Access

Avoid generic endpoints such as:

```text
GET /users/{id}/everything
```

Expose narrowly defined resource/query contracts instead.

---

# 56. Fail Closed

Security failures must fail closed.

Examples:

```text
invalid authentication → reject
invalid authorization → reject
invalid webhook signature → reject
```

Never fall back to permissive behavior because a security dependency is unavailable.

---

# 57. Availability vs Security

Dependency failure must not weaken authorization.

For example:

```text
OIDC/JWKS unavailable
```

must not become:

```text
accept unverified JWT
```

Likewise:

```text
Redis unavailable
```

must not become:

```text
skip authorization
```

Availability problems must fail safely.

---

# 58. Security Testing

Security tests should include:

```text
invalid tokens
expired tokens
wrong issuer
wrong audience
missing claims
cross-user resource access
cross-driver resource access
role escalation
mass assignment
SQL injection attempts
rate-limit bypass
duplicate payment commands
invalid webhooks
replay attempts
```

---

# 59. Threat Model

Before production exposure, explicitly threat-model:

```text
account takeover
API abuse
driver impersonation
resource enumeration
payment manipulation
location leakage
privilege escalation
webhook forgery
credential theft
database compromise
Redis compromise
```

Threat modeling should focus on realistic attack paths rather than only generic
checklists.

---

# 60. What We Should Not Build Yet

Do not build:

```text
custom identity provider
custom cryptography
service mesh
full zero-trust platform
complex secrets platform
multi-region security architecture
full SIEM stack
custom WAF
```

These may become appropriate later, but they are not prerequisites for V1.

---

# 61. Complete Security Architecture

```text
                         OIDC Provider
                              │
                              ▼
                         Access Token
                              │
                              ▼
                       API Authentication
                              │
                              ▼
                         User Identity
                              │
                              ▼
                         Authorization
                              │
                              ▼
                     Application Use Case
                              │
                              ▼
                         Domain Rules
                              │
                 ┌────────────┼────────────┐
                 ▼            ▼            ▼
            PostgreSQL      Redis      External APIs
                 │                         │
                 ▼                         ▼
              Outbox                 signed/verified
                 │                    integrations
                 ▼
         Realtime / Notifications
```

---

# 62. Design Principles

1. External OIDC is the authentication authority.
2. The backend derives identity from verified tokens.
3. Authentication, authorization, and domain eligibility are separate decisions.
4. Resource ownership is checked server-side.
5. The Flutter client is never the security boundary.
6. WebSocket connections use the same identity and authorization model.
7. OIDC signatures, issuer, audience, and token lifetime must be validated.
8. JWKS keys are cached and rotated through the provider mechanism.
9. Secrets never enter Git or container images.
10. PostgreSQL and Redis are not publicly exposed.
11. Production traffic uses encrypted transport.
12. Sensitive commands require explicit authorization and domain validation.
13. Request DTOs prevent mass assignment.
14. SQL uses parameterized queries.
15. Sensitive location/address/payment data is minimized and access-controlled.
16. Security logs never contain credentials or unnecessary sensitive payloads.
17. Security-sensitive business actions require auditability.
18. Rate limiting and idempotency protect important operations.
19. External webhooks are cryptographically verified and idempotent.
20. External responses are treated as untrusted input.
21. Administrative access uses explicit privileges.
22. Components receive least-privilege access.
23. Containers and dependencies receive supply-chain security controls.
24. Security failures fail closed.
25. Availability failures must never silently weaken authorization.
26. Security testing covers identity, authorization, injection, abuse, replay, and webhook threats.
27. Avoid building enterprise security infrastructure before actual requirements justify it.
