# DB-014 transaction manager and repository-interface evidence

Date: 2026-09-06

## Implemented boundary

- `internal/ports/transaction` defines a vendor-neutral transaction manager whose
  callback receives only `context.Context`; no pgx or PostgreSQL type crosses the
  application port.
- The PostgreSQL transaction manager establishes transaction-local tenant scope
  before invoking application work. A callback error is preserved and rolls back
  every participating repository operation; commit and persistence failures map
  to bounded typed errors.
- Existing tenant-scoped domain repository interfaces remain the application
  persistence boundaries. All PostgreSQL implementations now join the transaction
  carried by the callback context, using savepoints so their standalone commit and
  rollback behavior remains backward compatible.
- The outbox domain also exposes a transaction-required writer interface. Its
  PostgreSQL implementation allocates the tenant sequence and records immutable
  event bytes through the same callback context without exposing pgx.
- Nested transaction-manager calls use savepoints and cannot change tenant scope.
  Repository calls that attempt to join a transaction under another tenant fail
  before executing a database statement.
- Disposable PostgreSQL coverage composes Publisher, automatic AuditEvent,
  IdempotencyRecord, and OutboxMessage writes. It proves full rollback (including
  sequence allocation), successful commit, nested composition, callback-error
  identity, and cross-tenant nesting rejection.

DB-014 does not add new public API or schema, emit events for incomplete
application use cases, or implement optimistic concurrency. Those remain
sequenced work, including DB-015 and later publication/lifecycle/promotion tasks.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test ./internal/adapters/postgres/... ./internal/domain/... ./internal/ports/...`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
