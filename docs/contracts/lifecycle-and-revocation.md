# Artifact lifecycle and revocation

Lifecycle is independent from catalog membership.

All v1 lifecycle actions become effective at server commit time. Caller-scheduled future lifecycle transitions are not supported.

## Active

An active artifact may be considered by catalog policy. Active status does not imply eligibility or authorization.

## Deprecated

Deprecation is an immutable historical record that policy-blocks new protected promotion and resolution. Existing immutable resolutions remain reconstructable and are not invalidated automatically.

A deprecation may recommend an existing exact replacement digest of the same logical Artifact and compatible kind. Descriptive migration guidance may mention other artifacts but is not an authoritative replacement.

## Quarantined

Quarantine is reversible investigative state. While effective it blocks new promotion and protected resolution and emits an event identifying affected digest and reason. Existing resolutions are marked affected; MP does not terminate or authorize Runs. ThinkPixelAG applies its runtime policy.

Quarantine transitions and release are append-only administrative records; current state is derived without deleting history.

Release requires revocation-administrator authority. Ordinary publication or catalog administration cannot clear quarantine.

## Revoked

Revocation is an irreversible append-only statement against one exact digest. It excludes the digest from protected resolution and emits an event. A later correction may append explanation but cannot restore eligibility. Corrected content requires a new digest and semantic version.

Lifecycle actions require a stable reason code and may include bounded explanation plus up to 64 EvidenceRecord references.

A revocation records:

- UUIDv7 record ID, tenant, exact digest, and ArtifactVersion;
- severity: `low`, `medium`, `high`, or `critical`;
- stable reason code and bounded explanation;
- effective timestamp and authenticated revocation administrator;
- evidence record IDs/digests;
- optional exact replacement digest satisfying the same-artifact/kind rule;
- audit and outbox identity.

Revocation never deletes the ArtifactVersion, descriptor, evidence, catalog history, promotion decisions, locks, resolutions, or audit trail.
