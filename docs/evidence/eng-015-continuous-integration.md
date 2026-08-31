# ENG-015 continuous-integration evidence

Date: 2026-08-31

Implementation commit: `1aa4120` (`Add least-privilege pinned CI`)

## Acceptance result

- Added GitHub Actions validation for pull requests, pushes to `main`, and manual dispatches.
- Restricted the workflow token to `contents: read`; no write, OIDC identity-token, or secret access is requested.
- Disabled persisted checkout credentials and pinned every action to a verified full commit SHA with its reviewed release beside it.
- Installed contract tooling from `package-lock.json`, selected Go from `go.mod`, and ran the stable root `make verify` interface.
- Added an independent image job that builds without registry credentials or publication permissions and exercises the service with a read-only root filesystem, all capabilities dropped, `no-new-privileges`, and UID/GID `65532:65532`.
- Added a repository architecture test guarding the least-privilege and immutable-pin properties.

## Verification

The following completed successfully from the repository root:

```text
GOCACHE=/tmp/thinkpixelmp-eng015-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./test/architecture/...
npm ci --ignore-scripts
GOCACHE=/tmp/thinkpixelmp-eng013-go-cache GOTOOLCHAIN=go1.26.7 make verify
make image
curl --fail --silent --show-error http://127.0.0.1:18080/livez
docker inspect --format '{{.Config.User}} {{.HostConfig.ReadonlyRootfs}} {{index .HostConfig.CapDrop 0}} {{index .HostConfig.SecurityOpt 0}}' thinkpixelmp-eng015
git diff --check
```

The runtime probe returned `{"status":"ok"}`. Inspection returned `65532:65532 true ALL no-new-privileges`. The aggregate gate passed formatting, vet/Staticcheck, unit and race tests, dependency policy, govulncheck, license policy, OpenAPI/contracts, and both binary builds. Existing non-fatal Redocly duplicate-schema-name warnings remained unchanged.

The first sandboxed focused-test attempt could not download locked Go modules because network access was denied. The same command passed once approved network access was used; this was an environment restriction, not a test failure in the implementation.
