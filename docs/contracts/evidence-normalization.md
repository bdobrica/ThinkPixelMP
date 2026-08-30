# Evidence normalization contracts

The shared machine-readable [`EvidenceSummary` schema](../../api/schemas/evidence-summary-v1.schema.json) is a closed, category-discriminated union. An accepted EvidenceRecord contains one such strict normalized summary. Producer-native fields remain only in the exact-digest raw report; normalizers do not copy arbitrary keys into domain state or policy input.

## Provenance

SLSA 1.2 provenance normalization preserves the v1 predicate identifier, exact subjects, builder identity, build type, invocation/config digest when present, and exact material digests. Subject matching is explicit. These facts remain policy inputs and never collapse into a generic trusted-build score.

## Vulnerabilities and licenses

Vulnerability findings normalize package name/PURL, advisory identity, fixed version when known, and the closed severity vocabulary `unknown`, `none`, `low`, `medium`, `high`, or `critical`. Counts and maximum severity are derived from the normalized findings. Scanner-specific scores, vectors, prose, and enrichment stay in the raw report.

License findings use SPDX expressions and a policy outcome of `allowed`, `denied`, or `unknown`. Summary flags expose whether any denied or unknown finding exists. An expression is evidence, not permission to redistribute.

## Agent evaluations

Agent evaluation evidence binds an evaluator suite and version, exact scenario-set digest, named metrics, units, configured comparison operators and thresholds, individual outcomes, and an overall outcome. MP does not compare unrelated suites, invent a universal score, or embed an evaluation engine.

For accepted evaluation evidence, the EvidenceRecord conclusion MUST equal the summary's overall outcome. Other categories derive the top-level conclusion through the configured, versioned normalizer. Any supplied or derived inconsistency is rejected rather than resolved by precedence.

## Other evidence

SPDX/CycloneDX summaries record the pinned format/version, component count, and exact-subject match. Human review binds the reviewer principal and review type. Remaining verification, analysis, compatibility, and endpoint categories use bounded named checks and closed outcomes. Category-specific schema evolution requires a new schema version.
