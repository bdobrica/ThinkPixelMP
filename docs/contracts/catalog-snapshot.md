# CatalogSnapshot contract

A CatalogSnapshot is a signed immutable OCI artifact scoped to exactly one tenant and catalog. Its machine-readable representation is the [`CatalogSnapshot` schema](../../api/schemas/catalog-snapshot-v1.schema.json).

It contains:

- snapshot ID, schema/profile version, tenant/catalog stable IDs, and creation time;
- exact active CatalogEntry identities and artifact digests;
- immutable ArtifactLock and resolution references/digests where applicable;
- applicable evidence record/report digests and bounded eligibility summaries;
- effective deprecation, quarantine, and revocation records;
- policy bundle and activation digests;
- promotion decision and MP attestation references;
- optional parent snapshot digest.

It excludes artifact payload bytes, credentials, raw evidence reports, secrets, and unrelated tenant data. Payload and referrer copying is a separate OCI mirror/controlled-transfer operation.

Every snapshot contains the complete catalog view. An optional parent digest records ancestry and supports audit or transfer optimization only; verification and reconstruction never require the parent. Verification validates the snapshot's own manifest/content/signature, schema, tenant/catalog binding, and every included digest.

Before hashing, entries are sorted by artifact digest then CatalogEntry ID; lifecycle records by artifact digest; and nested digest lists lexicographically. `snapshot_digest` is SHA-256 over RFC 8785 canonical JSON with `snapshot_digest` and `signing_identity` omitted. The final document inserts both fields. Its Cosign signature is an external OCI referrer, so changing or rotating the signer does not change snapshot content identity.

Disconnected import verifies snapshot signer policy and contents before making a local candidate snapshot available; it does not automatically activate a catalog or grant runtime authority.
