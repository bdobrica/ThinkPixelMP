# ThinkPixelMP Release-Candidate TODO

This is the chronological implementation checklist for ThinkPixelMP.

Execute the first unchecked item whose dependencies are complete.

An item is checked only after its acceptance evidence passes.

Follow the coding-agent and commit protocol in `PLAN.md`.

Status notation:

- `[ ]` pending
- `[x]` implemented and verified

Completion metadata format:

    — completed YYYY-MM-DD, commit <sha>, evidence: <commands/artifacts>

---

## Phase 0 — Decisions, threats, and contracts

- [x] ARC-001 Create `docs/` structure and ADR template covering context, decision, alternatives, consequences, security, compatibility, operations, and references. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-002 Write system-context diagram covering publishers, clients, MP, PostgreSQL, OCI registries, signature infrastructure, scanners/evaluators, OPA, AG, AR, TG, federation sources, and evidence sinks. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-003 Write threat model assuming malicious publishers, artifacts, OCI metadata/layers, dependencies, remote endpoints, federation sources, and forged evidence. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-004 Define normative invariants: marketplace eligibility != Run authority; requirement != grant; installation != capability expansion; digest identity; evidence exact-subject binding. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-005 Define trust-boundary table for MP, AG, AR, TG, LLMGW, GR, OCI registries, scanners, evaluators, and publishers. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-006 Define artifact taxonomy: `skill`, `agent-runtime`, `mcp-server`, `remote-agent`, and `bundle`. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-007 Define artifact classes `instructional`, `executable-local`, `remote-service`, and `composite`. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-008 Define delivery-model vocabulary and relationships to artifact kinds. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-009 Define Publisher domain model, verification states, suspension/revocation, and private-enterprise first-release semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-010 Define Namespace ownership/delegation rules and collision behavior. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-011 Define immutable Artifact/ArtifactVersion identity rules including semantic version, mutable tags, re-publication, and digest conflicts. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-012 Define OCI 1.1 distribution profile and media types required by ThinkPixel-specific artifacts. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-013 Define OCI referrer/evidence attachment strategy and fallback for registries with incomplete referrer support. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-014 Define Agent Skill OCI packaging profile while preserving Agent Skills semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-015 Define `agent-runtime` manifest including immutable image digest, adapter compatibility, Workspace/state paths, abstract runtime/network/capability requirements, and architecture. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-016 Define MCP artifact descriptor profile and delivery-mode normalization. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-017 Define remote-agent representation using immutable A2A Agent Card snapshots and endpoint metadata. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-018 Define Bundle manifest and dependency semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-019 Define ArtifactRequirement schema for capabilities, runtime, network, model/protocol, and integration requirements. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-020 Define ArtifactDependency schema including exact digest, exact version, semver range, source/catalog context, and optional/required edges. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-021 Define deterministic dependency-resolution algorithm and tie-breaking rules. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-022 Define ArtifactLock immutable graph schema and lock digest. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-023 Define ArtifactResolution semantics, evidence snapshot behavior, and AG consumption contract. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-024 Define EvidenceRecord schema, trusted producer identity, evidence categories, freshness, signature, normalized result, and raw-report references. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-025 Define distinction between publisher declaration and trusted evidence. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-026 Define signature-verification contract and Sigstore/Cosign identity/trust semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-027 Define SLSA-compatible provenance normalization. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-028 Define SPDX/CycloneDX SBOM metadata normalization. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-029 Define vulnerability/security/license evidence normalization and severity/freshness rules. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-030 Define agent-evaluation evidence contract without embedding an evaluation engine into MP. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-031 Define Catalog, CatalogEntry, catalog policy, and contextual eligibility semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-032 Define artifact lifecycle independently from catalog membership: active, deprecated, quarantined, revoked. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-033 Define PromotionRequest/Review/Decision state machine, four-eyes controls, and idempotency. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-034 Define deprecation semantics and replacement reference behavior. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-035 Define quarantine semantics. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-036 Define append-only digest revocation semantics, reason/severity, replacement, and downstream event behavior. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-037 Define `CatalogPolicyEvaluator` typed contract and OPA/Rego reference adapter decision. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-038 Define marketplace administrative authorization separately from catalog eligibility policy. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-039 Define ImportSource/ImportRecord model and federation trust boundaries. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-040 Define safe import rules for OCI, MCP Registry, A2A, and future Git/plugin sources. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-041 Define central remote fetcher/SSRF security contract including address ranges, redirects, TLS, DNS resolution, response limits, and credential forwarding. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-042 Define safe OCI/archive inspection limits for manifests, layers, files, decompression, path traversal, symlinks, and media types. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-043 Define RegistryProvider interface independent of ORAS/library types. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-044 Define registry credential ownership, scoping, and rotation requirements. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-045 Define catalog OCI snapshot/export format and relationship to actual payload mirroring. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-046 Define ThinkPixelMP-generated promotion attestation semantics without impersonating publisher identity. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-047 Define OIDC/JWT authentication, tenant mapping, principal roles, and administrative authorization model. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-048 Define PostgreSQL schema model, tenant scope, immutable records, transactional outbox, audit, and migration strategy. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-049 Define API conventions: OpenAPI 3.1, RFC 7807, UUIDv7, pagination, idempotency, tracing, limits, and SSE. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-050 Draft OpenAPI for discovery, publication, evidence, catalogs, promotion, resolution, lifecycle, import, and event APIs. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-051 Define AG integration including immutable resolution consumption, caching/freshness responsibilities, and MP-unavailable behavior. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-052 Define AG response to MP digest revocation without making MP authoritative for live Run state. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-053 Define AR integration: exact digest only, no marketplace search/latest resolution at runtime. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-054 Define TG integration: marketplace discovery/onboarding metadata cannot grant tool availability or credentials. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-055 Define search/discovery semantics and explicitly separate browse ranking from authoritative resolution. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-056 Define data-classification/redaction rules for artifact metadata, proprietary descriptors, evidence, registry credentials, signing material, audit, logs, and traces. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-057 Define target SLOs/capacity assumptions for API, registry resolution, evidence verification, promotion, resolution, import, and events. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-058 Define supported-version policy for Go, PostgreSQL, OCI Distribution, ORAS, Cosign/Sigstore, OPA, and supported descriptor standards. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-059 Reconcile Phase 0 against the enterprise-agent blueprint and document any additions required for Marketplace invariants. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh
- [x] ARC-060 Validate all schemas/OpenAPI/docs and commit Phase 0 evidence. — completed 2026-08-30, commit aa6b571, evidence: docs/evidence/phase-0-exit.md; ./scripts/validate-phase0.sh

