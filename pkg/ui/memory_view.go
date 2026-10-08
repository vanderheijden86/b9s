package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// MemorySource is the read-only graph preview surface the Memory browser
// reads. Every write stays in bd (ADR 0037).
type MemorySource interface {
	Inventory(ctx context.Context) (datasource.GraphPreview, error)
	SearchMemories(ctx context.Context, query string) ([]datasource.MemorySummary, error)
	Memory(ctx context.Context, id, version string) (datasource.GraphBead, error)
	Links(ctx context.Context, id string) ([]datasource.GraphLink, error)
	Versions(ctx context.Context, id string) ([]datasource.GraphVersion, error)
	Graph(ctx context.Context, progress func(done, total int)) (datasource.MemoryGraph, error)
}

const memoryReadTimeout = 30 * time.Second

type embeddedMemoryMsg struct {
	generation uint64
	msg        tea.Msg
}

// A project switch invalidates every in-flight command from the previous browser.
func wrapMemoryCmd(cmd tea.Cmd, generation uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			commands := make([]tea.Cmd, 0, len(batch))
			for _, child := range batch {
				commands = append(commands, wrapMemoryCmd(child, generation))
			}
			return tea.BatchMsg(commands)
		}
		if _, ok := msg.(tea.QuitMsg); ok {
			return msg
		}
		return embeddedMemoryMsg{generation: generation, msg: msg}
	}
}

func (m Model) openMemories() (Model, tea.Cmd) {
	return m.startMemories(true)
}

// startMemories opens the Memory browser of the active project and starts
// its graph read. A hidden start serves an Issue detail's Memory Links: it
// leaves the view closed and reports no failure, since nobody asked for the
// browser, and it is not retried until the project changes.
func (m Model) startMemories(visible bool) (Model, tea.Cmd) {
	if m.memoryBrowser != nil && m.memoryBrowser.sourceErr == nil && m.memoryBrowser.graphErr == nil {
		m.memoryVisible = m.memoryVisible || visible
		return m, nil
	}
	project := m.activeProjectPath
	if project == "" && m.beadsPath != "" {
		project = filepath.Dir(filepath.Dir(m.beadsPath))
	}
	if project == "" {
		if visible {
			m.statusMsg, m.statusIsError = "Memories unavailable: this project has no local checkout", false
		}
		m.memoryRefused = true
		return m, nil
	}
	source, err := datasource.OpenGraphPreview(project)
	if err != nil {
		m.memoryRefused = true
		if !visible {
			return m, nil
		}
		m.memoryVisible = false
		if reason, ok := datasource.MemoryUnavailableReason(err); ok {
			m.statusMsg, m.statusIsError = "Memories unavailable: "+reason, false
			return m, nil
		}
		m.statusMsg, m.statusIsError = strings.ReplaceAll(datasource.ExplainMemoryFailure(err), "\n", " "), true
		return m, nil
	}
	browser := newMemoryBrowser(source, m.activeProjectName, m.theme)
	browser.width, browser.height = m.width, m.height
	beadsDir := m.memoryBeadsDir()
	if beadsDir == "" {
		beadsDir, _ = projectBeadsDir(project)
	}
	if graph, ok := datasource.MemoryGraphFor(beadsDir); ok {
		browser = browser.applyGraph(memoryGraphMsg{graph: graph})
	}
	browser, graphCmd := browser.enterMode(memoryModeFocus)
	m.memoryGeneration++
	m.memoryBrowser, m.memoryVisible = &browser, visible
	return m, wrapMemoryCmd(tea.Batch(browser.Init(), graphCmd), m.memoryGeneration)
}

// memoryBeadsDir names the .beads directory whose Memory graph the tree
// reads; only an embedded store can be a graph workspace.
func (m Model) memoryBeadsDir() string {
	if m.doltSource.Type != datasource.SourceTypeDoltEmbedded {
		return ""
	}
	return datasource.EmbeddedBeadsDir(m.doltSource)
}

