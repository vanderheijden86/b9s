package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"

	"github.com/charmbracelet/lipgloss"
)

var selectedCardTextColor = lipgloss.AdaptiveColor{Light: "#101010", Dark: "#101010"}

// BoardModel represents the Kanban board view with adaptive columns
type BoardModel struct {
	columns      [4][]model.Issue
	activeColIdx []int  // Indices of non-empty columns (for navigation)
	focusedCol   int    // Index into activeColIdx
	selectedRow  [4]int // Store selection for each column
	theme        Theme

	// Swimlane grouping mode (bv-wjs0)
	swimLaneMode SwimLaneMode
	allIssues    []model.Issue // Store all issues for re-grouping on mode change
	boardState   *BoardState   // Optional precomputed columns for all swimlane modes (bv-guxz)

	// Reverse dependency index: maps issue ID -> slice of issue IDs it blocks (bv-1daf)
	blocksIndex map[string][]string

	// Issue lookup map: ID -> *Issue for getting blocker titles (bv-kklp)
	issueMap map[string]*model.Issue

	// Epic presentation (docs/adr/0017). rawColumns holds the grouped columns
	// before the design arranges them into columns.
	epicView     BoardEpicView
	rawColumns   [4][]model.Issue
	epicUniverse []model.Issue // whole project; nil means allIssues
	epicOf       map[string]string
	epics        map[string]*boardEpic
	foldedEpics  map[string]bool

	// The epics form the first column of a lane board. While onEpicColumn is
	// set, the selection is the epic of lane epicColumnLane ("" being the
	// lane without an epic) and focusedCol only remembers the status column.
	onEpicColumn   bool
	epicColumnLane string

	// branchRoot limits the board to one top-level issue and its descendants.
	// groupedColumns keeps the full grouping so clearing it needs no reload.
	branchRoot     string
	rootOf         map[string]string
	groupedColumns [4][]model.Issue

	// Search state (bv-yg39)
	searchMode    bool
	searchQuery   string
	searchMatches []searchMatch // Cards matching current query
	searchCursor  int           // Current match index

	// Vim key combo tracking (bv-yg39)
	waitingForG bool // True if we're waiting for second 'g' in 'gg' combo

	// Empty column visibility override (bv-tf6j)
	// nil = auto (status shows all, priority/type hide empty)
	// true = always show all columns
	// false = always hide empty columns
	showEmptyColumns *bool

	// showClosed shows the closed column in status mode. Closed work is
	// history, not a queue, so the column stays hidden until c (docs/adr/0018).
	showClosed bool

	// foldedCols are columns z folded into a rail. A folded column leaves
	// activeColIdx, so no move can focus it, but still renders as a rail.
	foldedCols [4]bool

	// Active project name for the prefix badge on cards (bd-dy6r)
	// Empty string means all-projects mode: fall back to ExtractRepoPrefix.
	activeProjectName string
}

// SetActiveProjectName sets the project name shown as a badge on each card (bd-dy6r).
func (b *BoardModel) SetActiveProjectName(name string) {
	b.activeProjectName = name
}

// searchMatch holds info about a matching card (bv-yg39)
type searchMatch struct {
	col int // Column index (0-3)
	row int // Row index within column
}

// Column indices for the Kanban board
const (
	ColOpen       = 0
	ColInProgress = 1
	ColBlocked    = 2
	ColClosed     = 3
)

// SwimLaneMode determines how cards are grouped into columns (bv-wjs0)
type SwimLaneMode int

const (
	SwimByStatus   SwimLaneMode = iota // Default: Open | In Progress | Blocked | Closed
	SwimByPriority                     // P0 Critical | P1 High | P2 Medium | P3+ Other
	SwimByType                         // Bug | Feature | Task | Epic
)

// SwimLaneModeCount is the total number of swimlane modes for cycling
const SwimLaneModeCount = 3

// ColumnStats holds computed statistics for a board column (bv-nl8a)
type ColumnStats struct {
	Total        int           // Total issues in column
	P0Count      int           // Critical priority count
	P1Count      int           // High priority count
	BlockedCount int           // Issues with blocking dependencies
	OldestAge    time.Duration // Age of oldest item
}

