# Beadwork UX Redesign: k9s-Inspired Design

> Design document for beadwork (bw) TUI redesign
> Created: 2026-02-18 | Epic: bd-q5z

## Vision

Transform beadwork from a single-project issue viewer into a polished, k9s-inspired multi-project TUI that feels fast, discoverable, and keyboard-native. The core insight from k9s is that complex data becomes manageable when you combine **instant context switching** (number keys), **command-driven navigation** (`:` command mode), and **progressive disclosure** (drill-down with breadcrumb trail).

## Rationale

Today, beadwork loads a single `.beads/issues.jsonl` and renders it. To switch projects, you quit and restart with `--path`. This is the equivalent of k9s without namespace switching: technically functional, but hostile to any workflow involving multiple projects.

Most developers work across 2-5 projects. A project manager or tech lead might track 10+. The project switching pattern from k9s (where namespaces map to our projects) solves this without adding complexity for single-project users.

---

## 1. Project Switching (Core Feature)

### Concept

Projects in beadwork = Namespaces in k9s.

Number keys `1-9` are bound to favorite projects. Key `0` shows "all projects" (aggregate view). Projects are directories containing `.beads/issues.jsonl`. Users register projects in a config file and mark favorites.

### Configuration

```yaml
# ~/.config/bw/config.yaml
projects:
  - name: beadwork
    path: ~/Documents/beadwork
  - name: api-server
    path: ~/work/api-server
  - name: frontend
    path: ~/work/frontend
  - name: infra
    path: ~/work/infrastructure
  - name: docs
    path: ~/work/documentation

favorites:     # Maps number keys 1-9 to project names
  1: beadwork
  2: api-server
  3: frontend
  4: infra
```

### Auto-Discovery

When launched without `--path`, bw scans common locations for `.beads/` directories:
1. Current directory and parents (walk up to home)
2. Registered projects from config
3. Recently opened projects (persisted in state)

### UX Flow

```
Press 1-9     -> Instant switch to favorite project
Press 0       -> Aggregate view (all projects)
Type :project -> Open project picker (fuzzy search)
Type :p name  -> Switch to project by name
Press P       -> Toggle project picker overlay
```

### State Preservation

