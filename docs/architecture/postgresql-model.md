# PostgreSQL logical model

PostgreSQL is authoritative for marketplace control metadata. OCI registries and object stores retain artifact/report bytes; caches and search indexes are derived.

## Tenant isolation

Every tenant-owned table carries `tenant_id`. Tenant-scoped natural and surrogate uniqueness includes `tenant_id`, and cross-table references use tenant-consistent composite foreign keys. Application repositories require an authenticated tenant context and cannot expose unscoped methods.

Production enables PostgreSQL row-level security as defense in depth. The service database role cannot bypass RLS. Migration, repair, and narrowly scoped administrative identities are separate, audited, and unavailable to normal request handling.

## Logical aggregates

| Area | Principal records |
| --- | --- |
| Identity | Tenant reference, Publisher, PublisherStateRecord, Namespace, NamespaceDelegation |
| Artifact | Artifact, ArtifactVersion, ArtifactSource, ArtifactDescriptor, ArtifactRequirement, ArtifactDependency |
| Evidence | EvidenceProducer, EvidenceRecord, signature/provenance/SBOM and category summaries |
| Catalog | Catalog, PolicyBundle, PolicyActivation, CatalogEntry, CatalogEntryStateRecord |
| Promotion | PromotionRequest, PolicyEvaluation, PromotionReview, PromotionDecision |
| Resolution | ArtifactLock, ArtifactLockNode/Edge, ArtifactResolution, ResolutionEvidenceSnapshot |
| Lifecycle | Deprecation, QuarantineRecord, Revocation, RevocationCorrection |
| Federation | ImportSource, ImportRecord, remote endpoint observations |
| Reliability | IdempotencyRecord, AuditEvent, OutboxMessage |

## Immutability and transactions

Database constraints supplement domain validation for digest/version immutability, tenant consistency, namespace and version uniqueness, evidence exact-subject binding, terminal promotion decisions, catalog history, lock/resolution immutability, and append-only revocation.

Every authoritative mutation writes its domain record, AuditEvent, and OutboxMessage in one transaction. Consumers are at-least-once and deduplicate stable event IDs. Outbox claiming uses bounded leases/retries and dead-letter metadata without losing the original event.

The Phase 0 model is logical rather than executable SQL. Migrations, indexes, partitioning, retention, and concurrency tests begin in Phase 2.
