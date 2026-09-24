package ui

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// BoardEpicView selects how the board shows which epic each issue belongs to.
// All three keep the status columns of layout A; v cycles them (docs/adr/0017).
type BoardEpicView int

const (
	// BoardEpicLanes cuts the columns into horizontal bands, one per epic,
	// aligned across the columns. Tab folds a band down to its epic row.
	BoardEpicLanes BoardEpicView = iota
	// BoardEpicChips keeps the plain column order and tags each row with a
	// colored bar and an epic chip.
	BoardEpicChips
	// BoardEpicGroups groups each column by epic under its own subheader.
	BoardEpicGroups

	boardEpicViewCount = 3
)

func (v BoardEpicView) String() string {
	switch v {
	case BoardEpicChips:
		return "chips"
	case BoardEpicGroups:
		return "groups"
	}
	return "lanes"
}

// Label is the name the board bar and the status line show.
func (v BoardEpicView) Label() string {
	switch v {
	case BoardEpicChips:
		return "Epic chips"
	case BoardEpicGroups:
		return "Epic groups"
	}
	return "Epic lanes"
}

// ParseBoardEpicView reads a ui.board_epics config value. The design numbers
// are accepted beside the names because the board bar shows them.
func ParseBoardEpicView(s string) (BoardEpicView, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "lanes", "1":
		return BoardEpicLanes, true
	case "chips", "2":
		return BoardEpicChips, true
	case "groups", "3":
		return BoardEpicGroups, true
	}
	return BoardEpicLanes, false
}

// boardEpic is what the board knows about one epic. Done and Total count the
// issues under it, the epic itself excluded, over the whole project rather
// than the filtered board, so a filter never changes an epic's completion.
type boardEpic struct {
	ID    string
	Title string
	Done  int
	Total int
	// rank orders the bands: the most urgent open work first.
	rank  int
	color lipgloss.AdaptiveColor
}

// maxEpicDepth bounds the walk up the parent chain. Beads forbids cycles, but
// imported data can carry one, and the board must not hang on it.
const maxEpicDepth = 32

// epicPalette gives each epic a stable color. The hues avoid the status and
// priority reds so an epic bar never reads as a warning.
var epicPalette = []lipgloss.AdaptiveColor{
	{Light: "#00838f", Dark: "#4dd0e1"},
	{Light: "#6a1b9a", Dark: "#ce93d8"},
	{Light: "#2e7d32", Dark: "#81c784"},
	{Light: "#ef6c00", Dark: "#ffb74d"},
	{Light: "#283593", Dark: "#9fa8da"},
	{Light: "#ad1457", Dark: "#f48fb1"},
	{Light: "#558b2f", Dark: "#c5e1a5"},
	{Light: "#4e342e", Dark: "#bcaaa4"},
}

func epicColor(id string) lipgloss.AdaptiveColor {
	h := fnv.New32a()
	h.Write([]byte(id))
	return epicPalette[h.Sum32()%uint32(len(epicPalette))]
}

// EpicView returns the active epic design.
func (b *BoardModel) EpicView() BoardEpicView { return b.epicView }

// SetEpicView selects an epic design and keeps the selected issue.
func (b *BoardModel) SetEpicView(v BoardEpicView) {
	b.epicView = v
	b.rearrangeKeepingSelection()
}

// CycleEpicView moves to the next epic design.
func (b *BoardModel) CycleEpicView() {
	b.SetEpicView(BoardEpicView((int(b.epicView) + 1) % boardEpicViewCount))
}

// SetEpicUniverse gives the board every issue of the project, so epics and
// their completion resolve even when a filter hides a parent or a closed child.
func (b *BoardModel) SetEpicUniverse(issues []model.Issue) {
	b.epicUniverse = issues
	b.rebuildEpicIndex()
	b.rearrangeKeepingSelection()
}

// epicFor returns the nearest epic at or above id, or "" when there is none.
func (b *BoardModel) epicFor(id string) string { return b.epicOf[id] }

