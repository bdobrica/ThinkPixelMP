# ThinkPixel integration contracts

## ThinkPixelAG

AG requests an eligible resolution in an authenticated tenant/catalog context and receives exact immutable signed content. AG independently determines whether a principal and Run may use it, which declared capabilities are granted, resource envelopes, policy freshness, and response to later lifecycle events.

MP publishes quarantine/revocation events and authoritative digest-state reconciliation. It does not terminate Runs or mint AG authority.

## ThinkPixelAR

MP exposes no direct execution API to AR. AR receives exact admitted digests through AG, pulls exact OCI content, and independently verifies materialized digests and runtime admission. It does not search MP or execute `latest` aliases during normal execution.

## ThinkPixelTG

MP supplies discovery and onboarding metadata only through administrator-controlled workflows. TG owns live MCP/tool configuration, destinations, credential isolation, reviewed tool schemas, risk/side-effect metadata, and execution. MP catalog membership never enables a tool or creates credentials.

## ThinkPixelWS

WS may retain immutable MP artifact identifiers and resolution references as Workspace provenance. Workspace membership, content, snapshots, or materializations do not alter MP catalog eligibility and do not authorize artifact use. A consumer must obtain any Run authority from AG independently of the Workspace reference.

## ThinkPixelMEM

MEM may submit learned procedure candidates through MP's authenticated publication workflow. MP treats the candidate and its provenance as untrusted publisher input and applies the same immutable registration, evidence, policy, review, and promotion controls as for other submissions. Learned content never becomes catalog-eligible or executable merely because MEM produced or retained it.

## ThinkPixelLLMGW and ThinkPixelGR

Marketplace requirements may describe model or guardrail compatibility, but MP has no authority to configure provider credentials, routing, model access, or runtime content/risk policy.

## Failure posture

Compromise or unavailability of MP must not cause AG/AR/TG to substitute mutable content, broaden authority, or expose credentials. Previously verified immutable state remains usable only under each consumer's own freshness/revocation contract.

## Coupling and identity

ThinkPixel integrations use public, versioned wire schemas and stable tenant, artifact, digest, catalog, resolution, and event identifiers where applicable. MP does not read or write another component's database and does not import another repository's internal implementation types. ThinkPixel-specific clients belong behind adapters so MP remains independently deployable with contract-compatible alternatives.
