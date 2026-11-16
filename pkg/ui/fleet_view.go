package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/vanderheijden86/b9s/internal/agents"
)

// fleetGraphWidth caps the commit graph column. Long-lived merge parents
// otherwise widen the graph until it pushes the subject off screen.
const fleetGraphWidth = 14

func (b FleetBrowser) style(c lipgloss.AdaptiveColor) lipgloss.Style {
	r := b.theme.Renderer
	if r == nil {
		return lipgloss.NewStyle().Foreground(c)
	}
	return r.NewStyle().Foreground(c)
}

func (b FleetBrowser) muted(s string) string { return b.style(b.theme.Muted).Render(s) }
func (b FleetBrowser) bold(s string) string  { return b.style(b.theme.Primary).Bold(true).Render(s) }

// fit truncates or pads s, which may hold ANSI styling, to exactly w cells.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func (b FleetBrowser) stateColor(s agents.State) lipgloss.AdaptiveColor {
	switch s {
	case agents.StateRunning:
		return b.theme.Open
	case agents.StateWaiting:
		return b.theme.InProgress
	case agents.StateFailed:
		return b.theme.Blocked
	case agents.StatePending:
		return b.theme.Deferred
	}
	return b.theme.Muted
}

func (b FleetBrowser) kindColor(k agents.EventKind) lipgloss.AdaptiveColor {
	switch k {
	case agents.EventTask, agents.EventUser:
		return b.theme.Primary
	case agents.EventAI:
		return b.theme.Open
	case agents.EventTool:
		return b.theme.Secondary
	case agents.EventBD:
		return b.theme.Epic
	case agents.EventCtx:
		return b.theme.Feature
	case agents.EventErr:
		return b.theme.Blocked
	}
	return b.theme.Muted
}

// bar draws a w-cell meter, coloured by how full it is.
func (b FleetBrowser) bar(pct, w int) string {
	filled := pct * w / 100
	if pct > 0 && filled == 0 {
		filled = 1
	}
	c := b.theme.Open
	switch {
	case pct >= 85:
		c = b.theme.Blocked
	case pct >= 60:
		c = b.theme.InProgress
	}
	return b.style(c).Render(strings.Repeat("█", filled)) + b.muted(strings.Repeat("░", w-filled))
}

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func spark(values []int) string {
	max := 0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var sb strings.Builder
	for _, v := range values {
		if max == 0 || v == 0 {
			sb.WriteRune(' ')
			continue
		}
		sb.WriteRune(sparkRunes[(v*(len(sparkRunes)-1)+max-1)/max])
	}
	return sb.String()
}

// View draws the whole screen.
func (b FleetBrowser) View() string {
	if b.width <= 0 || b.height <= 0 {
		return ""
	}
	full := b.view() == fleetLogs && b.fullscreen
	bodyH := b.height - 4
	if full {
		bodyH = b.height - 1
	}
	if bodyH < 1 {
		bodyH = 1
	}
	var body []string
	switch b.view() {
	case fleetAgents:
		body = b.agentsBody(bodyH)
	case fleetLogs:
		body = b.logsBody(bodyH)
	case fleetContext:
		body = b.contextBody(bodyH)
	case fleetXray:
		body = b.xrayBody(bodyH)
	case fleetPulse:
		body = b.pulseBody(bodyH)
	case fleetCommits:
		body = b.commitsBody(bodyH)
	case fleetCommit:
		body = b.commitBody(bodyH)
	case fleetText:
		body = b.textBody(bodyH)
	}
	for len(body) < bodyH {
		body = append(body, "")
	}
	var lines []string
	if full {
		lines = append(append(lines, body[:bodyH]...), b.statusLine())
	} else {
		lines = append([]string{b.header(), b.crumbs()}, body[:bodyH]...)
		lines = append(lines, b.hints(), b.statusLine())
	}
	for i, l := range lines {
		lines[i] = fit(l, b.width)
	}
	return strings.Join(lines, "\n")
}

