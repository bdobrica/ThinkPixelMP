# System context

ThinkPixelMP is the metadata, trust, evidence, curation, and immutable-resolution control plane around external artifact stores and evidence producers.

```mermaid
flowchart TB
    PUB[Publishers] -->|publish descriptors and references| MP[ThinkPixelMP]
    CLIENT[Marketplace clients and administrators] -->|REST / JSON over /v1| MP
    FED[OCI, MCP Registry, A2A, and other federation sources] -->|untrusted imported metadata| FETCH[Hardened import and fetch boundary]
    FETCH --> MP

    MP --> PG[(PostgreSQL authoritative metadata)]
    MP -->|resolve manifests, referrers, and digests| OCI[External OCI 1.1 registries]
    MP -->|verify signatures and attestations| SIG[Signature and transparency infrastructure]
    SCAN[Trusted scanners and evaluators] -->|digest-bound evidence| MP
    MP -->|typed policy input| OPA[OPA / Rego catalog policy]
    OPA -->|typed eligibility decision| MP
    MP -->|events and evidence export| SINK[Independent evidence sinks]

    AG[ThinkPixelAG] -->|request approved immutable resolution| MP
    MP -->|resolution and revocation events| AG
    AG -->|exact admitted digests| AR[ThinkPixelAR]
    AR -->|pull exact digest| OCI
    MP -->|discovery and onboarding metadata only| TG[ThinkPixelTG]
    TG -->|live tool capabilities remain TG/AG authority| AG
```

## Ownership summary

| Component | Authoritative for | Not authoritative for |
| --- | --- | --- |
| ThinkPixelMP | Artifact identity, normalized evidence, catalog eligibility, promotion, immutable resolution, digest revocation | Run authorization, runtime privilege, live tool credentials, artifact bytes |
| PostgreSQL | Durable MP control-plane state | OCI payload storage |
| OCI registry | Addressed artifact bytes and registry manifests | MP approval or Run authority |
| Evidence producer | Its authenticated, digest-bound assertion | Publisher identity or catalog approval unless separately authorized |
| OPA adapter | Evaluation of active catalog policy | Administrative authentication or runtime authorization |
| ThinkPixelAG | Run admission, capability grants, resource authority, runtime revocation policy | Marketplace artifact qualification |
| ThinkPixelAR | Materialization and isolated execution of exact admitted digests | Marketplace search or authorization |
| ThinkPixelTG | Live tool configuration, credentials, and tool execution | Marketplace qualification or Run admission |

Cross-boundary inputs are untrusted unless a contract explicitly names an authenticated trusted producer and validates its scope.