---

## Phase 1 — Engineering foundation

- [x] ENG-001 Initialize Go module using a supported pinned Go release. — completed 2026-08-30, commit 32e5b3b, evidence: `docs/evidence/eng-001-go-module.md`; `GOTOOLCHAIN=go1.26.7 go version`; `GOTOOLCHAIN=go1.26.7 go list -m`; `GOTOOLCHAIN=go1.26.7 go mod edit -json`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-002 Create repository structure matching domain/app/ports/adapters separation in `PLAN.md`. — completed 2026-08-30, commit f30ac05, evidence: `docs/evidence/eng-002-repository-structure.md`; `gofmt -l cmd internal test`; `GOCACHE=/tmp/thinkpixelmp-eng002-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng002-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-003 Add dependency/source/license policy. — completed 2026-08-30, commit 965aefb, evidence: `docs/evidence/eng-003-dependency-policy.md`; `GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng003-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-004 Implement typed configuration loading with safe defaults, validation, secret references, and redaction. — completed 2026-08-30, commit 80a4480, evidence: `docs/evidence/eng-004-typed-configuration.md`; `test -z "$(gofmt -l cmd internal scripts test)"`; `GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `GOCACHE=/tmp/thinkpixelmp-eng004-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-005 Implement structured logging with request/trace/tenant/artifact correlation and secret redaction. — completed 2026-08-30, commit a2e83e8, evidence: `docs/evidence/eng-005-structured-logging.md`; `test -z "$(gofmt -l cmd internal scripts test)"`; `GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/telemetry/logging`; `GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `GOCACHE=/tmp/thinkpixelmp-eng005-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-006 Implement Prometheus registry and OpenTelemetry initialization without artifact/evidence payload capture by default. — completed 2026-08-31, commit 425d1cf, evidence: `docs/evidence/eng-006-metrics-and-tracing.md`; `test -z "$(gofmt -l cmd internal scripts test)"`; `GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/telemetry/...`; `GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `GOCACHE=/tmp/thinkpixelmp-eng006-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-007 Implement shared primitives: UUIDv7, injectable clock, typed digest, typed artifact reference, typed errors, bounded strings, authenticated pagination cursors. — completed 2026-08-31, commit 25aee64, evidence: `docs/evidence/eng-007-shared-primitives.md`; `test -z "$(gofmt -l cmd internal scripts test)"`; `GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/shared ./internal/ports/clock`; `GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `GOCACHE=/tmp/thinkpixelmp-eng007-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-008 Implement baseline HTTP server with request IDs, W3C tracing, panic recovery, RFC 7807, limits, timeouts, graceful shutdown, `/livez`, `/readyz`, and `/metrics`. — completed 2026-08-31, commit 6f82cba, evidence: `docs/evidence/eng-008-http-server.md`; `test -z "$(gofmt -l cmd internal scripts test)"`; `GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go test -race ./internal/adapters/http`; `GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go test ./...`; `GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go vet ./...`; `GOCACHE=/tmp/thinkpixelmp-eng008-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck`; `./scripts/validate-phase0.sh`; `git diff --check`
- [x] ENG-009 Add OpenAPI generation/validation and drift checks. — completed 2026-08-31, commit 1b8a8ed, evidence: `docs/evidence/eng-009-openapi-generation.md`; `./scripts/openapi.sh check`; `npm run validate:openapi`; `GOCACHE=/tmp/thinkpixelmp-eng009-go-cache GOTOOLCHAIN=go1.26.7 make verify`; `git diff --check`
- [x] ENG-010 Create root Makefile as stable developer/CI interface. — completed 2026-08-31, commit 2c8b415, evidence: `docs/evidence/eng-010-makefile-interface.md`; `make help`; `GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make fmt generate test-integration test-contract test-security test-e2e dependency-check test-race`; `GOCACHE=/tmp/thinkpixelmp-eng010-go-cache GOTOOLCHAIN=go1.26.7 make verify`; `git diff --check`
- [ ] ENG-011 Add format, vet/static, unit, race, dependency, vulnerability, license, OpenAPI, binary-build verification.
- [ ] ENG-012 Add PostgreSQL development dependency and explicit migration command.
- [ ] ENG-013 Create `thinkpixelmpctl` CLI skeleton using API client rather than direct DB access.
- [ ] ENG-014 Create baseline hardened non-root `thinkpixelmp` container image.
- [ ] ENG-015 Add CI with least-privilege jobs and immutable action/version pins where practical.
- [ ] ENG-016 Add repository hygiene checks preventing registry credentials, signing keys, OIDC tokens, private evidence, and test secrets from being committed.
- [ ] ENG-017 Start `docs/supported-versions.md`.
- [ ] ENG-018 Verify clean checkout baseline with generation, formatting, static checks, unit/race, OpenAPI, vulnerability/license, build, and image smoke.
- [ ] ENG-019 Publish `docs/phase-1-evidence.md` and commit Phase 1.

