# Marketplace events

ThinkPixelMP emits CloudEvents 1.0.2 through a transactional outbox with at-least-once delivery. CloudEvents supplies the envelope, not authenticity, authorization, confidentiality, or durable retention by itself.

## Envelope

- `specversion`: `1.0`;
- `id`: stable UUIDv7 logical event ID;
- `source`: stable MP deployment/service URN;
- `type`: `io.thinkpixel.mp.<aggregate>.<event>.v1`;
- `subject`: tenant-safe stable resource identity without secret or proprietary labels;
- `time`: event commit time;
- `datacontenttype`: versioned JSON media type;
- `data`: bounded typed event payload containing exact digests and resource IDs.

The normative envelope and payload union is [`MarketplaceEvent` v1](../../api/schemas/marketplace-event-v1.schema.json). Payloads contain only tenant-safe IDs, exact digests, state transitions, bounded reason codes, and a transaction cursor. Complete descriptors, raw evidence, proprietary metadata, credentials, and free-form operator explanations are prohibited.

Initial event families include artifact registration/deprecation/quarantine/revocation, evidence acceptance, policy activation, promotion decision, catalog-entry state, immutable resolution, and import result.

Consumers deduplicate by event ID. Redelivery preserves the same logical event and payload digest.

Every committed tenant event receives a strictly increasing positive `sequence`. Ordering is defined only within one tenant; no cross-tenant or deployment-global order is claimed. The external SSE cursor is an opaque authenticated encoding of tenant and sequence, so clients cannot switch tenant scope or forge progress.

## SSE

`GET /v1/events` is authenticated and tenant scoped. It supports `Last-Event-ID`, authorized type/resource filters, heartbeat comments, bounded connection duration, and at least seven days of resumable retention.

When a cursor predates retained events, MP returns a typed reset-required response. Consumers reconcile through authoritative list/get/revocation APIs before resuming; they never assume that an event gap means no change.

SSE is a delivery mechanism, not the source of truth. PostgreSQL state and immutable records remain authoritative.
