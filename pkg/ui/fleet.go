package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/internal/gitlog"
)

// fleetView is the closed set of screens the fleet browser stacks, k9s
// style: a resource list at the bottom and drill-down views above it.
type fleetView uint8

const (
	fleetAgents fleetView = iota
	fleetLogs
	fleetContext
	fleetXray
	fleetPulse
	fleetCommits
	fleetCommit
	fleetText
)

var fleetViewNames = map[fleetView]string{
	fleetAgents: "agents", fleetLogs: "logs", fleetContext: "context", fleetXray: "xray",
	fleetPulse: "pulse", fleetCommits: "commits", fleetCommit: "commit", fleetText: "text",
}

// fleetRootViews are the views a ':' command or Tab opens as the base of the
// stack.
var fleetRootViews = []fleetView{fleetAgents, fleetXray, fleetPulse, fleetCommits}

// fleetRefresh is how often a visible fleet browser rescans transcripts.
const fleetRefresh = 2 * time.Second

// FleetDeps is what the fleet browser reads from the rest of b9s. Tests
// replace every field.
type FleetDeps struct {
	Collector *agents.Collector
	// RepoDir is the active project's checkout; Repos maps every known
	// project name to its checkout for the all-repos commit log.
	RepoDir string
	Repos   map[string]string
	// Issue looks up an issue of the active project.
	Issue func(id string) (title, status string, ok bool)
	// Closed lists the close times of the active project's issues.
	Closed    func() []time.Time
	Clipboard func(string) error
	Now       func() time.Time
	// Exec runs a side-effecting command (tmux attach, send-keys). Tests
	// record the call instead.
	Exec func(name string, args ...string) error
}

// fleetOutcome is what a key press asks the surrounding Model to do.
type fleetOutcome struct {
	close     bool
	openIssue string
	command   string
}

type fleetPromptKind uint8

const (
	fleetPromptNone fleetPromptKind = iota
	fleetPromptFilter
	fleetPromptCommand
	fleetPromptInject
)

// Messages the fleet browser's commands return. The Model routes them back
// in an embeddedFleetMsg so a project switch drops stale replies.
type (
	fleetTickMsg   struct{}
	fleetScanMsg   struct{ snap agents.Snapshot }
	fleetEventsMsg struct {
		id     string
		events []agents.Event
		err    error
	}
	fleetContextMsg struct {
		id  string
		rep agents.ContextReport
	}
	fleetCommitsMsg struct {
		commits []gitlog.Commit
		err     error
	}
	fleetDetailMsg struct {
		detail gitlog.Detail
		dir    string
		issue  []gitlog.Commit
		err    error
	}
	fleetTextMsg struct {
		title string
		text  string
	}
	fleetStatusMsg struct{ text string }
)

type embeddedFleetMsg struct {
	generation uint64
	msg        tea.Msg
}

func wrapFleetCmd(cmd tea.Cmd, generation uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			wrapped := make(tea.BatchMsg, 0, len(batch))
			for _, child := range batch {
				wrapped = append(wrapped, wrapFleetCmd(child, generation))
			}
			return wrapped
		}
		if msg == nil {
			return nil
		}
		return embeddedFleetMsg{generation: generation, msg: msg}
	}
}

// Sort orders of the agents table.
const (
	fleetSortActivity = iota
	fleetSortContext
	fleetSortProject
	fleetSortState
	fleetSortCount
)

var fleetSortNames = []string{"activity", "context", "project", "state"}

// Log filters of the logs view, keys 1 to 5.
var fleetLogFilters = []struct {
	name  string
	kinds []agents.EventKind
}{
	{"all", nil},
	{"messages", []agents.EventKind{agents.EventTask, agents.EventUser, agents.EventAI}},
	{"tools", []agents.EventKind{agents.EventTool, agents.EventResult, agents.EventBD}},
	{"errors", []agents.EventKind{agents.EventErr}},
	{"bd", []agents.EventKind{agents.EventBD}},
}

// FleetBrowser is the full-screen view of agent sessions and commit history
// across projects.
type FleetBrowser struct {
	deps          FleetDeps
	theme         Theme
	width, height int
	stack         []fleetView
	ticking       bool

	snap     agents.Snapshot
	scanned  bool
	activity agents.Activity

	cursor   int
	sortBy   int
	project  string
	filter   string
	prompt   fleetPromptKind
	input    string
	status   string
	stopArm  string // session id whose stop needs a second ctrl+k
	selected string // session id the drill-down views show

	events    []agents.Event
	logCursor int
	follow    bool
	logFilter int
	expanded  bool
	logErr    string
	// wrap, hideTime and fullscreen are the k9s log toggles w, t and f.
	wrap       bool
	hideTime   bool
	fullscreen bool

	ctx       agents.ContextReport
	ctxCursor int
	ctxFold   map[agents.Layer]bool

	xrayCursor int

	commits      []gitlog.Commit
	commitCursor int
	commitErr    string
	showAuthor   bool
	allBranches  bool
	allRepos     bool
	commitsRead  bool

	detail       gitlog.Detail
	detailDir    string
	detailIssue  []gitlog.Commit
	detailScroll int
	detailErr    string

	textTitle  string
	text       []string
	textScroll int
}