func (b FleetBrowser) header() string {
	live, waiting, failed := 0, 0, 0
	for _, s := range b.scopedSessions() {
		if s.Live() {
			live++
		}
		switch s.State {
		case agents.StateWaiting:
			waiting++
		case agents.StateFailed:
			failed++
		}
	}
	left := b.bold(" b9s fleet ")
	if !b.scanned {
		return left + b.muted(" scanning transcripts…")
	}
	stats := fmt.Sprintf(" %d sessions · %d live · ", len(b.scopedSessions()), live) +
		b.style(b.theme.InProgress).Render(fmt.Sprintf("%d waiting", waiting)) + " · " +
		b.style(b.theme.Blocked).Render(fmt.Sprintf("%d failed", failed))
	if b.project != "" {
		stats += b.muted("  project:") + b.project
	} else {
		stats += b.muted("  all projects")
	}
	if b.snap.PodError != "" {
		stats += b.style(b.theme.Blocked).Render("  pods: " + b.snap.PodError)
	}
	return left + stats
}

func (b FleetBrowser) crumbs() string {
	var tabs []string
	root := b.stack[0]
	for _, v := range fleetRootViews {
		name := fleetViewNames[v]
		if v == root {
			tabs = append(tabs, b.style(b.theme.Highlight).Bold(true).Render("<"+name+">"))
		} else {
			tabs = append(tabs, b.muted(name))
		}
	}
	path := ""
	if len(b.stack) > 1 {
		parts := []string{}
		if s, ok := b.sessionByID(b.selected); ok && b.view() != fleetCommit {
			parts = append(parts, s.Name)
		}
		for _, v := range b.stack[1:] {
			parts = append(parts, fleetViewNames[v])
		}
		if b.view() == fleetCommit {
			parts = append(parts, b.detail.Commit.Short)
		}
		path = "  › " + strings.Join(parts, " › ")
	}
	return " " + strings.Join(tabs, " ") + path
}

func (b FleetBrowser) hints() string {
	var h string
	switch b.view() {
	case fleetAgents:
		h = "enter logs  c context  d describe  a attach  t task  ctrl+k stop  s sort  0-9 project  / filter  tab view  : command  q quit"
	case fleetLogs:
		h = "1 all 2 msgs 3 tools 4 errors 5 bd  s autoscroll  w wrap  t timestamps  f fullscreen  c copy  ctrl+s save  x expand  i inject  C context  T task  / search  esc back"
	case fleetContext:
		h = "enter fold/open  w who else loaded this  D diff since load  esc back"
	case fleetXray:
		h = "enter logs/task  c context  t task  tab view  esc quit"
	case fleetPulse:
		h = "tab view  : command  q quit"
	case fleetCommits:
		h = "enter detail  t issue  U author  b branches  0-9 project  c copy hash  r reload  / filter  esc quit"
	case fleetCommit:
		h = "j/k scroll diff  t issue  l agent log  c copy hash  esc back"
	case fleetText:
		h = "j/k scroll  esc back"
	}
	return b.muted(" " + h)
}

func (b FleetBrowser) statusLine() string {
	switch b.prompt {
	case fleetPromptFilter:
		return " /" + b.input + "█"
	case fleetPromptCommand:
		return " :" + b.input + "█" + b.muted("   agents xray pulse commits, or any b9s command")
	case fleetPromptInject:
		return " send> " + b.input + "█" + b.muted("   enter sends the text and Enter to the pane")
	}
	if b.status != "" {
		return " " + b.style(b.theme.InProgress).Render(b.status)
	}
	if b.filter != "" {
		return b.muted(" filter: ") + b.filter
	}
	return ""
}

// window returns the slice bounds that keep cursor visible in h rows.
func window(cursor, n, h int) (int, int) {
	if n <= h {
		return 0, n
	}
	start := cursor - h/2
	if start < 0 {
		start = 0
	}
	if start+h > n {
		start = n - h
	}
	return start, start + h
}

