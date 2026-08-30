# ADR 0003: Schema canonicalization and dependency resolution

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

Digest-bound descriptors, locks, and evidence require one unambiguous JSON interpretation. Dependency ranges must resolve reproducibly without allowing prerelease or build metadata ambiguity to select unexpected code.

## Decision

- ThinkPixelMP schemas use JSON Schema Draft 2020-12 and canonical IDs under `https://schemas.thinkpixel.io/thinkpixelmp/`.
- Objects reject unknown properties. Parsers reject duplicate JSON keys before schema validation.
- Trusted application code applies explicit defaults; schema validators do not mutate submitted documents.
- Hashable JSON is serialized with RFC 8785 JSON Canonicalization Scheme and hashed with SHA-256.
- MP adopts ThinkPixelAR AgentRuntimeSpec v1 structurally and references its canonical future identifier `https://schemas.thinkpixel.io/thinkpixelar/agent-runtime-spec/v1`. The MP contract records the current cross-repository compatibility requirement without modifying ThinkPixelAR.
- Dependency selector precedence is exact digest, exact semantic version, then semantic-version range.
- A range selects the highest satisfying catalog-eligible version. Prereleases are excluded unless the range explicitly includes a prerelease comparator.
- If multiple candidates have equal SemVer precedence and differ only by build metadata, range resolution fails as ambiguous.
- Optional dependencies are excluded by default and included only through an explicit resolution request selection by dependency name.
- Every successful resolution produces a canonical immutable lock graph and SHA-256 lock digest.

## Alternatives considered

- Permissive unknown fields were rejected because older readers could silently ignore security-relevant meaning.
- Validator-applied defaults were rejected because validators vary and may change hashed content.
- Lexical build-metadata tie-breaking was rejected because SemVer declares build metadata precedence-neutral.
- Automatically including optional dependencies was rejected because it creates implicit graph and requirement expansion.

## Consequences

Writers must version schemas to add fields. Range resolution can require publisher or administrator intervention when build variants are ambiguous. Repeated resolution over the same catalog snapshot and inputs produces the same lock.

## Security and privacy

Canonicalization is an integrity mechanism, not authorization. Every resolved dependency remains independently subject to catalog, lifecycle, evidence, and policy checks. A bundle never receives the union of dependency authority.

## Compatibility and migration

Unknown schema versions fail closed. Adding an algorithm, field, selector, or canonicalization profile requires an explicit version and compatibility contract.

## Operations

Validation and resolution errors use stable bounded reason codes. Implementations require golden canonicalization vectors, duplicate-key rejection tests, ordering tests, and property tests for deterministic resolution.

## References

- [Schema profile](../contracts/schema-profile.md)
- [Dependency resolution](../contracts/dependency-resolution.md)
- [Media types](../contracts/media-types.md)