func (b *BoardModel) rebuildEpicIndex() {
	universe := b.epicUniverse
	if universe == nil {
		universe = b.allIssues
	}
	byID := make(map[string]*model.Issue, len(universe))
	parent := make(map[string]string, len(universe))
	for i := range universe {
		is := &universe[i]
		byID[is.ID] = is
		for _, dep := range is.Dependencies {
			if dep != nil && dep.Type == model.DepParentChild && dep.DependsOnID != "" {
				parent[is.ID] = dep.DependsOnID
				break
			}
		}
	}

	b.epicOf = make(map[string]string, len(universe))
	b.epics = make(map[string]*boardEpic)
	for _, is := range universe {
		epic := ""
		for id, depth := is.ID, 0; id != "" && depth < maxEpicDepth; id, depth = parent[id], depth+1 {
			if p, ok := byID[id]; ok && p.IssueType == model.TypeEpic {
				epic = id
				break
			}
		}
		if epic == "" {
			continue
		}
		b.epicOf[is.ID] = epic
		info := b.epics[epic]
		if info == nil {
			e := byID[epic]
			info = &boardEpic{ID: epic, Title: withoutOwnIDPrefix(sanitizeTerminalLine(e.Title), epic), rank: 99, color: epicColor(epic)}
			b.epics[epic] = info
		}
		if !isClosedLikeStatus(is.Status) && is.Priority < info.rank {
			info.rank = is.Priority
		}
		if is.ID == epic {
			continue
		}
		info.Total++
		if isClosedLikeStatus(is.Status) {
			info.Done++
		}
	}
}

// epicOrder returns the epic IDs by rank, then by ID.
func (b *BoardModel) epicOrder() map[string]int {
	ids := make([]string, 0, len(b.epics))
	for id := range b.epics {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ri, rj := b.epics[ids[i]].rank, b.epics[ids[j]].rank
		if ri != rj {
			return ri < rj
		}
		return ids[i] < ids[j]
	})
	order := make(map[string]int, len(ids))
	for i, id := range ids {
		order[id] = i
	}
	return order
}

// arrangeColumns derives b.columns from b.rawColumns for the active design.
// Chips keep the column order. Lanes and groups sort each column into epic
// bands, the epic's own row first, issues without an epic last. Lanes also
// drop the children of a folded epic.
func (b *BoardModel) arrangeColumns() {
	if b.epicView == BoardEpicChips || len(b.epics) == 0 {
		b.columns = b.rawColumns
		return
	}
	order := b.epicOrder()
	key := func(is model.Issue) (int, int) {
		epic := b.epicOf[is.ID]
		if epic == "" {
			return len(order), 0
		}
		if is.ID == epic {
			return order[epic], 0
		}
		return order[epic], 1
	}
	for col := range b.rawColumns {
		arranged := make([]model.Issue, 0, len(b.rawColumns[col]))
		for _, is := range b.rawColumns[col] {
			epic := b.epicOf[is.ID]
			if b.epicView == BoardEpicLanes && epic != "" && epic != is.ID && b.foldedEpics[epic] {
				continue
			}
			arranged = append(arranged, is)
		}
		sort.SliceStable(arranged, func(i, j int) bool {
			bi, si := key(arranged[i])
			bj, sj := key(arranged[j])
			if bi != bj {
				return bi < bj
			}
			return si < sj
		})
		b.columns[col] = arranged
	}
}

// setColumns stores freshly grouped columns and arranges them for the design.
func (b *BoardModel) setColumns(cols [4][]model.Issue) {
	b.rawColumns = cols
	b.rebuildEpicIndex()
	b.arrangeColumns()
}

func (b *BoardModel) rearrangeKeepingSelection() {
	var selID string
	if sel := b.SelectedIssue(); sel != nil {
		selID = sel.ID
	}
	b.arrangeColumns()
	b.clampSelection()
	b.updateActiveColumns()
	if selID != "" {
		b.SelectIssueByID(selID)
	}
}

func (b *BoardModel) clampSelection() {
	for i := 0; i < 4; i++ {
		if b.selectedRow[i] >= len(b.columns[i]) {
			b.selectedRow[i] = max(len(b.columns[i])-1, 0)
		}
	}
}

