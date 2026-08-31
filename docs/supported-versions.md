# Supported standards and versions

Status: Phase 1 compatibility baseline, reviewed 2026-08-30 against primary official sources.

| Standard/tool | Baseline | Notes |
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
| Go | language/module baseline `1.26.0`; toolchain `1.26.7` | `go.mod` pins the exact development/build toolchain while retaining Go 1.26 language semantics; ORAS Go v2 supports Go 1.26 |
| Staticcheck | `2026.2.1` (`honnef.co/go/tools` `v0.8.1`) | Pinned static-analysis gate with Go 1.26 support |
| govulncheck | `v1.7.0` | Official Go call-graph-aware vulnerability scanner |
| go-licenses | `v2.0.1` | Classifies runtime and test dependency licenses against repository policy |
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

Implementation dependencies are pinned as their Phase 1 items introduce them, with compatibility and security evidence. A standards upgrade is an explicit contract change, never an automatic consequence of updating a library.

Go 1.27 was current at review time but was not selected because the stable ORAS Go v2 line documents support for Go 1.25 and 1.26. Go 1.26.7 was released 2026-08-19 with `net/http` fixes and remains the exact Phase 1 toolchain pin. ORAS Go v2.6.2 is the selected future adapter baseline because it contains the current content-extraction and registry-pagination security fixes; it is not added to `go.mod` until the item that implements the OCI adapter.