When switching projects:
- Current view mode persists (if you're in board view, you stay in board view)
- Filter state resets (filters are project-specific)
- Scroll position resets
- The previous project's state is cached for instant switch-back

---

## 2. Navigation Redesign

### Current State

Today: `E` toggles tree, `b` toggles board, `tab` cycles focus, `enter` opens detail. No command mode. No breadcrumbs.

### Proposed Navigation Model

**Three layers, inspired by k9s + lazygit:**

| Layer | Mechanism | Example |
|-------|-----------|---------|
| **Instant switch** | Number keys 1-9 | `2` switches to api-server project |
| **View switch** | Letter keys | `l` list, `t` tree, `b` board, `d` detail |
| **Command mode** | `:` prefix | `:filter status=open`, `:sort priority` |

### View Keys

| Key | View | Description |
|-----|------|-------------|
| `l` | List | Column-based issue list (current default) |
| `t` | Tree | Hierarchical epic/feature/task tree |
| `b` | Board | Kanban board by status |
| `d` | Detail | Full issue detail (selected issue) |
| `s` | Split | List + detail side-by-side |

### Command Mode

Pressing `:` activates the command input at the bottom of the screen with fish-shell-style inline suggestions:

```
:filter status=open priority<3
:sort updated desc
:project frontend
:goto bd-q5z
:label add bd-q5z "urgent"
```

Auto-complete suggests commands as you type. `Tab` accepts the suggestion. `Esc` cancels.

### Filter Mode

Pressing `/` activates filter mode (same as today, but improved):

```
/authentication          -> Fuzzy match on title
/status:open             -> Structured filter
/priority:1,2            -> Multi-value filter
/!blocked                -> Invert filter
```

---

## 3. Header and Footer Redesign

### Current State

Today: minimal header with column labels. No breadcrumb. No project indicator. Footer shows basic stats.

### Proposed Header

```
 bw | beadwork (1)         tree view | open:12 ready:5 blocked:3 | /auth filter
     ^^^^^^^^ ^^^          ^^^^^^^^^   ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^  ^^^^^^^^^^^^
     project  favorite#    active view issue counts                    active filter
```

The header is a single line containing:
- **App name** (`bw`) - always visible, left-aligned
- **Project name** with favorite number in parens
- **Active view** name
- **Issue counts** - context-sensitive summary
- **Active filter** - shows what's currently filtered

### Proposed Footer

Context-sensitive shortcut hints that change based on the current view:

**List view footer:**
```
 1-9:project  l:list  t:tree  b:board  /:filter  ?:help  e:edit  n:new  q:quit
```

**Tree view footer:**
```
 1-9:project  h/l:collapse/expand  j/k:navigate  enter:detail  e:edit  q:back
```

**Board view footer:**
```
 1-9:project  h/l:column  j/k:card  m:move  e:edit  /:filter  q:back
```

**Command mode footer:**
```
 tab:complete  enter:run  esc:cancel  ctrl+u:clear
```

### Breadcrumb Trail

Below the header, a breadcrumb shows navigation depth (only when depth > 1):

```
 beadwork > tree view > Epic: k9s redesign > bd-q5z.1
```

Pressing `Esc` or `Backspace` navigates up one level.

---

## 4. Project Picker

When pressing `P` or typing `:project`, an overlay appears:

```
 Select Project                                    [esc to close]
 ─────────────────────────────────────────────────────────────────
  #  Project          Path                    Open  Ready  Blocked
 ─────────────────────────────────────────────────────────────────
  1  beadwork      *  ~/Documents/beadwork       12     5       3
  2  api-server       ~/work/api-server          24     8       6
  3  frontend         ~/work/frontend             8     3       1
  4  infra            ~/work/infrastructure      15     4       2
  -  docs             ~/work/documentation        3     1       0
  -  mobile-app       ~/work/mobile               6     2       1
 ─────────────────────────────────────────────────────────────────
  * = current project | u:set favorite | enter:switch | /:search
```

Features:
- Shows issue counts per project (lazy-loaded on open)
- `*` marks current project
- Number shows favorite slot (1-9)
- `u` marks/unmarks as favorite (like k9s namespace favorites)
- `/` to fuzzy filter projects
- `enter` to switch
- Stats update as you navigate

---

## 5. TUI Mockups

### Mockup A: List View (Default)

```
 bw | beadwork (1)          list view | open:12 ready:5 blocked:3

  TYPE     PRI  STATUS       ID          TITLE
  ────     ───  ──────       ──          ─────
  epic     P1   in_progress  bd-q5z      Epic: k9s-inspired UX redesign
  task     P1   open         bd-qal      Set HOMEBREW_TAP_GITHUB_TOKEN secret
  task     P2   open         bd-q5z.1    Research k9s UX patterns
  feature  P3   open         bd-n0wb     Migrate to cobra/pflag
  feature  P4   open         bd-36s      Agenda view: time-horizon grouping
  bug      P2   open         bd-a1b      Fix tree view collapse on resize
  task     P2   in_progress  bd-c3d      Add project switching config
  task     P3   open         bd-e5f      Implement command mode
  feature  P2   open         bd-g7h      Theme support (light/dark)
  task     P3   open         bd-i9j      Breadcrumb navigation
  bug      P1   open         bd-k2l      Board view card overflow
  task     P4   open         bd-m4n      Export to markdown

  page 1/1 (12 issues)

 1-9:project  t:tree  b:board  s:split  /:filter  ?:help  e:edit  n:new  q:quit
```

### Mockup B: Tree View

```
 bw | beadwork (1)          tree view | open:12 ready:5 blocked:3

  Epic: k9s-inspired UX redesign ────────────────── [P1] in_progress  bd-q5z
  ├─ Research k9s UX patterns ──────────────────── [P2] open          bd-q5z.1
  ├─ Add project switching config ──────────────── [P2] in_progress   bd-c3d
  ├─ Implement command mode ────────────────────── [P3] open          bd-e5f
  ├─ Breadcrumb navigation ─────────────────────── [P3] open          bd-i9j
  └─ Theme support (light/dark) ────────────────── [P2] open          bd-g7h

  Migrate to cobra/pflag ──────────────────────── [P3] open          bd-n0wb

  (standalone)
  ├─ Set HOMEBREW_TAP_GITHUB_TOKEN secret ──────── [P1] open          bd-qal
  ├─ Fix tree view collapse on resize ──────────── [P2] open          bd-a1b
  ├─ Board view card overflow ──────────────────── [P1] open          bd-k2l
  ├─ Agenda view: time-horizon grouping ────────── [P4] open          bd-36s
  └─ Export to markdown ────────────────────────── [P4] open          bd-m4n


 1-9:project  h/l:collapse/expand  j/k:nav  enter:detail  e:edit  q:back
```

### Mockup C: Board View (Kanban)

```
 bw | beadwork (1)         board view | open:12 ready:5 blocked:3

  OPEN (5)              IN_PROGRESS (2)       BLOCKED (3)            CLOSED (2)
  ──────────────────    ──────────────────    ──────────────────    ──────────────────
  [P1] Set HOMEBREW..   [P1] Epic: k9s..     [P2] bd-q5z.1         [P2] bd-xyz
  [P2] Fix tree view    [P2] Add project..   [P3] Implement cmd     [P3] bd-abc
  [P3] Migrate cobra                         [P1] Board overflow
  [P2] Theme support
  [P4] Agenda view..



  ──────────────────    ──────────────────    ──────────────────    ──────────────────
  5 issues              2 issues              3 issues               2 issues

 1-9:project  h/l:column  j/k:card  m:move status  e:edit  /:filter  q:back
```

### Mockup D: Split View (List + Detail)

```
 bw | beadwork (1)         split view | open:12 ready:5 blocked:3

  TYPE     PRI  STATUS       ID       TITLE           │ bd-q5z: Epic: k9s-inspired UX redesign
  ────     ───  ──────       ──       ─────           │ ────────────────────────────────────────
  epic     P1   in_progress  bd-q5z > Epic: k9s..     │ Type: epic | Priority: P1
  task     P1   open         bd-qal   Set HOMEBR..    │ Status: in_progress
  task     P2   open         bd-q5z.1 Research k..    │ Assignee: andre
  feature  P3   open         bd-n0wb  Migrate to..    │ Created: 2026-02-18
  feature  P4   open         bd-36s   Agenda vie..    │ Updated: 2026-02-18 14:30
  bug      P2   open         bd-a1b   Fix tree v..    │
  task     P2   in_progress  bd-c3d   Add projec..    │ Description:
  task     P3   open         bd-e5f   Implement ..    │ Transform beadwork from a single-project
                                                      │ issue viewer into a polished, k9s-inspired
                                                      │ multi-project TUI...
                                                      │
                                                      │ Children (5):
                                                      │   bd-q5z.1  Research k9s UX patterns
                                                      │   bd-c3d    Add project switching config
                                                      │   bd-e5f    Implement command mode
                                                      │   bd-i9j    Breadcrumb navigation
                                                      │   bd-g7h    Theme support

 1-9:project  tab:focus  </>:resize  t:tree  b:board  e:edit  q:back
```

### Mockup E: Command Mode Active

```
 bw | beadwork (1)          list view | open:12 ready:5 blocked:3

  TYPE     PRI  STATUS       ID          TITLE
  ────     ───  ──────       ──          ─────
  epic     P1   in_progress  bd-q5z      Epic: k9s-inspired UX redesign
  task     P1   open         bd-qal      Set HOMEBREW_TAP_GITHUB_TOKEN secret
  task     P2   open         bd-q5z.1    Research k9s UX patterns
  feature  P3   open         bd-n0wb     Migrate to cobra/pflag
  feature  P4   open         bd-36s      Agenda view: time-horizon grouping
  bug      P2   open         bd-a1b      Fix tree view collapse on resize
  task     P2   in_progress  bd-c3d      Add project switching config
  task     P3   open         bd-e5f      Implement command mode
  feature  P2   open         bd-g7h      Theme support (light/dark)
  task     P3   open         bd-i9j      Breadcrumb navigation
  bug      P1   open         bd-k2l      Board view card overflow
  task     P4   open         bd-m4n      Export to markdown

  page 1/1 (12 issues)

 :filter sta                             filter status=open | filter status=closed
 ^^^^^^^^^^                              ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
 user input                              ghost suggestion (dimmed, fish-style)
```

### Mockup F: Project Picker Overlay

```
 bw | beadwork (1)          list view | open:12 ready:5 blocked:3

  ┌─ Select Project ────────────────────────────────────────────┐
  │                                                             │
  │  #  Project          Path                     Open  Ready   │
  │ ─────────────────────────────────────────────────────────── │
  │  1  beadwork      *  ~/Documents/beadwork       12     5   │
  │  2  api-server       ~/work/api-server          24     8   │
  │  3  frontend         ~/work/frontend             8     3   │
  │  4  infra            ~/work/infrastructure      15     4   │
  │  -  docs             ~/work/documentation        3     1   │
  │  -  mobile-app       ~/work/mobile               6     2   │
  │                                                             │
  │  * = active | u:favorite | enter:switch | /:search | esc   │
  └─────────────────────────────────────────────────────────────┘



 P:close picker  1-9:quick switch  /:search  enter:select  esc:close
```

### Mockup G: Help Overlay

```
 bw | beadwork (1)                                           ? to close

  ┌─ Keyboard Shortcuts ───────────────────────────────────────┐
  │                                                             │
  │  NAVIGATION                    VIEWS                       │
  │  j/k or arrows  move cursor   l   list view               │
  │  enter           open detail   t   tree view               │
  │  esc/backspace   go back       b   board view              │
  │  g/G             top/bottom    s   split view              │
  │                                d   detail view             │
  │  PROJECTS                                                  │
  │  1-9             switch favorite project                   │
  │  0               all projects (aggregate)                  │
  │  P               project picker                            │
  │  :project name   switch by name                            │
  │                                                            │
  │  ACTIONS                       SEARCH & FILTER             │
  │  e               edit issue    /    filter (fuzzy)         │
  │  n               new issue     :    command mode           │
  │  m               move status   :filter key=val             │
  │  x               close issue   :sort field [asc|desc]     │
  │                                                            │
  │  q  quit  |  ?  this help  |  `  tutorial                 │
  └────────────────────────────────────────────────────────────┘

```

### Mockup H: Aggregate View (All Projects)

```
 bw | all projects (0)      list view | open:68 ready:24 blocked:15

  PROJECT      TYPE     PRI  STATUS       ID          TITLE
  ───────      ────     ───  ──────       ──          ─────
  beadwork     epic     P1   in_progress  bd-q5z      Epic: k9s-inspired UX..
  beadwork     task     P1   open         bd-qal      Set HOMEBREW_TAP_GITH..
  api-server   bug      P0   open         api-x1y     Auth token expiry not..
  api-server   task     P1   in_progress  api-a2b     Rate limiting middlew..
  frontend     feature  P1   open         fe-c3d      Dark mode toggle
  frontend     bug      P2   open         fe-e5f      Mobile nav menu overl..
  infra        task     P1   in_progress  inf-g7h     Upgrade k8s cluster t..
  beadwork     task     P2   open         bd-q5z.1    Research k9s UX patte..
  api-server   feature  P2   open         api-i9j     Webhook retry with ex..
  beadwork     feature  P3   open         bd-n0wb     Migrate to cobra/pfla..

  page 1/7 (68 issues across 5 projects)

 1-9:project  t:tree  b:board  /:filter  :filter project=api-server  q:quit
```

---

## 6. Theme System

### Concept

Support light and dark themes with user customization via YAML config. Ship two built-in themes.

### Default Dark Theme

```yaml
# ~/.config/bw/themes/dark.yaml
colors:
  bg: "#1e1e2e"
  fg: "#cdd6f4"
  accent: "#89b4fa"
  success: "#a6e3a1"
  warning: "#f9e2af"
  error: "#f38ba8"
  muted: "#6c7086"
  border: "#45475a"

  priority:
    p0: "#f38ba8"    # Red - critical
    p1: "#fab387"    # Orange - high
    p2: "#f9e2af"    # Yellow - medium
    p3: "#a6e3a1"    # Green - low
    p4: "#6c7086"    # Gray - backlog

  status:
    open: "#89b4fa"       # Blue
    in_progress: "#f9e2af" # Yellow
    blocked: "#f38ba8"     # Red
    closed: "#6c7086"      # Gray
```

### Default Light Theme

```yaml
# ~/.config/bw/themes/light.yaml
colors:
  bg: "#eff1f5"
  fg: "#4c4f69"
  accent: "#1e66f5"
  # ... (inverted from dark)
```

### Theme Selection

```yaml
# ~/.config/bw/config.yaml
ui:
  theme: dark        # or "light", or path to custom theme
  icons: true        # Nerd Font icons (with fallback)
  headless: false    # Compact header mode (like k9s headless)
```

---

## 7. Configuration System

### Config File Location

Follow XDG Base Directory spec:

| Type | Path | Contents |
|------|------|----------|
| Config | `~/.config/bw/config.yaml` | Projects, UI prefs, keybindings |
| Data | `~/.local/share/bw/` | Themes, plugins (future) |
| State | `~/.local/state/bw/` | Recent projects, view state cache |

### Full Config Example

```yaml
# ~/.config/bw/config.yaml

# Project registry
projects:
  - name: beadwork
    path: ~/Documents/beadwork
  - name: api-server
    path: ~/work/api-server
  - name: frontend
    path: ~/work/frontend

# Favorite shortcuts (number keys 1-9)
favorites:
  1: beadwork
  2: api-server
  3: frontend

# UI preferences
ui:
  theme: dark
  default_view: list     # list, tree, board, split
  icons: true            # Nerd Font icons
  headless: false        # Compact header
  split_ratio: 0.4       # Default split pane ratio

# Auto-discovery
discovery:
  scan_paths:            # Additional paths to scan for .beads/ dirs
    - ~/work
    - ~/projects
  max_depth: 3           # How deep to scan
```

### Hot Reload

Config changes are watched via fsnotify. Theme changes apply immediately. Project list changes apply on next project switch.

---

## 8. Keyboard Architecture

### Layered Binding System

Inspired by k9s's `KeyActions` registry:

```
Global bindings (always active)
  ├── q          quit
  ├── ?          help overlay
  ├── :          command mode
  ├── /          filter mode
  ├── 0-9        project switch
  ├── P          project picker
  ├── l          list view
  ├── t          tree view
  ├── b          board view
  ├── s          split view
  └── ctrl+r     reload

View-specific bindings (active when view has focus)
  ├── List: j/k/arrows navigate, enter detail, e edit, n new
  ├── Tree: h/l collapse/expand, j/k navigate, enter detail
  ├── Board: h/l column, j/k card, m move status
  └── Detail: j/k scroll, e edit, q back

Context-specific bindings (active based on selection)
  ├── Epic selected: c create child task
  ├── Blocked issue: u unblock (show blockers)
  └── Closed issue: r reopen
```

### Key Binding Config (Future)

```yaml
# ~/.config/bw/keybindings.yaml
bindings:
  global:
    "ctrl+p": project_picker    # Override P
  list:
    "x": close_issue            # Custom binding
```

---

## 9. Implementation Phases

### Phase 1: Foundation (config + header/footer)

- Add `~/.config/bw/config.yaml` support (viper)
- Redesign header line (project name, view name, counts)
- Redesign footer with context-sensitive shortcut hints
- Refactor view switching to use single-key bindings (`l`, `t`, `b`, `s`)

### Phase 2: Project Switching

- Project registry in config
- Project picker overlay (P key)
- Number key favorites (1-9)
- Project state caching (view mode, scroll position)
- Auto-discovery of `.beads/` directories

### Phase 3: Command Mode

- Command input bar at bottom (`:` activation)
- Fish-style inline suggestions
- Core commands: `filter`, `sort`, `project`, `goto`
- Auto-complete from command registry

### Phase 4: Aggregate View

- Load multiple projects simultaneously
- Cross-project list/tree/board views
- Project column in aggregate mode
- Cross-project search

### Phase 5: Theming

- Theme YAML format
- Built-in dark/light themes
- Hot-reload theme changes
- Nerd Font icon support with ASCII fallback

### Phase 6: Polish

- Breadcrumb navigation trail
- View transition animations (optional)
- Help overlay redesign
- Tutorial flow for new users
- Keybinding customization

---

## 10. Architecture Changes

### Current Architecture

```
cmd/bw/main.go -> pkg/ui/NewModel() -> single Model manages everything
```

### Proposed Architecture

```
cmd/bw/main.go
  -> pkg/config/Load()                  # NEW: config management
  -> pkg/project/Registry               # NEW: project registry + discovery
  -> pkg/ui/NewApp()                    # CHANGED: app-level coordinator
       -> pkg/ui/Header                 # NEW: header component
       -> pkg/ui/Footer                 # NEW: footer component
       -> pkg/ui/ProjectPicker          # NEW: project picker overlay
       -> pkg/ui/CommandBar             # NEW: command input bar
       -> pkg/ui/Model                  # EXISTING: per-project view model
            -> ListView
            -> TreeView
            -> BoardView
            -> DetailView
            -> SplitView
```

The key architectural change is introducing an `App` layer above `Model`. The `App` manages:
- Project switching (loading/unloading project data)
- Global keybindings (project keys, command mode)
- Header/footer rendering
- Overlay management (project picker, help, command bar)

The `Model` remains responsible for per-project view rendering, just as today, but no longer owns the header, footer, or global key handling.

### Component Interface

Each view implements:

```go
type View interface {
    Init() tea.Cmd
    Update(tea.Msg) (View, tea.Cmd)
    View() string
    ShortcutHints() []Hint     // For context-sensitive footer
    Breadcrumb() string        // For breadcrumb trail
}
```

---

## Appendix: Comparison with k9s Patterns

| k9s Pattern | Beadwork Equivalent | Notes |
|-------------|-------------------|-------|
| Namespaces | Projects | Number keys 1-9 for favorites |
| `:resource` command | `:filter`, `:sort`, `:project` | Simpler command set |
| Resource views (pods, deploy) | Views (list, tree, board) | Letter keys instead of commands |
| Label filtering (`-l key=val`) | Filter syntax (`:filter key=val`) | Similar concept |
| Namespace favorites (`u` key) | Project favorites (`u` in picker) | Same UX |
| XRay view | Aggregate view (key `0`) | Cross-project overview |
| Breadcrumbs | Breadcrumb trail | Navigation depth indicator |
| Skins (YAML themes) | Themes (YAML) | Same format |
| `~/.config/k9s/config.yaml` | `~/.config/bw/config.yaml` | XDG compliant |
| Context-specific menu hints | Context-sensitive footer | Same concept |
| Command suggestions (fish-style) | Command mode suggestions | Same UX pattern |
| Stack-based navigation | View stack with Esc/Backspace | Same navigation model |
