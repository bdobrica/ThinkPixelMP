# Phase 2 authoritative-persistence and identity evidence

- Date: 2026-09-06
- Phase baseline commit: `60c61c7`
- Exit gate: real PostgreSQL tests prove immutability, tenant isolation,
  namespace ownership, concurrency, rollback, and replay

## Outcome

Phase 2 established tenant-scoped authoritative persistence and the initial
publication identity boundary. PostgreSQL now stores Publisher, Namespace,
Artifact, immutable ArtifactVersion/source/descriptor/requirement/dependency
state, AuditEvent, IdempotencyRecord, and OutboxMessage records. Tenant-aware
repositories, forced row-level security, tenant-consistent constraints,
transactional composition, audit coupling, and bounded concurrency controls
protect that state.

The application boundary now verifies and maps OIDC identities, enforces exact
marketplace administrative actions, provides an explicitly development-only
local authentication mode, exposes the initial Publisher, Namespace, and
Artifact APIs, resolves delegated namespace publication ownership, and provides
the trusted immutable ArtifactVersion registration persistence seam. These
capabilities establish publication authority only; they do not grant runtime
authority or catalog eligibility.

## Delivered persistence and publication state

| Area | Items | Evidence |
| --- | --- | --- |
| Tenant schema and publication aggregates | DB-001–DB-010 | [migrations](evidence/db-001-migrations.md), [Publishers](evidence/db-002-publishers.md), [Namespaces](evidence/db-003-namespaces.md), [Artifacts](evidence/db-004-artifacts.md), [ArtifactVersions](evidence/db-005-artifact-versions.md), [mutation guards](evidence/db-006-artifact-version-mutation-guards.md), [sources](evidence/db-007-artifact-sources.md), [descriptors](evidence/db-008-artifact-descriptors.md), [requirements](evidence/db-009-artifact-requirements.md), and [dependencies](evidence/db-010-artifact-dependencies.md) |
| Reliability and transaction controls | DB-011–DB-015 | [audit](evidence/db-011-audit-events.md), [idempotency](evidence/db-012-idempotency-records.md), [outbox](evidence/db-013-outbox-messages.md), [transaction manager](evidence/db-014-transaction-manager.md), and [optimistic concurrency](evidence/db-015-optimistic-concurrency.md) |
| Authentication and administrative authorization | IAM-001–IAM-004 | [OIDC verification](evidence/iam-001-oidc-jwt-verification.md), [claim mapping](evidence/iam-002-claim-mapping.md), [administrative authorization](evidence/iam-003-administrative-authorization.md), and [local development authentication](evidence/iam-004-local-development-authentication.md) |
| Publication application boundary | PUB-001–PUB-005 | [Publisher API](evidence/pub-001-publisher-api.md), [Namespace API](evidence/pub-002-namespace-api.md), [delegation and ownership](evidence/pub-003-namespace-delegation.md), [Artifact API](evidence/pub-004-artifact-api.md), and [ArtifactVersion registration seam](evidence/pub-005-artifact-version-registration.md) |
| PostgreSQL exit properties | DB-016–DB-021 | [migration from empty](evidence/db-016-migration-from-empty.md), [tenant isolation](evidence/db-017-tenant-isolation.md), [concurrent registration](evidence/db-018-concurrent-registration.md), [registration rollback](evidence/db-019-partial-registration-rollback.md), [idempotency/outbox replay](evidence/db-020-concurrent-idempotency-outbox-replay.md), and [identity properties](evidence/db-021-artifact-version-identity-properties.md) |

## Exit verification

The Docker-backed suite uses the repository-pinned PostgreSQL 18.6 image and a
non-superuser, non-`BYPASSRLS` service role for application behavior. Together,
its tests prove the exit properties in `PLAN.md`:

| Exit property | Proof |
| --- | --- |
| Immutability | Database guards reject ArtifactVersion updates/deletes, and deterministic generated cases prove that content and descriptor identities remain byte-for-byte unchanged after attempted replacement. |
| Tenant isolation | Every implemented repository is exercised with two populated tenants through explicit tenant predicates, forced RLS, and tenant-consistent foreign keys. |
| Namespace ownership | Repository and application tests prove verified ownership, strict child-prefix delegation, longest-prefix selection, revocation fallback, and fail-closed inactive-owner behavior. |
| Concurrency | Eight-connection races produce one Namespace, Artifact, semantic-version binding, and digest identity; Publisher state uses an explicit optimistic-concurrency precondition. |
| Rollback | Injected registration failures leave no partial idempotency, ArtifactVersion, ArtifactSource, or audit state, and the same request can then succeed exactly once. |
| Replay | Concurrent idempotency callers converge on one result, while outbox retries preserve immutable event identity and allow only one live lease owner. |

The phase-close change was verified with:

```text
GOTOOLCHAIN=go1.26.7 make test-migrations
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

`make test-migrations` exercised migration from empty, repository behavior, and
all Phase 2 database exit properties against disposable real PostgreSQL. The
aggregate gate covered formatting, vet, Staticcheck, unit and race tests,
repository hygiene, dependency vulnerability/license policy, OpenAPI/schema
validation, contract validation, and binary builds.

## Scope boundary

Phase 2 does not implement OCI resolution or fetching, descriptor
canonicalization and kind-specific validation, evidence verification, catalogs,
promotion, lifecycle, dependency resolution, federation, or runtime
authorization. The ArtifactVersion application service is intentionally the
trusted persistence seam behind the later hostile-input inspection pipeline; it
is not a public shortcut around that pipeline. PostgreSQL remains authoritative
for marketplace control metadata, while external registries retain artifact
bytes.

## Exit assessment

DB-001 through DB-021, IAM-001 through IAM-004, and PUB-001 through PUB-005 are
implemented and have item-level evidence. The real PostgreSQL suite proves every
Phase 2 exit property, and the aggregate repository gate passes. Phase 2
therefore meets its documented exit condition, and Phase 3 may begin at OCI-001
without changing the component boundary or the immutable-identity and
non-expansion guarantees established by accepted ADRs and versioned contracts.
