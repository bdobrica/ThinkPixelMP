# DB-011 AuditEvent evidence

Date: 2026-09-05

## Implemented boundary

- `000011_audit_events.sql` adds append-only, forced-RLS AuditEvent storage with a
  database-generated UUIDv7 identity and tenant-consistent ownership.
- Audit facts retain only verified actor identity, stable action/resource/decision
  and reason codes, opaque resource identity, exact artifact/evidence/policy
  references, occurrence time, and optional request/trace correlation. Arbitrary
  request metadata, descriptors, source locations, explanations, and credentials
  have no persistence field.
- Publisher creation/state changes and the existing Namespace, Artifact,
  ArtifactVersion, ArtifactSource, ArtifactDescriptor, ArtifactRequirement, and
  ArtifactDependency creates insert their audit fact through the same transaction
  before commit. Artifact-version-related facts include the exact content digest.
- Deferred constraint triggers independently reject commit when a covered
  authoritative mutation has no matching audit fact in the current transaction.
  Audit write failure therefore rolls back the domain mutation.
- The audit domain validates bounded values and verified actor context. Its
  PostgreSQL repository exposes tenant-scoped get/list only; it does not expose an
  independent audit-create operation or any update/delete operation.

DB-011 does not implement OIDC claim mapping, authorization, tenant provisioning,
outbox/SSE delivery, a general transaction manager, or retention deletion. Those
remain explicitly sequenced work. No public API or versioned schema changed.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db011-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/audit ./internal/adapters/postgres/audit ./internal/adapters/postgres/{publisher,namespace,artifact,artifactversion,artifactsource,artifactdescriptor,artifactrequirement,artifactdependency} ./internal/adapters/postgres/migration ./cmd/migrate`
- `GOCACHE=/tmp/thinkpixelmp-db011-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOCACHE=/tmp/thinkpixelmp-db011-go-cache GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
