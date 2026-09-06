# PUB-002 Namespace API evidence

- Date: 2026-09-06
- Scope: authenticated tenant-scoped Namespace create/read/list API

## Outcome

ThinkPixelMP now exposes a composable HTTP handler and application service for
`POST /v1/namespaces`, `GET /v1/namespaces/{namespace_id}`, and
`GET /v1/namespaces`. The transport strictly decodes bounded requests, accepts
only Bearer authorization syntax when credential material is present, never
accepts tenant authority in a body or query, emits the existing Namespace v1
representation, and maps typed failures to safe RFC 7807 responses.

Create requires the exact IAM-003 `namespace.manage` action. Its canonical
request digest and `(tenant, principal, namespace.create, key)` ownership record
are transactionally composed with the Namespace write, existing audit trigger,
and established `201` resource identity. Identical replay returns the original
Namespace; changed content conflicts. The repository contract and PostgreSQL
guard require the owner to be a verified Publisher in the same tenant. Read and
UUIDv7-ordered list operations always use the authenticated tenant. List
pagination defaults to 50, caps at 200, and uses an authenticated cursor bound
to tenant, endpoint, ordering, page size, and expiry.

The OpenAPI source documents the supported bounded authentication, validation,
authorization, conflict, not-found, and dependency-failure responses for these
routes. Namespace delegation and publication ownership checks remain scoped to
PUB-003.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/app/publication ./internal/adapters/http ./internal/adapters/postgres/namespace
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

Focused tests cover exact-role authorization, atomic application composition,
same-request replay, changed-request conflict, verified-owner rejection,
tenant-scoped read, opaque multi-page traversal, cross-tenant cursor rejection,
strict transport decoding, tenant injection rejection, authorization syntax,
required idempotency keys, UUIDv7 paths, and bounded query parameters.