// requestMemoryGraph starts the graph read an Issue detail needs for its
// Memory Links. The graph costs one traversal per Issue component, so it is
// read only once a detail is on screen in a Memory workspace (ADR 0051).
func (m *Model) requestMemoryGraph() tea.Cmd {
	if m.memoryRefused || !m.issueDetailShown() {
		return nil
	}
	if _, ok := m.tree.MemoryGraph(); ok {
		return nil
	}
	if m.memoryBrowser == nil {
		dir := m.memoryBeadsDir()
		if dir == "" || !datasource.MemoryWorkspaceFor(dir) {
			return nil
		}
		next, cmd := m.startMemories(false)
		*m = next
		return cmd
	}
	if m.memoryBrowser.graphLoad != graphNotRead {
		return nil
	}
	browser, cmd := m.memoryBrowser.readGraph()
	m.memoryBrowser = &browser
	return wrapMemoryCmd(cmd, m.memoryGeneration)
}

// refreshMemoryGraph re-reads a graph already read, after the workspace
// changed, so Memory Links follow writes without slowing the Issue reload.
func (m *Model) refreshMemoryGraph() tea.Cmd {
	if m.memoryBrowser == nil {
		return nil
	}
	browser, cmd := m.memoryBrowser.refreshGraph()
	m.memoryBrowser = &browser
	return wrapMemoryCmd(cmd, m.memoryGeneration)
}

// issueDetailShown reports whether an Issue detail is on screen.
func (m Model) issueDetailShown() bool {
	return (m.isSplitView && !m.treeDetailHidden) || m.showDetails
}

type memoryPane uint8

const (
	memoryPaneList memoryPane = iota
	memoryPaneDetail
	memoryPaneVersions
	memoryPaneSearch
)

// memoryKey names one readable Memory version. An empty version is the
// current one, which bd resolves at read time.
type memoryKey struct {
	id      string
	version string
}

type memoryDetail struct {
	bead     datasource.GraphBead
	incoming []datasource.GraphLink
	err      error
}

type memorySummariesMsg struct {
	query string
	items []datasource.MemorySummary
	err   error
}

type memoryInventoryMsg struct {
	graph datasource.GraphPreview
	err   error
}

type memoryDetailMsg struct {
	key    memoryKey
	detail memoryDetail
}

type memoryVersionsMsg struct {
	id       string
	versions []datasource.GraphVersion
	err      error
}

// memoryLinkRow is one Link as seen from the selected Memory.
type memoryLinkRow struct {
	outgoing bool
	link     datasource.GraphLink
	other    string
}

// MemoryBrowser lists the Memories of a graph preview workspace and reads one
// Memory's body, Links and versions only when it is selected: each read is a
// bd process, and reading every Memory up front took about 14 seconds for 32
// Memories in the POC.
type MemoryBrowser struct {
	source  MemorySource
	project string
	width   int
	height  int
	pane    memoryPane
	prior   memoryPane

	query     string
	input     string
	searching bool
	summaries []datasource.MemorySummary
	listErr   error
	cursor    int

	endpoints map[string]datasource.GraphBead
	total     int

	selected memoryKey
	details  map[memoryKey]memoryDetail
	pending  map[memoryKey]bool
	back     []memoryKey
	linkRow  int
	scroll   int

	versions      map[string][]datasource.GraphVersion
	versionErr    map[string]error
	versionCursor int

	mode           memoryMode
	graph          datasource.MemoryGraph
	fullGraph      datasource.MemoryGraph
	graphQuery     string
	graphStatus    model.Status
	expandedDetail bool
	detailScroll   int
	graphLoad      graphLoad
	graphErr       error
	// graphDone of graphTotal Issue components are read; a zero total means
	// the inventory is still being read.
	graphDone, graphTotal int
	// graphRefreshing re-reads a shown graph; graphStale repeats a read that
	// a workspace change overtook.
	graphRefreshing bool
	graphStale      bool
	sourceErr       error
	decide          decisionViewState
	// wiresDetail shows the selected record beside the wires; wiresBack holds
	// the records a followed Link left, for backspace.
	wiresDetail bool
	wiresBack   []string

	status   string
	theme    Theme
	renderer *MarkdownRenderer
	rendered map[memoryKey]string
}

