# ENG-001 Go module evidence

Date: 2026-08-30

Implementation commit: `32e5b3b` (`Initialize Go module`)

## Decision

ThinkPixelMP uses module path `github.com/bdobrica/ThinkPixelMP`, Go language/module baseline `1.26.0`, and exact toolchain `go1.26.7`.

The module path follows the ThinkPixelAG and ThinkPixelTG repository convention. Separating the `go` and `toolchain` directives retains Go 1.26 language semantics while selecting the patched 1.26.7 toolchain for development and builds.

No packages or third-party dependencies are introduced by ENG-001. Domain/application/port/adapter structure belongs to ENG-002, and implementation dependencies are added only by the item that uses them.

## Compatibility review

- The official Go release history records Go 1.26.7 on 2026-08-19 with `net/http` fixes.
- The stable ORAS Go v2 branch documents support for the latest two Go releases, currently 1.25 and 1.26.
- ORAS Go v2.6.2 contains security fixes for hardlink extraction escape and unbounded registry pagination and remains the planned OCI-adapter baseline.

Primary sources:

- <https://go.dev/doc/devel/release>
- <https://github.com/oras-project/oras-go>
- <https://github.com/oras-project/oras-go/releases/tag/v2.6.2>

## Acceptance commands

```bash
GOTOOLCHAIN=go1.26.7 go version
GOTOOLCHAIN=go1.26.7 go env GOMOD GOVERSION
GOTOOLCHAIN=go1.26.7 go list -m
GOTOOLCHAIN=go1.26.7 go mod edit -json
./scripts/validate-phase0.sh
git diff --check
```

These commands prove exact toolchain selection, module discovery and identity, directive parsing, preservation of Phase 0 contracts, and whitespace integrity. Package compilation begins with ENG-002 because ENG-001 intentionally creates no Go packages.
