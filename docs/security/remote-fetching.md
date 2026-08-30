# Remote fetching and SSRF controls

All OCI, MCP Registry, A2A, and future Git/plugin retrieval passes through one hardened fetch abstraction. Adapters cannot instantiate bypass clients.

## Production policy

- HTTPS only; TLS 1.2 or newer, hostname verification, and operator-managed trust roots.
- Deny loopback, RFC1918/private, link-local, carrier-grade NAT, multicast, unspecified, documentation/reserved, and cloud metadata ranges by default for IPv4 and IPv6.
- Allow internal targets only through operator-configured per-source hostname and CIDR rules.
- Resolve all addresses before connection, validate every candidate address, and bind the validated result to the connection to resist DNS rebinding.
- Permit at most three redirects. Re-parse, re-resolve, and re-authorize each hop. Reject HTTPS downgrade.
- Never forward authorization headers, cookies, bearer tokens, client certificates, registry credentials, or source-specific secrets across origins.
- Enforce methods, content types, byte limits, streaming deadlines, connection/read timeouts, and bounded error bodies.
- Artifact/publisher metadata cannot select proxy configuration, trust roots, credentials, client certificates, DNS resolver, or allowlist exceptions.

## Development profile

An explicit local-development profile may allow HTTP, loopback, and private targets for disposable local services and tests. It must be visibly identified in health/configuration output and structurally rejected when production mode is enabled. Development credentials and trust state are not reusable as production configuration.

## Initial defaults

- remote metadata response: 16 MiB;
- overall fetch deadline: 30 seconds;
- redirects: 3.

Instance configuration may lower or raise these defaults up to compiled ceilings. Artifact metadata cannot alter them. The compiled remote-response ceiling is 64 MiB and the fetch-deadline ceiling is 120 seconds.
