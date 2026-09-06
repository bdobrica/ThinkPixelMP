# DB-016 migration-from-empty evidence

Date: 2026-09-06

## Outcome

The PostgreSQL integration suite now has an explicit migration-from-empty
contract against the repository-pinned `postgres:18.6-bookworm` image. It proves
that read-only status leaves a new database empty, the complete shipped migration
set applies atomically, every expected table exists, every tenant-owned table has
enabled and forced RLS, the ledger contains one row per shipped migration, and a
repeated upgrade is a no-op with an unchanged valid status.

The Docker-backed migration gate now runs as a separate bounded GitHub Actions
job. It uses no application or operator database credential, exposes PostgreSQL
only through a random loopback port, removes the disposable container, retains
the workflow's read-only repository permission, and uses immutable action pins.
Ordinary `make verify` remains Docker-independent for local development.

No migration, production database identity, public API, cross-component
contract, or dependency changed.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/migration_from_empty' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
