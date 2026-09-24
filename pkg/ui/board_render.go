package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// Board layout methods.

// Layout returns the active board layout.
func (b *BoardModel) Layout() BoardLayoutKind { return b.layout }

// SetLayout selects a board layout. Leaving the inspector layout returns
// focus to the columns, since layout A has no inspector to hold it.
func (b *BoardModel) SetLayout(k BoardLayoutKind) {
	b.layout = k
	if k != BoardLayoutInspector {
		b.inspectorFocused = false
	}
}

// ToggleLayout switches between layout A and layout E.
func (b *BoardModel) ToggleLayout() { b.SetLayout(b.layout.other()) }

// IsInspectorFocused reports whether keys scroll the inspector instead of
// moving the selection.
func (b *BoardModel) IsInspectorFocused() bool {
	return b.layout == BoardLayoutInspector && b.inspectorFocused
}

// ToggleInspectorFocus moves focus between the columns and the inspector.
// It does nothing in layout A.
func (b *BoardModel) ToggleInspectorFocus() {
	if b.layout == BoardLayoutInspector {
		b.inspectorFocused = !b.inspectorFocused
	}
}

// ScrollInspector moves the inspector by delta lines for a board rendered at
// width x height, stopping at the first and last line.
func (b *BoardModel) ScrollInspector(delta, width, height int) {
	sel := b.SelectedIssue()
	if sel == nil {
		return
	}
	offset := b.inspectorOffset() + delta
	_, inspW, bodyH := inspectorGeometry(width, height)
	if maxOff := len(b.inspectorLines(*sel, inspW-2)) - bodyH; offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	b.inspectorID = sel.ID
	b.inspectorScroll = offset
}

// inspectorOffset is the scroll position for the selected issue. A new
// selection starts at the top.
func (b *BoardModel) inspectorOffset() int {
	if sel := b.SelectedIssue(); sel != nil && sel.ID == b.inspectorID {
		return b.inspectorScroll
	}
	return 0
}

// FocusPopulatedColumn moves the focus off an empty column onto the first
// column that holds issues. Both layouts give the focused column the most
// width, so an empty focus spends the board on nothing.
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

// View renders the board in the active layout. Every line is at most width
// cells and the result is at most height lines.
func (b BoardModel) View(width, height int) string {
	if width < 20 || height < 4 {
		return ""
	}
	bodyH := height - 2 // board bar above, key hints below
	var body []string
	if b.layout == BoardLayoutInspector {
		body = b.inspectorBody(width, height)
	} else {
		body = b.columnsBody(width, bodyH, false)
	}
	lines := append([]string{b.renderBoardBar(width)}, body...)
	lines = append(lines, b.renderKeyHints(width))
	return strings.Join(lines, "\n")
}

// inspectorGeometry splits the width between the compact board and the
// inspector, with one separator column between them.
func inspectorGeometry(width, height int) (boardW, inspW, bodyH int) {
	inspW = width * 38 / 100
	if inspW < 34 {
		inspW = 34
	}
	if inspW > 64 {
		inspW = 64
	}
	if inspW > width/2 {
		inspW = width / 2
	}
	return width - inspW - 1, inspW, height - 2
}

func (b BoardModel) inspectorBody(width, height int) []string {
	boardW, inspW, bodyH := inspectorGeometry(width, height)
	left := b.columnsBody(boardW, bodyH, true)
	right := b.renderInspector(inspW, bodyH)
	sepColor := b.theme.Border
	if b.inspectorFocused {
		sepColor = b.theme.Primary
	}
	sep := b.theme.Renderer.NewStyle().Foreground(sepColor).Render("│")
	out := make([]string, bodyH)
	for i := range out {
		out[i] = left[i] + sep + right[i]
	}
	return out
}

func (b BoardModel) renderBoardBar(width int) string {
	t := b.theme
	bold := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)
	muted := t.Renderer.NewStyle().Foreground(t.Secondary)

	headers := b.getColumnHeaders()
	var counts []string
	for col := 0; col < 4; col++ {
		counts = append(counts, fmt.Sprintf("%s %d", strings.ToLower(headers[col]), len(b.columns[col])))
	}
	left := bold.Render("BOARD") + "  " + bold.Render(b.layout.Label()) + "  " +
		muted.Render(fmt.Sprintf("by %s · %d issues", strings.ToLower(b.GetSwimLaneModeName()), b.TotalCount()))
	right := strings.Join(counts, " · ")
	if hidden := b.HiddenColumnCount(); hidden > 0 {
		right += fmt.Sprintf(" · +%d hidden", hidden)
	}
	if b.IsInspectorFocused() {
		right = "inspector focused · " + right
	}
	return joinLeftRight(left, muted.Render(right), width)
}

