# DB-015 optimistic-concurrency evidence

Date: 2026-09-06
Implementation commit: `87b4c24`

## Implemented boundary

- Publisher verification state is the mutable administrative aggregate currently
  implemented. Its repository state-change operation now requires the caller's
  expected current `StateVersion`.
- The PostgreSQL adapter rejects non-positive preconditions, detects a stale
  version before mutation, and conditionally advances `current_state_version`
  using the same expected version. A concurrent state-record collision is also
  mapped to the stable `publisher.stale_state_version` conflict.
- A stale or losing request rolls back its candidate append and produces no
  mutation audit record. Exactly one competing transition can advance a given
  version.
- The later publisher HTTP application boundary can map the stable stale-version
  conflict to the published strong-ETag `412 Precondition Failed` response.
  Supplying and parsing `If-Match` remains part of that API implementation.
- Immutable creates and append-only administrative actions do not require this
  precondition. Operational idempotency and outbox delivery transitions retain
  their purpose-specific ownership/lease concurrency rather than adopting an
  administrative ETag contract.
- No migration, public API schema, external contract, or dependency changed.
  Future mutable administrative aggregates must add their own monotonic version
  and conditional repository update when their persistence is implemented.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test ./internal/domain/publisher ./internal/adapters/postgres/publisher ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 go test -count=3 -race -tags=dbintegration -run 'TestPostgres/publisher_repository' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`

The disposable PostgreSQL 18.6 coverage verifies a normal version-1 transition,
invalid and stale version rejection, and two concurrent valid transitions based
on version 1. Across repeated fresh-database runs, one transition committed at
version 2 and one returned `publisher.stale_state_version`; audit counts proved
that the losing request did not record a mutation.
