# Hostile OCI and archive inspection

Inspection is static and streaming. MP never runs artifact scripts, hooks, entrypoints, interpreters, package managers, binaries, or generated code.

## Default instance limits

| Resource | Default |
| --- | ---: |
| OCI manifest | 4 MiB |
| JSON descriptor or Agent Card | 1 MiB |
| Compressed archive/layer | 256 MiB |
| Total expanded content | 1 GiB |
| Single expanded file | 128 MiB |
| Archive entries | 10,000 |
| Decompression ratio | 100:1 |

Limits are enforced incrementally across nested and aggregate content. Instance configuration may lower or raise defaults up to these compiled ceilings:

| Resource | Compiled ceiling |
| --- | ---: |
| OCI manifest | 16 MiB |
| JSON descriptor or Agent Card | 4 MiB |
| Compressed archive/layer | 1 GiB |
| Total expanded content | 4 GiB |
| Single expanded file | 512 MiB |
| Archive entries | 100,000 |
| Decompression ratio | 1,000:1 |

Publisher content cannot alter limits. Raising a compiled ceiling requires a new inspection profile and build.

## Path and file rules

- Reject absolute, empty, dot, traversal, duplicate-normalized, invalid UTF-8, NUL-containing, and platform-ambiguous paths.
- Reject symlinks, hardlinks, devices, sockets, FIFOs, sparse-file abuse, and unsupported special metadata in profiles that do not expressly allow them.
- Do not write hostile archives to a shared filesystem merely to inspect them.
- Enforce file count, compressed bytes, expanded bytes, per-file bytes, path length, nesting, and ratio before allocating unbounded memory or disk.
- Verify manifest, blob, layer, descriptor, and subject digests over exact bytes.
- Reject unknown media types for protected registration unless an explicit versioned profile supports them.

Limit errors are deterministic and reveal no partial content beyond bounded safe metadata.
