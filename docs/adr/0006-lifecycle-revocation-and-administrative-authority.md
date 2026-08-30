# ADR 0006: Lifecycle, revocation, and administrative authority

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

Deprecation, investigation, and confirmed compromise require different failure postures. Administrative access must not collapse unrelated marketplace powers into one implied hierarchy.

## Decision

- Deprecation policy-blocks new protected promotion and resolution while preserving existing immutable resolutions.
- Quarantine is reversible, blocks new promotion/protected resolution, emits an event, and marks existing resolutions as affected. AG owns live-Run response.
- Digest revocation is append-only and irreversible. Correction records may explain but never restore eligibility.
- Revocation severity is `low`, `medium`, `high`, or `critical` and includes stable reason, evidence, effective time, and optional exact replacement.
- An authoritative replacement is an existing non-revoked version of the same logical Artifact and compatible kind.
- Tenant/principal derive only from verified configured identity mappings.
- Administrative roles are tenant-scoped, separate, explicitly granted, and have no implied inheritance.

## Alternatives considered

- Reversible revocation was rejected because consumers cannot safely interpret a previously compromised digest as trustworthy again.
- Automatic termination of Runs by MP was rejected because AG owns runtime revocation policy.
- A single marketplace-admin role was rejected because it defeats least privilege and separation of duties.

## Consequences

False-positive revocations require a new artifact digest/version. Existing resolution consumers need event/reconciliation integration to apply quarantine and revocation policy.

## Security and privacy

Caller tenant fields and forwarded headers are never authority. Lifecycle mutations are authenticated, authorized, audited, and transactionally evented. Historical evidence is never deleted.

## Compatibility and migration

New severity or lifecycle meanings require versioned contracts. Consumers must treat unknown effective revocation state conservatively.

## Operations

MP supports event delivery plus authoritative reconciliation. Revocation and quarantine events have stable IDs and at-least-once delivery semantics.

## References

- [Lifecycle and revocation](../contracts/lifecycle-and-revocation.md)
- [Authentication and administration](../security/authentication-and-administration.md)
