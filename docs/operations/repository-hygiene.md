# Repository hygiene

`make repository-hygiene` scans every file recorded in the Git index and fails
when it finds high-confidence registry credentials, private signing keys,
JWT/OIDC tokens, assigned integration or test secrets, credential-bearing files,
or files beneath explicitly private/raw evidence and test-secret directories.
The root `make verify` gate runs this check in CI and local verification.

Public, minimized verification records belong in `docs/evidence/`. Raw reports,
tenant-private evidence, credentials, signing material, and token-bearing test
fixtures must remain outside Git. Tests use unmistakably synthetic values created
at runtime. Local configuration uses ignored files or external secret references;
`.env.example` may document names with obvious placeholders but no usable values.

If the check finds real restricted material, removing it from the latest revision
is not sufficient: rotate or revoke the credential and follow the repository
owner's history-remediation and incident process. Do not add scanner exceptions
for real material.
