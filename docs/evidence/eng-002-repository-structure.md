# ENG-002 repository structure evidence

Date: 2026-08-30

Implementation commit: `f30ac05` (`Establish repository package structure`)

## Implemented structure

The Go repository now matches the `PLAN.md` domain/application/ports/adapters layout:

- documentation-only command packages reserve `thinkpixelmp`, `migrate`, and `thinkpixelmpctl` without introducing executable behavior owned by later items;
- domain packages separate publisher, artifact, evidence, catalog, promotion, resolution, and revocation rules;
- application packages separate publication, discovery, evidence, promotion, resolution, and federation use cases;
- technology-neutral port packages reserve registry, signature, provenance, evidence, policy, identity, key, importer, and clock boundaries;
- adapter packages reserve ORAS, Sigstore, OPA, MCP Registry, OCI import, A2A, Git, HTTP, OIDC, PostgreSQL, evidence, and key-provider implementations;
- telemetry and security remain separate internal concerns;
- migration, Helm deployment, and integration/contract/security/federation/end-to-end test boundaries are tracked without placeholder implementations.

The `internal/adapters/import/` parent is intentionally a Markdown-documented grouping directory because `import` is a Go keyword. Its four child directories are normal Go packages.

Every Go placeholder is a package documentation file rather than an empty marker. No third-party dependency, domain type, port method, adapter behavior, configuration, server, migration, CLI behavior, or deployment default is introduced.

## Architecture enforcement

`test/architecture/layout_test.go` verifies:

- every planned package and non-package directory exists;
- every expected package is discoverable through the pinned Go toolchain, while allowing deliberate packages added by later work;
- packages beneath `internal/domain/...` do not import ORAS, Sigstore, OPA, pgx, common HTTP frameworks, MCP Registry, A2A, Kubernetes, or ThinkPixelAG transport packages.

Nested Go invocations use the currently running toolchain from `runtime.GOROOT()` with an isolated writable module cache. This avoids silently falling back to a different host Go launcher and keeps the architecture test usable in restricted CI environments.

## Acceptance commands

```bash
test -z "$(gofmt -l cmd internal test)"
GOCACHE=/tmp/thinkpixelmp-eng002-go-cache GOTOOLCHAIN=go1.26.7 go test ./...
GOCACHE=/tmp/thinkpixelmp-eng002-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...
GOCACHE=/tmp/thinkpixelmp-eng002-go-cache GOTOOLCHAIN=go1.26.7 go list ./...
./scripts/validate-phase0.sh
git diff --check
```

These commands prove formatting, package compilation, architecture assertions, static analysis, package discovery, preservation of Phase 0 contracts, and whitespace integrity.
