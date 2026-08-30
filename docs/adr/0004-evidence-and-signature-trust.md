# ADR 0004: Evidence and signature trust

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

Marketplace evidence comes from publishers, registries, signature systems, scanners, evaluators, and humans with different authority and failure modes. A single status or generic verified badge would collapse incompatible assertions.

## Decision

- Evidence ingestion status and normalized conclusion are separate.
- Producer trust is tenant-scoped and evidence-category-scoped. Publishers cannot designate trusted producers.
- Evidence binds to an exact artifact digest and immutable report digest.
- Producer expiry is an upper bound. Catalog policy may require a shorter maximum age from `observed_at`.
- Full reports remain in OCI or an operator-configured object store; PostgreSQL stores bounded normalized summaries and immutable references.
- Signature verification supports Sigstore keyless identities and configured public keys. Trust rules are scoped by tenant, namespace, issuer, and signer identity.
- Cryptographic validity and policy trust are recorded independently.
- Human review evidence is distinct from promotion review and approval.

## Alternatives considered

- One trust score or verified boolean was rejected because signatures, provenance, scans, evaluations, and approval establish different facts.
- Publisher-selected evidence producers were rejected because that lets untrusted metadata choose its verifier.
- Trusting producer expiry without policy limits was rejected because it permits indefinitely fresh evidence.
- Storing unbounded reports in PostgreSQL was rejected for security, privacy, and operational reasons.

## Consequences

Policy inputs are larger but explicit. Consumers can distinguish failed evaluation from processing failure, cryptographic validity from trusted identity, and general human review from catalog authorization.

## Security and privacy

Rejected or malformed reports cannot satisfy policy. Future observation timestamps outside configured skew are rejected. Raw report contents are excluded from ordinary logs, traces, and policy inputs.

## Compatibility and migration

New evidence categories and normalized summary versions require explicit schema versions. Original report digests preserve reprocessing and auditability.

## Operations

Producer configuration, trust policy, parser version, freshness evaluation, and normalization errors are observable and auditable. Report storage retention is operator policy.

## References

- [Evidence records](../contracts/evidence-record.md)
- [Signature verification](../contracts/signature-verification.md)
- [SBOM ingestion](../contracts/sbom.md)
