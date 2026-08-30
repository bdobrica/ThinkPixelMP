# External adapter baseline

## Event sink

The first sink sends CloudEvents 1.0.2 over authenticated HTTPS. Delivery is at least once with stable event IDs, bounded exponential backoff with jitter, retry classification, and dead-letter state. Cross-origin redirects and credential forwarding are prohibited. Receiver success is acknowledged only by configured bounded 2xx behavior.

## Report store

The neutral report-store port supports immutable put/reference, digest-verified read, metadata/head, and retention/deletion according to policy. The first object-store adapter is S3-compatible. OCI digest references are handled through RegistryProvider. Bucket, endpoint, credentials, encryption, and retention are operator configuration.

## Signer

The signer port accepts a domain-separated statement digest/profile and returns signer identity plus verifiable signature/bundle metadata. Production configuration requires a KMS/HSM-compatible provider. MP never receives exportable private key material when the provider can sign remotely.

An explicit development profile may create ephemeral local keys. Development signatures are visibly identified and cannot satisfy production trust configuration.

## Search

PostgreSQL full-text search and trigram similarity implement RC browsing. Indexes are tenant scoped and derived from authoritative records. Ranking cannot choose a resolution or bypass catalog/lifecycle policy.

## Identity mapping

Each configured OIDC issuer defines exact audience, algorithms, required claims, and mapping rules from verified claims to tenant ID, principal ID, and explicit role grants. No caller field or unverified claim chooses mapping configuration. Mapping ambiguity fails closed.