func (b BoardModel) renderKeyHints(width int) string {
	hints := "h/l column  j/k item  enter detail  s swimlane  / search  v layout " + b.layout.other().letter()
	if b.layout == BoardLayoutInspector {
		hints = "h/l column  j/k item  tab board/inspector  ctrl+j/k scroll  enter detail  v layout A"
	}
	return padCells(b.theme.Renderer.NewStyle().Foreground(b.theme.Secondary).Render(truncateRunesHelper(hints, width, "…")), width)
}

// columnsBody renders the status regions side by side as exactly height lines
// of exactly width cells.
func (b BoardModel) columnsBody(width, height int, compact bool) []string {
	var inputs []boardRegionInput
	for _, col := range b.activeColIdx {
		inputs = append(inputs, boardRegionInput{
			col:        col,
			count:      len(b.columns[col]),
			preferRail: b.swimLaneMode == SwimByStatus && col == ColClosed,
		})
	}
	bp := adaptiveBreakpoints
	if compact {
		bp = inspectorBreakpoints
	}
	regions := planBoardRegions(width, inputs, b.actualFocusedCol(), bp)

	blocks := make([][]string, len(regions))
	for i, r := range regions {
		if r.collapsed {
			blocks[i] = b.renderRail(r.col, r.width, height)
		} else {
			blocks[i] = b.renderColumn(r.col, r.width, height, compact)
		}
	}
	sep := b.theme.Renderer.NewStyle().Foreground(b.theme.Border).Render("│")
	out := make([]string, height)
	for line := range out {
		parts := make([]string, len(blocks))
		for i := range blocks {
			parts[i] = blocks[i][line]
		}
		out[line] = padCells(strings.Join(parts, sep), width)
	}
	return out
}

func (b BoardModel) columnHeader(col, width int, compact bool) string {
	t := b.theme
	focused := col == b.actualFocusedCol() && !b.IsInspectorFocused()
	title := b.getColumnHeaders()[col]
	issues := b.columns[col]
	stats := computeColumnStats(issues, b.issueMap)

	meta := []string{fmt.Sprintf("%d", len(issues))}
	if !compact {
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

func (b BoardModel) columnColor(col int) lipgloss.TerminalColor {
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

// renderColumn draws a full region: header, rule, then rows scrolled so the
// selection stays visible.
func (b BoardModel) renderColumn(col, width, height int, compact bool) []string {
	t := b.theme
	out := []string{b.columnHeader(col, width, compact), padCells(t.Renderer.NewStyle().Foreground(t.Border).Render(strings.Repeat("─", width)), width)}
	issues := b.columns[col]
	avail := height - len(out)

	if len(issues) == 0 {
		out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).Render(truncateRunesHelper(" "+strings.Join(b.emptyColumnNote(col), " · "), width, "…")), width))
		return fillLines(out, width, height)
	}

	rowH := 3 // two content lines and a separator
	if compact {
		rowH = 1
	}
	visible := avail / rowH
	if len(issues) > visible {
		visible = (avail - 1) / rowH // keep a line for the "more" hint
	}
	if visible < 1 {
		visible = 1
	}
	sel := b.selectedRow[col]
	start := 0
	if sel >= visible {
		start = sel - visible + 1
	}
	end := start + visible
	if end > len(issues) {
		end = len(issues)
	}
	focusedCol := col == b.actualFocusedCol()
	for row := start; row < end; row++ {
		selected := focusedCol && row == sel
		if compact {
			out = append(out, b.renderCompactRow(issues[row], width, selected, col, row))
			continue
		}
		out = append(out, b.renderRowLines(issues[row], width, selected, col, row)...)
		if row < end-1 {
			out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Border).Render(strings.Repeat("┄", width)), width))
		}
	}
	if hidden := len(issues) - (end - start); hidden > 0 {
		more := fmt.Sprintf(" %d/%d · %d more", sel+1, len(issues), hidden)
		out = append(out, padCells(t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).Render(truncateRunesHelper(more, width, "…")), width))
	}
	return fillLines(out, width, height)
}

// emptyColumnNote explains an empty column in short phrases, one per rail
// line. An empty stored BLOCKED column is common while open issues wait on
// dependencies, so it names that count.
func (b BoardModel) emptyColumnNote(col int) []string {
	if b.swimLaneMode == SwimByStatus && col == ColBlocked {
		if n := computeColumnStats(b.columns[ColOpen], b.issueMap).BlockedCount; n > 0 {
			return []string{"none stored", fmt.Sprintf("%d open", n), "wait on deps"}
		}
		return []string{"none stored"}
	}
	return []string{"empty"}
}