// NewMemoryBrowser must run before the program starts: building the Markdown
// renderer asks the terminal for its background colour, and that reply would
// be lost to the program's input reader once it runs.
func NewMemoryBrowser(source MemorySource, project string) MemoryBrowser {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	return newMemoryBrowser(source, project, theme)
}

func newMemoryBrowser(source MemorySource, project string, theme Theme) MemoryBrowser {
	return MemoryBrowser{
		source:     source,
		project:    project,
		theme:      theme,
		renderer:   NewMarkdownRendererWithTheme(80, theme),
		width:      100,
		height:     30,
		searching:  true,
		endpoints:  map[string]datasource.GraphBead{},
		details:    map[memoryKey]memoryDetail{},
		pending:    map[memoryKey]bool{},
		versions:   map[string][]datasource.GraphVersion{},
		versionErr: map[string]error{},
		rendered:   map[memoryKey]string{},
	}
}

func (b MemoryBrowser) Init() tea.Cmd {
	return tea.Batch(b.searchCmd(""), b.inventoryCmd())
}

func (b MemoryBrowser) searchCmd(query string) tea.Cmd {
	source := b.source
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), memoryReadTimeout)
		defer cancel()
		items, err := source.SearchMemories(ctx, query)
		return memorySummariesMsg{query: query, items: items, err: err}
	}
}

func (b MemoryBrowser) inventoryCmd() tea.Cmd {
	source := b.source
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), memoryReadTimeout)
		defer cancel()
		graph, err := source.Inventory(ctx)
		return memoryInventoryMsg{graph: graph, err: err}
	}
}

// selectCmd shows key, reading it unless it is cached or already in flight.
func (b *MemoryBrowser) selectCmd(key memoryKey) tea.Cmd {
	if b.selected != key {
		b.linkRow = 0
		b.scroll = 0
	}
	b.selected = key
	if _, ok := b.details[key]; ok || b.pending[key] || key.id == "" {
		return nil
	}
	b.pending[key] = true
	source := b.source
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), memoryReadTimeout)
		defer cancel()
		var detail memoryDetail
		var linksErr error
		var links []datasource.GraphLink
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			links, linksErr = source.Links(ctx, key.id)
		}()
		detail.bead, detail.err = source.Memory(ctx, key.id, key.version)
		wg.Wait()
		if detail.err == nil && linksErr != nil {
			detail.err = linksErr
		}
		for _, link := range links {
			if link.Target == key.id && link.Source != key.id {
				detail.incoming = append(detail.incoming, link)
			}
		}
		return memoryDetailMsg{key: key, detail: detail}
	}
}

func (b MemoryBrowser) versionsCmd(id string) tea.Cmd {
	source := b.source
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), memoryReadTimeout)
		defer cancel()
		versions, err := source.Versions(ctx, id)
		return memoryVersionsMsg{id: id, versions: versions, err: err}
	}
}

