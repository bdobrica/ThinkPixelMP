# Agent Skill OCI packaging profile

## Native format preservation

The payload is an Agent Skills directory, not a converted ThinkPixel instruction schema. It contains `SKILL.md` at its root and may contain scripts, references, assets, and other files allowed by the pinned Agent Skills specification.

`SKILL.md` YAML frontmatter and Markdown body retain their native semantics. ThinkPixel metadata, evidence, requirements, and promotion state remain outside the file unless the upstream standard explicitly defines a compatible field.

The experimental Agent Skills `allowed-tools` field is a publisher declaration only. It is never interpreted as ThinkPixelTG availability, AG capability authority, credential permission, or pre-approval.

## OCI representation

- OCI artifact type: `application/vnd.thinkpixel.skill.v1`.
- Descriptor media type: `application/vnd.thinkpixel.skill.manifest.v1+json`.
- Payload media type: `application/vnd.thinkpixel.skill.layer.v1+tar+gzip`.
- The payload is one deterministic compressed tar archive rooted at the skill contents.
- Archive entries are sorted by normalized path. UID and GID are `0`, owner and group names are empty, and modification time is Unix epoch `0`.
- Directory mode is `0755`; ordinary regular-file mode is `0644`; executable script mode is `0755`. Other permission and metadata bits are removed.
- The gzip header timestamp is `0` and optional host-specific header fields are omitted.
- `SKILL.md` is present exactly once at the archive root and its declared name matches the package identity mapping.
- Paths are normalized relative UTF-8 paths. Absolute paths, empty/dot/traversal segments, duplicate normalized paths, links, devices, sockets, FIFOs, and other special files are rejected.
- Archive validation is bounded and never executes scripts, hooks, binaries, or package managers.

ThinkPixelMP generates the normalized [`SkillDescriptor` JSON](../../api/schemas/skill-descriptor-v1.schema.json) after safe inspection. Publishers do not maintain a second manifest that could disagree with `SKILL.md`. The generated descriptor is the OCI config blob and records the exact `SKILL.md` digest, normalized supported metadata, payload digest/size/count, requirements, dependencies, and derived class.

Resource bounds are recorded in the hostile-content inspection contract before implementation.

## Derived artifact class

The publisher may declare `instructional` or `executable-local`, but MP derives an effective class from validated contents. The stricter class wins, preventing executable content from being represented as instruction-only.

A skill is `executable-local` if any of these are present:

- any content beneath `scripts/`;
- any regular file carrying an executable bit before normalization;
- a recognized executable shebang;
- binary-looking content outside `assets/`;
- any fenced or indented Markdown code block in `SKILL.md`, regardless of its language label.

The code-block rule is intentionally conservative: legitimate examples and malicious instructions can use the same Markdown syntax. A prose-only skill can still instruct an agent to synthesize and execute code, so `instructional` means only that static validation found none of the versioned executable indicators. It never means safe, trusted, or eligible to bypass runtime, tool, network, or capability authorization.

Classification does not authorize execution. A compatible runtime and AG policy independently decide whether and how any script can run.

## Imported skills

Git or another upstream source may be snapshotted and repackaged into this OCI profile. Import records the original publisher/source, exact source revision or digest, importer identity/version, import time, validation profile, and resulting OCI digest. Import produces candidate state only.
