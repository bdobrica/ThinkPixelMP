# PUB-001 Publisher API evidence

- Date: 2026-09-06
- Scope: authenticated tenant-scoped Publisher create/read/list API

## Outcome

ThinkPixelMP now exposes a composable HTTP handler and application service for
`POST /v1/publishers`, `GET /v1/publishers/{publisher_id}`, and
`GET /v1/publishers`. The transport strictly decodes bounded requests, accepts
only Bearer authorization syntax when credential material is present, never
accepts tenant authority in a body or query, emits the existing Publisher v1
representations, and maps typed failures to safe RFC 7807 responses.
The OIDC adapter now composes its existing verifier and claim mapper behind the
same authenticator port used by the HTTP boundary; explicit local development
authentication retains its credential-rejecting behavior.

Create requires the exact IAM-003 `publisher.manage` action. Its canonical
request digest and `(tenant, principal, publisher.create, key)` ownership record
are transactionally composed with the Publisher write, existing audit trigger,
and established `201` resource identity. Identical replay returns the original
Publisher; changed content conflicts. Read and UUIDv7-ordered list operations
always use the authenticated tenant. List pagination defaults to 50, caps at
200, and uses an authenticated cursor bound to tenant, endpoint, ordering, page
size, and expiry.

The OpenAPI source now documents the already-supported bounded authentication,
validation, authorization, conflict, and dependency-failure responses for these
routes. No Publisher or cross-component ownership contract changed.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/app/publication ./internal/adapters/http ./internal/adapters/oidc ./internal/adapters/postgres/publisher
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

Focused tests cover exact-role authorization, atomic application composition,
same-request replay, changed-request conflict, tenant-scoped read, opaque
multi-page traversal, cross-tenant cursor rejection, strict transport decoding,
tenant injection rejection, authorization syntax, required idempotency keys,
UUIDv7 paths, and bounded query parameters.