---

## Phase 2 — Authoritative persistence, identity, and publication state

- [ ] DB-001 Add migration framework and initial tenant schema.
- [ ] DB-002 Add Publisher tables/domain/repository.
- [ ] DB-003 Add Namespace tables/domain/repository with tenant-scoped ownership uniqueness.
- [ ] DB-004 Add Artifact tables/domain/repository.
- [ ] DB-005 Add immutable ArtifactVersion tables/domain including digest, semantic version, artifact kind, delivery model, and lifecycle.
- [ ] DB-006 Add database guards preventing ArtifactVersion payload/digest mutation after registration.
- [ ] DB-007 Add ArtifactSource metadata and immutable resolved source digest/reference.
- [ ] DB-008 Add ArtifactDescriptor persistence using bounded normalized metadata.
- [ ] DB-009 Add ArtifactRequirement persistence.
- [ ] DB-010 Add ArtifactDependency persistence.
- [ ] DB-011 Add AuditEvent schema and transactionally coupled mutation audit.
- [ ] DB-012 Add IdempotencyRecord with tenant/principal/action/request-digest ownership.
- [ ] DB-013 Add transactional OutboxMessage with replay-safe claiming/retry/dead-letter metadata.
- [ ] DB-014 Add transaction manager and repository interfaces.
- [ ] DB-015 Add optimistic concurrency where mutable administrative state requires it.
- [ ] IAM-001 Implement OIDC/JWT verification with issuer/audience/algorithm/expiry/clock-skew validation.
- [ ] IAM-002 Implement claim-to-tenant/principal mapping.
- [ ] IAM-003 Implement marketplace administrative roles/actions for publisher, namespace, publication, evidence producer, reviewer, catalog admin, revocation admin, federation admin.
- [ ] IAM-004 Implement explicitly configured local development auth mode that cannot activate under production config accidentally.
- [ ] PUB-001 Implement Publisher create/read/list API.
- [ ] PUB-002 Implement Namespace create/read/list API.
- [ ] PUB-003 Implement namespace delegation/ownership checks.
- [ ] PUB-004 Implement Artifact create/read/list API.
- [ ] PUB-005 Implement ArtifactVersion registration skeleton requiring immutable digest/source data.
- [ ] DB-016 Add migration-from-empty tests against real pinned PostgreSQL.
- [ ] DB-017 Add tenant-isolation tests across all repositories.
- [ ] DB-018 Add concurrent namespace/artifact/version registration tests.
- [ ] DB-019 Add rollback tests for partially completed registration.
- [ ] DB-020 Add concurrent idempotency/outbox replay tests.
- [ ] DB-021 Add property tests proving registered ArtifactVersion digest/descriptor identity cannot change.
- [ ] DB-022 Commit Phase 2 with persistence/identity evidence.

---

## Phase 3 — OCI registry and descriptor substrate

