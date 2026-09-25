package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// FocusPopulatedColumn moves the focus off an empty column onto the first
// column that holds issues. The focused column is always shown in full, so an
// empty focus spends a full column on nothing.
func (b *BoardModel) FocusPopulatedColumn() {
	if len(b.columns[b.actualFocusedCol()]) > 0 {
		return
	}
	for i, col := range b.activeColIdx {
		if len(b.columns[col]) > 0 {
			b.focusedCol = i
			return
		}
	}
}

// View renders the board in the active epic design. Every line is at most
// width cells and the result is at most height lines.
func (b BoardModel) View(width, height int) string {
	if width < 20 || height < 4 {
		return ""
	}
	lines := append([]string{b.renderBoardBar(width)}, b.columnsBody(width, height-2)...)
	lines = append(lines, b.renderKeyHints(width))
	return strings.Join(lines, "\n")
}

func (b *BoardModel) renderBoardBar(width int) string {
	t := b.theme
	bold := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)
	muted := t.Renderer.NewStyle().Foreground(t.Secondary)

	// Counts come from the grouped columns, so a folded epic lane never makes
	// its issues look gone.
	headers := b.getColumnHeaders()
	var counts []string
	total, shown := 0, 0
	for col := 0; col < 4; col++ {
		if b.columnHidden(col) {
			continue
		}
		counts = append(counts, fmt.Sprintf("%s %d", strings.ToLower(headers[col]), len(b.rawColumns[col])))
		total += len(b.rawColumns[col])
		shown += len(b.columns[col])
	}
	summary := fmt.Sprintf("by %s · %d issues", strings.ToLower(b.GetSwimLaneModeName()), total)
	if folded := total - shown; folded > 0 {
		summary += fmt.Sprintf(" · %d folded", folded)
	}
	left := bold.Render("BOARD") + "  " + bold.Render(fmt.Sprintf("%s %d/%d", b.epicView.Label(), int(b.epicView)+1, boardEpicViewCount)) + "  " +
		muted.Render(summary)
	right := strings.Join(counts, " · ")
	if hidden := b.HiddenColumnCount(); hidden > 0 {
		right += fmt.Sprintf(" · +%d hidden", hidden)
	}
	if b.columnHidden(ColClosed) {
		right += fmt.Sprintf(" · closed %d hidden · c", len(b.rawColumns[ColClosed]))
	}
	return joinLeftRight(left, muted.Render(right), width)
}

func (b *BoardModel) renderKeyHints(width int) string {
	hints := "h/l column  j/k card  { } epic  tab fold  S-tab all  c closed  enter detail  / search  s swimlane  v design"
	return padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Secondary).Render(truncateRunesHelper(hints, width, "…")), width)
}

func (b *BoardModel) regionInputs() []boardRegionInput {
	var inputs []boardRegionInput
	for _, col := range b.activeColIdx {
		inputs = append(inputs, boardRegionInput{col: col, count: len(b.columns[col])})
	}
	return inputs
}

// columnsBody renders the board body as exactly height lines of exactly
// width cells: epic lanes when the board shows an epic, plain columns
// otherwise.
func (b *BoardModel) columnsBody(width, height int) []string {
	if b.hasEpicLanes() {
		return b.lanesBody(width, height)
	}
	regions := planBoardRegions(width, b.regionInputs(), b.actualFocusedCol(), adaptiveBreakpoints)
	blocks := make([][]string, len(regions))
	for i, r := range regions {
		if r.collapsed {
			blocks[i] = b.renderRail(r.col, r.width, height)
		} else {
			blocks[i] = b.renderColumn(r.col, r.width, height)
		}
	}
	out := make([]string, height)
	for line := range out {
		parts := make([]string, len(blocks))
		for i := range blocks {
			parts[i] = blocks[i][line]
		}
		out[line] = padCells(strings.Join(parts, " "), width)
	}
	return out
}