func (b MemoryBrowser) Update(msg tea.Msg) (MemoryBrowser, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = msg.Width, msg.Height
		b.setExpandedDetail(b.expandedDetail)
		return b, nil
	case memorySummariesMsg:
		if msg.query != b.query {
			return b, nil
		}
		b.searching = false
		if msg.err != nil {
			b.sourceErr = msg.err
		}
		b.summaries, b.listErr = msg.items, msg.err
		if msg.query == "" && msg.err == nil {
			b.total = len(msg.items)
		}
		b.cursor = 0
		if len(b.summaries) == 0 {
			return b, nil
		}
		return b, b.selectCmd(memoryKey{id: b.summaries[0].ID})
	case memoryInventoryMsg:
		if msg.err != nil {
			b.sourceErr = msg.err
			b.status = "Link endpoint titles unavailable: " + msg.err.Error()
			return b, nil
		}
		for _, bead := range msg.graph.Beads {
			b.endpoints[bead.ID] = bead
		}
		return b, nil
	case memoryDetailMsg:
		delete(b.pending, msg.key)
		b.details[msg.key] = msg.detail
		return b, nil
	case memoryGraphProgressMsg:
		if b.graphLoad == graphReading {
			b.graphDone, b.graphTotal = msg.done, msg.total
		}
		return b, msg.next
	case memoryGraphMsg:
		b = b.applyGraph(msg)
		if b.graphStale {
			b.graphStale = false
			return b.refreshGraph()
		}
		return b, nil
	case memoryVersionsMsg:
		b.versions[msg.id] = msg.versions
		b.versionErr[msg.id] = msg.err
		return b, nil
	case tea.KeyMsg:
		return b.handleKey(msg)
	case tea.MouseMsg:
		return b.handleMouse(msg), nil
	}
	return b, nil
}

func (b MemoryBrowser) handleKey(msg tea.KeyMsg) (MemoryBrowser, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" {
		return b, tea.Quit
	}
	if b.pane == memoryPaneSearch {
		return b.handleSearchKey(msg)
	}
	b.status = ""
	if b.mode == memoryModeFocus && b.wiresDetail {
		if next, ok := b.handleWiresPanelKey(key); ok {
			return next, nil
		}
	}
	switch key {
	case "q":
		return b, tea.Quit
	case "1":
		return b.enterMode(memoryModeList)
	case "2":
		return b.enterMode(memoryModeFocus)
	case "\\":
		b.setExpandedDetail(!b.expandedDetail)
		return b, nil
	}
	if b.mode != memoryModeList {
		return b.handleGraphKey(key)
	}
	if b.expandedDetail {
		switch key {
		case "J":
			b.scroll++
			return b, nil
		case "K":
			b.scroll = max(0, b.scroll-1)
			return b, nil
		case "esc":
			b.setExpandedDetail(false)
			return b, nil
		}
	}
	switch key {
	case "/":
		b.prior = b.pane
		b.pane = memoryPaneSearch
		b.input = b.query
		return b, nil
	case "v":
		if b.selected.id == "" {
			return b, nil
		}
		if b.pane != memoryPaneVersions {
			b.prior = b.pane
		}
		b.pane = memoryPaneVersions
		b.versionCursor = 0
		if _, ok := b.versions[b.selected.id]; ok {
			return b, nil
		}
		return b, b.versionsCmd(b.selected.id)
	}
	switch b.pane {
	case memoryPaneDetail:
		return b.handleDetailKey(key)
	case memoryPaneVersions:
		return b.handleVersionsKey(key)
	}
	return b.handleListKey(key)
}

func (b *MemoryBrowser) setExpandedDetail(expanded bool) {
	b.expandedDetail = expanded
	b.detailScroll = 0
	b.rendered = map[memoryKey]string{}
	width := b.detailWidth()
	if expanded {
		width = max(1, b.width-2)
	}
	b.renderer.SetWidthWithTheme(width, b.theme)
}

func (b MemoryBrowser) handleSearchKey(msg tea.KeyMsg) (MemoryBrowser, tea.Cmd) {
	switch msg.String() {
	case "esc":
		b.pane = b.prior
	case "enter":
		if b.mode != memoryModeList {
			b.pane = b.prior
			b.graphQuery = strings.TrimSpace(b.input)
			b.filterGraph()
			return b, nil
		}
		b.pane = memoryPaneList
		b.query = strings.TrimSpace(b.input)
		b.searching = true
		b.back = nil
		return b, b.searchCmd(b.query)
	case "backspace":
		if runes := []rune(b.input); len(runes) > 0 {
			b.input = string(runes[:len(runes)-1])
		}
	default:
		b.input += string(typedRunes(msg))
	}
	return b, nil
}