// computeColumnStats calculates statistics for issues in a column (bv-nl8a)
func computeColumnStats(issues []model.Issue, issueMap map[string]*model.Issue) ColumnStats {
	stats := ColumnStats{Total: len(issues)}

	var oldest time.Time
	for _, issue := range issues {
		if issue.Priority == 0 {
			stats.P0Count++
		} else if issue.Priority == 1 {
			stats.P1Count++
		}

		// Count blocked items (has unresolved blocking deps)
		hasOpenBlocker := false
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			if issueMap == nil {
				hasOpenBlocker = true
				break
			}
			if blocker, ok := issueMap[dep.DependsOnID]; ok && blocker != nil && !isClosedLikeStatus(blocker.Status) {
				hasOpenBlocker = true
				break
			}
		}
		if hasOpenBlocker {
			stats.BlockedCount++
		}

		// Track oldest by created date
		if !issue.CreatedAt.IsZero() {
			if oldest.IsZero() || issue.CreatedAt.Before(oldest) {
				oldest = issue.CreatedAt
			}
		}
	}

	if !oldest.IsZero() {
		stats.OldestAge = time.Since(oldest)
	}

	return stats
}

// formatOldestAge formats age duration for display (bv-nl8a)
func formatOldestAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days == 0 {
		return "<1d"
	}
	if days < 7 {
		return fmt.Sprintf("%dd", days)
	}
	if days < 30 {
		weeks := days / 7
		return fmt.Sprintf("%dw", weeks)
	}
	months := days / 30
	return fmt.Sprintf("%dmo", months)
}

// buildBoardColumnHeaderText formats board column header metadata with
// compact plain-text tokens (k9s-style, no emoji decorations).
func buildBoardColumnHeaderText(baseHeader string, stats ColumnStats, width int, swimLaneMode SwimLaneMode, colIdx int) string {
	if width < 100 {
		return baseHeader
	}

	parts := make([]string, 0, 4)
	if stats.P0Count > 0 {
		parts = append(parts, fmt.Sprintf("P0:%d", stats.P0Count))
	}
	if stats.P1Count > 0 {
		parts = append(parts, fmt.Sprintf("P1:%d", stats.P1Count))
	}

	if width >= 140 {
		if swimLaneMode == SwimByStatus && colIdx == ColInProgress && stats.BlockedCount > 0 {
			parts = append(parts, fmt.Sprintf("BLK:%d", stats.BlockedCount))
		}
		if stats.OldestAge > 0 && stats.Total > 0 {
			parts = append(parts, "AGE:"+formatOldestAge(stats.OldestAge))
		}
	}

	if len(parts) == 0 {
		return baseHeader
	}
	return baseHeader + " " + strings.Join(parts, " ")
}

// sortIssuesByPriorityAndDate sorts issues by priority (ascending) then by creation date (descending)
func sortIssuesByPriorityAndDate(issues []model.Issue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Priority != issues[j].Priority {
			return issues[i].Priority < issues[j].Priority
		}
		return issues[i].CreatedAt.After(issues[j].CreatedAt)
	})
}

// updateActiveColumns rebuilds the list of non-empty column indices (bv-tf6j)
// Behavior depends on swimlane mode unless explicitly overridden:
// - Status mode: shows all 4 columns (even empty) for workflow visibility
// - Priority/Type modes: hides empty columns to save space
func (b *BoardModel) updateActiveColumns() {
	// Determine whether to show empty columns
	showEmpty := b.shouldShowEmptyColumns()

	b.activeColIdx = nil
	for i := 0; i < 4; i++ {
		if b.columnHidden(i) || b.foldedCols[i] {
			continue
		}
		if len(b.columns[i]) > 0 || showEmpty {
			b.activeColIdx = append(b.activeColIdx, i)
		}
	}
	// If all columns are empty (and we're hiding empty), include all columns anyway
	if len(b.activeColIdx) == 0 {
		for i := 0; i < 4; i++ {
			if !b.columnHidden(i) && !b.foldedCols[i] {
				b.activeColIdx = append(b.activeColIdx, i)
			}
		}
	}
	// Ensure focused column is within valid range
	if b.focusedCol >= len(b.activeColIdx) {
		b.focusedCol = len(b.activeColIdx) - 1
	}
	if b.focusedCol < 0 {
		b.focusedCol = 0
	}
	b.leaveStaleEpicColumn()
}

