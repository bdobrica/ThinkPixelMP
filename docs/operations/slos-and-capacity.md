# SLOs, recovery, and capacity assumptions

Status: Phase 0 release-candidate planning baseline. Phase 8 evidence may tighten targets but cannot weaken security failure posture without an explicit contract change.

## Availability

Authenticated control-plane requests that do not require an unavailable external registry, policy source, or evidence producer target 99.9% monthly availability. Every external dependency outcome remains measured and explicitly classified; it is not hidden from telemetry merely because it is outside the MP availability numerator.

## Latency objectives

Measured under the initial capacity assumptions:

| Operation | p95 | p99 |
| --- | ---: | ---: |
| Cached/get/list API | 250 ms | 1 s |
| Accepted mutation or operation creation | 500 ms | 2 s |
| Local immutable resolution requiring no new external fetch | 1 s | 3 s |
| Embedded catalog-policy evaluation | 100 ms | 500 ms |

Latency excludes client network time and separately reported external-provider time. Security validation, tenant isolation, redaction, digest verification, and policy checks are never disabled to meet latency.

## Asynchronous work

- Committed outbox event available for delivery: p95 within 5 seconds and p99 within 30 seconds.
- Publication, import, evidence verification, and resolution operations expose progress within 2 seconds.
- MP-owned bounded verification targets p95 completion within 60 seconds at default artifact limits.
- External scanner/evaluator completion has no MP SLO; MP reports waiting/dependency state distinctly.

## Recovery

Committed PostgreSQL control metadata targets RPO 0 with a correctly configured synchronous or managed high-availability deployment and RTO no more than 1 hour. Derived search/cache state may tolerate loss and is rebuilt from authoritative data.

OCI payload and raw-report recovery follows the external store's operator contract. MP recovery verifies referenced digests rather than assuming external bytes are unchanged.

## Initial deployment assumptions

- 100 tenants;
- 1,000,000 ArtifactVersions;
- 10,000,000 EvidenceRecords;
- 100 catalogs;
- ArtifactLocks up to 10,000 nodes;
- 100 sustained read requests per second;
- 20 sustained mutation requests per second;
- 100 concurrent external fetch/verification operations.

These are sizing/test assumptions, not product hard limits. Load, concurrency, storage-growth, outbox-lag, and recovery tests validate them before RC closure.

## Degraded behavior

Browsing and historical reads remain available during OPA, registry, or evidence-producer outages when PostgreSQL can serve them safely. Registration, fresh verification, protected promotion, and new protected resolution fail closed whenever a required dependency is unavailable.

`/readyz` fails only when MP cannot safely serve its configured role, including PostgreSQL unavailability or invalid required signing/policy initialization. Optional external-source outages appear in dependency health and metrics without making the entire API unready. `/livez` reports process liveness only.
