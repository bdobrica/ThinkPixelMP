# Search and discovery

Default discovery uses deterministic tenant-scoped structured filters and lexical search with stable pagination and ordering. Supported filters include namespace/publisher, artifact kind/class, semantic version, lifecycle, catalog membership, delivery model, evidence presence/conclusion/freshness, and requirement summaries.

Semantic ranking may be added as an optional browse aid. It is non-authoritative, visibly identified, and cannot determine catalog eligibility, promotion, dependency selection, or Run authority.

Search may return logical names, aliases, and versions for human discovery. Only the resolution endpoint applies catalog/lifecycle/policy rules and returns an exact immutable digest graph.

Search indexing is derived state. On index outage or lag, authoritative get/resolution behavior remains correct; stale ranking never becomes a fallback resolver.
