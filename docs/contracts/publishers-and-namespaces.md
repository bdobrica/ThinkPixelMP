# Publishers and namespaces

## Publisher

A Publisher is a tenant-local enterprise identity permitted to own or receive delegation for namespaces. Release-candidate verification is an authenticated administrative marketplace action.

Required state is `claimed`, `verified`, `suspended`, or `revoked`. Suspension prevents new publication and promotion without erasing history. Revocation permanently records loss of trust and does not revoke every artifact automatically; artifact digest revocation remains explicit.

Both `suspended` and `revoked` block new publication, namespace mutation, and promotion by that Publisher. Existing catalog entries and immutable resolutions remain available unless catalog policy, quarantine, or a separate digest revocation says otherwise. Publisher state remains visible to future policy evaluation and historical records.

V1 publisher transitions are `claimed → verified | suspended | revoked`, `verified → suspended | revoked`, and `suspended → verified | revoked`. Revoked is terminal. Transitions require publisher administration, a strong ETag, and reason metadata.

## Namespace

A namespace is a tenant-scoped hierarchical path of lowercase DNS-style segments. Each segment matches:

```text
[a-z0-9](?:[a-z0-9-]*[a-z0-9])?
```

An artifact name follows the same single-segment rule. The canonical logical identity is `{namespace}/{name}`.

Within one tenant:

- canonical namespace paths are unique;
- one verified Publisher owns a namespace;
- parent ownership does not implicitly transfer to another Publisher;
- an owner may explicitly delegate a child prefix;
- delegations may overlap only through strict ancestor/descendant nesting;
- the valid longest matching prefix controls publication at a path;
- delegation cannot cross tenants;
- ownership and delegation changes are audited and do not rewrite historical publisher attribution;
- no ownership or delegation grants runtime capability or infrastructure access.

Sibling or otherwise ambiguous overlapping delegations are invalid. Collision and ambiguity attempts fail closed.

Only verified Publishers may own newly created namespaces or receive new delegations. A delegation must name a strict descendant prefix and cannot delegate the owned root itself. Delegations are append-only `active` or `revoked`; revocation is ETag-protected and reassignment creates a new record without changing historical attribution.
