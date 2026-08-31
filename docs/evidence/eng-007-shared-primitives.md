# ENG-007 shared primitives evidence

Date: 2026-08-31

Implementation commit: `25aee64` (`Implement shared domain primitives`)

## Acceptance evidence

The dependency-free `internal/domain/shared` package implements canonical RFC 9562 UUIDv7 parsing/generation, algorithm-aware release-candidate SHA-256 digests, tenant-relative logical artifact references, bounded printable strings, stable reason codes, safe typed error classes, and authenticated pagination cursors. `internal/ports/clock` provides the injectable time boundary with UTC system and deterministic fixed implementations.

Cursor tests prove HMAC authentication and binding to tenant, endpoint, canonical query digest, page size, and expiry. They reject tampering, cross-tenant and cross-query replay, changed endpoints/page sizes, expired cursors, weak keys, and unsafe scopes. The cursor carries a keyed query binding rather than the raw filter digest. The durable security and usage contract is documented in [`docs/architecture/shared-primitives.md`](../architecture/shared-primitives.md).

Identity tests prove canonical UUIDv7 version/variant handling, injected-clock timestamp construction, strict lowercase full-length SHA-256 parsing, exact hashing, DNS-style artifact identity parsing, schema-aligned namespace/reference limits, and rejection of mutable or malformed representations. Bounded-value tests prove UTF-8/byte/control-character restrictions and typed errors containing only stable class/code fields.

No module was added or changed. The implementation uses Go standard-library cryptography and encoding packages, so the repository dependency posture is unchanged.

## Commands run

```text
test -z "$(gofmt -l cmd internal scripts test)"
GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/shared ./internal/ports/clock
GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
./scripts/validate-phase0.sh
git diff --check
```

All commands passed. The first sandboxed broad test attempt could not reach the public Go module proxy used by the dependency-policy and isolated architecture tests; the same command was rerun with approved network access and passed. Phase 0 validation retained its pre-existing non-fatal Redocly duplicate-schema-name warnings and passed.
