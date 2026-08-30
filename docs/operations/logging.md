# Structured logging

ThinkPixelMP emits newline-delimited JSON events through `internal/telemetry/logging`. Production processes normally direct this stream to standard error for collection by the deployment platform. The configured levels are `debug`, `info`, `warn`, and `error`.

## Event contract

Every record uses a stable lowercase dot-separated event name, for example `http.request.completed`. The event name is both the `msg` and `event` value. Arbitrary free-text messages are rejected so caller-controlled or secret material cannot bypass attribute redaction through a message.

The logger exposes constrained event methods rather than its underlying `slog.Logger`. Pre-bound and per-event attributes cross the same sanitizer. Callers must report safe stable error codes and outcomes; raw `error` values are deliberately redacted.

## Trusted correlation

Trusted middleware and application orchestration attach correlation through `context.Context`. The canonical fields are:

- `tenant`;
- `publisher_id`;
- `artifact_id`;
- `artifact_version_id`;
- `artifact_digest`;
- `catalog_id`;
- `promotion_request_id`;
- `resolution_id`;
- `import_source_id`;
- `request_id`;
- `trace_id`.

Non-empty identifiers must be trimmed, printable, whitespace-free, and at most 128 bytes. ENG-007 will replace applicable opaque identifiers with shared typed UUID and digest primitives.

Correlation and JSON metadata names are reserved. Attributes cannot replace or duplicate trusted correlation, `event`, `time`, `level`, `msg`, or `source` fields, including through pre-bound or nested data.

## Redaction and bounds

Credential-shaped names are matched case-insensitively after normalizing punctuation. Authorization values, cookies, passwords, secrets, tokens, API keys, credentials, private/signing keys, database URLs/DSNs, sensitive query fields, request/response bodies, descriptors, evidence payloads, policy input, prompts, and source content are redacted structurally before JSON serialization.

Callers use `logging.Confidential(value)` or `logging.Restricted(value)` when a safe-looking field name carries C2 or C3 data. Those wrappers do not retain the supplied value. Raw errors are redacted regardless of attribute name. Unsupported or uncertain reflection values are suppressed rather than passed to a fallback formatter.

Safe strings are bounded to 4 KiB. Groups, maps, structs, slices, arrays, and pre-bound attribute sets retain at most 64 entries at each level. Recursion stops after eight levels, and pointer cycles are replaced with safe markers. Exported struct fields honor JSON field names; unexported fields, ignored JSON fields, unsupported map keys, functions, channels, and unsafe values are not serialized.

Redaction is defense in depth. Callers must still avoid constructing raw descriptors, reports, policy inputs, request/response bodies, proprietary source, or credential values as ordinary logging attributes.