// fleetIssueKeys are the function keys the issue views own: the type
// filters and reload (ADR 0069). Inside the fleet they close it and do what
// they do everywhere else.
var fleetIssueKeys = map[string]bool{"f1": true, "f2": true, "f3": true, "f4": true, "f5": true}

// withDefaults fills the clock and the command runner the browser calls
// without a nil check. Every path that hands the browser deps goes through it.
func (d FleetDeps) withDefaults() FleetDeps {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Exec == nil {
		d.Exec = func(name string, args ...string) error {
			// A test run must never drive the tmux server it happens to run under.
			if testMode() {
				return nil
			}
			return exec.Command(name, args...).Run()
		}
	}
	return d
}

// NewFleetBrowser opens the browser on one root view.
func NewFleetBrowser(deps FleetDeps, theme Theme, width, height int, view fleetView) FleetBrowser {
	deps = deps.withDefaults()
	return FleetBrowser{deps: deps, theme: theme, width: width, height: height, stack: []fleetView{view}, follow: true, ctxFold: map[agents.Layer]bool{}}
}

func (b FleetBrowser) view() fleetView { return b.stack[len(b.stack)-1] }

// Init starts the scan ticker and the reads the root view needs.
func (b *FleetBrowser) Init() tea.Cmd {
	cmds := []tea.Cmd{b.scanCmd()}
	b.ticking = true
	if b.view() == fleetCommits && !b.commitsRead {
		cmds = append(cmds, b.commitsCmd())
	}
	return tea.Batch(cmds...)
}

func (b FleetBrowser) scanCmd() tea.Cmd {
	c := b.deps.Collector
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return fleetScanMsg{snap: c.Scan(ctx)}
	}
}

func fleetTick() tea.Cmd {
	return tea.Tick(fleetRefresh, func(time.Time) tea.Msg { return fleetTickMsg{} })
}

func (b FleetBrowser) eventsCmd(id string) tea.Cmd {
	c := b.deps.Collector
	if c == nil || id == "" {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ev, err := c.Events(ctx, id)
		return fleetEventsMsg{id: id, events: ev, err: err}
	}
}

func (b FleetBrowser) contextCmd(id string) tea.Cmd {
	c := b.deps.Collector
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		rep, _ := c.Context(id)
		return fleetContextMsg{id: id, rep: rep}
	}
}

func (b FleetBrowser) commitsCmd() tea.Cmd {
	repoDir, repos, all, branches := b.deps.RepoDir, b.deps.Repos, b.allRepos, b.allBranches
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if all {
			logs := map[string][]gitlog.Commit{}
			for name, dir := range repos {
				if dir == "" {
					continue
				}
				cs, err := gitlog.Read(ctx, dir, gitlog.Options{Max: 200, All: branches})
				if err == nil {
					logs[name] = cs
				}
			}
			return fleetCommitsMsg{commits: gitlog.Merge(logs)}
		}
		if repoDir == "" {
			return fleetCommitsMsg{err: fmt.Errorf("the active project has no checkout")}
		}
		cs, err := gitlog.Read(ctx, repoDir, gitlog.Options{Max: 500, All: branches})
		return fleetCommitsMsg{commits: cs, err: err}
	}
}

func (b FleetBrowser) detailCmd(c gitlog.Commit) tea.Cmd {
	dir := b.deps.RepoDir
	if c.Repo != "" {
		dir = b.deps.Repos[c.Repo]
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		d, err := gitlog.Show(ctx, dir, c.Hash)
		var issue []gitlog.Commit
		if len(c.Issues) > 0 {
			issue, _ = gitlog.Read(ctx, dir, gitlog.Options{Max: 50, All: true, Grep: c.Issues[0]})
		}
		return fleetDetailMsg{detail: d, dir: dir, issue: issue, err: err}
	}
}