func (b *BoardModel) columnHeader(col, width int) string {
	t := b.theme
	focused := col == b.actualFocusedCol()
	title := b.getColumnHeaders()[col]
	issues := b.columns[col]
	stats := computeColumnStats(issues, b.issueMap)

	meta := []string{fmt.Sprintf("%d", len(issues))}
	if stats.P0Count > 0 {
		meta = append(meta, fmt.Sprintf("P0 %d", stats.P0Count))
	}
	if stats.P1Count > 0 {
		meta = append(meta, fmt.Sprintf("P1 %d", stats.P1Count))
	}
	if stats.BlockedCount > 0 && !(b.swimLaneMode == SwimByStatus && col == ColClosed) {
		meta = append(meta, fmt.Sprintf("waiting %d", stats.BlockedCount))
	}
	if stats.OldestAge > 0 && len(issues) > 0 {
		meta = append(meta, "oldest "+formatOldestAge(stats.OldestAge))
	}

	marker := "  "
	titleStyle := t.Renderer.NewStyle().Foreground(b.columnColor(col)).Bold(true)
	if focused {
		marker = "▸ "
		titleStyle = titleStyle.Foreground(t.Primary)
	}
	plainMeta := truncateRunesHelper(strings.Join(meta, " · "), width-len(marker)-lipgloss.Width(title)-2, "…")
	line := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render(marker) + titleStyle.Render(title)
	if plainMeta != "" {
		line += "  " + t.Renderer.NewStyle().Foreground(t.Secondary).Render(plainMeta)
	}
	return padCells(line, width)
}

func (b *BoardModel) columnColor(col int) lipgloss.TerminalColor {
	switch b.swimLaneMode {
	case SwimByPriority:
		return []lipgloss.AdaptiveColor{
			{Light: "#c62828", Dark: "#ef5350"},
			{Light: "#f57c00", Dark: "#ffb74d"},
			{Light: "#1565c0", Dark: "#64b5f6"},
			{Light: "#616161", Dark: "#9e9e9e"},
		}[col]
	case SwimByType:
		return []lipgloss.AdaptiveColor{
			{Light: "#c62828", Dark: "#ef5350"},
			{Light: "#2e7d32", Dark: "#81c784"},
			{Light: "#1565c0", Dark: "#64b5f6"},
			{Light: "#7b1fa2", Dark: "#ce93d8"},
		}[col]
	}
	return []lipgloss.AdaptiveColor{b.theme.Open, b.theme.InProgress, b.theme.Blocked, b.theme.Closed}[col]
}

// columnHead is the column title and its rule.
func (b *BoardModel) columnHead(col, width int) []string {
	return []string{b.columnHeader(col, width), padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Border).Render(strings.Repeat("─", width)), width)}
}

// renderColumn draws a full region: header, rule, then cards scrolled so the
// selection stays visible.
func (b *BoardModel) renderColumn(col, width, height int) []string {
	t := b.theme
	out := b.columnHead(col, width)
	issues := b.columns[col]
	avail := height - len(out)

	if len(issues) == 0 {
		out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).Render(truncateRunesHelper(" "+strings.Join(b.emptyColumnNote(col), " · "), width, "…")), width))
		return fillLines(out, width, height)
	}

	var lines []string
	starts, ends := make([]int, len(issues)), make([]int, len(issues))
	focusedCol := col == b.actualFocusedCol()
	sel := b.selectedRow[col]
	for row, issue := range issues {
		starts[row] = len(lines)
		lines = append(lines, b.cardLines(issue, width, focusedCol && row == sel, col, row)...)
		ends[row] = len(lines) - 1
	}
	if len(lines) <= avail {
		return fillLines(append(out, lines...), width, height)
	}
	shown := max(avail-1, 1)
	from := windowStart(len(lines), shown, starts[sel], ends[sel])
	out = append(out, lines[from:min(from+shown, len(lines))]...)
	hidden := 0
	for row := range issues {
		if starts[row] < from || ends[row] >= from+shown {
			hidden++
		}
	}
	more := fmt.Sprintf(" %d/%d · %d more", sel+1, len(issues), hidden)
	out = fillLines(out, width, height-1)
	out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).Render(truncateRunesHelper(more, width, "…")), width))
	return out
}

