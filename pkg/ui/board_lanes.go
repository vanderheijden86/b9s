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
func (b *BoardModel) railWidth(width int) int {
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
func (b *BoardModel) hasEpicLanes() bool {
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
func (b *BoardModel) laneOrder(regions []boardRegion) []string {
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
func (b *BoardModel) laneCount(epic string, col int) int {
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
	cells    []cellLine // one per region; drawn only when the line is in view
	lane     int
	laneHead bool
}

// cellLine is one line of a region: plain text, or line off of a card that is
// drawn on first use.
type cellLine struct {
	text string
	card *lazyCard
	off  int
}

type lazyCard struct {
	issue    model.Issue
	width    int
	selected bool
	col, row int
	lines    []string
}

func (b *BoardModel) cellText(c cellLine, width int) string {
	if c.card == nil {
		if c.text == "" {
			return strings.Repeat(" ", width)
		}
		return c.text
	}
	if c.card.lines == nil {
		k := c.card
		k.lines = b.cardLines(k.issue, k.width, k.selected, k.col, k.row)
	}
	return c.card.lines[c.off]
}

// colsText joins a line's region cells, drawing the cards it crosses.
func (b *BoardModel) colsText(l boardLine, regions []boardRegion, colsW int) string {
	if l.cells == nil {
		return l.cols
	}
	parts := make([]string, len(regions))
	for i, r := range regions {
		parts[i] = b.cellText(l.cells[i], r.width)
	}
	return padCells(strings.Join(parts, " "), colsW)
}

type boardLane struct {
	epic     string
	start    int
	height   int      // lines from start, the rows design's header included
	rail     []string // the epic cell's content, boxed by railBox
	selected bool
	head     []string // the full-width header box in the rows design
}

// lanesBody renders the epic lanes as exactly height lines of width cells:
// the column headers, a rule, then the lanes under one scroll offset that
// keeps the selection in view.
func (b *BoardModel) lanesBody(width, height int) []string {
	t := b.theme
	border := t.Renderer.NewStyle().Foreground(t.Border)
	railW := b.railWidth(width)
	colsW := width
	if railW > 0 {
		colsW = width - railW - 1 // a one-cell gap between the rail and the columns
	}
	regions := planBoardRegions(colsW, b.regionInputs(), b.actualFocusedCol(), adaptiveBreakpoints)
	focused := b.actualFocusedCol()
	if b.onEpicColumn {
		focused = -1 // no card is selected
	}

	var lines []boardLine
	var lanes []boardLane
	type span struct{ start, end int }
	var cards []span
	selSpan := span{-1, -1}

	for li, epic := range b.laneOrder(regions) {
		folded := epic != "" && b.foldedEpics[epic]
		epicSelected := b.onEpicColumn && b.epicColumnLane == epic
		lane := boardLane{epic: epic, start: len(lines), selected: epicSelected}
		top := 0
		if railW > 0 {
			lane.rail = b.railLines(epic, railW, folded, epicSelected)
		} else {
			lane.head = b.laneBox(epic, width, folded, epicSelected)
			top = len(lane.head)
		}

		cells := make([][]cellLine, len(regions))
		cellSpans := make([]map[int]span, len(regions))
		laneH := 0
		if railW > 0 {
			laneH = len(lane.rail) + 2 // the box's top and bottom edges
		}
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
				cells[i] = []cellLine{{text: padCells(muted.Render(countOrDot(n, "")), r.width)}}
			case folded:
				cells[i] = []cellLine{{text: padCells(muted.Render(countOrDot(b.laneCount(epic, r.col), " hidden")), r.width)}}
			default:
				cellSpans[i] = map[int]span{}
				for row, is := range b.columns[r.col] {
					if b.epicOf[is.ID] != epic || is.ID == epic {
						continue
					}
					s := len(cells[i])
					selected := r.col == focused && row == b.selectedRow[r.col]
					card := &lazyCard{issue: is, width: r.width, selected: selected, col: r.col, row: row}
					for off := range b.cardHeight(is, r.width) {
						cells[i] = append(cells[i], cellLine{card: card, off: off})
					}
					cellSpans[i][row] = span{s, len(cells[i]) - 1}
				}
			}
			laneH = max(laneH, len(cells[i]))
		}
		lane.height = top + laneH

		for _, h := range lane.head {
			lines = append(lines, boardLine{cols: h, lane: li, laneHead: true})
		}
		for off := 0; off < laneH; off++ {
			row := make([]cellLine, len(regions))
			for i := range regions {
				if off < len(cells[i]) {
					row[i] = cells[i][off]
				}
			}
			lines = append(lines, boardLine{cells: row, lane: li})
		}
		for i, r := range regions {
			for row, s := range cellSpans[i] {
				g := span{lane.start + top + s.start, lane.start + top + s.end}
				cards = append(cards, g)
				if r.col == focused && row == b.selectedRow[r.col] {
					selSpan = g
				}
			}
		}
		if epicSelected {
			selSpan = span{lane.start, lane.start + len(lane.head) - 1}
			if railW > 0 {
				selSpan.end = lane.start + len(lane.rail) + 1
			}
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
	boxes := map[int][]string{}
	for i := from; i < to; i++ {
		l := lines[i]
		colsPart := b.colsText(l, regions, colsW)
		// A lane cut by the top edge keeps its row in view (rows design).
		if i == from && from > 0 && railW == 0 && l.lane >= 0 && !l.laneHead && selSpan.start != from {
			colsPart = lanes[l.lane].head[1] // the header's content line
		}
		if railW == 0 {
			out = append(out, padCells(colsPart, width))
			continue
		}
		if l.lane < 0 {
			out = append(out, border.Render(strings.Repeat("─", railW+1))+colsPart)
			continue
		}
		// The cell is boxed over the lane's visible lines, so a lane cut by
		// the top edge still names its epic and the box always reaches the
		// lane rule or the bottom of the view.
		lane := lanes[l.lane]
		vis := max(lane.start, from)
		box, ok := boxes[l.lane]
		if !ok {
			box = b.railBox(lane, railW, min(lane.start+lane.height, to)-vis)
			boxes[l.lane] = box
		}
		railPart := strings.Repeat(" ", railW)
		if off := i - vis; off < len(box) {
			railPart = box[off]
		}
		out = append(out, padCells(railPart+" "+colsPart, width))
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
func (b *BoardModel) laneHeader(regions []boardRegion, railW, colsW, width int) []string {
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
		padCells(padCells(epics, railW)+" "+head, width),
		padCells(border.Render(strings.Repeat("─", railW+1))+rule, width),
	}
}

// laneIssueCount is the number of issues a lane holds across the shown
// columns, the epic's own issue left out.
func (b *BoardModel) laneIssueCount(epic string) int {
	n := 0
	for _, col := range b.activeColIdx {
		n += b.laneCount(epic, col)
	}
	return n
}

// railLines returns the content of one lane's epic cell, which railBox
// frames:
//
//	▾ ◆ eg0
//	Stream capture
//	pipeline
//	━━━━──────── 1/4
//	3 issues
func (b *BoardModel) railLines(epic string, width int, folded, selected bool) []string {
	t := b.theme
	muted := b.fg(selected, t.Secondary)
	inner := railInner(width)
	var lines []string
	if epic == "" {
		lines = append(lines, muted.Bold(true).Render(truncateRunesHelper("◇ No epic", inner, "…")))
	} else {
		info := b.epics[epic]
		marker := "▾ "
		if folded {
			marker = "▸ "
		}
		lines = append(lines, b.fg(selected, info.color).Bold(true).Render(truncateRunesHelper(marker+"◆ "+b.displayID(epic), inner, "…")))
		maxTitle := laneRailTitleMax
		if folded {
			maxTitle = 1
		}
		for _, l := range clampLines(wrapTitleLines(info.Title, inner), maxTitle, inner) {
			lines = append(lines, b.fg(selected, t.Base.GetForeground()).Bold(true).Render(l))
		}
		count := fmt.Sprintf(" %d/%d", info.Done, info.Total)
		if barW := inner - lipgloss.Width(count); barW >= 3 {
			lines = append(lines, b.fg(selected, info.color).Render(progressBar(info.Done, info.Total, barW))+muted.Render(count))
		} else {
			lines = append(lines, muted.Render(strings.TrimSpace(count)))
		}
	}
	if !folded {
		lines = append(lines, muted.Render(truncateRunesHelper(issueCount(b.laneIssueCount(epic)), inner, "…")))
	}
	return lines
}

// railInner is the content width of an epic cell: a side and a space each side.
func railInner(width int) int { return max(width-4, 1) }

// railBox frames a lane's epic cell as a box height lines tall, as a card is
// framed, so the cell runs the lane's full height. The left side carries the
// epic's color; a selected epic takes the selected card's border and fill.
// Below three lines there is no room for a box, and the content stands alone
// behind the colored side.
func (b *BoardModel) railBox(lane boardLane, width, height int) []string {
	if height <= 0 {
		return nil
	}
	side, edge, bg := b.epicFrame(lane.epic, lane.selected)
	inner := railInner(width)
	if height < 3 {
		var out []string
		for i := 0; i < height && i < len(lane.rail); i++ {
			out = append(out, side+surface(" "+padCells(lane.rail[i], inner), width-1, bg))
		}
		for len(out) < height {
			out = append(out, side+surface("", width-1, bg))
		}
		return out
	}
	content := make([]string, height-2)
	copy(content, lane.rail)
	return frameLines(content, width, side, edge, bg)
}

// epicFrame returns the pieces of an epic's box: the left side in the epic's
// color, the other edges in the border color, and the primary color with the
// selected fill when the epic is selected, as a selected card has.
func (b *BoardModel) epicFrame(epic string, selected bool) (side string, edge lipgloss.Style, bg string) {
	t := b.theme
	sideColor := lipgloss.TerminalColor(t.Border)
	if epic != "" {
		sideColor = b.epics[epic].color
	}
	edgeColor := lipgloss.TerminalColor(t.Border)
	if selected {
		sideColor, edgeColor = t.Primary, t.Primary
		bg = bgSeqFromColor(t.Highlight, t.Renderer)
	}
	return t.Renderer.NewStyle().Foreground(sideColor).Render("┃"), t.Renderer.NewStyle().Foreground(edgeColor), bg
}

// frameLines boxes content lines of width-4 cells into width cells.
func frameLines(content []string, width int, side string, edge lipgloss.Style, bg string) []string {
	inner := max(width-4, 1)
	out := []string{edge.Render("╭" + strings.Repeat("─", max(width-2, 0)) + "╮")}
	for _, c := range content {
		out = append(out, side+surface(" "+padCells(c, inner)+" ", width-2, bg)+edge.Render("│"))
	}
	return append(out, edge.Render("╰"+strings.Repeat("─", max(width-2, 0))+"╯"))
}

// laneBox draws the full-width epic header of the rows design as a box:
//
//	╭──────────────────────────────────────────────────────────────────────╮
//	┃ ▾ ◆ eg0 Stream capture pipeline  3 issues          ━━━━──────── 1/4 │
//	╰──────────────────────────────────────────────────────────────────────╯
func (b *BoardModel) laneBox(epic string, width int, folded, selected bool) []string {
	side, edge, bg := b.epicFrame(epic, selected)
	return frameLines([]string{b.laneRow(epic, max(width-4, 1), folded, selected)}, width, side, edge, bg)
}

// laneRow is the content line of an epic header box, width cells wide.
func (b *BoardModel) laneRow(epic string, width int, folded, selected bool) string {
	t := b.theme
	muted := b.fg(selected, t.Secondary)
	count := "  " + issueCount(b.laneIssueCount(epic))
	if epic == "" {
		return muted.Bold(true).Render("◇ No epic") + muted.Render(count)
	}
	info := b.epics[epic]
	marker := "▾ "
	if folded {
		marker = "▸ "
	}
	head := marker + "◆ " + b.displayID(epic)
	done := fmt.Sprintf(" %d/%d", info.Done, info.Total)
	barW := 12
	right := b.fg(selected, info.color).Render(progressBar(info.Done, info.Total, barW)) + muted.Render(done)
	rightW := barW + lipgloss.Width(done) + 1
	titleW := width - lipgloss.Width(head) - 1 - lipgloss.Width(count) - rightW - 2
	if titleW < 8 {
		right, rightW = "", 0
		titleW = width - lipgloss.Width(head) - 1 - lipgloss.Width(count)
	}
	title := ""
	if titleW >= 4 {
		title = " " + truncateRunesHelper(info.Title, titleW, "…")
	}
	left := b.fg(selected, info.color).Bold(true).Render(head) +
		b.fg(selected, t.Base.GetForeground()).Bold(true).Render(title) + muted.Render(count)
	line := left
	if right != "" {
		if gap := width - lipgloss.Width(left) - rightW; gap > 0 {
			line = left + strings.Repeat(" ", gap) + right
		}
	}
	return line
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

type cardTag struct {
	text  string
	color lipgloss.TerminalColor
}

// cardTags returns the tags that fit on one line of inner cells, in priority
// order: a later tag that does not fit is left out.
func (b *BoardModel) cardTags(card boardCardView, inner int) []cardTag {
	t := b.theme
	var tags []cardTag
	used := 0
	add := func(text string, c lipgloss.TerminalColor) {
		sep := 0
		if len(tags) > 0 {
			sep = 2
		}
		if used+sep+lipgloss.Width(text) > inner {
			return
		}
		tags = append(tags, cardTag{text, c})
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
	return tags
}

// cardHeight is the number of lines cardLines draws for issue, found without
// styling anything, so the lanes can be laid out before any card is drawn.
func (b *BoardModel) cardHeight(issue model.Issue, width int) int {
	card := b.cardView(issue)
	inner := max(width-4, 1)
	h := 3 + min(len(wrapTitleLines(card.Title, inner)), cardTitleMaxLines) // edges and footer
	if len(b.cardTags(card, inner)) > 0 {
		h++
	}
	return h
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
func (b *BoardModel) cardLines(issue model.Issue, width int, selected bool, col, row int) []string {
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
	for _, tag := range b.cardTags(card, inner) {
		tags = append(tags, b.fg(selected, tag.color).Render(tag.text))
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
