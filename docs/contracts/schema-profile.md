# JSON schema and canonicalization profile

## Rules

- Dialect: JSON Schema Draft 2020-12.
- Canonical schema origin: `https://schemas.thinkpixel.io`.
- MP schema path prefix: `/thinkpixelmp/`.
- Object schemas use `additionalProperties: false` unless a field is explicitly defined as an opaque extension map.
- JSON parsing rejects duplicate keys, invalid Unicode, non-finite numbers, and trailing content.
- Schema defaults document behavior but do not mutate input. Trusted code creates the normalized representation.
- Set-like arrays are sorted and de-duplicated according to their contract before hashing.
- Canonical bytes use RFC 8785 JCS.
- Canonical digests use lowercase SHA-256.
- Unknown schema versions fail closed.

Original submitted bytes may be retained as bounded evidence or by immutable external reference. Consumers use the validated normalized representation and its canonical digest.

## Cross-repository AgentRuntimeSpec

ThinkPixelMP `agent-runtime` descriptors embed or integrity-bind a ThinkPixelAR AgentRuntimeSpec v1 structurally compatible with the AR Phase 0 contract. Its canonical target identifier is:

```text
https://schemas.thinkpixel.io/thinkpixelar/agent-runtime-spec/v1
```

Until ThinkPixelAR publishes the corrected identifier, MP validation pins the reviewed schema content/version rather than following an unversioned network URL. Runtime compatibility remains an AR/operator decision; MP validation does not grant a runtime profile.
