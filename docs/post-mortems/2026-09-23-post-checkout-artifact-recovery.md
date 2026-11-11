# Checkout hook incident: artifact recovery status

Task: bd-p8zj. Incident provenance: bd-i8ag, reported on 2026-09-23.
Recovery attempt: 2026-11-11, in an isolated Runner Pod.
Status: blocked on incident artifacts, with no causal reproduction or product fix.

## Reported incident

The bd-i8ag description records `git worktree add .worktrees/docs-readme-rewrite`
invoking `.beads/hooks/post-checkout`, then `bd hooks run post-checkout`.
It reports more than two minutes waiting under a 300-second timeout, followed by
this output:

```text
beads: repair: bd dolt remote add origin git+ssh://git@github.com/vanderheijden86/b9s.git && bd dolt push
```

This is reported output, not a repair instruction. The recovery attempt did not
execute either suggested command. The incident command's exact duration, final
exit code and complete original output were not available.

## Recovered evidence

The last commit on `main` before the reported incident timestamp is
`6e05818d5c4980949fb98c2008a8a8949785da10`. Its tracked post-checkout shim is
byte-identical to the shim at the recovery base,
`eb77cef9f8e68e9854f3c1b6ec6a6074b812830a`.
Both have SHA-256
`cfaf5be55306380ad70b2831dc64f218b6006b8b3f8bff6d425e0a96c5e22752`.
This establishes the committed shim bytes, not the incident worktree's bytes.

The shim has a `v0.63.3` integration marker. It resolves `bd` through `PATH`,
exports `BD_GIT_HOOK=1`, and invokes `bd hooks run post-checkout` with the Git
arguments. If `timeout` is available, it uses `BEADS_HOOK_TIMEOUT`, defaulting to
300 seconds. Otherwise, that timeout does not apply. The marker does not identify
the executable selected by the incident shell.

The historical tracked `.beads/config.yaml` contains:

```yaml
backup:
  enabled: false
```

Its comments place the server configuration in machine-specific
`.beads/metadata.json`. Git history records that file being untracked on
2026-07-07, in `430e6217dcac329dddce93b843f028ba275abdc4`.
The pre-incident tree contains no tracked metadata or `post-checkout.old`.
That absence cannot establish whether either existed on the incident machine.
No `beads: repair:` string was found in the tracked `.beads` or `scripts`
trees at either the pre-incident commit or the recovery base.

## Prior checkpoint recovery

The unapproved checkpoint `refs/heads/omnigent-checkpoints/bd-i8ag` resolves to
`d2c8ce2de3b1445b5d3b9cdc62ceb5767d91eae6`. It was fetched and inspected as
data, without restoring it over the delivery worktree.
Its checkpoint directory contains only the manifest and
`reports/0-implementation-report.md`.

That report records an official Linux v0.63.3 archive checksum of
`5f4efd2e010209b3f381dbcd783b2a3a652f50ea72f40ef04c8ba434d408bf9e`,
source revision `884cc1171e3eb3ee44c1762973dfb8da6b56cdcc`, and loader
exit 127 for missing `libicui18n.so.74`.
It also records a different Pod binary returning in 0.2446 seconds without the
repair hint. These are prior reported observations, not executions in this
attempt or proof of incident executable identity.

The checkpoint does not contain the referenced `isolated-hook.json`,
`current-hook.json`, `stock-runtime.txt`, release binary, or incident artifacts.
The previous `/tmp/bd-i8ag-reports` directory is absent in this Pod.

## Current Pod identity

`bd` resolves to `/usr/local/bin/bd`, a lane connection wrapper with SHA-256
`6d379e04af9063ec49e955a4da7d0a2cbf998f9e51cd690207d3acc6cbb21d8b`.
The wrapper loads the lane's mounted connection when needed and delegates to
`BD_REAL`, defaulting to `/usr/local/libexec/beads/bd`.
No `BD_REAL` override was present during the inventory.

The default executable has SHA-256
`33eef85573dc10c0f846dad743f9a30503f52e4fb60d6f9eb4ab5be83bb2c501`.
Its Go build metadata identifies v1.3.0-rc.1, revision
`9c6a69ec12350959ec8c495c74eeb02902d629b6`, built with Go 1.26.7 for
Linux amd64. This is the Pod's identity, not recovered incident provenance.

`core.hooksPath` is unset in the Pod. There is no installed
`.git/hooks/post-checkout`, `.git/hooks/post-checkout.old`, or
`.beads/hooks/post-checkout.old`. The tracked shim was preserved.
The dynamic linker cache lists ICU 76, with no ICU 74 entry.
Neither the original executable format nor its library requirements are known.
The stock Linux loader failure cannot establish the incident machine's runtime.

## Required external prerequisites

All unresolved acceptance items belong to **bd-p8zj**:

1. The actual incident `bd` executable, its checksum, build/install provenance
   and executable format. A release marker or a substitute binary cannot identify it.
2. The incident shell's ordered PATH, selected executable path, wrapper or function
   contents, relevant overrides and Git hook configuration.
3. The complete incident hook chain, including the active post-checkout hook
   and `post-checkout.old`, or contemporaneous evidence that no chained hook existed.
4. Sanitized incident metadata, config and relevant environment settings, plus
   a compatible isolated runtime for that identified executable. Secrets and
   shared-server write access are not prerequisites.
5. A bounded replay that captures exact arguments, elapsed time, final exit,
   output and the process responsible for both the delay and repair diagnostic.

These artifacts are absent from the inspected checkout and prior checkpoint.
Laptop worktree paths are discovery information and are not mounted Pod paths.
Without them, constructing a runtime or executing a different stock hook would
not satisfy the incident identity or reproduction requirements.

No replacement hooks, database repair, Dolt remotes, shared database experiments,
or speculative product changes were made. No regression test is justified until
the responsible executable or hook and its behavior are established.