// Update handles one message and reports what the Model must do.
func (b FleetBrowser) Update(msg tea.Msg) (FleetBrowser, tea.Cmd, fleetOutcome) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = msg.Width, msg.Height
	case fleetTickMsg:
		cmds := []tea.Cmd{b.scanCmd()}
		if v := b.view(); v == fleetLogs {
			cmds = append(cmds, b.eventsCmd(b.selected))
		}
		return b, tea.Batch(cmds...), fleetOutcome{}
	case fleetScanMsg:
		b.snap, b.scanned = msg.snap, true
		if b.deps.Collector != nil {
			b.activity = b.deps.Collector.Activity(b.deps.Now(), 36, 10*time.Minute)
		}
		b.clampCursors()
		return b, fleetTick(), fleetOutcome{}
	case fleetEventsMsg:
		if msg.id != b.selected {
			return b, nil, fleetOutcome{}
		}
		b.logErr = ""
		if msg.err != nil {
			b.logErr = msg.err.Error()
		}
		b.events = msg.events
		if b.follow {
			b.logCursor = len(b.filteredEvents()) - 1
		}
		b.clampCursors()
	case fleetContextMsg:
		if msg.id == b.selected {
			b.ctx = msg.rep
			b.clampCursors()
		}
	case fleetCommitsMsg:
		b.commitsRead = true
		b.commits, b.commitErr = msg.commits, ""
		if msg.err != nil {
			b.commitErr = msg.err.Error()
		}
		b.clampCursors()
	case fleetDetailMsg:
		b.detail, b.detailDir, b.detailIssue, b.detailErr = msg.detail, msg.dir, msg.issue, ""
		if msg.err != nil {
			b.detailErr = msg.err.Error()
		}
	case fleetTextMsg:
		b.textTitle, b.text, b.textScroll = msg.title, strings.Split(msg.text, "\n"), 0
	case fleetStatusMsg:
		b.status = msg.text
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			b.move(-3)
		case tea.MouseButtonWheelDown:
			b.move(3)
		}
	case tea.KeyMsg:
		return b.handleKey(msg)
	}
	return b, nil, fleetOutcome{}
}

func (b FleetBrowser) handleKey(msg tea.KeyMsg) (FleetBrowser, tea.Cmd, fleetOutcome) {
	key := msg.String()
	if b.prompt != fleetPromptNone {
		return b.handlePromptKey(msg)
	}
	if key != "ctrl+k" {
		b.stopArm = ""
	}
	b.status = ""
	switch key {
	case "q", "esc":
		if b.view() == fleetLogs && b.expanded {
			b.expanded = false
			return b, nil, fleetOutcome{}
		}
		if b.view() == fleetLogs && b.fullscreen {
			b.fullscreen = false
			return b, nil, fleetOutcome{}
		}
		if b.filter != "" && key == "esc" {
			b.filter = ""
			return b, nil, fleetOutcome{}
		}
		if len(b.stack) > 1 {
			b.stack = b.stack[:len(b.stack)-1]
			return b, nil, fleetOutcome{}
		}
		return b, nil, fleetOutcome{close: true}
	case ":":
		b.prompt, b.input = fleetPromptCommand, ""
		return b, nil, fleetOutcome{}
	case "/":
		b.prompt, b.input = fleetPromptFilter, b.filter
		return b, nil, fleetOutcome{}
	case "tab", "shift+tab":
		if len(b.stack) == 1 {
			i := 0
			for j, v := range fleetRootViews {
				if v == b.view() {
					i = j
				}
			}
			step := 1
			if key == "shift+tab" {
				step = len(fleetRootViews) - 1
			}
			return b.switchRoot(fleetRootViews[(i+step)%len(fleetRootViews)])
		}
	case "j", "down":
		b.move(1)
		return b, nil, fleetOutcome{}
	case "k", "up":
		b.move(-1)
		return b, nil, fleetOutcome{}
	case "ctrl+d", "pgdown":
		b.move(b.pageSize())
		return b, nil, fleetOutcome{}
	case "ctrl+u", "pgup":
		b.move(-b.pageSize())
		return b, nil, fleetOutcome{}
	case "g", "home":
		b.move(-1 << 30)
		return b, nil, fleetOutcome{}
	case "G", "end":
		b.move(1 << 30)
		return b, nil, fleetOutcome{}
	}
	switch b.view() {
	case fleetAgents:
		return b.agentsKey(key)
	case fleetLogs:
		return b.logsKey(key)
	case fleetContext:
		return b.contextKey(key)
	case fleetXray:
		return b.xrayKey(key)
	case fleetPulse:
		return b, nil, fleetOutcome{}
	case fleetCommits:
		return b.commitsKey(key)
	case fleetCommit:
		return b.commitKey(key)
	}
	return b, nil, fleetOutcome{}
}

