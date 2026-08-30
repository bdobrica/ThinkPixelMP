# Phase 0 enterprise blueprint reconciliation

Date: 2026-08-30

## Scope and source

This review reconciles ThinkPixelMP Phase 0 against the local `EnterpriseBlueprints/enterprise-execution-platform-for-ai-agents` source, especially its separation-of-concerns, threat, MCP gateway, governance plane, skill supply chain, independent evidence, subagent delegation, failure, and internal-standardization chapters.

The blueprint describes the enterprise platform's authority and enforcement model. ThinkPixelMP adds a marketplace and supply-chain control plane within that model; it does not replace the Agent Governance Plane, trusted gateways, runtime, revocation enforcement, or independent security evidence plane.

## Preserved blueprint invariants

| Blueprint invariant | ThinkPixelMP treatment |
| --- | --- |
| Loading an agent, skill, content, or subagent cannot expand Run authority | Artifact capabilities, network, runtime, model, protocol, and integration fields are requirements only. Resolution is not a grant. |
| Credentials stay outside the reasoning environment | Descriptors cannot contain or select enterprise credentials. Registry, report-store, signer, and remote-fetch credentials are operator-scoped adapter configuration. |
| Trusted gateways enforce consequential access | TG remains authoritative for live MCP tools, destinations, schemas, credentials, risk, and invocation. MP onboarding metadata cannot enable tools. |
| Revocation is reconciled at enforcement points | MP owns artifact-digest lifecycle records and emits ordered tenant events plus exact digest reconciliation. AG decides admission and live-Run response; MP does not terminate Runs. |
| Governance authority is itself governed | Administrative roles are explicit and non-inherited; production promotions default to independent reviewers; policy/signing configuration and lifecycle mutations are audited. |
| Skills are independent signed supply-chain artifacts | Skills preserve Agent Skills semantics, use deterministic OCI packaging, exact digests, Cosign evidence, hostile-content inspection, and conservative executable classification. |
| Skill restrictions and requirements are subtractive | Bundle aggregation forms an effective requirement union and fails conflicts. No declaration creates authority or combines with policy as an additive grant. |
| CloudEvents are envelopes, not proof | MP uses strict CloudEvents with transactional outbox delivery. Authenticity, retention, and independent security evidence remain separate concerns. |
| Failure of enforcement dependencies must not broaden authority | Protected publication, promotion, resolution, policy evaluation, fetch, verification, and stale lifecycle paths fail closed. Previously stored immutable resolutions remain usable only under consumer freshness policy. |

## Marketplace-specific additions

The blueprint intentionally does not specify a marketplace. Phase 0 adds the following bounded contracts needed to preserve its invariants across artifact discovery and promotion:

- tenant-local publishers, hierarchical namespaces, explicit delegation, and permanent immutable-version digest conflict rejection;
- five closed artifact kinds and four artifact classes, with OCI delivery normalized separately from authoring/import source;
- SHA-256 OCI manifest identity, typed config descriptors, deterministic skill archives, safe bounded inspection, and exact descriptor/payload digest separation;
- immutable evidence records with trusted category-scoped producers, exact-subject binding, raw-report separation, category-specific normalization, and independent freshness;
- contextual catalogs, signed OPA policy bundles, four-eyes promotion, complete signed catalog snapshots, and MP promotion attestations that never impersonate publishers;
- deterministic dependency selection, transitive requirement aggregation, exact immutable locks/resolutions, and current lifecycle impact returned separately from historical signed content;
- candidate-only federation imports, hardened centralized remote fetching, immutable source snapshots, explicit conflict handling, and no automatic trust inheritance;
- append-only deprecation/quarantine/revocation records and per-tenant ordered events with reconciliation after gaps;
- strict OpenAPI/JSON Schema contracts, tenant-safe PostgreSQL modeling, RLS defense in depth, transactional audit/outbox, and bounded external adapter ports.

## Authority boundary conclusions

- MP may determine marketplace eligibility in a catalog; it cannot authorize a principal, Run, model, tool, credential, network destination, or runtime resource expansion.
- AG remains the governed admission authority and consumes exact signed resolutions.
- AR materializes only exact admitted digests and independently enforces compatible runtime profiles.
- TG remains the MCP/tool reference monitor and credential broker.
- Enterprise revocation and security-evidence systems may consume MP state, but MP artifact lifecycle is not a substitute for their broader principal/Run/tool revocation scopes or pre-dispatch evidence modes.
- Cross-organization A2A use still requires explicit federation, trust translation, and downstream authorization; an imported Agent Card is only a candidate immutable descriptor.

## Result

No conflict was found between the enterprise blueprint and the Phase 0 marketplace contracts. The marketplace-specific additions preserve the blueprint's non-expansion, exact-identity, gateway enforcement, revocation freshness, independent evidence, and fail-closed principles. No blueprint authority was reassigned to ThinkPixelMP.