// columnHidden reports whether a column is left off the board whatever it
// holds: the closed column in status mode until c shows it.
func (b *BoardModel) columnHidden(col int) bool {
	return col == ColClosed && b.swimLaneMode == SwimByStatus && !b.showClosed
}

// ShowsClosedColumn reports whether c has shown the closed column.
func (b *BoardModel) ShowsClosedColumn() bool { return b.showClosed }

// ToggleClosedColumn shows or hides the closed column and keeps the selection
// unless it sat in the column being hidden.
func (b *BoardModel) ToggleClosedColumn() {
	var selID string
	if sel := b.SelectedIssue(); sel != nil {
		selID = sel.ID
	}
	b.showClosed = !b.showClosed
	b.updateActiveColumns()
	if selID == "" || !b.SelectIssueByID(selID) {
		b.FocusPopulatedColumn()
	}
}

// ColumnFolded reports whether z has folded col into a rail.
func (b *BoardModel) ColumnFolded(col int) bool {
	return col >= 0 && col < 4 && b.foldedCols[col]
}

// FoldedColumnCount is the number of columns z has folded.
func (b *BoardModel) FoldedColumnCount() int {
	n := 0
	for _, f := range b.foldedCols {
		if f {
			n++
		}
	}
	return n
}

// FoldFocusedColumn folds the focused column into a rail and moves the focus
// to the nearest column still shown. It returns the folded column's header,
// or "" when nothing folds: on the epic column, or when the focused column
// is the last one shown, since the focused column always renders in full.
func (b *BoardModel) FoldFocusedColumn() string {
	if b.onEpicColumn || len(b.activeColIdx) < 2 {
		return ""
	}
	col := b.actualFocusedCol()
	lane := b.selectedLane()
	b.foldedCols[col] = true
	b.updateActiveColumns()
	b.focusAfterFold(col, lane)
	return b.getColumnHeaders()[col]
}

// focusAfterFold puts the focus on the first shown column right of the
// folded one, else left of it, preferring a card in the same epic lane.
func (b *BoardModel) focusAfterFold(folded int, lane string) {
	order := make([]int, 0, len(b.activeColIdx))
	for i, col := range b.activeColIdx {
		if col > folded {
			order = append(order, i)
		}
	}
	for i := len(b.activeColIdx) - 1; i >= 0; i-- {
		if b.activeColIdx[i] < folded {
			order = append(order, i)
		}
	}
	if b.hasEpicLanes() {
		for _, i := range order {
			if row := b.laneCardRow(b.activeColIdx[i], lane); row >= 0 {
				b.focusedCol = i
				b.selectedRow[b.activeColIdx[i]] = row
				return
			}
		}
	}
	for _, i := range order {
		if len(b.columns[b.activeColIdx[i]]) > 0 {
			b.focusedCol = i
			return
		}
	}
	if len(order) > 0 {
		b.focusedCol = order[0]
	}
}

// UnfoldColumns unfolds every column z folded and keeps the selection.
func (b *BoardModel) UnfoldColumns() {
	if b.FoldedColumnCount() == 0 {
		return
	}
	var selID string
	if sel := b.SelectedIssue(); sel != nil && !b.onEpicColumn {
		selID = sel.ID
	}
	b.foldedCols = [4]bool{}
	b.updateActiveColumns()
	if selID != "" {
		b.SelectIssueByID(selID)
	}
}

// ClosedHiddenCount is the number of issues the hidden closed column holds,
// or 0 when the column is shown.
func (b *BoardModel) ClosedHiddenCount() int {
	if !b.columnHidden(ColClosed) {
		return 0
	}
	return len(b.rawColumns[ColClosed])
}

// shouldShowEmptyColumns returns whether empty columns should be visible (bv-tf6j)
func (b *BoardModel) shouldShowEmptyColumns() bool {
	// Explicit override takes precedence
	if b.showEmptyColumns != nil {
		return *b.showEmptyColumns
	}
	// Auto behavior: Status mode shows all, others hide empty
	return b.swimLaneMode == SwimByStatus
}

// ToggleEmptyColumns cycles through empty column visibility modes (bv-tf6j)
// nil (auto) -> true (show all) -> false (hide empty) -> nil (auto)
func (b *BoardModel) ToggleEmptyColumns() {
	if b.showEmptyColumns == nil {
		showAll := true
		b.showEmptyColumns = &showAll
	} else if *b.showEmptyColumns {
		hideEmpty := false
		b.showEmptyColumns = &hideEmpty
	} else {
		b.showEmptyColumns = nil // Back to auto
	}
	b.updateActiveColumns()
}