// ToggleEpicFold folds or unfolds the band of the selected issue and selects
// the epic row, since a folded band shows only that row. It does nothing
// outside the lanes design or for an issue without an epic.
func (b *BoardModel) ToggleEpicFold() bool {
	sel := b.SelectedIssue()
	if b.epicView != BoardEpicLanes || sel == nil {
		return false
	}
	epic := b.epicOf[sel.ID]
	if epic == "" {
		return false
	}
	if b.foldedEpics == nil {
		b.foldedEpics = map[string]bool{}
	}
	b.foldedEpics[epic] = !b.foldedEpics[epic]
	b.rearrangeKeepingSelection()
	if !b.SelectIssueByID(epic) {
		b.SelectIssueByID(sel.ID)
	}
	return true
}

// AnyEpicFolded reports whether at least one band is folded.
func (b *BoardModel) AnyEpicFolded() bool {
	for _, folded := range b.foldedEpics {
		if folded {
			return true
		}
	}
	return false
}

// ToggleAllEpicFolds unfolds every band when any is folded, and folds them
// all otherwise.
func (b *BoardModel) ToggleAllEpicFolds() {
	if b.epicView != BoardEpicLanes {
		return
	}
	var selEpic string
	if sel := b.SelectedIssue(); sel != nil {
		selEpic = b.epicOf[sel.ID]
	}
	if b.AnyEpicFolded() {
		b.foldedEpics = nil
		b.rearrangeKeepingSelection()
		return
	}
	b.foldedEpics = make(map[string]bool, len(b.epics))
	for id := range b.epics {
		b.foldedEpics[id] = true
	}
	b.rearrangeKeepingSelection()
	if selEpic != "" {
		b.SelectIssueByID(selEpic)
	}
}

// ── Rendering ────────────────────────────────────────────────────────────

// epicBand is one horizontal band in the lanes design, or one group in the
// groups design. The empty epic ID is the band of issues without an epic.
type epicBand struct {
	epic string
	rows []int // row indexes into the column
}

// bandsOf splits an arranged column into its consecutive epic runs.
func (b BoardModel) bandsOf(col int) []epicBand {
	var bands []epicBand
	for row, is := range b.columns[col] {
		epic := b.epicOf[is.ID]
		if n := len(bands); n == 0 || bands[n-1].epic != epic {
			bands = append(bands, epicBand{epic: epic})
		}
		bands[len(bands)-1].rows = append(bands[len(bands)-1].rows, row)
	}
	return bands
}

// hasEpicBands reports whether the board shows at least one epic, so a
// project without epics renders as plain layout A.
func (b BoardModel) hasEpicBands() bool {
	for _, col := range b.activeColIdx {
		for _, is := range b.columns[col] {
			if b.epicOf[is.ID] != "" {
				return true
			}
		}
	}
	return false
}

// progressBar draws done/total as width cells of heavy and light rule.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat("─", max(width, 0))
	}
	filled := done * width / total
	return strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
}

// epicHeader draws the full band header: fold marker, epic, title, progress.
//
//	▾ ◆ eg0 Stream capture pipeline  ━━──────  1/4
func (b BoardModel) epicHeader(epic string, width int, withMarker bool) string {
	t := b.theme
	muted := t.Renderer.NewStyle().Foreground(t.Secondary)
	if epic == "" {
		return padCells(muted.Bold(true).Render(" No epic"), width)
	}
	info := b.epics[epic]
	accent := t.Renderer.NewStyle().Foreground(info.color).Bold(true)
	marker := ""
	if withMarker {
		marker = "▾ "
		if b.foldedEpics[epic] {
			marker = "▸ "
		}
	}
	count := fmt.Sprintf("%d/%d", info.Done, info.Total)
	barW := 8
	if width < 50 {
		barW = 4
	}
	head := " " + marker + "◆ " + b.displayID(epic)
	titleW := width - lipgloss.Width(head) - barW - lipgloss.Width(count) - 6
	title := ""
	if titleW >= 4 {
		title = " " + truncateRunesHelper(info.Title, titleW, "…")
	}
	gap := width - lipgloss.Width(head) - lipgloss.Width(title) - barW - lipgloss.Width(count) - 3
	if gap < 1 {
		return padCells(accent.Render(head+title)+" "+muted.Render(count), width)
	}
	line := accent.Render(head) + t.Renderer.NewStyle().Foreground(t.Base.GetForeground()).Bold(true).Render(title) +
		strings.Repeat(" ", gap) + t.Renderer.NewStyle().Foreground(info.color).Render(progressBar(info.Done, info.Total, barW)) +
		"  " + muted.Render(count)
	return padCells(line, width)
}

