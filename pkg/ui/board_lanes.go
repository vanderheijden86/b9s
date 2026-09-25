package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// Epic lanes (docs/adr/0018). The board is cut into one horizontal lane per
// epic, aligned across every column. The rail design names the epic in a
// column left of the status columns; the rows design names it in a
// full-width row above its cards. The epic's own issue is not drawn as a
// card: selecting it highlights the lane's rail or row instead.

const (
	laneRailMin       = 12
	laneRailMax       = 26
	laneRailTitleMax  = 3
	cardTitleMaxLines = 2
)

// railWidth is the width of the epic rail, or 0 in the rows design.
func (b BoardModel) railWidth(width int) int {
	if b.epicView != BoardEpicRail {
		return 0
	}
	return min(max(width/6, laneRailMin), laneRailMax)
}

// progressBar draws done/total as width cells of heavy and light rule.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat("─", max(width, 0))
	}
	filled := done * width / total
	return strings.Repeat("━", filled) + strings.Repeat("─", width-filled)
}

// hasEpicLanes reports whether the board shows at least one epic, so a
// project without epics renders plain columns.
func (b BoardModel) hasEpicLanes() bool {
	for _, col := range b.activeColIdx {
		for _, is := range b.columns[col] {
			if b.epicOf[is.ID] != "" {
				return true
			}
		}
	}
	return false
}

// laneOrder lists the epics that hold an issue in a shown column, most
// urgent first, with "" (no epic) last when any issue has no epic.
func (b BoardModel) laneOrder(regions []boardRegion) []string {
	seen := map[string]bool{}
	var lanes []string
	for _, r := range regions {
		for _, is := range b.columns[r.col] {
			if epic := b.epicOf[is.ID]; !seen[epic] {
				seen[epic] = true
				lanes = append(lanes, epic)
			}
		}
	}
	order := b.epicOrder()
	rank := func(epic string) int {
		if epic == "" {
			return len(order)
		}
		return order[epic]
	}
	sort.SliceStable(lanes, func(i, j int) bool { return rank(lanes[i]) < rank(lanes[j]) })
	return lanes
}

// laneCount counts the issues of a lane in col, the epic's own issue left
// out. It reads rawColumns, so a folded lane still counts what it hides.
func (b BoardModel) laneCount(epic string, col int) int {
	n := 0
	for _, is := range b.rawColumns[col] {
		if b.epicOf[is.ID] == epic && is.ID != epic {
			n++
		}
	}
	return n
}

// boardLine is one line of the lanes body. lane is the index of the lane it
// belongs to, or -1 for the rule between lanes; laneHead marks the line that
// names the lane in the rows design.
type boardLine struct {
	cols     string
	lane     int
	laneHead bool
}

type boardLane struct {
	epic  string
	start int
	rail  []string
	head  string // the full-width row in the rows design
}

