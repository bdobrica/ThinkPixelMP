# HTTP server baseline

`internal/adapters/http` owns ThinkPixelMP's inbound HTTP transport and process-serving lifecycle. Application routes are supplied as an `http.Handler`; dependency readiness is supplied through the narrow `Readiness` interface. The adapter does not perform authentication, tenant selection, policy evaluation, or domain operations.

## Request handling

Every request receives a new server-owned UUIDv7 in `X-Request-ID`. Caller-provided request IDs are not trusted or reflected. The adapter extracts only W3C `traceparent` and `tracestate` using the configured tracing propagator, starts a server span with a fixed name, and adds the resulting request and trace IDs to validated logging correlation. It does not propagate baggage or capture headers, URLs, bodies, errors, artifact metadata, or evidence in telemetry.

The configured maximum body size is enforced with `http.MaxBytesReader`; a declared oversized body is rejected with `413` before application dispatch. Streaming handlers remain responsible for mapping a size error encountered while reading a body through `WriteError` or an endpoint-specific safe problem. Server header, read-header, read, write, idle, and shutdown limits come from validated configuration.

Panic recovery emits a stable safe event and an RFC 7807 response when the response is not already committed. Panic values, stacks, application errors, dependency errors, and upstream response content are never returned to clients. `WriteError` maps only shared typed error class/reason pairs; unknown errors collapse to `internal_error`.

## Operational endpoints

- `GET /livez` reports process liveness and has no dependency checks.
- `GET /readyz` invokes required-dependency readiness. It returns a stable `503 service_not_ready` without serializing the dependency error. Optional external-source health must not be included in this aggregate check.
- `GET /metrics` exposes the process-private Prometheus gatherer. It does not use or reveal the global registry.

Health and problem responses are `no-store`. These endpoints are intentionally unauthenticated as described by the OpenAPI contract; operators should expose them only on appropriately controlled infrastructure paths. API routes are mounted below `/v1/` and retain their own authentication and authorization responsibility.

## Lifecycle

`Server.Run` serves until cancellation or an unexpected listener failure. Cancellation initiates `http.Server.Shutdown` with the configured bounded timeout and waits for the serving goroutine to exit. The process owner remains responsible for initializing and shutting down tracing and other external resources around this lifecycle.
