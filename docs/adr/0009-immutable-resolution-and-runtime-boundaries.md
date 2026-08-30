# ADR 0009: Immutable resolution and runtime boundaries

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

Downstream governance needs reproducible artifact graphs without making MP a synchronous runtime dependency or allowing marketplace discovery to become execution authority.

## Decision

- ArtifactResolution is canonical JSON with its own SHA-256 digest and complete immutable graph/evidence/lifecycle context.
- MP signs resolutions with an operator-managed MP identity distinct in purpose from publisher and promotion identities.
- Historical resolutions remain retrievable and are marked affected after quarantine/revocation; new protected resolution excludes affected content.
- AG may use a previously verified durable resolution during MP outage according to AG's revocation-freshness policy.
- MP supplies at-least-once events and authoritative digest-state reconciliation.
- Search is deterministic structured/lexical by default. Optional semantic ranking is non-authoritative.
- MP has no direct execution or credential authority path to AR or TG.

## Alternatives considered

- Resolving mutable aliases during each Run was rejected because it breaks reproducibility and availability.
- Deleting affected resolutions was rejected because it destroys historical evidence.
- Making semantic ranking select production content was rejected because ranking is not eligibility.

## Consequences

AG stores and verifies resolution bytes/digest/signature and owns freshness decisions. Consumers must reconcile after event gaps. Search and resolution use separate contracts.

## Security and privacy

Resolution signatures attest MP output, not publisher authorship or Run authorization. Per-node requirements remain declarations. TG credentials and AR infrastructure are absent.

## Compatibility and migration

Resolution schema/canonicalization versions are explicit. Unknown versions fail closed. Historical versions remain verifiable.

## Operations

Resolution output includes lifecycle observation time and event cursor. Signing/key failures fail protected resolution closed.

## References

- [ArtifactResolution](../contracts/artifact-resolution.md)
- [ThinkPixel integrations](../contracts/thinkpixel-integrations.md)
