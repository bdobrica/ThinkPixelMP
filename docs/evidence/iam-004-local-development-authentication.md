# IAM-004 local development authentication evidence

- Date: 2026-09-06
- Scope: explicit fixed-identity authentication for disposable local use

## Outcome

ThinkPixelMP now has a provider-neutral authenticator boundary and a local
adapter that returns one operator-configured tenant/principal identity. Local
authentication has no implicit default: operators must select
`authentication.mode=local-development` and provide both a UUIDv7 tenant and a
bounded principal label. Resulting principal IDs are visibly prefixed
`local-development:` and never derive from request data.

Configuration validation rejects local authentication in `test` or
`production` process mode, rejects mixed local/OIDC configuration, and rejects
incomplete or malformed fixed identities. The adapter constructor independently
requires the development process and authentication modes, so bypassing the
ordinary loader cannot construct it for a production configuration. Supplying
bearer material to the local adapter fails with the bounded typed error
`unauthorized:identity.local_credential_forbidden`.

This change establishes authentication identity only. It does not grant an
IAM-003 administrative role, bypass domain checks, introduce a public API, or
change the OIDC verification and mapping contracts.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/config ./internal/adapters/localauth ./internal/ports/identity
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

The focused race-enabled tests cover explicit activation, production and test
rejection, OIDC/local mutual exclusion, required identity fields, visibly local
principal construction, constructor-level mode enforcement, and bearer
credential rejection.
