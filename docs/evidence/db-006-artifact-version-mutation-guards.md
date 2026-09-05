# DB-006 ArtifactVersion mutation guards

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The sixth forward migration installs a database trigger that rejects every update
or deletion of a registered ArtifactVersion. This protects the exact content
digest and semantic-version binding together with tenant and resource identity,
logical Artifact and Publisher attribution, fixed kind, class and delivery
metadata, registration timestamp, and initial lifecycle value. The function is
not executable by `PUBLIC` directly.

The application repository remains registration-only and exposes no update or
delete operation. The database guard independently protects authoritative state
from administrative SQL mistakes and future adapters. Append-only deprecation,
quarantine, and revocation records remain lifecycle work; this change does not
invent direct lifecycle mutation. ArtifactSource, descriptor, requirement,
dependency, audit, and outbox persistence remain in their sequenced tasks. No
public API/schema or cross-component contract changed, and no dependency was
added.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db006-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/artifactversion ./internal/adapters/postgres/artifactversion ./internal/adapters/postgres/migration ./cmd/migrate`
  passed.
- `GOCACHE=/tmp/thinkpixelmp-db006-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered all six
  migrations and verified rejection of digest, semantic-version, payload metadata,
  lifecycle, registration-time, and deletion mutations while preserving the
  registered row.
- `GOCACHE=/tmp/thinkpixelmp-db006-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.
