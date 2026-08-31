# Metrics and tracing

ThinkPixelMP owns a private Prometheus registry in `internal/telemetry/metrics`. It does not register collectors with Prometheus's process-global registry. The future HTTP adapter may expose its `Gatherer` at `/metrics` without gaining access to collector mutation.

## Metric contract

The baseline registry covers marketplace operation results, external dependency latency, stale evidence, catalog entry count, revocations, outbox lag, and database-pool saturation. Operation and dependency methods accept only package-defined categorical values. Metric labels are limited to `operation`, `outcome`, `reason_family`, `dependency`, and revocation `severity`.

Tenant, principal, publisher, artifact, artifact version, digest, catalog, evidence, resolution, request, trace, source, destination, URL, provider instance, and arbitrary error values are prohibited as metric labels. Artifact descriptors, evidence reports, request/response bodies, policy inputs, prompts, archive contents, and credentials are never metric values.

## Trace initialization

`internal/telemetry/tracing` returns an owned OpenTelemetry tracer provider and W3C Trace Context propagator. The safe default is `noop`: it configures no exporter and uses a never-sample policy. `otlp` mode creates an OTLP/HTTP exporter and applies parent-based ratio sampling using the validated telemetry configuration. The process that creates a provider owns its shutdown.

Only W3C `traceparent` and `tracestate` are propagated. Baggage is deliberately not propagated because arbitrary caller-controlled baggage is outside the trace attribute allowlist.

Initialization adds only the configured service name. It does not install HTTP body capture, payload processors, automatic artifact/evidence instrumentation, host/environment detectors, or arbitrary resource attributes. Instrumentation callers may add bounded stable operation names, outcomes, reason families, and the allowlisted C1 correlation identifiers described in the data-classification contract. They must not attach raw errors, URLs, headers, bodies, descriptors, evidence, policy inputs, secrets, or other C2/C3 content.

Telemetry export failure must be reported using a stable safe event code; it must never trigger payload fallback logging.

## Dependency review

ENG-006 pins `github.com/prometheus/client_golang` v1.24.1 and the OpenTelemetry Go API, SDK, and OTLP/HTTP exporter modules at v1.46.0. These are the upstream reference Go clients for the selected open standards, remain confined to `internal/telemetry`, and are justified by the required Prometheus and OpenTelemetry integration. Their transitive module graph is checksum-pinned in `go.sum` and remains subject to `scripts/dependencycheck`.
