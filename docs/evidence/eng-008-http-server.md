# ENG-008 baseline HTTP server evidence

Completed: 2026-08-31

Implementation commit: `6f82cba` (`Implement baseline HTTP server`)

## Acceptance evidence

`internal/adapters/http` now composes validated HTTP configuration, the shared UUIDv7 generator, W3C-only propagation, safe correlation logging, and the process-private Prometheus gatherer. It mounts application handling below `/v1/` without taking ownership of identity, tenant, policy, or domain behavior.

Tests verify server-owned canonical UUIDv7 request IDs; liveness, readiness, and metrics endpoints; W3C parent trace extraction; rejection of declared oversized bodies before application dispatch; RFC 7807 content types and stable typed-error mapping; safe readiness failure behavior; unknown-route handling; and panic/unknown-error responses that do not disclose injected secret canaries.

The server applies configured header, read-header, read, write, idle, body, and graceful-shutdown bounds. It binds synchronously before serving, performs bounded graceful shutdown after context cancellation, and preserves optional response-writer capabilities through `Unwrap`. The durable behavior and operator responsibilities are documented in `docs/operations/http-server.md`.

No module dependency or public API/schema change was required. The existing OpenAPI health shapes and shared Problem schema remain authoritative.

## Verification

The following commands passed from the repository root:

```text
test -z "$(gofmt -l cmd internal scripts test)"
GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/adapters/http
GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
./scripts/validate-phase0.sh
git diff --check
```

The repository-wide Go checks were rerun with public Go module proxy access because the dependency-policy and architecture tests intentionally create isolated module caches and query module retractions. Phase 0 validation retained the pre-existing non-fatal Redocly duplicate schema-name bundling warnings and passed.
