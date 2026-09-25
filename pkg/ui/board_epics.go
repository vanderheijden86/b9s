package ui

import (
	"hash/fnv"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// BoardEpicView selects how the board lays out its epic lanes. Both keep the
// status columns and cut them into one horizontal lane per epic; v switches
// between them (docs/adr/0018).
type BoardEpicView int

const (
	// BoardEpicRail puts each epic in a rail left of the columns: its title
	// wrapped, its completion and its size, beside its cards.
	BoardEpicRail BoardEpicView = iota
	// BoardEpicRows gives each epic a full-width row above its cards.
	BoardEpicRows

	boardEpicViewCount = 2
)

func (v BoardEpicView) String() string {
	if v == BoardEpicRows {
		return "rows"
	}
	return "rail"
}

// Label is the name the board bar and the status line show.
func (v BoardEpicView) Label() string {
	if v == BoardEpicRows {
		return "Epic rows"
	}
	return "Epic rail"
}

// ParseBoardEpicView reads a ui.board_epics config value. The design numbers
// are accepted beside the names because the board bar shows them. Anything
// else, the retired lanes, chips and groups included, falls back to the rail.
func ParseBoardEpicView(s string) (BoardEpicView, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "rail", "1":
		return BoardEpicRail, true
	case "rows", "2":
		return BoardEpicRows, true
	}
	return BoardEpicRail, false
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

// OnEpicColumn reports whether the selection is in the epic column.
func (b *BoardModel) OnEpicColumn() bool { return b.onEpicColumn }

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

// arrangeColumns derives b.columns from b.rawColumns: each column sorted into
// epic lanes, the epic's own issue first, issues without an epic last, and
// the children of a folded epic left out.
func (b *BoardModel) arrangeColumns() {
	if len(b.epics) == 0 {
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
			if epic := b.epicOf[is.ID]; epic != "" && epic != is.ID && b.foldedEpics[epic] {
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

// epicOnBoard reports whether the epic's own issue sits in a shown column. A
// folded lane is reached through that issue, so without it a fold would leave
// nothing to select.
func (b *BoardModel) epicOnBoard(epic string) bool {
	for col := range b.rawColumns {
		if b.columnHidden(col) {
			continue
		}
		for _, is := range b.rawColumns[col] {
			if is.ID == epic {
				return true
			}
		}
	}
	return false
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

// ToggleEpicFold folds or unfolds the lane of the selected issue and selects
// the epic, since a folded lane shows only the epic. It does nothing for an
// issue without an epic or an epic whose own issue is not on the board.
func (b *BoardModel) ToggleEpicFold() bool {
	sel := b.SelectedIssue()
	if sel == nil {
		return false
	}
	epic := b.epicOf[sel.ID]
	if epic == "" || !b.epicOnBoard(epic) {
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

// AnyEpicFolded reports whether at least one lane is folded.
func (b *BoardModel) AnyEpicFolded() bool {
	for _, folded := range b.foldedEpics {
		if folded {
			return true
		}
	}
	return false
}

// ToggleAllEpicFolds unfolds every lane when any is folded, and otherwise
// folds every lane whose epic is on the board.
func (b *BoardModel) ToggleAllEpicFolds() {
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
		if b.epicOnBoard(id) {
			b.foldedEpics[id] = true
		}
	}
	b.rearrangeKeepingSelection()
	if selEpic != "" && b.foldedEpics[selEpic] {
		b.SelectIssueByID(selEpic)
	}
}

// The epic column is the first column of a lane board, and a lane runs
// across it and the status columns. Left and right move between the columns
// inside the selected lane, skipping a column where the lane has no card; up
// and down move inside the lane, or from lane to lane in the epic column.

// selectedLane returns the lane the selection is in.
func (b *BoardModel) selectedLane() string {
	if b.onEpicColumn {
		return b.epicColumnLane
	}
	if sel := b.selectedCard(); sel != nil {
		return b.epicOf[sel.ID]
	}
	return ""
}

// laneCardRow returns the row of col to select for lane: the column's own
// selection while it is a card of that lane, else the lane's first card, or
// -1 when the lane has no card in col.
func (b *BoardModel) laneCardRow(col int, lane string) int {
	cols := b.columns[col]
	if row := b.selectedRow[col]; row < len(cols) && b.isCardRow(col, row) && b.epicOf[cols[row].ID] == lane {
		return row
	}
	for row, is := range cols {
		if b.epicOf[is.ID] == lane && b.isCardRow(col, row) {
			return row
		}
	}
	return -1
}

// moveAcross selects the lane's card in the nearest shown column from index
// from in direction step. Moving left past the first column with a card
// selects the lane's epic.
func (b *BoardModel) moveAcross(from, step int) {
	lane := b.selectedLane()
	for i := from; i >= 0 && i < len(b.activeColIdx); i += step {
		col := b.activeColIdx[i]
		if row := b.laneCardRow(col, lane); row >= 0 {
			b.focusedCol = i
			b.selectedRow[col] = row
			b.onEpicColumn = false
			return
		}
	}
	if step < 0 {
		b.selectEpicColumn(lane)
	}
}

func (b *BoardModel) selectEpicColumn(lane string) {
	b.onEpicColumn = true
	b.epicColumnLane = lane
}

// stepEpicColumn selects the epic step lanes away, staying at either end.
func (b *BoardModel) stepEpicColumn(step int) {
	lanes := b.shownLanes()
	for i, lane := range lanes {
		if lane == b.epicColumnLane {
			b.epicColumnLane = lanes[max(min(i+step, len(lanes)-1), 0)]
			return
		}
	}
}

// shownLanes lists the lanes with an issue in a shown column, in lane order.
func (b *BoardModel) shownLanes() []string {
	regions := make([]boardRegion, 0, len(b.activeColIdx))
	for _, col := range b.activeColIdx {
		regions = append(regions, boardRegion{col: col})
	}
	return b.laneOrder(regions)
}

// leaveStaleEpicColumn drops the epic column selection once its lane is
// gone, so the selection falls back to the focused column's card.
func (b *BoardModel) leaveStaleEpicColumn() {
	if !b.onEpicColumn {
		return
	}
	if b.hasEpicLanes() {
		for _, lane := range b.shownLanes() {
			if lane == b.epicColumnLane {
				return
			}
		}
	}
	b.onEpicColumn = false
}

// epicIssue returns the issue of epic id, which may sit outside the filter.
func (b *BoardModel) epicIssue(id string) *model.Issue {
	if id == "" {
		return nil
	}
	if is := b.issueMap[id]; is != nil {
		return is
	}
	for i := range b.epicUniverse {
		if b.epicUniverse[i].ID == id {
			return &b.epicUniverse[i]
		}
	}
	return nil
}

// NextEpic selects the epic of the lane after the selection's lane.
func (b *BoardModel) NextEpic() {
	if !b.hasEpicLanes() {
		return
	}
	if !b.onEpicColumn {
		b.selectEpicColumn(b.selectedLane())
	}
	b.stepEpicColumn(1)
}

// PrevEpic selects the epic of the selection's lane, or the epic before it
// when that epic is already selected.
func (b *BoardModel) PrevEpic() {
	if !b.hasEpicLanes() {
		return
	}
	if !b.onEpicColumn {
		b.selectEpicColumn(b.selectedLane())
		return
	}
	b.stepEpicColumn(-1)
}
