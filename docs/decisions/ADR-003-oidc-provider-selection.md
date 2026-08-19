# ADR-003: OIDC Provider Selection

- Status: Proposed
- Date: 2026-08-19

## Decision summary

For this project, the strongest fit is **Ory Kratos as the headless identity/authentication layer**, with **Ory Hydra added only if we require a standards-based OIDC provider for the mobile client or future third-party clients**.

This is preferable to selecting Firebase/Auth0-style hosted authentication because our primary requirement is control over the authentication experience and over SMS/email delivery. It is also preferable to forcing Keycloak or authentik into a role they are not optimized for: a product-specific, API-driven, headless authentication experience owned by our Flutter application.

## Important correction to the original architecture wording

OIDC is primarily an identity/authentication protocol. It is not the source of truth for our application authorization.

The correct boundary is:

```text
Flutter
  |
  | authentication interaction
  v
Identity layer
  |
  | identity/token/session
  v
Go API
  |
  | application authorization + domain rules
  v
PostgreSQL
```

A valid identity token proves authentication. The Go application still decides whether that authenticated user may create rides, submit bids, operate as a driver, access another user's data, or perform administrative actions.

## Why Ory is the leading candidate

Ory Kratos is explicitly headless and is designed to let the application build its own authentication UI while Kratos handles identity, login, registration, MFA, recovery, and session management. Its current project documentation lists SMS, TOTP, passkeys, social sign-in and other authentication capabilities, and the project is Apache-2.0 licensed. citehttps://github.com/ory/kratos

Ory Hydra is an OpenID Certified OAuth 2.0/OIDC provider. It supports Authorization Code with PKCE and is designed to delegate the actual login UI to an application through a Login and Consent flow. This makes it a strong fit when we need a standards-based OIDC layer without surrendering control of the authentication UX. citehttps://www.ory.com/hydra

Kratos also supports SMS-based authentication/recovery through configurable gateways and customizable notification behavior, which aligns better with our requirement to control the SMS/email delivery path. citehttps://changelog.ory.com/announcements/account-recovery-via-sms

## Candidate comparison

| Requirement | Ory Kratos + optional Hydra | Keycloak | authentik | Firebase-style managed IdP |
|---|---|---|---|---|
| Self-hosted | Yes | Yes | Yes | No |
| Headless/custom application UI | Excellent | Possible, but more provider-centric | Strong flow customization | Provider-dependent |
| Flutter product-specific UX | Strong fit | Possible | Possible | Usually coupled to provider SDK/UX |
| OIDC | Hydra provides it | Yes | Yes | Yes, but vendor-specific ecosystem |
| Authorization Code + PKCE | Hydra | Yes | Yes | Yes |
| SMS OTP | Kratos capabilities/gateway | Requires configuration/extensions | Flow-dependent | Usually built in |
| Custom SMS gateway | Strong via gateway/notification integration | Extension/provider dependent | Flow/integration dependent | Vendor/provider dependent |
| Custom email delivery | Strong | Strong | Strong | Vendor dependent |
| MFA | Yes | Yes | Yes | Yes |
| API-first/headless model | Excellent | Moderate | Moderate/strong | Vendor API |
| Provider lock-in | Low | Low | Low | High |
| Operational complexity | Higher if Kratos + Hydra are both deployed | High | Moderate | Low infrastructure burden |

This table is a decision aid, not a claim that every feature is identical across versions. Concrete provider capabilities must be validated against the exact release we deploy before production.

## Why not Firebase as the foundation

Firebase may be operationally convenient, but it conflicts with the core requirement that authentication behavior, messaging providers, and the product authentication experience remain under our control. It should therefore not be the architectural foundation for this project.

## Why not Keycloak as the default

Keycloak is a serious and mature choice and should not be dismissed. It is particularly strong when enterprise federation, LDAP/Active Directory, SAML, and complex IAM administration are central requirements. Current comparisons continue to identify Keycloak as the mature enterprise-oriented option. citehttps://www.cerbos.dev/blog/authentik-vs-keycloak-selfhosted-idp