func (b MemoryBrowser) handleListKey(key string) (MemoryBrowser, tea.Cmd) {
	move := 0
	switch key {
	case "j", "down":
		move = 1
	case "k", "up":
		move = -1
	case "g", "home":
		move = -len(b.summaries)
	case "G", "end":
		move = len(b.summaries)
	case "enter", "l", "right", "tab":
		if b.selected.id != "" {
			b.pane = memoryPaneDetail
		}
		return b, nil
	}
	if move == 0 || len(b.summaries) == 0 {
		return b, nil
	}
	b.cursor = max(0, min(len(b.summaries)-1, b.cursor+move))
	b.back = nil
	return b, b.selectCmd(memoryKey{id: b.summaries[b.cursor].ID})
}

func (b MemoryBrowser) handleDetailKey(key string) (MemoryBrowser, tea.Cmd) {
	rows := b.linkRows()
	switch key {
	case "j", "down":
		b.linkRow = min(b.linkRow+1, max(0, len(rows)-1))
	case "k", "up":
		b.linkRow = max(b.linkRow-1, 0)
	case "ctrl+d", "pgdown", " ":
		b.scroll += max(1, b.bodyHeight()/2)
	case "ctrl+u", "pgup":
		b.scroll = max(0, b.scroll-max(1, b.bodyHeight()/2))
	case "tab":
		b.pane = memoryPaneList
	case "esc", "h", "left":
		if len(b.back) == 0 {
			b.pane = memoryPaneList
			return b, nil
		}
		previous := b.back[len(b.back)-1]
		b.back = b.back[:len(b.back)-1]
		return b, b.selectCmd(previous)
	case "enter":
		if b.linkRow >= len(rows) {
			return b, nil
		}
		other := rows[b.linkRow].other
		if endpoint, ok := b.endpoints[other]; ok && endpoint.Kind != "memory" {
			b.status = "Issue snapshots open in bd: bd show " + graphPath(other)
			return b, nil
		}
		b.back = append(b.back, b.selected)
		return b, b.selectCmd(memoryKey{id: other})
	}
	return b, nil
}

func (b MemoryBrowser) handleVersionsKey(key string) (MemoryBrowser, tea.Cmd) {
	versions := b.versions[b.selected.id]
	switch key {
	case "j", "down":
		b.versionCursor = min(b.versionCursor+1, max(0, len(versions)-1))
	case "k", "up":
		b.versionCursor = max(b.versionCursor-1, 0)
	case "esc":
		b.pane = b.prior
	case "enter":
		if b.versionCursor >= len(versions) {
			return b, nil
		}
		b.pane = memoryPaneDetail
		next := memoryKey{id: b.selected.id}
		// bd lists versions newest first, so the first one is the current one.
		if b.versionCursor > 0 {
			next.version = versions[b.versionCursor].Version
		}
		return b, b.selectCmd(next)
	}
	return b, nil
}

// linkRows lists outgoing Links as owned by the version shown, then incoming
// Links as they are now: bd retains Links with their owner's versions, but an
// incoming Link belongs to another Bead's history.
func (b MemoryBrowser) linkRows() []memoryLinkRow {
	detail, ok := b.details[b.selected]
	if !ok || detail.err != nil {
		return nil
	}
	var rows []memoryLinkRow
	for _, link := range detail.bead.Owned {
		rows = append(rows, memoryLinkRow{outgoing: true, link: link, other: link.Target})
	}
	for _, link := range detail.incoming {
		rows = append(rows, memoryLinkRow{link: link, other: link.Source})
	}
	return rows
}

