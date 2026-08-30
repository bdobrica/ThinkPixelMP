# Phase 0 exit evidence

Date: 2026-08-30  
Baseline commit: `aa6b571` (`Bootstrap Phase 0 marketplace contracts`)

## Outcome

Phase 0 defines the product and authority boundary, hostile-input posture, immutable artifact identity, OCI profiles, five artifact descriptors, dependencies and deterministic locks/resolutions, trusted evidence and category normalization, catalog/policy/promotion/lifecycle semantics, federation and remote fetching, PostgreSQL tenancy, API/events, ThinkPixel integrations, operational targets, and supported versions.

The [enterprise-blueprint reconciliation](phase-0-enterprise-blueprint-reconciliation.md) found no authority conflict and documents the marketplace-specific additions required to preserve non-expansion, gateway enforcement, exact identity, revocation freshness, independent evidence, and fail-closed behavior.

All Phase 0 checklist entries ARC-001 through ARC-060 are satisfied by the baseline commit and this follow-up evidence record.

## Validation evidence

The following commands passed from the repository root after the TODO reconciliation:

```bash
./scripts/validate-phase0.sh
git diff --check
git status --short
```

`validate-phase0.sh` performs:

- `jq empty` against every JSON Schema file;
- Redocly OpenAPI 3.1 lint with unresolved-reference and contract-quality rules;
- a complete OpenAPI bundle, which traverses the externally referenced descriptor, evidence, event, snapshot, lock, API request, and response schema graph;
- Git whitespace validation.

The Redocly bundle may report deterministic component renames when independent external schemas reuse local `$defs` names. These are non-failing bundle namespace notices, not unresolved references or validation errors.

A repository secret-pattern scan covering common private-key, cloud-key, GitHub-token, Slack-token, and bearer-token forms found no matches outside ignored dependencies. `node_modules/` and generated OpenAPI bundles are ignored; the pinned `package-lock.json` is committed.

## Contract inventory

- ADRs: `docs/adr/0001` through `0010` plus the reusable template.
- Architecture: system context, trust boundaries, and PostgreSQL logical model.
- Security: invariants, threat model, authentication/administration, hostile fetching/inspection, registry credentials, and C0–C3 data handling.
- Contracts: artifact identity/taxonomy, all five descriptors, OCI/media types, dependencies/requirements/locks/resolutions, evidence/signatures/SBOM/normalization, catalogs/policy/promotion/snapshots, lifecycle, federation, events, external adapters, discovery, and ThinkPixel integrations.
- Machine contracts: OpenAPI 3.1 plus strict JSON Schema 2020-12 files under `api/schemas/` using `https://schemas.thinkpixel.io/thinkpixelmp/...` identifiers.
- Operations: SLO/capacity assumptions and pinned supported-version policy.

## Exit assessment

No known ambiguity remains in the Phase 0 exit areas identified by `PLAN.md`: artifact identity, authority boundaries, promotion semantics, evidence trust, or revocation. Implementation begins in Phase 1; Phase 0 documents do not claim that runtime code, migrations, adapters, or conformance tests already exist.