func (b FleetBrowser) selectedLine(s string, on bool) string {
	if !on {
		return " " + s
	}
	return b.style(b.theme.Highlight).Bold(true).Render("▌") + s
}

func (b FleetBrowser) agentsBody(h int) []string {
	list := b.visibleSessions()
	now := b.deps.Now()
	wide := b.width >= 130
	head := fmt.Sprintf("%-24s %-15s %-6s ", "NAME", "PROJECT", "TOOL")
	if wide {
		head += fmt.Sprintf("%-11s ", "ROLE")
	}
	head += fmt.Sprintf("%-17s %-8s %-15s %6s %3s %5s ", "TASK", "STATE", "CONTEXT", "TOK/m", "RST", "AGE")
	head += "WHERE"
	lines := []string{b.muted(" " + head)}
	if b.scanned && len(list) == 0 {
		return append(lines, b.muted("  no agent sessions in the last 48 hours"))
	}
	start, end := window(b.cursor, len(list), h-1)
	for i := start; i < end; i++ {
		s := list[i]
		task := s.TaskID
		if task == "" {
			task = "-"
		}
		where := s.Where.Label
		if s.Where.Kind == agents.WherePod {
			where = "pod " + where
		}
		row := fit(s.Name, 24) + " " + fit(s.Project, 15) + " " + fit(string(s.Tool), 6) + " "
		if wide {
			row += fit(s.Role, 11) + " "
		}
		row += fit(task, 17) + " " + b.style(b.stateColor(s.State)).Render(fit(string(s.State), 8)) + " "
		row += b.bar(s.ContextPercent(), 10) + fmt.Sprintf(" %3d%%", s.ContextPercent()) + " "
		row += fmt.Sprintf("%6d %3d %5s ", s.TokensPerMin, s.Compactions, fmtAge(now.Sub(s.Started)))
		row += b.muted(where)
		lines = append(lines, b.selectedLine(row, i == b.cursor))
	}
	return lines
}

func (b FleetBrowser) logsBody(h int) []string {
	s, _ := b.sessionByID(b.selected)
	evs := b.filteredEvents()
	title := fmt.Sprintf(" %s  %s  %s  %s  ctx %d%%  filter:%s", b.bold(s.Name), s.Project, s.TaskID, b.style(b.stateColor(s.State)).Render(string(s.State)), s.ContextPercent(), fleetLogFilters[b.logFilter].name)
	for _, on := range []struct {
		set  bool
		name string
	}{{b.follow, "following"}, {b.wrap, "wrap"}, {b.hideTime, "no-timestamps"}} {
		if on.set {
			title += b.style(b.theme.Open).Render("  " + on.name)
		}
	}
	lines := []string{title}
	if b.logErr != "" {
		lines = append(lines, b.style(b.theme.Blocked).Render(" "+b.logErr))
	}
	listH := h - len(lines)
	var detail []string
	if b.expanded && b.logCursor < len(evs) {
		e := evs[b.logCursor]
		text := e.Detail
		if text == "" {
			text = e.Text
		}
		detail = append(detail, b.muted(" ─── "+string(e.Kind)+" "+e.Tool+" "+e.Time.Local().Format("15:04:05")+" "+strings.Repeat("─", 20)))
		for _, ln := range strings.Split(text, "\n") {
			for _, w := range wrapCells(ln, b.width-2) {
				detail = append(detail, " "+w)
			}
		}
		if max := listH / 2; len(detail) > max {
			detail = append(detail[:max-1], b.muted(" …"))
		}
		listH -= len(detail)
	}
	rows := make([][]string, len(evs))
	heights := make([]int, len(evs))
	for i, e := range evs {
		rows[i] = b.logRow(e, i == b.logCursor)
		heights[i] = len(rows[i])
	}
	start, end := wrapWindow(heights, b.logCursor, listH)
	for i := start; i < end; i++ {
		lines = append(lines, rows[i]...)
	}
	if max := h - len(detail); len(lines) > max && max >= 0 {
		lines = lines[:max]
	}
	for len(lines) < h-len(detail) {
		lines = append(lines, "")
	}
	return append(lines, detail...)
}