// GetEmptyColumnVisibilityMode returns the current visibility mode name (bv-tf6j)
func (b *BoardModel) GetEmptyColumnVisibilityMode() string {
	if b.showEmptyColumns == nil {
		return "Auto"
	}
	if *b.showEmptyColumns {
		return "Show All"
	}
	return "Hide Empty"
}

// HiddenColumnCount returns the number of empty columns currently hidden (bv-tf6j)
func (b *BoardModel) HiddenColumnCount() int {
	hidden := 0
	for i := 0; i < 4; i++ {
		if len(b.columns[i]) == 0 && !b.columnHidden(i) {
			// Check if this column is in activeColIdx
			found := false
			for _, idx := range b.activeColIdx {
				if idx == i {
					found = true
					break
				}
			}
			if !found {
				hidden++
			}
		}
	}
	return hidden
}

// buildBlocksIndex creates a reverse dependency map: for each issue that is depended on,
// it stores the list of issue IDs that depend on it (bv-1daf)
func buildBlocksIndex(issues []model.Issue) map[string][]string {
	index := make(map[string][]string)
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if dep != nil && dep.Type.IsBlocking() {
				// dep.DependsOnID blocks issue.ID
				index[dep.DependsOnID] = append(index[dep.DependsOnID], issue.ID)
			}
		}
	}
	return index
}

// groupIssuesByMode distributes issues into 4 columns based on swimlane mode (bv-wjs0)
func groupIssuesByMode(issues []model.Issue, mode SwimLaneMode) [4][]model.Issue {
	var cols [4][]model.Issue

	for _, issue := range issues {
		var colIdx int
		switch mode {
		case SwimByStatus:
			// Default: Open | In Progress | Blocked | Closed
			switch {
			case isClosedLikeStatus(issue.Status):
				colIdx = 3
			case issue.Status == model.StatusOpen:
				colIdx = 0
			case issue.Status == model.StatusInProgress:
				colIdx = 1
			case issue.Status == model.StatusBlocked:
				colIdx = 2
			default:
				colIdx = 0
			}
		case SwimByPriority:
			// P0 Critical | P1 High | P2 Medium | P3+ Other
			switch {
			case issue.Priority == 0:
				colIdx = 0 // Critical
			case issue.Priority == 1:
				colIdx = 1 // High
			case issue.Priority == 2:
				colIdx = 2 // Medium
			default:
				colIdx = 3 // P3+ Other
			}
		case SwimByType:
			// Bug | Feature | Task | Epic
			switch issue.IssueType {
			case model.TypeBug:
				colIdx = 0
			case model.TypeFeature:
				colIdx = 1
			case model.TypeTask:
				colIdx = 2
			case model.TypeEpic:
				colIdx = 3
			default:
				colIdx = 2 // Default to Task
			}
		}
		cols[colIdx] = append(cols[colIdx], issue)
	}

	// Sort each column
	for i := 0; i < 4; i++ {
		sortIssuesByPriorityAndDate(cols[i])
	}

	return cols
}

// GetSwimLaneModeName returns the display name for the current swimlane mode (bv-wjs0)
func (b *BoardModel) GetSwimLaneModeName() string {
	switch b.swimLaneMode {
	case SwimByStatus:
		return "Status"
	case SwimByPriority:
		return "Priority"
	case SwimByType:
		return "Type"
	default:
		return "Status"
	}
}

// GetSwimLaneMode returns the current swimlane mode (bv-wjs0)
func (b *BoardModel) GetSwimLaneMode() SwimLaneMode {
	return b.swimLaneMode
}

// CycleSwimLaneMode cycles to the next swimlane mode and regroups issues (bv-wjs0)
func (b *BoardModel) CycleSwimLaneMode() {
	b.swimLaneMode = SwimLaneMode((int(b.swimLaneMode) + 1) % SwimLaneModeCount)
	// A fold names a column position, which means another group in the new mode.
	b.foldedCols = [4]bool{}
	b.regroupIssues()
}

