# ENG-011 verification gate evidence

Completed: 2026-08-31

Implementation commit: `3e01661` (`Add complete repository verification gate`)

## Acceptance evidence

The root `make verify` gate now rejects unformatted Go, vet and Staticcheck findings, unit or race failures, dependency-policy violations, reachable known Go vulnerabilities, disallowed runtime or test dependency licenses, invalid or drifting OpenAPI output, contract failures, and failure to build the `thinkpixelmp` service binary. Staticcheck `v0.8.1`, govulncheck `v1.7.0`, and go-licenses `v2.0.1` are exact overridable pins; the license allowlist matches `dependency-policy.json`.

Adding binary verification exposed that the existing HTTP server had no process composition root. `cmd/thinkpixelmp/main.go` now composes only existing typed configuration, structured logging, process-private metrics, UUIDv7 generation, tracing, signal cancellation, and the HTTP adapter. It does not add API/domain authority, persistence, or another ThinkPixel component's responsibility.

Staticcheck exposed three pre-existing hygiene findings. The intentional nil-context validation test and exact-selected-toolchain uses are documented suppressions, and an unused test helper was removed.

## Verification

The following commands passed from the repository root:

```text
GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make static
GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make vulnerability-check license-check
GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make verify
test -x /tmp/thinkpixelmp-build/thinkpixelmp
git diff --check
```

The aggregate gate used public Go proxy, checksum-database, module-metadata, and vulnerability-database access. Govulncheck reported no reachable vulnerabilities. Go-licenses accepted all classified dependencies and emitted informational warnings that assembly files in `xxhash/v2` and `x/sys/unix` cannot be traversed as Go imports; their modules were still classified. OpenAPI validation retained the six pre-existing non-fatal duplicate schema-name bundling warnings and reported no generated-bundle drift.