// lanesBody renders the epic lanes as exactly height lines of width cells:
// the column headers, a rule, then the lanes under one scroll offset that
// keeps the selection in view.
func (b BoardModel) lanesBody(width, height int) []string {
	t := b.theme
	border := t.Renderer.NewStyle().Foreground(t.Border)
	railW := b.railWidth(width)
	colsW := width
	if railW > 0 {
		colsW = width - railW - 2 // "│ " between the rail and the columns
	}
	regions := planBoardRegions(colsW, b.regionInputs(), b.actualFocusedCol(), adaptiveBreakpoints)
	focused := b.actualFocusedCol()
	sel := b.SelectedIssue()

	var lines []boardLine
	var lanes []boardLane
	type span struct{ start, end int }
	var cards []span
	selSpan := span{-1, -1}

	for li, epic := range b.laneOrder(regions) {
		folded := epic != "" && b.foldedEpics[epic]
		epicSelected := sel != nil && epic != "" && sel.ID == epic
		lane := boardLane{epic: epic, start: len(lines)}
		top := 0
		if railW > 0 {
			lane.rail = b.railLines(epic, railW, folded, epicSelected)
		} else {
			lane.head = b.laneRow(epic, width, folded, epicSelected)
			top = 1
		}

		cells := make([][]string, len(regions))
		cellSpans := make([]map[int]span, len(regions))
		laneH := len(lane.rail)
		for i, r := range regions {
			if folded && railW == 0 {
				break // a folded row is its header alone
			}
			muted := t.Renderer.NewStyle().Foreground(t.Secondary)
			switch {
			case r.collapsed:
				n := 0
				for _, is := range b.columns[r.col] {
					if b.epicOf[is.ID] == epic && is.ID != epic {
						n++
					}
				}
				cells[i] = []string{padCells(muted.Render(countOrDot(n, "")), r.width)}
			case folded:
				cells[i] = []string{padCells(muted.Render(countOrDot(b.laneCount(epic, r.col), " hidden")), r.width)}
			default:
				cellSpans[i] = map[int]span{}
				for row, is := range b.columns[r.col] {
					if b.epicOf[is.ID] != epic || is.ID == epic {
						continue
					}
					s := len(cells[i])
					selected := r.col == focused && row == b.selectedRow[r.col]
					cells[i] = append(cells[i], b.cardLines(is, r.width, selected, r.col, row)...)
					cellSpans[i][row] = span{s, len(cells[i]) - 1}
				}
			}
			laneH = max(laneH, len(cells[i]))
		}
		if railW > 0 {
			laneH = max(laneH, 1)
		}

		if top == 1 {
			lines = append(lines, boardLine{cols: lane.head, lane: li, laneHead: true})
		}
		for off := 0; off < laneH; off++ {
			parts := make([]string, len(regions))
			for i, r := range regions {
				if off < len(cells[i]) {
					parts[i] = cells[i][off]
				} else {
					parts[i] = strings.Repeat(" ", r.width)
				}
			}
			lines = append(lines, boardLine{cols: padCells(strings.Join(parts, " "), colsW), lane: li})
		}
		for i, r := range regions {
			for row, s := range cellSpans[i] {
				g := span{lane.start + top + s.start, lane.start + top + s.end}
				cards = append(cards, g)
				if r.col == focused && row == b.selectedRow[r.col] && sel != nil && b.columns[r.col][row].ID == sel.ID {
					selSpan = g
				}
			}
		}
		if epicSelected {
			selSpan = span{lane.start, lane.start + max(len(lane.rail), 1) - 1}
		}
		lanes = append(lanes, lane)
		if railW > 0 {
			lines = append(lines, boardLine{cols: border.Render(strings.Repeat("─", colsW)), lane: -1})
		}
	}
	if railW > 0 && len(lines) > 0 {
		lines = lines[:len(lines)-1] // no rule after the last lane
	}

	avail := height - 2
	from, to := 0, min(len(lines), avail)
	var pos string
	if len(lines) > avail {
		shown := max(avail-1, 1)
		from = windowStart(len(lines), shown, selSpan.start, selSpan.end)
		to = from + shown
		hidden := 0
		for _, c := range cards {
			if c.start < from || c.end >= to {
				hidden++
			}
		}
		col := b.actualFocusedCol()
		pos = fmt.Sprintf(" %d/%d · %d more", b.selectedRow[col]+1, len(b.columns[col]), hidden)
	}

	out := b.laneHeader(regions, railW, colsW, width)
	for i := from; i < to; i++ {
		l := lines[i]
		colsPart := l.cols
		// A lane cut by the top edge keeps its row in view (rows design).
		if i == from && from > 0 && railW == 0 && l.lane >= 0 && !l.laneHead && selSpan.start != from {
			colsPart = lanes[l.lane].head
		}
		if railW == 0 {
			out = append(out, padCells(colsPart, width))
			continue
		}
		if l.lane < 0 {
			out = append(out, border.Render(strings.Repeat("─", railW)+"┼─")+colsPart)
			continue
		}
		// The rail starts at the lane's first visible line, so a lane cut by
		// the top edge still names its epic.
		lane := lanes[l.lane]
		off := i - max(lane.start, from)
		railPart := strings.Repeat(" ", railW)
		if off < len(lane.rail) {
			railPart = lane.rail[off]
		}
		out = append(out, padCells(railPart+border.Render("│")+" "+colsPart, width))
	}
	if pos != "" {
		out = fillLines(out, width, height-1)
		out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).Render(truncateRunesHelper(pos, width, "…")), width))
	}
	return fillLines(out, width, height)
}

func issueCount(n int) string {
	if n == 1 {
		return "1 issue"
	}
	return fmt.Sprintf("%d issues", n)
}

func countOrDot(n int, suffix string) string {
	if n == 0 {
		return " ·"
	}
	return fmt.Sprintf(" %d%s", n, suffix)
}

// windowStart returns the first of shown lines out of total so that lines
// selStart through selEnd stay in view.
func windowStart(total, shown, selStart, selEnd int) int {
	start := 0
	if selEnd >= shown {
		start = selEnd - shown + 1
	}
	if selStart >= 0 && selStart < start {
		start = selStart
	}
	return max(min(start, total-shown), 0)
}

