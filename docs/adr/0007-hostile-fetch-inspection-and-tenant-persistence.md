# ADR 0007: Hostile fetch, inspection, and tenant persistence

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

Marketplace registration and federation consume attacker-controlled URLs, DNS, manifests, archives, and descriptors. Durable multi-tenant state must remain isolated even if an application query is incorrect.

## Decision

- Production remote fetching is HTTPS-only and denies non-public address classes unless a source-specific operator allowlist permits an internal target.
- A visibly isolated development profile may permit HTTP, loopback, and private targets for local testing.
- Redirects are limited to three, cannot downgrade TLS, are revalidated at every hop, and never receive cross-origin credentials.
- TLS 1.2 or newer, hostname verification, and operator-managed trust roots are mandatory in production.
- Hostile-content inspection uses configurable instance defaults, approved compiled ceilings, and bounded archive semantics. Artifact metadata cannot alter limits.
- Registry credentials are operator-owned, origin/repository scoped, and read-only by default; publish credentials are separate.
- Every tenant-owned PostgreSQL relation carries `tenant_id`, tenant consistency is enforced by constraints and repository context, and production enables RLS defense in depth.

## Alternatives considered

- A general-purpose unrestricted HTTP client was rejected because it centralizes no SSRF or credential controls.
- Globally blocking all private addresses was rejected because private enterprise deployments require explicitly approved internal registries and endpoints.
- Application-only tenant filtering was rejected because database defense in depth is warranted for authoritative marketplace state.

## Consequences

Operators must configure internal sources explicitly. Development behavior cannot be copied into production configuration. Administrative migration identities are separate from service identities.

## Security and privacy

DNS answers, redirects, archives, media types, and error bodies remain hostile. Limits apply while streaming and decompressing, before full materialization. RLS supplements rather than replaces application authorization.

## Compatibility and migration

Limit profiles and fetch policy are versioned configuration. Raising compiled hard ceilings requires an explicit profile/build change.

## Operations

Blocked-address, redirect, TLS, timeout, limit, and archive failures produce bounded reason codes without leaking credentials or sensitive response content.

## References

- [Remote fetch security](../security/remote-fetching.md)
- [Hostile content inspection](../security/hostile-content-inspection.md)
- [PostgreSQL model](../architecture/postgresql-model.md)
