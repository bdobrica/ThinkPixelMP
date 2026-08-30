# SBOM ingestion profile

The release candidate accepts:

- SPDX 3.0 JSON-LD;
- SPDX 2.3 JSON for ecosystem compatibility;
- CycloneDX 1.7 JSON.

XML, RDF, protobuf, and SPDX tag-value parsing are deferred. An unsupported encoding remains an opaque untrusted report reference and cannot satisfy an SBOM-required policy.

Ingestion records the exact artifact subject digest, SBOM standard, specification version, encoding/media type, document digest, producer/tool identity and version where present, creation/observation time, component and relationship summary, declared completeness where supported, and immutable raw-report reference.

SBOM parsing never trusts component names, licenses, external URLs, or embedded signatures as separate authoritative evidence. Subject matching, producer trust, signature verification, license evidence, and vulnerability evidence remain independent checks.
