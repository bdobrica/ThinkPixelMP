# Agent runtime artifact contract

An `agent-runtime` is an executable-local artifact described by the machine-readable [`agent-runtime` schema](../../api/schemas/agent-runtime-v1.schema.json). Its common envelope binds artifact coordinates, requirements, and dependencies. Its `spec` is structurally aligned with ThinkPixelAR `AgentRuntimeSpec` v1 and uses the ThinkPixel schema authority.

The contract contains an immutable image reference and matching SHA-256 digest, HarnessAdapter kind and compatibility, argv entrypoint metadata, paths beneath `/state`, the fixed `/workspace` mount, abstract Runtime Profile requirements, Linux architectures, `agentd` compatibility, and names—but never values—of declared non-secret environment inputs.

The schema is a materialization requirement, not a Pod, Sandbox, capability grant, network grant, credential bundle, or mutable operator configuration. ThinkPixelAR remains authoritative for resolving a compatible Runtime Profile and enforcing the resulting sandbox. A descriptor that names an unavailable adapter/profile/capability is valid for publication but cannot pass a resolution context requiring availability.

ThinkPixelMP MUST track the adopted ThinkPixelAR contract for semantic compatibility. A breaking AR schema change requires a new MP schema/media-type version; MP MUST NOT silently reinterpret an existing descriptor.
