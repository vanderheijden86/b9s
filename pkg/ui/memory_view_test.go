package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/vanderheijden86/b9s/internal/datasource"
)

const memScope = "http://example.test/poc/"

func memID(path string) string { return memScope + "beads/" + path }

// fakeMemorySource serves a fixed graph and records each read, so a test can
// prove the browser reads one Memory's detail only when it is selected.
type fakeMemorySource struct {
	summaries []datasource.MemorySummary
	beads     map[string]datasource.GraphBead // key: id + "@" + version
	links     map[string][]datasource.GraphLink
	versions  map[string][]datasource.GraphVersion
	inventory datasource.GraphPreview
	memoryErr error
	graph     datasource.MemoryGraph
	graphErr  error
	calls     []string
}

func (f *fakeMemorySource) Graph(context.Context) (datasource.MemoryGraph, error) {
	f.calls = append(f.calls, "graph")
	return f.graph, f.graphErr
}

func (f *fakeMemorySource) Inventory(context.Context) (datasource.GraphPreview, error) {
	f.calls = append(f.calls, "inventory")
	return f.inventory, nil
}

func (f *fakeMemorySource) SearchMemories(_ context.Context, query string) ([]datasource.MemorySummary, error) {
	f.calls = append(f.calls, "search:"+query)
	if query == "" {
		return f.summaries, nil
	}
	var out []datasource.MemorySummary
	for _, s := range f.summaries {
		if strings.Contains(s.Title, query) {
			s.MatchedFields = []string{"title"}
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeMemorySource) Memory(_ context.Context, id, version string) (datasource.GraphBead, error) {
	f.calls = append(f.calls, "memory:"+graphPath(id)+"@"+version)
	if f.memoryErr != nil {
		return datasource.GraphBead{}, f.memoryErr
	}
	bead, ok := f.beads[id+"@"+version]
	if !ok {
		return datasource.GraphBead{}, errors.New("not_found: graph resource not found")
	}
	return bead, nil
}

func (f *fakeMemorySource) Links(_ context.Context, id string) ([]datasource.GraphLink, error) {
	f.calls = append(f.calls, "links:"+graphPath(id))
	return f.links[id], nil
}

func (f *fakeMemorySource) Versions(_ context.Context, id string) ([]datasource.GraphVersion, error) {
	f.calls = append(f.calls, "versions:"+graphPath(id))
	return f.versions[id], nil
}

func (f *fakeMemorySource) callCount(prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func newFakeMemorySource() *fakeMemorySource {
	cites := datasource.GraphLink{ID: memScope + "links/seed-1", Type: memScope + "types/example-cites", Source: memID("adr-0030"), Target: memID("adr-0025"), Properties: datasource.GraphProperties{Note: "uses embedded-store constraints"}}
	follows := datasource.GraphLink{ID: memScope + "links/seed-9", Type: memScope + "types/example-follows", Source: memID("sample-db5q.2"), Target: memID("adr-0030"), Properties: datasource.GraphProperties{Note: "viewer follows ADR"}}
	adr30 := datasource.GraphBead{ID: memID("adr-0030"), Version: "v3", Properties: datasource.GraphProperties{Title: "ADR 0030 (active): Read through CLI", Body: "Current decision body"}, Owned: []datasource.GraphLink{cites}}
	adr30old := datasource.GraphBead{ID: memID("adr-0030"), Version: "v1", Properties: datasource.GraphProperties{Title: "ADR 0030 (active): Read through CLI", Body: "Original decision body"}}
	adr25 := datasource.GraphBead{ID: memID("adr-0025"), Version: "w1", Properties: datasource.GraphProperties{Title: "ADR 0025 (active): Read embedded Dolt", Body: "Embedded body"}}
	return &fakeMemorySource{
		summaries: []datasource.MemorySummary{
			{ID: memID("adr-0025"), Title: adr25.Properties.Title, Version: "w1"},
			{ID: memID("adr-0030"), Title: adr30.Properties.Title, Version: "v3"},
		},
		beads: map[string]datasource.GraphBead{
			memID("adr-0030") + "@":   adr30,
			memID("adr-0030") + "@v1": adr30old,
			memID("adr-0025") + "@":   adr25,
		},
		links: map[string][]datasource.GraphLink{
			memID("adr-0030"): {cites, follows},
			memID("adr-0025"): {cites},
		},
		versions: map[string][]datasource.GraphVersion{
			memID("adr-0030"): {
				{Version: "v3", Ordinal: 3, ChangeAt: time.Date(2026, 10, 2, 13, 5, 22, 0, time.UTC), Actor: "dev"},
				{Version: "v2", Ordinal: 2, ChangeAt: time.Date(2026, 10, 2, 13, 5, 21, 0, time.UTC), Actor: "dev"},
				{Version: "v1", Ordinal: 1, ChangeAt: time.Date(2026, 10, 2, 13, 4, 5, 0, time.UTC), Actor: "dev"},
			},
		},
		inventory: datasource.GraphPreview{Beads: []datasource.GraphBead{
			{ID: memID("adr-0025"), Kind: "memory", Properties: adr25.Properties},
			{ID: memID("adr-0030"), Kind: "memory", Properties: adr30.Properties},
			{ID: memID("sample-db5q.2"), Kind: "issue", Properties: datasource.GraphProperties{Title: "Snapshot: display Memory Beads", Status: "closed"}},
		}},
	}
}

// drive runs cmd and feeds every message it yields back into the browser,
// following batches, until no command remains.
func drive(t *testing.T, b MemoryBrowser, cmd tea.Cmd) MemoryBrowser {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 100 {
			t.Fatal("browser kept issuing commands")
		}
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if batch, ok := msg.(tea.BatchMsg); ok {
			queue = append(queue, batch...)
			continue
		}
		if msg == nil {
			continue
		}
		var out tea.Cmd
		b, out = b.Update(msg)
		queue = append(queue, out)
	}
	return b
}

func press(t *testing.T, b MemoryBrowser, keys ...string) MemoryBrowser {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		var cmd tea.Cmd
		b, cmd = b.Update(msg)
		b = drive(t, b, cmd)
	}
	return b
}

func startBrowser(t *testing.T, source *fakeMemorySource) MemoryBrowser {
	t.Helper()
	b := NewMemoryBrowser(source, "b9s-memory-poc")
	b, _ = b.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	return drive(t, b, b.Init())
}

func screen(b MemoryBrowser) string { return ansi.Strip(b.View()) }

func TestMemoryBrowserListsMemoriesAndReadsOnlyTheSelectedDetail(t *testing.T) {
	source := newFakeMemorySource()
	b := startBrowser(t, source)
	view := screen(b)
	for _, want := range []string{"2 Memories", "adr-0025", "adr-0030", "Embedded body"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
	if got := source.callCount("memory:"); got != 1 {
		t.Fatalf("memory reads = %d, want only the selected one; calls %v", got, source.calls)
	}
	if got := source.callCount("links:"); got != 1 {
		t.Fatalf("links reads = %d, want only the selected one; calls %v", got, source.calls)
	}
}

func TestMemoryBrowserShowsBothLinkDirectionsAsInformational(t *testing.T) {
	b := startBrowser(t, newFakeMemorySource())
	b = press(t, b, "j")
	view := screen(b)
	for _, want := range []string{
		"Current decision body",
		"Informational Links",
		"no effect on scheduling",
		"→ example-cites",
		"adr-0025",
		"uses embedded-store constraints",
		"← example-follows",
		"sample-db5q.2",
		"Issue · closed",
		"Snapshot: display Memory Beads",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
}

func TestMemoryBrowserCachesDetailWhenReturningToAMemory(t *testing.T) {
	source := newFakeMemorySource()
	b := startBrowser(t, source)
	b = press(t, b, "j", "k", "j")
	if got := source.callCount("memory:"); got != 2 {
		t.Fatalf("memory reads = %d, want one per distinct Memory; calls %v", got, source.calls)
	}
}

func TestMemoryBrowserIgnoresDetailForAnEarlierSelection(t *testing.T) {
	source := newFakeMemorySource()
	b := startBrowser(t, source)
	b, cmd := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	late := datasourceDetailFor(t, cmd)
	b, _ = b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	b, _ = b.Update(memoryDetailMsg{key: memoryKey{id: memID("adr-0025")}, detail: memoryDetail{bead: datasource.GraphBead{ID: memID("adr-0025"), Properties: datasource.GraphProperties{Body: "Embedded body"}}}})
	b, _ = b.Update(late)
	if view := screen(b); strings.Contains(view, "Current decision body") || !strings.Contains(view, "Embedded body") {
		t.Fatalf("a late reply replaced the selected Memory:\n%s", view)
	}
}

// datasourceDetailFor runs the detail read a key press started, without
// delivering it, so a test can deliver it late.
func datasourceDetailFor(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("selection started no read")
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if m, ok := c().(memoryDetailMsg); ok {
				return m
			}
		}
	}
	if m, ok := msg.(memoryDetailMsg); ok {
		return m
	}
	t.Fatalf("no detail message in %T", msg)
	return nil
}

func TestMemoryBrowserSearchIsLiteralAndSaysSo(t *testing.T) {
	source := newFakeMemorySource()
	b := startBrowser(t, source)
	b = press(t, b, "/", "0", "0", "3", "0", "enter")
	view := screen(b)
	if source.callCount("search:0030") != 1 {
		t.Fatalf("search not run; calls %v", source.calls)
	}
	for _, want := range []string{`literal search "0030"`, "1 of 2 Memories", "adr-0030", "title"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "adr-0025  ADR 0025") {
		t.Errorf("non-matching Memory still listed:\n%s", view)
	}
	b = press(t, b, "/", "backspace", "backspace", "backspace", "backspace", "enter")
	if view := screen(b); !strings.Contains(view, "2 Memories") || strings.Contains(view, "literal search") {
		t.Fatalf("empty search did not list every Memory:\n%s", view)
	}
}

func TestMemoryBrowserReadsARetainedVersion(t *testing.T) {
	source := newFakeMemorySource()
	b := startBrowser(t, source)
	b = press(t, b, "j", "v")
	view := screen(b)
	for _, want := range []string{"Retained versions", "#3", "current", "#1", "not full history"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in versions view:\n%s", want, view)
		}
	}
	b = press(t, b, "j", "j", "enter")
	view = screen(b)
	for _, want := range []string{"Original decision body", "retained version v1", "not current", "Incoming (current version)"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in retained view:\n%s", want, view)
		}
	}
	if strings.Contains(view, "→ example-cites") {
		t.Errorf("retained version showed a Link it did not own:\n%s", view)
	}
	if source.callCount("memory:adr-0030@v1") != 1 {
		t.Fatalf("retained version not read by token; calls %v", source.calls)
	}
}

