# Memory Beads POC

## Workspace

Setup task: `bd-db5q.4`, tracked in the original b9s database.

- Clone: `$HOME/Documents/b9s-memory-poc`
- Branch: `feat/bd-db5q.4-memory-poc`
- Source branch: `feat/bd-87qs-memory-beads`
- Origin: local `$HOME/Documents/b9s`
- Preview source: `https://github.com/versioned-beads/beads.git`, integration commit `1f3466b5dfcb8c76ecb9ceaf4b847df695131f96`
- Preview executable: `.memory-preview/bin/bd`
- Viewer executable: `.memory-preview/bin/b9s`
- Storage: embedded Dolt, graph schema 6, under this clone's `.beads`
- Scope identifier: `http://127.0.0.1:8765/b9s-memory-poc/`

The scope is an identity URL. Setup does not start an HTTP listener or Dolt server.
The graph metadata contains the absolute workspace path. Keep the initialized clone at this location.
Setup created an empty graph without copying the source project's Beads store or credentials.
Task `bd-db5q.5` seeded 30 ADR Memories, two instruction Memories, and five Issue snapshots.
See [the seed and retrieval evaluation](memory-poc-evaluation.md) for provenance, limitations, and results.

## Run commands

Interactive terminals load `bd-prev` from `~/.zshrc`. This function always targets this POC, including when invoked from another repository:

```sh
bd-prev status --graph --json
bd-prev list --format records-json --all
bd-prev memories "embedded Dolt"
bd-prev recall adr-0025
```

Agent shells may not load `~/.zshrc`. Define the function below in the same shell invocation as the commands that use it.
It scopes both preview binaries and clears inherited database settings and credentials.
It does not change the installed `bd` or the shell's global PATH.

```sh
poc() (
  local poc_root="$HOME/Documents/b9s-memory-poc"
  builtin cd "$poc_root" || exit 1
  env -i \
    HOME="$HOME" \
    PATH="$poc_root/.memory-preview/bin:/opt/homebrew/bin:/usr/bin:/bin" \
    XDG_CONFIG_HOME="$poc_root/.memory-preview/config" \
    TMPDIR=/tmp \
    BEADS_DOLT_AUTO_START=0 \
    BEADS_DIR="$poc_root/.beads" \
    "$@"
)

poc bd status --graph --json
poc bd list --format records-json --all
poc b9s memories --project "$HOME/Documents/b9s-memory-poc"
poc b9s memories --project "$HOME/Documents/b9s-memory-poc" --id adr-0030
```

The first command opens the interactive Memory browser. Add `--print` for plain output. A test harness under `script` must set `TERM=screen-256color`, or the colour query waits five seconds and swallows the first keys.
Use this explicit `memories` command for the POC.

## Local checkout and binaries

The clone uses a non-cone sparse checkout with these patterns:

```text
/*
!/.beads/
```

This excludes the branch's tracked legacy Beads files. The preview creates a fresh `.beads` directory independently.
Do not disable sparse checkout or restore legacy `.beads` files into this directory.
`.git/info/exclude` excludes the fresh `.beads` and `.memory-preview` directories from new-file staging.
Database data and binaries are local and are not included in commits.

The preview binary was copied from the existing verified preview build.
Its Go build information records the pinned revision, `vcs.modified=false`, `CGO_ENABLED=1`, and `-tags=gms_pure_go`.
To rebuild, check out the exact revision in a separate source directory and run:

```sh
CGO_ENABLED=1 go build -tags gms_pure_go -o "$HOME/Documents/b9s-memory-poc/.memory-preview/bin/bd" ./cmd/bd
```

Build the viewer from this clone with:

```sh
go build -o "$HOME/Documents/b9s-memory-poc/.memory-preview/bin/b9s" ./cmd/b9s
```

## Seed plan

Task `bd-db5q.5` owns the seed operation. Export current b9s with its released `bd`, retain the source export, and convert selected Issues into supported preview CLI calls.
Recreate supported relationships and record source IDs. Report unsupported fields and history explicitly.
Graph mode does not support `bd import`. This is a one-way snapshot with no synchronization back to current b9s.

Keep the export and experimental data local. GitHub writes require explicit user consent for each action.
