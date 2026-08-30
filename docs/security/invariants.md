# Normative security invariants

These invariants apply to every API, adapter, import path, policy, and persistence implementation.

1. `MarketplaceEligibility != RunAuthorization`.
2. `DeclaredRequirement != GrantedAuthority`.
3. Installing, importing, selecting, or promoting an artifact MUST NOT expand capability authority.
4. Marketplace metadata MUST NOT directly select privileged infrastructure, service accounts, secrets, host access, or credentials.
5. Production artifact identity is immutable and content-addressed.
6. Evidence, approval, lock, resolution, and revocation subjects bind to exact digests.
7. Evidence for digest A MUST NOT qualify digest B, regardless of matching names, versions, tags, publishers, or source locations.
8. Mutable tags and version ranges MUST resolve to exact digests before protected promotion or consumption.
9. Import creates candidate state only and MUST NOT imply local approval.
10. Publisher or namespace verification proves publishing authority only; it does not prove artifact safety.
11. A valid signature proves only what the configured signer and verification policy establish; it is not a generic security approval.
12. Deprecation, quarantine, and revocation remain distinct. Revocation is append-only and MUST NOT delete history.
13. MP unavailability MUST NOT silently replace or re-resolve an already authorized immutable graph.
14. Protected promotion fails closed when required policy or evidence is missing, stale, malformed, unavailable, or unverifiable.
15. Publisher-controlled descriptors, OCI content, remote endpoints, federation metadata, and raw evidence are hostile input.
