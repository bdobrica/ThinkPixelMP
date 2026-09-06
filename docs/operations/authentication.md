# OIDC authentication configuration

ThinkPixelMP's OIDC verifier is configured for exactly one operator-trusted
issuer and audience. It performs provider discovery over HTTPS, follows issuer
JWKS rotation, and accepts only an explicit signing-algorithm allowlist. It
validates the signature, literal issuer, audience, expiry, not-before time,
subject, and multi-audience authorized-party claim before exposing verified
claims to the identity-mapping boundary.

The configuration file shape is:

```json
{
  "oidc": {
    "issuer": "https://identity.example.test/tenant",
    "audience": "thinkpixelmp",
    "allowed_algorithms": ["RS256"],
    "clock_skew": "30s",
    "discovery_timeout": "5s",
    "tenant_claim": "groups",
    "principal_claim": "employee_id",
    "tenant_mappings": [
      {
        "claim_value": "thinkpixel-marketplace-acme",
        "tenant_id": "0198fc21-ced5-7000-8000-000000000001"
      }
    ]
  }
}
```

The equivalent environment variables are `TPMP_OIDC_ISSUER`,
`TPMP_OIDC_AUDIENCE`, `TPMP_OIDC_ALLOWED_ALGORITHMS` (a comma-separated list),
`TPMP_OIDC_CLOCK_SKEW`, `TPMP_OIDC_DISCOVERY_TIMEOUT`,
`TPMP_OIDC_TENANT_CLAIM`, `TPMP_OIDC_PRINCIPAL_CLAIM`, and
`TPMP_OIDC_TENANT_MAPPINGS`. The mappings environment value is a JSON array in
the same shape as the configuration file. Command-line flags use the
corresponding lowercase hyphenated names.

Issuer URLs must use HTTPS and cannot contain user information, a query, or a
fragment. Clock skew is bounded to five minutes. Algorithms are selected from
the verifier's asymmetric-signature allowlist; `none` and symmetric MAC
algorithms are structurally rejected. Tokens are bounded to 64 KiB and all
token-validation failures share the stable
`unauthorized:identity.invalid_token` classification.

The mapper reads only the configured top-level claim names from the verified
claim set. A tenant claim may be one string or an array of strings. Exactly one
value must match the operator's explicit claim-value-to-UUIDv7 allowlist; no
match or more than one match returns
`unauthorized:identity.unmapped`. Tenant IDs are never taken directly from a
claim. The principal claim must be one bounded string. MP converts it to a
stable, issuer-scoped opaque identifier before it reaches audit or idempotency
state, avoiding persistence of the raw identity claim. Duplicate JSON claim
names, malformed shapes, wrong issuers, and invalid mapping configuration fail
closed. Ordinary safe configuration rendering reports only the number of
tenant mappings and does not expose their claim values or tenant IDs.

Mapping establishes tenant and principal identity only. IAM-003 owns
marketplace roles and administrative authorization. Until protected API
handlers are introduced, an absent OIDC issuer leaves authentication
unconfigured rather than enabling an alternative identity source. Local
development authentication remains separately deferred to IAM-004.
