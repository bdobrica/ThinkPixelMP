# Threat model

## Scope and assets

Protected assets include tenant and namespace ownership, immutable artifact identity, descriptors and dependency locks, trusted producer configuration, evidence records, catalog membership, promotion decisions, revocations, registry credentials, signing material references, audit history, and resolution snapshots.

## Adversaries

ThinkPixelMP assumes malicious or compromised publishers, artifacts, descriptors, OCI manifests and layers, dependency graphs, remote endpoints, federation sources, scanners/evaluators, credentials, administrators, and upstream tags. It also assumes ordinary software faults, concurrency races, partial failures, stale caches, and dependency outages.

## Principal threats and required controls

| Threat | Required control and failure posture |
| --- | --- |
| Namespace takeover or tenant confusion | Verified identity-to-tenant mapping, tenant-scoped uniqueness, explicit delegation, auditable mutations; fail closed |
| Mutable tag substitution or semantic-version overwrite | Resolve once to digest; immutable registration; permanently reject conflicting version/digest pairs |
| Evidence replay onto different content | Exact subject digest validation and producer-scoped idempotency; reject |
| Forged or over-trusted evidence producer | Authenticated configured producers, category scope, signature/integrity checks, independent evidence dimensions |
| Dependency confusion or nondeterminism | Catalog/source context, deterministic tie-breaking, immutable lock graph, cycle and conflict rejection |
| Malicious OCI/archive content | Bounded manifests/layers/files, decompression limits, path and link safety, allowed media types, no execution |
| SSRF, DNS rebinding, or redirect credential theft | One hardened fetcher, validated schemes and addresses, DNS/IP checks, bounded redirects and responses, no cross-origin credential forwarding |
| Privilege injection through descriptors | Closed abstract requirement schemas; reject infrastructure-specific privileged fields |
| Promotion bypass or self-approval | Typed fail-closed policy, authorized state machine, separation-of-duty enforcement, atomic decision/catalog transaction |
| Revocation deletion or race | Append-only digest revocation, transactional audit/outbox, protected resolution exclusion, concurrency controls |
| Secret or proprietary report disclosure | Data classification, bounded normalized storage, raw-report references, redaction in errors/logs/traces |
| Database or marketplace compromise | Least privilege, immutable constraints, complete audit/outbox history, independent sinks, external digest verification by consumers |

## Trust-boundary rule

No caller-controlled field establishes tenant, publisher verification, evidence-producer trust, policy activation, registry credential selection, catalog approval, or runtime authority. Those values derive from authenticated identity and operator-controlled configuration or authoritative stored state.

## Residual risk

ThinkPixelMP cannot prove that artifact behavior is safe, that remote-service implementation matches its descriptor, or that every external scanner is correct. It preserves independently inspectable assertions so policy and human review can reason about those limitations.
