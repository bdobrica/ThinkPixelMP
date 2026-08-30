# ADR 0010: Initial external adapters

- Status: accepted
- Date: 2026-08-29
- Decision owners: ThinkPixelMP maintainers
- Supersedes: none
- Superseded by: none

## Context

MP requires event export, raw-report storage, signing, discovery search, and identity mapping without making domain contracts dependent on one vendor.

## Decision

- The first event sink is an authenticated HTTP CloudEvents 1.0.2 webhook.
- The first external report-store adapter is S3-compatible; OCI report references are also native.
- Signing uses a neutral KMS interface. Production requires a KMS/HSM-compatible provider; development may use ephemeral local keys that cannot activate in production.
- RC browse search uses PostgreSQL full-text/trigram facilities. External or vector indexes are deferred.
- OIDC tenant/principal/role mapping is issuer-specific operator configuration; claim names are not globally hard-coded beyond standard token validation.

## Alternatives considered

- Kafka/NATS-first export was deferred because authenticated HTTP provides the smallest interoperable initial adapter.
- Cloud-vendor-specific signing/storage interfaces were rejected to preserve deployability.
- Elasticsearch/OpenSearch as a mandatory dependency was rejected until scale evidence requires it.

## Consequences

Ports remain vendor-neutral. HTTP sink receivers must be idempotent by stable event ID. PostgreSQL search is derived browse behavior and cannot become resolution authority.

## Security and privacy

Webhook destinations, object-store buckets, KMS identities, and OIDC mappings are operator-controlled. Publisher metadata cannot select them. Development keys and authentication are structurally rejected in production.

## Compatibility and migration

Additional adapters can be added without changing domain contracts. Search ranking changes do not change resolution semantics.

## Operations

Adapters expose typed health, timeout, retry, throttling, and permanent-failure classifications. Secrets remain in provider/configuration mechanisms, not MP records.