- [ ] OCI-001 Implement `RegistryProvider` domain port.
- [ ] OCI-002 Select/pin ORAS implementation version and document adapter choice.
- [ ] OCI-003 Implement registry reference parsing and normalization.
- [ ] OCI-004 Implement mutable-tag-to-digest resolution during registration.
- [ ] OCI-005 Persist both submitted source reference and resolved immutable digest.
- [ ] OCI-006 Implement manifest fetch with media-type and size limits.
- [ ] OCI-007 Implement OCI 1.1 referrer discovery.
- [ ] OCI-008 Implement bounded artifact/layer reader.
- [ ] OCI-009 Implement registry authentication with per-registry scoped credential configuration.
- [ ] OCI-010 Prevent credentials being forwarded across untrusted redirect/origin boundaries.
- [ ] OCI-011 Add local disposable OCI registry integration environment.
- [ ] SEC-001 Implement shared hostile archive inspector with file-count, layer-size, decompression-ratio, and extraction limits.
- [ ] SEC-002 Reject `../` path traversal and absolute paths.
- [ ] SEC-003 Handle symlinks/hardlinks without escape.
- [ ] SEC-004 Reject device/special file semantics where artifact profile forbids them.
- [ ] SEC-005 Ensure descriptor inspection never executes artifact scripts/hooks/binaries.
- [ ] DSC-001 Implement generic ArtifactDescriptor validator registry.
- [ ] DSC-002 Implement Agent Skill descriptor/package validator.
- [ ] DSC-003 Implement ThinkPixel `agent-runtime` manifest validator.
- [ ] DSC-004 Implement MCP server descriptor validator.
- [ ] DSC-005 Validate requirement vocabulary independently from any runtime grant.
- [ ] DSC-006 Validate runtime requirements as abstract requirements, rejecting privileged Kubernetes/runtime configuration fields.
- [ ] DSC-007 Record descriptor digest separately from artifact payload digest where appropriate.
- [ ] PUB-006 Complete ArtifactVersion registration: OCI resolve → safe descriptor inspect → validate → immutable commit.
- [ ] PUB-007 Reject same logical immutable version with conflicting digest according to Phase 0 policy.
- [ ] PUB-008 Ensure tag mutation upstream cannot change an existing registered version.
- [ ] OCI-012 Test valid OCI image/artifact registration.
- [ ] OCI-013 Test mutated tag behavior.
- [ ] OCI-014 Test digest mismatch.
- [ ] OCI-015 Test malicious/oversized manifest.
- [ ] OCI-016 Test decompression bomb protection.
- [ ] OCI-017 Test path/symlink attack fixtures.
- [ ] OCI-018 Test registry outage/latency/timeout/retry classifications.
- [ ] OCI-019 Verify no artifact code executes during registration.
- [ ] OCI-020 Commit Phase 3 with OCI/security evidence.

---

## Phase 4 — Supply-chain evidence and verification

- [ ] EVD-001 Add EvidenceRecord schema/domain/repository.
- [ ] EVD-002 Add trusted EvidenceProducer identity/configuration model.
- [ ] EVD-003 Enforce evidence subject as exact immutable artifact digest.
- [ ] EVD-004 Add evidence digest/raw-reference fields and bounded normalized summaries.
- [ ] EVD-005 Add evidence observation time, creation time, optional expiry/freshness deadline.
- [ ] EVD-006 Implement evidence ingestion idempotency by producer/event ID.
- [ ] EVD-007 Prevent publisher declarations from automatically becoming trusted scanner/evaluator evidence.
- [ ] SIG-001 Implement `SignatureVerifier` port.
- [ ] SIG-002 Implement Sigstore/Cosign-compatible OCI signature verification adapter.
- [ ] SIG-003 Record signer identity, issuer, transparency/certificate metadata where available, and verification result.
- [ ] SIG-004 Implement configurable namespace/signer trust policy input.
- [ ] SIG-005 Test cryptographically valid but policy-untrusted signer.
- [ ] SIG-006 Test signature against wrong digest.
- [ ] PRV-001 Implement provenance evidence parser/normalizer.
- [ ] PRV-002 Support SLSA-compatible subject/source/builder metadata.
- [ ] PRV-003 Verify provenance subject digest matches ArtifactVersion.
- [ ] SBOM-001 Implement SPDX SBOM metadata parser.
- [ ] SBOM-002 Implement CycloneDX SBOM metadata parser.
- [ ] SBOM-003 Discover SBOM OCI referrers where available.
- [ ] VUL-001 Implement normalized vulnerability evidence ingestion.
- [ ] VUL-002 Record scanner version/database revision/timestamp/severity summary.
- [ ] LIC-001 Implement normalized license evidence ingestion.
- [ ] SEC-006 Implement malware/static-analysis evidence categories.
- [ ] EVA-001 Implement generic agent-evaluation evidence category without embedding evaluator execution.
- [ ] EVA-002 Record evaluation definition/version, evaluator identity, result, subject digest, timestamp.
- [ ] REVW-001 Implement human-review EvidenceRecord category.
- [ ] EVD-008 Implement evidence-list/read API.
- [ ] EVD-009 Implement trusted producer evidence-write API.
- [ ] EVD-010 Implement evidence freshness evaluation helpers.
- [ ] EVD-011 Add tests for forged producer identity.
- [ ] EVD-012 Add tests for stale evidence.
- [ ] EVD-013 Add tests proving evidence for digest A cannot qualify digest B.
- [ ] EVD-014 Add referrer poisoning/malformed evidence tests.
- [ ] EVD-015 Add log/trace redaction tests for raw evidence/secrets.
- [ ] EVD-016 Commit Phase 4 with evidence verification artifacts.

---

## Phase 5 — Catalogs, policy, promotion, and revocation