// renderRail draws a folded column: its name, its count and a short summary.
func (b BoardModel) renderRail(col, width, height int) []string {
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
func (b BoardModel) rowSurface(selected bool, col, row int) string {
	t := b.theme
	switch {
	case selected:
		return bgSeqFromColor(t.Highlight, t.Renderer)
	case b.IsMatchHighlighted(col, row):
		return bgSeqFromColor(lipgloss.AdaptiveColor{Light: "#e1bee7", Dark: "#4a148c"}, t.Renderer)
	}
	return ""
}

func (b BoardModel) fg(selected bool, c lipgloss.TerminalColor) lipgloss.Style {
	if selected {
		c = selectedCardTextColor
	}
	return b.theme.Renderer.NewStyle().Foreground(c)
}

func (b BoardModel) priorityStyle(issue model.Issue, selected bool) lipgloss.Style {
	if issue.Priority <= 1 {
		return b.fg(selected, lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}).Bold(true)
	}
	return b.fg(selected, b.theme.Secondary)
}

// renderRowLines draws the two lines of a layout A row:
//
//	[eg0.4.2]  Wire the flow subscription transport        P1  2d
//	task  blocked by eg0.4.1  lane: blocked  blocks 3
func (b BoardModel) renderRowLines(issue model.Issue, width int, selected bool, col, row int) []string {
	t := b.theme
	card := b.cardView(issue)
	idColor := lipgloss.TerminalColor(t.Primary)
	if b.IsSearchMatch(col, row) {
		idColor = lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#64b5f6"}
	}

	id := "[" + card.ShortID + "]"
	right := card.Priority + "  " + card.Age
	titleW := width - lipgloss.Width(id) - lipgloss.Width(right) - 5
	if titleW < 4 {
		titleW = 4
	}
	title := truncateRunesHelper(card.Title, titleW, "…")
	gap := width - 1 - lipgloss.Width(id) - 2 - lipgloss.Width(title) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	line1 := " " + b.fg(selected, idColor).Bold(true).Render(id) + "  " +
		b.fg(selected, t.Base.GetForeground()).Bold(selected).Render(title) + strings.Repeat(" ", gap) +
		b.priorityStyle(issue, selected).Render(card.Priority) + "  " +
		b.fg(selected, getAgeColor(issue.UpdatedAt)).Render(card.Age)

	icon, iconColor := t.GetTypeIcon(string(issue.IssueType))
	parts := []string{b.fg(selected, iconColor).Render(icon + " " + card.Type)}
	used := lipgloss.Width(icon + " " + card.Type)
	add := func(text string, c lipgloss.TerminalColor) {
		if used+2+lipgloss.Width(text) > width-2 {
			return
		}
		parts = append(parts, b.fg(selected, c).Render(text))
		used += 2 + lipgloss.Width(text)
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
	line2 := " " + strings.Join(parts, "  ")

	bg := b.rowSurface(selected, col, row)
	return []string{surface(line1, width, bg), surface(line2, width, bg)}
}

// renderCompactRow draws the one-line layout E row: short ID, title, priority.
func (b BoardModel) renderCompactRow(issue model.Issue, width int, selected bool, col, row int) string {
	t := b.theme
	card := b.cardView(issue)
	idColor := lipgloss.TerminalColor(t.Primary)
	if b.IsSearchMatch(col, row) {
		idColor = lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#64b5f6"}
	}
	marker := " "
	if card.BlockedBy != "" {
		marker = "⧗"
	}
	id := truncateRunesHelper(card.ShortID, width/3, "…")
	titleW := width - lipgloss.Width(id) - lipgloss.Width(card.Priority) - 6
	title := truncateRunesHelper(card.Title, titleW, "…")
	gap := width - 2 - lipgloss.Width(id) - 1 - lipgloss.Width(title) - lipgloss.Width(card.Priority) - 1
	if gap < 1 {
		gap = 1
	}
	line := b.fg(selected, t.Blocked).Render(marker) + " " + b.fg(selected, idColor).Bold(true).Render(id) + " " +
		b.fg(selected, t.Base.GetForeground()).Bold(selected).Render(title) + strings.Repeat(" ", gap) +
		b.priorityStyle(issue, selected).Render(card.Priority)
	return surface(line, width, b.rowSurface(selected, col, row))
}

// renderInspector draws the persistent detail panel of layout E.
func (b BoardModel) renderInspector(width, height int) []string {
	t := b.theme
	sel := b.SelectedIssue()
	if sel == nil {
		return fillLines([]string{padCells(" "+t.Renderer.NewStyle().Foreground(t.Secondary).Render("No issue selected"), width)}, width, height)
	}
	lines := b.inspectorLines(*sel, width-2)
	offset := b.inspectorOffset()
	if maxOff := len(lines) - height; offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	var out []string
	for i := offset; i < len(lines) && len(out) < height; i++ {
		out = append(out, padCells(" "+lines[i], width))
	}
	if offset+height < len(lines) && len(out) > 0 {
		out[len(out)-1] = padCells(" "+t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).
			Render(fmt.Sprintf("↓ %d more lines · ctrl+j", len(lines)-offset-height)), width)
	}
	return fillLines(out, width, height)
}