// epicStub is the band header in the columns after the first: the epic and
// how many of its rows this column holds.
func (b BoardModel) epicStub(epic string, n, width int) string {
	t := b.theme
	muted := t.Renderer.NewStyle().Foreground(t.Secondary)
	if epic == "" {
		return padCells(muted.Render(fmt.Sprintf(" No epic · %d", n)), width)
	}
	accent := t.Renderer.NewStyle().Foreground(b.epics[epic].color)
	return padCells(accent.Render(" ◆ "+b.displayID(epic))+muted.Render(fmt.Sprintf(" · %d", n)), width)
}

// columnLines is a column body as plain lines, with the line span of each
// row and the band header that governs each line (for the sticky header).
type columnLines struct {
	lines     []string
	rowStart  map[int]int
	rowEnd    map[int]int
	headerFor []string // rendered header of the band each line sits in
	isHeader  []bool
}

func (c *columnLines) add(line, header string, isHeader bool) {
	c.lines = append(c.lines, line)
	c.headerFor = append(c.headerFor, header)
	c.isHeader = append(c.isHeader, isHeader)
}

// window picks avail lines so rows [selStart, selEnd] stay in view, and
// returns the span of c.lines it shows. When the view starts inside a band,
// that band's header replaces the first line.
func (c columnLines) window(selStart, selEnd, avail int) (lines []string, from, to int) {
	if len(c.lines) <= avail || selEnd < avail {
		to = min(avail, len(c.lines))
		return c.lines[:to], 0, to
	}
	start := selEnd - avail + 2
	if start > selStart {
		start = selStart
	}
	if start < 0 {
		start = 0
	}
	if c.isHeader[start] {
		to = min(start+avail, len(c.lines))
		return c.lines[start:to], start, to
	}
	to = min(start+avail-1, len(c.lines))
	return append([]string{c.headerFor[start]}, c.lines[start:to]...), start, to
}

// scrolled windows a column that overflows avail lines, keeping the last line
// for the position of the selection and the rows out of view, as the plain
// column does.
func (b BoardModel) scrolled(c columnLines, col, width, avail, selStart, selEnd int) []string {
	if len(c.lines) <= avail {
		return c.lines
	}
	out, from, to := c.window(selStart, selEnd, avail-1)
	hidden := 0
	for row, start := range c.rowStart {
		if start < from || c.rowEnd[row] >= to {
			hidden++
		}
	}
	pos := fmt.Sprintf(" %d/%d · %d more", b.selectedRow[col]+1, len(b.columns[col]), hidden)
	return append(fillLines(out, width, avail-1), padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Secondary).Italic(true).Render(truncateRunesHelper(pos, width, "…")), width))
}

func (b BoardModel) rowRule(width int) string {
	return padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Border).Render(strings.Repeat("┄", width)), width)
}

// columnHead is the column title and its rule.
func (b BoardModel) columnHead(col, width int) []string {
	return []string{b.columnHeader(col, width), padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Border).Render(strings.Repeat("─", width)), width)}
}

