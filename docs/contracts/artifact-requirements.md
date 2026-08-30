# Artifact requirements

Requirements are publisher declarations used for compatibility and policy. They never grant authority, choose infrastructure, or inject credentials.

## Capabilities

Capabilities are separated into `required` and `optional` sets of lowercase dotted identifiers such as `scm.repository.read`. MP validates syntax and canonical ordering. ThinkPixelAG's configured capability registry remains authoritative for semantic meaning and Run grants.

An unknown identifier remains a declaration for discovery, but protected catalog promotion fails unless policy explicitly permits that unknown capability. Loading a dependency or Bundle cannot add it to a Run.

## Runtime

Runtime requirements are abstract lower bounds:

- minimum isolation class and optional Runtime Profile name;
- supported operating system and architectures;
- minimum CPU in integer millicores;
- minimum memory, ephemeral storage, and Workspace storage in integer bytes;
- durable Workspace requirement;
- GPU requirement and abstract GPU classes;
- adapter kind and bounded compatibility range.

Publisher metadata cannot contain Kubernetes resource names, RuntimeClass, StorageClass, node selectors, service accounts, host mounts, device names, Linux capabilities, seccomp/AppArmor configuration, or secret references. Trusted AR/operator configuration maps abstract requirements to implementation.

## Network

The ordered v1 vocabulary is:

1. `none`;
2. `thinkpixel-only`;
3. `package-mirrors`;
4. `restricted-external`;
5. `unrestricted-standalone`.

`restricted-external` uses operator-defined endpoint-class identifiers, never publisher-provided URLs, CIDRs, DNS names, proxy settings, or credentials. A declaration expresses need; AG/AR policy may deny it or provide something more restrictive.

## External integrations

External requirements identify a typed integration class such as MCP server, A2A peer, model feature/family, or artifact store. They reference stable marketplace or operator vocabulary, not live credentials or arbitrary endpoints.

## Canonicalization

Set-like lists are unique and sorted. Quantities are non-negative bounded integers in the declared base unit. Unknown fields fail schema validation. The normalized requirement document uses the MP JSON canonicalization profile.
