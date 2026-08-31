# ENG-014 acceptance evidence

Date: 2026-08-31

Implementation commit: `6918986` (`Add hardened non-root service image`)

## Implemented baseline

- The Dockerfile frontend and `golang:1.26.7-bookworm` build stage are pinned
  by digest.
- The service is compiled with `CGO_ENABLED=0`, reproducible source paths, no
  build ID, and the `netgo`/`osusergo` tags.
- The final `scratch` image contains only the service binary, CA roots, and UTC
  timezone data. It has no shell or package manager.
- The runtime identity is the numeric unprivileged user/group `65532:65532`.
- The image contains no database URL, credential, or secret build argument and
  uses the existing operator-managed configuration boundary.
- Container-specific configuration listens on `0.0.0.0:8080`; host defaults
  remain loopback-only.
- An architecture test guards the pinned builder, static build, scratch runtime,
  non-root identity, entrypoint, and absence of obvious insecure directives.

## Verification

- `GOCACHE=/tmp/thinkpixelmp-eng014-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./test/architecture/...` passed.
- `GOCACHE=/tmp/thinkpixelmp-eng014-go-cache GOTOOLCHAIN=go1.26.7 go test ./...` passed.
- `GOCACHE=/tmp/thinkpixelmp-eng013-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed after clearing only the disposable ENG-014 build cache when the first attempt exhausted `/tmp`. The successful aggregate gate included format, vet/Staticcheck, unit/race, dependency, vulnerability, license, OpenAPI/contract, and binary-build checks; `govulncheck` reported no vulnerabilities.
- `make image` completed from the digest-pinned `Containerfile`. The resulting
  local image was approximately 6.0 MB and reported user `65532:65532` with
  entrypoint `/thinkpixelmp`.
- A disposable container started successfully with `--read-only`,
  `--cap-drop=ALL`, and `--security-opt=no-new-privileges`; `/livez` returned
  `{"status":"ok"}` and inspection confirmed all requested hardening flags.
- `git diff --check` passed.

The disposable smoke container was stopped and removed. The local development
image contains no runtime secret or retained application state.
