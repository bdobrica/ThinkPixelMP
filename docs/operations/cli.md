# Command-line API client

`thinkpixelmpctl` is an operator-facing client for the versioned ThinkPixelMP HTTP
API. It never reads or writes PostgreSQL directly. The initial skeleton exposes
only the implemented health operations:

```sh
go run ./cmd/thinkpixelmpctl live
go run ./cmd/thinkpixelmpctl --endpoint https://mp.example.test ready
```

The default endpoint is `http://127.0.0.1:8080`. `TPMPCTL_ENDPOINT` may select a
different origin and `--endpoint` takes precedence. Plain HTTP is accepted only
for loopback endpoints; remote endpoints require HTTPS. Embedded credentials,
queries, and fragments are rejected.

Protected API commands added in later implementation phases can obtain a bearer
token from `TPMPCTL_TOKEN`. `--token-env NAME` selects another environment
variable. Tokens are never accepted as command-line values, included in output,
or persisted by the CLI. Responses are bounded to 1 MiB and requests default to
a 15-second timeout, configurable with `--timeout` up to five minutes.

Marketplace resource commands will be added alongside their server-side use
cases. They must continue to use the versioned HTTP API and must not bypass its
identity, authorization, idempotency, or audit controls through database access.
