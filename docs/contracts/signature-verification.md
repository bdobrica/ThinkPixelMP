# Signature verification contract

Signature verification and signer-policy trust are separate assertions.

## Supported modes

- Sigstore keyless verification using certificate identity, OIDC issuer, transparency material, trusted root material, and bundle/timestamp evidence as applicable.
- Configured public-key verification using operator-managed trust references. Private signing material is never stored by MP.

Trust rules are tenant-scoped and may be narrowed by namespace, issuer, signer identity, key identity, and signature/attestation profile. Publisher metadata cannot select a trust root or weaken a rule.

## Result

The normalized result records:

- exact subject digest;
- signature and bundle digest/reference;
- verification profile and implementation version;
- cryptographic validity;
- signer certificate/key identity;
- issuer and subject/SAN identity where applicable;
- transparency-log and timestamp evidence where available;
- matched trust rule and policy-trust conclusion;
- bounded stable reason codes.

A signature can be cryptographically valid and policy-untrusted. That result is retained as evidence but cannot satisfy a trusted-signature requirement.

A valid signature does not imply secure behavior, acceptable provenance, an SBOM, successful evaluation, publisher verification, catalog approval, or runtime authority.
