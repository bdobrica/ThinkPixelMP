# Dependency and lock resolution

## Dependency declaration

Each dependency has a stable name within its parent, a target logical artifact, a required/optional flag, source and catalog context, and exactly one selector:

1. exact digest;
2. exact strict semantic version;
3. bounded semantic-version range.

Mutable tags, branches, and unconstrained `latest` selectors are invalid production dependencies.

## Resolution inputs

Resolution binds an authenticated tenant, requested catalog, root artifact selector, optional-dependency selections, resolution algorithm version, and one immutable view of candidate catalog state.

Optional dependencies are excluded unless the request explicitly selects their dependency names. An unknown selection or selection of a non-optional edge is rejected.

## Deterministic algorithm

For each edge, MP:

1. applies exact-digest, exact-version, then range semantics according to the declared selector;
2. restricts candidates to the requested source/catalog context and exact tenant;
3. removes candidates that are ineligible under lifecycle, revocation, catalog, or applicable policy rules;
4. excludes prereleases from range matching unless a comparator explicitly includes a prerelease;
5. chooses the highest SemVer-precedence candidate;
6. fails if equally ranked candidates differ only by build metadata;
7. validates artifact kind compatibility and expands required plus explicitly selected optional edges;
8. detects cycles, contradictory exact identities, and incompatible constraints;
9. emits nodes and edges in canonical order and computes the lock digest.

The same immutable inputs must produce byte-identical canonical lock output.

## Lock contents

An ArtifactLock records the algorithm/profile version, root digest, every node's logical identity and exact digest, every resolved edge and selection reason, optional-dependency choices, catalog/source context, and canonical SHA-256 lock digest.

The lock is a dependency identity decision, not a union of declared capability or runtime authority. Each node's requirements remain separate and inspectable.
