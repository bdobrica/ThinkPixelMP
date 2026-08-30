# Bundle contract

A Bundle is a composition artifact containing named ArtifactDependency declarations. It may depend on agent runtimes, skills, MCP servers, remote agents, and other bundles.

Nested bundles are allowed. Resolution rejects direct and transitive cycles, duplicate component names, contradictory identities, incompatible selectors, and disallowed artifact-kind relationships.

Authoring may retain exact versions or bounded ranges. Protected promotion and consumption bind an immutable ArtifactLock containing exact digests.

The machine-readable [`Bundle` schema](../../api/schemas/bundle-v1.schema.json) declares its permitted component kinds and the fixed `transitive-union-fail-on-conflict-v1` aggregation algorithm. Effective requirements are the deterministic union of the bundle's requirements and every selected transitive node. Conflicting requirements fail resolution.

Requirements also remain attached to their originating lock nodes. Their effective union is a compatibility/admission requirement, never a capability or network grant, and does not discard provenance. Optional dependencies follow explicit-selection semantics from the dependency-resolution contract.
