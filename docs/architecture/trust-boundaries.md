# Trust boundaries

| Boundary | Input posture | Authority retained by receiver | Required failure posture |
| --- | --- | --- | --- |
| Publisher/client → MP | Authenticated caller but metadata remains untrusted | Tenant mapping, administrative authorization, validation, immutable registration | Reject unauthorized, ambiguous, conflicting, or malformed input |
| Federation source → fetcher/MP | Fully hostile remote metadata and content | Source allowlist, network policy, limits, normalization, candidate-only import | Fail closed; never auto-promote |
| OCI registry → MP | Registry response and blobs are hostile | Digest verification, media-type/profile validation, bounded inspection | Reject mismatch or unsafe content |
| Signature infrastructure → MP | Cryptographic material is untrusted until verified | Configured roots, identities, issuer and namespace policy | Record invalid/untrusted independently; never convert to generic approval |
| Scanner/evaluator → MP | Trusted only for configured producer/category scope | Producer authentication, exact subject, normalization and freshness | Reject forged/out-of-scope evidence |
| MP ↔ OPA | Typed bounded data only | MP validates active policy and typed output; OPA decides catalog eligibility only | Protected promotion fails closed |
| MP → PostgreSQL | Internal mutation path | Transactional invariants, tenant scope, immutability, audit/outbox | Roll back incomplete mutation |
| MP → evidence sink | At-least-once exported evidence | Stable event identity; sink provides independent retention | Retry without duplicating logical event |
| AG ↔ MP | Authenticated service integration | MP qualifies exact digest; AG authorizes Run | No mutable fallback or authority expansion |
| MP → AR | No direct runtime authority path | AR accepts exact digests through AG and independently verifies materialized digest | Reject alias/latest or mismatch |
| MP → TG | Discovery/onboarding metadata only | TG owns live configuration and credentials; AG owns Run capability | No automatic tool enablement |

ThinkPixelLLMGW and ThinkPixelGR have no direct MP authority relationship in the release candidate. Marketplace metadata cannot configure their privileged runtime policy.
