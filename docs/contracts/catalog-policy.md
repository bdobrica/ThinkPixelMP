# Catalog and policy contract

## Catalog

A Catalog is tenant-scoped and uniquely named within its tenant. Its name is a label, not a universal assurance level. `production`, `experimental`, and other familiar names receive meaning only from the exact active policy and organizational controls.

A CatalogEntry binds one exact ArtifactVersion digest and immutable promotion decision. Membership is historically append-only. An entry may become `removed`, preventing new catalog resolution while preserving prior resolutions, reviews, audit, and evidence.

## Policy bundle

A policy bundle is an immutable OCI artifact identified by digest and signed by a configured trusted policy-administrator identity. It contains bounded Rego modules, data, manifest/profile version, named entrypoint, and declared input/output schema versions.

Activation validates syntax, bundle structure, signature/trust, allowed built-ins, entrypoint, schemas, and resource constraints before atomically making the exact digest active for a catalog. Activation history is never overwritten.

## Evaluator port

The domain `CatalogPolicyEvaluator` accepts a typed bounded input and returns a typed result. It exposes no OPA/library types. The reference adapter evaluates embedded OPA in process with a deadline, bounded input/output, restricted built-ins, and no network/filesystem/secret access.

## Typed input

Input contains:

- tenant and catalog identifiers;
- exact root artifact/version/digest and descriptor summary;
- immutable dependency lock and per-node requirements;
- publisher and namespace state;
- evidence IDs/digests, category, normalized conclusions, freshness, and bounded summaries;
- artifact lifecycle and live revocation facts;
- request/reviewer context required by policy;
- exact policy/evaluator profile version.

It excludes raw reports, credentials, tokens, signing keys, proprietary payloads, arbitrary URLs, and unbounded publisher data.

## Typed output

Output contains `allow`, stable reason codes, evidence requirements, reviewer count and separation obligations, and bounded operator-facing detail. MP rejects unknown/malformed output and independently enforces domain invariants, live safety checks, reviewer identity rules, and authorization.

Missing, malformed, unavailable, unsigned/untrusted, timed-out, or invalid policy fails closed for protected promotion.
