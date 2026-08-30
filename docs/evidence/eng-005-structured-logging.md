# ENG-005 structured logging evidence

Date: 2026-08-30

Implementation commit: `a2e83e8` (`Implement bounded structured logging`)

## Outcome

ThinkPixelMP now owns a standard-library-only structured logger in `internal/telemetry/logging`. It emits newline-delimited JSON at configured `debug`, `info`, `warn`, or `error` thresholds through a caller-supplied writer. The package exposes constrained event methods rather than an unrestricted `slog.Logger`, and accepts only stable lowercase dot-separated event names. Arbitrary messages cannot bypass attribute controls.

Validated context correlation covers all canonical fields from `PLAN.md`: tenant, publisher, artifact, artifact version, artifact digest, catalog, promotion request, resolution, import source, request, and trace. Non-empty values are bounded printable identifiers. Caller attributes cannot spoof these values or duplicate trusted JSON metadata, whether supplied directly, pre-bound, grouped, or nested.

The sanitizer recursively processes attributes, groups, `slog.LogValuer` output, maps, JSON-tagged exported struct fields, slices, arrays, and pointers. It redacts credential-shaped keys, restricted bodies and payloads, descriptors, evidence, policy input, prompts, source content, raw errors, and explicitly classified C2/C3 values. Custom errors remain redacted even when they implement `slog.LogValuer`.

Safe strings are limited to 4 KiB with UTF-8 boundary preservation. Collections and attribute sets are limited to 64 entries per level, nesting is limited to eight levels, cycles are suppressed, and unsupported or uncertain values receive safe markers instead of fallback serialization. Pre-bound loggers are immutable and repeated `With` calls cannot bypass the root bound.

The logging contract, correlation ownership, redaction vocabulary, structural limits, and caller obligations are documented in `docs/operations/logging.md`.

## Test coverage

Unit, adversarial, and race-enabled tests cover:

- JSON event shape and configured level filtering;
- rejection of arbitrary and secret-bearing event messages;
- all canonical trusted correlation fields;
- direct, pre-bound, nested, and JSON-metadata correlation spoofing;
- malformed, whitespace-bearing, control-bearing, and oversized identifiers;
- credential headers, maps, groups, structs, slices, and `slog.LogValuer` output;
- explicit confidential and restricted values;
- ordinary and `slog.LogValuer` errors under non-sensitive keys;
- request bodies and ENG-004 configuration secret references;
- strings, collections, depth, repeated pre-binding, cycles, and unsupported values;
- immutable child loggers and concurrent complete-record emission.

## Acceptance commands

```bash
test -z "$(gofmt -l cmd internal scripts test)"
GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/telemetry/logging
GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
./scripts/validate-phase0.sh
git diff --check
```

These commands prove concurrent structured output, adversarial redaction and correlation integrity, package compilation, static analysis, dependency-policy preservation, Phase 0 contract preservation, and whitespace integrity.
