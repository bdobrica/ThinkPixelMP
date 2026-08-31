# ThinkPixelMP API

The canonical OpenAPI 3.1 document is [openapi/thinkpixelmp.yaml](openapi/thinkpixelmp.yaml). Reusable artifact and evidence schemas live in [schemas](schemas/).

The Phase 0 specification describes the intended release-candidate surface. A documented endpoint is not an implementation claim.

Edit the canonical source and referenced JSON Schemas, then run `make openapi-generate`. The committed [bundled contract](openapi/generated/thinkpixelmp.bundle.yaml) is generated for consumers that cannot resolve repository-relative references and MUST NOT be edited by hand. `make openapi-check` validates the canonical source, resolves all references, regenerates a candidate in a temporary directory, and fails when the committed bundle has drifted.
