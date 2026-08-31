# ENG-013 API CLI evidence

Date: 2026-08-31

Implementation commit: `bd3c30e`

## Result

The repository now builds `thinkpixelmpctl` as an HTTP-only operator client. Its
initial `live` and `ready` commands call the service health endpoints through the
public transport rather than accessing PostgreSQL or importing persistence code.
The root `build` gate now produces both service and CLI binaries.

The client defaults to the loopback development service, requires HTTPS for
non-loopback endpoints, rejects URL credentials and decorations, applies bounded
timeouts and response sizes, and accepts bearer tokens only through a named
environment variable. It does not print response bodies for unsuccessful status
codes, reducing the chance that an upstream error payload reaches terminal logs.

Marketplace resource commands remain deferred until their server-side use cases
exist. This avoids creating a parallel authority path around HTTP authentication,
tenant derivation, authorization, idempotency, and audit controls.

## Acceptance evidence

- `GOCACHE=/tmp/thinkpixelmp-eng013-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./cmd/thinkpixelmpctl/...` passed.
- `GOCACHE=/tmp/thinkpixelmp-eng013-go-cache GOTOOLCHAIN=go1.26.7 go build -trimpath -o /tmp/thinkpixelmp-build/thinkpixelmpctl ./cmd/thinkpixelmpctl` passed.
- `GOCACHE=/tmp/thinkpixelmp-eng013-go-cache GOTOOLCHAIN=go1.26.7 make verify` passed, including format, static analysis, unit/race, dependency, vulnerability, license, OpenAPI/contract, and both binary-build gates.
- `git diff --check` passed.

The first aggregate-gate attempt stopped during static analysis because the
sandbox blocked resolution of its pinned tool. The approved network-enabled rerun
passed. No token, credential, response payload, or database state was retained in
the repository evidence.
