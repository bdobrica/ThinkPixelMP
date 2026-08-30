# ENG-003 dependency, source, and license policy evidence

Date: 2026-08-30

## Outcome

ThinkPixelMP now has a closed machine-readable `dependency-policy.json`, normative `docs/security/dependencies.md`, and a standard-library-only checker in `scripts/dependencycheck`.

The policy begins with no third-party modules and no exceptions. It permits only the approved module-path prefixes and licenses, requires exact selected versions and public checksum verification, forbids private modules, vendoring, replacements, exclusions, graph errors, and retracted modules, and requires exact active exceptions for pseudo-versions or other approved deviations.

Exceptions contain an exact module/version scope, kind-specific license or vulnerability identity where applicable, owner, justification, approval, dates, compensating controls, and removal plan. The checker rejects malformed, wildcard, duplicate, future-created, expired, or longer-than-90-day exceptions.

The checker evaluates the selected graph through the currently running pinned Go toolchain. It rejects selected modules exempted from public checksum verification by matching `GOPRIVATE` or `GONOSUMDB` patterns and executes `go mod verify`. Full dependency license classification and call-graph vulnerability scanning remain explicitly assigned to ENG-011.

## Test coverage

Unit and repository-integration tests cover:

- the real repository policy, `go.mod`, graph, checksum configuration, and `go mod verify`;
- closed JSON decoding and rejection of trailing JSON values;
- required policy posture, sorted unique values, and wildcard rejection;
- exact version, source, replacement, retraction, graph-error, and pseudo-version failures;
- exact pseudo-version exception matching;
- exception kind, completeness, lifetime, expiry, and wildcard failures;
- forbidden `replace` and `exclude` directives;
- disabled checksum database and matching checksum-bypass patterns.

## Acceptance commands

```bash
test -z "$(gofmt -l scripts/dependencycheck)"
GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
./scripts/validate-phase0.sh
git diff --check
```

These commands prove policy enforcement against the selected graph, unit/adversarial behavior, complete package compilation, static analysis, preservation of Phase 0 contracts, and whitespace integrity.
