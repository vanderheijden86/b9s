# Gate Memory views on bd capability detection

Task: bd-db5q.22. Status: implemented under bd-db5q.23; the decision is [ADR 0045](../adr/0045-probe-bd-help-before-memory-reads.md).

## Goal

Ship the Memory Beads views in the ordinary b9s build. One binary works against
any bd: when the bd underneath and the open workspace both support Memory
Beads, the Memory views open; otherwise `M` explains why in one line, and no Memory-only
work runs. People
working on Memory Beads then get a good viewer without a separate launcher.

## What exists today

- Every Memory read shells out to the `bd` on PATH (`internal/datasource/graph_preview.go`,
  `memory_graph.go`): `bd list --format records-json`, `bd memories --format records-json`,
  `bd show --json`, `bd links --json`, `bd versions --json`, `bd graph --view generic --json`.
- ADR 0044 detects support from `.beads/metadata.json` (`graph_mode == "link"`,
  `graph_ready == true`) and from refusals in actual reads, then explains failures.
- The `M` key, its help entry and the browser are always present. A stock bd
  user who presses `M` gets an error explanation instead of no feature at all.

## Upstream state (gastownhall/beads, 2026-10-07)

- Memory Beads is a proposal: #5877, tracker #6535, phases #6536 to #6542, all open.
  No released bd (latest v1.3.1, pre-release v1.3.2-rc.1) contains it.
