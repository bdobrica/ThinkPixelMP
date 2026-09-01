# Phase 1 engineering-foundation evidence

- Date: 2026-09-01
- Phase baseline commit: `0687d1f`
- Exit gate: a clean checkout passes the baseline verification gate

## Outcome

Phase 1 established the repository's engineering foundation without moving
Phase 2 persistence, identity, or publication responsibilities into the
foundation. The repository now has a pinned Go module and layered package
structure, secure typed configuration and telemetry, shared domain primitives,
a bounded HTTP baseline, a PostgreSQL development service and explicit
migration command, stable Makefile and CI interfaces, a generated and validated
OpenAPI contract, an API-only CLI skeleton, and a hardened non-root service
image.

The clean-checkout acceptance run is recorded in
[ENG-018](evidence/eng-018-clean-checkout-baseline.md). It passed generation and
formatting with no tracked drift, the complete `make verify` gate, and the
hardened image smoke test. This satisfies the Phase 1 exit condition in
`PLAN.md`.

## Delivered foundation

| Area | Items | Evidence |
| --- | --- | --- |
| Module and architecture | ENG-001–ENG-003 | [Go module](evidence/eng-001-go-module.md), [repository structure](evidence/eng-002-repository-structure.md), and [dependency policy](evidence/eng-003-dependency-policy.md) |
| Runtime safety and observability | ENG-004–ENG-008 | [configuration](evidence/eng-004-typed-configuration.md), [logging](evidence/eng-005-structured-logging.md), [metrics and tracing](evidence/eng-006-metrics-and-tracing.md), [shared primitives](evidence/eng-007-shared-primitives.md), and [HTTP server](evidence/eng-008-http-server.md) |
| Developer and contract gates | ENG-009–ENG-011 | [OpenAPI generation](evidence/eng-009-openapi-generation.md), [Makefile interface](evidence/eng-010-makefile-interface.md), and [verification gate](evidence/eng-011-verification-gate.md) |
| Development and delivery surfaces | ENG-012–ENG-015 | [PostgreSQL development](evidence/eng-012-postgresql-development.md), [API CLI](evidence/eng-013-api-cli.md), [container image](evidence/eng-014-container-image.md), and [continuous integration](evidence/eng-015-continuous-integration.md) |
| Repository and compatibility controls | ENG-016–ENG-017 | [repository hygiene](evidence/eng-016-repository-hygiene.md) and [supported versions](evidence/eng-017-supported-versions.md) |
| Exit verification | ENG-018 | [Clean-checkout baseline](evidence/eng-018-clean-checkout-baseline.md) |

## Exit verification

The ENG-018 run started from a clean tracked checkout at commit `39b164a` and
verified:

- generated OpenAPI and Go formatting produced no tracked changes;
- vet, Staticcheck, unit tests, and race tests passed;
- repository hygiene, dependency-source, vulnerability, and license policies
  passed;
- OpenAPI drift, schema validation, and Phase 0 contract validation passed;
- both Go binaries built; and
- the pinned container image served `/livez` as `65532:65532` with a read-only
  root filesystem, all capabilities dropped, and `no-new-privileges`.

The exact commands, environment notes, and non-failing tool warnings are
preserved in the [clean-checkout evidence](evidence/eng-018-clean-checkout-baseline.md).

The phase-close documentation change was also checked with:

```text
GOCACHE="$PWD/.tmp-eng019/go-cache" \
GOTMPDIR="$PWD/.tmp-eng019/go-tmp" \
BUILD_DIR="$PWD/.tmp-eng019/build" \
GOTOOLCHAIN=go1.26.7 \
make verify

git diff --check
```

The aggregate gate reported no vulnerabilities. Its existing non-failing
Redocly duplicate-schema-name and `go-licenses` assembly-inspection warnings
were unchanged.

## Scope boundary

This phase proves the engineering and delivery baseline only. It does not claim
that the Phase 2 authoritative PostgreSQL model, tenant identity, publisher and
namespace state, artifact publication, transactional audit/outbox behavior, or
OIDC authorization are implemented. The migration command intentionally has no
released migrations until Phase 2 introduces the initial tenant schema.

## Exit assessment

ENG-001 through ENG-018 are implemented and have item-level evidence. The
clean-checkout baseline passed the aggregate repository gate and hardened image
smoke test. Phase 1 therefore meets its documented exit condition, and Phase 2
may begin at DB-001 without changing the component or authority boundaries
established by the accepted ADRs and versioned contracts.
