# PUB-003 Namespace delegation and ownership evidence

- Date: 2026-09-06
- Scope: tenant-scoped delegation lifecycle and publication ownership checks

## Outcome

ThinkPixelMP now models explicit Namespace delegation as an immutable identity
with append-only `active` and `revoked` state history. Creation accepts only a
strict child of the named Namespace and only a currently verified Publisher in
the same tenant. Active prefixes are unique, cannot collide with Namespace
ownership roots, and may be reassigned only by revoking the historical record
and creating a new one. Revocation uses an expected state version and records
bounded reason metadata.

The application service requires the exact `namespace.manage` action for
delegation mutations, binds create and revoke to distinct idempotency ownership
actions, and composes each change transactionally with its audit and bounded
outbox event. The additive v1 event variants contain only tenant-safe resource
IDs, states, and a stable revocation reason code; namespace paths and free-form
explanations are excluded. Its publication check separately requires
`publication.publish`, resolves ownership
inside the authenticated tenant, and compares the asserted Publisher with the
live controlling Publisher. This authority is publication-only and creates no
runtime capability or infrastructure access.

PostgreSQL resolves the longest matching Namespace root or active delegation.
An inactive controlling Publisher fails closed rather than restoring authority
to a shorter prefix; a revoked delegation is excluded so the next valid ancestor
may control. Forced RLS, tenant-consistent foreign keys, collision guards, state
transition guards, and deferred audit constraints enforce the same invariants at
the persistence boundary. The reserved delegation HTTP routes remain unexposed;
their transport contract is not part of PUB-003.

## Verification

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/domain/namespace ./internal/domain/outbox ./internal/app/publication ./internal/adapters/postgres/namespace
GOTOOLCHAIN=go1.26.7 make test-migrations
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

Coverage includes strict-descendant validation, terminal revocation, exact-role
authorization, idempotent mutation replay, longest-prefix selection, inactive
Publisher denial, revocation fallback, prefix/root collision rejection,
append-only reassignment, stale versions, audit coupling, and tenant isolation.
