# ENG-018 clean-checkout baseline evidence

- Date: 2026-09-01
- Baseline commit: `39b164a`

## Acceptance result

The tracked checkout was clean before verification. Regenerating the committed
OpenAPI bundle and formatting all Go packages produced no tracked diff. The
root aggregate gate then passed formatting, vet and Staticcheck, unit and race
tests, repository hygiene, dependency policy, vulnerability and license checks,
OpenAPI drift and contract validation, and both binary builds.

The pinned container image also built successfully. Its disposable smoke
container returned `{"status":"ok"}` from `/livez` while running as
`65532:65532` with a read-only root filesystem, all Linux capabilities dropped,
and `no-new-privileges`. The container was removed after inspection.

## Verification

```text
git status --porcelain=v1
git diff --exit-code
git diff --cached --exit-code

GOCACHE="$PWD/.tmp-eng018/go-cache" \
GOTMPDIR="$PWD/.tmp-eng018/go-tmp" \
BUILD_DIR="$PWD/.tmp-eng018/build" \
GOTOOLCHAIN=go1.26.7 \
make generate fmt
git diff --exit-code

GOCACHE="$PWD/.tmp-eng018/go-cache" \
GOTMPDIR="$PWD/.tmp-eng018/go-tmp" \
BUILD_DIR="$PWD/.tmp-eng018/build" \
GOTOOLCHAIN=go1.26.7 \
make verify

make image
docker run --detach --rm --name thinkpixelmp-eng018 \
  --read-only --cap-drop=ALL --security-opt=no-new-privileges \
  --publish 127.0.0.1:18080:8080 thinkpixelmp:dev
curl --fail --silent --show-error http://127.0.0.1:18080/livez
docker inspect --format \
  '{{.Config.User}} {{.HostConfig.ReadonlyRootfs}} {{index .HostConfig.CapDrop 0}} {{index .HostConfig.SecurityOpt 0}}' \
  thinkpixelmp-eng018

{"status":"ok"}
65532:65532 true ALL no-new-privileges
```

The first sandboxed aggregate attempt stopped when network policy denied the
pinned Staticcheck download. The same gate passed with approved dependency
access. `govulncheck` reported no vulnerabilities. Existing non-fatal Redocly
duplicate-schema-name warnings and `go-licenses` assembly-inspection warnings
were unchanged. Repository-local temporary build storage was removed after the
run.
