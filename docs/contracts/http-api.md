# HTTP API conventions

The canonical release-candidate API is REST/JSON described by OpenAPI 3.1 under `/v1`.

## Identity and errors

Resource and event IDs use UUIDv7. Errors use RFC 7807 Problem Details with stable ThinkPixelMP problem types and bounded extension fields. Errors never echo credentials, raw evidence, artifact bodies, tokens, or sensitive upstream responses.

Authenticated responses may expose `tenant_id` as C1 context. Write bodies never accept it as tenant authority; the server derives tenant from verified identity.

## Pagination

List APIs use opaque authenticated cursors bound to tenant, endpoint, filters, sort order, page size, and expiry. A cursor cannot be replayed across tenants or query shapes. Default page size is 50 and maximum is 200. Authoritative ordering includes a stable unique tie-breaker.

## Idempotency

Mutating create/action endpoints require `Idempotency-Key`. Ownership is `(tenant, principal, action, key)` and the record binds the canonical request digest and resulting status/resource.

Records are retained for at least 24 hours. Reuse with different content returns conflict. Concurrent identical requests converge on one logical result. Permanent domain uniqueness and immutable records continue protecting correctness after idempotency-record expiry.

## Long-running operations

Publication, import, evidence verification, and resolution expose operation resources with `pending`, `running`, `succeeded`, `failed`, and `cancelled` states. Creation returns the operation reference rather than holding an HTTP connection through external work.

Cancellation is best effort and cannot erase committed records, external side effects, or audit history. Terminal failures contain stable bounded reason codes and safe retry guidance.

## Concurrency and caching

Mutable administrative resources use explicit version/ETag preconditions where lost updates matter. Immutable resources may use long-lived digest/ETag caching. APIs never cache an authorization decision as an artifact property.

Mutable administrative actions require `If-Match` with a strong ETag. Absence returns `428 Precondition Required`; a stale value returns `412 Precondition Failed`. Immutable creates and idempotent append-only actions do not require `If-Match`.

## Time and units

All API timestamps use UTC RFC 3339 with required fractional seconds and the `Z` suffix. Inputs with offsets may be rejected where a contract requires canonical form; persisted and emitted values are normalized to UTC.

Durations, byte quantities, CPU, counts, and similar limits use explicitly named integer-unit fields such as `deadline_seconds`, `size_bytes`, and `cpu_millicores`. Ambiguous duration or quantity strings are not accepted in authoritative contracts.
