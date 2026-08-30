# OCI distribution profile

## Identity

ThinkPixelMP resolves every submitted OCI reference to a manifest digest before registration commits. The canonical ArtifactVersion payload identity is the resolved manifest digest, restricted to SHA-256 for the release candidate.

MP persists both the submitted source reference and the resolved immutable reference. Later movement of a mutable tag cannot alter the registered ArtifactVersion.

## Artifact shape

Each locally distributed artifact has one OCI 1.1 manifest with:

- a versioned, typed config or descriptor blob;
- zero or more bounded payload layers appropriate to its artifact kind;
- annotations used only as descriptive metadata;
- an exact manifest digest used as ArtifactVersion identity.

Open formats remain native inside their packaging. ThinkPixel-specific schemas are limited to contracts that open standards do not cover, including agent runtimes, bundles, locks/resolutions, catalog snapshots, and MP attestations.

Exact media-type constants and per-kind layer rules are intentionally specified in the corresponding versioned descriptor contracts.

## Referrers

Evidence artifacts set the exact subject manifest digest. Supported discovery order is:

1. OCI 1.1 Referrers API;
2. OCI referrers tag schema fallback when supported;
3. normalized PostgreSQL evidence plus immutable external raw-report references.

The third case does not authorize MP to synthesize proprietary registry state. Evidence ingestion always verifies that the declared subject equals the registered ArtifactVersion digest.

## RegistryProvider boundary

The domain port exposes reference normalization, tag-to-digest resolution, bounded manifest/blob reads, referrer discovery, capability reporting, and typed error classes. It does not expose ORAS or registry-library types.

Registry credentials are selected only from operator-controlled configuration scoped to a registry origin. Publisher descriptors cannot choose credentials. Credentials are not forwarded across an origin change or untrusted redirect.

## Failure posture

Digest mismatch, unsupported media type, oversized content, malformed descriptors, unsafe archive semantics, missing required evidence, or unverifiable subject binding fail closed for registration or protected promotion as applicable.
