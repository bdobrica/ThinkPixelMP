# ADR 0002: OCI artifact and referrer model

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

ThinkPixelMP must distribute local artifacts through standards-compatible registries without implementing a registry or creating a proprietary evidence silo. Evidence must remain bound to exact immutable content even when registry capabilities vary.

## Decision

- Each locally distributed ThinkPixel ArtifactVersion uses an OCI 1.1 image manifest or artifact manifest representing one immutable payload identity.
- Typed config or descriptor blobs describe the artifact kind. Payload layers preserve the artifact's native format where an open ecosystem format exists.
- The resolved manifest digest is the ArtifactVersion payload identity used by MP.
- Signatures, provenance, SBOMs, security/evaluation evidence, and ThinkPixelMP attestations attach as OCI referrers whose subject is that exact manifest digest.
- MP uses the OCI 1.1 Referrers API when available and the OCI referrers tag schema fallback when the registry supports it.
- If neither mechanism is usable, MP does not invent a proprietary registry mutation. PostgreSQL retains bounded normalized evidence and immutable digest/reference metadata for raw reports stored externally.
- PostgreSQL remains authoritative for marketplace-normalized evidence and promotion history. Registry attachments are portable source material, not automatic trusted evidence.
- Registry capability differences MUST NOT weaken subject-digest validation or silently make missing required evidence pass policy.

## Alternatives considered

- Building or requiring a ThinkPixel registry was rejected because MP is a control plane around external OCI infrastructure.
- Encoding all evidence inside the primary artifact was rejected because it would change artifact identity whenever evidence changes.
- Using mutable tags as evidence identity was rejected because it breaks exact-subject binding.
- Writing a ThinkPixel-specific registry index when referrers are unavailable was rejected in favor of normalized database records plus immutable external references.

## Consequences

Evidence can arrive after artifact publication without changing payload identity. Registry portability depends on OCI capabilities, while MP retains a consistent normalized model. Air-gap export must copy both payloads and applicable referrers or external evidence objects.

## Security and privacy

All manifests, descriptors, layers, referrers, and reports are hostile input until bounded, parsed, authenticated where required, and matched to the exact subject digest. Registry presence does not confer trust. Credentials must never follow an untrusted origin or redirect.

## Compatibility and migration

ThinkPixel media types use `application/vnd.thinkpixel.*`. Schema identifiers use `https://schemas.thinkpixel.io/thinkpixelmp/`. Exact media-type names and schema shapes are defined by versioned contracts, not inferred from tags.

## Operations

The RegistryProvider exposes capability detection and typed failure classification without leaking ORAS types into the domain. Operators configure registry credentials by trusted registry scope. Missing referrer support is observable and must be reflected in evidence eligibility.

## References

- [OCI distribution profile](../contracts/oci-distribution.md)
- [Security invariants](../security/invariants.md)
