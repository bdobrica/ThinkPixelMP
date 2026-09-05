# DB-005 ArtifactVersion persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The ArtifactVersion domain binds UUIDv7 tenant, version, logical Artifact, and
attributed Publisher identifiers to the complete strict SemVer 2.0.0 string, an
exact canonical SHA-256 digest, the Artifact's fixed kind, a validated artifact
class and delivery model, lifecycle, and a UTC registration timestamp. New values
start active. Restore accepts the four published lifecycle values so later
append-only lifecycle records can supply an effective state without weakening the
closed vocabulary. The domain validates the artifact taxonomy's kind/class/delivery
combinations and uses the shared algorithm-aware digest primitive.

The PostgreSQL schema makes `(tenant_id, artifact_id, semantic_version)` unique,
including prerelease and build metadata in the key, and makes
`(tenant_id, resolved_digest)` unique so one tenant-local content identity cannot
create multiple ArtifactVersion resources. Composite foreign keys preserve tenant
consistency and require the version kind to equal its logical Artifact's fixed
kind. Publisher attribution is also tenant-consistent. Check constraints reproduce
the strict SemVer, digest, closed vocabulary, and taxonomy rules, while forced RLS
provides database defense in depth.

The registration-only repository exposes tenant-scoped create, read by ID, exact
digest, or Artifact/SemVer, and bounded per-Artifact UUID-ordered listing. Every
operation uses a transaction, validates identifiers, sets tenant scope through a
bound transaction-local parameter, and repeats tenant predicates. Duplicate
version or digest identities, missing parents, invalid input, and unavailable
persistence map to stable redacted typed errors.

DB-006 remains responsible for database mutation guards. DB-007 and DB-008 remain
responsible for source/resolved-reference and descriptor persistence. Requirements,
dependencies, publication authorization, idempotency, and audit/outbox coupling
also remain in their sequenced tasks. No public API/schema or cross-component
contract changed, and no dependency was added.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db005-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactversion ./internal/adapters/postgres/artifactversion ./internal/adapters/postgres/migration ./cmd/migrate` passed.
- `GOCACHE=/tmp/thinkpixelmp-db005-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all five
  migrations plus ArtifactVersion registration/read/list, strict SemVer and digest
  constraints, tenant-scoped uniqueness and lookup, parent consistency, taxonomy
  combinations, initial lifecycle, and RLS denial.
- `GOCACHE=/tmp/thinkpixelmp-db005-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.
