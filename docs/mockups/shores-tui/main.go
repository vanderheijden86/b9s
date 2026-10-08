//go:build ignore

// Historical design study, retained as evidence for ADR 0038.
// The runnable native view is scripts/memory-preview b9s, key 2 (ADR 0042).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/vanderheijden86/b9s/internal/datasource"
)

var designs = []struct {
	name, title string
	render      func(m *model, w, h int) string
}{
	{"focus", "A · Focus wires: only the selected Issue's Links are drawn", renderFocus},
	{"matrix", "B · Matrix: Issues × decisions, no wires", renderMatrix},
	{"chips", "C · Chips: decisions inline on each Issue, detail on the right", renderChips},
	{"grouped", "D · Grouped: each decision with the work that follows it", renderGrouped},
	{"trunks", "E · Trunks: one lane per decision, everything else dimmed", renderTrunks},
}

func main() {
	design := flag.String("design", "focus", "focus | matrix | chips | grouped | trunks")
	width := flag.Int("w", 180, "frame width")
	height := flag.Int("h", 48, "frame height")
	data := flag.String("data", filepath.Join(os.TempDir(), "b9s-shores-graph.json"), "graph JSON cache")
	workspace := flag.String("workspace", filepath.Join(os.Getenv("HOME"), "Documents/b9s-memory-poc"), "graph workspace for the first read")
	flag.Parse()
	lipgloss.SetColorProfile(termenv.TrueColor)

	g, err := loadGraph(*data, *workspace)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shores-tui:", err)
		os.Exit(1)
	}
	m := newModel(g)
	for _, d := range designs {
		if d.name == *design {
			head := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render(d.title)
			fmt.Print(head + "\n\n" + d.render(m, *width, *height-3))
			return
		}
	}
	fmt.Fprintln(os.Stderr, "shores-tui: unknown design", *design)
	os.Exit(2)
}

type snapshot struct {
	Nodes []datasource.GraphNode
	Edges []datasource.GraphEdge
}

func loadGraph(path, workspace string) (datasource.MemoryGraph, error) {
	if raw, err := os.ReadFile(path); err == nil {
		var s snapshot
		if err := json.Unmarshal(raw, &s); err != nil {
			return datasource.MemoryGraph{}, err
		}
		return datasource.NewMemoryGraph(s.Nodes, s.Edges), nil
	}
	client, err := datasource.OpenGraphPreview(workspace)
	if err != nil {
		return datasource.MemoryGraph{}, err
	}
	g, err := client.Graph(context.Background(), nil)
	if err != nil {
		return datasource.MemoryGraph{}, err
	}
	raw, _ := json.Marshal(snapshot{g.Nodes, g.Edges})
	return g, os.WriteFile(path, raw, 0o644)
}

// ---- model ---------------------------------------------------------------

type issue struct {
	id, short, title string
	health           datasource.Health
	links            []link
}

type memory struct {
	id, num, title, status string
	followers, citers      []string
}

type link struct {
	issue, memory, kind, note string
}

type model struct {
	issues   []*issue
	memories []*memory // grouped: active, proposed, superseded, none
	byMem    map[string]*memory
	sel      *issue
	g        datasource.MemoryGraph
}

var statusOrder = map[string]int{"active": 0, "proposed": 1, "superseded": 2, "": 3}

func newModel(g datasource.MemoryGraph) *model {
	m := &model{byMem: map[string]*memory{}, g: g}
	for _, n := range g.Nodes {
		switch n.Kind {
		case "memory":
			num := n.Number
			if num == "" {
				num = short(n.ID)
			}
			mem := &memory{id: n.ID, num: num, title: memoryTitle(n.Title), status: n.Status}
			m.memories = append(m.memories, mem)
			m.byMem[n.ID] = mem
		case "issue":
			m.issues = append(m.issues, &issue{id: n.ID, short: short(n.ID), title: issueTitle(n.Title), health: g.Health(n.ID)})
		}
	}
	byIssue := map[string]*issue{}
	for _, is := range m.issues {
		byIssue[is.id] = is
	}
	for _, e := range g.Edges {
		if e.Kind != datasource.EdgeFollows && e.Kind != datasource.EdgeCites {
			continue
		}
		is, mem := byIssue[e.Source], m.byMem[e.Target]
		if is == nil || mem == nil {
			continue
		}
		is.links = append(is.links, link{is.id, mem.id, e.Kind, e.Note})
		if e.Kind == datasource.EdgeFollows {
			mem.followers = append(mem.followers, is.id)
		} else {
			mem.citers = append(mem.citers, is.id)
		}
	}
	sort.SliceStable(m.memories, func(i, j int) bool {
		a, b := m.memories[i], m.memories[j]
		if statusOrder[a.status] != statusOrder[b.status] {
			return statusOrder[a.status] < statusOrder[b.status]
		}
		return a.num < b.num
	})
	sort.SliceStable(m.issues, func(i, j int) bool { return m.issues[i].short < m.issues[j].short })
	// The selection is the Issue with the most decisions, preferring one with a problem,
	// so every mockup shows its busiest and most telling case.
	for _, is := range m.issues {
		if m.sel == nil || score(is) > score(m.sel) {
			m.sel = is
		}
	}
	return m
}