- [ ] CAT-001 Add Catalog schema/domain/repository.
- [ ] CAT-002 Add CatalogEntry schema/domain referencing exact ArtifactVersion.
- [ ] CAT-003 Add catalog create/read/list API.
- [ ] CAT-004 Add catalog entry query/search API.
- [ ] POL-001 Define typed `CatalogPolicyEvaluator` interface in code.
- [ ] POL-002 Implement OPA/Rego adapter.
- [ ] POL-003 Add immutable PolicyBundle/PolicyActivation metadata.
- [ ] POL-004 Verify/validate policy before activation.
- [ ] POL-005 Implement fail-closed protected promotion on missing/malformed/unavailable policy.
- [ ] POL-006 Implement typed policy input containing exact digest, artifact metadata, dependency graph, evidence summaries/freshness, publisher state, and catalog.
- [ ] POL-007 Ensure policy input excludes secrets and unbounded raw artifact data.
- [ ] POL-008 Implement typed output with allow, reason codes, obligations/reviewer requirements, and evidence requirements.
- [ ] PROM-001 Add PromotionRequest schema/domain/state machine.
- [ ] PROM-002 Snapshot exact requested artifact digest and dependency lock at PromotionRequest creation.
- [ ] PROM-003 Snapshot relevant evidence IDs/digests for deterministic decision evidence.
- [ ] PROM-004 Add PromotionReview schema/domain.
- [ ] PROM-005 Enforce reviewer authorization.
- [ ] PROM-006 Enforce distinct reviewers where policy requires four-eyes approval.
- [ ] PROM-007 Prevent requester from satisfying forbidden self-approval combinations.
- [ ] PROM-008 Implement PromotionDecision transaction creating CatalogEntry and audit/outbox record atomically.
- [ ] PROM-009 Make duplicate/concurrent promotion idempotent.
- [ ] PROM-010 Implement promotion-request/read/review/decide APIs.
- [ ] LIFE-001 Implement deprecation record and API.
- [ ] LIFE-002 Implement quarantine record and API.
- [ ] REV-001 Implement append-only Revocation schema/domain.
- [ ] REV-002 Implement digest-level revocation API with reason/severity/replacement.
- [ ] REV-003 Ensure revocation does not delete ArtifactVersion, Catalog history, Evidence, or PromotionDecision.
- [ ] REV-004 Exclude revoked artifacts from protected resolution.
- [ ] REV-005 Emit artifact revocation outbox event.
- [ ] REV-006 Implement revocation listing/history API.
- [ ] POL-009 Add golden policy tests: trusted signature required.
- [ ] POL-010 Add golden policy tests: SBOM required.
- [ ] POL-011 Add golden policy tests: critical vulnerability denial.
- [ ] POL-012 Add golden policy tests: stale vulnerability evidence denial.
- [ ] POL-013 Add golden policy tests: license denial.
- [ ] POL-014 Add golden policy tests: evaluation required.
- [ ] POL-015 Add golden policy tests: unverified publisher denial.
- [ ] POL-016 Add golden policy tests: two-reviewer requirement.
- [ ] PROM-011 Add PostgreSQL concurrency tests for reviews/promotion/revocation races.
- [ ] PROM-012 Verify contextual approval: same digest permitted in one catalog and denied in another.
- [ ] PROM-013 Verify deprecated and revoked semantics remain distinct.
- [ ] PROM-014 Commit Phase 5 with production-catalog policy evidence.

---

## Phase 6 — Dependency resolution and ThinkPixel-integrated MVP

- [ ] DEP-001 Implement deterministic dependency graph builder.
- [ ] DEP-002 Implement semantic-version constraint parser/resolver according to Phase 0 rules.
- [ ] DEP-003 Prefer exact digest over mutable/version constraints.
- [ ] DEP-004 Implement cycle detection.
- [ ] DEP-005 Implement transitive dependency resolution.
- [ ] DEP-006 Prevent dependency fallback to unauthorized/unconfigured public sources.
- [ ] DEP-007 Reject revoked dependency.
- [ ] DEP-008 Reject dependency not eligible in required catalog context.
- [ ] DEP-009 Reject incompatible artifact-kind relationships.
- [ ] DEP-010 Implement deterministic ArtifactLock serialization.
- [ ] DEP-011 Implement ArtifactLock digest.
- [ ] DEP-012 Persist immutable ArtifactLock.
- [ ] RES-001 Add ArtifactResolution schema/domain/repository.
- [ ] RES-002 Implement resolution creation against exact catalog/policy context.
- [ ] RES-003 Persist root digest, ArtifactLock digest, policy reference, catalog reference, and evidence/promotion snapshot identifiers.
- [ ] RES-004 Make completed resolution immutable.
- [ ] RES-005 Add `POST /v1/resolutions`.
- [ ] RES-006 Add `GET /v1/resolutions/{id}`.
- [ ] RES-007 Add resolution-by-exact-digest/canonical-marketplace-reference input.
- [ ] RES-008 Explicitly reject unresolved mutable `latest` as final production resolution.
- [ ] DEP-013 Add property tests for graph determinism.
- [ ] DEP-014 Add randomized graph/cycle tests.
- [ ] DEP-015 Add conflicting semver tests.
- [ ] DEP-016 Add dependency-confusion tests.
- [ ] TAG-001 Add ThinkPixelAG client port/adapter.
- [ ] TAG-002 Define/implement secure MP↔AG service authentication.
- [ ] TAG-003 Implement AG consumption of immutable marketplace resolution using actual AG extension required by contract.
- [ ] TAG-004 Ensure AG persists exact artifact digests rather than marketplace aliases/tags.
- [ ] TAG-005 Expose declared capability/runtime requirements to AG as metadata only.
- [ ] TAG-006 Verify MP cannot cause AG capability expansion merely by artifact declaration.
- [ ] TAG-007 Publish revocation events consumable by AG.
- [ ] TAG-008 Add AG integration test: approved digest admitted.
- [ ] TAG-009 Add AG integration test: artifact present in MP but Run lacks capability → AG denies/narrows independently.
- [ ] TAG-010 Add AG integration test: revoked artifact denied for future Runs.
- [ ] TAG-011 Add MP outage test proving previously durably resolved/authorized runtime path follows AG's documented behavior rather than synchronously requiring MP.
- [ ] TAR-001 Document AR contract: AR accepts exact authorized agent-runtime digest, never marketplace `latest`.
- [ ] TTG-001 Document TG onboarding contract for Marketplace MCP artifacts.
- [ ] TTG-002 Verify Marketplace catalog membership does not create a live TG tool or credential.
- [ ] MVP-001 Run private enterprise MVP: register OCI artifact → collect evidence → promote → resolve → AG consume → revoke.
- [ ] MVP-002 Publish `docs/mvp-thinkpixel-evidence.md`.
- [ ] MVP-003 Commit Phase 6 as first usable ThinkPixelMP milestone.

