# Service container image

The root `Containerfile` builds the `thinkpixelmp` service with the exact Go
toolchain pinned by `go.mod`, then copies the statically linked binary and CA
roots into a `scratch` runtime image. The runtime has no shell or package
manager and runs as numeric user and group `65532:65532`.

Build the local image through the stable repository interface:

```sh
make image
```

The image listens on `0.0.0.0:8080`, unlike the host-development default which
is loopback-only. Publish that port only behind the deployment's intended
network controls. `/livez`, `/readyz`, and `/metrics` remain unauthenticated
operational endpoints and must not be exposed as public application routes.

The process does not require a writable root filesystem or Linux capabilities.
Deployments should enforce a read-only root filesystem, prevent privilege
escalation, drop all capabilities, and mount configuration or secret files
read-only. Secrets are supplied by operator-managed references; they are not
baked into the image or passed as build arguments.

Example smoke run:

```sh
docker run --rm --read-only --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --publish 127.0.0.1:18080:8080 \
  thinkpixelmp:dev
```

From another terminal, `curl --fail http://127.0.0.1:18080/livez` should return
the bounded JSON liveness response. Production mode additionally requires the
database secret reference and other production configuration described in
[`configuration.md`](configuration.md).