func (b FleetBrowser) handlePromptKey(msg tea.KeyMsg) (FleetBrowser, tea.Cmd, fleetOutcome) {
	switch msg.Type {
	case tea.KeyEsc:
		if b.prompt == fleetPromptFilter {
			b.filter = ""
		}
		b.prompt = fleetPromptNone
		return b, nil, fleetOutcome{}
	case tea.KeyEnter:
		kind, text := b.prompt, strings.TrimSpace(b.input)
		b.prompt = fleetPromptNone
		switch kind {
		case fleetPromptFilter:
			b.filter = text
			b.clampCursors()
		case fleetPromptCommand:
			if v, ok := fleetCommandView(text); ok {
				return b.switchRoot(v)
			}
			if text != "" {
				return b, nil, fleetOutcome{close: true, command: text}
			}
		case fleetPromptInject:
			return b, b.inject(text), fleetOutcome{}
		}
		return b, nil, fleetOutcome{}
	case tea.KeyBackspace:
		if r := []rune(b.input); len(r) > 0 {
			b.input = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		b.input += string(msg.Runes)
		if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
			b.input += " "
		}
	}
	if b.prompt == fleetPromptFilter {
		b.filter = b.input
		b.clampCursors()
	}
	return b, nil, fleetOutcome{}
}

// fleetCommandView maps a ':' command typed inside the fleet browser to the
// view it opens.
func fleetCommandView(text string) (fleetView, bool) {
	cmd, err := ResolveCommand(text)
	if err != nil || cmd.Kind != CommandFleet {
		return 0, false
	}
	return cmd.FleetView, true
}

func (b FleetBrowser) switchRoot(v fleetView) (FleetBrowser, tea.Cmd, fleetOutcome) {
	b.stack = []fleetView{v}
	b.filter = ""
	if v == fleetCommits && !b.commitsRead {
		return b, b.commitsCmd(), fleetOutcome{}
	}
	return b, nil, fleetOutcome{}
}

func (b *FleetBrowser) push(v fleetView) {
	b.stack = append(b.stack, v)
}

func (b FleetBrowser) pageSize() int {
	if n := b.height - 8; n > 1 {
		return n
	}
	return 1
}

// visibleSessions applies the project filter, the text filter and the sort.
func (b FleetBrowser) visibleSessions() []agents.Session {
	var out []agents.Session
	f := strings.ToLower(b.filter)
	for _, s := range b.snap.Sessions {
		if b.project != "" && s.Project != b.project {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(strings.Join([]string{s.Name, s.Project, s.TaskID, s.Role, string(s.Tool), string(s.State), s.Branch}, " ")), f) {
			continue
		}
		out = append(out, s)
	}
	rank := map[agents.State]int{agents.StateFailed: 0, agents.StateWaiting: 1, agents.StateRunning: 2, agents.StatePending: 3, agents.StateIdle: 4, agents.StateDone: 5}
	sort.SliceStable(out, func(i, j int) bool {
		a, c := out[i], out[j]
		switch b.sortBy {
		case fleetSortContext:
			return a.ContextPercent() > c.ContextPercent()
		case fleetSortProject:
			if a.Project != c.Project {
				return a.Project < c.Project
			}
		case fleetSortState:
			if rank[a.State] != rank[c.State] {
				return rank[a.State] < rank[c.State]
			}
		}
		if a.Live() != c.Live() {
			return a.Live()
		}
		return a.LastActivity.After(c.LastActivity)
	})
	return out
}

// projects lists the distinct projects of the scanned sessions; keys 1 to 9
// pick one of the first nine.
func (b FleetBrowser) projects() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range b.snap.Sessions {
		if s.Project != "" && !seen[s.Project] {
			seen[s.Project] = true
			out = append(out, s.Project)
		}
	}
	sort.Strings(out)
	return out
}

func (b FleetBrowser) currentSession() (agents.Session, bool) {
	list := b.visibleSessions()
	if b.cursor >= 0 && b.cursor < len(list) {
		return list[b.cursor], true
	}
	return agents.Session{}, false
}

func (b FleetBrowser) sessionByID(id string) (agents.Session, bool) {
	for _, s := range b.snap.Sessions {
		if s.ID == id {
			return s, true
		}
	}
	return agents.Session{}, false
}

func (b FleetBrowser) agentsKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	s, ok := b.currentSession()
	switch key {
	case "s":
		b.sortBy = (b.sortBy + 1) % fleetSortCount
		b.status = "sort: " + fleetSortNames[b.sortBy]
	case "0":
		b.project = ""
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		ps := b.projects()
		if i := int(key[0] - '1'); i < len(ps) {
			b.project = ps[i]
			b.cursor = 0
		}
	case "enter", "l":
		if ok {
			return b.openLogs(s)
		}
	case "c":
		if ok {
			return b.openContext(s)
		}
	case "d":
		if ok {
			b.textTitle, b.text, b.textScroll = "describe "+s.Name, describeSession(s), 0
			b.push(fleetText)
		}
	case "t":
		if ok && s.TaskID != "" {
			return b, nil, fleetOutcome{close: true, openIssue: s.TaskID}
		}
		b.status = "this session names no task"
	case "a":
		if ok {
			return b, b.attach(s), fleetOutcome{}
		}
	case "ctrl+k":
		if !ok {
			break
		}
		if s.Where.Kind != agents.WhereTmux {
			b.status = "stop needs a tmux pane"
			break
		}
		if b.stopArm != s.ID {
			b.stopArm = s.ID
			b.status = "press ctrl+k again to send Escape to " + s.Where.Label
			break
		}
		b.stopArm = ""
		return b, b.sendKeys(s, "Escape"), fleetOutcome{}
	}
	return b, nil, fleetOutcome{}
}

