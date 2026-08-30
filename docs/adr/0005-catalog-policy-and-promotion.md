# ADR 0005: Catalog policy and promotion

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

The same artifact may be acceptable in one organizational context and unacceptable in another. Promotion must be deterministic, inspectable, separation-of-duty aware, and safe under concurrent revocation or publisher-state changes.

## Decision

- Catalogs are tenant-scoped uniquely named collections. Names carry no hard-coded trust meaning.
- Catalog membership preserves append-only history and uses active/removed state rather than deletion.
- `CatalogPolicyEvaluator` is a transport-neutral typed domain port. The reference adapter embeds OPA in process.
- Policy bundles are immutable content-addressed OCI artifacts signed by trusted policy administrators. PostgreSQL stores transactional activation of one exact digest per catalog.
- Promotion states are `submitted`, `evaluating`, `awaiting-review`, `approved`, `denied`, and `cancelled`.
- A request snapshots the exact artifact, lock, policy digest, and evidence IDs/digests. Its evidence set never changes silently.
- Immediately before approval, MP rechecks live publisher suspension, quarantine, and digest revocation.
- Policy determines distinct-reviewer requirements. The protected production baseline requires two reviewers and excludes requester self-review.

## Alternatives considered

- Global meaning for catalog names was rejected because eligibility is tenant-contextual.
- Remote-only policy evaluation was rejected as the reference path because network availability would enlarge the protected promotion failure surface.
- Mutable Rego files/tags were rejected because decisions must identify exact policy content.
- Mutating pending requests when new evidence arrives was rejected because it destroys deterministic review context.

## Consequences

New evidence requires a new PromotionRequest. Policy administrators must publish and sign bundles before activation. Catalog removal and policy changes preserve historical decisions.

## Security and privacy

Protected promotion fails closed on missing, malformed, unavailable, unsigned/untrusted, or invalid policy. Policy input is typed and bounded and excludes secrets and raw reports. OPA output cannot bypass Go domain invariants or administrative authorization.

## Compatibility and migration

Policy input/output schema and evaluator profile are versioned. A new profile requires explicit activation and compatibility validation.

## Operations

Bundle activation is transactional and auditable. Evaluation records policy digest, entrypoint, input digest, output, duration, and bounded reason codes. Embedded evaluation has deadlines and resource limits.

## References

- [Catalog policy](../contracts/catalog-policy.md)
- [Promotion workflow](../contracts/promotion.md)
