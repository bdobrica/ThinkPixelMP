# DB-013 OutboxMessage evidence

Date: 2026-09-06

## Implemented boundary

- `000013_outbox_messages.sql` adds forced-RLS, tenant-scoped outbox storage and
  a transaction-local tenant sequence allocator. Every message has a stable
  UUIDv7 event ID and strictly increasing tenant-local sequence.
- Exact bounded MarketplaceEvent v1 bytes and their SHA-256 digest are stored
  immutably. The domain constructor checks envelope/metadata agreement, tenant
  identity, the closed event-family union, required typed fields, and per-family
  field allowlists so arbitrary request data cannot become outbox content.
- Worker claims are ordered and use `FOR UPDATE SKIP LOCKED`, bounded leases,
  bounded attempts, and a UUIDv7 claim token. Expired leases can be reclaimed;
  an older token cannot retry, deliver, or dead-letter a newer claim.
- Retry records contain only a stable reason code and a next-attempt time no more
  than 24 hours after the claim. Expired claims at the 1,000-attempt ceiling are
  dead-lettered with a stable reason without losing the original event.
- Successful delivery and dead-letter transitions retain the immutable event and
  enforce the documented minimum retention windows of 30 and 90 days.
- `NextSequence` and `Record` deliberately use a caller-supplied transaction so
  future application transaction composition can atomically write domain, audit,
  idempotency, and outbox state. Worker-facing get/claim/retry/deliver/dead-letter
  operations remain tenant scoped and map failures to stable typed errors.

DB-013 does not add the sink worker, HTTPS adapter, SSE endpoint, retention job,
or the general transaction manager. It does not emit incomplete events from the
current persistence-only aggregate repositories. Those integrations remain
sequenced work, including DB-014, RESL-005, and later lifecycle/promotion items.
No public API or versioned schema changed.

## Verification

- workspace-local-cache `GOTOOLCHAIN=go1.26.7 go test ./internal/domain/outbox ./internal/adapters/postgres/outbox ./internal/adapters/postgres/migration`
- workspace-local-cache `GOTOOLCHAIN=go1.26.7 make test-migrations`
- workspace-local-cache `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
