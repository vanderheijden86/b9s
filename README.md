# B9s

![Go Version](https://img.shields.io/github/go-mod/go-version/vanderheijden86/b9s?style=for-the-badge&color=6272a4)
![License](https://img.shields.io/badge/License-MIT-50fa7b?style=for-the-badge)

> A fast, focused TUI viewer and editor for [Beads](https://github.com/steveyegge/beads) issue tracking projects. Inspired by [k9s](https://k9scli.io/).

## What is this?

B9s is a terminal-based interface for browsing, editing, and managing issues stored in `.beads/issues.jsonl`. It renders your issue data as an interactive TUI with list, tree, and kanban board views, a detail panel with Markdown rendering, and inline editing.

The UI takes heavy inspiration from [k9s](https://k9scli.io/) (the Kubernetes CLI), borrowing its project picker header, keyboard-driven navigation, and information-dense terminal layout.

Originally forked from [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), B9s has been **stripped to its core**: the TUI viewer. Added features include a full-fledged treeview, a k9s-style project picker, and editing capabilities. The upstream project's graph analysis engine, robot protocol, export wizards, semantic search, drift detection, recipe system, and other advanced features have been removed to keep the tool small, fast, and focused on the primary use case: reading and updating issues from the terminal.

### Why strip it down?

The upstream beads_viewer is an impressive piece of software with a graph analysis engine (PageRank, betweenness, HITS, critical path), AI agent protocols, static site export, time-travel diffs, sprint analytics, and more. That breadth is its strength, but it also means ~90k lines of Go source code, ~108k lines of tests, heavy dependencies like `gonum`, and complexity that isn't needed if all you want is a terminal viewer/editor.

B9s takes the opposite approach: **do fewer things well**. By stripping the codebase down to ~27k lines of source and ~26k lines of tests, and removing heavy vendor dependencies like `gonum`, B9s starts faster, compiles faster, and is easier to understand, maintain, and contribute to.

## Features

- **Tree view** with parent/child hierarchy, split-pane detail, search with occurrence filtering, bookmarking, and XRay drill-down
- **List view** with fuzzy search, sorting (created, priority, updated), and status/label filtering
- **Kanban board** with three swimlane modes: by status, by priority, and by type
- **Detail panel** with full Markdown rendering (via Glamour), scrollable and toggleable
- **Project picker** (k9s-style header) with multi-project switching, favorites (1-9 keys), and issue count columns (Open, In Progress, Ready)
- **Inline editing** of title, status, priority, type, assignee, labels, description, and notes (via huh forms)
- **Issue creation** directly from the TUI (`Ctrl+n`)
- **Label filtering** with count display
- **Live reload** on file changes (filesystem watcher with debounce + optional background snapshot loading)
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

## Dolt Backend

B9s supports two data backends for reading issues: **JSONL** (flat file) and **Dolt** (versioned SQL database). The Dolt backend is powered by `bd` (the Beads CLI), which can run Dolt in two modes.

### Embedded Mode (Default)

When you run `bd init`, Beads creates a local embedded Dolt engine inside `.beads/embeddeddolt/`. No external server is needed. This is the default for new projects.

```bash
bd init
```

Data lives entirely on disk. The embedded engine runs in-process, supports single-writer access, and requires no network configuration. B9s reads from this local database automatically.

**Limitations:** No `bd dolt push/pull`, no concurrent writers, no remote sync.

### Remote Server Mode

Remote mode connects to an external Dolt SQL server (self-hosted or [DoltHub](https://www.dolthub.com/)). This enables multi-user access, `push`/`pull` replication, and concurrent writes.

```bash
bd init --server \
  --server-host=osen.co \
  --server-port=3306 \
  --server-user=root \
  --database=myproject
```

To import existing JSONL issues into a new remote database:

```bash
bd init --from-jsonl --server \
  --server-host=osen.co \
  --server-port=3306 \
  --server-user=root \
  --database=myproject
```

Set the password via environment variable (never stored in config files):

```bash
export BEADS_DOLT_PASSWORD="your-password"
```

### How Mode Detection Works

Beads determines the mode from these sources (highest priority first):

1. **Environment variables**: `BEADS_DOLT_SERVER_MODE=1` forces remote mode
2. **`.beads/metadata.json`**: `"dolt_mode": "server"` indicates remote mode
3. **`.beads/config.yaml`**: presence of `dolt.host` indicates remote mode
4. **Default**: embedded mode

### Configuration Reference

**`.beads/config.yaml`** (team defaults, checked into git):

```yaml
dolt.host: "osen.co"
dolt.port: 3306
dolt.user: "root"
dolt.database: "myproject"
```

**Environment variables** (override config, useful for CI or per-user settings):

| Variable | Purpose |
|----------|---------|
| `BEADS_DOLT_SERVER_MODE` | Set to `1` to force remote server mode |
| `BEADS_DOLT_SERVER_HOST` | Dolt server hostname |
| `BEADS_DOLT_SERVER_PORT` | Dolt server port |
| `BEADS_DOLT_SERVER_USER` | MySQL user for Dolt server |
| `BEADS_DOLT_PASSWORD` | Server password (never in config files) |

### Remote Sync (Push/Pull)

In remote mode, Beads supports Dolt-native version control:

```bash
bd dolt remote list          # List configured remotes
bd dolt remote add origin https://dolthub.com/user/database
bd dolt push                 # Push local commits to remote
bd dolt pull                 # Pull remote commits
bd dolt commit               # Create a Dolt commit from pending changes
bd dolt show                 # Show connection status and details
```

These commands are only available in remote server mode. In embedded mode, data is local-only.

### Switching Between Modes

Use `bd backup` to safely migrate data between modes:

```bash
# Embedded -> Remote
bd backup init /tmp/beads-backup
bd backup sync
bd init --force --server --server-host=osen.co --server-port=3306 --database=myproject
bd backup restore --force /tmp/beads-backup

# Remote -> Embedded
bd backup init /tmp/beads-backup
bd backup sync
bd init --force
bd backup restore --force /tmp/beads-backup
```

### Embedded vs Remote at a Glance

| | Embedded | Remote Server |
|--|----------|---------------|
| **Setup** | `bd init` | `bd init --server` |
| **Data location** | `.beads/embeddeddolt/` | Remote Dolt server |
| **Concurrent writers** | No (file-locked) | Yes |
| **Push/pull sync** | No | Yes |
| **Network required** | No | Yes |
| **Use case** | Personal, single machine | Teams, shared infra |

### Fallback Behavior in B9s

If b9s cannot connect to the configured Dolt server, it falls back to reading from `.beads/issues.jsonl`. Press `Shift+D` in the TUI to open the health popup, which shows the active datasource and any connection failure details.

## Keyboard Quick Reference

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j` / `k` | Next / Previous | `q` / `Esc` | Quit / Back |
| `g` / `G` | Top / Bottom | `Tab` | Switch pane focus |
| `/` | Fuzzy search | `s` | Cycle sort mode |
| `n` / `N` | Next / Prev match | `l` | Label picker |
| `o` / `c` / `r` / `a` | Filter: Open / Closed / Ready / All | `d` | Toggle detail panel |

| Key | Action |
|-----|--------|
| `b` | Kanban board |
| `E` | Tree view |
| `e` | Edit issue |
| `Ctrl+n` | Create new issue |
| `?` | Keyboard shortcuts help |
| `[` / `]` | Resize split pane |

## Acknowledgments

- **Steve Yegge** for the vision behind [Beads](https://github.com/steveyegge/beads), a refreshingly simple approach to issue tracking that respects developers' workflows.
- **[@Dicklesworthstone](https://github.com/Dicklesworthstone)** for the original [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), whose architecture and implementation form the foundation of this project.
- **[k9s](https://k9scli.io/)** for the UI inspiration: the header-style project picker, keyboard-first navigation, and information-dense terminal layout.
- The **[Charm](https://charm.sh)** team for [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), [Huh](https://github.com/charmbracelet/huh), and [Glamour](https://github.com/charmbracelet/glamour), the terminal UI libraries that make building beautiful CLI tools a joy.

## License

MIT License. See [LICENSE](LICENSE) for details.
