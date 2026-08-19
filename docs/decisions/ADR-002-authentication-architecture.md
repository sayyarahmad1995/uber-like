# ADR-002: Authentication Architecture

- Status: Accepted
- Date: 2026-08-19

## Context

The mobile application needs authentication that is customizable without coupling the Go backend to a proprietary authentication service. The project also requires an external OIDC identity system, while the Flutter application should provide the product-specific authentication experience.

The application must be able to change SMS and email delivery providers without changing the Flutter client or Go business logic. Authentication must remain separate from application authorization.

## Decision

Use a self-hosted, OIDC-compliant identity provider as the authentication system. Do not implement passwords, OTP generation, session management, MFA, token signing, or token issuance in the Go API.

Flutter owns the authentication user experience. The identity provider owns authentication and OIDC token issuance. The Go API validates authenticated identity and owns application authorization and business rules.

The concrete identity provider is intentionally not selected by this ADR. It must satisfy the requirements below before adoption.

## Responsibilities

### Flutter

- Provide the product authentication UI.
- Collect user input through the supported OIDC authentication flow.
- Initiate authorization using OAuth 2.0/OIDC with PKCE for the mobile public client.
- Store tokens using platform-secure storage.
- Refresh tokens according to the provider's supported flow.
- Never treat a user-supplied identifier as authenticated identity without a valid provider-issued token.

### Identity provider

- User authentication.
- OTP and MFA flows.
- Password/passkey authentication where enabled.
- Email and phone verification.
- Session and credential lifecycle.
- OIDC discovery, signing keys, and token issuance.
- Configurable notification delivery for SMS and email.
- Provider-side authentication policies and rate limiting.

### Go API

- Validate access tokens using the provider's issuer, audience, signature, and token validity rules.
- Extract the stable OIDC subject (`sub`) as the external identity key.
- Map the OIDC subject to the local `users.oidc_subject` record.
- Reject invalid, expired, incorrectly issued, or incorrectly targeted tokens.
- Perform application authorization independently of authentication.
- Enforce rider/driver/admin permissions and all business rules.

## Authentication flows

### 1. New account

```text
Flutter signup UI
  -> OIDC authorization/authentication flow
  -> identity verification (phone/email/etc.)
  -> identity provider issues tokens
  -> Flutter calls Go API with access token
  -> Go API resolves users.oidc_subject
  -> local user profile is created/completed as required
```

The identity provider is the authority for authentication. The Go API is the authority for application account state.

### 2. Existing account

```text
Flutter login UI
  -> OIDC authorization flow with PKCE
  -> identity provider authenticates user
  -> provider issues access/ID tokens
  -> Flutter calls Go API
  -> Go API validates token
  -> Go API resolves local user
```

### 3. OTP

OTP is an identity-provider concern. The provider must support configurable SMS/email notification delivery so that changing the delivery gateway does not require changes to the Flutter authentication UI or Go business services.

### 4. MFA

MFA remains within the identity provider. The Flutter application presents the required provider-defined authentication steps and does not implement its own parallel MFA state machine.

### 5. Logout and session revocation

Flutter clears its local token/session state. Provider-side session and token revocation semantics remain the identity provider's responsibility. The API must continue to reject expired or otherwise invalid access tokens.

### 6. Account recovery

Account recovery is handled by the identity provider using its configured recovery mechanisms. Application-specific recovery of business data is separate and remains under the Go API's control.

## Authorization model

OIDC authentication answers:

> Who is this user?

Application authorization answers:

> What is this authenticated user allowed to do in our system?

The API therefore must not equate possession of a valid OIDC token with permission to perform an operation.

Examples:

- An authenticated rider may create a ride.
- An authenticated rider may not create a driver bid.
- A driver may update driver-specific state only when the local driver account is eligible and active.
- Administrative operations require explicit application-level authorization.

These rules are derived from local application state and domain policies, not solely from OIDC claims.

## Identity mapping

The OIDC `sub` claim is the stable external identity identifier and maps to:

```text
users.oidc_subject
```

The local UUID remains the application's internal user identifier. Business tables must reference the local UUID rather than the OIDC subject directly.

```text
OIDC subject
     |
     v
users.oidc_subject
     |
     v
users.id (local UUID)
     |
     +--> driver_profiles.user_id
     +--> rides.rider_id
     +--> other business relations
```

This prevents the domain model from becoming coupled to a particular identity provider.

## Provider selection requirements

The concrete self-hosted identity provider must support, or provide a secure extension mechanism for:

1. OIDC discovery and standard OAuth 2.0 authorization flows.
2. Authorization Code + PKCE for Flutter/mobile clients.
3. Phone authentication and SMS OTP.
4. Email verification and email OTP or an equivalent secure email authentication flow.
5. Configurable SMS delivery providers, preferably through a generic HTTP/webhook adapter.
6. Configurable email delivery through SMTP or provider adapters.
7. Customizable notification templates and messages.
8. MFA.
9. Secure token signing and JWKS publication.
10. Issuer and audience configuration suitable for a self-managed deployment.
11. Rate limiting and abuse controls for authentication endpoints.
12. Session/revocation controls appropriate for a mobile application.
13. A documented way to customize the authentication UI or expose APIs suitable for the Flutter-owned UI without requiring proprietary Flutter authentication logic.

## Explicit non-goals

- The Go API will not become an authentication server.
- The Flutter app will not implement password hashing or OTP generation.
- The application will not directly store user passwords.
- The application will not hard-code an SMS or email vendor into business logic.
- This ADR does not select ZITADEL, Keycloak, authentik, or another concrete provider.

## Consequences

### Positive

- Strong separation between authentication and business authorization.
- Flutter can provide a fully customized product experience.
- SMS/email delivery can change independently of application code.
- The Go API remains portable across OIDC providers.
- Identity-provider security responsibilities remain concentrated in a purpose-built component.
- Local application identity remains stable if the external provider changes.

### Negative

- The system has an additional infrastructure component to operate.
- Provider integration requires careful validation of issuer, audience, signing keys, claims, token lifetime, and mobile OAuth behavior.
- Custom Flutter authentication UI increases client implementation work compared with using a provider-hosted UI.

## Follow-up

Before implementing a concrete OIDC adapter, compare candidate self-hosted providers against the provider selection requirements above. Only after that comparison should the repository add a provider-specific dependency or configuration.
