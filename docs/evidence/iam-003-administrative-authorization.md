# IAM-003 administrative authorization evidence

Date: 2026-09-06

## Implemented boundary

- Added transport- and identity-provider-neutral role, action, grant, and
  authorizer types for all nine initial marketplace administrative roles.
- Kept `policy-admin` separate from `catalog-admin`, as required by the
  authoritative security model, even though the checklist abbreviates the
  catalog administration area.
- Added an immutable authorizer built from explicit tenant/principal role
  grants. Each role grants only its matching action; multiple explicit grants
  compose without inheritance.
- Authorization uses the exact UUIDv7 tenant and opaque principal established
  by verified identity mapping. Missing identity, absent grants, another
  tenant/principal, and unknown actions fail closed with stable typed errors.
- Kept authorization separate from namespace ownership, trusted producer
  scope, review separation, lifecycle checks, state machines, audit, and
  transactional outbox behavior. Application flows must enforce those domain
  requirements in addition to calling the authorizer.

Grant provisioning remains an operator/application bootstrap concern. This
item does not accept role claims from JWTs, add caller-controlled grant APIs,
or add public HTTP/schema surface. Protected handlers introduced by later
feature items can depend on the authorization port without importing a
provider-specific policy type.

## Verification

The following gates passed for this item:

```text
GOTOOLCHAIN=go1.26.7 go test -race ./internal/security ./internal/ports/authorization
GOTOOLCHAIN=go1.26.7 make verify
git diff --check
```

The focused suite covers every role/action pair, absence of implied
inheritance, multi-role composition, exact tenant/principal isolation,
grant-policy validation, missing identity, absent grants, and unknown actions.
