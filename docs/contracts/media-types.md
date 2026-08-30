# ThinkPixel OCI media types

The following artifact types are reserved for version 1:

| Artifact kind | OCI `artifactType` |
| --- | --- |
| Agent Skill | `application/vnd.thinkpixel.skill.v1` |
| Agent runtime | `application/vnd.thinkpixel.agent-runtime.v1` |
| MCP server | `application/vnd.thinkpixel.mcp-server.v1` |
| Remote agent | `application/vnd.thinkpixel.remote-agent.v1` |
| Bundle | `application/vnd.thinkpixel.bundle.v1` |

Typed JSON descriptors use `application/vnd.thinkpixel.<kind>.manifest.v1+json`. Kind-specific archive payloads use `application/vnd.thinkpixel.<kind>.layer.v1+tar+gzip` where an archive is applicable.

The immutable dependency lock uses `application/vnd.thinkpixel.artifact-lock.v1+json`. The descriptor is the OCI config blob; payloads are OCI layers. ThinkPixelMP records the config descriptor digest separately from the OCI manifest digest that identifies the published ArtifactVersion.

Evidence and catalog artifact media types are defined by their own versioned contracts. A media type identifies syntax/profile only; it does not establish trust, eligibility, or authority.

Unknown artifact or layer media types fail closed during protected registration unless a future explicitly configured compatibility profile handles them.