// laneHeader is the column header line and the rule under it.
func (b BoardModel) laneHeader(regions []boardRegion, railW, colsW, width int) []string {
	t := b.theme
	border := t.Renderer.NewStyle().Foreground(t.Border)
	parts := make([]string, len(regions))
	for i, r := range regions {
		if r.collapsed {
			title := t.Renderer.NewStyle().Foreground(b.columnColor(r.col)).Bold(true).
				Render(truncateRunesHelper(b.getColumnHeaders()[r.col], r.width-1, "…"))
			parts[i] = padCells(" "+title, r.width)
		} else {
			parts[i] = b.columnHeader(r.col, r.width)
		}
	}
	head := padCells(strings.Join(parts, " "), colsW)
	rule := border.Render(strings.Repeat("─", colsW))
	if railW == 0 {
		return []string{padCells(head, width), padCells(rule, width)}
	}
	epics := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render(" EPIC") +
		t.Renderer.NewStyle().Foreground(t.Secondary).Render(fmt.Sprintf("  %d", len(b.epics)))
	return []string{
		padCells(padCells(epics, railW)+border.Render("│")+" "+head, width),
		padCells(border.Render(strings.Repeat("─", railW)+"┼─")+rule, width),
	}
}

// laneIssueCount is the number of issues a lane holds across the shown
// columns, the epic's own issue left out.
func (b BoardModel) laneIssueCount(epic string) int {
	n := 0
	for _, col := range b.activeColIdx {
		n += b.laneCount(epic, col)
	}
	return n
}

// railLines draws the epic rail of one lane:
//
//	▌ ▾ ◆ eg0
//	▌ Stream capture
//	▌ pipeline
//	▌ ━━━━──────── 1/4
//	▌ 3 issues
func (b BoardModel) railLines(epic string, width int, folded, selected bool) []string {
	t := b.theme
	muted := b.fg(selected, t.Secondary)
	inner := max(width-3, 1) // the edge, a space each side
	edge := t.Renderer.NewStyle().Foreground(t.Border).Render("▌")
	var lines []string
	if epic == "" {
		lines = append(lines, " "+muted.Bold(true).Render(truncateRunesHelper("◇ No epic", inner, "…")))
	} else {
		info := b.epics[epic]
		edge = t.Renderer.NewStyle().Foreground(info.color).Render("▌")
		marker := "▾ "
		if folded {
			marker = "▸ "
		}
		lines = append(lines, " "+b.fg(selected, info.color).Bold(true).Render(truncateRunesHelper(marker+"◆ "+b.displayID(epic), inner, "…")))
		maxTitle := laneRailTitleMax
		if folded {
			maxTitle = 1
		}
		for _, l := range clampLines(wrapTitleLines(info.Title, inner), maxTitle, inner) {
			lines = append(lines, " "+b.fg(selected, t.Base.GetForeground()).Bold(true).Render(l))
		}
		count := fmt.Sprintf(" %d/%d", info.Done, info.Total)
		if barW := inner - lipgloss.Width(count); barW >= 3 {
			lines = append(lines, " "+b.fg(selected, info.color).Render(progressBar(info.Done, info.Total, barW))+muted.Render(count))
		} else {
			lines = append(lines, " "+muted.Render(strings.TrimSpace(count)))
		}
	}
	if !folded {
		lines = append(lines, " "+muted.Render(truncateRunesHelper(issueCount(b.laneIssueCount(epic)), inner, "…")))
	}
	bg := ""
	if selected {
		bg = bgSeqFromColor(t.Highlight, t.Renderer)
	}
	for i, l := range lines {
		lines[i] = edge + surface(l, width-1, bg)
	}
	return lines
}

// laneRow draws the full-width epic row of the rows design:
//
//	▌ ▾ ◆ eg0 Stream capture pipeline  3 issues          ━━━━──────── 1/4
func (b BoardModel) laneRow(epic string, width int, folded, selected bool) string {
	t := b.theme
	muted := b.fg(selected, t.Secondary)
	count := "  " + issueCount(b.laneIssueCount(epic))
	bg := ""
	if selected {
		bg = bgSeqFromColor(t.Highlight, t.Renderer)
	}
	if epic == "" {
		edge := t.Renderer.NewStyle().Foreground(t.Border).Render("▌")
		return edge + surface(" "+muted.Bold(true).Render("◇ No epic")+muted.Render(count), width-1, bg)
	}
	info := b.epics[epic]
	edge := t.Renderer.NewStyle().Foreground(info.color).Render("▌")
	marker := "▾ "
	if folded {
		marker = "▸ "
	}
	head := " " + marker + "◆ " + b.displayID(epic)
	done := fmt.Sprintf(" %d/%d", info.Done, info.Total)
	barW := 12
	right := b.fg(selected, info.color).Render(progressBar(info.Done, info.Total, barW)) + muted.Render(done)
	rightW := barW + lipgloss.Width(done) + 1
	titleW := width - 1 - lipgloss.Width(head) - 1 - lipgloss.Width(count) - rightW - 2
	if titleW < 8 {
		right, rightW = "", 0
		titleW = width - 1 - lipgloss.Width(head) - 1 - lipgloss.Width(count)
	}
	title := ""
	if titleW >= 4 {
		title = " " + truncateRunesHelper(info.Title, titleW, "…")
	}
	left := b.fg(selected, info.color).Bold(true).Render(head) +
		b.fg(selected, t.Base.GetForeground()).Bold(true).Render(title) + muted.Render(count)
	line := left
	if right != "" {
		if gap := width - 1 - lipgloss.Width(left) - rightW; gap > 0 {
			line = left + strings.Repeat(" ", gap) + right
		}
	}
	return edge + surface(line, width-1, bg)
}