// regroupIssues rebuilds columns based on current swimlane mode (bv-wjs0)
func (b *BoardModel) regroupIssues() {
	if b.boardState != nil {
		b.setColumns(b.boardState.ColumnsForMode(b.swimLaneMode))
	} else {
		b.setColumns(groupIssuesByMode(b.allIssues, b.swimLaneMode))
	}

	// Reset selection to avoid out-of-bounds
	for i := 0; i < 4; i++ {
		if b.selectedRow[i] >= len(b.columns[i]) {
			if len(b.columns[i]) > 0 {
				b.selectedRow[i] = len(b.columns[i]) - 1
			} else {
				b.selectedRow[i] = 0
			}
		}
	}

	b.updateActiveColumns()
	b.CancelSearch() // Clear stale search matches
}

// getColumnHeaders returns the column header titles based on swimlane mode (bv-wjs0)
func (b *BoardModel) getColumnHeaders() []string {
	switch b.swimLaneMode {
	case SwimByPriority:
		return []string{"P0 CRITICAL", "P1 HIGH", "P2 MEDIUM", "P3+ OTHER"}
	case SwimByType:
		return []string{"BUG", "FEATURE", "TASK", "EPIC"}
	default: // SwimByStatus
		return []string{"OPEN", "IN PROGRESS", "BLOCKED", "CLOSED"}
	}
}

// NewBoardModel creates a new Kanban board from the given issues
func NewBoardModel(issues []model.Issue, theme Theme) BoardModel {
	// Group issues by default mode (status) - bv-wjs0
	cols := groupIssuesByMode(issues, SwimByStatus)

	// Build issue lookup map for getting blocker titles (bv-kklp)
	issueMap := make(map[string]*model.Issue, len(issues))
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}

	b := BoardModel{
		columns:      cols,
		focusedCol:   0,
		theme:        theme,
		swimLaneMode: SwimByStatus, // Default mode (bv-wjs0)
		allIssues:    issues,       // Store for regrouping (bv-wjs0)
		blocksIndex:  buildBlocksIndex(issues),
		issueMap:     issueMap,
	}
	b.setColumns(cols)
	b.updateActiveColumns()
	return b
}

// SetIssues updates the board data, typically after filtering
func (b *BoardModel) SetIssues(issues []model.Issue) {
	// Store all issues for regrouping on mode change (bv-wjs0)
	b.allIssues = issues
	b.boardState = nil

	// Group by current swimlane mode (bv-wjs0)
	b.setColumns(groupIssuesByMode(issues, b.swimLaneMode))

	b.blocksIndex = buildBlocksIndex(issues) // Rebuild reverse dependency index (bv-1daf)

	// Rebuild issue lookup map for blocker titles (bv-kklp)
	b.issueMap = make(map[string]*model.Issue, len(issues))
	for i := range issues {
		b.issueMap[issues[i].ID] = &issues[i]
	}

	// Clear search state - stale matches could reference invalid positions (bv-yg39)
	b.CancelSearch()

	// Sanitize selection to prevent out-of-bounds
	for i := 0; i < 4; i++ {
		if b.selectedRow[i] >= len(b.columns[i]) {
			if len(b.columns[i]) > 0 {
				b.selectedRow[i] = len(b.columns[i]) - 1
			} else {
				b.selectedRow[i] = 0
			}
		}
	}

	b.updateActiveColumns()
}

// SetSnapshot updates the board data directly from a DataSnapshot (bv-guxz).
// This avoids UI-thread grouping/sorting work when the full dataset is shown.
func (b *BoardModel) SetSnapshot(s *DataSnapshot) {
	if s == nil {
		b.SetIssues(nil)
		return
	}

	b.allIssues = s.Issues
	b.boardState = s.BoardState

	if b.boardState != nil {
		b.setColumns(b.boardState.ColumnsForMode(b.swimLaneMode))
	} else {
		b.setColumns(groupIssuesByMode(s.Issues, b.swimLaneMode))
	}

	// Build reverse-dependency index from issues.
	b.blocksIndex = buildBlocksIndex(s.Issues)

	// Use snapshot issue map for blocker titles.
	b.issueMap = s.IssueMap

	// Clear search state - stale matches could reference invalid positions (bv-yg39)
	b.CancelSearch()

	// Sanitize selection to prevent out-of-bounds
	for i := 0; i < 4; i++ {
		if b.selectedRow[i] >= len(b.columns[i]) {
			if len(b.columns[i]) > 0 {
				b.selectedRow[i] = len(b.columns[i]) - 1
			} else {
				b.selectedRow[i] = 0
			}
		}
	}

	b.updateActiveColumns()
}