func (b FleetBrowser) openLogs(s agents.Session) (FleetBrowser, tea.Cmd, fleetOutcome) {
	b.selected, b.events, b.logCursor, b.follow, b.expanded, b.logErr = s.ID, nil, 0, true, false, ""
	b.push(fleetLogs)
	return b, b.eventsCmd(s.ID), fleetOutcome{}
}

func (b FleetBrowser) openContext(s agents.Session) (FleetBrowser, tea.Cmd, fleetOutcome) {
	b.selected, b.ctx, b.ctxCursor = s.ID, agents.ContextReport{}, 0
	b.push(fleetContext)
	return b, b.contextCmd(s.ID), fleetOutcome{}
}

func (b FleetBrowser) logsKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	switch key {
	case "1", "2", "3", "4", "5":
		b.logFilter = int(key[0] - '1')
		b.logCursor = len(b.filteredEvents()) - 1
	case "s":
		b.follow = !b.follow
		if b.follow {
			b.logCursor = len(b.filteredEvents()) - 1
		}
	case "w":
		b.wrap = !b.wrap
	case "t":
		b.hideTime = !b.hideTime
	case "f":
		b.fullscreen = !b.fullscreen
	case "x", "enter":
		b.expanded = !b.expanded
	case "c":
		if b.deps.Clipboard == nil {
			b.status = "no clipboard"
			break
		}
		if err := b.deps.Clipboard(b.logText()); err != nil {
			b.status = "copy: " + err.Error()
			break
		}
		b.status = fmt.Sprintf("copied %d log lines", len(b.filteredEvents()))
	case "ctrl+s":
		return b, b.saveLogCmd(), fleetOutcome{}
	case "C":
		if s, ok := b.sessionByID(b.selected); ok {
			return b.openContext(s)
		}
	case "i":
		s, ok := b.sessionByID(b.selected)
		if !ok || s.Where.Kind != agents.WhereTmux {
			b.status = "inject needs a tmux pane"
			break
		}
		b.prompt, b.input = fleetPromptInject, ""
	case "T":
		if s, ok := b.sessionByID(b.selected); ok && s.TaskID != "" {
			return b, nil, fleetOutcome{close: true, openIssue: s.TaskID}
		}
	}
	return b, nil, fleetOutcome{}
}

// logText is the filtered log as plain text, one event per line, the way
// copy and save hand it out.
func (b FleetBrowser) logText() string {
	var sb strings.Builder
	for _, e := range b.filteredEvents() {
		fmt.Fprintf(&sb, "%s %-6s %-8s %s\n", e.Time.Local().Format("2006-01-02 15:04:05"), e.Kind, e.Tool, e.Text)
	}
	return sb.String()
}

// saveLogCmd writes the filtered log under the temp directory, as k9s does
// with ctrl+s, and reports the path.
func (b FleetBrowser) saveLogCmd() tea.Cmd {
	s, _ := b.sessionByID(b.selected)
	name := strings.Map(func(r rune) rune {
		if r == '/' || r == os.PathSeparator || r == ' ' {
			return '-'
		}
		return r
	}, s.Name)
	if name == "" {
		name = "session"
	}
	text := b.logText()
	stamp := b.deps.Now().Format("20060102-150405")
	return func() tea.Msg {
		dir := filepath.Join(os.TempDir(), "b9s-logs")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fleetStatusMsg{text: "save: " + err.Error()}
		}
		path := filepath.Join(dir, name+"-"+stamp+".log")
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			return fleetStatusMsg{text: "save: " + err.Error()}
		}
		return fleetStatusMsg{text: "saved " + path}
	}
}

func (b FleetBrowser) filteredEvents() []agents.Event {
	kinds := fleetLogFilters[b.logFilter].kinds
	f := strings.ToLower(b.filter)
	var out []agents.Event
	for _, e := range b.events {
		if kinds != nil && !containsKind(kinds, e.Kind) {
			continue
		}
		if f != "" && !strings.Contains(strings.ToLower(e.Text), f) {
			continue
		}
		out = append(out, e)
	}
	return out
}

func containsKind(kinds []agents.EventKind, k agents.EventKind) bool {
	for _, x := range kinds {
		if x == k {
			return true
		}
	}
	return false
}

