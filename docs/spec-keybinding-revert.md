# Spec: Revert Key Bindings (Tab/Enter/Space)

## Summary

Restore previous key bindings: Tab = expand/collapse, Enter = detail toggle, Space = removed.

## Tree View (tree-only mode)

| Key | Action |
|-----|--------|
| Tab | CycleNodeVisibility (3-state: collapsed → children → full subtree → collapsed) |
| Shift+Tab | CycleGlobalVisibility (all nodes cycle together) |
| Enter | Open detail view (always, regardless of leaf/parent node) |
| Space | **Removed** — does nothing |

## Tree View (split mode)

| Key | Action |
|-----|--------|
| Tab | Focus toggle between tree and detail pane (existing behavior preserved) |
| Enter | Open detail view / return from detail to tree (true toggle) |
| Space | **Removed** — does nothing |

## Detail View

| Key | Action |
|-----|--------|
| Enter | Return to tree view (true toggle with tree Enter) |

## Board View

| Key | Action |
|-----|--------|
| Tab | Card expand/collapse (3-state cycle, replaces 'd' key) |
| Enter | Open detail panel for selected card |
| d | **Removed** as card cycle key (Tab replaces it) |

## List View

| Key | Action |
|-----|--------|
| Space | **Removed** — no longer opens status picker |
| Enter | Open detail view (unchanged) |

## Help Overlay

| Key | Action |
|-----|--------|
| Space | **Removed** — no longer opens tutorial |

Tutorial is no longer accessible from the help overlay.

## Modal Pickers (repo picker, label picker)

Space is **kept** in modal pickers (repo picker uses Space for toggle selection). These are special modal contexts.

## Shortcut Bar (footer hints)

**Tree view hints:**
```
tab:fold  enter:detail  ...
```

**Board view hints:**
Update to reflect Tab = card fold, Enter = detail.

## Key Decisions

- Tab is context-dependent: fold in tree-only/board, focus-toggle in split view
- Enter is a true toggle: opens detail from tree, returns to tree from detail
- Space is removed from all main views but kept in modal pickers
- Shift+Tab restored for global visibility cycling in tree
- Board 'd' key replaced by Tab for card cycling
- Board Enter opens detail panel (previously Tab did this)
