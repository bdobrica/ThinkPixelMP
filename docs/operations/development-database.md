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

Inspect the current migration state through the explicit migration entry point:

```sh
make migrate
```

Stop the dependency with `make postgres-down`. To also remove the disposable data
volume, run `docker compose down --volumes` explicitly. Removing the volume deletes
local database state.

The service process never runs migrations automatically. DB-001 will add the
executable migration framework and initial schema while preserving this command
surface. Production uses separate service and administrative migration identities
as required by the accepted persistence ADR; the single Compose identity is only
for the pre-schema development dependency.