---

## Phase 7 — Bundles, federation, and remote services

- [ ] BND-001 Implement Bundle artifact schema validator.
- [ ] BND-002 Support dependencies on agent-runtime, skill, MCP, remote-agent, and nested bundles according to policy.
- [ ] BND-003 Resolve Bundle to exact transitive ArtifactLock.
- [ ] BND-004 Detect nested Bundle cycles.
- [ ] BND-005 Aggregate declared requirements for display/policy while preserving per-component provenance.
- [ ] BND-006 Verify Bundle approval does not grant dependency capabilities.
- [ ] BND-007 Verify Bundle cannot hide a revoked/ineligible transitive dependency.
- [ ] FED-001 Add ImportSource schema/domain/repository.
- [ ] FED-002 Add ImportRecord/job schema and replay-safe worker.
- [ ] FED-003 Implement central hardened remote fetcher.
- [ ] FED-004 Deny loopback/link-local/private/service/metadata IP ranges according to configured federation policy.
- [ ] FED-005 Revalidate every redirect target.
- [ ] FED-006 Add DNS resolution and address-change protections appropriate to implementation.
- [ ] FED-007 Add strict connect/read/response size/time limits.
- [ ] FED-008 Ensure credentials are not forwarded across unexpected origins.
- [ ] MCP-001 Implement MCP Registry importer.
- [ ] MCP-002 Preserve upstream registry identity/version/source metadata.
- [ ] MCP-003 Import entries as candidate artifacts only.
- [ ] MCP-004 Do not auto-promote imported MCP entries.
- [ ] MCP-005 Handle upstream removal/update without mutating historical local ArtifactVersions.
- [ ] FED-009 Implement generic OCI repository/catalog importer.
- [ ] A2A-001 Implement A2A Agent Card parser/validator.
- [ ] A2A-002 Add `remote-agent` ArtifactVersion support.
- [ ] A2A-003 Snapshot Agent Card immutably and compute descriptor digest.
- [ ] A2A-004 Verify Agent Card signature where available.
- [ ] A2A-005 Add RemoteEndpoint schema/state.
- [ ] A2A-006 Implement TLS/endpoint ownership/health verification evidence.
- [ ] A2A-007 Keep endpoint health state separate from immutable Agent Card identity.
- [ ] A2A-008 Clearly expose remote-service assurance limitations through API.
- [ ] FED-010 Add malicious upstream metadata tests.
- [ ] FED-011 Add SSRF tests for loopback, metadata endpoint, redirect, private IP, oversized responses.
- [ ] FED-012 Add federation outage/retry/backoff tests.
- [ ] FED-013 Add duplicate import/idempotency tests.
- [ ] FED-014 Add source-attribution tests proving imported/repackaged identity is not falsely assigned to upstream publisher.
- [ ] FED-015 Commit Phase 7 with federation/remote-service evidence.

---

## Phase 8 — Portability, attestations, mirroring, and hardening