// inspectorLines builds the inspector content wrapped to width. The first
// lines mirror the mockup: type and priority, the ID, the title, then the
// status/lane/ready/impact facts the spec keeps apart.
func (b BoardModel) inspectorLines(raw model.Issue, width int) []string {
	t := b.theme
	if width < 10 {
		width = 10
	}
	issue := sanitizeIssueForTerminal(raw)
	card := b.cardView(raw)
	label := t.Renderer.NewStyle().Foreground(t.Secondary)
	value := t.Renderer.NewStyle().Foreground(t.Base.GetForeground())
	heading := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)

	var lines []string
	wrap := func(text string, style lipgloss.Style) {
		for _, l := range strings.Split(t.Renderer.NewStyle().Width(width).Render(text), "\n") {
			lines = append(lines, style.Render(strings.TrimRight(l, " ")))
		}
	}
	field := func(name, v string) {
		const labelW = 10
		prefix := label.Render(fmt.Sprintf("%-*s", labelW, name))
		for i, l := range strings.Split(t.Renderer.NewStyle().Width(width-labelW).Render(v), "\n") {
			if i > 0 {
				prefix = strings.Repeat(" ", labelW)
			}
			lines = append(lines, prefix+value.Render(strings.TrimRight(l, " ")))
		}
	}

	icon, iconColor := t.GetTypeIcon(string(issue.IssueType))
	head := t.Renderer.NewStyle().Foreground(iconColor).Bold(true).Render(icon+" "+strings.ToUpper(card.Type)) +
		label.Render(" · ") + b.priorityStyle(raw, false).Render(card.Priority)
	lines = append(lines, head, heading.Render("["+card.ShortID+"]"))
	wrap(card.Title, value.Bold(true))
	lines = append(lines, "")

	field("Status", strings.ToUpper(string(issue.Status)))
	lane := card.LaneStage
	if lane == "" {
		field("Lane", "none")
	} else {
		field("Lane", strings.ToUpper(lane))
	}
	switch {
	case isClosedLikeStatus(issue.Status):
		field("Ready", "closed")
	case card.BlockedBy != "":
		field("Ready", "no, blocked by "+card.BlockedBy)
	default:
		field("Ready", "yes")
	}
	blocks := b.blocksIndex[raw.ID]
	if len(blocks) == 0 {
		field("Impact", "blocks nothing")
	} else {
		var ids []string
		for _, id := range blocks {
			ids = append(ids, b.displayID(sanitizeTerminalLine(id)))
		}
		field("Impact", "blocks "+strings.Join(ids, ", "))
	}
	if issue.Assignee != "" {
		field("Assignee", "@"+issue.Assignee)
	}
	if issue.CreatedBy != "" {
		field("Creator", "@"+issue.CreatedBy)
	}

	if strings.TrimSpace(issue.Description) != "" {
		lines = append(lines, "")
		for _, para := range strings.Split(strings.TrimSpace(issue.Description), "\n") {
			wrap(para, value)
		}
	}

	lines = append(lines, "", heading.Render("Dependencies"))
	var deps int
	for _, dep := range issue.Dependencies {
		if dep == nil || !dep.Type.IsBlocking() {
			continue
		}
		deps++
		text := b.displayID(sanitizeTerminalLine(dep.DependsOnID))
		if blocker, ok := b.issueMap[dep.DependsOnID]; ok && blocker != nil {
			text += " " + strings.ToLower(string(blocker.Status)) + " · " + sanitizeTerminalLine(blocker.Title)
		}
		wrap("← "+text, value)
	}
	if deps == 0 {
		lines = append(lines, label.Render("none"))
	}

	lines = append(lines, "", heading.Render("Labels"))
	if len(issue.Labels) == 0 {
		lines = append(lines, label.Render("none"))
	} else {
		wrap(strings.Join(issue.Labels, " · "), value)
	}
	lines = append(lines, "")
	field("Created", FormatTimeRel(issue.CreatedAt))
	field("Updated", FormatTimeRel(issue.UpdatedAt))
	return lines
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
