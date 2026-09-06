# DB-012 IdempotencyRecord evidence

Date: 2026-09-06

## Implemented boundary

- `000012_idempotency_records.sql` adds forced-RLS, tenant-scoped idempotency
  storage whose unique owner is `(tenant, principal, action, key)` and whose
  canonical SHA-256 request digest cannot be changed.
- Records start pending and permit exactly one database-guarded completion with
  a bounded HTTP response status and optional paired resource type/opaque ID.
  Repeated identical completion returns the established result; different request
  content or a different result is a conflict.
- The schema enforces UUIDv7 identities, bounded non-control ownership values,
  internally consistent state/result fields, and at least 24 hours between
  creation and expiry. An expiry index supports future policy-driven cleanup.
- The domain package validates records and results. The PostgreSQL repository
  exposes tenant-scoped acquire, get, and complete operations. Persistence errors
  expose only stable typed reason codes and never echo the idempotency key.

DB-012 does not add HTTP middleware, execute retention cleanup, or couple the
idempotency claim to domain/audit/outbox writes. The public HTTP contract is
unchanged. General transaction composition and concurrent replay stress tests
remain DB-014 and DB-020.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-db012-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/idempotency ./internal/adapters/postgres/idempotency ./internal/adapters/postgres/migration ./cmd/migrate`
- `GOCACHE=/tmp/thinkpixelmp-db012-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOCACHE=/tmp/thinkpixelmp-db012-go-cache GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