// logRow draws one event as one screen row, or as several when wrap is on,
// with the continuation rows indented under the text column.
func (b FleetBrowser) logRow(e agents.Event, on bool) []string {
	prefix := ""
	if !b.hideTime {
		prefix = e.Time.Local().Format("15:04:05") + " "
	}
	prefix += b.style(b.kindColor(e.Kind)).Render(fit(string(e.Kind), 6)) + " " + b.muted(fit(e.Tool, 8)) + " "
	text := strings.ReplaceAll(e.Text, "\n", " ")
	if !b.wrap {
		return []string{b.selectedLine(prefix+text, on)}
	}
	indent := lipgloss.Width(prefix)
	parts := wrapCells(text, b.width-1-indent)
	out := []string{b.selectedLine(prefix+parts[0], on)}
	for _, p := range parts[1:] {
		out = append(out, b.selectedLine(strings.Repeat(" ", indent)+p, on))
	}
	return out
}

// wrapWindow picks the events [start, end) whose rows fill h screen rows
// around the cursor event: half the room above it, the rest below, and any
// room left at the bottom of the log goes back to the rows above.
func wrapWindow(heights []int, cursor, h int) (int, int) {
	if len(heights) == 0 || h <= 0 {
		return 0, 0
	}
	start, end := cursor, cursor+1
	used := heights[cursor]
	for start > 0 && used+heights[start-1] <= h/2 {
		start--
		used += heights[start]
	}
	for end < len(heights) && used+heights[end] <= h {
		used += heights[end]
		end++
	}
	for start > 0 && used+heights[start-1] <= h {
		start--
		used += heights[start]
	}
	return start, end
}

// wrapCells splits a line into pieces of at most w cells.
func wrapCells(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	r := []rune(strings.ReplaceAll(s, "\t", "    "))
	var out []string
	for len(r) > w {
		out = append(out, string(r[:w]))
		r = r[w:]
	}
	return append(out, string(r))
}

var layerGlyph = map[agents.Layer]string{
	agents.LayerInstructions: "I", agents.LayerMemories: "M", agents.LayerSkills: "S", agents.LayerTask: "T",
	agents.LayerFiles: "F", agents.LayerToolOutput: "O", agents.LayerConversation: "C", agents.LayerSystem: "Y",
}

func (b FleetBrowser) layerColor(l agents.Layer) lipgloss.AdaptiveColor {
	switch l {
	case agents.LayerInstructions:
		return b.theme.Primary
	case agents.LayerMemories:
		return b.theme.Epic
	case agents.LayerSkills:
		return b.theme.Feature
	case agents.LayerTask:
		return b.theme.InProgress
	case agents.LayerFiles:
		return b.theme.Open
	case agents.LayerToolOutput:
		return b.theme.Secondary
	case agents.LayerConversation:
		return b.theme.Chore
	}
	return b.theme.Muted
}

