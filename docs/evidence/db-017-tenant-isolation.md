# DB-017 tenant-isolation evidence

Date: 2026-09-06

## Outcome

The real-PostgreSQL integration suite now proves tenant isolation across every
implemented repository using a non-superuser, non-`BYPASSRLS` service role. The
coverage combines explicit repository tenant predicates, forced row-level
security, and tenant-consistent database constraints.

| Repository | Cross-tenant evidence |
| --- | --- |
| Publisher | ID, slug, and list reads select only the requested tenant; a state change cannot target another tenant's Publisher. |
| Namespace and delegation | ID, path, list, owner resolution, and delegation reads remain tenant-local; cross-tenant owner references and delegation revocation fail closed. |
| Artifact | ID, logical-identity, and list reads remain tenant-local; another tenant's Namespace cannot be used as a parent. |
| ArtifactVersion | ID, digest, semantic-version, and list reads remain tenant-local; another tenant's Artifact or Publisher cannot be used as a parent. |
| ArtifactSource | Reads remain tenant-local and another tenant's ArtifactVersion cannot be used as a parent. |
| ArtifactDescriptor | Reads remain tenant-local and another tenant's ArtifactVersion cannot be used as a parent. |
| ArtifactRequirement | Reads remain tenant-local and another tenant's Descriptor cannot be used as a parent. |
| ArtifactDependency | Get/list remain tenant-local and another tenant's Descriptor cannot be used as a parent. |
| AuditEvent | Get/list remain tenant-local under the same service role used by audited mutations. |
| IdempotencyRecord | Ownership lookup and completion cannot observe or change another tenant's record. |
| OutboxMessage | Get and ordered claims cannot observe another tenant's message; delivery cannot complete another tenant's claim; event sequences start independently per tenant. |

The tests use two populated tenants with intentionally repeated slugs, paths,
artifact identities, semantic versions, digests, descriptor content,
requirements, and dependencies. This demonstrates isolation rather than merely
testing an empty second tenant. Cross-tenant misses retain the repositories'
existing bounded `not_found` or stale-claim result and do not expose the hidden
row.

No migration, production role, public API, cross-component contract, or
dependency changed.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/(publisher_repository|namespace_repository|artifact_repository|artifact_version_repository|idempotency_repository|outbox_repository)' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
