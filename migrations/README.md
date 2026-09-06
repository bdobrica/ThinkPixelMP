# Database migrations

`cmd/migrate` embeds these SQL files. Only that explicit command upgrades the
database; ordinary service startup never changes its schema.

Name forward migrations `NNNNNN_description.sql`, starting at `000001` without
gaps. Released files are immutable: add a new migration for repairs or compatible
schema evolution. Down migrations and automatic data deletion are not supported.
SQL must be transactional and must not contain transaction control or commands
such as `CREATE INDEX CONCURRENTLY` that cannot run inside a transaction.

The PostgreSQL adapter validates the names and SHA-256 checksums against
`public.schema_migrations`. Missing, changed, or unknown applied migrations fail
closed. `up` serializes with a transaction-scoped advisory lock and commits all
pending SQL and ledger entries in one transaction. Any failure rolls back the
whole pending batch. `status` uses a read-only snapshot and never creates the ledger.

`000001_tenants.sql` creates the tenant reference root with an explicit UUIDv7
`tenant_id` primary key, creation timestamp, and forced row-level security. It
creates no default tenant, login, identity mapping, or service grants. Tenant
provisioning and authenticated repository operations belong to later identity and
persistence items.

`000002_publishers.sql` creates tenant-scoped publishers and append-only publisher
state records. It enforces UUIDv7 identifiers, tenant-local slug uniqueness, the
V1 state-transition graph, bounded metadata, a deferred current-state reference,
and forced RLS on both relations. It creates no publisher, role, or grant.

`000003_namespaces.sql` creates immutable tenant-scoped namespace ownership roots.
It enforces UUIDv7 identifiers, canonical V1 paths, one owner per tenant-local
path, tenant-consistent Publisher references, verified Publisher ownership at
creation, and forced RLS. Delegation records and longest-prefix authorization
remain separate later work. It creates no namespace, role, or grant.

`000004_artifacts.sql` creates tenant-scoped logical Artifact identities. It
enforces UUIDv7 identifiers, tenant-consistent Namespace references, canonical
artifact names, tenant-local `{namespace}/{name}` uniqueness, the closed V1
artifact-kind vocabulary, bounded discovery metadata and labels, fixed identity
and kind, and forced RLS. ArtifactVersion, digest, source, descriptor, lifecycle,
publication authorization, and audit/outbox records remain separate later work.
It creates no Artifact, role, or grant.

`000005_artifact_versions.sql` creates tenant-scoped ArtifactVersion identity
records. It enforces UUIDv7 identifiers, the complete strict SemVer key,
canonical SHA-256 content digests, one version key per logical Artifact, one
ArtifactVersion identity per tenant-local digest, tenant-consistent Artifact and
Publisher attribution, matching fixed Artifact kind, the closed V1 artifact-class,
delivery-model, and lifecycle vocabularies, taxonomy-compatible combinations,
and forced RLS. New repository registrations are active. Source and
resolved-reference metadata, descriptors, requirements, dependencies,
publication authorization, and audit/outbox records remain separate later work.
It creates no ArtifactVersion, role, or grant.

`000006_artifact_version_mutation_guards.sql` makes registered ArtifactVersion
rows immutable at the database boundary. It rejects every update and deletion,
protecting the exact digest, semantic-version binding, payload classification,
attribution, identity, registration time, and initial lifecycle value even for
callers outside the registration-only repository. Append-only lifecycle records,
source metadata, and audit/outbox coupling remain separate later work.

`000007_artifact_sources.sql` creates one immutable, tenant-scoped ArtifactSource
value per ArtifactVersion. It preserves the submitted `oci`, `remote`, or
`import-record` source metadata and a bounded resolved reference, binds the
resolved SHA-256 digest and delivery model to the exact parent ArtifactVersion,
requires OCI resolved references to contain that digest, enforces HTTPS remote
endpoints, and applies forced RLS. Updates and deletions are rejected. Import
history, remote endpoint evidence, source fetching, descriptors, and audit/outbox
coupling remain separate later work.

