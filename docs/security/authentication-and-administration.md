# Authentication and administrative authorization

## Authentication

Protected APIs require verified OIDC JWT or workload identity according to operator configuration. Validation includes exact issuer, audience, allowed algorithm, signature, expiry/not-before, bounded clock skew, and required subject/client claims.

Tenant and principal are derived exclusively through configured verified-claim mappings. Request bodies, query parameters, arbitrary forwarded headers, source IP, registry metadata, artifact descriptors, and policy output cannot establish or change tenant/principal identity.

## Roles

Initial tenant-scoped roles are:

- `publisher-admin`;
- `namespace-admin`;
- `publication-admin`;
- `evidence-producer-admin`;
- `reviewer`;
- `catalog-admin`;
- `policy-admin`;
- `revocation-admin`;
- `federation-admin`.

Roles have no implied inheritance. A principal receives only explicitly granted actions; multiple grants compose permissions. Domain checks such as namespace ownership, trusted producer scope, reviewer separation, and live lifecycle checks still apply after role authorization.

The initial role-to-action mapping is exact:

| Role | Administrative action |
| --- | --- |
| `publisher-admin` | `publisher.manage` |
| `namespace-admin` | `namespace.manage` |
| `publication-admin` | `publication.publish` |
| `evidence-producer-admin` | `evidence_producer.manage` |
| `reviewer` | `promotion.review` |
| `catalog-admin` | `catalog.manage` |
| `policy-admin` | `policy.activate` |
| `revocation-admin` | `revocation.manage` |
| `federation-admin` | `federation.manage` |

Grant lookup uses the exact mapped tenant and opaque principal. Missing identity,
unknown roles/actions, absent grants, and grants for another tenant or principal
fail closed. Authorization answers only whether the principal holds the action;
it cannot satisfy resource ownership, state-machine, separation-of-duty,
evidence-scope, lifecycle, or other domain requirements.

## Boundary rules

- Publisher verification cannot be self-issued through publication authority.
- Evidence-producer administration is separate from evidence ingestion identity.
- Catalog administration cannot activate policy without `policy-admin` authority.
- Reviewer authority does not bypass required distinct-principal or requester-exclusion rules.
- Revocation administration cannot delete or reverse revocation history.
- Federation administration cannot promote imported content.
- Policy output cannot grant administrative actions.

All administrative mutations produce tenant/principal-bound audit and transactional outbox records with redacted request metadata.

## Local development

Local development authentication uses one fixed, operator-configured tenant and
principal. It activates only when both process mode is `development` and
authentication mode is explicitly `local-development`. Startup rejects this
mode in `test` and `production`, rejects incomplete local identity
configuration, and rejects simultaneous OIDC configuration. The adapter repeats
these mode checks when it is constructed.

The resulting principal is always prefixed `local-development:`. The adapter
accepts no bearer credential and never derives tenant or principal from a
request header, body, query, source address, or other caller-controlled value.
Supplying bearer material in this mode fails closed. Local identities remain
subject to separately configured role grants and all domain authorization
checks; the mode grants no administrative action by itself.
