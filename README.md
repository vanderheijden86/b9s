# Beads Viewer (bv)

A polished, high-performance TUI for managing and exploring [Beads](https://github.com/steveyegge/beads) issue trackers.

## Features

### 🖥️ Visual Dashboard
*   **Kanban Board**: Press `b` to toggle a 4-column Kanban board (Open, In Progress, Blocked, Closed).
*   **Adaptive Split-View**: Automatically transitions to a master-detail dashboard on wide screens.
*   **Rich Visualization**: Markdown rendering, syntax highlighting, and emoji status icons.

### ⚡ Workflow
*   **Instant Filtering**: `o` (Open), `r` (Ready), `c` (Closed), `a` (All).
*   **Markdown Export**: Generate comprehensive reports with dependency graphs using `bv --export-md report.md`.
*   **Keyboard Centric**: `vim` style navigation (`j`/`k`), `h`/`l` for board columns.

### 🛠️ Robustness
*   **Self-Updating**: Automatically notifies you of new releases.
*   **Reliable**: Handles complex data gracefully.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/Dicklesworthstone/beads_viewer/main/install.sh | bash
```

## Usage

```bash
bv
```

### Controls

| Key | Context | Action |
| :--- | :--- | :--- |
| `b` | Global | Toggle **Kanban Board** / List View |
| `Tab` | Split View | Switch focus between List and Details |
| `h`/`j`/`k`/`l`| Board | Navigate columns (h/l) and items (j/k) |
| `Enter` | List/Board| Open/Focus details |
| `o` / `r` / `c` | Global | Filter status |
| `q` | Global | Quit |

## CI/CD

*   **CI**: Runs tests on every push.
*   **Release**: Builds binaries for all platforms.

## License

MIT