- [ ] ATT-001 Implement ThinkPixelMP promotion-attestation schema bound to exact digest/catalog/policy decision.
- [ ] ATT-002 Add `KeyProvider`/signing port for organization promotion attestations.
- [ ] ATT-003 Implement development signing backend and production KMS/HSM integration seam.
- [ ] ATT-004 Ensure MP approval signature identity is distinguishable from publisher signature identity.
- [ ] ATT-005 Publish promotion attestation as OCI referrer where registry supports it.
- [ ] CATX-001 Define canonical CatalogSnapshot serialization.
- [ ] CATX-002 Include exact CatalogEntries, resolution/lock references, revocations, and evidence digests.
- [ ] CATX-003 Package CatalogSnapshot as OCI artifact.
- [ ] CATX-004 Sign CatalogSnapshot.
- [ ] MIR-001 Implement controlled RegistryProvider copy operation if Phase 0/registry support proves safe.
- [ ] MIR-002 Preserve exact content digest during registry-to-registry copy where OCI semantics allow.
- [ ] MIR-003 Copy required referrers/signatures/SBOM/provenance according to configured promotion profile.
- [ ] MIR-004 Record copy provenance and destination.
- [ ] AIR-001 Implement catalog snapshot export manifest for air-gap transfer.
- [ ] AIR-002 Implement catalog snapshot import validation.
- [ ] AIR-003 Reject imported snapshot when signature/digest/policy trust requirements fail.
- [ ] AIR-004 Verify imported catalog refers only to immutable artifacts.
- [ ] SEC-007 Add hostile OCI registry fixture returning inconsistent manifests/digests.
- [ ] SEC-008 Add media-type confusion tests.
- [ ] SEC-009 Add malicious referrer graph tests.
- [ ] SEC-010 Add registry redirect credential exfiltration tests.
- [ ] SEC-011 Add dependency namespace/squatting/confusion tests.
- [ ] SEC-012 Add attempted metadata-to-runtime-privilege fields and verify rejection/neutralization.
- [ ] SEC-013 Add attempted evidence forgery tests.
- [ ] SEC-014 Add attempted publisher/namespace takeover tests.
- [ ] RESL-001 Add PostgreSQL outage/recovery tests.
- [ ] RESL-002 Add registry outage/recovery tests.
- [ ] RESL-003 Add OPA outage/recovery tests.
- [ ] RESL-004 Add evidence-provider outage/staleness scenarios.
- [ ] RESL-005 Add outbox worker crash/replay tests.
- [ ] CAP-001 Add configurable per-tenant publication/import/promotion/resolution limits.
- [ ] CAP-002 Add bounded worker queues/backpressure.
- [ ] CAP-003 Load test artifact discovery/search.
- [ ] CAP-004 Load test concurrent OCI registrations.
- [ ] CAP-005 Load test promotion/evidence ingestion.
- [ ] CAP-006 Load test dependency resolution.
- [ ] CAP-007 Load test federation imports.
- [ ] CAP-008 Document tested capacity envelope and bottlenecks.
- [ ] HRD-001 Run full hostile-artifact/security suite repeatedly.
- [ ] HRD-002 Commit Phase 8 with portability/security/performance evidence.

---

## Phase 9 — Production packaging and operations

- [ ] OPS-001 Finalize reproducible non-root ThinkPixelMP OCI image with immutable build metadata.
- [ ] OPS-002 Finalize `thinkpixelmpctl` release binary.
- [ ] OPS-003 Create Helm chart for API/workers, migration Job, ServiceAccount, configuration, secret references, Service, and optional ingress.
- [ ] OPS-004 Add least-privilege Kubernetes RBAC.
- [ ] OPS-005 Add NetworkPolicies restricting MP egress to configured PostgreSQL, OPA, registries, federation sources, identity infrastructure, and evidence sinks as required.
- [ ] OPS-006 Add hardened pod security context, seccomp, dropped capabilities, read-only root filesystem, and bounded temporary storage.
- [ ] OPS-007 Add startup/readiness/liveness probes.
- [ ] OPS-008 Add PodDisruptionBudget/topology guidance.
- [ ] OPS-009 Add optional HPA based on appropriate API/worker signals.
- [ ] OPS-010 Add ServiceMonitor/PodMonitor resources where applicable.
- [ ] OPS-011 Create dashboards for publication, OCI verification, evidence freshness, catalogs, promotion, resolution, revocation, federation, outbox, PostgreSQL, and Go runtime.
- [ ] OPS-012 Define alerts tied to SLOs and runbooks.
- [ ] OPS-013 Write private-enterprise installation/configuration runbook.
- [ ] OPS-014 Write trusted-registry and registry-credential configuration runbook.
- [ ] OPS-015 Write publisher/namespace administration runbook.
- [ ] OPS-016 Write production-catalog policy/promotion runbook.
- [ ] OPS-017 Write artifact compromise/quarantine/revocation incident runbook.
- [ ] OPS-018 Write signature/trust-root rotation runbook.
- [ ] OPS-019 Write federation-source incident/disable runbook.
- [ ] OPS-020 Write PostgreSQL migration/backup/restore runbook.
- [ ] OPS-021 Write OCI mirror/air-gap workflow runbook where implemented.
- [ ] OPS-022 Test PostgreSQL backup/restore preserving immutable versions, evidence, promotions, locks, resolutions, revocations, audit, outbox, and idempotency.
- [ ] OPS-023 Test fresh install in disposable cluster.
- [ ] OPS-024 Test schema/chart upgrade.
- [ ] OPS-025 Test failed upgrade and documented rollback/roll-forward strategy.
- [ ] OPS-026 Test rolling restart while publication/import/promotion work is active.
- [ ] OPS-027 Test registry/signature/policy dependency degradation.
- [ ] OPS-028 Run production-like load test.
- [ ] OPS-029 Generate SBOM and vulnerability reports for released MP image.
- [ ] OPS-030 Add build provenance/signature hooks and release checksums.
- [ ] OPS-031 Add release automation producing images, chart, CLI, OpenAPI, checksums, SBOM/provenance, and release notes.
- [ ] OPS-032 Commit Phase 9 with operations evidence.

---

## Phase 10 — Release-candidate closure