// ctxRow is one line of the context tree: a layer heading or a source.
type ctxRow struct {
	layer  agents.Layer
	tokens int
	source *agents.Source
}

func (b FleetBrowser) contextRows() []ctxRow {
	var rows []ctxRow
	for _, l := range b.ctx.Layers {
		if l.Tokens == 0 {
			continue
		}
		rows = append(rows, ctxRow{layer: l.Layer, tokens: l.Tokens})
		if b.ctxFold[l.Layer] {
			continue
		}
		for i := range b.ctx.Sources {
			if s := &b.ctx.Sources[i]; s.Layer == l.Layer {
				rows = append(rows, ctxRow{layer: l.Layer, tokens: s.Tokens, source: s})
			}
		}
	}
	return rows
}

func (b FleetBrowser) contextKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	rows := b.contextRows()
	if b.ctxCursor >= len(rows) {
		return b, nil, fleetOutcome{}
	}
	row := rows[b.ctxCursor]
	switch key {
	case "enter", "h", "l":
		if row.source == nil {
			b.ctxFold[row.layer] = !b.ctxFold[row.layer]
			return b, nil, fleetOutcome{}
		}
		if row.source.Path != "" {
			path := row.source.Path
			b.push(fleetText)
			return b, readFileCmd(path), fleetOutcome{}
		}
	case "w":
		if row.source != nil {
			b.push(fleetText)
			return b, b.whoElseCmd(*row.source), fleetOutcome{}
		}
	case "D":
		if row.source != nil && row.source.Path != "" {
			b.push(fleetText)
			return b, diffSinceCmd(*row.source), fleetOutcome{}
		}
	}
	return b, nil, fleetOutcome{}
}

func readFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(path)
		if err != nil {
			return fleetTextMsg{title: path, text: err.Error()}
		}
		return fleetTextMsg{title: path, text: string(data)}
	}
}

// diffSinceCmd shows what changed in a source file after the session loaded
// it: the commits since then plus any uncommitted change.
func diffSinceCmd(s agents.Source) tea.Cmd {
	return func() tea.Msg {
		dir := filepath.Dir(s.Path)
		since := s.LoadedAt.Format(time.RFC3339)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		logOut, _ := exec.CommandContext(ctx, "git", "-C", dir, "log", "--no-color", "-p", "--since="+since, "--", s.Path).Output()
		work, _ := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--no-color", "HEAD", "--", s.Path).Output()
		text := strings.TrimSpace(string(logOut) + "\n" + string(work))
		if text == "" {
			text = "No change in git since " + s.LoadedAt.Format("15:04") + ". The file may sit outside a repository."
		}
		return fleetTextMsg{title: "changes to " + s.Name + " since " + s.LoadedAt.Format("15:04"), text: text}
	}
}

// whoElseCmd lists the live sessions whose context holds the same source.
func (b FleetBrowser) whoElseCmd(src agents.Source) tea.Cmd {
	c := b.deps.Collector
	sessions := b.snap.Sessions
	return func() tea.Msg {
		var lines []string
		for _, s := range sessions {
			if !s.Live() || c == nil {
				continue
			}
			rep, ok := c.Context(s.ID)
			if !ok {
				continue
			}
			for _, x := range rep.Sources {
				if (src.Path != "" && x.Path == src.Path) || (src.Path == "" && x.Layer == src.Layer && x.Name == src.Name) {
					mark := ""
					if x.Stale {
						mark = "  stale"
					}
					lines = append(lines, fmt.Sprintf("%-28s %-16s %-8s %6s%s", s.Name, s.Project, s.State, fmtTokens(x.Tokens), mark))
					break
				}
			}
		}
		if len(lines) == 0 {
			lines = []string{"No other live session loaded this source."}
		}
		return fleetTextMsg{title: "sessions that loaded " + src.Name, text: strings.Join(lines, "\n")}
	}
}

// xrayRow is one line of the xray tree: project, task or session.
type xrayRow struct {
	depth   int
	project string
	task    string
	session *agents.Session
	count   int
}

