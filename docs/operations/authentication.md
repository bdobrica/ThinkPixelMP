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
    "discovery_timeout": "5s"
  }
}
```

The equivalent environment variables are `TPMP_OIDC_ISSUER`,
`TPMP_OIDC_AUDIENCE`, `TPMP_OIDC_ALLOWED_ALGORITHMS` (a comma-separated list),
`TPMP_OIDC_CLOCK_SKEW`, and `TPMP_OIDC_DISCOVERY_TIMEOUT`. Command-line flags
use the corresponding lowercase hyphenated names.

Issuer URLs must use HTTPS and cannot contain user information, a query, or a
fragment. Clock skew is bounded to five minutes. Algorithms are selected from
the verifier's asymmetric-signature allowlist; `none` and symmetric MAC
algorithms are structurally rejected. Tokens are bounded to 64 KiB and all
token-validation failures share the stable
`unauthorized:identity.invalid_token` classification.

Verification alone does not establish a ThinkPixel tenant, principal, role, or
administrative action. IAM-002 owns issuer-specific verified-claim mapping and
IAM-003 owns marketplace administrative authorization. Until protected API
handlers are introduced, an absent OIDC issuer leaves the verifier unconfigured
rather than enabling an alternative identity source. Local development
authentication remains separately deferred to IAM-004.