// actualFocusedCol returns the actual column index (0-3) being focused
func (b *BoardModel) actualFocusedCol() int {
	if len(b.activeColIdx) == 0 {
		return 0
	}
	return b.activeColIdx[b.focusedCol]
}

// Navigation methods
//
// With epic lanes, the moves below stop on cards only. An epic's own issue
// sits in the column of its status but is drawn as the lane header, so as a
// stop it would appear in some columns and not in others. The epics are
// reached through the epic column instead (board_epics.go).

// isCardRow reports whether row of col is a card rather than a lane header.
func (b *BoardModel) isCardRow(col, row int) bool {
	is := b.columns[col][row]
	return !b.hasEpicLanes() || b.epicOf[is.ID] != is.ID
}

// cardRowFrom returns the first card row from row in direction step (+1 or
// -1), or -1 when there is none.
func (b *BoardModel) cardRowFrom(col, row, step int) int {
	for r := row; r >= 0 && r < len(b.columns[col]); r += step {
		if b.isCardRow(col, r) {
			return r
		}
	}
	return -1
}

func (b *BoardModel) MoveDown() { b.moveVertical(1) }

func (b *BoardModel) MoveUp() { b.moveVertical(-1) }

// moveVertical moves one card in direction step through the column, into
// the next lane at a lane's edge. In the epic column it moves one epic.
func (b *BoardModel) moveVertical(step int) {
	if b.onEpicColumn {
		b.stepEpicColumn(step)
		return
	}
	col := b.actualFocusedCol()
	if r := b.cardRowFrom(col, b.selectedRow[col]+step, step); r >= 0 {
		b.selectedRow[col] = r
	}
}

func (b *BoardModel) MoveRight() {
	if b.hasEpicLanes() {
		from := b.focusedCol + 1
		if b.onEpicColumn {
			from = 0
		}
		b.moveAcross(from, 1)
		return
	}
	if b.focusedCol < len(b.activeColIdx)-1 {
		b.focusedCol++
	}
}

func (b *BoardModel) MoveLeft() {
	if b.onEpicColumn {
		return
	}
	if b.hasEpicLanes() {
		b.moveAcross(b.focusedCol-1, -1)
		return
	}
	if b.focusedCol > 0 {
		b.focusedCol--
	}
}

func (b *BoardModel) MoveToTop() {
	if b.onEpicColumn {
		b.stepEpicColumn(-len(b.shownLanes()))
		return
	}
	col := b.actualFocusedCol()
	if r := b.cardRowFrom(col, 0, 1); r >= 0 {
		b.selectedRow[col] = r
	}
}

func (b *BoardModel) MoveToBottom() {
	if b.onEpicColumn {
		b.stepEpicColumn(len(b.shownLanes()))
		return
	}
	col := b.actualFocusedCol()
	if r := b.cardRowFrom(col, len(b.columns[col])-1, -1); r >= 0 {
		b.selectedRow[col] = r
	}
}

func (b *BoardModel) PageDown(visibleRows int) {
	b.pageTo(visibleRows / 2)
}

func (b *BoardModel) PageUp(visibleRows int) {
	b.pageTo(-visibleRows / 2)
}