func (b FleetBrowser) xrayRows() []xrayRow {
	byProject := map[string]map[string][]agents.Session{}
	for _, s := range b.snap.Sessions {
		if !s.Live() {
			continue
		}
		p := s.Project
		if p == "" {
			p = "(no project)"
		}
		if byProject[p] == nil {
			byProject[p] = map[string][]agents.Session{}
		}
		byProject[p][s.TaskID] = append(byProject[p][s.TaskID], s)
	}
	var projects []string
	for p := range byProject {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	var rows []xrayRow
	for _, p := range projects {
		n := 0
		for _, ss := range byProject[p] {
			n += len(ss)
		}
		rows = append(rows, xrayRow{depth: 0, project: p, count: n})
		var tasks []string
		for t := range byProject[p] {
			tasks = append(tasks, t)
		}
		sort.Strings(tasks)
		for _, t := range tasks {
			rows = append(rows, xrayRow{depth: 1, project: p, task: t, count: len(byProject[p][t])})
			for i := range byProject[p][t] {
				s := byProject[p][t][i]
				rows = append(rows, xrayRow{depth: 2, project: p, task: t, session: &s})
			}
		}
	}
	return rows
}

func (b FleetBrowser) xrayKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	rows := b.xrayRows()
	if b.xrayCursor >= len(rows) {
		return b, nil, fleetOutcome{}
	}
	row := rows[b.xrayCursor]
	switch key {
	case "enter", "l":
		if row.session != nil {
			return b.openLogs(*row.session)
		}
		if row.depth == 1 && row.task != "" {
			return b, nil, fleetOutcome{close: true, openIssue: row.task}
		}
	case "c":
		if row.session != nil {
			return b.openContext(*row.session)
		}
	case "t":
		if row.task != "" {
			return b, nil, fleetOutcome{close: true, openIssue: row.task}
		}
	}
	return b, nil, fleetOutcome{}
}

func (b FleetBrowser) visibleCommits() []gitlog.Commit {
	if b.filter == "" {
		return b.commits
	}
	f := strings.ToLower(b.filter)
	var out []gitlog.Commit
	for _, c := range b.commits {
		hay := strings.ToLower(strings.Join(append([]string{c.Hash, c.Subject, c.Author, c.Repo}, c.Issues...), " "))
		if strings.Contains(hay, f) {
			out = append(out, c)
		}
	}
	return out
}

func (b FleetBrowser) commitsKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	list := b.visibleCommits()
	var cur *gitlog.Commit
	if b.commitCursor < len(list) {
		cur = &list[b.commitCursor]
	}
	switch key {
	case "enter", "l":
		if cur != nil {
			b.detail, b.detailIssue, b.detailScroll, b.detailErr = gitlog.Detail{Commit: *cur}, nil, 0, ""
			b.push(fleetCommit)
			return b, b.detailCmd(*cur), fleetOutcome{}
		}
	case "U":
		b.showAuthor = !b.showAuthor
	case "b":
		b.allBranches = !b.allBranches
		b.status = map[bool]string{true: "all branches", false: "HEAD only"}[b.allBranches]
		return b, b.commitsCmd(), fleetOutcome{}
	case "0":
		b.allRepos = !b.allRepos
		b.commitCursor = 0
		b.status = map[bool]string{true: "all repositories", false: "active project only"}[b.allRepos]
		return b, b.commitsCmd(), fleetOutcome{}
	case "t":
		if cur != nil && len(cur.Issues) > 0 {
			return b, nil, fleetOutcome{close: true, openIssue: cur.Issues[0]}
		}
		b.status = "this commit has no Refs: trailer"
	case "c":
		if cur != nil && b.deps.Clipboard != nil {
			if err := b.deps.Clipboard(cur.Hash); err != nil {
				b.status = "copy: " + err.Error()
			} else {
				b.status = "copied " + cur.Short
			}
		}
	case "r":
		return b, b.commitsCmd(), fleetOutcome{}
	}
	return b, nil, fleetOutcome{}
}

func (b FleetBrowser) commitKey(key string) (FleetBrowser, tea.Cmd, fleetOutcome) {
	c := b.detail.Commit
	switch key {
	case "t":
		if len(c.Issues) > 0 {
			return b, nil, fleetOutcome{close: true, openIssue: c.Issues[0]}
		}
	case "l":
		if ss := b.commitAgents(c); len(ss) > 0 {
			return b.openLogs(ss[0])
		}
		b.status = "no agent session matches this commit"
	case "c":
		if b.deps.Clipboard != nil {
			_ = b.deps.Clipboard(c.Hash)
			b.status = "copied " + c.Short
		}
	}
	return b, nil, fleetOutcome{}
}

// commitAgents lists the sessions that were active in the commit's
// repository when the commit was made. Transcripts do not record commit
// hashes, so time and project are the link.
func (b FleetBrowser) commitAgents(c gitlog.Commit) []agents.Session {
	project := c.Repo
	if project == "" {
		project = agents.ProjectFromCwd(b.deps.RepoDir)
	}
	var out []agents.Session
	for _, s := range b.snap.Sessions {
		if s.Project != project || s.Started.IsZero() {
			continue
		}
		if !c.Time.Before(s.Started.Add(-time.Minute)) && !c.Time.After(s.LastActivity.Add(2*time.Minute)) {
			out = append(out, s)
		}
	}
	return out
}

