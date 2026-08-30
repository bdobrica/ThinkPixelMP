# ADR 0001: Artifact identity and namespaces

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

ThinkPixelMP needs human-readable discovery names without allowing mutable labels to become production identity. Its first release is a private, multi-tenant enterprise marketplace, while future federation must not create cross-tenant namespace authority.

## Decision

- A logical artifact is named `{namespace}/{name}`.
- Namespace segments and artifact names are lowercase ASCII DNS-style tokens: they begin and end with an ASCII letter or digit and may contain `-` internally.
- `/` separates namespace segments. Empty, `.` and `..` segments are invalid.
- Namespace ownership and uniqueness are tenant-scoped.
- A namespace has one owning Publisher. Explicit delegation may grant a Publisher control of a child prefix; it does not transfer the parent or grant runtime authority.
- Strictly nested delegations are allowed. The valid longest matching namespace prefix controls publication.
- Publishers in the release candidate are tenant-local enterprise identities whose verification is an administrative marketplace action. Public publisher identity schemes are deferred.
- An ArtifactVersion is identified authoritatively by its immutable content digest. Semantic versions and tags are discovery metadata.
- Release-candidate versions use strict SemVer 2.0.0 without a leading `v`. Prerelease and build metadata are preserved, and the full canonical version string is the immutable version key.
- Release-candidate content identity accepts only lowercase `sha256` digests containing exactly 64 hexadecimal characters. Schemas retain an algorithm boundary for future compatible extension.
- Re-registering the same logical artifact and semantic version with a different digest is permanently rejected. A publisher must issue a new semantic version.
- API resources use UUIDv7 identifiers. External artifact identity remains digest-based and does not become UUID-based.

## Alternatives considered

- Global namespaces were rejected because the first release is tenant-local and federation must not bypass local ownership.
- Mutable semantic-version replacement was rejected because it breaks evidence binding, promotion history, and reproducibility.
- Flat namespaces were rejected because explicit hierarchical delegation is needed for enterprise ownership.

## Consequences

Human labels remain useful for search, but all approvals, locks, resolutions, evidence, and revocations can bind to exact content. Mirrored upstream identities require explicit local namespace ownership or mapping.

## Security and privacy

Namespace ownership proves permission to publish under a tenant-local name only. It does not prove artifact safety, grant infrastructure privilege, or authorize a Run. Conflict rejection prevents tag or version substitution from rewriting approved history.

## Compatibility and migration

Canonical schema identifiers use `https://schemas.thinkpixel.io/`. ThinkPixel-specific OCI media types use the `application/vnd.thinkpixel.*` vendor tree. The HTTP API is versioned under `/v1`.

## Operations

Registration conflict responses must be deterministic and auditable. Ownership and delegation mutations require authenticated administrative authorization and audit/outbox records.

## References

- [Artifact identity contract](../contracts/artifact-identity.md)
- [Publisher and namespace contract](../contracts/publishers-and-namespaces.md)
- [Security invariants](../security/invariants.md)
