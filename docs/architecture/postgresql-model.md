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

DB-001 implements the tenant reference root and explicit transactional migration
runner. DB-002 implements the Publisher aggregate with tenant-local slug
uniqueness, append-only state records, and repository-enforced transaction-local
tenant scope. DB-003 implements immutable Namespace ownership roots with canonical
tenant-local path uniqueness, verified tenant-consistent Publisher ownership, and
the same repository/RLS scoping. PUB-003 adds immutable delegation identity,
append-only active/revoked history, verified tenant-local recipients, and live
longest-prefix publication ownership resolution that fails closed for an
inactive controlling Publisher. DB-004 implements the
logical Artifact root with tenant-consistent Namespace membership, canonical
tenant-local identity uniqueness, a fixed V1 kind, bounded discovery metadata,
and repository/RLS scoping. DB-005 implements the ArtifactVersion identity
root with strict semantic-version and SHA-256 digest uniqueness, tenant-consistent
Artifact and Publisher attribution, fixed-kind consistency, taxonomy-compatible
class/delivery metadata, initial lifecycle, and repository/RLS scoping. DB-006
makes registered ArtifactVersion rows database-immutable, including their exact
digest and payload metadata, and prevents deletion. DB-007 implements immutable,
tenant-scoped ArtifactSource values that preserve submitted OCI, remote, or import
metadata while binding the resolved source reference and SHA-256 digest to the
parent ArtifactVersion and its delivery model. DB-008 implements one immutable,
tenant-scoped ArtifactDescriptor per ArtifactVersion. It preserves the exact
bounded normalized bytes used for the separately computed descriptor digest,
stores their equivalent JSONB representation, and binds the V1 media type, kind,
and repeated artifact coordinates to the parent ArtifactVersion. Kind-specific
descriptor validation and RFC 8785 canonicalization remain in Phase 3; the
persistence boundary accepts only bytes already normalized by trusted code.
DB-009 implements one immutable, tenant-scoped ArtifactRequirement per persisted
descriptor. It preserves exact normalized requirement bytes and their digest,
stores an equivalent JSONB projection, validates the closed V1 declaration
vocabulary, and requires the projection to equal the descriptor's `requirements`
member. Requirements remain declarations and cannot grant Run authority.
DB-010 implements immutable, ordered, tenant-scoped ArtifactDependency rows. It
preserves exact normalized declaration bytes and digests, stable names, logical
targets, required/optional flags, optional source/catalog context, and the exact
typed selector. Each row is bound to its declaration index in the parent
descriptor; dependency names are unique within that parent. These records are
resolution inputs only and cannot grant authority. DB-011 implements immutable,
tenant-scoped AuditEvent facts with minimized actor, action, opaque resource,
digest/reference, decision/reason, and request/trace correlation fields. Existing
authoritative mutation repositories write their audit fact in the mutation
transaction, while deferred database triggers reject commits without the matching
fact. Outbox sink delivery remains later application work. DB-012
implements tenant-scoped IdempotencyRecord ownership using the contract tuple of
tenant, principal, action, and key. Each tuple binds one canonical request digest,
can establish one bounded HTTP status and optional opaque resource identity, and
cannot be repurposed or rewritten. Records have a minimum 24-hour retention
window, forced RLS, and an expiry index; cleanup remains an operator-policy
concern. Transactional coupling to domain mutations and concurrent replay stress
tests remain DB-014 and DB-020.
DB-013 implements the tenant-scoped transactional OutboxMessage store. It
preserves exact bounded MarketplaceEvent bytes and their digest under a stable
UUIDv7 event identity, allocates strictly increasing tenant-local sequences in
the caller's transaction, and exposes ordered lease-token claims. Bounded retry
delay and attempt metadata, stale-claim rejection, expired-lease reclamation,
automatic attempt-exhaustion dead-lettering, and 30/90-day delivered/dead-letter
retention minima support at-least-once delivery without changing or losing the
logical event. Event construction accepts only the versioned MarketplaceEvent v1
families and allowlisted payload fields. The sink worker, SSE surface, retention
job, and coupling to later event-producing mutations remain later work. DB-014
adds a vendor-neutral transaction-manager port and a PostgreSQL implementation.
Repository calls made with its callback context join the same tenant-bound
transaction through savepoints; standalone calls preserve their existing
transaction ownership. Callback errors roll back all composed work, nested
composition cannot change tenant scope, and no pgx transaction type crosses into
application contracts. The OutboxMessage writer requires this transaction context
so sequence allocation and immutable event insertion cannot commit separately.
DB-015 makes the Publisher state version an explicit repository precondition,
which is the mutable administrative aggregate implemented so far. State changes
compare the caller's expected version with the loaded aggregate and conditionally
advance `current_state_version`; a competing or stale change fails with the stable
`publisher.stale_state_version` code and cannot append history, audit a mutation,
or overwrite the winner. The application HTTP boundary will translate this
stable conflict into the contract's strong ETag and `412 Precondition Failed`
behavior when publisher administration is implemented. Immutable creates,
append-only actions, and mutable delivery/idempotency machinery are deliberately
outside this administrative optimistic-concurrency rule. Later mutable
administrative aggregates must expose and condition updates on their own
monotonic version when they are implemented.
See
[migration files](../../migrations/README.md) and [database
operations](../operations/development-database.md). The remaining aggregates,
indexes, partitioning, and retention are still a logical model sequenced in Phase
2 and later.
