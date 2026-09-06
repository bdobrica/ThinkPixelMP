# Development PostgreSQL

ThinkPixelMP uses the PostgreSQL `18.6-bookworm` container image for its disposable
development dependency, matching the supported-version baseline. Docker Compose
publishes it only on `127.0.0.1:15432` and stores its data in a named development
volume. Set `TPMP_POSTGRES_PORT` when that development port is already occupied.
No database credential is committed: Compose requires `TPMP_POSTGRES_PASSWORD`
to be supplied by the developer and it must not be reused outside this disposable
service.

Start the dependency and wait for readiness:

```sh
export TPMP_POSTGRES_PASSWORD='<local development secret>'
make postgres-up
```

Configure a separate administrative connection secret. Set
`TPMP_MIGRATION_DATABASE_URL_REF=env:TPMP_MIGRATION_DATABASE_URL` and put the
PostgreSQL connection URL in that referenced environment variable, or use
`file:/absolute/path` pointing to an operator-managed secret file. The Compose
connection uses host `127.0.0.1`, port `15432` (or the configured override), database
`thinkpixelmp`, and user `thinkpixelmp_migrator`. URL-encode credential components.
Do not put credentials in command arguments, checked-in files, or evidence.
The migration command does not fall back to the service database credential.

Inspect and apply the embedded migration set:

```sh
make migrate
make migrate MIGRATE_ARGS=up
make migrate
```

Stop the dependency with `make postgres-down`. To also remove the disposable data
volume, run `docker compose down --volumes` explicitly. Removing the volume deletes
local database state.

The service process never runs migrations automatically. `status` is read-only,
including on an empty database. `up` validates the complete applied history,
serializes concurrent runners, and applies all pending SQL and history entries
atomically. History checksum drift, gaps, unknown migrations, SQL failures, and
connection failures return a nonzero exit code without printing raw database
errors or connection material. `help` requires no database configuration.

The command bounds connection establishment to five seconds, lock waits to ten
seconds, statements to sixty seconds, and the whole operation to two minutes.
Interrupts cancel the operation. A failed or cancelled pending batch rolls back;
inspect `status` before retrying if the connection was lost during commit. Repairs
use new forward migrations; back up authoritative state before production changes.
See the [migration authoring rules](../../migrations/README.md).

## Database identities and tenant scope

Production uses separate service and administrative migration identities as
required by [ADR 0007](../adr/0007-hostile-fetch-inspection-and-tenant-persistence.md).
The Compose identity is an administrative identity for disposable development,
not a service credential. Production provisioning owns login creation and grants;
migrations never create reusable credentials or grant permissions to `PUBLIC`.

`public.tenants` stores only a UUIDv7 tenant reference and its creation time. The
schema seeds no tenant or publisher and makes no claim about external
identity-provider membership. RLS is enabled and forced. The service role must be
`NOSUPERUSER NOBYPASSRLS`, with no permission to change policies, schema,
migration history, or role membership. The Publisher repository needs `SELECT`
on `tenants`, `SELECT, INSERT, UPDATE` on `publishers`, and `SELECT, INSERT` on
`publisher_state_records`; it neither updates nor deletes append-only state
records. The Namespace repository additionally needs `SELECT, INSERT` on
`namespaces` and the Publisher read permissions used by its verified-owner
database guard. It does not update or delete immutable namespace ownership roots.
Delegation operations additionally require `SELECT, INSERT, UPDATE` on
`namespace_delegations` and `SELECT, INSERT` on
`namespace_delegation_state_records`; updates are constrained to the single
active-to-revoked transition, and state records remain append-only.
The Artifact repository additionally needs `SELECT, INSERT` on `artifacts` and
`SELECT` on `namespaces`; it does not update or delete logical identity records.
The ArtifactVersion repository additionally needs `SELECT, INSERT` on
`artifact_versions` and `SELECT` on `artifacts` and `publishers`; it exposes no
update or delete operation. Its create operation registers only active versions.
The ArtifactSource repository additionally needs `SELECT, INSERT` on
`artifact_sources` and `SELECT` on `artifact_versions`; it exposes no update or
delete operation. The ArtifactDescriptor repository additionally needs `SELECT,
INSERT` on `artifact_descriptors` plus `SELECT` on `artifact_versions`, `artifacts`,
and `namespaces`; it exposes no update or delete operation. The
ArtifactRequirement repository additionally needs `SELECT, INSERT` on
`artifact_requirements` and `SELECT` on `artifact_descriptors`; it exposes no
update or delete operation. The ArtifactDependency repository additionally needs
`SELECT, INSERT` on `artifact_dependencies` and `SELECT` on
`artifact_descriptors`; it exposes ordered create/read/list operations and no
update or delete operation. All mutation repositories also require `INSERT` on
`audit_events`; audit readers require `SELECT`. No service repository receives
`UPDATE` or `DELETE` on the append-only audit trail. Grant only the table operations
needed by implemented repositories. The IdempotencyRecord repository requires
`SELECT, INSERT, UPDATE` on `idempotency_records`; its update is constrained to
one pending-to-completed transition. It exposes no deletion operation. A separate
authorized retention job may receive narrowly scoped deletion permission when
that job is implemented.
The OutboxMessage repository requires `SELECT, INSERT, UPDATE` on
`tenant_event_sequences` and `outbox_messages`. Event insertion and sequence
allocation use the caller's transaction; delivery-state updates are constrained
by lease tokens and database transition guards. It exposes no deletion operation.
A future sink worker receives these permissions only for configured tenants, and
a separate retention identity may receive narrowly scoped deletion permission.

