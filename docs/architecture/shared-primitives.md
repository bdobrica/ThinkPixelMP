# Shared domain primitives

`internal/domain/shared` owns dependency-free value objects used across marketplace domains. `internal/ports/clock` owns the injectable time boundary. Domain and application code uses these types instead of accepting ambiguous strings, mutable tags, process time, or unauthenticated paging state.

## Identity and references

- `UUID` accepts and emits canonical RFC 9562 UUIDv7 only. `UUIDGenerator` receives an injectable clock and cryptographic randomness; production may use the default `crypto/rand` reader.
- `Digest` is algorithm-aware but the release candidate accepts only canonical lowercase `sha256:` followed by exactly 64 hexadecimal characters. `SHA256Digest` hashes exact bytes; callers canonicalize structured inputs before hashing.
- `ArtifactReference` represents the tenant-relative logical `{namespace}/{name}` reference from ADR 0001. It validates lowercase DNS-style segments and deliberately contains no tenant authority, version, tag, source location, or runtime grant.

Zero values do not serialize as valid wire identities. Parsing must occur at adapter boundaries, and domain state retains the typed result.

## Bounded values and errors

`BoundedString` requires valid printable UTF-8 and a caller-selected byte maximum no larger than 4096. It rejects empty values and control characters rather than truncating authoritative content. `ReasonCode` follows the contract-wide lowercase stable-code vocabulary and the 128-byte maximum.

`TypedError` contains only an enumerated error class and a validated reason code. It intentionally has no free-form detail or wrapped implementation error. HTTP, job, and external-provider adapters map it to their versioned error contracts without serializing SQL errors, upstream bodies, credentials, or sensitive payloads.

## Authenticated pagination cursors

`CursorCodec` uses HMAC-SHA-256 with an operator-owned key of at least 32 bytes. The encoded cursor is URL-safe, versioned, and bound to the authenticated tenant UUID, normalized endpoint path, a keyed binding of the canonical query/filter/sort digest, page size, position, and expiry. The raw query digest is not placed in the cursor. Decode verifies the MAC before parsing or using claims and rejects cross-tenant, cross-endpoint, cross-query, cross-page-size, expired, malformed, and oversized cursors.

The cursor is authenticated, not encrypted. Positions must therefore be bounded opaque database ordering keys or stable IDs classified for the response context; they must never contain credentials, proprietary metadata, raw filters, or payload content. Key material remains an operator secret and must not enter logs, traces, storage records, cursor payloads, or API responses. Key rotation and multi-key decode policy will be owned by the HTTP adapter/configuration work that provisions the codec.
