# DB-021 ArtifactVersion identity property evidence

Date: 2026-09-06

## Outcome

The Docker-backed PostgreSQL integration suite now uses deterministic generative
testing to register 32 distinct ArtifactVersions and normalized descriptors. For
each generated identity it attempts to replace:

- the registered content digest;
- the descriptor digest; and
- the descriptor digest, exact normalized bytes, and equivalent JSON projection
  together with another internally consistent descriptor.

The mutations run through an administrative PostgreSQL connection so rejection
proves the database immutability guards rather than the service role's absence of
`UPDATE` permission. After every rejected mutation, tenant-scoped repository reads
prove that the original content digest, descriptor digest, and exact normalized
descriptor bytes remain unchanged. Generated inputs use a fixed seed so failures
are reproducible, and the test adds no dependency.

No migration, production behavior, public API/schema, accepted ADR, dependency,
or cross-component boundary changed.

## Verification

- `GOTOOLCHAIN=go1.26.7 go test -count=1 -race -tags=dbintegration -run 'TestPostgres/artifact_version_identity_properties' ./internal/adapters/postgres/migration`
- `GOTOOLCHAIN=go1.26.7 make test-migrations`
- `GOTOOLCHAIN=go1.26.7 make verify`
- `git diff --check`
