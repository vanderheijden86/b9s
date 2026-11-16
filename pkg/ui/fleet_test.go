package ui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/pkg/config"
)

var fleetNow = time.Date(2026, 11, 15, 10, 12, 0, 0, time.UTC)

// fleetFixture writes the agents package's Claude transcript into a temporary
// home and returns a collector over it, with one tmux pane on the session.
func fleetFixture(t *testing.T) (*agents.Collector, string) {
	t.Helper()
	home := t.TempDir()
	project := filepath.Join(home, "work", "b9s")
	data, err := os.ReadFile("../../internal/agents/testdata/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.ReplaceAll(string(data), "/work/b9s/.worktrees/feat-x", project)
	body = strings.ReplaceAll(body, "2026-11-15T09:", "2026-11-15T10:")
	path := filepath.Join(home, ".claude", "projects", agents.ClaudeProjectDir(project), "s-claude-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, fleetNow, fleetNow)
	c := agents.NewCollector(agents.Options{Home: home, Now: func() time.Time { return fleetNow }, Panes: func(context.Context) []agents.Pane {
		return []agents.Pane{{ID: "%7", Target: "main:3.0", Window: "review-thing", Path: project, Command: "2.1.325"}}
	}})
	return c, project
}

type execCall struct {
	name string
	args []string
}

func newFleetTest(t *testing.T, view fleetView) (FleetBrowser, *[]execCall) {
	t.Helper()
	t.Setenv("B9S_TEST_MODE", "1")
	c, _ := fleetFixture(t)
	calls := &[]execCall{}
	deps := FleetDeps{
		Collector: c,
		Now:       func() time.Time { return fleetNow },
		Issue: func(id string) (string, string, bool) {
			if id == "bd-ab12.3" {
				return "Review the thing", "in_progress", true
			}
			return "", "", false
		},
		Exec: func(name string, args ...string) error {
			*calls = append(*calls, execCall{name, args})
			return nil
		},
	}
	b := NewFleetBrowser(deps, Theme{Renderer: lipgloss.DefaultRenderer()}, 140, 30, view)
	b, _, _ = b.Update(fleetScanMsg{snap: c.Scan(context.Background())})
	return b, calls
}

func fleetKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// drive presses keys and runs each returned command once, feeding its
// message back, so a key that reads data finishes before the next key.
func fleetDrive(t *testing.T, b FleetBrowser, keys ...string) (FleetBrowser, fleetOutcome) {
	t.Helper()
	var out fleetOutcome
	for _, k := range keys {
		var cmd tea.Cmd
		b, cmd, out = b.Update(fleetKey(k))
		b = runFleetCmd(b, cmd)
	}
	return b, out
}

func runFleetCmd(b FleetBrowser, cmd tea.Cmd) FleetBrowser {
	if cmd == nil {
		return b
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			b = runFleetCmd(b, c)
		}
		return b
	}
	if _, tick := msg.(fleetTickMsg); tick {
		return b
	}
	b, _, _ = b.Update(msg)
	return b
}

func TestFleetCommandsResolve(t *testing.T) {
	for text, want := range map[string]fleetView{"agents": fleetAgents, "ag": fleetAgents, "xray": fleetXray, "pulse": fleetPulse, "commits": fleetCommits, "git": fleetCommits} {
		cmd, err := ResolveCommand(text)
		if err != nil || cmd.Kind != CommandFleet || cmd.FleetView != want {
			t.Errorf("%s: %+v %v", text, cmd, err)
		}
	}
	if _, err := ResolveCommand("agents extra"); err == nil {
		t.Error("agents takes no arguments")
	}
}

func TestFleetAgentsTableShowsTheSession(t *testing.T) {
	b, _ := newFleetTest(t, fleetAgents)
	v := b.View()
	for _, want := range []string{"review-thing", "b9s", "claude", "bd-ab12.3", "waiting", "main:3.0", "1 sessions"} {
		if !strings.Contains(v, want) {
			t.Errorf("agents view lacks %q:\n%s", want, v)
		}
	}
}

func TestFleetLogsAndContextOpenFromTheTable(t *testing.T) {
	b, _ := newFleetTest(t, fleetAgents)
	b, _ = fleetDrive(t, b, "enter")
	if b.view() != fleetLogs || len(b.events) == 0 {
		t.Fatalf("view %v with %d events", b.view(), len(b.events))
	}
	if v := b.View(); !strings.Contains(v, "TASK") || !strings.Contains(v, "following") {
		t.Errorf("logs view:\n%s", v)
	}
	all := len(b.filteredEvents())
	b, _ = fleetDrive(t, b, "3")
	for _, e := range b.filteredEvents() {
		if e.Kind != agents.EventTool && e.Kind != agents.EventResult && e.Kind != agents.EventBD {
			t.Errorf("tools filter kept %s", e.Kind)
		}
	}
	if len(b.filteredEvents()) >= all {
		t.Error("tools filter dropped nothing")
	}
	b, _ = fleetDrive(t, b, "C")
	if b.view() != fleetContext || b.ctx.SessionID != "s-claude-1" {
		t.Fatalf("context view %v %q", b.view(), b.ctx.SessionID)
	}
	if v := b.View(); !strings.Contains(v, "Instructions") || !strings.Contains(v, "window") {
		t.Errorf("context view:\n%s", v)
	}
	b, out := fleetDrive(t, b, "esc", "esc", "esc")
	if !out.close {
		t.Errorf("third esc should close, stack %v", b.stack)
	}
}

func TestFleetAttachAndStopDriveTmux(t *testing.T) {
	b, calls := newFleetTest(t, fleetAgents)
	b, _ = fleetDrive(t, b, "a")
	if len(*calls) != 1 || strings.Join((*calls)[0].args, " ") != "switch-client -t %7" {
		t.Fatalf("attach calls = %+v", *calls)
	}
	b, _ = fleetDrive(t, b, "ctrl+k")
	if len(*calls) != 1 || !strings.Contains(b.status, "again") {
		t.Fatalf("one ctrl+k must only arm: %+v %q", *calls, b.status)
	}
	fleetDrive(t, b, "ctrl+k")
	if len(*calls) != 2 || strings.Join((*calls)[1].args, " ") != "send-keys -t %7 Escape" {
		t.Fatalf("stop calls = %+v", *calls)
	}
}

func TestFleetTaskKeyOpensTheIssue(t *testing.T) {
	b, _ := newFleetTest(t, fleetAgents)
	_, out := fleetDrive(t, b, "t")
	if !out.close || out.openIssue != "bd-ab12.3" {
		t.Errorf("outcome = %+v", out)
	}
}

func TestFleetXrayGroupsByProjectAndTask(t *testing.T) {
	b, _ := newFleetTest(t, fleetXray)
	v := b.View()
	for _, want := range []string{"▾ b9s", "bd-ab12.3", "Review the thing", "review-thing"} {
		if !strings.Contains(v, want) {
			t.Errorf("xray lacks %q:\n%s", want, v)
		}
	}
}

func TestFleetPulseRendersSixPanels(t *testing.T) {
	b, _ := newFleetTest(t, fleetPulse)
	v := b.View()
	for _, want := range []string{"Agents running", "Tasks closed per hour", "Tokens per minute", "Failures", "Context pressure", "Waiting on you"} {
		if !strings.Contains(v, want) {
			t.Errorf("pulse lacks %q", want)
		}
	}
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Ann", "GIT_AUTHOR_EMAIL=a@x", "GIT_COMMITTER_NAME=Ann", "GIT_COMMITTER_EMAIL=a@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "chore: start")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644)
	run("commit", "-q", "-am", "feat: add two\n\nRefs: bd-ab12.3")
	return dir
}