func (b FleetBrowser) contextBody(h int) []string {
	s, _ := b.sessionByID(b.selected)
	rep := b.ctx
	if rep.SessionID == "" {
		return []string{b.muted(" reading context…")}
	}
	pct := 0
	if rep.Limit > 0 {
		pct = rep.Window * 100 / rep.Limit
	}
	lines := []string{fmt.Sprintf(" %s  window %s of %s (%d%%)  compactions %d", b.bold(s.Name), fmtTokens(rep.Window), fmtTokens(rep.Limit), pct, s.Compactions)}
	// One stacked bar: each layer's share of the limit.
	barW := b.width - 4
	if barW > 100 {
		barW = 100
	}
	var sb strings.Builder
	used := 0
	for _, l := range rep.Layers {
		if rep.Limit == 0 || l.Tokens == 0 {
			continue
		}
		n := l.Tokens * barW / rep.Limit
		if n == 0 {
			n = 1
		}
		used += n
		sb.WriteString(b.style(b.layerColor(l.Layer)).Render(strings.Repeat(layerGlyph[l.Layer], n)))
	}
	if used < barW {
		sb.WriteString(b.muted(strings.Repeat("·", barW-used)))
	}
	lines = append(lines, " "+sb.String(), "")
	rows := b.contextRows()
	start, end := window(b.ctxCursor, len(rows), h-len(lines))
	for i := start; i < end; i++ {
		r := rows[i]
		var row string
		if r.source == nil {
			fold := "▾"
			if b.ctxFold[r.layer] {
				fold = "▸"
			}
			share := 0
			if rep.Window > 0 {
				share = r.tokens * 100 / rep.Window
			}
			row = b.style(b.layerColor(r.layer)).Bold(true).Render(fmt.Sprintf("%s %s %-18s", fold, layerGlyph[r.layer], r.layer)) + fmt.Sprintf(" %8s %3d%%", fmtTokens(r.tokens), share)
		} else {
			src := r.source
			loaded := ""
			if !src.LoadedAt.IsZero() {
				loaded = "loaded " + src.LoadedAt.Local().Format("15:04")
			}
			row = "    " + fit(src.Name, 46) + " " + b.muted(fit(src.Scope, 8)) + fmt.Sprintf(" %8s  ", fmtTokens(src.Tokens)) + b.muted(loaded)
			if src.Stale {
				row += b.style(b.theme.Blocked).Render("  ⚠ changed " + src.ChangedAt.Local().Format("15:04") + " after load")
			}
		}
		lines = append(lines, b.selectedLine(row, i == b.ctxCursor))
	}
	return lines
}

func (b FleetBrowser) xrayBody(h int) []string {
	rows := b.xrayRows()
	if len(rows) == 0 {
		if !b.scanned {
			return []string{b.muted(" scanning…")}
		}
		return []string{b.muted(" no live agent sessions")}
	}
	var lines []string
	start, end := window(b.xrayCursor, len(rows), h)
	for i := start; i < end; i++ {
		r := rows[i]
		var row string
		switch r.depth {
		case 0:
			row = b.bold("▾ "+r.project) + b.muted(fmt.Sprintf("  %d agents", r.count))
		case 1:
			if r.task == "" {
				row = "  ├─ " + b.style(b.theme.InProgress).Render("⚠ no task") + b.muted(fmt.Sprintf("  %d sessions work without a Beads task", r.count))
			} else {
				title, status := "", ""
				if b.deps.Issue != nil {
					title, status, _ = b.deps.Issue(r.task)
				}
				row = "  ├─ " + b.style(b.theme.Primary).Render(r.task) + " " + title + b.muted(" "+status)
			}
		case 2:
			s := r.session
			where := s.Where.Label
			if where == "" {
				where = "no pane"
			}
			branch := "├─ "
			if i+1 >= len(rows) || rows[i+1].depth != 2 {
				branch = "└─ "
			}
			row = "  │   " + branch + b.style(b.stateColor(s.State)).Render("● ") + fit(s.Name, 26) + " " + fit(string(s.Tool), 6) + " " + fit(s.Role, 11) + " " +
				b.style(b.stateColor(s.State)).Render(fit(string(s.State), 8)) + " " + b.bar(s.ContextPercent(), 8) + fmt.Sprintf(" %3d%% ", s.ContextPercent()) + b.muted(where)
		}
		lines = append(lines, b.selectedLine(row, i == b.xrayCursor))
	}
	return lines
}

