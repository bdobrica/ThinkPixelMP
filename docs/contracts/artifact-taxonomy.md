# Artifact taxonomy

Artifact kind and artifact class are separate dimensions.

| Kind | Class | Initial delivery model | Description |
| --- | --- | --- | --- |
| `skill` | `instructional` or `executable-local` | `oci` | Agent Skill content preserved in its native format and packaged as a deterministic archive; MP derives the effective class |
| `agent-runtime` | `executable-local` | `oci` | ThinkPixel runtime manifest bound to an immutable runnable image and adapter contract |
| `mcp-server` | `executable-local` or `remote-service` | `oci` or `remote` | MCP server descriptor; publication never enables live tools or credentials |
| `remote-agent` | `remote-service` | `oci` plus remote endpoint | Immutable OCI snapshot of an A2A Agent Card; endpoint state remains separate evidence |
| `bundle` | `composite` | `oci` | Manifest composing exact or resolvable dependencies without merging their authority |

Delivery models are `oci`, `remote`, and `imported-source`. `imported-source` records origin and import history; locally repackaged output still receives an immutable local distribution identity.

MCP server delivery normalizes to `oci` or `remote`. npm, PyPI, Git, and similar locations are source/provenance mechanisms rather than additional runtime delivery authority.

Classification affects applicable validation and evidence. It does not grant authority or collapse distinct trust dimensions.
