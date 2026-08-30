# Runtime configuration

ThinkPixelMP loads typed process configuration from four layers, in increasing precedence:

1. compiled safe defaults;
2. an optional strict JSON file selected by `--config`;
3. `TPMP_*` environment variables;
4. command-line flags.

Unknown JSON fields, `TPMP_*` variables, flags, positional arguments, malformed values, repeated environment variables, trailing JSON values, non-regular configuration files, and configuration files larger than 1 MiB are rejected. A configuration error prevents process startup.

## Initial fields

| JSON path | Environment | Flag |
| --- | --- | --- |
| `mode` | `TPMP_MODE` | `--mode` |
| `http.address` | `TPMP_HTTP_ADDRESS` | `--http-address` |
| `http.read_header_timeout` | `TPMP_HTTP_READ_HEADER_TIMEOUT` | `--http-read-header-timeout` |
| `http.read_timeout` | `TPMP_HTTP_READ_TIMEOUT` | `--http-read-timeout` |
| `http.write_timeout` | `TPMP_HTTP_WRITE_TIMEOUT` | `--http-write-timeout` |
| `http.idle_timeout` | `TPMP_HTTP_IDLE_TIMEOUT` | `--http-idle-timeout` |
| `http.shutdown_timeout` | `TPMP_HTTP_SHUTDOWN_TIMEOUT` | `--http-shutdown-timeout` |
| `http.max_header_bytes` | `TPMP_HTTP_MAX_HEADER_BYTES` | `--http-max-header-bytes` |
| `http.max_body_bytes` | `TPMP_HTTP_MAX_BODY_BYTES` | `--http-max-body-bytes` |
| `database.url` | `TPMP_DATABASE_URL` | `--database-url` |
| `database.connect_timeout` | `TPMP_DATABASE_CONNECT_TIMEOUT` | `--database-connect-timeout` |
| `database.health_timeout` | `TPMP_DATABASE_HEALTH_TIMEOUT` | `--database-health-timeout` |
| `database.statement_timeout` | `TPMP_DATABASE_STATEMENT_TIMEOUT` | `--database-statement-timeout` |
| `database.lock_timeout` | `TPMP_DATABASE_LOCK_TIMEOUT` | `--database-lock-timeout` |
| `database.max_connection_lifetime` | `TPMP_DATABASE_MAX_CONNECTION_LIFETIME` | `--database-max-connection-lifetime` |
| `database.max_connection_idle_time` | `TPMP_DATABASE_MAX_CONNECTION_IDLE_TIME` | `--database-max-connection-idle-time` |
| `database.min_connections` | `TPMP_DATABASE_MIN_CONNECTIONS` | `--database-min-connections` |
| `database.max_connections` | `TPMP_DATABASE_MAX_CONNECTIONS` | `--database-max-connections` |
| `log.level` | `TPMP_LOG_LEVEL` | `--log-level` |
| `telemetry.mode` | `TPMP_TELEMETRY_MODE` | `--telemetry-mode` |
| `telemetry.endpoint` | `TPMP_TELEMETRY_ENDPOINT` | `--telemetry-endpoint` |
| `telemetry.service_name` | `TPMP_TELEMETRY_SERVICE_NAME` | `--telemetry-service-name` |
| `telemetry.sample_ratio` | `TPMP_TELEMETRY_SAMPLE_RATIO` | `--telemetry-sample-ratio` |

Durations use Go duration syntax such as `500ms`, `5s`, or `30m`. Configuration supports `development`, `test`, and `production` modes. Defaults bind only to `127.0.0.1:8080`, enable no external telemetry exporter, impose bounded HTTP sizes and timeouts, and contain no database credential. Production requires `database.url` to be configured as a secret reference.

## Secret references

Secret-bearing fields accept only:

- `env:VARIABLE_NAME`;
- `file:/clean/absolute/path`.

Inline database URLs, credentials, relative file paths, and unrecognized schemes are rejected. Loading validates and retains the opaque reference but does not resolve it. The owning adapter explicitly resolves the reference at the point of use. File secrets must be non-empty regular files no larger than 1 MiB; their bytes are returned unchanged.

Secret references and resolved secrets redact ordinary string, debug, and JSON representations. Safe configuration output reports only whether a reference is configured and whether its source is `environment` or `file`. It never reports an environment-variable name, file path, or secret value. Validation and resolution errors likewise omit secret targets and values.

Example development file:

```json
{
  "mode": "development",
  "database": {
    "url": "file:/run/secrets/thinkpixelmp-database-url"
  },
  "log": {
    "level": "info"
  }
}
```

Future OIDC, registry, policy, federation, evidence-provider, and other integration settings will be added with their owning contracts. They are intentionally not accepted as untyped extension data.