func (b FleetBrowser) pulseBody(h int) []string {
	now := b.deps.Now()
	colW := (b.width - 3) / 2
	if b.width < 80 {
		colW = b.width - 2
	}
	sparkW := colW - 4
	tail := func(v []int) []int {
		if len(v) > sparkW && sparkW > 0 {
			return v[len(v)-sparkW:]
		}
		return v
	}
	counts := map[agents.State]int{}
	var failed, waiting, pressure []agents.Session
	tpm := 0
	for _, s := range b.scopedSessions() {
		counts[s.State]++
		if !s.Live() {
			continue
		}
		tpm += s.TokensPerMin
		switch s.State {
		case agents.StateFailed:
			failed = append(failed, s)
		case agents.StateWaiting:
			waiting = append(waiting, s)
		}
		pressure = append(pressure, s)
	}
	sortSessions(pressure, func(a, c agents.Session) bool { return a.ContextPercent() > c.ContextPercent() })
	sortSessions(waiting, func(a, c agents.Session) bool { return a.LastActivity.After(c.LastActivity) })

	closed := make([]int, 12)
	total := 0
	if b.deps.Closed != nil {
		for _, t := range b.deps.Closed() {
			d := now.Sub(t)
			if d < 0 || d >= 12*time.Hour {
				continue
			}
			closed[11-int(d/time.Hour)]++
			total++
		}
	}
	a := b.activity
	panel := func(title string, body ...string) []string {
		out := []string{b.bold(title)}
		return append(out, body...)
	}
	p1 := panel("Agents running",
		fmt.Sprintf("%s %d  %s %d  %s %d  %s %d",
			b.style(b.theme.Open).Render("running"), counts[agents.StateRunning],
			b.style(b.theme.InProgress).Render("waiting"), counts[agents.StateWaiting],
			b.muted("idle"), counts[agents.StateIdle],
			b.style(b.theme.Blocked).Render("failed"), counts[agents.StateFailed]),
		b.style(b.theme.Open).Render(spark(tail(a.Active))),
		b.muted("active sessions per 10 min, last 6 h"))
	p2 := panel("Tasks closed per hour", b.style(b.theme.Closed).Render(spark(closed)), b.muted(fmt.Sprintf("%d closed in the last 12 h (active project)", total)))
	p3 := panel("Tokens per minute", b.style(b.theme.Secondary).Render(spark(tail(a.Tokens))), b.muted(fmt.Sprintf("%d output tokens/min now across live sessions", tpm)))
	errSum := 0
	for _, e := range a.Errors {
		errSum += e
	}
	p4 := panel("Failures", b.style(b.theme.Blocked).Render(spark(tail(a.Errors))), b.muted(fmt.Sprintf("%d error events in 6 h", errSum)))
	for i, s := range failed {
		if i == 3 {
			break
		}
		p4 = append(p4, b.style(b.theme.Blocked).Render("✗ ")+s.Name+b.muted(" "+s.Project))
	}
	p5 := panel("Context pressure")
	for i, s := range pressure {
		if i == 5 {
			break
		}
		p5 = append(p5, fit(s.Name, colW-18)+" "+b.bar(s.ContextPercent(), 10)+fmt.Sprintf(" %3d%%", s.ContextPercent()))
	}
	p6 := panel("Waiting on you")
	for i, s := range waiting {
		if i == 5 {
			break
		}
		p6 = append(p6, b.style(b.theme.InProgress).Render("◷ ")+fit(s.Name, colW-24)+" "+b.muted(fmt.Sprintf("%-10s %5s", s.Project, fmtAge(now.Sub(s.LastActivity)))))
	}
	if len(waiting) == 0 {
		p6 = append(p6, b.muted("nobody waits"))
	}
	panels := [][]string{p1, p2, p3, p4, p5, p6}
	var lines []string
	if b.width < 80 {
		for _, p := range panels {
			lines = append(lines, p...)
			lines = append(lines, "")
		}
		return lines
	}
	for i := 0; i < len(panels); i += 2 {
		l, r := panels[i], panels[i+1]
		n := len(l)
		if len(r) > n {
			n = len(r)
		}
		for j := 0; j < n; j++ {
			var lc, rc string
			if j < len(l) {
				lc = l[j]
			}
			if j < len(r) {
				rc = r[j]
			}
			lines = append(lines, " "+fit(lc, colW)+" "+rc)
		}
		lines = append(lines, "")
	}
	return lines
}

