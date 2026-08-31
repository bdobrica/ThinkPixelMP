# ENG-010 root Makefile evidence

Date: 2026-08-31

Implementation commit: `2c8b415`

## Result

The root `Makefile` is the documented developer and CI interface. Its default
`help` target lists generation, formatting, static analysis, general and focused
test suites, OpenAPI and contract validation, dependency policy, aggregate
verification, and image build entry points.

Targets compose the repository's existing scripts and pinned Go module tooling
instead of duplicating their behavior. `GO` and `GO_PACKAGES` remain overrideable
for CI. The `image` target reserves the planned stable name and fails with an
actionable message until ENG-014 adds the container definition; it does not claim
that an image can already be built. README quick-start guidance points developers
to `make help`.

ENG-010 does not expand `verify` with vulnerability, license, binary-build, or
other new gates assigned to ENG-011.

## Verification

The following commands passed:

```sh
make help
GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make fmt generate test-integration test-contract test-security test-e2e dependency-check test-race
GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

The dependency policy and aggregate Go test gate require network access to query
public module retraction metadata. They passed with that access enabled. OpenAPI
generation retained the six previously known non-fatal duplicate schema-name
warnings and produced no bundle drift.

The reserved target was also checked directly: `make image` exited with status 2
and reported that ENG-014 must add `Containerfile`.
