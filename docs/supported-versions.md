# Supported standards and versions

Status: Phase 0 compatibility baseline, reviewed 2026-08-29 against primary official sources.

| Standard/tool | Phase 0 baseline | Notes |
| --- | --- | --- |
| Agent Skills | Official specification snapshot reviewed 2026-08-29 | The public specification does not expose an independent specification release number; MP pins validator behavior and records the review snapshot |
| MCP protocol | `2026-07-28` | Matches the ThinkPixelTG Phase 0 contract |
| Official MCP Registry API | `1.0.0` | Imported metadata remains candidate state |
| A2A protocol | `1.0.0` | Agent Card snapshots record the exact protocol/profile version |
| OCI Image Specification | `1.1.1` | Uses OCI image manifest artifact guidance and `artifactType` |
| OCI Distribution Specification | `1.1.1` | Uses Referrers API and standardized referrers tag fallback |
| SPDX | `3.0` | Current stable specification; supported encodings require an explicit parser contract |
| CycloneDX | `1.7` | Current stable specification |
| SLSA | `1.2` | Current approved specification; provenance subjects remain exact-digest bound |
| Sigstore Cosign | `3.0.6` | Phase 1 pins the implementation/library dependency; Phase 0 pins verification semantics |
| CloudEvents | `1.0.2` | Event envelope only; does not itself supply authenticity or durability |
| JSON Schema | Draft 2020-12 | MP descriptor and API component schemas |
| OpenAPI | `3.1` | Canonical REST API description |
| RFC 8785 JCS | RFC 8785 | Hashable JSON canonicalization |
| Go | `1.26.7` | Compatibility-first Phase 1 pin; Go 1.27 evaluation waits for declared ORAS support |
| PostgreSQL | development/test `18.6`; production majors `17` and `18` | Production uses a currently maintained minor release; migration tests cover the supported majors |
| ORAS Go | `2.6.2` | Stable v2 adapter candidate; includes 2026 security hardening fixes |
| Open Policy Agent | `1.17.0` | Embedded reference adapter candidate |

## Primary references

- [Agent Skills specification](https://agentskills.io/specification)
- [MCP specification](https://modelcontextprotocol.io/specification/2026-07-28)
- [Official MCP Registry API](https://registry.modelcontextprotocol.io/docs)
- [A2A specification](https://a2a-protocol.org/v1.0.0/specification)
- [OCI Image Specification 1.1.1](https://specs.opencontainers.org/image-spec/?v=v1.1.1)
- [OCI Distribution Specification 1.1.1](https://specs.opencontainers.org/distribution-spec/?v=v1.1.1)
- [SPDX specifications](https://spdx.dev/use/specifications/)
- [CycloneDX specification](https://cyclonedx.org/specification/overview/)
- [SLSA 1.2](https://slsa.dev/spec/v1.2/)
- [Cosign releases](https://github.com/sigstore/cosign/releases)
- [CloudEvents](https://cloudevents.io/)

Exact implementation dependencies remain Phase 1 decisions and must be pinned with compatibility and security evidence. A standards upgrade is an explicit contract change, never an automatic consequence of updating a library.

Go 1.27 was current at review time but was not selected because the stable ORAS Go v2 line documented support through Go 1.26. Phase 1 must re-check compatibility and all current security advisories before creating the module lock.