- [ ] RC-001 Freeze OpenAPI and public error contracts for RC.
- [ ] RC-002 Freeze ThinkPixel `agent-runtime`, Bundle, ArtifactLock, ArtifactResolution, EvidenceRecord, CatalogSnapshot, and promotion-attestation schemas for RC.
- [ ] RC-003 Freeze supported artifact-kind/delivery-mode vocabulary for RC.
- [ ] RC-004 Run generated-artifact/backward-compatibility checks.
- [ ] RC-005 Run `make verify` from a clean checkout.
- [ ] RC-006 Archive unit/race/fuzz/property/PostgreSQL/OCI/policy/security/federation/e2e evidence.
- [ ] RC-007 Confirm immutable digest identity across tag mutation and repeated publication.
- [ ] RC-008 Confirm publisher metadata cannot grant runtime capability or privileged infrastructure configuration.
- [ ] RC-009 Confirm evidence for one digest cannot qualify another digest.
- [ ] RC-010 Confirm publisher self-claims cannot impersonate trusted evidence producers.
- [ ] RC-011 Confirm production catalog fails closed on missing/stale/untrusted required evidence.
- [ ] RC-012 Confirm four-eyes policy under concurrent/replayed review attempts.
- [ ] RC-013 Confirm dependency lock is deterministic and contains no unresolved mutable references.
- [ ] RC-014 Confirm revoked transitive dependency prevents protected resolution.
- [ ] RC-015 Confirm revoked artifact history remains available and auditable.
- [ ] RC-016 Confirm MP revocation propagates to AG integration without MP directly mutating Run state.
- [ ] RC-017 Confirm previously durably authorized executions follow AG's documented behavior during temporary MP outage.
- [ ] RC-018 Confirm remote/federation fetchers resist SSRF and redirect credential leakage.
- [ ] RC-019 Confirm hostile archives/OCI layers cannot escape bounded inspection or execute code.
- [ ] RC-020 Confirm registry compromise simulations do not bypass digest/signature/promotion policy.
- [ ] RC-021 Confirm catalog snapshot/air-gap workflow preserves exact identities/evidence where RC scope includes it.
- [ ] RC-022 Confirm no unresolved critical/high security finding.
- [ ] RC-023 Confirm no required test is flaky/skipped without explicit disposition.
- [ ] RC-024 Confirm SLO/capacity envelope is documented and measured.
- [ ] RC-025 Exercise install, upgrade, rollback/forward, backup/restore, rolling restart, registry outage, OPA outage, and federation outage game days.
- [ ] RC-026 Reconcile every TODO against implementation/tests/docs/commits.
- [ ] RC-027 Update README with architecture, artifact kinds, OCI model, supply-chain model, catalogs, promotion, resolution, ThinkPixel integration, deployment, security, and known limitations.
- [ ] RC-028 Create numbered ADRs for all durable decisions in `PLAN.md`.
- [ ] RC-029 Ensure ADRs preserve meaningful rejected alternatives and implementation lessons.
- [ ] RC-030 Prepare RC release notes, compatibility matrix, operator checklist, and artifact inventory.
- [ ] RC-031 Explicitly document post-RC scope: public marketplace, payments, UI, Git/plugin import, richer federation, recommendation/search, evaluator integration.
- [ ] RC-032 Remove `PLAN.md` and `TODO.md` only after durable rationale is transferred to permanent docs/ADRs.
- [ ] RC-033 Run documentation/link validation and `make verify` against resulting tree.
- [ ] RC-034 Commit final documentation transition to `main`.
- [ ] RC-035 Build release artifacts from that exact commit and verify digest/checksum/SBOM/provenance consistency.
- [ ] RC-036 Create/tag RC only after all previous gates pass.

---

## Deferred / post-RC backlog

- [ ] FUT-001 Add public multi-organization marketplace operating mode.
- [ ] FUT-002 Add billing, licensing entitlement, payments, revenue sharing, and commercial vendor contracts only if product direction requires them.
- [ ] FUT-003 Add user ratings/reviews and abuse moderation.
- [ ] FUT-004 Add full web marketplace UI.
- [ ] FUT-005 Add recommendation/personalization engine.
- [ ] FUT-006 Integrate hybrid lexical/semantic marketplace search while keeping authoritative resolution digest-based.
- [ ] FUT-007 Add Claude/OpenAI plugin importer where stable export formats make this reliable.
- [ ] FUT-008 Add generalized Git repository importer/repackager.
- [ ] FUT-009 Add npm/PyPI import profiles where useful without weakening OCI-native runtime distribution.
- [ ] FUT-010 Add additional software-supply-chain standards/evidence types as ecosystem converges.
- [ ] FUT-011 Add cross-enterprise publisher trust federation.
- [ ] FUT-012 Add transparency-log publication for marketplace promotion/revocation decisions if useful.
- [ ] FUT-013 Add richer artifact compatibility analysis against AR Runtime Profiles.
- [ ] FUT-014 Add automated evaluation-orchestration integration while keeping evaluator execution external to MP.
- [ ] FUT-015 Add notification/subscription workflows for artifact updates, vulnerabilities, deprecation, and revocation.
- [ ] FUT-016 Add SBOM dependency impact queries across catalogs.
- [ ] FUT-017 Add automated remediation suggestions/replacement graph for revoked/deprecated artifacts.
- [ ] FUT-018 Add public upstream ThinkPixel catalog service if ecosystem demand justifies it.

---

## Progress log

Append one row per completed atomic item or tightly coupled group.

Do not delete historical entries.

Supersede obsolete assumptions with a later entry.

Date | TODO IDs | Commit | Verification evidence | Notes/deviations
--- | --- | --- | --- | ---
YYYY-MM-DD | `ARC-...` | `<sha>` | `<commands/artifacts>` | `<notes>`
