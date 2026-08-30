# ADR 0008: Federation, catalog snapshots, and promotion attestations

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

ThinkPixelMP must import multiple ecosystems without laundering upstream status into local trust, support disconnected environments, and export local promotion evidence without impersonating publishers.

## Decision

- ImportSource is tenant-scoped, administrator-configured, disabled by default, and bound to one source kind and fetch profile.
- Imports create candidate state only and preserve original source and importer provenance.
- Changed content under an unchanged upstream version creates a conflict for review; history is never overwritten.
- Catalog snapshots are signed immutable OCI artifacts scoped to one tenant/catalog. They contain metadata and digests, not artifact payload bytes.
- A snapshot may identify a parent snapshot digest but remains independently verifiable.
- MP promotion attestations bind exact artifact, catalog, policy, evidence snapshot, lock, decision, time, and MP signer identity. They explicitly identify MP as promoter, not publisher.

## Alternatives considered

- Trusting upstream verified/approved status was rejected because local trust policy is authoritative.
- Rewriting changed imports in place was rejected because it destroys provenance and enables substitution.
- Embedding payload bytes in catalog snapshots was rejected because OCI mirroring is a separate distribution operation.
- Signing as the publisher was rejected because promotion and authorship are distinct identities.

## Consequences

Federation requires local validation and promotion. Air-gap workflows transfer a snapshot plus separately mirrored payloads/referrers. Conflicts require administrative resolution.

## Security and privacy

Imports use the hardened fetcher and hostile-content inspector. Snapshot and attestation signatures follow operator-managed MP trust, not publisher trust. Tenant-private metadata is not included in another tenant's snapshot.

## Compatibility and migration

Import normalization, snapshot, and attestation profiles are versioned. Parent ancestry is optional and cannot replace full snapshot verification.

## Operations

Every import, conflict, snapshot, and attestation has stable identity, audit/outbox records, exact input/output digests, and bounded failure metadata.

## References

- [Federation and import](../contracts/federation-and-import.md)
- [Catalog snapshots](../contracts/catalog-snapshot.md)
- [Promotion attestations](../contracts/promotion-attestation.md)