func score(is *issue) int {
	s := len(is.links) * 10
	if is.health.Problem() {
		s += 5
	}
	return s
}

// short drops the path and the workspace prefix: beads/sample-hffz.18 → hffz.18.
func short(id string) string {
	id = id[strings.LastIndex(id, "/")+1:]
	if strings.HasPrefix(id, "sample-") {
		return strings.TrimPrefix(id, "sample-")
	}
	return id
}

// issueTitle strips the bracketed ids and "Snapshot: " the POC import adds.
func issueTitle(t string) string {
	for {
		before := t
		t = strings.TrimPrefix(t, "Snapshot: ")
		if strings.HasPrefix(t, "[") {
			if i := strings.Index(t, "] "); i > 0 {
				t = t[i+2:]
			}
		}
		if t == before {
			return t
		}
	}
}

func memoryTitle(t string) string {
	if i := strings.Index(t, "): "); i > 0 {
		return t[i+3:]
	}
	if strings.HasPrefix(t, "[") {
		if i := strings.Index(t, "] "); i > 0 {
			return t[i+2:]
		}
	}
	return t
}

func (m *model) linked(memID string) (link, bool) {
	for _, l := range m.sel.links {
		if l.memory == memID {
			return l, true
		}
	}
	return link{}, false
}

// ---- styles --------------------------------------------------------------

var (
	cText     = lipgloss.Color("#c8ccd8")
	cDim      = lipgloss.Color("#5c6370")
	cFaint    = lipgloss.Color("#3a3f4b")
	cAccent   = lipgloss.Color("#56c8f0")
	cActive   = lipgloss.Color("#3ecf8e")
	cProposed = lipgloss.Color("#f0a742")
	cDead     = lipgloss.Color("#f0616d")
	cNone     = lipgloss.Color("#8a8f9c")
	cSelBg    = lipgloss.Color("#26323f")
)

func statusColor(s string) lipgloss.Color {
	switch s {
	case "active":
		return cActive
	case "proposed":
		return cProposed
	case "superseded":
		return cDead
	}
	return cNone
}

func statusName(s string) string {
	if s == "" {
		return "no status"
	}
	return s
}

func healthGlyph(h datasource.Health) string {
	switch h {
	case datasource.HealthDead:
		return fg(cDead, "✗")
	case datasource.HealthProposed:
		return fg(cProposed, "?")
	case datasource.HealthNone:
		return fg(cNone, "·")
	case datasource.HealthEpic:
		return fg(cDim, "◆")
	}
	return fg(cActive, "✓")
}

