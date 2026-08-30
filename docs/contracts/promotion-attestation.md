# ThinkPixelMP promotion attestation

MP may publish a signed OCI referrer attesting that one exact artifact digest received one exact marketplace promotion decision.

The statement binds:

- predicate/profile version;
- exact artifact subject digest;
- tenant and catalog stable identifiers;
- policy bundle digest and policy-input/decision digest;
- evidence snapshot digest;
- dependency-lock digest;
- PromotionRequest and PromotionDecision IDs;
- decision outcome and time;
- MP deployment/service identity and signing identity;
- optional superseding lifecycle references known at issuance.

The attestation identifies ThinkPixelMP as promoter/attester. It never claims to be a publisher signature, source provenance, SBOM, vulnerability scan, evaluation, or runtime authorization.

Later deprecation, quarantine, or revocation does not rewrite the attestation. Consumers reconcile current lifecycle state separately.

The canonical statement records the expected MP signing identity but does not embed a signature or signature reference. Cosign signs the statement externally as an OCI referrer. Consumers verify the referrer, signature bundle, subject binding, and configured signer policy independently.
