# Tree View Enhancement: Full-Featured Split View

**Beads Task:** bd-zvh
**Priority:** P1
**Status:** Design complete, ready for implementation

## Goal

Replace the current minimal tree view (`tree.go`) with a full-featured split view that mirrors the main list view exactly, but with hierarchical ordering and expand/collapse — like Emacs org-mode.

## Current State

The tree view (`pkg/ui/tree.go`, ~29KB) has:
- Hierarchical parent-child display with expand/collapse
- Vim navigation (h/j/k/l, gg/G, P for parent)
- State persistence (`.beads/tree-state.json`)
- Windowed rendering (O(viewport) performance)
- Tree characters (branch/pipe/tee/elbow)

It's missing: detail panel, sorting, filtering, search, rich column rendering.

## Design

### Layout

Exact same layout as the main list view split screen:

```
┌─ TYPE PRI STATUS  ID     TITLE ─┬─ Detail Panel ────────────────────┐
│ ▼ ✨  P3  OPEN  bd-n0wb  Migrat│  ✨ Migrate from Go flag to cobra │
│   ├── 🐛 P3 OPEN bd-3far Updat │  ┌─────────┬────────┬──────┬─────┐│
│   └── ✅ P1 DONE bd-dikp Creat │  │ ID      │ Status │ Pri  │ ... ││
│ ▼ ✨  P0  DONE  bv-jwie  Phase │  │ bd-n0wb │ OPEN   │ P3   │     ││
│   ├── ✅ P0 DONE bv-78g6 Creat │  └─────────┴────────┴──────┴─────┘│
│   └── ✅ P0 DONE bv-jvzi Add p │                                    │
│ ► ✨  P0  DONE  bv-0cfl  Share │  Triage Insights                   │
│ ► 🐛  P1  DONE  bd-wphn  Fix C │  - Triage Score: ...               │
│                                  │  - Quick Win / Primary Reason     │
│                                  │                                    │
│                                  │  Graph Analysis                    │
│                                  │  - Impact Depth, Centrality, etc.  │
│                                  │                                    │
│                                  │  Description                       │
│                                  │  (full markdown, scrollable)       │
├──────────────────────────────────┴───────────────────────────────────┤
│ [ALL] L:labels  v0.14.4  ...  734 issues  tab focus  ...  ? help    │
└─────────────────────────────────────────────────────────────────────┘
```

### Left Pane: Tree List

- Same columns as main view: TYPE, PRI, STATUS, ID, TITLE
- Same column width logic, same color coding, same icons
- Tree indentation added before TYPE column:
  - Expand/collapse arrows: `▼` (expanded), `►` (collapsed), space (leaf)
  - Tree characters: `├──`, `└──`, `│` for hierarchy lines
- Indentation eats into TITLE width (not column headers)
- Column headers remain at the top, same as main view

### Right Pane: Detail Panel

Identical to the main view's detail panel:
- Issue metadata table (ID, Status, Priority, Assignee, Created)
- Triage Insights (score, quick win, primary reason)
- Graph Analysis (impact depth, centrality, flow role)
- Description (full markdown rendering via glamour)
- Comments section
- Scrollable viewport

Reuse the existing detail rendering code from `model.go` — the `renderDetailPanel()` or equivalent function that builds the right pane content.

### Footer

Identical to main view footer. Same filter indicators, sort mode display, issue count, keybinding hints.

## Keybindings

### Carried over from main view (identical behavior)
| Key | Action |
|-----|--------|
| `s` | Cycle sort mode (Created asc/desc, Priority, Updated, Default) |
| `o` | Filter: open issues only |
| `c` | Filter: closed issues only |
| `r` | Filter: ready (unblocked) only |
| `/` | Enter search mode |
| `Tab` | Toggle detail panel focus (for scrolling detail) |
| `Ctrl+R` | Refresh |
| `x` | Export |
| `C` | Copy |

### Tree-specific (existing, kept as-is)
| Key | Action |
|-----|--------|
| `h` / `Left` | Collapse node or jump to parent |
| `l` / `Right` | Expand node or move to first child |
| `j` / `k` | Move up/down in flattened tree |
| `X` | Expand all nodes |
| `Z` | Collapse all nodes |
| `P` | Jump to parent |
| `gg` / `G` | Jump to top / bottom |

### Navigation priority
When detail panel is focused (`Tab`), `j/k` scroll the detail viewport. When tree is focused, `j/k` navigate the tree.

## Filtering Behavior

When a filter is active (e.g., "open only"):
- **Matching issues** render normally with full colors
- **Ancestor context** — non-matching parents that have matching descendants are shown **dimmed** (reduced opacity/grey) to preserve tree structure
- **Non-matching subtrees** — entire subtrees with no matching descendants are hidden
- The detail panel only shows full details for matching issues; selecting a dimmed ancestor shows it greyed/minimal

### Filter implementation
1. Walk the tree, mark each node as `matches: true/false` based on filter
2. Walk bottom-up: if any child matches, mark ancestors as `contextAncestor: true`
3. During rendering: skip nodes that are neither `matches` nor `contextAncestor`
4. Context ancestors render with dimmed style (lipgloss `.Faint(true)` or grey foreground)

## Sorting Behavior

Sorting reorders **siblings within each parent**, preserving the hierarchy:
- Sort by Priority: P0 children appear first within each parent
- Sort by Created: newest/oldest children first within each parent
- Sort by Updated: most recently updated first
- Default: original insertion order

Root-level nodes are also sorted among themselves.

The sort mode cycles with `s`, same as main view. The footer shows the current sort mode.

## Implementation Strategy

### Phase 1: Split view layout
- Add `viewport.Model` for detail panel to `TreeModel`
- Add `showDetail` / `isSplitView` logic mirroring main view
- Render left pane (tree) + right pane (detail) side by side
- Wire `Tab` to toggle focus between tree and detail viewport
- Update detail content on cursor movement

### Phase 2: Column-based tree rendering
- Replace current single-line tree node rendering with column-aligned rendering
- Match the main view's column widths and header
- Add TYPE, PRI, STATUS, ID columns with same styling
- Tree indentation applied within the row, before TYPE or consuming TITLE space

### Phase 3: Detail panel content
- Reuse or extract the main view's detail rendering into a shared function
- Wire it into tree view — on cursor change, rebuild detail content
- Include: metadata table, triage insights, graph analysis, description markdown

### Phase 4: Sorting
- Add `sortMode` to `TreeModel`
- On sort change, re-sort children arrays within each `IssueTreeNode`
- Rebuild `flatList` after sorting
- Show sort mode in footer

### Phase 5: Filtering
- Add `currentFilter` to `TreeModel`
- Implement filter-with-ancestors logic (match + contextAncestor marking)
- Dimmed rendering for context ancestors
- Filter keybindings: `o`, `c`, `r`, `/`
- Show active filter in footer

### Phase 6: Search
- Add search input (same as main view's `/` mode)
- Highlight matching nodes in tree
- `n`/`N` to cycle through matches
- Auto-expand parent nodes of matches

## Files to Modify

- `pkg/ui/tree.go` — Main changes: split view, columns, sort, filter, search
- `pkg/ui/model.go` — Extract shared detail rendering; wire tree keybindings
- `pkg/ui/theme.go` — Possibly add dimmed/context ancestor style (if not already available)

## Out of Scope

- Swimlane modes (the tree IS the organizational mode)
- Inline card expansion (detail panel serves this purpose)
- Kanban-style card formatting (tree uses row-based list format)
