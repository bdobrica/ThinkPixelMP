# Dependency, source, and license policy

This policy applies to runtime, test, generator, scanner, and build dependencies. `dependency-policy.json` is the machine-readable policy; this document defines its security meaning. An allowed source or license is necessary but never sufficient approval for a dependency.

## Selection and ownership

- Prefer the Go standard library and small technology-neutral ports.
- Every new direct dependency requires a named owner and documented purpose/boundary in the change that introduces it.
- Review upstream ownership, maintenance and incident response, release provenance, transitive footprint, license, known vulnerabilities, privileged behavior, data/network access, and replacement/rollback cost.
- Infrastructure libraries stay behind ports/adapters. `internal/domain` cannot acquire them for convenience.
- Add one logical dependency group at a time and review `go mod graph`, `go mod why`, release notes, checksums, and the complete diff.

## Module source and version rules

The initial allowed module-path prefixes are exactly:

- `github.com/`;
- `go.opentelemetry.io/`;
- `go.yaml.in/`;
- `golang.org/`;
- `google.golang.org/`;
- `oras.land/`.

Any other prefix fails until a reviewed policy change adds it. An allowed prefix does not approve every owner or module below it.

Selected versions must be exact Go module versions. Branch queries, unversioned revisions, unresolved module errors, retracted versions, vendored edits, `replace`, and `exclude` directives fail closed. A pseudo-version is permitted only by an active exception for that exact module and version.

Public modules use ordinary Go checksum verification. `GOSUMDB=off`, checksum failures, and missing `go.sum` for a non-empty third-party graph fail. Private modules and source bypasses are not permitted by the initial policy. Corporate proxies may cache public modules only when module paths and public checksum verification remain intact; proxy configuration does not expand the source allowlist.

## License rules

The initial allowlist is:

- Apache-2.0;
- BSD-2-Clause;
- BSD-3-Clause;
- ISC;
- MIT.

Unknown, unclassified, copyleft, source-available, commercially restricted, or otherwise unlisted licenses fail closed. Dual licensing must select an allowed expression and preserve its notices. A license exception requires legal and security approval plus exact module/version scope and required notice/source-offer handling.

Tool and test dependencies are included because they execute in trusted developer or CI environments. Generated license reports are release evidence, not a substitute for source review.

## Exceptions

An exception is narrow, exact, and temporary. It records:

- kind: `source`, `pseudo-version`, `license`, or `vulnerability`;
- exact module and version, plus the exact license or finding when applicable;
- owner, justification, approval reference, creation and expiry dates;
- compensating controls and removal plan.

The initial duration cannot exceed 90 days. Expired, future-created, malformed, wildcard, incomplete, or version-mismatched exceptions fail. Renewal is a new review, not an automatic extension. The initial exception list is empty.

## Enforcement split

ENG-003 provides the standard-library-only policy checker. It validates policy integrity, the selected module graph, source prefixes, exact/pseudo versions, forbidden directives, vendoring, checksum posture, graph/retraction errors, and exception shape/expiry.

The aggregate verification gate pins go-licenses for dependency license classification and the official govulncheck scanner for call-graph-aware vulnerability analysis. Scanner success complements rather than replaces the required source, maintenance, provenance, and advisory review for a dependency change. Network or vulnerability-database failure fails the gate; it is not treated as a clean result.

Run the current gate from the repository root:

```bash
GOCACHE=/tmp/thinkpixelmp-dependency-go-cache GOTOOLCHAIN=go1.26.7 go run ./scripts/dependencycheck
GOCACHE=/tmp/thinkpixelmp-dependency-go-cache GOTOOLCHAIN=go1.26.7 go test ./scripts/dependencycheck
GOCACHE=/tmp/thinkpixelmp-dependency-go-cache GOTOOLCHAIN=go1.26.7 make vulnerability-check license-check
```

Network, checksum-database, module metadata, or graph-resolution failures are failures, not clean results.
