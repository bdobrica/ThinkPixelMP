# ENG-012 PostgreSQL development and migration evidence

Date: 2026-08-31

Implementation commit: `4b7d3e0`

## Result

The repository now provides a pinned PostgreSQL `18.6-bookworm` development
dependency through Docker Compose. It binds to loopback on configurable host port
`15432`, requires an externally supplied disposable password, persists in a named
development volume, and exposes a bounded readiness check.

The stable Makefile surface adds `postgres-up`, `postgres-down`, and `migrate`.
`cmd/migrate` is now executable and explicitly reports the empty migration set.
It cannot mutate schema before DB-001 introduces executable immutable migrations.
Ordinary service startup remains separate and never migrates the database.

## Acceptance evidence

- `TPMP_POSTGRES_PASSWORD=<redacted> docker compose config --quiet` passed.
- `TPMP_POSTGRES_PASSWORD=<redacted> docker compose up --detach --wait postgres`
  reached healthy state.
- `docker compose exec -T postgres psql ... --command 'SHOW server_version;'`
  reported `18.6 (Debian 18.6-1.pgdg12+2)`.
- `GOCACHE=/tmp/thinkpixelmp-eng012-go-cache GOTOOLCHAIN=go1.26.7 make migrate`
  reported no executable migrations.
- `GOCACHE=/tmp/thinkpixelmp-eng012-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./cmd/migrate/...`
  passed.
- `GOCACHE=/tmp/thinkpixelmp-eng012-go-cache GOTOOLCHAIN=go1.26.7 make verify`
  passed, including format, static analysis, unit/race, dependency, vulnerability,
  license, OpenAPI/contract, and binary-build gates.
- `git diff --check` passed.

The smoke-test container, network, and named volume were removed after verification.
No database credential or database state was retained in the repository evidence.
