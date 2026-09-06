# DB-020 concurrent idempotency and outbox replay evidence

Date: 2026-09-06

## Outcome

The Docker-backed PostgreSQL integration suite now exercises concurrent replay
through eight independent connections using a `NOSUPERUSER NOBYPASSRLS` service
role.

| Contended operation | Concurrent outcome | Durable replay invariant |
| --- | --- | --- |
| Idempotency acquire | Eight identical ownership/request tuples produce one pending record; exactly one caller reports creation. | Every caller observes the same stable record identity and request digest. |
| Idempotency completion | Eight matching completions race over that pending record and all succeed. | Every caller receives the same completed resource result; the one-way transition is not duplicated or rewritten. |
| Initial outbox claim | Eight workers race for one pending event and exactly one receives it. | A delivery attempt has one lease owner. |
| Retry claim | After the winner records a retry, eight workers race again and exactly one receives the event. | The replay increments the attempt count while preserving the event ID, tenant sequence, exact payload, and payload digest. The earlier lease token is stale and cannot deliver the replayed claim. |

The test uses repository APIs and real PostgreSQL locking, uniqueness, forced RLS,
and transition guards. It proves the concurrent idempotency and at-least-once
outbox replay behavior required for the Phase 2 database exit without adding a
sink worker, changing a migration, or changing a public or cross-component
contract.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/concurrent_idempotency_outbox_replay' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 go test -count=3 -race -tags=dbintegration -run 'TestPostgres/concurrent_idempotency_outbox_replay' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
