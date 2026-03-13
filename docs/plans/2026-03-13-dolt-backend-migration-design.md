# Dolt Backend Migration Design

**Date:** 2026-03-13
**Status:** Deferred (capturing context for future implementation)
**Beads task:** bd-18c8

## Table of Contents

- [Context](#context)
- [The Ecosystem Split](#the-ecosystem-split)
- [Why Yegge Moved to Dolt](#why-yegge-moved-to-dolt)
- [Why Dicklesworthstone Refused](#why-dicklesworthstone-refused)
- [What is Dolt](#what-is-dolt)
- [How Beads Uses Dolt](#how-beads-uses-dolt)
- [Current b9s Data Layer Architecture](#current-b9s-data-layer-architecture)
- [The Dilemma for b9s](#the-dilemma-for-b9s)
- [Migration Strategy](#migration-strategy)
- [Open Questions](#open-questions)
- [References](#references)

---

## Context

Three projects form the beads ecosystem:

| Project | Binary | Author | Role |
|---------|--------|--------|------|
| **Beads** | `bd` | Steve Yegge | Issue tracker CLI (data owner) |
| **Beads Viewer** | `bv` | @Dicklesworthstone | TUI viewer + analysis engine |
| **Beadwork** | `b9s` | @vanderheijden86 | Stripped-down fork of `bv` |

A fourth project emerged from the conflict: **beads_rust** (`br`) by @Dicklesworthstone, a Rust rewrite of `bd` that preserves JSONL compatibility.

In February 2026, Steve Yegge's `bd` v0.56+ switched its storage backend from flat JSONL files to Dolt, a version-controlled SQL database. This broke the entire viewer ecosystem, triggering four upstream issues in quick succession:

- **#112**: "support new beads Dolt-Powered mode" (Feb 20)
- **#118**: "Does this work with bead's new dolt engine?" (Feb 23)
- **#121**: "Dolt is not supported" (Feb 24)
- **#123**: "Support Dolt server mode (bd v0.56+) -- no issues.jsonl file" (Feb 27)

@Dicklesworthstone closed all four, declined Dolt support, and directed users to `beads_rust` instead.

**Our position:** b9s follows Steve Yegge's `bd` because it is the original, more popular, and more actively maintained project. We will need to support Dolt to stay compatible with the upstream ecosystem. We are currently on an older `bd` version using JSONL, which is a workable interim solution.

---

## The Ecosystem Split

```
┌─ Yegge Ecosystem ─────────────────────────────────────────────┐
│                                                                │
│   ┌──────────┐   MySQL wire    ┌──────────────┐               │
│   │  bd CLI  │ ──────────────> │ Dolt Server  │               │
│   │  v0.56+  │                 │ :3306        │               │
│   └──────────┘                 │              │               │
│                                │  ┌─────────┐ │               │
│                                │  │ issues  │ │               │
│                                │  │ deps    │ │               │
│                                │  │ comments│ │               │
│                                │  └─────────┘ │               │
│                                └──────────────┘               │
│   No JSONL. No bv/b9s compatibility.                          │
└───────────────────────────────────────────────────────────────┘

┌─ Dicklesworthstone Ecosystem ─────────────────────────────────┐
│                                                                │
│   ┌──────────┐   embedded     ┌──────────────┐               │
│   │  br CLI  │ ─────────────> │  fsqlite     │               │
│   │ (Rust)   │                │  (beads.db)  │               │
│   └────┬─────┘                └──────────────┘               │
│        │                                                      │
│        │ br sync --flush-only                                 │
│        v                                                      │
│   ┌──────────────────┐          ┌──────────────────┐         │
│   │ .beads/          │          │                  │         │
│   │  issues.jsonl    │ ────────>│  bv (viewer)     │         │
│   │  (flat file)     │  reads   │  b9s (fork)      │         │
│   └──────────────────┘          └──────────────────┘         │
│                                                                │
│   JSONL preserved. bv/b9s work as before.                     │
└───────────────────────────────────────────────────────────────┘
```

---

## Why Yegge Moved to Dolt

### 1. Concurrent Multi-Agent Writes

JSONL + git breaks when multiple AI agents work simultaneously:

```
Agent A: bd create "Add OAuth"     -> appends to issues.jsonl
Agent B: bd create "Add Stripe"   -> appends to issues.jsonl
                                          |
                                          v
                                git merge -> CONFLICT
                                Same file, same region
```

Dolt merges at the cell level (individual column values in individual rows). Two agents inserting different rows never conflict:

```
Agent A: INSERT INTO issues VALUES ('bd-a1b2', 'Add OAuth')
Agent B: INSERT INTO issues VALUES ('bd-f14c', 'Add Stripe')
                                          |
                                          v
                                dolt merge -> CLEAN
                                Different rows, no conflict
```

### 2. Git Merge is Structurally Unaware

Git treats JSONL as opaque text. When two agents update different fields on the same issue:

```
Agent A: {"id":"bd-x1", "status":"in_progress", "assignee":"agent-A"}
Agent B: {"id":"bd-x1", "status":"in_progress", "notes":"started work"}
```

Git sees two changed lines at the same position and creates a conflict. A human must resolve it manually. Dolt sees: "column `assignee` changed by A, column `notes` changed by B, different columns, auto-merge."

### 3. Real SQL Queries

With JSONL, every query loads the entire file into memory and filters. With Dolt, `bd ready` can compute transitive blocking via SQL JOINs across `issues` + `dependencies` tables efficiently.

### 4. Independent Data Sync

Dolt has its own push/pull/remote system (DoltHub, S3, GCS, or `git+ssh://`). This means:
- Issue data syncs independently of the code repo
- No bloat in git history from large JSONL files
- Time-travel queries: `SELECT * FROM issues AS OF 'abc123'`
- Share issue state without sharing code

### 5. Community Discussion (GitHub Discussion #1836)

The community discussion revealed the tension clearly:

**Arguments for Dolt (Peter KC, contributor):**
1. SQLite's WAL contention blocks concurrent writers; Dolt's server handles simultaneous sessions
2. Every mutation is tracked; users can query historical states without manual snapshots
3. SQL views and queries work without blocking active agents
4. Native push/pull semantics like Git

**Arguments against (community):**
- "Removing SQLite creates a setup dependency issue hell"
- "Introducing Dolt just isn't aligning with what beads is supposed to be doing" (Unix philosophy)
- Migration was problematic: non-deterministic event ordering, missing field migrations, silent data loss

**Yegge's decision:** Drop SQLite entirely, Dolt is the sole backend. In v0.51.0, the SQLite backend was removed.

---

## Why Dicklesworthstone Refused

### 1. Architectural Complexity

From upstream issue #112:
> "Dolt integration would add significant architectural complexity that's hard to justify... adding a separate Dolt backend would essentially double the data layer surface area."

### 2. Runtime Dependency

JSONL: read a file. No server process, no port, no PID file, no lifecycle management.

Dolt: need `dolt` installed, a server running, connection handling, reconnection logic, timeout handling.

### 3. The Viewer Doesn't Write (or Writes Via CLI)

`bv` is primarily read-only (mutations delegate to `bd`/`br` CLI). The concurrent multi-agent write problem that justified Dolt does not apply to a viewer. A viewer reading a file is simpler and more reliable than one connecting to a database server.

### 4. Fork the CLI Instead

Rather than adapting the viewer to Dolt, @Dicklesworthstone built `beads_rust` (`br`), an alternative CLI that uses fsqlite + JSONL. If you control both the CLI and the viewer, you control the data format.

---

## What is Dolt

[Dolt](https://github.com/dolthub/dolt) is a MySQL-compatible database with built-in version control. It provides:

- **100% MySQL compatibility** (connects on standard MySQL port)
- **Git-like version control**: branches, commits, merges, diffs, blame
- **Cell-level merge**: resolves conflicts at the individual field level, not line level
- **Time travel**: query any historical state with `AS OF` syntax
- **Dolt remotes**: push/pull to DoltHub, S3, GCS, or `git+ssh://`

| Aspect | JSONL + Git | Dolt |
|--------|-------------|------|
| Runtime dependency | None (just a file) | Dolt binary (embedded or server) |
| Access pattern | Read a text file | MySQL client connection |
| Portability | Copy the file anywhere | Need Dolt installed |
| Version control | Git tracks the file | Dolt has its own branch/commit/merge |
| Query capability | Load all, filter in-memory | Full SQL |
| Merge conflicts | Text-based git merge | Schema-aware cell-level merge |
| Complexity | Minimal | Significant (server process, port, config) |

---

## How Beads Uses Dolt

Dolt operates in two modes, both storing data in `.beads/dolt/` (which is `.gitignore`'d):

### Embedded Mode (default for solo use)

- No server needed. The `bd` binary includes the Dolt engine in-process.
- Single-writer only (one process at a time).
- Data lives in `.beads/dolt/` alongside your code.
- Zero ops: no server, no ports, no PID files.

### Server Mode (default for multi-agent)

- Runs `dolt sql-server` locally (MySQL protocol on port 3306).
- Supports multiple agents writing simultaneously.
- Auto-starts when needed, PID stored in `.beads/dolt-server.pid`.

```
Embedded (solo)              Server (multi-agent)
┌──────────┐                 ┌──────────┐
│ bd CLI   │                 │ bd CLI   │──┐
│ (in-proc │                 │          │  │ MySQL
│  Dolt)   │                 ├──────────┤  │ protocol
└────┬─────┘                 │ bd CLI   │──┤
     │                       │ (agent2) │  │
     v                       └──────────┘  │
.beads/dolt/                      v        │
(local files)              dolt sql-server <┘
                                 │
                            .beads/dolt/
                            (local files)
```

Key architectural points:

- `.beads/dolt/` is NOT checked into Git. The docs have a troubleshooting section for people who accidentally committed it.
- Sync uses Dolt remotes, not Git: `bd dolt push` / `bd dolt pull`.
- `bd export` still produces JSONL for portability, but it is no longer the primary storage.
- Dolt provides its own versioning: branching, time travel, diff, blame, cell-level merge.
- The Dolt database schema has tables: `issues`, `dependencies`, `comments`, `wisps` (ephemeral issues), and more.

### Detection

A Dolt-mode project is identified by:
- `.beads/metadata.json` containing `"database": "dolt"`
- `.beads/config.yaml` with `dolt-auto-commit: "on"`
- Presence of `.beads/dolt/` directory

---

## Current b9s Data Layer Architecture

### Read Path

```
cmd/b9s/main.go
  |
  v
datasource.LoadIssues("")
  |
  v
datasource.DiscoverSources()        -- finds SQLite, local JSONL, worktree JSONL
  |
  v
datasource.SelectBestSource()       -- picks freshest valid source
  |
  v
datasource.LoadFromSource(source)   -- dispatches by type:
  |                                      SourceTypeSQLite    -> SQLiteReader
  |                                      SourceTypeJSONLLocal -> loader.LoadIssuesFromFile()
  v
[]model.Issue
```

**Key types already in place:**

- `SourceType` enum: `"sqlite"`, `"jsonl_worktree"`, `"jsonl_local"`
- `DataSource` struct: Type, Path, Priority, ModTime, Valid, IssueCount
- `SQLiteReader`: full reader with `LoadIssues()`, `GetIssueByID()`, dependency/comment loading
- `DiscoveryOptions`, `SelectionOptions`, `ValidationOptions`: configurable discovery

### Write Path

```
EditModal (huh form)
  |  user submits
  v
EditModal.BuildUpdateArgs() / BuildCreateArgs()
  |  builds map[string]string of changed fields
  v
IssueWriter.UpdateIssue() / CreateIssue() / CloseIssue()
  |  queues async tea.Cmd
  v
exec.Command("bd", args...).CombinedOutput()
  |  runs in activeProjectPath
  v
bd CLI writes to storage (JSONL today, Dolt in future)
  |
  v
FileChangedMsg (watcher detects write)
  |
  v
datasource.LoadIssues("") -- fresh reload
```

**The write path delegates entirely to `bd` CLI.** This is architecturally significant: b9s does not write to the data store directly. It calls `bd update`, `bd create`, `bd close`, etc. This means the write side will work unchanged with Dolt, as long as `bd` is Dolt-configured.

### Live Reload

- `pkg/watcher/watcher.go` monitors the JSONL/SQLite file via fsnotify + polling fallback
- On change: reloads all issues via `datasource.LoadIssues()`
- This mechanism will need adaptation for Dolt (no file to watch)

---

## The Dilemma for b9s

### Why We Must Eventually Support Dolt

1. **Upstream alignment**: `bd` is the canonical beads CLI. Steve Yegge is the co-founder. The community follows `bd`.
2. **JSONL is deprecated**: `bd` v0.56+ no longer generates `issues.jsonl`. Staying on JSONL means staying on an old `bd` version forever.
3. **b9s writes**: Unlike pure-viewer `bv`, b9s creates and edits issues. The write path goes through `bd`, so as `bd` upgrades to Dolt, the write side just works. But the read side breaks.
4. **Multi-agent future**: As AI agent usage grows, concurrent write support becomes more important, and Dolt handles this natively.

### Why We Can Defer

1. **Current `bd` version works**: We're on an older `bd` version using JSONL. Everything functions.
2. **No urgent need**: We don't have multi-agent concurrent write scenarios today.
3. **Complexity budget**: Adding a MySQL client dependency, connection management, and a polling-based change detector is non-trivial.
4. **Upstream is still alpha**: `bd` is v0.9.x. The Dolt integration may change before 1.0.

### The Fundamental Tension

```
                     SIMPLICITY                          CAPABILITY
                         |                                   |
  <──────────────────────┼───────────────────────────────────┼──>
                         |                                   |
    JSONL + git          |                            Dolt (SQL DB)
                         |
    Zero dependencies    |                    Cell-level merge
    Read with cat/jq     |              Multi-agent concurrent writes
    git tracks it free   |                   SQL queries
    Any tool can parse   |                Time travel
    Works offline        |              Dolt remote sync
```

For b9s specifically: the write path already delegates to `bd`, so it will transparently support Dolt when we upgrade `bd`. The read path is the real work. The watcher also needs rethinking (no file to watch with Dolt).

---

## Migration Strategy

### Phase 0: Current State (now)

- b9s reads JSONL via `pkg/loader/` and SQLite via `internal/datasource/sqlite.go`
- b9s writes via `bd` CLI
- Watcher monitors JSONL file
- `bd` is an older version using JSONL storage
- **This works. No action needed yet.**

### Phase 1: Add DoltReader (when ready to migrate)

Add a new `SourceType` and reader:

```go
const SourceTypeDolt = "dolt"
```

Two sub-approaches for the reader:

**Option A: Direct MySQL connection**
- Add `go-sql-driver/mysql` dependency
- Create `DoltReader` that connects to `127.0.0.1:3306` (or embedded mode)
- Reuse the SQLiteReader pattern (same SQL tables, different driver)
- Pro: Real-time reads, full SQL capability
- Con: MySQL driver dependency, connection lifecycle management

**Option B: Use `bd list --json --all` for reads**
- Shell out to `bd list --json --all` and parse the JSON output
- Pro: Zero new dependencies, always compatible with whatever `bd` does
- Con: Slower (process spawn per read), no incremental updates

**Recommendation:** Option A for reads (direct MySQL), keep Option B as fallback. The SQLiteReader already demonstrates the pattern: same tables, different driver.

### Phase 2: Adapt Discovery

Update `DiscoverSources()` to detect Dolt:
- Check for `.beads/metadata.json` with `"database": "dolt"`
- Check for `.beads/dolt/` directory
- Check for running Dolt server on port 3306
- Return `SourceTypeDolt` with high priority (110, above SQLite's 100)

### Phase 3: Adapt Watcher

The file watcher won't work for Dolt (no file changes to detect). Options:

**Option A: Poll the database**
- Periodically query `SELECT MAX(updated_at) FROM issues`
- Compare with last known timestamp
- Pro: Simple, works with both embedded and server mode
- Con: Polling interval introduces latency

**Option B: MySQL binlog / Dolt diff**
- Subscribe to Dolt's change feed
- Pro: Real-time notifications
- Con: Complex, Dolt-specific API

**Recommendation:** Option A (polling). The current watcher already has a polling fallback mode for remote filesystems. Extend this pattern for Dolt.

### Phase 4: Upgrade bd

- Upgrade local `bd` to v0.56+ with Dolt support
- Run `bd init` to create Dolt database from existing JSONL
- Verify b9s reads from Dolt correctly
- Verify b9s writes (create/edit/close) work through the new `bd`

### Phase 5: Remove JSONL as Primary

- Keep JSONL loading as a fallback (for portability, `bd export` output)
- Make Dolt the preferred source when detected
- Update documentation

---

## Open Questions

1. **Dolt schema**: What exact tables and columns does `bd` v0.56+ create? The SQLiteReader's schema may not match Dolt's. Need to inspect a real Dolt database.
2. **Embedded vs. Server**: Should b9s connect via MySQL protocol even in embedded mode, or use the Dolt Go library directly? The MySQL approach is simpler and works for both modes.
3. **Connection lifecycle**: When should b9s connect/disconnect? On startup? On demand? Keep-alive?
4. **Error handling**: What happens when the Dolt server is down? Graceful fallback to JSONL export?
5. **Dolt Go SDK**: Does Dolt have a Go library for embedded access, or must we always go through MySQL protocol?
6. **bd version detection**: How does b9s detect whether the local `bd` is JSONL-mode or Dolt-mode? Check `bd --version`? Check `.beads/metadata.json`?
7. **Watcher polling interval**: What's an acceptable latency for detecting changes? 1s? 2s? Configurable?

---

## References

- [Dolt: Git for Data (GitHub)](https://github.com/dolthub/dolt)
- [Dolt Documentation](https://docs.dolthub.com)
- [Beads (bd) by Steve Yegge](https://github.com/steveyegge/beads)
- [Beads FAQ](https://github.com/steveyegge/beads/blob/main/docs/FAQ.md)
- [Beads DOLT-BACKEND.md](https://github.com/steveyegge/beads/blob/main/docs/DOLT-BACKEND.md)
- [Beads DOLT.md](https://github.com/steveyegge/beads/blob/main/docs/DOLT.md)
- [Beads Viewer (upstream bv)](https://github.com/Dicklesworthstone/beads_viewer)
- [Upstream Issue #112: support new beads Dolt-Powered mode](https://github.com/Dicklesworthstone/beads_viewer/issues/112)
- [Upstream Issue #118: Does this work with bead's new dolt engine?](https://github.com/Dicklesworthstone/beads_viewer/issues/118)
- [Upstream Issue #121: Dolt is not supported](https://github.com/Dicklesworthstone/beads_viewer/issues/121)
- [Upstream Issue #123: Support Dolt server mode](https://github.com/Dicklesworthstone/beads_viewer/issues/123)
- [Discussion #1836: Keep SQLite Backwards Compatibility](https://github.com/steveyegge/beads/discussions/1836)
- [Beads Best Practices (Medium)](https://steve-yegge.medium.com/beads-best-practices-2db636b9760c)