// clampLines keeps at most n lines and marks a cut with an ellipsis on the
// last one kept.
func clampLines(lines []string, n, width int) []string {
	if len(lines) <= n {
		return lines
	}
	lines = append([]string(nil), lines[:n]...)
	last := lines[n-1]
	if lipgloss.Width(last)+1 > width {
		last = truncateRunesHelper(last, width-1, "")
	}
	lines[n-1] = last + "…"
	return lines
}

// cardLines draws one issue as a rounded box:
//
//	╭──────────────────────────────╮
//	│ Wire the flow subscription   │
//	│ transport                    │
//	│ blocked by eg0.4.1           │
//	│ ● eg0.4.2          P1 · 2d   │
//	╰──────────────────────────────╯
//
// The title takes at most two lines. The tag line appears only when the card
// waits on a dependency, sits in a dispatcher lane or blocks other work.
func (b BoardModel) cardLines(issue model.Issue, width int, selected bool, col, row int) []string {
	t := b.theme
	card := b.cardView(issue)
	inner := max(width-4, 1)
	edgeColor := lipgloss.TerminalColor(t.Border)
	idColor := lipgloss.TerminalColor(t.Primary)
	if b.IsSearchMatch(col, row) {
		edgeColor = lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#64b5f6"}
		idColor = edgeColor
	}
	if selected {
		edgeColor = t.Primary
	}
	edge := t.Renderer.NewStyle().Foreground(edgeColor)
	bg := b.rowSurface(selected, col, row)
	box := func(content string) string {
		return edge.Render("│") + surface(" "+padCells(content, inner)+" ", width-2, bg) + edge.Render("│")
	}

	lines := []string{edge.Render("╭" + strings.Repeat("─", max(width-2, 0)) + "╮")}
	for _, l := range clampLines(wrapTitleLines(card.Title, inner), cardTitleMaxLines, inner) {
		lines = append(lines, box(b.fg(selected, t.Base.GetForeground()).Bold(selected).Render(l)))
	}

	var tags []string
	used := 0
	add := func(text string, c lipgloss.TerminalColor) {
		sep := 0
		if len(tags) > 0 {
			sep = 2
		}
		if used+sep+lipgloss.Width(text) > inner {
			return
		}
		tags = append(tags, b.fg(selected, c).Render(text))
		used += sep + lipgloss.Width(text)
	}
	if card.BlockedBy != "" {
		add("blocked by "+card.BlockedBy, t.Blocked)
	}
	if card.LaneStage != "" {
		add("lane: "+card.LaneStage, t.InProgress)
	}
	if card.BlocksCount > 0 {
		add(fmt.Sprintf("blocks %d", card.BlocksCount), t.Feature)
	}
	if len(tags) > 0 {
		lines = append(lines, box(strings.Join(tags, "  ")))
	}

	icon, iconColor := t.GetTypeIcon(string(issue.IssueType))
	right := b.priorityStyle(issue, selected).Render(card.Priority) + b.fg(selected, t.Secondary).Render(" · ") +
		b.fg(selected, getAgeColor(issue.UpdatedAt)).Render(card.Age)
	rightW := lipgloss.Width(card.Priority + " · " + card.Age)
	idW := max(inner-lipgloss.Width(icon)-1-rightW-1, 4)
	left := b.fg(selected, iconColor).Render(icon) + " " +
		b.fg(selected, idColor).Bold(true).Render(truncateRunesHelper(card.ShortID, idW, "…"))
	footer := left
	if gap := inner - lipgloss.Width(left) - rightW; gap >= 1 {
		footer = left + strings.Repeat(" ", gap) + right
	}
	lines = append(lines, box(footer))
	lines = append(lines, edge.Render("╰"+strings.Repeat("─", max(width-2, 0))+"╯"))
	return lines
}
