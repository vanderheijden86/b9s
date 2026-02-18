# TUI Design Best Practices Research (2025-2026)

> Research conducted: 2026-02-18
> Task: bd-q5z.2 (child of bd-q5z: Epic: k9s-inspired UX redesign for beadwork)

---

## Table of Contents

1. [Navigation Patterns](#1-navigation-patterns)
2. [Notable TUI Applications and UX Innovations](#2-notable-tui-applications-and-ux-innovations)
3. [Information Density Best Practices](#3-information-density-best-practices)
4. [Keyboard-Driven UX Patterns](#4-keyboard-driven-ux-patterns)
5. [Project/Context Switching Patterns](#5-projectcontext-switching-patterns)
6. [Go + Bubbletea Specific Patterns](#6-go--bubbletea-specific-patterns)
7. [Recommendations for Beadwork](#7-recommendations-for-beadwork)

---

## 1. Navigation Patterns

### 1.1 Multi-Level Navigation Approaches

Modern TUIs handle multi-level navigation through three primary paradigms:

**Command-Driven Navigation (k9s pattern)**
Users type `:resource-name` to switch between views entirely. This is fast for experienced users and supports auto-completion and alias recognition. k9s accepts kubectl resource abbreviations (`:svc`, `:deploy`, `:pod`), making it immediately familiar to its target audience. The colon prefix mirrors Vim's command mode, creating a transferable mental model.

**Drill-Down Navigation (ranger/lf pattern)**
Miller columns display parent, current, and child/preview simultaneously. Navigation uses directional keys: left to go up, right to go deeper, up/down to select. This provides excellent spatial orientation since the user always sees where they came from and where they can go. ranger uses a 1:3:4 column ratio by default (parent:current:preview).

**Panel-Based Navigation (lazygit pattern)**
Multiple panels are always visible, with focus moving between them. lazygit shows 5 panels simultaneously (status, files, branches, commits, stash) with number keys (1-5) or arrow keys to switch focus. Panels use tabs within them (navigable with `[` and `]`). The key insight is that "most views are generally visible always, no matter what operation you're doing."

### 1.2 Tab-Based vs. Pane-Based vs. Modal Navigation

| Pattern | Best For | Examples | Trade-offs |
|---------|----------|----------|------------|
| **Tab-based** | Parallel views of same data | btop (CPU/Mem/Disk tabs), htop (F5 tree toggle) | Simple mental model but limited to flat hierarchies |
| **Pane-based** | Related but independent data | lazygit (files + branches + commits), superfile (side-by-side folders) | Shows more context but requires clear focus indicators |
| **Modal** | Focused tasks within a workflow | k9s (resource views are modal screens), huh forms | Full screen for task but loses context of parent view |
| **Hybrid** | Complex applications | k9s (modal views + overlay dialogs + command mode) | Most flexible but highest learning curve |

### 1.3 Breadcrumb Patterns in Terminal Apps

TUI breadcrumbs differ significantly from web breadcrumbs. Key findings:

- **k9s**: Displays the current context path in the header: `context > cluster > namespace > resource-type`. Recently visited namespaces are accessible through number keys 0-9, functioning as a "breadcrumb shortcut bar."
- **ranger**: Uses Miller columns as a visual breadcrumb. The left column IS the breadcrumb, always showing the parent directory with the current directory highlighted.
- **File managers** (Nemo, Windows Explorer): Implement clickable path segments in address bars, which translates to TUI as typed path segments.

**Recommended TUI breadcrumb pattern**: A header line showing `context > view > filter` with each segment reflecting a navigational choice the user made. Segments should be keyboard-accessible (e.g., Backspace to go up one level).

### 1.4 Quick-Switch Shortcuts

- **Number keys for tabs/favorites**: k9s uses 0-9 for namespace favorites; lazygit uses 1-5 for panel switching
- **Colon-command mode**: k9s uses `:resource` for instant view switching
- **Slash filtering**: Nearly universal: `/pattern` to filter current view (k9s, lazygit, htop)
- **Direct key mapping**: btop maps sections to letter keys (`p` for processes, `m` for memory)

---

## 2. Notable TUI Applications and UX Innovations

### 2.1 k9s (Kubernetes Management)

**Repository**: github.com/derailed/k9s | **Language**: Go (custom TUI, not bubbletea)

Key UX innovations:
- **Command mode (`:`)**: Type resource names to instantly switch views. Supports auto-completion and kubectl aliases.
- **Namespace favorites**: Number keys 0-9 bound to recently visited namespaces. Press `u` to mark a namespace as favorite.
- **Resource drill-down**: Press Enter on a resource to see its children (deployment -> pods). Creates a navigation stack.
- **Contextual shortcuts**: Available shortcuts are displayed in the top-right corner of every resource view. Shortcuts change based on the current resource type.
- **XRay and Pulses views**: Overview displays that show related resources across the cluster, not limited to a single resource type.
- **Label filtering**: `/`-l app=web` filters by labels; `/!pattern` inverts filters. Filters stack for progressive refinement.
- **YAML/Describe/Logs**: Single-key actions (`y`, `d`, `l`) on any selected resource.

**Key takeaway for beadwork**: The command mode pattern (`:resource`) is extremely powerful for applications with many distinct views. It provides discoverability through auto-complete while rewarding muscle memory.

### 2.2 lazygit (Git Management)

**Repository**: github.com/jesseduffield/lazygit | **Language**: Go (custom TUI, gocui-based)

Key UX innovations:
- **Persistent multi-panel layout**: All panels visible simultaneously. Focus indicated by highlight color. No views are hidden during normal operation.
- **Command transparency**: A command log shows every git command being executed, building user trust and serving as a learning tool. Jesse Duffield notes this became "one of the things people like best about Lazygit."
- **Vim-consistent keybindings**: `q` quits, `h/j/k/l` navigate, `/` filters, `y` copies. Action keys follow command names (`c` commits, `f` fetches, `p` pulls, `P` pushes, `r` rebases).
- **State -> Action -> New State feedback**: After committing, users immediately see staged files disappear, new commits appear in the log, and branch heads update. This direct visual feedback is a core advantage over CLI workflows.
- **Progressive zoom**: Users can zoom into a panel for full-screen view when needed, then zoom back out.

**Key takeaway for beadwork**: The "transparent operations" pattern (showing what commands are being run) builds trust. For beadwork, showing what changes are being made to `.beads/issues.jsonl` would provide similar confidence.

### 2.3 btop/htop (System Monitoring)

**btop**: github.com/aristocratos/btop | **Language**: C++
**htop**: github.com/htop-dev/htop | **Language**: C

Key UX innovations:
- **Dashboard layout**: Multiple metrics displayed simultaneously in a grid (CPU, memory, disk, network, processes).
- **Function key menu bar**: htop uses F1-F10 along the bottom as a permanent shortcut reference. Each key is labeled with its action.
- **Section navigation**: btop maps sections to keys (`Esc/m` for menu, `F2/o` for options). Arrow keys navigate within sections.
- **Sort toggling**: Press a key to cycle sort columns (P for CPU%, M for memory%, N for PID in htop).
- **Real-time visual graphs**: CPU/memory shown as live-updating charts, not just numbers.

**Key takeaway for beadwork**: The permanent function key bar at the bottom is an excellent discoverability pattern, immediately showing users what they can do without consulting documentation.

### 2.4 ranger/lf (File Management)

**ranger**: github.com/ranger/ranger | **Language**: Python
**lf**: github.com/gokcehan/lf | **Language**: Go

Key UX innovations:
- **Miller columns**: Three-column layout showing parent/current/preview with 1:3:4 ratio. Navigation is spatial and intuitive.
- **Preview pane**: Right column previews file contents, images (in supported terminals), or directory listings. This progressive disclosure shows more detail without requiring a modal.
- **Bookmarks**: Mark directories with `m` + letter, jump to them with `'` + letter. Provides O(1) navigation to frequent locations.
- **Tabs**: ranger supports multiple tabs with `gn` (new tab), `gt`/`gT` (switch tabs), allowing parallel browsing.
- **Filter and search**: `/` searches file names; `f` filters the current listing.

**Key takeaway for beadwork**: The bookmark pattern (`m` to mark, `'` to jump) maps well to issue management: users could bookmark frequently accessed issues or views.

### 2.5 Superfile (Modern File Manager)

**Repository**: github.com/yorukot/superfile | **Language**: Go (Bubble Tea)

Key UX innovations:
- **Modern visual design**: Uses Nerd Font icons, themed color schemes (Nord, Dracula), and clean panel borders.
- **Multi-panel file browsing**: Multiple panels side by side for viewing different folders simultaneously.
- **Clipboard viewer**: Dedicated panel showing clipboard contents (cut/copied files).
- **Process list**: Shows background operations in progress.
- **Plugin architecture**: Extensible through plugins.

**Key takeaway for beadwork**: Demonstrates that Bubble Tea can produce visually polished, modern-feeling TUIs. Themes and Nerd Font icons significantly improve the perceived quality.

### 2.6 Other Notable Go/Bubbletea TUIs

- **Circumflex**: Hacker News reader built with Bubble Tea
- **Discordo**: Discord TUI client
- **OpenCode**: AI coding assistant with TUI, uses Bubble Tea with a sophisticated component architecture (context providers, dialog system, overlay management)
- **pug** (leg100/pug): Terraform TUI built with Bubble Tea, notable for its nested model architecture documentation

---

## 3. Information Density Best Practices

### 3.1 Showing Contextual Info Without Overwhelming

The core principle is **progressive disclosure**: show only the most important information up front, with more detail available on demand.

**Levels of disclosure in TUI applications:**

| Level | What to Show | How to Access | Example |
|-------|-------------|---------------|---------|
| **L0: Always visible** | Resource name, status, key metric | Rendered in list/table | Issue title + status badge |
| **L1: On selection** | Details panel, preview | Highlight/cursor position | Issue description in split pane |
| **L2: On demand** | Full detail view, history | Enter/specific key | Full issue detail with all fields |
| **L3: Explicit action** | Edit forms, dangerous actions | Command or multi-key | Edit modal, delete confirmation |

**Critical finding**: Designs that go beyond 2 disclosure levels in a single interaction often suffer poor usability. Users get lost when navigating between too many levels. Chunk advanced features into logical groups.

### 3.2 Status Bars

**Header bar** (top of screen):
- Context information: current project, view name, active filters
- Connection/sync status indicators
- k9s pattern: `context > cluster > namespace > resource` breadcrumb

**Status bar** (bottom of screen):
- Context-sensitive shortcut hints (change based on current view/selection)
- Terminal.Gui pattern: `~F1~ Help ~F2~ Save ~F3~ Load` format
- Application status messages (loading, error, success)
- htop pattern: F-key actions as permanent reference

**Best practice**: Status bars should be context-sensitive. When a dialog is open, the status bar should show dialog-relevant shortcuts, not the main view shortcuts.

### 3.3 Sidebars

- **Detail sidebar** (lazygit pattern): Shows details of the selected item in a side or bottom panel. Always visible, content changes with selection.
- **Tree sidebar** (file manager pattern): Hierarchical navigation in a narrow left panel.
- **Collapsible sidebars**: Toggle with a key to reclaim screen space when not needed.

### 3.4 Information Density Guidelines

- **Avoid redundancy**: Duplicate information increases text density but not information density. Show each fact once.
- **Use visual encoding**: Colors, icons, and alignment convey information without using text. A red circle communicates "error" faster than the word "error."
- **Align data in columns**: Tabular data is much faster to scan than paragraph text.
- **Truncate with access**: Show truncated values in lists (e.g., first 40 chars of a title) with full values available on hover or selection.

---

## 4. Keyboard-Driven UX Patterns

### 4.1 Keybinding Philosophies

**Vim-style (hjkl)**
- Most popular in developer-oriented TUIs
- `h/j/k/l` for directional movement, `g/G` for jump-to-start/end
- Modal editing: normal mode, insert mode, command mode
- Used by: k9s, lazygit, ranger, lf, superfile, vifm
- Pro: Extremely efficient for experienced users; large existing user base
- Con: Steep learning curve for non-Vim users

**Arrow-key primary with Vim alternatives**
- Arrow keys as primary, with `h/j/k/l` as alternatives
- Used by: btop, htop, most modern TUIs
- Pro: Immediately usable by anyone; Vim users get their shortcuts too
- Con: Arrow keys require hand movement from home row

**Emacs-style (Ctrl+)**
- `Ctrl+n/p` for next/previous, `Ctrl+f/b` for forward/back
- Less common in modern TUIs; mainly seen in readline-based interfaces
- Used by: some CLI tools, zsh line editing mode

**Recommendation**: Offer both arrow keys and Vim bindings. Arrow keys for discoverability, Vim keys for efficiency. This is the pattern followed by nearly every successful modern TUI.

### 4.2 Discoverability Mechanisms

**Context-sensitive shortcut display (k9s pattern)**
- Available shortcuts shown in a header or footer area
- Shortcuts change based on the current view and selected resource type
- Most effective discoverability pattern: users see what they can do right now

**Permanent function key bar (htop pattern)**
- Bottom bar showing F1-F10 with labels: `F1Help F2Setup F3Search F4Filter F5Tree F6Sort`
- Always visible, never changes
- Best for applications with a fixed set of global actions

**Help overlay (`?` key)**
- Press `?` to show a full keybinding reference
- Should be scrollable for applications with many shortcuts
- Can be organized by category (navigation, actions, views)

**Command palette (fzf/Telescope pattern)**
- Fuzzy-searchable list of all available commands
- Triggered by a key combo (often `Ctrl+P` or `:`)
- Combines discoverability with speed: new users browse, experienced users type

**Inline hints (huh/form pattern)**
- Show available actions next to interactive elements
- Example: `[Enter] confirm [Esc] cancel [Tab] next field`
- Useful for modal dialogs and forms

### 4.3 Command Palettes in TUIs

The command palette pattern, popularized by VS Code, is increasingly appearing in TUIs:

- **fzf as a component**: Many TUIs embed fzf-style fuzzy finding for command/item selection
- **Centralized command registry**: Applications register commands with metadata (title, keybinding, category), then expose them through a searchable palette
- **Type-to-filter**: Users start typing and results narrow immediately
- **OpenCode pattern**: Commands are registered with the TUI and accessible through both direct keybindings and a searchable command palette

**Implementation approach for Bubble Tea**: Create a command registry that maps command names to handler functions, then build a fuzzy-search overlay that filters and displays these commands.

---

## 5. Project/Context Switching Patterns

### 5.1 How TUI Tools Handle Multiple Projects

**Namespace/Context selector (k9s pattern)**
- `:ctx` shows all available Kubernetes contexts (analogous to projects)
- `:ns` shows all namespaces within a context
- Number keys 0-9 map to favorites for instant switching
- Favorites persist across sessions

**Session/Workspace management (tmux/TUIOS pattern)**
- TUIOS implements 9 virtual workspaces, each with independent window sets
- Current workspace stored in state, with per-workspace focus tracking
- Switch workspaces with number keys
- Each workspace maintains its own layout and focus history

**Multi-repo/workspace (beadwork-relevant patterns)**
- OpenCode manages agent and model selection with per-agent preferences
- bv (beadwork viewer) could support `--workspace` flag for multi-repo contexts
- File managers (superfile) support multiple panels pointing to different directories

### 5.2 Quick-Select Patterns

| Pattern | How It Works | Speed | Discoverability |
|---------|-------------|-------|-----------------|
| **Number keys** | 0-9 mapped to favorites/recent | Instant (O(1)) | High (visible in header) |
| **Fuzzy search** | Type to filter from full list | Fast (type 2-3 chars) | Medium (needs activation key) |
| **Recent list** | MRU list shown on activation | Fast (scan + select) | High (shows actual options) |
| **Named bookmarks** | `m` + letter to mark, `'` + letter to jump | Instant (O(1)) | Low (must be learned) |
| **Colon-command** | `:name` with auto-complete | Fast (type + tab) | Medium (auto-complete helps) |

### 5.3 Recommended Pattern for Beadwork

Given beadwork's use case (managing issues across views), a hybrid approach works best:

1. **View switching**: Number keys or single letters (`l` list, `t` tree, `b` board, `g` graph, `s` split) for instant view switching
2. **Issue jumping**: `/` to fuzzy-search issues by title from any view
3. **Filter stacking**: `:filter status=open` or `:f priority=P1` for progressive refinement
4. **Breadcrumb display**: Header shows current state: `beadwork > tree view > P1 issues > (3 of 47)`

---

## 6. Go + Bubbletea Specific Patterns

### 6.1 Component Composition Patterns

**Embedding (Simple Composition)**
Parent model embeds child models as struct fields:
```go
type Model struct {
    list     list.Model
    viewport viewport.Model
    help     help.Model
}
```
Parent delegates messages to all embedded children in `Update()` and composes their `View()` outputs with Lipgloss.

**Model Stack (Complex Applications)**
For apps with multiple screens/views that should be independent:
- A controller manages a stack of models
- Models are completely self-contained; they don't know about siblings
- The controller intercepts all commands and manages navigation
- The `bubblon` library provides `Open()`, `Close()`, `Replace()`, and `ReplaceAll()` operations
- This eliminates complex session state management in parent models

**Root Screen Router (Multi-View Apps)**
A root model acts as a router:
- Holds the currently active view model
- All screen transitions flow through the root
- Child screens call `root.SwitchScreen(&newScreen)` to navigate
- Root calls `.Init()` on each new screen to ensure proper initialization
- Root state persists across transitions (global data), while child state resets per switch

### 6.2 State Management for Multi-View Apps

**State Machine Pattern**
For multi-step workflows:
- Define stages as structs with Action, Error, IsComplete, and Reset functions
- `runStage()` executes the current stage and returns a completion message
- `Update()` advances the stage index on success, halts on failure
- `IsCompleteFunc()` enables idempotent re-runs (skip already-done work)
- This pattern keeps the UI responsive during long-running operations

**Message Routing Rules**
Three categories of messages:
1. **Global messages** (quit, window resize, help): Handled at root level, then forwarded to all children
2. **Active view messages** (user input): Routed only to the currently focused view
3. **Structural messages** (`tea.WindowSizeMsg`): Broadcast to all children so they can recalculate layouts

**Key warning**: Messages from concurrent commands arrive in unpredictable order. Use `tea.Sequence()` when order matters, or design update logic to handle any order.

### 6.3 Responsive Layouts in Bubbletea

**Window Size Handling**
Bubble Tea sends `tea.WindowSizeMsg` on startup and every resize. Store these dimensions and use them in `View()`:
```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    // Propagate to children
    m.viewport.Width = msg.Width
    m.viewport.Height = msg.Height - headerHeight - footerHeight
```

**Dynamic Layout Calculation**
Avoid hard-coded dimension offsets. Instead, use Lipgloss to measure rendered content:
```go
headerHeight := lipgloss.Height(headerView)
footerHeight := lipgloss.Height(footerView)
contentHeight := m.height - headerHeight - footerHeight
```

**Adaptive Layouts**
Switch between compact and full layouts based on terminal size:
```go
if m.width < 80 {
    // Compact: stack vertically
} else {
    // Full: side-by-side panels
}
```

**BubbleLayout Library**
For complex layouts, `winder/bubblelayout` translates `tea.WindowSizeMsg` into per-component layout messages. Each component has a unique ID and receives its absolute dimensions through `bl.BubbleLayoutMsg`.

### 6.4 Testing Patterns

**Unit testing with teatest**
- Emulate key presses and verify program output
- Use golden files for visual regression testing

**VHS for documentation**
- Declarative scripts that produce GIFs and screenshots
- Useful for CI-based visual regression detection

**Message logging for debugging**
- Dump messages to a file using the `spew` library
- Tail the file in another terminal during development

### 6.5 Common Pitfalls

1. **Blocking in Update/View**: Never do expensive work in Update() or View(). Offload to tea.Cmd.
2. **Race conditions**: Never modify model state from goroutines outside Update(). All changes must flow through the message loop.
3. **Forgetting to propagate WindowSizeMsg**: All child models that render content need terminal dimensions.
4. **Not calling Init() on screen switch**: Spinner animations and async commands freeze if Init() is skipped when switching views.
5. **Hard-coded dimensions**: Break on different terminal sizes. Always calculate dynamically.

---

## 7. Recommendations for Beadwork

Based on this research, here are specific recommendations for the beadwork TUI:

### 7.1 Navigation Model

**Adopt a hybrid k9s + lazygit approach:**
- **Single-key view switching**: `1`-`5` or letter keys (`l/t/b/g/s`) for list/tree/board/graph/split views
- **Command mode**: `:` for advanced navigation (`:filter`, `:sort`, `:goto`)
- **Breadcrumb header**: Show `view > filter > selection` path at top
- **Detail panel**: Show selected issue details in a side or bottom panel (lazygit-style persistent panel)

### 7.2 Discoverability

**Three-tier help system:**
1. **Always visible**: Bottom status bar with context-sensitive shortcuts (changes per view)
2. **On demand**: `?` key shows full help overlay, scrollable, organized by view
3. **Deep reference**: `:help` command for searchable help

### 7.3 Information Architecture

**Apply progressive disclosure:**
- **L0 (list)**: ID, title, status badge, priority, labels (truncated)
- **L1 (selection)**: Description preview, dependencies count, timestamps in detail panel
- **L2 (detail view)**: Full description, all metadata, dependency graph, history
- **L3 (edit)**: huh-based edit forms

### 7.4 Component Architecture

**Use the Root Screen Router pattern:**
- Root model manages active view and global state (loaded issues, current filters)
- Each view (list, tree, board, graph, split) is an independent model
- Views receive filtered/sorted issue data from root; views handle their own rendering
- Screen switches go through root to ensure proper Init() calls

### 7.5 Keyboard Design

**Layer keybindings:**
- **Global** (work everywhere): `?` help, `q` quit, `1-5` view switch, `:` command mode, `/` search
- **View-specific**: Arrow/hjkl navigation, Enter for detail, `e` edit, `n` new
- **Context-specific**: Actions that only apply to the current selection type

### 7.6 Visual Polish

- Use Lipgloss for consistent styling
- Consider Nerd Font icon support (with graceful fallback)
- Implement theme support (light/dark at minimum)
- Use color meaningfully: status colors, priority indicators, focus highlighting

---

## Sources

### Articles and Blog Posts
- [Tips for building Bubble Tea programs](https://leg100.github.io/en/posts/building-bubbletea-programs/) - Comprehensive guide to Bubbletea patterns
- [Lazygit Turns 5: Musings on Git, TUIs, and Open Source](https://jesseduffield.com/Lazygit-5-Years-On/) - UX lessons from lazygit's development
- [Multi-View Interfaces in Bubble Tea](https://shi.foo/weblog/multi-view-interfaces-in-bubble-tea) - Root screen router pattern
- [Managing Nested Models with Bubble Tea](https://donderom.com/posts/managing-nested-models-with-bubble-tea/) - Model stack architecture
- [The Bubbletea State Machine Pattern](https://zackproser.com/blog/bubbletea-state-machine) - State machine pattern for complex workflows
- [The Complete K9s Cheatsheet](https://ahmedjama.com/blog/2025/09/the-complete-k9s-cheatsheet/) - K9s navigation patterns
- [The (lazy) Git UI You Didn't Know You Need](https://www.bwplotka.dev/2025/lazygit/) - Lazygit UX analysis
- [Beyond the GUI: Modern TUI Applications](https://www.blog.brightcoding.dev/2025/09/07/beyond-the-gui-the-ultimate-guide-to-modern-terminal-user-interface-applications-and-development-libraries/) - TUI landscape overview

### Framework Documentation
- [Bubbletea GitHub](https://github.com/charmbracelet/bubbletea) - Framework source and examples
- [Bubbles GitHub](https://github.com/charmbracelet/bubbles) - Component library
- [BubbleLayout](https://pkg.go.dev/github.com/winder/bubblelayout) - Layout management library
- [k9s Official Site](https://k9scli.io/) - k9s commands and documentation

### Application Repositories
- [k9s](https://github.com/derailed/k9s) - Kubernetes TUI
- [lazygit](https://github.com/jesseduffield/lazygit) - Git TUI
- [superfile](https://github.com/yorukot/superfile) - Modern file manager (Bubble Tea)
- [ranger](https://github.com/ranger/ranger) - Miller columns file manager
- [lf](https://github.com/gokcehan/lf) - Terminal file manager (Go)
- [awesome-tuis](https://github.com/rothgar/awesome-tuis) - Curated list of TUI projects

### Design References
- [Progressive Disclosure (NN/g)](https://www.nngroup.com/articles/progressive-disclosure/) - UX research on information layering
- [Information Density and Progressive Disclosure (Algolia)](https://www.algolia.com/blog/ux/information-density-and-progressive-disclosure-search-ux/) - Search UX patterns
- [Terminal.Gui Views](https://gui-cs.github.io/Terminal.Gui/docs/views.html) - Status bar and shortcut hint patterns