Our current problem is different. We are building a consumer mobile product and want a headless, product-controlled authentication experience. Keycloak can be customized, but its model is heavier than necessary for this requirement.

## Why not authentik as the default

authentik is also a credible self-hosted candidate, particularly when forward authentication, LDAP, and visual authentication flows are important. It is not a bad choice. However, those strengths are less important to our current architecture than a headless identity API and explicit separation between identity management and OIDC token issuance.

## Mobile authentication flow

We should not interpret "Flutter sends the username/password/OTP directly to OIDC" as a requirement to implement a proprietary password-grant protocol.

For a standards-based OIDC architecture, the preferred flow is:

```text
Flutter
  |
  | Authorization Code + PKCE
  v
OIDC authorization endpoint
  |
  | authentication UI / authentication API
  v
Kratos identity layer
  |
  | successful authentication
  v
Hydra authorization flow
  |
  | authorization code
  v
Flutter
  |
  | code + PKCE verifier
  v
OIDC token endpoint
  |
  v
Access token / ID token
  |
  v
Go API
```

The product can still own the visual experience. The security-sensitive protocol and credential handling must remain within the identity stack rather than being reimplemented in Flutter or the Go API.

For a truly native Flutter-only credential UI, we must be explicit about the trade-off: directly submitting credentials to a headless identity API is not the same thing as a normal browser-based OIDC Authorization Code flow. If we choose that route, the architecture should use Kratos's headless APIs and session model rather than pretending it is standard OIDC. We should not create a custom OAuth password grant just to make the diagrams look simpler.

## Recommended implementation phases

### Phase 1 — Identity provider proof of concept

Deploy Kratos with PostgreSQL in the existing Docker Compose environment.

Validate:

- registration
- login
- logout/session lifecycle
- email verification
- email OTP/code flow
- phone verification
- SMS OTP
- MFA
- recovery
- custom notification gateway
- custom templates
- rate limiting and abuse controls

### Phase 2 — Mobile integration

Build the Flutter authentication abstraction without coupling the rest of the app to Kratos-specific APIs.

The abstraction should expose concepts such as:

```text
signUp
signIn
verifyEmail
verifyPhone
verifyOTP
enableMFA
logout
refreshSession
currentIdentity
```

The implementation behind this interface can change without changing ride, driver, bidding, or payment features.

### Phase 3 — OIDC layer

If the Go API must consume standard OIDC bearer tokens, deploy Hydra and connect the authentication/login flow to Kratos.

Then implement the Go API's verifier against OIDC discovery/JWKS and validate:

- issuer
- audience
- signature
- expiry/not-before
- token type/use
- subject

### Phase 4 — Local identity mapping

Keep the existing model:

```text
OIDC sub
  -> users.oidc_subject
  -> users.id
```

Never put provider-specific identifiers into ride, bid, reservation, assignment, payment, or other domain tables.

## Security rules

1. Never implement password hashing in the Flutter app.
2. Never generate authentication OTPs in Flutter.
3. Never accept a caller-supplied `user_id` as proof of identity.
4. Never trust an unsigned or unverified JWT.
5. Validate OIDC issuer and audience explicitly.
6. Keep OIDC signing keys outside application source code.
7. Keep identity-provider secrets out of Git.
8. Do not expose Hydra administrative endpoints publicly.
9. Rate-limit authentication and OTP attempts.
10. Treat SMS as a weaker authentication channel than passkeys/WebAuthn and use appropriate risk controls.
11. Keep application authorization in the Go API even when OIDC claims contain roles/groups.
12. Keep the provider behind an internal abstraction so changing providers does not require rewriting business logic.

## Consequence

We are not building authentication ourselves. We are building the **product experience around a dedicated identity system**.

That distinction matters. Reimplementing password, OTP, MFA, session, and token security in the Flutter/Go application would create exactly the security and maintenance problem we are trying to avoid.

The next implementation task is therefore a **Kratos proof of concept**, not an OIDC dependency added to the Go API immediately.