func (b MemoryBrowser) View() string {
	t := b.theme
	header := t.Header.Render(" b9s memories ") + " " + b.headerText()
	if b.sourceErr != nil {
		failure := ansi.Wrap(memoryText(datasource.ExplainMemoryFailure(b.sourceErr)), max(1, b.width), "")
		return clipBlock(header+"\n"+b.errorStyle().Render(failure)+"\nr retry · q back", b.width, b.height)
	}
	if b.expandedDetail && b.graphErr == nil {
		body := b.detailView(max(1, b.width-2), max(1, b.height-4))
		if b.mode != memoryModeList {
			body = b.graphDetail(max(1, b.width-2), max(1, b.height-4))
		}
		return clipBlock(header+" · Expanded detail\n"+body+"\n"+b.footer(), b.width, b.height)
	}
	if b.mode != memoryModeList {
		if b.width < 30 {
			return clipBlock(t.MutedText.Render("Widen terminal\nto 30 columns."), b.width, b.height)
		}
		paneHeight := max(1, b.height-4)
		pane := func(active bool, width int, content string) string {
			color := t.Border
			if active {
				color = t.Highlight
			}
			return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(color).Width(width).Height(paneHeight).
				Render(clipBlock(content, width, paneHeight))
		}
		wiresWidth, wiresShown := b.wiresPaneWidth()
		var body string
		switch {
		case !b.wiresPanelShown():
			body = pane(true, wiresWidth, b.graphView(wiresWidth, paneHeight))
		case wiresShown:
			panelWidth := max(20, b.width-wiresWidth-4)
			body = lipgloss.JoinHorizontal(lipgloss.Top, pane(false, wiresWidth, b.graphView(wiresWidth, paneHeight)), pane(true, panelWidth, b.graphDetail(panelWidth, paneHeight)))
		default:
			body = pane(true, wiresWidth, b.graphDetail(wiresWidth, paneHeight))
		}
		return clipBlock(lipgloss.JoinVertical(lipgloss.Left, t.Header.Render(" b9s memories ")+" "+b.graphHeaderText(), body, b.footer()), b.width, b.height)
	}
	listWidth := b.listWidth()
	detailWidth := b.detailWidth()
	// The header, the footer and the two border rows leave this for content.
	paneHeight := max(1, b.height-4)
	border := func(active bool) lipgloss.Style {
		color := t.Border
		if active {
			color = t.Highlight
		}
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(color).Height(paneHeight)
	}
	list := border(b.pane == memoryPaneList || b.pane == memoryPaneSearch).Width(listWidth).Render(clipBlock(b.listView(listWidth, paneHeight), listWidth, paneHeight))
	right := b.detailView(detailWidth, paneHeight)
	if b.pane == memoryPaneVersions {
		right = b.versionsView(detailWidth)
	}
	detail := border(b.pane != memoryPaneList && b.pane != memoryPaneSearch).Width(detailWidth).Render(clipBlock(right, detailWidth, paneHeight))
	frame := lipgloss.JoinVertical(lipgloss.Left, header, lipgloss.JoinHorizontal(lipgloss.Top, list, detail), b.footer())
	return clipBlock(frame, b.width, b.height)
}

