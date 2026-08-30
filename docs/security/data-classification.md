# Data classification, retention, and redaction

## Classes

| Class | Meaning | MP examples | Default handling |
| --- | --- | --- | --- |
| C0 Public | Intentionally public and reviewed | Public documentation, problem-type definitions | May appear in public outputs |
| C1 Internal | Low-disclosure operational or published private-marketplace metadata | Artifact names, published catalog summaries, opaque IDs, stable reason codes | Authenticated access; bounded telemetry |
| C2 Confidential | Tenant, proprietary, or security-relevant content | Unpublished descriptors, requirements, source locations, evidence details, policy inputs/bundles | Least privilege, encryption in transit/at rest, no ordinary logs |
| C3 Restricted | Credentials or material that grants authority | Registry credentials, bearer tokens, cookies, private keys, signing material, secret-bearing headers | Minimum memory lifetime; never durable MP metadata or telemetry |

The highest applicable class wins. Unknown publisher/tenant content defaults to C2; credential-shaped values default to C3. A digest of C2 data is at least C1 and remains C2 when dictionary or correlation risk exists.

## Surface rules

- C3 is prohibited from PostgreSQL, outbox/dead-letter records, logs, traces, metrics, policy inputs, catalog snapshots, evidence summaries, audit payloads, and API responses.
- Raw descriptors, reports, policy inputs, request/response bodies, and remote error bodies are excluded from ordinary telemetry.
- Tenant, principal, artifact, resolution, and evidence IDs may appear only as bounded allowlisted log/trace attributes. They are prohibited as metric labels.
- Metrics use fixed low-cardinality labels such as operation, outcome, reason family, artifact kind, and dependency class.
- Raw evidence stays in an operator-configured OCI/object store with its own C2 access and retention controls.
- URL userinfo and sensitive query parameters are prohibited. Logs retain at most normalized origin and safe operation identity.

## Structural redaction

Redaction occurs before serialization at every adapter boundary. Secret-bearing types have no unrestricted string/debug/JSON representation. Recursive maps and headers redact case-insensitive credential names and configured sensitive fields. Error mapping emits stable codes and opaque correlation IDs, never upstream bodies or SQL details.

On a C3-capable path, uncertain serialization suppresses the questionable field or event and emits a separate safe security alert. Telemetry failure never triggers raw fallback logging.

## Default retention

| Record | Default |
| --- | --- |
| Immutable artifact identity, promotion, resolution, revocation, and audit facts | Until tenant deletion or governing legal policy |
| Normalized evidence | While referenced plus compliance policy |
| Raw external evidence reports | 90 days, operator configurable |
| Operational logs and traces | 30 days |
| Successfully delivered outbox records | 30 days |
| Dead-letter records | 90 days |
| Idempotency records | 24 hours |
| Resumable SSE event history | At least 7 days |

Legal hold overrides deletion/expiry where authorized. Retention jobs preserve referential integrity and the immutable fact/digest that a consequential decision used evidence even when optional raw content expires.

## Tenant deletion

Tenant deletion is an explicit authorized asynchronous workflow. It inventories legal holds and cross-record dependencies, prevents new tenant mutations, deletes tenant content according to policy, and produces auditable progress/result records.

Only minimized non-reversible platform security/accounting tombstones may remain where legally required. They cannot contain reusable tenant content, names, credentials, raw reports, descriptors, or reversible identifiers.

## Verification

Tests inject secret canaries through requests, descriptors, archives, reports, upstream errors, policy paths, database failures, panic recovery, logs, spans, metrics, audit, outbox, dead-letter, snapshots, and support diagnostics. Any unauthorized canary occurrence fails verification.
