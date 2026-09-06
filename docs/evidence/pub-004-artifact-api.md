# PUB-004 Artifact API evidence

- Date: 2026-09-06
- Scope: authenticated tenant-scoped logical Artifact create/read/list API

## Outcome

ThinkPixelMP now exposes a composable HTTP handler and application service for
`POST /v1/artifacts`, `GET /v1/artifacts/{artifact_id}`, and
`GET /v1/artifacts`. The transport strictly decodes bounded requests, accepts
only Bearer authorization syntax when credential material is present, rejects
tenant injection and unknown query fields, emits the existing Artifact v1
representation, and maps typed failures to safe RFC 7807 responses.

Create requires the exact IAM-003 `publication.publish` action. Within the same
tenant transaction it resolves the requested Namespace and its live controlling
Publisher, failing closed unless the root owner remains verified, then couples
the canonical idempotency claim, logical Artifact write, mutation audit, and
established `201` resource identity. Identical replay returns the original
Artifact; changed content conflicts. Artifact kind and identity remain fixed by
the existing domain and database guards.

Reads always use the authenticated tenant. Lists use ascending UUIDv7 Artifact
ID order, the API-wide default/maximum page sizes, and optional bounded lexical
matching over namespace path, artifact name, and display name. Wildcards are
treated literally. Opaque cursors are authenticated and bound to tenant,
endpoint, normalized query, page size, ordering, and expiry.

This API creates only a logical Artifact. Immutable version/source/digest
registration remains PUB-005 and later OCI work. No event variant,
cross-component responsibility, runtime authority, or catalog eligibility was
introduced.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/app/publication ./internal/adapters/http ./internal/adapters/postgres/artifact ./internal/domain/artifact
GOTOOLCHAIN=go1.26.7 make test-migrations
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

Focused tests cover exact-role authorization, live ownership failure, atomic
application composition, identical replay, changed-request conflict,
tenant-scoped read, query- and tenant-bound pagination, strict decoding,
tenant injection rejection, required idempotency keys, UUIDv7 paths, bounded
query parameters, and literal SQL wildcard handling.