func sortSessions(s []agents.Session, less func(a, c agents.Session) bool) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && less(s[j], s[j-1]); j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// commitDate follows the lgl layout: day/month/year with the time, or the
// time alone for today.
func commitDate(t, now time.Time) string {
	t, now = t.Local(), now.Local()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Format("15:04")
	}
	return t.Format("02/01/2006, 15:04")
}

func (b FleetBrowser) commitsBody(h int) []string {
	if b.commitErr != "" {
		return []string{b.style(b.theme.Blocked).Render(" " + b.commitErr)}
	}
	if !b.commitsRead {
		return []string{b.muted(" reading git log…")}
	}
	list := b.visibleCommits()
	now := b.deps.Now()
	graphW := 0
	for _, c := range list {
		if w := lipgloss.Width(c.Graph); w > graphW {
			graphW = w
		}
	}
	if graphW > fleetGraphWidth {
		graphW = fleetGraphWidth
	}
	head := " "
	if b.allRepos {
		head += fmt.Sprintf("%-14s ", "REPO")
	}
	head += fit("", graphW) + fmt.Sprintf(" %-8s %-17s %-14s ", "HASH", "DATE", "ISSUE")
	head += fmt.Sprintf("%-16s ", "BY") + "SUBJECT"
	lines := []string{b.muted(head)}
	start, end := window(b.commitCursor, len(list), h-1)
	for i := start; i < end; i++ {
		c := list[i]
		row := ""
		if b.allRepos {
			row += fit(c.Repo, 14) + " "
		}
		row += b.style(b.theme.Epic).Render(fit(c.Graph, graphW)) + " " + b.style(b.theme.InProgress).Render(c.Short) + " " + b.muted(fit(commitDate(c.Time, now), 17)) + " "
		switch {
		case len(c.Issues) > 0:
			issue := c.Issues[0]
			if len(c.Issues) > 1 {
				issue += fmt.Sprintf(" +%d", len(c.Issues)-1)
			}
			row += b.style(b.theme.Primary).Render(fit(issue, 14))
		case len(c.Parents) > 1:
			row += b.muted(fit("merge", 14))
		default:
			row += b.style(b.theme.Blocked).Render(fit("⚠ no Refs", 14))
		}
		row += " "
		// BY names the agent session that made the commit; U shows the git
		// author instead, which for agent work is the person who runs them.
		by := c.Author
		if !b.showAuthor {
			if ss := b.commitAgents(c); len(ss) > 0 {
				by = "◆ " + ss[0].Name
			}
		}
		row += fit(by, 16) + " "
		row += c.Subject
		if len(c.Refs) > 0 {
			row += " " + b.style(b.theme.Feature).Render("("+strings.Join(c.Refs, ", ")+")")
		}
		lines = append(lines, b.selectedLine(row, i == b.commitCursor))
	}
	return lines
}

func (b FleetBrowser) diffLine(s string) string {
	switch {
	case strings.HasPrefix(s, "+++"), strings.HasPrefix(s, "---"), strings.HasPrefix(s, "diff "), strings.HasPrefix(s, "index "):
		return b.bold(s)
	case strings.HasPrefix(s, "+"):
		return b.style(b.theme.Open).Render(s)
	case strings.HasPrefix(s, "-"):
		return b.style(b.theme.Blocked).Render(s)
	case strings.HasPrefix(s, "@@"):
		return b.style(b.theme.Secondary).Render(s)
	}
	return s
}

