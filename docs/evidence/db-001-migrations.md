# DB-001 migration framework and tenant schema

Date: 2026-09-05
Implementation commit: working tree (not committed).

## Outcome

The explicit migration command now supports read-only `status` and atomic `up`.
It embeds sequential forward-only SQL, validates SHA-256 history, rejects drift
and unknown/gapped history, and serializes upgrades with a transaction advisory
lock. The ledger and all pending SQL share one transaction. Connection, lock,
statement, and operation deadlines bound execution; database errors are redacted.
A separate environment/file secret reference supplies administrative credentials.

The initial schema creates only the UUIDv7 tenant reference root and timestamp.
It enables and forces RLS with transaction-local tenant context and seeds no tenant,
login, or grants. Identity mapping, tenant provisioning, aggregate repositories,
audit/outbox APIs, and broader DB-016 tests remain later work. No public contract
or ThinkPixel ownership boundary changed.

## Dependency review

Owner: ThinkPixelMP maintainers. The adapter and command pin the MIT-licensed
`github.com/jackc/pgx/v5` driver at v5.9.2. The small migration runner uses pgx
transactions directly, avoiding a second framework dependency and keeping SQL
history under this repository's control. Replacement is confined to the PostgreSQL
adapter and composition root; no driver types enter domain packages.

The [upstream changelog](https://github.com/jackc/pgx/blob/v5.9.2/CHANGELOG.md)
records PostgreSQL 18/protocol support, malformed-server-message hardening in
v5.9.0, and the SQL placeholder sanitization fix in v5.9.2 (GO-2026-5004). Public Go checksum verification binds
the selected sources; this is not a claim of independent upstream build-attestation
verification. `go mod graph`, `go mod why`, the module requirements, changelog,
and the pgservicefile parser source were reviewed. New runtime imports come from
pgx, pgpassfile v1.0.0, and pgservicefile at its exact upstream pseudo-version;
existing selected x/text and other shared versions are unchanged.

The driver can read operator PostgreSQL environment/service/password files and
connect to the operator-selected database. It remains confined to trusted
administrative state. It is never configured by marketplace metadata. No tracing
callback is installed, and raw driver errors are not printed. pgservicefile has no
tagged release, so an exact policy exception expires on 2026-12-04 with a removal
plan. Source allowlists and license/vulnerability requirements are unchanged.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-go-cache GOTOOLCHAIN=go1.26.7 make test-migrations`
  passed against a fresh PostgreSQL 18.6 container. Includes command-line
  status/up/repeat, empty read-only status, transactional rollback, four concurrent
  runners, checksum/unknown/gapped history rejection, UUID version/variant checks,
  and tenant reads/writes under a non-superuser, non-bypass role. Context resets
  after rollback and does not expose another tenant.
- The initial aggregate scan found GO-2026-5004 in pgx v5.9.1; the driver was upgraded to the patched v5.9.2. No vulnerability exception was added.
- `GOCACHE=/tmp/thinkpixelmp-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed:
  formatting, vet/Staticcheck, unit/race tests, tracked-file hygiene, dependency
  policy, vulnerability scan (no findings), license checks, OpenAPI/schema checks,
  and builds. Existing non-fatal OpenAPI duplicate-schema bundling warnings remain.
  Complete module-graph checksums were populated after the initial read-only
  dependency check identified missing entries.
- `git diff --check` passed.

The Docker suite has an explicit build tag and is invoked by `make test-migrations`;
ordinary `make verify` does not silently claim live database coverage.
