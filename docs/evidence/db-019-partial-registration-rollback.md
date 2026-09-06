# DB-019 partial registration rollback evidence

Date: 2026-09-06

## Outcome

The Docker-backed PostgreSQL integration suite now invokes the trusted
ArtifactVersion registration persistence seam through the real application
service, PostgreSQL transaction manager, and tenant-scoped repositories under a
`NOSUPERUSER NOBYPASSRLS` service role.

| Injected checkpoint | Writes completed before failure | Verified rollback |
| --- | --- | --- |
| ArtifactSource persistence | Idempotency claim, ArtifactVersion, and version audit | No idempotency record, ArtifactVersion, ArtifactSource, or new audit event remains. |
| Idempotency completion | Idempotency claim, ArtifactVersion, ArtifactSource, and both mutation audits | No idempotency record, ArtifactVersion, ArtifactSource, or new audit event remains. |

Each injected error is preserved by the transaction boundary. After rollback,
the test retries the same canonical request with the same idempotency key and
proves that registration commits exactly one version, its matching immutable
source, a completed idempotency result, and the two required mutation audit
facts. This covers the atomic registration guarantee in
`docs/contracts/artifact-identity.md` without changing production behavior,
database schema, public API, or a cross-component contract.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/partial_registration_rollback' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
