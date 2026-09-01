# ENG-016 repository hygiene evidence

- Date: 2026-09-01
- Implementation commit: `7d7a74b`

## Implemented

- Added a dependency-free scanner over every Git-indexed file.
- Rejects private keys, JWT/OIDC tokens, encoded registry authentication,
  assigned integration/test secrets, credential-bearing filenames, keystores,
  and explicitly private/raw evidence or test-secret directories.
- Added matching ignore rules for local secret material while retaining
  `.env.example` as a placeholder-only documentation surface.
- Added `make repository-hygiene` to the aggregate `make verify` and CI gate.
- Documented safe evidence, fixture, placeholder, rotation, and remediation use.

## Verification

```text
GOCACHE=/tmp/thinkpixelmp-eng016-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./scripts/repositoryhygiene
ok github.com/bdobrica/ThinkPixelMP/scripts/repositoryhygiene

GOCACHE=/tmp/thinkpixelmp-eng016-final-go-cache GOTOOLCHAIN=go1.26.7 make repository-hygiene
Repository hygiene passed (220 tracked files scanned).

GOCACHE="$PWD/.tmp-eng016/go-cache" GOTMPDIR="$PWD/.tmp-eng016/go-tmp" BUILD_DIR="$PWD/.tmp-eng016/build" GOTOOLCHAIN=go1.26.7 make verify
No vulnerabilities found.
OpenAPI source is valid and generated bundle is current.
Phase 0 schema, OpenAPI, and whitespace validation passed.

git diff --check
```

The full gate used repository-local temporary build storage because the host
`/tmp` filesystem lacked capacity. The temporary directory was removed after
verification. Existing non-fatal Redocly duplicate-schema-name warnings were
unchanged.
