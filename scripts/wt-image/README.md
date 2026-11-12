# Build a local worktree image

`wt-image` builds the current worktree, including staged changes, unstaged
changes and nonignored untracked files. It reuses
`scripts/preview/Dockerfile`, uses this repository's committed vendor tree,
and retains a frozen build context outside the worktree.
It never commits, changes the index, creates a namespace or changes a cluster
configuration.

Run from the repository root with Go 1.25.5 or later. Install the committed
dependencies with `go mod download` first.

## Registry delivery

For a local Docker daemon and an existing registry mirror:

```sh
go run ./scripts/wt-image \
  --repo /absolute/path/to/b9s \
  --builder docker --delivery registry \
  --registry localhost:5000 \
  --push-registry localhost:5001 \
  --output /tmp/b9s-image-run-1
```

`--registry` is the reference cluster nodes pull. `--push-registry` is the
Docker push endpoint for the same registry. Both flags require an explicit
registry authority: `localhost`, a dotted hostname, or a hostname with a port.
An optional path prefix follows the authority, for example
`localregistry:5000/b9s`. Bare names such as `localregistry` are rejected
before source capture or building because Docker interprets them as Docker Hub
namespaces. A dot in a path prefix does not qualify the authority.
The cluster must already map
`localhost:5000` to that registry. A registry reference with
`imagePullPolicy: Always` needs this mirror even after a k3d import. See the
baseline's k3d gotchas. The command does not create or restart registries or nodes.

Inside a runner Pod, use the existing BuildKit service and local registry:

```sh
go run ./scripts/wt-image \
  --repo /home/user/workspace/b9s \
  --builder buildkit --delivery registry \
  --buildkit-addr tcp://buildkitd.buildkit.svc.cluster.local:1234 \
  --registry registry.registry.svc.cluster.local:5000 \
  --insecure-registry \
  --output /tmp/b9s-image-run-2
```

BuildKit pushes the image and exports its runtime filesystem to
`OUTPUT/runtime`. The local registry must be reachable by both BuildKit and
cluster nodes. `--insecure-registry` enables HTTP for this local registry.

## Direct k3d import

For Docker and an existing local k3d cluster:

```sh
go run ./scripts/wt-image \
  --repo /absolute/path/to/b9s \
  --builder docker --delivery k3d --cluster my-local-cluster \
  --output /tmp/b9s-image-run-3
```

Only the named cluster receives the image. Deploy the returned reference with
`imagePullPolicy: IfNotPresent` or `Never`. This command does not deploy a pod.

## Source identity and evidence

The final JSON and `OUTPUT/result.json` contain the cluster image reference,
commit, dirty flag, tree hash, delivery method and evidence path. The tag
contains the commit prefix, dirty flag and full tree hash. A failed build,
identity check, push or import exits nonzero and does not write `result.json`.
Every subprocess shares the total deadline, ten minutes by default.
`--timeout` can reduce it.

The image bakes `/app/.commit-sha` and `/app/.source.json`. The latter is:

```json
{"commit":"FULL_HEAD_SHA","dirty":true,"tree_hash":"64_LOWERCASE_HEX_DIGITS"}
```

OCI revision and `dev.b9s.worktree.dirty` / `dev.b9s.worktree.tree-hash`
labels carry the same values. Deployment environment variables cannot change
these files. The helper reads the baked source and compares all three fields
before it reports success. The existing preview entrypoint still requires its
read-only database and viewer configuration. A local sample pod can override
the entrypoint to run `/usr/local/bin/b9s` or a shell. This image does not add
`bd`, seed a database, provide pod access or compare a running pod with its
worktree. Those belong to the other tasks in bd-a5xi.

`OUTPUT/source.json` records the expected identity. `manifest.json` records
each included path, normalized mode and SHA-256 of its bytes (or symlink
target). The tree hash is SHA-256 of sorted UTF-8 records:
`path + NUL + mode + NUL + file_sha256 + LF`. Modes are `100644`,
`100755` or `120000`. The copied files use those modes. Modification times,
ownership, the index and absolute worktree path do not affect the hash.
The hash identifies the captured source input, not a reproducible image digest.
Base image digests and build commands are retained in the context and logs.

The context uses Git's tracked files and nonignored untracked files, omits
missing tracked files, and applies the preview Dockerfile's literal relative
path exclusions. This excludes `.git`, `.beads`, worktrees, docs and fixtures,
but retains embedded `pkg/web/dist` and the committed vendor tree. Unsupported
Docker ignore patterns fail before a build, so a filter change cannot silently
invalidate the hash contract. The dirty flag describes Git's whole worktree
status, including changes outside the build context. A changed HEAD or status
during capture fails the command. The frozen context is the builder's only
source, so later worktree edits cannot change the build input.

Each run requires a fresh output directory outside the worktree. Contexts,
runtime exports and logs remain there for review. BuildKit also writes
`image.json` with the image digest. No automatic cleanup runs.

## Tests

```sh
go test ./scripts/wt-image -count=1 -timeout=30s
```

The tests use disposable Git repositories and tool stubs, without cluster or
registry access. They cover dirty source capture, exclusions, deterministic
hashes, executable modes, symlink escapes, mirror delivery, scoped k3d import,
metadata mismatch and push failure. A real image build and runtime metadata
check are separate required evidence. Registry tests reject ambiguous
authorities in both flags before any builder runs and retain explicit local,
dotted and port-qualified destinations.