// lanesBody renders the lanes design: bands aligned across every full column
// and one scroll offset shared by all of them.
func (b BoardModel) lanesBody(width, height int, regions []boardRegion) [][]string {
	var bandOrder []string
	seen := map[string]bool{}
	bandRows := map[int]map[string][]int{}
	for _, r := range regions {
		if r.collapsed {
			continue
		}
		bandRows[r.col] = map[string][]int{}
		for _, band := range b.bandsOf(r.col) {
			bandRows[r.col][band.epic] = band.rows
			if !seen[band.epic] {
				seen[band.epic] = true
				bandOrder = append(bandOrder, band.epic)
			}
		}
	}
	order := b.epicOrder()
	sort.SliceStable(bandOrder, func(i, j int) bool {
		oi, oj := len(order), len(order)
		if bandOrder[i] != "" {
			oi = order[bandOrder[i]]
		}
		if bandOrder[j] != "" {
			oj = order[bandOrder[j]]
		}
		return oi < oj
	})

	firstFull := -1
	for _, r := range regions {
		if !r.collapsed {
			firstFull = r.col
			break
		}
	}
	focused := b.actualFocusedCol()
	built := map[int]columnLines{}
	for _, r := range regions {
		if r.collapsed || len(b.columns[r.col]) == 0 {
			continue
		}
		c := columnLines{rowStart: map[int]int{}, rowEnd: map[int]int{}}
		for _, epic := range bandOrder {
			bandH := 0
			for _, rows := range bandRows {
				if n := len(rows[epic]); n > 0 && n*3-1 > bandH {
					bandH = n*3 - 1
				}
			}
			var header string
			if r.col == firstFull {
				header = b.epicHeader(epic, r.width, true)
			} else {
				header = b.epicStub(epic, len(bandRows[r.col][epic]), r.width)
			}
			c.add(header, header, true)
			used := 0
			for i, row := range bandRows[r.col][epic] {
				if i > 0 {
					c.add(b.rowRule(r.width), header, false)
					used++
				}
				selected := r.col == focused && row == b.selectedRow[r.col]
				c.rowStart[row] = len(c.lines)
				for _, l := range b.renderRowLines(b.columns[r.col][row], r.width, selected, r.col, row) {
					c.add(l, header, false)
				}
				c.rowEnd[row] = len(c.lines) - 1
				used += 2
			}
			for ; used < bandH; used++ {
				c.add(strings.Repeat(" ", r.width), header, false)
			}
		}
		built[r.col] = c
	}

	// The shared offset follows the selection in the focused column.
	avail := height - 2
	fc := built[focused]
	selStart, selEnd := 0, 0
	if s, ok := fc.rowStart[b.selectedRow[focused]]; ok {
		selStart, selEnd = s, fc.rowEnd[b.selectedRow[focused]]
	}

	blocks := make([][]string, len(regions))
	for i, r := range regions {
		switch {
		case r.collapsed:
			blocks[i] = b.renderRail(r.col, r.width, height)
		case len(b.columns[r.col]) == 0:
			blocks[i] = b.renderColumn(r.col, r.width, height)
		default:
			out := append(b.columnHead(r.col, r.width), b.scrolled(built[r.col], r.col, r.width, avail, selStart, selEnd)...)
			blocks[i] = fillLines(out, r.width, height)
		}
	}
	return blocks
}

// renderGroupedColumn renders one column in the groups design: an epic
// subheader with progress above each run of rows.
func (b BoardModel) renderGroupedColumn(col, width, height int) []string {
	if len(b.columns[col]) == 0 {
		return b.renderColumn(col, width, height)
	}
	c := columnLines{rowStart: map[int]int{}, rowEnd: map[int]int{}}
	focused := col == b.actualFocusedCol()
	for bi, band := range b.bandsOf(col) {
		if bi > 0 {
			c.add(strings.Repeat(" ", width), "", false)
		}
		header := b.epicHeader(band.epic, width, false)
		c.add(header, header, true)
		for i, row := range band.rows {
			if i > 0 {
				c.add(b.rowRule(width), header, false)
			}
			c.rowStart[row] = len(c.lines)
			for _, l := range b.renderRowLines(b.columns[col][row], width, focused && row == b.selectedRow[col], col, row) {
				c.add(l, header, false)
			}
			c.rowEnd[row] = len(c.lines) - 1
		}
	}
	for i := range c.headerFor {
		if c.headerFor[i] == "" && i+1 < len(c.headerFor) {
			c.headerFor[i] = c.headerFor[i+1]
		}
	}
	sel := b.selectedRow[col]
	out := append(b.columnHead(col, width), b.scrolled(c, col, width, height-2, c.rowStart[sel], c.rowEnd[sel])...)
	return fillLines(out, width, height)
}

// epicChip is the line-2 tag of a row in the chips design. An epic's own row
// shows its completion instead of naming itself.
func (b BoardModel) epicChip(issue model.Issue, titleW int) (string, lipgloss.TerminalColor) {
	epic := b.epicOf[issue.ID]
	if epic == "" {
		return "", nil
	}
	info := b.epics[epic]
	if epic == issue.ID {
		return fmt.Sprintf("epic · %d/%d done", info.Done, info.Total), info.color
	}
	return "◆ " + b.displayID(epic) + " " + truncateRunesHelper(info.Title, titleW, "…"), info.color
}
