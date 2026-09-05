# DB-002 Publisher persistence

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The Publisher domain validates tenant and publisher UUIDv7 identifiers,
contract-compatible slugs and bounded metadata, UTC timestamps, the four V1
states, and the accepted transition graph. New publishers start in `claimed` at
state version 1. Revoked is terminal, and every later transition requires a stable
reason code.

The PostgreSQL schema stores tenant-scoped Publisher rows and append-only state
records. Composite keys and foreign keys preserve tenant consistency; slugs are
unique only within a tenant. Database triggers enforce initial state, legal
transitions, required reasons, immutable identity metadata, forward-only current
state, and immutable history. A deferred composite foreign key prevents a
Publisher from pointing at a missing current state. Both relations enable and
force RLS.

The PostgreSQL adapter implements create, read by ID/slug, bounded ordered list,
and state transition operations behind the Publisher repository contract. Every
operation opens a transaction, applies the validated tenant UUID with
transaction-local `set_config`, and repeats the tenant in its predicates. It maps
expected conflicts/not-found/invalid inputs to stable typed errors and does not
expose raw database errors. Audit/outbox coupling and strong ETag concurrency are
not weakened or preempted; they remain DB-011 and DB-015.

No public API or cross-component contract changed, and no new dependency was
introduced.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db002-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against the suite-owned PostgreSQL 18.6 container. It covered migration
  from empty/repeat/concurrency/rollback plus Publisher create/read/list, same-slug
  isolation across two tenants, duplicate conflict mapping, RLS denial of a
  cross-tenant lookup, valid and invalid state transitions, and database rejection
  of state-record mutation.
- `GOCACHE=/tmp/thinkpixelmp-db002-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed.
- `git diff --check` passed.

The first sandboxed Docker attempt could not access the Docker socket. The same
documented command was rerun with approved Docker access and passed; no
operator-supplied database or persistent development volume was used.