- Planned shape: a `type: "memory"` descriptor in the graph store's Type catalog,
  shipped behind an off-by-default flag. R15 (#6542) forbids advertising support
  until the History capabilities exist, which implies an explicit advertisement
  will exist, but no token is defined yet.
- Upstream already has one capability mechanism: `capabilities[]` on
  `GET /v0/beads/context` (`bd serve`), with legacy tokens `memories.list|remember|get|forget`.
  Those describe the key/value memories (`config` rows `kv.memory.*`), not Memory Beads.
- The preview bd used by b9s (`~/Documents/b9s-memory-poc/.memory-preview/bin/bd`)
  is a local build. It reports `"version": "1.3.0"`, identical to a release.

## Signals considered

| Signal | Verdict |
|---|---|
| `bd version --json` semver | Rejected. The preview reports 1.3.0; forks and rc builds lie; no target version exists |
| `bd version --json` `branch` | Rejected. Names the checkout, not the feature (the stock rc reports `feat/bd-87qs-memory-beads`) |
| `bd types --json` lists `memory` | Likely future upstream signal, but it opens the database: on the shared server it hits the migration gate. Usable only from a neutral directory, and only once upstream lands it |
| `bd versions --help` exits 0 | **Preview signal.** Stock bd answers `unknown command "versions"`. Help never opens a database |
| `.beads/metadata.json` `graph_mode`/`graph_ready`/`graph_schema_version` | **Workspace signal.** Free, already used. Says the data is a graph workspace, not that this bd can read it |
| Explicit token (`capabilities` in `bd version --json`, or `memory.beads` in `/v0/beads/context`) | **Best long-term signal.** Does not exist yet; request it upstream |

## Design

Support is the product of two independent axes, so detection has two parts.

```
  bd binary ──▶ BinaryProbe ──▶ dialect: none | preview-v2 | upstream
                                      │
  .beads/metadata.json ──▶ Workspace ─┤  graph_mode=link, graph_ready, schema in range
                                      ▼
                         MemoryCapability: Unsupported | NotEnabled | Ready
                                      │
               ┌──────────────────────┼─────────────────────┐
               ▼                      ▼                     ▼
          main TUI (M, help,     web (reason in            b9s memories
          DECISIONS, graph load)  /api/memory-graph)        (exit with reason)
```

### Binary probe (owned by `internal/bdrun`)

- Run in a fresh empty temp directory with `BEADS_DIR` unset, so no probe can
  open, migrate or lock a database.
- Ladder, first hit wins:
  1. An explicit capability token, once upstream defines one.
  2. Upstream dialect: `memory` in `bd types --json` (neutral directory), behind
     the token check until upstream ships.
  3. Preview dialect: `bd versions --help` and `bd links --help` both exit 0.
  4. Otherwise `none`.
- Bound every call (2 s each). A timeout counts as `none` and is logged through `pkg/debug`.
- Cache the result keyed by resolved bd path, size and modification time, in
  the b9s cache directory. A rebuilt or swapped bd re-probes; a normal start costs one `stat`.

### Workspace check (owned by `internal/datasource`)

- Keep the ADR 0044 metadata check. Add a supported range for
  `graph_schema_version` (today: 6) so a newer graph format reads as
  `NotEnabled: unsupported graph schema` instead of a parse failure.

### Combined state

- `Unsupported`: the bd has no Memory dialect.
- `NotEnabled`: the bd has a dialect, the workspace is not a ready graph workspace.
- `Ready`: both. Runtime read failures stay explained as ADR 0044 does.
- Computed once per project open and again on project switch.

### Gating

- **Main TUI**: `M` stays bound and listed in help, so the feature is
  discoverable with any bd. Pressing it checks the capability: `Ready` opens
  the browser; otherwise the status bar shows the reason in one line (for
  example "bd on PATH has no Memory Beads support") and no browser opens. Work
  that only makes sense with Memories is skipped when not `Ready`: the DECISIONS
  column and detail section are not rendered, and the Memory graph read is not
  run at load, so Issues load through the normal path.
- **Web**: `/api/memory-graph` answers `available: false` with a one-line
  `reason` unless a graph was captured. The Memory tab stays visible and shows
  "Memories unavailable: <reason>" instead of a graph. One endpoint keeps the
  tab to a single request and the reason in the same response as the graph.
- **`b9s memories`**: prints the state and reason (for example "bd on PATH has no
  Memory Beads support") and exits 2. This is the discovery path for someone
  who expects the feature.
- **Lever**: `B9S_MEMORY=off` (and a config key) forces the feature off. It can
  only subtract: there is no way to force it on against a failing probe.
- `b9s --debug` logs the probe result, dialect, cache hit and workspace reason.

### Response dialects

Parsing today requires `"preview": true` and type suffixes `/preview-memory-v2`
and `/preview-issue-v2`. Put that behind the `preview-v2` dialect. The
`upstream` dialect (`type: "memory"`) gets its own parser when upstream phase 1
(#6537) merges; until then the ladder never yields `upstream`.

## Implementation steps

1. Binary probe in `internal/bdrun` with cache, plus fake `bd` scripts in
   testdata (stock-like, preview-like, hanging) for tests.
2. `MemoryCapability` in `internal/datasource`: combine probe and workspace
   check, add the schema range, skip the graph read when not `Ready`.
3. Main TUI: `M` checks capability and explains when not `Ready`; DECISIONS column,
   detail section and graph load only when `Ready`.
4. Web UI: `reason` on `/api/memory-graph`; the tab shows it when there is no graph.
5. `b9s memories` reason and exit code, `B9S_MEMORY=off` lever, debug logging.
6. ADR 0045 extending 0044: add the binary probe ahead of the reads it already
   checks, and skip Memory-only work (graph load, DECISIONS) when not `Ready`.
   0038's "always offer, explain failure" stays.
   Update `ARCHITECTURE.md`, `USER_GUIDE.md`, `testing.md`.
7. E2E: the same built b9s against the stock-like and the preview-like fake bd,
   asserting `M` explains with stock bd and opens the browser with preview bd, and once against the real preview workspace.
8. Deferred: `upstream` dialect parser when gastownhall/beads #6537 merges.
9. Upstream request for an explicit capability token, drafted for the owner to post.

## Risks

- The preview signal (`bd versions` exists) is specific to this preview. When
  upstream ships `bd versions` for History (#6132) without Memory, the ladder
  must not misread it: step 3 also requires `bd memories --help` to mention
  `records-json`, and the explicit token takes over as soon as it exists.
- A stale cache after an in-place `go install` that keeps the mtime is unlikely;
  `b9s --reprobe` clears it.
