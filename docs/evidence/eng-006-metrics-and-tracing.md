# ENG-006 metrics and tracing evidence

Date: 2026-08-31

Implementation commit: `425d1cf` (`Implement bounded metrics and tracing`)

## Outcome

ThinkPixelMP now owns a process-local Prometheus registry in `internal/telemetry/metrics`. It never mutates Prometheus's global registry. The initial collector set covers bounded marketplace operation outcomes, dependency latency, stale evidence, catalog entries, digest revocations, outbox lag, and database-pool saturation. Its mutation methods accept package-defined operation, outcome, reason-family, dependency-class, and severity vocabularies; they reject caller-controlled labels, identifiers, URLs, negative values, and invalid ratios.

`internal/telemetry/tracing` initializes an owned OpenTelemetry SDK tracer provider. Safe `noop` configuration installs no exporter and never samples. Explicit `otlp` configuration creates the official OTLP/HTTP exporter and applies parent-based ratio sampling. The returned propagator supports W3C Trace Context only; arbitrary W3C baggage is deliberately excluded. Initialization adds only the validated service name and installs no host/environment detector, body capture, payload processor, or artifact/evidence instrumentation.

The operational contract in `docs/operations/telemetry.md` prohibits raw errors, URLs, headers, bodies, descriptors, evidence, policy inputs, prompts, archive content, credentials, and all C2/C3 values. Metric identifiers are categorically prohibited. Trace correlation remains limited to the bounded C1 allowlist in the data-classification contract.

## Dependency review

The implementation pins Prometheus Go client v1.24.1 and OpenTelemetry Go API, SDK, and OTLP/HTTP exporter v1.46.0. These are the reference Go implementations required by ENG-006 and are confined to telemetry infrastructure. `go.sum` binds the resolved public module graph.

The official exporter graph selects nonstandard-path and pseudo-version modules. The repository allowlist was not broadened. `dependency-policy.json` instead records exact ENG-006 exceptions expiring 2026-11-29, with public checksum verification, no direct production imports where applicable, and explicit upstream-removal plans. The repository dependency-policy test now evaluates live repository exceptions against the actual current date rather than its deterministic unit-test fixture date.

Adding real modules exposed a Windows/WSL cleanup issue in the architecture test's disposable module cache. `-modcacherw` now keeps that isolated cache removable; it does not change production module resolution or checksum policy.

## Test coverage

- private registry construction and complete baseline collector registration;
- categorical label acceptance and arbitrary label rejection;
- operation/result consistency, non-negative count/duration rules, ratio bounds, and revocation severity bounds;
- no-op provider never-sample behavior and clean shutdown;
- W3C `traceparent`/`tracestate` propagation without baggage;
- rejection of credential-bearing, query-bearing, mismatched-mode, and unknown telemetry configuration;
- race-enabled telemetry tests, full repository tests, static analysis, dependency policy, architecture discovery, Phase 0 schemas/OpenAPI, formatting, and whitespace.

## Acceptance commands

```bash
test -z "$(gofmt -l cmd internal scripts test)"
GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/telemetry/...
GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
./scripts/validate-phase0.sh
git diff --check
```

All commands passed. Phase 0 validation retained its pre-existing non-fatal Redocly duplicate-schema-name bundling warnings and completed successfully.
