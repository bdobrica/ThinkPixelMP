# DB-009 ArtifactRequirement persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The ArtifactRequirement domain accepts normalized requirement bytes from the
trusted descriptor-validation boundary, enforces the descriptor's 1 MiB compiled
ceiling, rejects invalid UTF-8/JSON, duplicate keys, trailing content, unknown
fields, malformed set-like arrays, invalid capability/runtime/network/integration
values, and infrastructure-specific fields outside the published closed schema.
It verifies the separately supplied SHA-256 digest over the exact bytes and
returns defensive copies. Requirements are modeled only as declarations.

The ninth forward migration creates one immutable requirement per tenant-local
persisted ArtifactDescriptor. It stores exact digest-bearing bytes and their JSONB
equivalent. A database trigger requires that projection to equal the parent
descriptor's `requirements` member, preventing an independently substituted
declaration. Forced RLS and explicit tenant predicates provide defense in depth;
another trigger rejects every update and deletion.

The registration-only PostgreSQL repository exposes tenant-scoped create and read
operations. Creation succeeds only when the parent descriptor exists in the same
tenant and contains an equivalent normalized requirement. Duplicate and
missing/mismatched cases map to stable redacted typed errors. Identical requirement
digests may validly recur because many artifact versions can declare the same
requirements.

This persistence slice does not grant capabilities, select runtime infrastructure,
resolve external integrations, implement the descriptor ingestion pipeline or RFC
8785 canonicalization, persist dependencies, or couple registration to audit/outbox
records. Those responsibilities remain in DSC-001 through DSC-007 and DB-010
through DB-013. No public API/schema, accepted ADR, dependency, or cross-component
boundary changed. `PLAN.md` remains unchanged because implementation sequencing
and intent did not change.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db009-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactrequirement ./internal/adapters/postgres/artifactrequirement ./internal/domain/artifactdescriptor ./internal/adapters/postgres/artifactdescriptor ./internal/adapters/postgres/migration ./cmd/migrate`
  passed.
- `GOCACHE=/tmp/thinkpixelmp-db009-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all nine
  migrations plus requirement create/read, exact normalized-byte round trip,
  digest preservation, descriptor-declaration binding, duplicate rejection,
  tenant isolation, and immutable update/delete guards.
- `GOCACHE=/tmp/thinkpixelmp-db009-go-cache GOTOOLCHAIN=go1.26.7 make verify`
  passed.
- `git diff --check` passed.