// clipBlock cuts text to width cells and height lines. lipgloss wraps a line
// wider than a fixed Width, which would push a pane past the terminal.
func clipBlock(text string, width, height int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

func (b MemoryBrowser) listWidth() int { return max(28, b.width*2/5) }

// detailWidth leaves room for both panes' borders.
func (b MemoryBrowser) detailWidth() int { return max(20, b.width-b.listWidth()-4) }

func (b MemoryBrowser) headerText() string {
	text := b.project + " · "
	switch {
	case b.searching:
		return text + "loading Memories…"
	case b.query != "":
		return text + fmt.Sprintf("literal search %q: %d of %d Memories (title and body text, no synonyms)", b.query, len(b.summaries), b.total)
	}
	return text + fmt.Sprintf("%d Memories", len(b.summaries))
}

func (b MemoryBrowser) listView(width, height int) string {
	t := b.theme
	if b.listErr != nil {
		return b.errorStyle().Render(memoryText(b.listErr.Error()))
	}
	if !b.searching && len(b.summaries) == 0 {
		return t.MutedText.Render("No Memories match.")
	}
	start := max(0, b.cursor-height+1)
	var lines []string
	for i := start; i < len(b.summaries) && len(lines) < height; i++ {
		summary := b.summaries[i]
		marker := "  "
		if i == b.cursor {
			marker = "▶ "
		}
		line := marker + graphPath(summary.ID) + "  " + memoryText(summary.Title)
		if b.query != "" && len(summary.MatchedFields) > 0 {
			line += "  [" + strings.Join(summary.MatchedFields, ",") + "]"
		}
		line = truncate(strings.ReplaceAll(line, "\n", " "), width)
		if i == b.cursor {
			line = t.Selected.Render(padRight(line, width))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (b MemoryBrowser) detailView(width, height int) string {
	t := b.theme
	if b.selected.id == "" {
		return t.MutedText.Render("Select a Memory.")
	}
	detail, ok := b.details[b.selected]
	if !ok {
		return t.MutedText.Render("Reading " + graphPath(b.selected.id) + "…")
	}
	if detail.err != nil {
		return b.errorStyle().Render(truncate(memoryText(detail.err.Error()), width*3))
	}
	bead := detail.bead
	title := t.PrimaryBold.Render(truncate(memoryText(bead.Properties.Title), width))
	version := graphPath(b.selected.id) + " · current version " + shortToken(bead.Version)
	if b.selected.version != "" {
		version = graphPath(b.selected.id) + " · retained version " + shortToken(bead.Version) + " · not current"
	}
	links := b.linksView(width, max(2, (height-2)/2))
	bodyHeight := max(0, height-2-lipgloss.Height(links))
	body := strings.Split(b.renderBody(width), "\n")
	scroll := min(b.scroll, max(0, len(body)-bodyHeight))
	body = body[scroll:min(len(body), scroll+bodyHeight)]
	return strings.Join(append(append([]string{title, t.MutedText.Render(version)}, body...), links), "\n")
}

// bodyHeight approximates the body window for page-sized scrolling.
func (b MemoryBrowser) bodyHeight() int { return max(1, b.height-14) }

func (b MemoryBrowser) renderBody(width int) string {
	if text, ok := b.rendered[b.selected]; ok {
		return text
	}
	body := memoryText(b.details[b.selected].bead.Properties.Body)
	rendered, err := b.renderer.Render(body)
	if err != nil {
		rendered = body
	}
	rendered = strings.Trim(rendered, "\n")
	b.rendered[b.selected] = rendered
	return rendered
}

// linksView renders at most limit lines: the heading, then the window of Link
// lines that holds the selected Link.
func (b MemoryBrowser) linksView(width, limit int) string {
	t := b.theme
	rows := b.linkRows()
	heading := t.InfoBold.Render("Informational Links") + t.MutedText.Render(" · no effect on scheduling")
	var lines []string
	selectedLine := 0
	outgoing := "Outgoing"
	if b.selected.version != "" {
		outgoing = "Outgoing (this version)"
	}
	incomingHeader := "Incoming"
	if b.selected.version != "" {
		incomingHeader = "Incoming (current version)"
	}
	wroteOut, wroteIn := false, false
	for i, row := range rows {
		if row.outgoing && !wroteOut {
			lines, wroteOut = append(lines, t.MutedText.Render(outgoing)), true
		}
		if !row.outgoing && !wroteIn {
			lines, wroteIn = append(lines, t.MutedText.Render(incomingHeader)), true
		}
		arrow := "←"
		if row.outgoing {
			arrow = "→"
		}
		marker := "  "
		if b.pane == memoryPaneDetail && i == b.linkRow {
			marker = "▶ "
		}
		if i == b.linkRow {
			selectedLine = len(lines)
		}
		lines = append(lines, truncate(marker+arrow+" "+graphPath(row.link.Type)+"  "+graphPath(row.other), width))
		lines = append(lines, truncate("    "+memoryText(strings.ReplaceAll(b.endpointLabel(row.other), "\n", " ")), width))
		if note := row.link.Properties.Note; note != "" {
			lines = append(lines, t.MutedText.Render(truncate("    "+memoryText(strings.ReplaceAll(note, "\n", " ")), width)))
		}
	}
	if !wroteIn && b.selected.version != "" {
		lines = append(lines, t.MutedText.Render(incomingHeader+": none"))
	}
	if len(rows) == 0 {
		lines = append(lines, t.MutedText.Render("none"))
	}
	window := max(1, limit-1)
	start := max(0, min(selectedLine-1, len(lines)-window))
	lines = lines[start:min(len(lines), start+window)]
	return strings.Join(append([]string{heading}, lines...), "\n")
}

func (b MemoryBrowser) endpointLabel(id string) string {
	endpoint, ok := b.endpoints[id]
	if !ok {
		return "(unknown endpoint)"
	}
	switch endpoint.Kind {
	case "memory":
		return "Memory · " + endpoint.Properties.Title
	case "issue":
		status := endpoint.Properties.Status
		if status == "" {
			status = "status unknown"
		}
		return "Issue · " + status + " · " + endpoint.Properties.Title
	}
	return endpoint.Properties.Title
}

func (b MemoryBrowser) versionsView(width int) string {
	t := b.theme
	lines := []string{t.PrimaryBold.Render("Retained versions of " + graphPath(b.selected.id)), t.MutedText.Render("Versions bd retains, newest first. This is not full history.")}
	if err := b.versionErr[b.selected.id]; err != nil {
		return strings.Join(append(lines, b.errorStyle().Render(memoryText(err.Error()))), "\n")
	}
	versions, ok := b.versions[b.selected.id]
	if !ok {
		return strings.Join(append(lines, t.MutedText.Render("Reading versions…")), "\n")
	}
	for i, version := range versions {
		marker := "  "
		if i == b.versionCursor {
			marker = "▶ "
		}
		line := fmt.Sprintf("%s#%d  %s  %s  %s", marker, version.Ordinal, version.ChangeAt.Local().Format("2006-01-02 15:04:05"), version.Actor, shortToken(version.Version))
		if i == 0 {
			line += "  current"
		}
		if version.Removed {
			line += "  removed"
		}
		line = truncate(memoryText(line), width)
		if i == b.versionCursor {
			line = t.Selected.Render(padRight(line, width))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (b MemoryBrowser) footer() string {
	t := b.theme
	var help string
	switch {
	case b.pane == memoryPaneSearch:
		return "/" + b.input + "█  " + t.MutedText.Render("enter search · esc cancel · empty lists all")
	case b.mode != memoryModeList && (b.sourceErr != nil || b.graphErr != nil):
		help = "r retry · q back"
	case b.mode == memoryModeFocus && b.wiresPanelShown():
		help = "esc/d wires · j/k move · 1-9 follow · backspace back · J/K scroll · \\ full width · q back"
	case b.mode == memoryModeFocus:
		help = "j/k move · enter/d open · tab side · A/O/C status · / search · \\ detail · shift+tab context · [ ] pages · J/K scroll · q back"
	case b.pane == memoryPaneDetail:
		help = "j/k link · enter follow · ctrl+d/u scroll · v versions · esc back · / search · q quit"
	case b.pane == memoryPaneVersions:
		help = "j/k version · enter read · esc back · q quit"
	default:
		help = "j/k move · enter open · v versions · / search · 2 wires · b9s web: graph · q quit"
	}
	if b.status != "" {
		return t.InfoText.Render(b.status) + "  " + t.MutedText.Render(help)
	}
	return t.MutedText.Render(help)
}

// graphPath shortens a canonical graph URL to the path a user types in bd.
func graphPath(id string) string {
	for _, marker := range []string{"/beads/", "/links/", "/types/"} {
		if index := strings.LastIndex(id, marker); index >= 0 {
			return id[index+len(marker):]
		}
	}
	return id
}

func shortToken(token string) string {
	if len(token) > 8 {
		return token[:8]
	}
	return token
}

// memoryText drops control characters from preview text, which a Memory body
// can carry and which would otherwise drive the terminal.
func memoryText(input string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, input)
}

func (b MemoryBrowser) errorStyle() lipgloss.Style {
	return b.theme.Renderer.NewStyle().Foreground(b.theme.Blocked)
}
