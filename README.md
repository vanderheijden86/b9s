# B9s

![Go Version](https://img.shields.io/github/go-mod/go-version/vanderheijden86/b9s?style=for-the-badge&color=6272a4)
![License](https://img.shields.io/badge/License-MIT-50fa7b?style=for-the-badge)

> A fast, focused TUI viewer and editor for [Beads](https://github.com/steveyegge/beads) issue tracking projects. Inspired by [k9s](https://k9scli.io/).

## Contents

- [What is this?](#what-is-this)
  - [Why strip it down?](#why-strip-it-down)
- [Features](#features)
  - [Relationship to the original](#relationship-to-the-original)
- [Installation](#installation)
  - [Homebrew (macOS/Linux)](#homebrew-macoslinux)
  - [From source](#from-source)
- [Quick Start](#quick-start)
- [Data Backends](#data-backends)
  - [Source Priority](#source-priority)
  - [Dolt Server Mode](#dolt-server-mode)
  - [Embedded Dolt Mode](#embedded-dolt-mode)
  - [JSONL and SQLite (Legacy)](#jsonl-and-sqlite-legacy)
  - [Fallback Behavior](#fallback-behavior)
- [Keyboard Quick Reference](#keyboard-quick-reference)
- [Acknowledgments](#acknowledgments)
- [License](#license)

## What is this?

B9s is a terminal-based interface for browsing, editing, and managing Beads issues. It supports multiple data backends natively (Dolt, SQLite, JSONL) and renders your issue data as an interactive TUI with list, tree, kanban board, and dependency graph views, a detail panel with Markdown rendering, and inline editing.

The UI takes heavy inspiration from [k9s](https://k9scli.io/) (the Kubernetes CLI), borrowing its project picker header, keyboard-driven navigation, and information-dense terminal layout.

Originally forked from [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), B9s has been **stripped to its core**: the TUI viewer. Added features include a full-fledged treeview, a k9s-style project picker, and editing capabilities. The upstream project's graph analysis engine, robot protocol, export wizards, semantic search, drift detection, recipe system, and other advanced features have been removed to keep the tool small, fast, and focused on the primary use case: reading and updating issues from the terminal.

### Why strip it down?

The upstream beads_viewer is an impressive piece of software with a graph analysis engine (PageRank, betweenness, HITS, critical path), AI agent protocols, static site export, time-travel diffs, sprint analytics, and more. That breadth is its strength, but it also means ~90k lines of Go source code, ~108k lines of tests, heavy dependencies like `gonum`, and complexity that isn't needed if all you want is a terminal viewer/editor.

B9s takes the opposite approach: **do fewer things well**. By stripping the codebase down to ~27k lines of source and ~26k lines of tests, and removing heavy vendor dependencies like `gonum`, B9s starts faster, compiles faster, and is easier to understand, maintain, and contribute to.

## Features

- **Tree view** with parent/child hierarchy, computed `◈N` open-blocker indicators, split-pane detail, search with occurrence filtering, bookmarking, and XRay drill-down. Press `g` on a row to inspect the blocker identities. Created-date sorting orders top-level items by date and descendants within epics by ascending natural title (1, 2, 3, …, 10), including numbered title prefixes and nested epics.
- **Global fuzzy search** across issue IDs, titles, and labels, shared by tree, list, and board
- **List view** with sorting (created, priority, updated) and status/label filtering
- **Kanban board** with three swimlane modes: by status, by priority, and by type
- **Dependency graph** with a focused view of what blocks the selected issue and what waits on it
- **Detail panel** with full Markdown rendering (via Glamour), scrollable and toggleable
- **Project picker** (k9s-style header) listing the current and recently opened projects on number keys 1-9, with issue count columns (Open, In Progress, Ready)
- **Inline editing** of title, status, priority, type, assignee, labels, description, and notes (via huh forms)
- **Issue creation** directly from the TUI (`Ctrl+n`)
- **Label filtering** with count display
- **Live reload** for JSONL file changes and Dolt working-set changes, with `Ctrl+R` / `F5` manual refresh
- **Self-updating** (`--update`, `--check-update`, `--rollback`)
- **Repository prefix filtering** (`--repo`)
- **Large dataset handling** with tiered loading and issue pooling for 1k-20k+ issues
- **Interactive tutorial** (`` ` `` backtick) for guided feature walkthrough

### Relationship to the original

Full credit goes to [@Dicklesworthstone](https://github.com/Dicklesworthstone) for the original architecture and implementation of [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer). The Bubbletea model structure, the background worker pattern, the file watcher integration, and the foundational UI components are all his work. B9s simply removes the features we don't use and makes different UX choices where our workflows diverge.

Per the upstream project's [contribution guidelines](https://github.com/Dicklesworthstone/beads_viewer/blob/main/CONTRIBUTING.md), beads_viewer does not accept external pull requests. B9s exists as a separate fork for users who want a leaner tool and the ability to contribute.

## Installation

### Homebrew (macOS/Linux)

```bash
brew install vanderheijden86/tap/b9s
```

### From source

Requires [Go 1.22+](https://go.dev/dl/).

```bash
git clone https://github.com/vanderheijden86/b9s.git
cd b9s
make install
```

This installs the `b9s` binary to your `$GOPATH/bin`. Make sure that directory is on your `PATH`.

For best display, use a terminal with a [Nerd Font](https://www.nerdfonts.com/).

## Quick Start

Navigate to any project initialized with `bd init` and run:

```bash
b9s
```

Press `?` for keyboard shortcuts or `` ` `` (backtick) for the interactive tutorial.

## Data Backends

B9s discovers and reads from multiple data backends automatically. On startup, it scans the `.beads/` directory for all available sources and selects the most authoritative one based on a fixed priority order.

The project picker lists the project b9s was started in and the projects you opened recently (`recent_projects` in `~/.config/b9s/config.yaml`, at most nine). It does not scan folders for other projects. Any supported backend can be a recent project. A server-mode project only needs valid Dolt configuration in `.beads/metadata.json`; it does not need a JSONL export. A recent project whose server or tunnel is unavailable stays in the picker and is marked `✗` in place of its issue counts.

### Source Priority

When multiple backends are present, B9s picks the highest-priority source:

| Priority | Backend | Source | Description |
|----------|---------|--------|-------------|
| **110** | Dolt (server mode) | `metadata.json` with `dolt_mode: "server"` | MySQL-compatible Dolt database via TCP. Supports concurrent writers and live change detection via `DOLT_HASHOF_DB()` working-set polling. |
| **100** | SQLite | `.beads/beads.db` | Legacy SQLite database from older `bd` versions. Read-only in B9s. |
| **80** | JSONL (worktree) | `.beads/issues.jsonl` in git worktrees | JSONL files discovered in linked git worktrees. |
| **50** | JSONL (local) | `.beads/issues.jsonl` | Flat-file JSONL. The original Beads storage format. |

Selection is **priority-first**, not freshness-first. When Dolt is configured, it always wins over JSONL, even if the JSONL file was modified more recently. This is correct because the Dolt database is the authoritative source when configured.

### Dolt Server Mode

The primary backend. Connects to a Dolt SQL server (self-hosted or [DoltHub](https://www.dolthub.com/)) via the MySQL wire protocol. Supports concurrent multi-agent access, push/pull replication, and live reload.

B9s polls the Dolt working-set hash, so uncommitted issue changes made by `bd` or another agent appear automatically. This deliberately avoids a database change stream or WebSocket layer. The default interval is 500 milliseconds and can be changed in `~/.config/b9s/config.yaml`:

```yaml
refresh:
  poll_interval: 2s
```

The minimum interval is 100 milliseconds. Restart B9s after changing the setting. Press `Ctrl+R` or `F5` at any time to refresh immediately.

### Tree Sort

The tree starts sorted newest first by creation date. Set a different default in `~/.config/b9s/config.yaml`:

```yaml
ui:
  sort:
    field: updated   # priority, created, updated, title, status, type, deps, pagerank
    direction: desc  # asc or desc; omit to use the field's natural direction
```

Press `s` to pick another sort for the current session. That choice survives live refreshes but is never saved, so every start returns to the configured sort. An unknown field or direction makes the config fail to load.

```bash
# Initialize a new project with a remote Dolt server
bd init --server \
  --server-host=your-server.example.com \
  --server-port=3306 \
  --server-user=root \
  --database=myproject

# Import existing JSONL issues into a new Dolt database
bd init --from-jsonl --server \
  --server-host=your-server.example.com \
  --server-port=3306 \
  --server-user=root \
  --database=myproject
```

Set the password via environment variable (never stored in config files):

```bash
export BEADS_DOLT_PASSWORD="your-password"
```

**Configuration** lives in `.beads/metadata.json` (written by `bd init`):

```json
{
  "dolt_mode": "server",
  "dolt_server_host": "your-server.example.com",
  "dolt_server_user": "root",
  "dolt_database": "myproject"
}
```

**Environment variables** override config values:

| Variable | Purpose |
|----------|---------|
| `BEADS_DOLT_PASSWORD` | Server password (never in config files) |
| `BEADS_DOLT_SERVER_HOST` | Dolt server hostname |
| `BEADS_DOLT_SERVER_PORT` | Dolt server port |
| `BEADS_DOLT_SERVER_USER` | MySQL user for Dolt server |

**Remote sync** (push/pull replication):

```bash
bd dolt remote add origin https://dolthub.com/user/database
bd dolt push                 # Push local commits to remote
bd dolt pull                 # Pull remote commits
bd dolt show                 # Show connection status and details
```

### Embedded Dolt Mode

When you run `bd init` without `--server`, Beads creates a local embedded Dolt engine inside `.beads/dolt/`. No external server is needed.

```bash
bd init
```

B9s cannot read from embedded Dolt directly (there is no TCP socket to connect to). Use server mode for B9s integration.

### JSONL and SQLite (Legacy)

B9s reads `.beads/issues.jsonl` and `.beads/beads.db` for backward compatibility with older `bd` versions. These are read-only fallbacks; all writes go through `bd` CLI regardless of backend.

### Fallback Behavior

If B9s cannot connect to the configured Dolt server, it falls back to the next available source (SQLite, then JSONL). Press `Shift+D` in the TUI to open the health popup, which shows the active datasource and any connection failure details.

## Keyboard Quick Reference

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j` / `k` | Next / Previous | `q` / `Esc` | Quit / Back |
| `Home` / `G` | Top / Bottom | `Tab` | Switch pane focus |
| `/` | Fuzzy search | `s` | Cycle sort mode |
| `n` / `N` | Next / Prev match | `l` | Label picker |
| `o` / `c` / `r` / `a` | Filter: Open / Closed / Ready / All | `d` | Toggle detail panel |
| `Ctrl+R` / `F5` | Refresh data immediately | `?` | Show help |

The mouse wheel moves through tasks and scrolls the detail pane. To select terminal text while mouse reporting is active, hold the terminal's override while dragging (`Option` in iTerm2, commonly `Shift` elsewhere).

| Key | Action |
|-----|--------|
| `b` | Kanban board |
| `g` | Dependency graph |
| `E` | Tree view |
| `e` | Edit issue |
| `Ctrl+n` | Create new issue |
| `Shift+K` | Close selected issue after confirmation |
| `Delete` | Permanently delete selected issue after confirmation |
| `?` | Keyboard shortcuts help |
| `[` / `]` | Resize split pane |

## Acknowledgments

- **Steve Yegge** for the vision behind [Beads](https://github.com/steveyegge/beads), a refreshingly simple approach to issue tracking that respects developers' workflows.
- **[@Dicklesworthstone](https://github.com/Dicklesworthstone)** for the original [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), whose architecture and implementation form the foundation of this project.
- **[k9s](https://k9scli.io/)** for the UI inspiration: the header-style project picker, keyboard-first navigation, and information-dense terminal layout.
- The **[Charm](https://charm.sh)** team for [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), [Huh](https://github.com/charmbracelet/huh), and [Glamour](https://github.com/charmbracelet/glamour), the terminal UI libraries that make building beautiful CLI tools a joy.

## License

MIT License. See [LICENSE](LICENSE) for details.
