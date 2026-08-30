# EvidenceRecord contract

## Identity and subject

An EvidenceRecord is immutable and tenant-scoped. It contains a UUIDv7 ID, exact ArtifactVersion ID and SHA-256 subject digest, evidence category and schema version, trusted producer identity/version, producer event ID, report digest/reference, normalized summary, timestamps, and integrity/signature metadata where applicable.

Evidence for one digest cannot qualify another digest even when names, versions, sources, or publishers match.

## Categories

Initial categories are:

- `publisher-verification`;
- `signature-verification`;
- `provenance`;
- `sbom`;
- `vulnerability-scan`;
- `malware-analysis`;
- `static-analysis`;
- `license-analysis`;
- `agent-evaluation`;
- `human-review`;
- `runtime-compatibility`;
- `endpoint-ownership`;
- `endpoint-tls`;
- `endpoint-health`.

Categories remain independently queryable and policy-visible.

## Processing and conclusion

`ingestion_status` is `accepted`, `rejected`, or `processing-error`.

Only accepted evidence has a normalized conclusion: `pass`, `fail`, `warning`, `unknown`, or `not-applicable`. Rejection or processing failure never maps to `pass`, and cannot satisfy protected policy.

Accepted summaries conform to the strict category-specific [evidence normalization contract](evidence-normalization.md). The summary's category must equal the enclosing EvidenceRecord category; mismatches are rejected.

## Producer trust

An EvidenceProducer is operator-configured for one tenant and an allowlist of categories. Authentication uses configured workload OIDC, mTLS identity, or verified signed-attestation identity. The authenticated identity must match producer configuration and category scope.

Evidence ingestion derives producer identity/version from authenticated configuration. The body redundantly supplies ArtifactVersion and exact subject digest, but cannot name a trusted producer or set the normalized conclusion. The producer's native result remains inside the immutable raw report; MP's configured normalizer creates the conclusion.

A publisher declaration, registry attachment, or syntactically valid report does not become trusted evidence automatically.

Ingestion is idempotent by `(tenant, producer, producer_event_id)`. Reuse with different content is rejected and audited.

Raw report locations are structured OCI references or configured object-store/key references with exact digest, media type, and size. Arbitrary HTTP report URLs are prohibited.

## Time and freshness

Records distinguish `created_at`, `observed_at`, `ingested_at`, and optional producer `expires_at`. Future observations outside configured clock skew are rejected.

Producer expiry is an upper bound. Effective freshness is the earliest of producer expiry and the active catalog policy's maximum age from observation. Evidence without an expiry may still become stale under policy.

## Raw reports

PostgreSQL stores bounded normalized summaries, parser/profile version, exact report digest, media type, size, and immutable OCI or object-store reference. Raw reports are not copied into logs, traces, errors, policy input, or ordinary audit metadata.

Object-store locations and credentials are operator-controlled. A report URL supplied by an untrusted publisher is not fetched outside the hardened fetch contract.

## Human review

Human review evidence binds the artifact digest, authenticated reviewer principal, review type, conclusion, timestamp, and optional expiry. It records expert analysis but does not transition a PromotionRequest or create a CatalogEntry. Promotion review is a separate authorized state-machine record.