func TestFleetCommitsAndDetail(t *testing.T) {
	b, _ := newFleetTest(t, fleetCommits)
	b.deps.RepoDir = gitRepo(t)
	b = runFleetCmd(b, b.commitsCmd())
	v := b.View()
	for _, want := range []string{"feat: add two", "bd-ab12.3", "⚠ no Refs", "chore: start"} {
		if !strings.Contains(v, want) {
			t.Errorf("commits lack %q:\n%s", want, v)
		}
	}
	b, _ = fleetDrive(t, b, "enter")
	if b.view() != fleetCommit || len(b.detail.Files) != 1 {
		t.Fatalf("detail view %v files %+v err %q", b.view(), b.detail.Files, b.detailErr)
	}
	v = b.View()
	for _, want := range []string{"a.txt", "+two", "Review the thing", "Made by"} {
		if !strings.Contains(v, want) {
			t.Errorf("detail lacks %q:\n%s", want, v)
		}
	}
	_, out := fleetDrive(t, b, "t")
	if out.openIssue != "bd-ab12.3" {
		t.Errorf("t outcome = %+v", out)
	}
}

func TestFleetCommandPromptSwitchesViews(t *testing.T) {
	b, _ := newFleetTest(t, fleetAgents)
	b, _ = fleetDrive(t, b, ":", "x", "r", "a", "y", "enter")
	if b.view() != fleetXray {
		t.Errorf("view = %v", b.view())
	}
	_, out := fleetDrive(t, b, ":", "e", "p", "i", "c", "enter")
	if !out.close || out.command != "epic" {
		t.Errorf("a b9s command closes the fleet and runs: %+v", out)
	}
}

