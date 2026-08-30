# Registry credential ownership

Registry credentials are operator-owned configuration selected by normalized registry origin and optional repository prefix. Publisher descriptors cannot select a credential identity or pass credential material.

Pull/inspection credentials are read-only by default. Push/publish credentials are separately configured and require an explicitly authorized workflow. Least-specific wildcard credentials are prohibited when a narrower scope is available.

Credentials are never forwarded across origin changes, untrusted redirects, mirrors not explicitly mapped by operators, or federation sources. They are excluded from PostgreSQL artifact metadata, API responses, logs, traces, policy input, evidence summaries, and audit payloads.

Rotation changes operator configuration without changing artifact identity. Authentication failure is typed separately from absence, authorization denial, registry outage, digest mismatch, and malformed content.
