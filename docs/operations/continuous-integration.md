# Continuous integration

The GitHub Actions workflow runs the root `make verify` gate, the Docker-backed
`make test-migrations` gate, and a separate hardened-container smoke test on pull
requests, pushes to `main`, and manual dispatches. The database job creates and
removes its own loopback-only PostgreSQL `18.6-bookworm` container and never uses
an operator database or repository secret. CI is validation-only: it does not
publish artifacts, mutate repository state, request an OIDC identity token, or
consume repository secrets.

The workflow grants only read access to repository contents. Checkout credentials are not persisted, every third-party action reference is pinned to a full commit SHA, tool versions come from `go.mod`, `package-lock.json`, and the root `Makefile`, and every job has a bounded timeout. Concurrent runs for the same ref cancel superseded work.

The aggregate gate also runs the tracked-file [repository hygiene check](repository-hygiene.md), which rejects committed credentials, signing keys, tokens, private evidence, and secret-bearing test fixtures.

Action upgrades require reviewing the upstream release and changing both the immutable SHA and its adjacent human-readable release comment. Reproduce the principal CI gate locally with:

```bash
npm ci --ignore-scripts
GOTOOLCHAIN=go1.26.7 make verify
GOTOOLCHAIN=go1.26.7 make test-migrations
make image
```

The image job runs the service as UID/GID `65532:65532` with a read-only root filesystem, all Linux capabilities dropped, and `no-new-privileges`, then checks `/livez`. See [Container image](container-image.md) for the equivalent local smoke procedure.