func TestModelOpensAndClosesTheFleet(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	c, _ := fleetFixture(t)
	m := newTestModel()
	m.width, m.height, m.ready = 120, 30, true
	m.SetFleetCollector(c)
	m, cmd := m.executeCommand(Command{Kind: CommandFleet, FleetView: fleetAgents})
	if !m.FleetVisible() || cmd == nil {
		t.Fatal("fleet not opened")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(m.View(), "review-thing") {
		t.Errorf("model view:\n%s", m.View())
	}
	next, _ = m.Update(fleetKey("q"))
	m = next.(Model)
	if m.FleetVisible() {
		t.Error("q should close the fleet")
	}
}

// logsWith opens the logs view of the fixture session and replaces its events.
func logsWith(t *testing.T, evs ...agents.Event) FleetBrowser {
	t.Helper()
	b, _ := newFleetTest(t, fleetAgents)
	b, _ = fleetDrive(t, b, "enter")
	b.events = evs
	b.logCursor = len(evs) - 1
	return b
}

func TestFleetLogsWrapKeyShowsTheWholeLine(t *testing.T) {
	long := strings.Repeat("word ", 60) + "TAILMARK"
	b := logsWith(t, agents.Event{Time: fleetNow, Kind: agents.EventAI, Text: long})
	if strings.Contains(b.View(), "TAILMARK") {
		t.Fatal("an unwrapped line must be cut at the screen edge")
	}
	b, _ = fleetDrive(t, b, "w")
	v := b.View()
	if !strings.Contains(v, "TAILMARK") || !strings.Contains(v, "wrap") {
		t.Errorf("w must wrap the line onto continuation rows:\n%s", v)
	}
	for _, ln := range strings.Split(v, "\n") {
		if lipgloss.Width(ln) > b.width {
			t.Errorf("row wider than the screen: %q", ln)
		}
	}
}

func TestFleetLogsTimestampKeyHidesTheTime(t *testing.T) {
	b := logsWith(t, agents.Event{Time: fleetNow, Kind: agents.EventAI, Text: "hello"})
	stamp := fleetNow.Local().Format("15:04:05")
	if !strings.Contains(b.View(), stamp) {
		t.Fatalf("timestamps show by default")
	}
	b, _ = fleetDrive(t, b, "t")
	if strings.Contains(b.View(), stamp) {
		t.Errorf("t must hide timestamps:\n%s", b.View())
	}
}

func TestFleetLogsAutoscrollAndFullscreenKeys(t *testing.T) {
	b := logsWith(t, agents.Event{Time: fleetNow, Kind: agents.EventAI, Text: "hello"})
	b, _ = fleetDrive(t, b, "s")
	if b.follow || strings.Contains(b.View(), "following") {
		t.Error("s must turn autoscroll off")
	}
	b, _ = fleetDrive(t, b, "f")
	v := b.View()
	if strings.Contains(v, "b9s fleet") || strings.Contains(v, "F1-8") || !strings.Contains(v, "hello") {
		t.Errorf("f must hide the header and hints and keep the log:\n%s", v)
	}
	if got := len(strings.Split(v, "\n")); got != b.height {
		t.Errorf("fullscreen draws %d rows, want %d", got, b.height)
	}
	b, _ = fleetDrive(t, b, "f")
	if !strings.Contains(b.View(), "b9s fleet") {
		t.Error("a second f must restore the header")
	}
}

func TestFleetLogsCopyAndSaveTheLog(t *testing.T) {
	b := logsWith(t,
		agents.Event{Time: fleetNow, Kind: agents.EventUser, Text: "first line"},
		agents.Event{Time: fleetNow, Kind: agents.EventAI, Text: "second line"})
	var copied string
	b.deps.Clipboard = func(s string) error { copied = s; return nil }
	b, _ = fleetDrive(t, b, "c")
	if !strings.Contains(copied, "first line") || !strings.Contains(copied, "second line") {
		t.Errorf("c copied %q", copied)
	}
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	b, _ = fleetDrive(t, b, "ctrl+s")
	files, _ := filepath.Glob(filepath.Join(dir, "b9s-logs", "*.log"))
	if len(files) != 1 {
		t.Fatalf("ctrl+s wrote %v, status %q", files, b.status)
	}
	data, _ := os.ReadFile(files[0])
	if !strings.Contains(string(data), "second line") || !strings.Contains(b.status, files[0]) {
		t.Errorf("saved %q, status %q", data, b.status)
	}
}

func TestFleetLogsShiftKeysOpenTaskAndContext(t *testing.T) {
	b := logsWith(t, agents.Event{Time: fleetNow, Kind: agents.EventAI, Text: "hello"})
	if _, out := fleetDrive(t, b, "T"); out.openIssue != "bd-ab12.3" {
		t.Errorf("T outcome = %+v", out)
	}
	if b, _ = fleetDrive(t, b, "C"); b.view() != fleetContext {
		t.Errorf("C opened %v", b.view())
	}
}

// The fleet draws under the header every issue view shares (ADR 0069), and the
// two together fill the screen exactly.
func TestFleetDrawsUnderTheSharedHeader(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := keybindingTestModel(t, nil)
	m.allProjects = []config.Project{{Name: "one", Path: t.TempDir()}, {Name: "two", Path: t.TempDir()}}
	issues := strings.Split(m.View(), "\n")
	m, _ = m.executeCommand(fleetCommand(fleetAgents))
	fleet := strings.Split(m.View(), "\n")
	if !strings.Contains(strings.Join(fleet, "\n"), "b9s fleet") {
		t.Fatalf("the fleet view is missing:\n%s", strings.Join(fleet, "\n"))
	}
	if len(fleet) != m.height {
		t.Fatalf("fleet view has %d lines, want the screen height %d", len(fleet), m.height)
	}
	if _, h := m.fleetSize(*m.fleet); lipgloss.Height(m.renderPickerHeader())+h != m.height {
		t.Fatalf("header %d rows + fleet %d rows, want %d", lipgloss.Height(m.renderPickerHeader()), h, m.height)
	}
	if fleet[0] != issues[0] || !strings.Contains(strings.Join(fleet, "\n"), "Types") {
		t.Fatalf("the fleet lost the shared header:\nissues: %q\nfleet:  %q", issues[0], fleet[0])
	}
}
