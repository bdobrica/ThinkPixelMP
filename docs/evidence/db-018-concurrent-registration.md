# DB-018 concurrent registration evidence

Date: 2026-09-06

## Outcome

The Docker-backed PostgreSQL integration suite now races eight simultaneous
repository registrations over independent connections using a
`NOSUPERUSER NOBYPASSRLS` service role.

| Identity race | Contender inputs | Durable outcome |
| --- | --- | --- |
| Namespace | Distinct UUIDv7 IDs, one tenant-local canonical path | One Namespace commits; seven calls return `namespace.conflict`. |
| Artifact | Distinct UUIDv7 IDs, one tenant-local `{namespace}/{name}` | One Artifact commits; seven calls return `artifact.conflict`. |
| ArtifactVersion semantic version | Distinct UUIDv7 IDs and digests, one Artifact and semantic version | One immutable version binding commits; seven calls return `artifact_version.conflict`. |
| ArtifactVersion digest | Distinct UUIDv7 IDs and semantic versions, one tenant-local digest | One digest identity commits; seven calls return `artifact_version.conflict`. |

Every race uses a start barrier after all contenders are ready. Post-race
repository lookups and lists prove that only one winner remains for each unique
identity. The audit count proves that losing transactions leave neither domain
rows nor mutation audit records. The test retains existing transaction-local
tenant scoping and exercises the database uniqueness constraints under actual
contention rather than sequential duplicate insertion.

No migration, production repository behavior, public API, cross-component
contract, or dependency changed.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/concurrent_identity_registration' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 go test -count=3 -race -tags=dbintegration -run 'TestPostgres/concurrent_identity_registration' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