func fg(c lipgloss.Color, s string) string { return lipgloss.NewStyle().Foreground(c).Render(s) }
func bold(c lipgloss.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = ansi.Truncate(s, w, "…")
	if pad := w - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func clip(lines []string, h int) string {
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func issueRow(m *model, is *issue, w int, extra string) string {
	row := healthGlyph(is.health) + " " + fg(cDim, fit(is.short, 7)) + " "
	titleW := w - 10 - lipgloss.Width(extra)
	if is == m.sel {
		row = "▶" + row + bold(cText, fit(is.title, titleW))
	} else {
		row = " " + row + fg(cText, fit(is.title, titleW))
	}
	return row + extra
}

func memRow(mem *memory, w int, emphasis bool) string {
	dot := fg(statusColor(mem.status), "●")
	num := fit(mem.num, 6)
	if emphasis {
		return dot + " " + bold(statusColor(mem.status), num) + " " + bold(cText, fit(mem.title, w-9))
	}
	return dot + " " + fg(cDim, num) + " " + fg(cDim, fit(mem.title, w-9))
}

// memoryColumn lists Memories under status headers and returns the row of each.
func memoryColumn(m *model, w int, emphasise func(*memory) bool, suffix func(*memory) string) ([]string, map[string]int) {
	var lines []string
	rows := map[string]int{}
	last := "-"
	for _, mem := range m.memories {
		if mem.status != last {
			lines = append(lines, bold(statusColor(mem.status), statusName(mem.status)))
			last = mem.status
		}
		rows[mem.id] = len(lines)
		s := ""
		if suffix != nil {
			s = suffix(mem)
		}
		lines = append(lines, memRow(mem, w-lipgloss.Width(s), emphasise(mem))+s)
	}
	return lines, rows
}

func joinColumns(cols ...[]string) []string {
	h := 0
	for _, c := range cols {
		h = max(h, len(c))
	}
	out := make([]string, h)
	for i := range out {
		var b strings.Builder
		for ci, c := range cols {
			cell := ""
			if i < len(c) {
				cell = c[i]
			}
			if ci < len(cols)-1 {
				cell = fit(cell, colWidth(c))
			}
			b.WriteString(cell)
		}
		out[i] = b.String()
	}
	return out
}

func colWidth(c []string) int {
	w := 0
	for _, s := range c {
		w = max(w, lipgloss.Width(s))
	}
	return w
}

func pad(lines []string, w int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = fit(l, w)
	}
	return out
}

func footer(text string) string { return fg(cDim, text) }

// ---- A · focus wires -----------------------------------------------------

func renderFocus(m *model, w, h int) string {
	leftW, laneW := 62, 14
	rightW := w - leftW - laneW
	left := []string{bold(cText, "Work · Issues")}
	issueRowAt := map[string]int{}
	for _, is := range m.issues {
		issueRowAt[is.id] = len(left)
		chip := ""
		if n := len(is.links); n > 0 && is != m.sel {
			chip = fg(cDim, fmt.Sprintf(" %d→", n))
		}
		left = append(left, issueRow(m, is, leftW-1, chip))
	}
	right, memRowAt := memoryColumn(m, rightW, func(mem *memory) bool { _, ok := m.linked(mem.id); return ok }, nil)
	right = append([]string{bold(cText, "Knowledge · Memories")}, right...)
	for k := range memRowAt {
		memRowAt[k]++
	}

	rows := max(len(left), len(right))
	grid := make([][]string, rows)
	for i := range grid {
		grid[i] = make([]string, laneW)
		for j := range grid[i] {
			grid[i][j] = " "
		}
	}
	y0 := issueRowAt[m.sel.id]
	lo, hi := y0, y0
	targets := map[int]link{}
	for _, l := range m.sel.links {
		y := memRowAt[l.memory]
		targets[y] = l
		lo, hi = min(lo, y), max(hi, y)
	}
	trunk := 2
	for y := lo; y <= hi; y++ {
		up, down := y > lo, y < hi
		_, right := targets[y]
		left := y == y0
		color := cAccent
		if l, ok := targets[y]; ok {
			color = statusColor(m.byMem[l.memory].status)
		}
		grid[y][trunk] = fg(cAccent, junction(up, down, left, right))
		if left {
			for x := 0; x < trunk; x++ {
				grid[y][x] = fg(cAccent, "─")
			}
		}
		if l, ok := targets[y]; ok {
			seg := "─"
			if l.kind == datasource.EdgeCites {
				seg = "┄"
			}
			for x := trunk + 1; x < laneW-2; x++ {
				grid[y][x] = fg(color, seg)
			}
			grid[y][laneW-2] = fg(color, "▶")
		}
	}
	lanes := make([]string, rows)
	for i := range grid {
		lanes[i] = strings.Join(grid[i], "")
	}
	body := joinColumns(pad(left, leftW), lanes, right)
	var detail []string
	detail = append(detail, "", bold(cText, m.sel.short+" · "+m.sel.title))
	for _, l := range m.sel.links {
		mem := m.byMem[l.memory]
		kind := "follows"
		if l.kind == datasource.EdgeCites {
			kind = "cites  "
		}
		detail = append(detail, "  "+fg(cDim, kind)+" "+bold(statusColor(mem.status), mem.num)+" "+fg(cText, mem.title)+"  "+fg(cDim, l.note))
	}
	body = append(body, pad(detail, w)...)
	body = append(body, "", footer("j/k Issue · tab Memory side (wires follow the cursor) · enter open · f follows only · p problems only · q quit"))
	return clip(body, h)
}

func junction(up, down, left, right bool) string {
	switch {
	case up && down && left && right:
		return "┼"
	case up && down && left:
		return "┤"
	case up && down && right:
		return "├"
	case up && down:
		return "│"
	case down && left && right:
		return "┬"
	case up && left && right:
		return "┴"
	case down && left:
		return "╮"
	case down && right:
		return "╭"
	case up && left:
		return "╯"
	case up && right:
		return "╰"
	}
	return "─"
}

// ---- B · matrix ----------------------------------------------------------

func renderMatrix(m *model, w, h int) string {
	var cols []*memory
	for _, mem := range m.memories {
		if len(mem.followers)+len(mem.citers) > 0 {
			cols = append(cols, mem)
		}
	}
	leftW := 58
	cell := 3
	label := func(mem *memory) string {
		if len(mem.num) == 4 && strings.HasPrefix(mem.num, "00") {
			return mem.num[2:]
		}
		parts := strings.Split(mem.num, "-")
		s := ""
		for _, p := range parts {
			s += p[:1]
		}
		return s
	}
	selCol := -1
	for i, mem := range cols {
		if _, ok := m.linked(mem.id); ok && selCol < 0 {
			selCol = i
		}
	}
	var head1, head2 strings.Builder
	head1.WriteString(fit(bold(cText, "Work · Issues"), leftW))
	head2.WriteString(strings.Repeat(" ", leftW))
	for i, mem := range cols {
		st := lipgloss.NewStyle().Foreground(statusColor(mem.status)).Bold(i == selCol)
		head1.WriteString(st.Render(fit(" "+label(mem), cell)))
		head2.WriteString(fg(statusColor(mem.status), " ━ "[:len(" ━ ")]))
	}
	lines := []string{head1.String(), head2.String()}
	for _, is := range m.issues {
		row := fit(issueRow(m, is, leftW-1, ""), leftW)
		for i, mem := range cols {
			mark := fg(cFaint, " · ")
			for _, l := range is.links {
				if l.memory == mem.id {
					c := statusColor(mem.status)
					if l.kind == datasource.EdgeFollows {
						mark = fg(c, " ● ")
					} else {
						mark = fg(c, " ○ ")
					}
				}
			}
			if is == m.sel && i == selCol {
				mark = lipgloss.NewStyle().Background(cSelBg).Render(mark)
			}
			row += mark
		}
		if is == m.sel {
			row = lipgloss.NewStyle().Background(cSelBg).Render(row)
		}
		lines = append(lines, row)
	}
	mem := cols[selCol]
	l, _ := m.linked(mem.id)
	kind := " follows "
	if l.kind == datasource.EdgeCites {
		kind = " cites "
	}
	lines = append(lines, "",
		bold(cText, "cell  ")+m.sel.short+fg(cDim, kind)+bold(statusColor(mem.status), mem.num+" "+mem.title)+fg(cDim, "  ("+statusName(mem.status)+")"),
		fg(cDim, fmt.Sprintf("      column: followed by %d, cited by %d", len(mem.followers), len(mem.citers))),
		"",
		fg(cDim, "legend  ")+fg(cActive, "● follows")+"  "+fg(cActive, "○ cites")+"  "+fg(cActive, "━ active")+" "+fg(cProposed, "━ proposed")+" "+fg(cDead, "━ superseded")+" "+fg(cNone, "━ no status"),
		"", footer("hjkl move cell · enter open · s sort columns by use · p problems only · q quit"))
	return clip(lines, h)
}

// ---- C · chips -----------------------------------------------------------

func chip(mem *memory, kind string) string {
	c := statusColor(mem.status)
	text := mem.num
	if mem.status == "superseded" {
		text += "✗"
	}
	if mem.status == "proposed" {
		text += "?"
	}
	if kind == datasource.EdgeFollows {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#15171c")).Background(c).Render(" " + text + " ")
	}
	return fg(c, "("+text+")")
}

func renderChips(m *model, w, h int) string {
	leftW := 104
	rightW := w - leftW - 2
	var left []string
	left = append(left, bold(cText, "Work · Issues")+fg(cDim, "                                                  decisions (filled = follows, (n) = cites)"))
	for _, is := range m.issues {
		var chips []string
		for _, l := range is.links {
			chips = append(chips, chip(m.byMem[l.memory], l.kind))
		}
		extra := strings.Join(chips, " ")
		if len(chips) == 0 && is.health == datasource.HealthNone {
			extra = fg(cNone, "no decision")
		}
		extra = fit(extra, 36)
		row := issueRow(m, is, leftW-2, " "+extra)
		if is == m.sel {
			row = lipgloss.NewStyle().Background(cSelBg).Render(row)
		}
		left = append(left, row)
	}
	var right []string
	right = append(right, bold(cText, m.sel.short), fg(cText, m.sel.title), fg(cDim, m.sel.health.String()), "")
	for _, l := range m.sel.links {
		mem := m.byMem[l.memory]
		kind := "follows"
		if l.kind == datasource.EdgeCites {
			kind = "cites"
		}
		right = append(right, chip(mem, l.kind)+" "+fg(cDim, kind+" · "+statusName(mem.status)))
		right = append(right, "  "+fg(cText, ansi.Wrap(mem.title, rightW-2, "")))
		if l.note != "" {
			right = append(right, "  "+fg(cDim, "“"+l.note+"”"))
		}
		var others []string
		for _, f := range append(append([]string{}, mem.followers...), mem.citers...) {
			if f != m.sel.id {
				others = append(others, short(f))
			}
		}
		if len(others) > 0 {
			right = append(right, "  "+fg(cDim, "shared with "+strings.Join(others, ", ")))
		}
		right = append(right, "")
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cFaint).Width(rightW-2).Padding(0, 1)
	r := strings.Split(box.Render(strings.Join(wrapAll(right, rightW-6), "\n")), "\n")
	body := joinColumns(pad(left, leftW), r)
	body = append(body, "", footer("j/k move · enter open decision · p problems only · same chips as the DECISIONS column in the main TUI · q quit"))
	return clip(body, h)
}

func wrapAll(lines []string, w int) []string {
	var out []string
	for _, l := range lines {
		out = append(out, strings.Split(ansi.Wrap(l, w, ""), "\n")...)
	}
	return out
}

// ---- D · grouped ---------------------------------------------------------

func renderGrouped(m *model, w, h int) string {
	colW := (w - 3) / 2
	var blocks [][]string
	order := append([]*memory{}, m.memories...)
	sort.SliceStable(order, func(i, j int) bool { // problems first
		pi := map[string]int{"superseded": 0, "proposed": 1, "active": 2, "": 3}
		return pi[order[i].status] < pi[order[j].status]
	})
	for _, mem := range order {
		if len(mem.followers)+len(mem.citers) == 0 {
			continue
		}
		head := fg(statusColor(mem.status), "● ") + bold(statusColor(mem.status), mem.num) + " " + bold(cText, fit(mem.title, colW-22)) + " " + fg(statusColor(mem.status), statusName(mem.status))
		if repl, ok := m.g.ReplacedBy(mem.id); ok {
			head += fg(cDead, " → "+m.byMem[repl].num)
		}
		block := []string{head}
		kids := append(append([]string{}, mem.followers...), mem.citers...)
		for i, id := range kids {
			branch := "├─"
			if i == len(kids)-1 {
				branch = "└─"
			}
			kind := ""
			if i >= len(mem.followers) {
				branch = strings.Replace(branch, "─", "┄", 1)
				kind = fg(cDim, " cites")
			}
			var is *issue
			for _, x := range m.issues {
				if x.id == id {
					is = x
				}
			}
			t := fg(cText, fit(is.title, colW-20))
			if is == m.sel {
				t = bold(cAccent, fit(is.title, colW-20))
			}
			block = append(block, "  "+fg(cFaint, branch)+" "+fg(cDim, fit(is.short, 7))+" "+t+kind)
		}
		blocks = append(blocks, append(block, ""))
	}
	var none []string
	for _, is := range m.issues {
		if is.health == datasource.HealthNone {
			none = append(none, "  "+fg(cFaint, "·")+" "+fg(cDim, fit(is.short, 7))+" "+fg(cText, fit(is.title, colW-12)))
		}
	}
	blocks = append(blocks, append([]string{bold(cNone, "No decision recorded")}, none...))
	// Flow blocks into two columns, never splitting one.
	var a, b []string
	for _, blk := range blocks {
		if len(a) <= len(b) {
			a = append(a, blk...)
		} else {
			b = append(b, blk...)
		}
	}
	body := joinColumns(pad(a, colW), []string{" │ "}, b)
	for i := range body {
		if i >= len(a) || true {
			body[i] = strings.Replace(body[i], " │ ", fg(cFaint, " │ "), 1)
		}
	}
	body = append(body, "", footer("j/k move · enter open · i invert (Issue → its decisions) · p problems only · q quit"))
	return clip(body, h)
}

// ---- E · trunks ----------------------------------------------------------

func renderTrunks(m *model, w, h int) string {
	leftW := 56
	var used []*memory
	for _, mem := range m.memories {
		if len(mem.followers)+len(mem.citers) > 0 {
			used = append(used, mem)
		}
	}
	laneW := len(used)*2 + 4
	rightW := w - leftW - laneW
	left := []string{bold(cText, "Work · Issues")}
	issueAt := map[string]int{}
	for _, is := range m.issues {
		issueAt[is.id] = len(left)
		left = append(left, issueRow(m, is, leftW-1, ""))
	}
	right, memAt := memoryColumn(m, rightW, func(mem *memory) bool { _, ok := m.linked(mem.id); return ok }, nil)
	right = append([]string{bold(cText, "Knowledge · Memories")}, right...)
	for k := range memAt {
		memAt[k]++
	}
	rows := max(len(left), len(right))
	type cellT struct {
		r     string
		c     lipgloss.Color
		owned bool
	}
	grid := make([][]cellT, rows)
	for i := range grid {
		grid[i] = make([]cellT, laneW)
		for j := range grid[i] {
			grid[i][j] = cellT{r: " "}
		}
	}
	laneX := func(i int) int { return 1 + i*2 }
	selected := map[string]bool{}
	for _, l := range m.sel.links {
		selected[l.memory] = true
	}
	// Verticals first; horizontals pass under them, so a crossing keeps its │.
	for i, mem := range used {
		x := laneX(i)
		ys := []int{memAt[mem.id]}
		for _, id := range append(append([]string{}, mem.followers...), mem.citers...) {
			ys = append(ys, issueAt[id])
		}
		lo, hi := ys[0], ys[0]
		for _, y := range ys {
			lo, hi = min(lo, y), max(hi, y)
		}
		c := cFaint
		if selected[mem.id] {
			c = statusColor(mem.status)
		}
		for y := lo; y <= hi; y++ {
			grid[y][x] = cellT{"│", c, selected[mem.id]}
		}
		grid[lo][x].r, grid[hi][x].r = "╷", "╵"
		for _, y := range ys[1:] {
			if y == lo {
				grid[y][x].r = "╮"
			} else if y == hi {
				grid[y][x].r = "╯"
			} else {
				grid[y][x].r = "┤"
			}
			for xx := 0; xx < x; xx++ {
				if strings.TrimSpace(grid[y][xx].r) == "" || (!grid[y][xx].owned && selected[mem.id]) {
					grid[y][xx] = cellT{"─", c, selected[mem.id]}
				}
			}
		}
		my := memAt[mem.id]
		switch {
		case my == lo:
			grid[my][x].r = "╭"
		case my == hi:
			grid[my][x].r = "╰"
		default:
			grid[my][x].r = "├"
		}
		for xx := x + 1; xx < laneW-1; xx++ {
			if strings.TrimSpace(grid[my][xx].r) == "" {
				grid[my][xx] = cellT{"─", c, selected[mem.id]}
			}
		}
		grid[my][laneW-1] = cellT{"▸", c, selected[mem.id]}
	}
	lanes := make([]string, rows)
	for y := range grid {
		var b strings.Builder
		for _, cl := range grid[y] {
			if cl.c == "" {
				b.WriteString(cl.r)
			} else {
				b.WriteString(fg(cl.c, cl.r))
			}
		}
		lanes[y] = b.String()
	}
	body := joinColumns(pad(left, leftW), lanes, right)
	body = append(body, "", footer("j/k move · selected Issue's lanes light up in decision colour, all others stay faint · q quit"))
	return clip(body, h)
}
