package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ContextHelpContent contains compact help content for each context.
// This is used when user triggers context-specific help (e.g., double-tap backtick).
// Content should fit on one screen (~20 lines) without scrolling.
var ContextHelpContent = map[Context]string{
	ContextList:        contextHelpList,
	ContextTree:        contextHelpTree,
	ContextGraph:       contextHelpGraph,
	ContextBoard:       contextHelpBoard,
	ContextDetail:      contextHelpDetail,
	ContextSplit:       contextHelpSplit,
	ContextFilter:      contextHelpFilter,
	ContextLabelPicker: contextHelpLabelPicker,
	ContextHelp:        contextHelpHelp,
	ContextTimeTravel:  contextHelpTimeTravel,
}

// GetContextHelp returns the help content for a given context.
// Falls back to generic help if the context has no specific content.
func GetContextHelp(ctx Context) string {
	if content, ok := ContextHelpContent[ctx]; ok {
		return content
	}
	return contextHelpGeneric
}

// RenderContextHelp renders the context-specific help modal.
// This is a compact modal (~60 chars wide) that shows quick reference info.
func RenderContextHelp(ctx Context, theme Theme, width, height int) string {
	content := GetContextHelp(ctx)

	r := theme.Renderer

	// Modal dimensions - compact
	modalWidth := 60
	if modalWidth > width-4 {
		modalWidth = width - 4
	}

	// Title
	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(theme.Primary)

	// Content style
	contentStyle := r.NewStyle().
		Foreground(theme.Subtext)

	// Footer hint
	footerStyle := r.NewStyle().
		Foreground(theme.Muted).
		Italic(true)

	// Build content
	var b strings.Builder
	b.WriteString(titleStyle.Render("Quick Reference"))
	b.WriteString("\n")
	b.WriteString(r.NewStyle().Foreground(theme.Border).Render(strings.Repeat("─", modalWidth-4)))
	b.WriteString("\n\n")
	b.WriteString(contentStyle.Render(content))
	b.WriteString("\n\n")
	b.WriteString(footerStyle.Render("Press ` for full tutorial │ Esc to close"))

	// Wrap in modal style
	modalStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Secondary).
		Padding(1, 2).
		Width(modalWidth)

	return modalStyle.Render(b.String())
}

// =============================================================================
// CONTEXT-SPECIFIC HELP CONTENT (bv-4swd)
// =============================================================================

const contextHelpList = `## List View

**Navigation**
  j/k       Move up/down
  Enter     View issue details
  G         Jump to bottom
  ^d/^u     Page down/up

**Filtering**
  o/c/r/a   Open/closed/ready/all
  /         Fuzzy search
  s         Cycle sort mode
  l         Label picker

**Switch Views**
  E         Tree view
  b         Board view

**Actions**
  y         Copy issue ID
  K         Close issue (confirm)
  Del       Delete issue (confirm)
  t/T       Time-travel
  U         Self-update bv`

const contextHelpTree = `## Tree View

**Navigation**
  j/k       Move up/down
  h         Collapse or go to parent
  l         Expand or go to child
  ^b/^f     Page back/fwd (←/→ PgUp/PgDn; ^u/^d half)
  Enter/Spc Toggle expand/collapse
  Home/End  Jump to top/bottom (G also bottom)
  p         Jump to parent node

**Structure**
  X/Z       Expand/collapse all · ^w Wide · v Wrap titles
  Tab       Cycle node visibility
  S-Tab     Cycle global visibility
  1-9       Expand to level N
  d  Detail panel   ◈N  Open blockers (g inspects)

**Filtering**
  o/c/r/a   Open/closed/ready/all
  s         Sort popup · C  Columns · /  Search
  f         Toggle highlighted branch
  n/N       Next/prev match

**Modes**
  O  Occur   x  XRay
  ` + "`" + `  Flat    F  Follow
  b/B  Bookmark/cycle   m/M  Mark

**Actions**
	K  Close   Del  Delete   g  Dependency graph`

const contextHelpGraph = `## Dependency Graph

**Navigation**
  j/k       Select issue
  Home/End  Jump to top/bottom (G also bottom)
  ^f/^b     Page down/up (also ^d/^u, PgDn/PgUp)
  Enter     Open issue details

**Relationships**
  BLOCKED BY  Prerequisites for selected issue
  BLOCKS      Issues waiting on selected issue

**Exit**
  Esc/g     Return to tree`

const contextHelpBoard = `## Board View

**Navigation**
  h/l  j/k  Column, issue (epics: the first column)
  1-4  H/L  Jump to column, first/last
  gg/G      Top/bottom of column

**Filter, search, group**
  o/r       Open/ready
  c         Show or hide closed column
  /  n/N    Search, next/prev match
  s  e      Swimlanes, empty columns

**Epics**
  v         Epic rail / epic rows
  { }       Previous / next epic
  Tab       Fold the selected epic
  S-Tab     Fold or unfold all

**Row facts**
  blocked by X   An open blocker
  lane: stage    Dispatcher lane stage
  blocks N       Issues waiting on this one

**Actions**
  y         Copy issue ID
  K / Del   Close / delete (confirm)
  Enter     View issue details
  Esc       Back to the tree`

const contextHelpDetail = `## Detail View

**Navigation**
  j/k       Scroll content
  Home/End  Jump to top/bottom of content
  n/p       Next/previous sibling issue (tree)
  Esc       Return to list
  Tab       Switch to split view

**Actions**
  c         Copy full ticket Markdown
  O         Open in editor

**Info Shown**
• Full description (markdown)
• Dependencies
• Labels and metadata`

const contextHelpSplit = `## Split View

**Focus**
  Enter     Move focus between list and detail
  <         Shrink list pane
  >         Expand list pane
  \ :layout  Stack detail below the list

**Left Pane (List)**
  j/k       Navigate issues

**Right Pane (Detail)**
  j/k       Scroll content

**Exit**
  Esc       Return to list view
  Enter     Open full detail

Tip: The focused pane has the bright border, and
the footer lists the keys for that pane`

const contextHelpFilter = `## Filter Mode

**Status Filters**
  o         Open only
  c         Closed only
  r         Ready (no blockers)
  a         All (clear filter)

**Search**
  /         Start fuzzy search
  n/N       Next/prev match
  Esc       Clear search

**Label Filters**
  l         Open label picker`

const contextHelpLabelPicker = `## Label Picker

**Navigation**
  j/k       Move selection
  Enter     Apply label
  Space     Toggle multi-select
  Esc       Cancel

**Search**
  /         Filter labels

**Actions**
  n         Create new label
  d         Delete label
  e         Edit label`

const contextHelpHelp = `## Help Overlay

You're looking at the help overlay!

**Navigation**
  j/k       Scroll help content
  Space     Open full tutorial
  Esc/?     Close this overlay

**Other Help**
  ` + "`" + `         Full tutorial (any time)
  ;         Toggle shortcuts sidebar`

const contextHelpTimeTravel = `## Time Travel Mode

**Currently Viewing**: Past state

This is read-only - you're viewing
how the project looked at a specific
point in history.

**Navigation**
  j/k       Navigate issues
  Enter     View issue detail

**Exit**
  Esc       Return to present

Tip: Use History view (h) to pick
different points in time`

const contextHelpGeneric = `## Quick Reference

**Global Keys**
  ?         Help overlay
  ` + "`" + `         Full tutorial
  Esc       Close/back
  q         Quit

**Navigation**
  j/k       Move up/down
  h/l       Move left/right
  Enter     Select/open

**Views**
  b         Board view
  E         Tree view
  g         Dependency graph`
