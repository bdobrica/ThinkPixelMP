# Remote A2A agent contract

A `remote-agent` is a remote-service artifact represented by an immutable A2A Agent Card snapshot.

## Immutable descriptor

MP packages the validated Agent Card bytes and an MP descriptor into an OCI artifact with artifact type `application/vnd.thinkpixel.remote-agent.v1`. The OCI manifest digest is the ArtifactVersion identity; the card also receives its own descriptor digest.

The machine-readable [`RemoteAgentDescriptor` schema](../../api/schemas/remote-agent-descriptor-v1.schema.json) references the exact card layer and records its byte digest separately from normalized endpoint, protocol, skill, and capability fields. Publication fails if a normalized field disagrees with the packaged card; normalization never rewrites the card bytes.

The snapshot records the pinned A2A version, endpoint metadata present at observation time, declared skills/capabilities, publisher/source attribution, and signature material where available. It does not claim that the provider's implementation is inspectable or unchanged.

## Mutable endpoint evidence

Endpoint ownership, TLS verification, supported protocol negotiation, health, and observation time are refreshable EvidenceRecords bound to the immutable ArtifactVersion. Refreshing them never changes the historical Agent Card snapshot.

Remote-service assurance is displayed separately from local OCI assurance. An OCI-packaged card does not turn the remote implementation into a locally controlled artifact.

Runtime invocation and cross-organization delegation remain outside MP authority.
