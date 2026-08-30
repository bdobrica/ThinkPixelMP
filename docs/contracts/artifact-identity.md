# Artifact and version identity

## Logical identity

An Artifact is a tenant-scoped logical identity named `{namespace}/{name}`. Its stable database identifier is UUIDv7, but the public logical name remains the human discovery key.

Artifact kind is fixed when the logical Artifact is created. Versions cannot change the Artifact between `skill`, `agent-runtime`, `mcp-server`, `remote-agent`, or `bundle`; a different kind requires a different logical identity.

## Immutable version identity

An ArtifactVersion binds:

- one Artifact;
- a canonical strict SemVer 2.0.0 version without a leading `v`;
- an artifact kind and class;
- submitted source metadata;
- a resolved immutable source reference;
- an authoritative content digest;
- an immutable descriptor digest where applicable;
- creation and publisher attribution.

The digest is production identity. Tags and semantic versions are mutable or human-facing lookup metadata and cannot replace it.

Prerelease and build metadata are preserved. The complete canonical semantic-version string is the version key; MP does not discard build metadata when enforcing version/digest uniqueness.

The release candidate accepts only digests matching:

```text
sha256:[0-9a-f]{64}
```

Uppercase hexadecimal, abbreviated digests, implicit algorithms, and non-SHA-256 algorithms are rejected. Digest types remain algorithm-aware so a future version can add algorithms without changing the meaning of existing identities.

Once registration commits, identity-bearing fields MUST NOT change. Corrections require a new ArtifactVersion. The pair `(tenant, artifact, semantic_version)` is permanently bound to its first accepted digest; a different digest is rejected even after deprecation, quarantine, or revocation.

Repeated identical registration MAY return the existing resource through idempotent semantics. It MUST NOT create a second identity or rewrite provenance.

The normalized descriptor repeats `{namespace, name, version}` as an integrity cross-check. Registration fails if any coordinate differs from the requested ArtifactVersion or resolved OCI repository context. Descriptor coordinates never select, redirect, or override marketplace identity.

Registration supplies the attributed `publisher_id`, strict version, typed source, and optional expected digest. MP obtains the descriptor from the immutable OCI/import source rather than trusting an unrelated inline descriptor. It resolves and verifies the authoritative digest asynchronously before commit. Expected-digest mismatch creates no ArtifactVersion.

Caller authentication, attributed publisher, and namespace ownership are independent checks. Naming a Publisher does not authorize the caller to publish for it.

All evidence, locks, resolutions, promotion decisions, catalog entries, and revocations reference the exact artifact digest and stable ArtifactVersion identifier.

## Schema and protocol namespaces

- Canonical schema IDs: `https://schemas.thinkpixel.io/thinkpixelmp/...`
- ThinkPixel OCI media types: `application/vnd.thinkpixel.*`
- Release-candidate HTTP API base: `/v1`
- OpenAPI dialect: 3.1
- Error format: RFC 7807 Problem Details