func (b FleetBrowser) commitBody(h int) []string {
	d := b.detail
	c := d.Commit
	leftW := b.width * 3 / 5
	rightW := b.width - leftW - 3
	if b.width < 100 {
		leftW, rightW = b.width-1, 0
	}
	var left []string
	left = append(left, b.style(b.theme.InProgress).Render(c.Short)+" "+b.bold(c.Subject))
	if b.detailErr != "" {
		left = append(left, b.style(b.theme.Blocked).Render(b.detailErr))
	}
	add, del := 0, 0
	for _, f := range d.Files {
		add += f.Added
		del += f.Deleted
	}
	left = append(left, b.muted(fmt.Sprintf("%d files  ", len(d.Files)))+b.style(b.theme.Open).Render(fmt.Sprintf("+%d", add))+" "+b.style(b.theme.Blocked).Render(fmt.Sprintf("-%d", del)))
	maxFiles := h / 3
	for i, f := range d.Files {
		if i == maxFiles {
			left = append(left, b.muted(fmt.Sprintf("… %d more files", len(d.Files)-i)))
			break
		}
		stat := b.style(b.theme.Open).Render(fmt.Sprintf("+%-4d", f.Added)) + b.style(b.theme.Blocked).Render(fmt.Sprintf("-%-4d", f.Deleted))
		if f.Binary {
			stat = b.muted("binary    ")
		}
		left = append(left, stat+" "+f.Path)
	}
	left = append(left, "")
	diff := strings.Split(d.Diff, "\n")
	for i := b.detailScroll; i < len(diff) && len(left) < h; i++ {
		left = append(left, b.diffLine(diff[i]))
	}
	if d.Truncated && len(left) < h {
		left = append(left, b.muted("… diff cut at 4000 lines"))
	}
	if rightW <= 0 {
		return left
	}
	var right []string
	right = append(right, b.bold("Commit"))
	right = append(right, b.muted("author ")+c.Author, b.muted("date   ")+c.Time.Local().Format("02/01/2006, 15:04"))
	if len(c.Refs) > 0 {
		right = append(right, b.muted("refs   ")+strings.Join(c.Refs, ", "))
	}
	for _, ln := range strings.Split(c.Body, "\n") {
		for _, w := range wrapCells(ln, rightW) {
			right = append(right, b.muted(w))
		}
	}
	right = append(right, "", b.bold("Issue"))
	if len(c.Issues) == 0 {
		right = append(right, b.style(b.theme.Blocked).Render("⚠ no Refs: trailer"))
	}
	for _, id := range c.Issues {
		line := b.style(b.theme.Primary).Render(id)
		if b.deps.Issue != nil {
			if title, status, ok := b.deps.Issue(id); ok {
				line += " " + title + b.muted(" "+status)
			}
		}
		right = append(right, line)
	}
	if len(b.detailIssue) > 0 {
		right = append(right, "", b.bold(fmt.Sprintf("Commits for %s", c.Issues[0])))
		for _, ic := range b.detailIssue {
			mark := "  "
			if ic.Hash == c.Hash {
				mark = "▸ "
			}
			right = append(right, mark+b.style(b.theme.InProgress).Render(ic.Short)+" "+ic.Subject)
		}
	}
	right = append(right, "", b.bold("Made by"))
	ss := b.commitAgents(c)
	if len(ss) == 0 {
		right = append(right, b.muted(c.Author+" (no agent session matches)"))
	}
	for _, s := range ss {
		right = append(right, b.style(b.stateColor(s.State)).Render("◆ ")+s.Name+b.muted(" "+string(s.Tool)+" "+s.Role))
	}
	var lines []string
	for i := 0; i < h; i++ {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, " "+fit(l, leftW)+b.muted(" │ ")+fit(r, rightW))
	}
	return lines
}

func (b FleetBrowser) textBody(h int) []string {
	lines := []string{" " + b.bold(b.textTitle)}
	if b.text == nil {
		return append(lines, b.muted(" reading…"))
	}
	for i := b.textScroll; i < len(b.text) && len(lines) < h; i++ {
		lines = append(lines, " "+b.diffLine(b.text[i]))
	}
	return lines
}