`000008_artifact_descriptors.sql` creates one immutable, tenant-scoped
ArtifactDescriptor per ArtifactVersion. It stores a canonical SHA-256 descriptor
digest, the exact normalized JSON bytes under a 1 MiB compiled ceiling, and an
equivalent JSONB representation. Composite references and common-envelope checks
bind schema version, kind, media type, namespace/name/version coordinates, and the
parent ArtifactVersion. Forced RLS and update/delete guards protect the record.
Kind-specific descriptor validation, canonicalization, requirements,
dependencies, and audit/outbox coupling remain separate sequenced work.

`000009_artifact_requirements.sql` creates one immutable, tenant-scoped
ArtifactRequirement per persisted ArtifactDescriptor. It stores the exact
normalized requirement bytes, their SHA-256 digest, and an equivalent JSONB
projection under the descriptor's 1 MiB ceiling. A database trigger requires the
projection to equal the parent descriptor's `requirements` member. Closed V1
envelope checks, forced RLS, and update/delete guards protect the record. These
declarations remain compatibility and policy input only; they do not grant
runtime authority. Dependency persistence and audit/outbox coupling remain
separate sequenced work.

`000010_artifact_dependencies.sql` creates immutable, ordered, tenant-scoped
ArtifactDependency declarations under each persisted ArtifactDescriptor. Each
row preserves exact normalized bytes, their SHA-256 digest, the equivalent JSONB
projection, stable dependency name, logical target, required/optional flag,
optional catalog/source context, and exactly one typed selector. Declaration
indexes and names are unique within a parent, and a database trigger binds every
row to the same array element in the parent descriptor. Forced RLS and
update/delete guards protect the declarations. Dependency resolution, locks, and
audit/outbox coupling remain separate sequenced work.

`000011_audit_events.sql` creates immutable, tenant-scoped AuditEvent facts with
database-generated UUIDv7 identities, verified actor identity, stable
action/resource/decision/reason fields, exact artifact and evidence/policy
references, and request/trace correlation. It excludes arbitrary request metadata
and free-form explanations. Existing mutation repositories record their audit fact
before committing, and deferred constraint triggers reject Publisher, Namespace,
Artifact, ArtifactVersion, ArtifactSource, ArtifactDescriptor,
ArtifactRequirement, and ArtifactDependency mutations that lack the matching
same-transaction audit event. Forced RLS protects reads and inserts; update/delete
guards make the trail append-only. Tenant provisioning audit, outbox sink
delivery, and the general transaction manager remain DB-014 and later application
work.

`000012_idempotency_records.sql` creates tenant-scoped IdempotencyRecord ownership
for mutating requests. A unique `(tenant, principal, action, key)` tuple binds one
canonical SHA-256 request digest and progresses only once from pending to a
completed HTTP status plus optional opaque resource identity. Reuse cannot rewrite
the request digest or established result. Forced RLS protects all access, the
schema enforces at least 24 hours of retention, and an expiry index supports a
future policy-driven cleanup job. The repository claims or reads an existing key,
rejects different-content reuse, and completes identical requests idempotently.
Coupling a claim, domain mutation, audit, and outbox record through the general
transaction manager remains DB-014; concurrent stress coverage remains
DB-020.

`000013_outbox_messages.sql` creates the tenant-scoped transactional outbox and
its per-tenant sequence allocator. Exact bounded CloudEvent bytes and their
SHA-256 digest are immutable, UUIDv7 event IDs remain stable across delivery,
and sequences are allocated monotonically inside the caller's transaction.
Forced RLS protects both relations. Worker claims use bounded leases, attempt
counts, opaque UUIDv7 claim tokens, ordered `SKIP LOCKED` selection, and expired
lease reclamation. Retry scheduling accepts only bounded stable error codes and a
maximum 24-hour delay; delivery and dead-letter transitions enforce the default
30-day and 90-day retention minima. Exhausted expired claims are moved to
dead-letter state without altering or discarding the original event. The worker,
HTTP event sink, SSE delivery, retention deletion, and general transaction
manager remain later work.

See [database operations](../docs/operations/development-database.md) for command,
credential, RLS context, and test guidance.