Future repositories must derive tenant identity from verified authentication,
open a transaction, and set its local scope using a bound parameter:

```sql
SELECT set_config('thinkpixelmp.tenant_id', $1, true);
```

The parameter is the validated tenant UUID. Repositories must also filter by tenant
and use tenant-consistent keys. Missing or reset context sees no tenant rows;
cross-tenant reads and writes fail closed. Transaction-local settings avoid leaking
scope across pooled connections. The setting is defense in depth, not authentication:
SQL callers can set it, so normal request handling must never expose arbitrary SQL
or administrative connections. Mutation callers must also attach the verified
principal plus optional UUIDv7 request ID and W3C trace ID through the audit actor
context; request fields and forwarded headers are not authority. IAM claim mapping,
tenant provisioning APIs, and general transaction composition remain later work.

## Verification

```sh
make test-migrations
```

This explicit Docker gate creates and removes its own PostgreSQL `18.6-bookworm`
container, using an ephemeral loopback-only port and disposable trust authentication.
It never uses the Compose volume or an operator-supplied database. It tests empty
and repeated upgrades, read-only status, rollback, concurrency, checksum/unknown
history rejection, UUIDv7 validation, and RLS using a non-bypass service role.
It also exercises Publisher create/read/list/state transitions, tenant-local slug
reuse, cross-tenant read denial, duplicate conflict mapping, and append-only state
history. Namespace coverage includes create/read/list, canonical path validation,
same-path reuse across tenants, duplicate rejection within a tenant, verified and
tenant-consistent owner enforcement, cross-tenant read denial, and immutable
ownership attribution.
Delegation coverage includes strict-child and verified-recipient enforcement,
active-prefix and Namespace-root collision rejection, cross-tenant isolation,
append-only revocation/reassignment, stale-version rejection, and live
longest-prefix ownership resolution. Artifact coverage includes create/read/list,
canonical
logical identity and fixed-kind enforcement, same-identity reuse across tenants,
duplicate rejection within a tenant, tenant-consistent Namespace references,
bounded labels, cross-tenant read denial, and database rejection of identity,
kind, and deletion mutations. ArtifactVersion coverage includes create/read/list,
strict SemVer preservation, exact digest lookup and tenant-local uniqueness,
same-version/digest reuse across tenants, fixed Artifact-kind consistency,
taxonomy-compatible class/delivery combinations, Publisher attribution,
cross-tenant read denial, initial active lifecycle, and database rejection of
payload, digest, lifecycle, and deletion mutations. ArtifactSource coverage
includes all three source variants, exact parent-digest/delivery binding, resolved
OCI references, tenant isolation, and immutability. ArtifactDescriptor coverage
includes bounded exact normalized bytes, their JSONB equivalent, descriptor
digest binding, V1 kind/media-type and coordinate consistency, tenant isolation,
duplicate rejection, and immutable update/delete guards. ArtifactRequirement
coverage includes the closed V1 vocabulary, exact normalized-byte and digest
round trips, equality with the parent descriptor declaration, repeated digests
across tenants, tenant isolation, duplicate rejection, and immutable update/delete
guards. ArtifactDependency coverage includes all three selector forms, stable
declaration ordering, exact normalized-byte and digest round trips, equality with
the corresponding parent descriptor array element, repeated declarations across
tenants, tenant isolation, duplicate rejection, and immutable update/delete
guards.
Audit coverage includes required verified actor correlation, transaction rollback
when audit recording fails, immutable tenant-isolated reads, bounded structured
fields, and deferred database rejection of otherwise valid unaudited mutations.
Idempotency coverage includes tenant/principal/action/key ownership, canonical
request-digest mismatch rejection, replay of established results, cross-tenant
isolation, one-way completion guards, and the minimum retention window.
Outbox coverage includes transactional tenant-local sequence allocation, exact
event/digest preservation, ordered claims, retry timing, expired-lease reclaim,
stale-token rejection, dead-letter and successful-delivery metadata, tenant
isolation, immutable payload guards, and delivered/dead-letter retention minima.

Tenant-isolation coverage spans every implemented PostgreSQL repository. The
suite creates colliding logical identities in two tenants through a
`NOSUPERUSER NOBYPASSRLS` service role, then verifies tenant-filtered lookup and
list behavior. It also proves that cross-tenant Publisher and Namespace
delegation state changes, Idempotency completion, Outbox claiming/delivery, and
Artifact, ArtifactVersion, Source, Descriptor, Requirement, and Dependency
parent references fail closed. AuditEvent reads remain tenant-isolated, and
Outbox sequences remain tenant-local. These checks exercise repository tenant
predicates together with forced RLS and tenant-consistent database constraints.

Concurrent identity-registration coverage starts eight repository calls on
independent service-role connections at the same barrier. Namespace path and
Artifact logical-identity races each commit one winner and return stable
conflicts for the other seven contenders. ArtifactVersion races cover both
immutable identity constraints: the same semantic version with different
digests, and different semantic versions with the same digest. Each race leaves
one durable identity and one matching audit event, with no partial loser state.

The ordinary `make verify` gate runs unit checks without requiring Docker; run
both when changing migrations or PostgreSQL repositories.