// pageTo moves delta rows, clamped to the column, and settles on the nearest
// card: onward in the paging direction first, then back.
func (b *BoardModel) pageTo(delta int) {
	col := b.actualFocusedCol()
	count := len(b.columns[col])
	if count == 0 {
		return
	}
	target := max(min(b.selectedRow[col]+delta, count-1), 0)
	step := 1
	if delta < 0 {
		step = -1
	}
	r := b.cardRowFrom(col, target, step)
	if r < 0 {
		r = b.cardRowFrom(col, target, -step)
	}
	if r >= 0 {
		b.selectedRow[col] = r
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Enhanced Navigation (bv-yg39)
// ═══════════════════════════════════════════════════════════════════════════

// JumpToColumn jumps directly to a specific column (1-4 maps to 0-3)
func (b *BoardModel) JumpToColumn(colIdx int) {
	if colIdx < 0 || colIdx > 3 {
		return
	}
	for i, activeCol := range b.activeColIdx {
		if activeCol == colIdx {
			b.focusedCol = i
			b.onEpicColumn = false
			return
		}
	}
	// Column is empty - find nearest active column
	bestIdx := 0
	bestDist := 100
	for i, activeCol := range b.activeColIdx {
		dist := activeCol - colIdx
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	b.focusedCol = bestIdx
	b.onEpicColumn = false
}

// JumpToFirstColumn jumps to the first non-empty column (H key)
func (b *BoardModel) JumpToFirstColumn() {
	if len(b.activeColIdx) > 0 {
		b.focusedCol = 0
		b.onEpicColumn = false
	}
}

// JumpToLastColumn jumps to the last non-empty column (L key)
func (b *BoardModel) JumpToLastColumn() {
	if len(b.activeColIdx) > 0 {
		b.focusedCol = len(b.activeColIdx) - 1
		b.onEpicColumn = false
	}
}

// ClearWaitingForG clears the gg combo state
func (b *BoardModel) ClearWaitingForG() { b.waitingForG = false }

// SetWaitingForG sets the gg combo state
func (b *BoardModel) SetWaitingForG() { b.waitingForG = true }

// IsWaitingForG returns whether we're waiting for second g
func (b *BoardModel) IsWaitingForG() bool { return b.waitingForG }

// ═══════════════════════════════════════════════════════════════════════════
// Search functionality (bv-yg39)
// ═══════════════════════════════════════════════════════════════════════════

// IsSearchMode returns whether search mode is active
func (b *BoardModel) IsSearchMode() bool { return b.searchMode }

// StartSearch enters search mode
func (b *BoardModel) StartSearch() {
	b.searchMode = true
	b.searchQuery = ""
	b.searchMatches = nil
	b.searchCursor = 0
}

// CancelSearch exits search mode and clears results
func (b *BoardModel) CancelSearch() {
	b.searchMode = false
	b.searchQuery = ""
	b.searchMatches = nil
	b.searchCursor = 0
}

// FinishSearch exits search mode but keeps results for n/N navigation
func (b *BoardModel) FinishSearch() {
	b.searchMode = false
}

// SetIssueQuery applies the shared query after the board's filtered issue set changes.
func (b *BoardModel) SetIssueQuery(query IssueQuery) {
	b.searchMode = false
	b.searchQuery = query.Raw()
	b.updateSearchMatches()
}

// SearchQuery returns the current search query
func (b *BoardModel) SearchQuery() string { return b.searchQuery }

// SearchMatchCount returns the number of matches
func (b *BoardModel) SearchMatchCount() int { return len(b.searchMatches) }

// SearchCursorPos returns current match position (1-indexed for display)
func (b *BoardModel) SearchCursorPos() int {
	if len(b.searchMatches) == 0 {
		return 0
	}
	return b.searchCursor + 1
}

// AppendSearchChar adds a character to the search query
func (b *BoardModel) AppendSearchChar(ch rune) {
	b.searchQuery += string(ch)
	b.updateSearchMatches()
}

// BackspaceSearch removes the last character from search query
func (b *BoardModel) BackspaceSearch() {
	if len(b.searchQuery) > 0 {
		runes := []rune(b.searchQuery)
		b.searchQuery = string(runes[:len(runes)-1])
		b.updateSearchMatches()
	}
}

// updateSearchMatches finds all cards matching the search query
func (b *BoardModel) updateSearchMatches() {
	b.searchMatches = nil
	b.searchCursor = 0
	if b.searchQuery == "" {
		return
	}
	query := ParseIssueQuery(b.searchQuery)
	for colIdx, issues := range b.columns {
		for rowIdx, issue := range issues {
			if query.Matches(issue) {
				b.searchMatches = append(b.searchMatches, searchMatch{col: colIdx, row: rowIdx})
			}
		}
	}
	if len(b.searchMatches) > 0 {
		b.jumpToMatch(0)
	}
}

// jumpToMatch navigates to a specific match
func (b *BoardModel) jumpToMatch(idx int) {
	if idx < 0 || idx >= len(b.searchMatches) {
		return
	}
	b.searchCursor = idx
	match := b.searchMatches[idx]
	for i, activeCol := range b.activeColIdx {
		if activeCol == match.col {
			b.focusedCol = i
			break
		}
	}
	b.selectedRow[match.col] = match.row
	b.onEpicColumn = false
}

// NextMatch jumps to the next search match (n key)
func (b *BoardModel) NextMatch() {
	if len(b.searchMatches) == 0 {
		return
	}
	b.jumpToMatch((b.searchCursor + 1) % len(b.searchMatches))
}

// PrevMatch jumps to the previous search match (N key)
func (b *BoardModel) PrevMatch() {
	if len(b.searchMatches) == 0 {
		return
	}
	prevIdx := b.searchCursor - 1
	if prevIdx < 0 {
		prevIdx = len(b.searchMatches) - 1
	}
	b.jumpToMatch(prevIdx)
}

// IsMatchHighlighted returns true if position is current search match
func (b *BoardModel) IsMatchHighlighted(colIdx, rowIdx int) bool {
	if b.searchQuery == "" || len(b.searchMatches) == 0 {
		return false
	}
	match := b.searchMatches[b.searchCursor]
	return match.col == colIdx && match.row == rowIdx
}

// IsSearchMatch returns true if position matches the search query
func (b *BoardModel) IsSearchMatch(colIdx, rowIdx int) bool {
	if b.searchQuery == "" {
		return false
	}
	for _, m := range b.searchMatches {
		if m.col == colIdx && m.row == rowIdx {
			return true
		}
	}
	return false
}

// SelectedIssue returns the currently selected issue, or nil if none. In the
// epic column that is the lane's epic, and nil for the lane without one.
func (b *BoardModel) SelectedIssue() *model.Issue {
	if b.onEpicColumn {
		return b.epicIssue(b.epicColumnLane)
	}
	return b.selectedCard()
}

// selectedCard returns the selected issue of the focused column.
func (b *BoardModel) selectedCard() *model.Issue {
	col := b.actualFocusedCol()
	cols := b.columns[col]
	row := b.selectedRow[col]
	if len(cols) > 0 && row < len(cols) {
		return &cols[row]
	}
	return nil
}

// SelectIssueByID attempts to focus and select the given issue ID on the board.
// Returns true if the issue was found in the current board columns.
func (b *BoardModel) SelectIssueByID(id string) bool {
	if id == "" {
		return false
	}
	if b.hasEpicLanes() && b.epicOf[id] == id {
		for _, lane := range b.shownLanes() {
			if lane == id {
				b.selectEpicColumn(id)
				return true
			}
		}
	}

	// Search the shown columns; if found, set both focused column and selected row.
	for col := 0; col < 4; col++ {
		if b.columnHidden(col) {
			continue
		}
		for row := range b.columns[col] {
			if b.columns[col][row].ID != id {
				continue
			}

			// Focus the matching column (focusedCol is an index into activeColIdx).
			for i, colIdx := range b.activeColIdx {
				if colIdx == col {
					b.focusedCol = i
					break
				}
			}
			b.selectedRow[col] = row
			b.onEpicColumn = false
			return true
		}
	}

	return false
}

// ColumnCount returns the number of issues in a column
func (b *BoardModel) ColumnCount(col int) int {
	if col >= 0 && col < 4 {
		return len(b.columns[col])
	}
	return 0
}

// TotalCount returns the total number of issues across all columns
func (b *BoardModel) TotalCount() int {
	total := 0
	for i := 0; i < 4; i++ {
		total += len(b.columns[i])
	}
	return total
}

// getAgeColor returns a color based on issue age (bv-1daf)
// green (<7d), yellow (7-30d), red (>30d stale)
func getAgeColor(t time.Time) lipgloss.TerminalColor {
	if t.IsZero() {
		return ColorMuted
	}
	days := int(time.Since(t).Hours() / 24)
	switch {
	case days < 7:
		return lipgloss.AdaptiveColor{Light: "#2e7d32", Dark: "#81c784"} // green
	case days < 30:
		return lipgloss.AdaptiveColor{Light: "#f57c00", Dark: "#ffb74d"} // yellow/orange
	default:
		return lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#e57373"} // red
	}
}

// formatPriority returns priority as P0/P1/P2/P3/P4 (bv-1daf)
func formatPriority(p int) string {
	if p < 0 {
		p = 0
	}
	if p > 4 {
		p = 4
	}
	return fmt.Sprintf("P%d", p)
}
