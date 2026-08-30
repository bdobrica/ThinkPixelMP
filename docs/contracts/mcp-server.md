# MCP server artifact contract

An `mcp-server` ArtifactVersion is immutable discovery and supply-chain metadata. It does not configure ThinkPixelTG, grant credentials, expose live tools, or authorize a Run.

## Canonical delivery modes

- `oci`: an immutable locally operable server package or image addressed by digest.
- `remote`: an HTTPS endpoint descriptor whose ownership, TLS, protocol, and health assertions are separate evidence.

npm, PyPI, Git, and other ecosystems are import/provenance sources. Protected promotion requires the imported server to be normalized into an immutable `oci` or `remote` descriptor snapshot.

The descriptor records protocol compatibility, delivery details, declared authentication mechanism, and bounded tool metadata where available. Authentication declarations never select an enterprise credential or credential provider.

The machine-readable [`MCPServerDescriptor` schema](../../api/schemas/mcp-server-descriptor-v1.schema.json) is a closed union: `oci` uses an immutable image digest and `stdio`; `remote` uses an HTTPS endpoint and MCP `streamable-http`. Optional tool entries are discovery snapshots only and do not override schemas observed and governed by TG.

ThinkPixelTG onboarding is an explicit administrator action after marketplace qualification. TG remains authoritative for destinations, live tool schemas, risk classification, credentials, policy, and runtime transport.