// emptyColumnNote explains an empty column in short phrases, one per rail
// line. An empty stored BLOCKED column is common while open issues wait on
// dependencies, so it names that count.
func (b *BoardModel) emptyColumnNote(col int) []string {
	if b.swimLaneMode == SwimByStatus && col == ColBlocked {
		if n := computeColumnStats(b.columns[ColOpen], b.issueMap).BlockedCount; n > 0 {
			return []string{"none stored", fmt.Sprintf("%d open", n), "wait on deps"}
		}
		return []string{"none stored"}
	}
	return []string{"empty"}
}

// renderRail draws a folded column: its name, its count and a short summary.
func (b *BoardModel) renderRail(col, width, height int) []string {
	t := b.theme
	muted := t.Renderer.NewStyle().Foreground(t.Secondary)
	issues := b.columns[col]
	title := t.Renderer.NewStyle().Foreground(b.columnColor(col)).Bold(true).
		Render(truncateRunesHelper(b.getColumnHeaders()[col], width-1, "…"))
	out := []string{padCells(" "+title, width), padCells(" "+muted.Render(fmt.Sprintf("%d", len(issues))), width)}
	note := func(s string) {
		out = append(out, padCells(" "+muted.Render(truncateRunesHelper(s, width-1, "…")), width))
	}

	switch {
	case len(issues) == 0:
		for _, s := range b.emptyColumnNote(col) {
			note(s)
		}
	case b.swimLaneMode == SwimByStatus && col == ColClosed:
		note(fmt.Sprintf("%d today", closedToday(issues)))
		note("")
		note("recent")
		for _, issue := range recentlyClosed(issues, height-len(out)-2) {
			note(b.displayID(sanitizeTerminalLine(issue.ID)))
		}
	default:
		for _, issue := range issues {
			if len(out) >= height-1 {
				break
			}
			note(b.displayID(sanitizeTerminalLine(issue.ID)))
		}
	}
	if len(out) < height {
		out = fillLines(out, width, height-1)
		out = append(out, padCells(" "+muted.Italic(true).Render(truncateRunesHelper("h/l opens", width-1, "")), width))
	}
	return fillLines(out, width, height)
}

func closedToday(issues []model.Issue) int {
	y, m, d := time.Now().Date()
	n := 0
	for _, issue := range issues {
		if at := closedAt(issue); !at.IsZero() {
			if ay, am, ad := at.Date(); ay == y && am == m && ad == d {
				n++
			}
		}
	}
	return n
}

func closedAt(issue model.Issue) time.Time {
	if issue.ClosedAt != nil {
		return *issue.ClosedAt
	}
	return issue.UpdatedAt
}

func recentlyClosed(issues []model.Issue, limit int) []model.Issue {
	if limit <= 0 {
		return nil
	}
	sorted := append([]model.Issue(nil), issues...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && closedAt(sorted[j]).After(closedAt(sorted[j-1])); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

// rowSurface returns the background sequence for a row: the selection, the
// current search match, or none.
func (b *BoardModel) rowSurface(selected bool, col, row int) string {
	t := b.theme
	switch {
	case selected:
		return bgSeqFromColor(t.Highlight, t.Renderer)
	case b.IsMatchHighlighted(col, row):
		return bgSeqFromColor(lipgloss.AdaptiveColor{Light: "#e1bee7", Dark: "#4a148c"}, t.Renderer)
	}
	return ""
}

func (b *BoardModel) fg(selected bool, c lipgloss.TerminalColor) lipgloss.Style {
	if selected {
		c = selectedCardTextColor
	}
	return b.theme.Renderer.NewStyle().Foreground(c)
}

func (b *BoardModel) priorityStyle(issue model.Issue, selected bool) lipgloss.Style {
	if issue.Priority <= 1 {
		return b.fg(selected, lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}).Bold(true)
	}
	return b.fg(selected, b.theme.Secondary)
}

// surface pads a styled line to width and, when bg is set, paints the whole
// line on that background even through the inner style resets.
func surface(line string, width int, bg string) string {
	line = padCells(line, width)
	if bg != "" {
		line = injectBackground(line, bg)
	}
	return line
}

// padCells clips a styled line to width cells and pads it with spaces to
// exactly width.
func padCells(s string, width int) string {
	if lipgloss.Width(s) > width {
		s = lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func fillLines(lines []string, width, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return lines
}

func joinLeftRight(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return padCells(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}