func TestMemoryBrowserFollowsALinkToAMemoryAndBack(t *testing.T) {
	b := startBrowser(t, newFakeMemorySource())
	b = press(t, b, "j", "enter", "enter")
	if view := screen(b); !strings.Contains(view, "Embedded body") {
		t.Fatalf("following the outgoing Link did not open adr-0025:\n%s", view)
	}
	b = press(t, b, "esc")
	if view := screen(b); !strings.Contains(view, "Current decision body") {
		t.Fatalf("esc did not return to adr-0030:\n%s", view)
	}
}

func TestMemoryBrowserDoesNotFollowALinkToAnIssue(t *testing.T) {
	b := startBrowser(t, newFakeMemorySource())
	b = press(t, b, "j", "enter", "j", "enter")
	view := screen(b)
	if !strings.Contains(view, "Current decision body") || !strings.Contains(view, "Issue snapshots open in bd") {
		t.Fatalf("an Issue endpoint should stay put with a hint:\n%s", view)
	}
}

func TestMemoryBrowserShowsAPreviewRefusal(t *testing.T) {
	source := newFakeMemorySource()
	source.memoryErr = errors.New("bd show: capability_unavailable: not supported")
	b := startBrowser(t, source)
	if view := screen(b); !strings.Contains(view, "capability_unavailable") {
		t.Fatalf("refusal not shown:\n%s", view)
	}
}

