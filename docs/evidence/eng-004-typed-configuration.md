# ENG-004 typed configuration evidence

Date: 2026-08-30

Implementation commit: `80a4480` (`Implement typed secure configuration`)

## Outcome

ThinkPixelMP now owns a standard-library-only `internal/config` package that applies bounded defaults, an optional strict JSON file, `TPMP_*` environment variables, and command-line flags in increasing precedence. It rejects unknown and malformed input, repeated ThinkPixelMP environment entries, positional arguments, trailing JSON, non-regular files, files larger than 1 MiB, invalid modes, unsafe limits, invalid connection-pool ranges, and inconsistent telemetry settings.

The initial typed surface covers deployment mode, HTTP limits and lifecycle timeouts, database connection behavior, logging, and telemetry. Development defaults bind to loopback, use bounded HTTP resources, export no telemetry, and contain no persistence credential. Production fails unless the database URL is an operator-controlled secret reference.

Secret-bearing settings accept only `env:NAME` and clean absolute `file:/path` references. Configuration loading retains opaque references and adapters resolve them explicitly at use time. Resolution rejects absent or empty environment values, unavailable or non-regular files, empty files, and files larger than 1 MiB. File contents are not normalized or copied into configuration.

`SecretRef`, resolved `Secret`, and complete `Config` values prevent disclosure through ordinary string, debug, and JSON output. Safe configuration output reports only secret presence and provider type, never the environment-variable name, file path, configuration-file path, or secret value. Parse, validation, and resolution errors do not echo secret input.

The operator-facing precedence, field mapping, defaults, validation posture, and secret-reference contract are documented in `docs/operations/configuration.md`.

## Test coverage

Unit and adversarial tests cover:

- safe and valid defaults;
- file/environment/flag precedence;
- unknown JSON fields, environment variables, and flags;
- duplicate environment entries, positional arguments, trailing JSON, malformed durations, and inline-secret rejection;
- the 1 MiB configuration-file ceiling;
- production database-reference requirements;
- HTTP, pool, logging, telemetry, endpoint, and sampling validation bounds;
- accepted and rejected secret-reference grammar;
- environment and regular-file secret resolution;
- empty, non-regular, unavailable, and oversized secret sources;
- canary non-disclosure through string, `%v`, `%#v`, JSON, configuration-file paths, and safe errors.

## Acceptance commands

```bash
test -z "$(gofmt -l cmd internal scripts test)"
GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
./scripts/validate-phase0.sh
git diff --check
```

These commands prove package compilation, strict and adversarial configuration behavior, static analysis, dependency-policy preservation, Phase 0 contract preservation, and whitespace integrity.