func (b *FleetBrowser) move(delta int) {
	switch b.view() {
	case fleetAgents:
		b.cursor += delta
	case fleetLogs:
		b.logCursor += delta
		b.follow = b.logCursor >= len(b.filteredEvents())-1 && delta > 0
	case fleetContext:
		b.ctxCursor += delta
	case fleetXray:
		b.xrayCursor += delta
	case fleetCommits:
		b.commitCursor += delta
	case fleetCommit:
		b.detailScroll += delta
	case fleetText:
		b.textScroll += delta
	}
	b.clampCursors()
}

func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (b *FleetBrowser) clampCursors() {
	b.cursor = clampInt(b.cursor, 0, len(b.visibleSessions())-1)
	b.logCursor = clampInt(b.logCursor, 0, len(b.filteredEvents())-1)
	b.ctxCursor = clampInt(b.ctxCursor, 0, len(b.contextRows())-1)
	b.xrayCursor = clampInt(b.xrayCursor, 0, len(b.xrayRows())-1)
	b.commitCursor = clampInt(b.commitCursor, 0, len(b.visibleCommits())-1)
	b.detailScroll = clampInt(b.detailScroll, 0, len(strings.Split(b.detail.Diff, "\n"))-1)
	b.textScroll = clampInt(b.textScroll, 0, len(b.text)-1)
}

func testMode() bool { return os.Getenv("B9S_TEST_MODE") != "" }

// attach switches the tmux client to the session's pane.
func (b FleetBrowser) attach(s agents.Session) tea.Cmd {
	if s.Where.Kind != agents.WhereTmux {
		return func() tea.Msg { return fleetStatusMsg{"attach needs a tmux pane"} }
	}
	run := b.deps.Exec
	return func() tea.Msg {
		if os.Getenv("TMUX") == "" && !testMode() {
			return fleetStatusMsg{"not inside tmux: run tmux attach -t " + s.Where.Label}
		}
		if err := run("tmux", "switch-client", "-t", s.Where.Ref); err != nil {
			return fleetStatusMsg{"tmux: " + err.Error()}
		}
		return fleetStatusMsg{"switched to " + s.Where.Label}
	}
}

func (b FleetBrowser) sendKeys(s agents.Session, keys ...string) tea.Cmd {
	run := b.deps.Exec
	return func() tea.Msg {
		args := append([]string{"send-keys", "-t", s.Where.Ref}, keys...)
		if err := run("tmux", args...); err != nil {
			return fleetStatusMsg{"tmux: " + err.Error()}
		}
		return fleetStatusMsg{"sent " + strings.Join(keys, " ") + " to " + s.Where.Label}
	}
}

// inject types text into the session's pane and presses Enter, as if the
// person typed it there.
func (b FleetBrowser) inject(text string) tea.Cmd {
	s, ok := b.sessionByID(b.selected)
	if !ok || text == "" || s.Where.Kind != agents.WhereTmux {
		return nil
	}
	run := b.deps.Exec
	return func() tea.Msg {
		if err := run("tmux", "send-keys", "-t", s.Where.Ref, "-l", text); err != nil {
			return fleetStatusMsg{"tmux: " + err.Error()}
		}
		if err := run("tmux", "send-keys", "-t", s.Where.Ref, "Enter"); err != nil {
			return fleetStatusMsg{"tmux: " + err.Error()}
		}
		return fleetStatusMsg{"sent to " + s.Where.Label}
	}
}

func describeSession(s agents.Session) []string {
	row := func(k, v string) string { return fmt.Sprintf("%-14s %s", k+":", v) }
	ts := func(t time.Time) string {
		if t.IsZero() {
			return "-"
		}
		return t.Local().Format("2006-01-02 15:04:05")
	}
	return []string{
		row("Name", s.Name), row("ID", s.ID), row("Tool", string(s.Tool)), row("Model", s.Model),
		row("State", string(s.State)), row("Project", s.Project), row("Cwd", s.Cwd), row("Branch", s.Branch),
		row("Role", s.Role), row("Task", s.TaskID),
		row("Context", fmt.Sprintf("%s / %s (%d%%)", fmtTokens(s.ContextTokens), fmtTokens(s.ContextLimit), s.ContextPercent())),
		row("Tokens/min", fmt.Sprint(s.TokensPerMin)), row("Output total", fmtTokens(s.TotalTokens)),
		row("Compactions", fmt.Sprint(s.Compactions)), row("Tool calls", fmt.Sprint(s.Tools)), row("Errors", fmt.Sprint(s.Errors)),
		row("Started", ts(s.Started)), row("Last activity", ts(s.LastActivity)),
		row("Where", strings.TrimSpace(string(s.Where.Kind)+" "+s.Where.Label+" "+s.Where.Ref)),
		row("Transcript", s.Transcript),
	}
}

func fmtTokens(n int) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func fmtAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
