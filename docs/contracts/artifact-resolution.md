# ArtifactResolution contract

An ArtifactResolution is immutable canonical JSON identified by SHA-256 and signed by an operator-managed ThinkPixelMP signing identity.

Its dependency graph conforms to the machine-readable [`ArtifactLock` schema](../../api/schemas/artifact-lock-v1.schema.json). Nodes are keyed and sorted by artifact digest. Edges are sorted by parent digest, original declaration index, dependency name, and resolved child digest. Policy, catalog, and evidence snapshots are referenced by exact digest rather than embedded.

`lock_digest` is computed as SHA-256 over RFC 8785 canonical JSON with the `lock_digest` property omitted, then inserted into the final object. Verification removes that property, canonicalizes, hashes, and compares. This rule avoids self-referential hashing.

## Contents

- schema, canonicalization, and resolution-algorithm versions;
- canonical resolution-input digest;
- UUIDv7 resolution ID, tenant, catalog, and creation time;
- exact root ArtifactVersion/digest;
- complete canonical ArtifactLock with exact transitive digests and optional selections;
- per-node declared capability, runtime, network, and integration requirements;
- policy bundle and decision/input digests;
- evidence snapshot digest and included EvidenceRecord/report digests;
- lifecycle state observed for every node;
- lifecycle observation time and authoritative event/reconciliation cursor;
- resolution digest and MP signing identity/signature reference.

The resolution does not grant capabilities, choose credentials, map Kubernetes/runtime infrastructure, authorize a principal/Run, or guarantee continued lifecycle eligibility.

## Creation

Protected resolution excludes removed catalog entries, deprecated policy-blocked artifacts, quarantined artifacts, and revoked digests. It resolves one immutable catalog/source view and emits canonical byte-identical output for identical inputs.

The request supplies catalog, logical Artifact, exactly one selector (ArtifactVersion ID, exact semantic version, or bounded range), and optional-dependency selections. Tags are not accepted. MP binds the active policy and complete current relevant trusted evidence set without caller overrides.

MP hashes the canonical resolution input containing catalog snapshot, selector, optional selections, policy, evidence snapshot, lifecycle facts, and event cursor. When that input digest already exists, MP reuses the established signed resolution. Creation and lifecycle-observation timestamps therefore describe the original immutable resolution.

## Later lifecycle changes

Historical resolution bytes are never rewritten or deleted. Read APIs overlay or return separately the current affected state when a contained digest is quarantined or revoked. New protected resolution cannot select the affected digest.

`GET` returns the signed immutable resolution unchanged plus a separate unsigned `current_impact` projection with newly quarantined/revoked nodes, observation time, and reconciliation cursor. Consumers verify the immutable object independently and treat current impact as fresh authenticated API state.

## Verification and availability

AG verifies schema, canonical digest, MP signature/trust, tenant/catalog context, and exact graph before durable storage. During MP outage, AG may use that stored resolution according to its own revocation-freshness contract. MP does not define AG's live-Run TTL.

Events are advisory delivery of authoritative changes. After a gap or reset-required response, AG reconciles exact digest state through MP APIs before claiming freshness.