func TestMemoryBrowserQuits(t *testing.T) {
	b := startBrowser(t, newFakeMemorySource())
	_, cmd := b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
}

func TestMemoryBrowserFitsASmallTerminal(t *testing.T) {
	source := newFakeMemorySource()
	adr := source.beads[memID("adr-0030")+"@"]
	adr.Properties.Body = strings.Repeat("A long decision paragraph that wraps in a narrow pane. ", 40)
	source.beads[memID("adr-0030")+"@"] = adr
	for _, size := range []tea.WindowSizeMsg{{Width: 60, Height: 12}, {Width: 140, Height: 40}} {
		b := NewMemoryBrowser(source, "b9s-memory-poc")
		b, _ = b.Update(size)
		b = drive(t, b, b.Init())
		b = press(t, b, "j", "enter")
		lines := strings.Split(screen(b), "\n")
		if len(lines) > size.Height {
			t.Errorf("%dx%d: view has %d lines", size.Width, size.Height, len(lines))
		}
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > size.Width {
				t.Errorf("%dx%d: line is %d cells wide: %q", size.Width, size.Height, w, line)
			}
		}
		if !strings.Contains(screen(b), "Informational Links") {
			t.Errorf("%dx%d: Links block pushed off screen:\n%s", size.Width, size.Height, screen(b))
		}
	}
}
