# Compact Board Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** One-line board cards that expand when selected, and an epic cell that shows the epic's description, counts, labels, owner, due date and last activity.

**Architecture:** The board keeps its lane renderer (`pkg/ui/board_lanes.go`). `boardEpic` grows by the facts the cell shows, all computed once in `rebuildEpicIndex` from the epic universe. `cardLines` draws one line unless the card is selected, and `cardHeight` takes the selection so the lane layout agrees with the drawn card. Design: `docs/plans/2026-10-02-compact-board-design.md`, ADR 0030.

**Tech Stack:** Go 1.25, bubbletea, lipgloss. Tests: `go test ./pkg/ui/`, E2E in `tests/e2e/`.

**Beads:** feature `bd-e5u3.20`. Each task below has a child task; mark it `in_progress` before its first step and close it at its commit.

**Test command for every step:** `go test ./pkg/ui/ -run '<Name>' -v` from the worktree root. Package run: `go test ./pkg/ui/`.

---

### Task 1: Epic facts in `boardEpic`

**Files:**
- Modify: `pkg/ui/board_epics.go` (struct `boardEpic`, `rebuildEpicIndex`)
- Create: `pkg/ui/board_epic_facts.go` (`firstSentence`)
- Test: `pkg/ui/board_epic_facts_test.go`

**Step 1: Write the failing tests**

```go
package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestFirstSentence(t *testing.T) {
	cases := map[string]string{
		"Live activity feed from bd events. Covers the TUI and web.": "Live activity feed from bd events.",
		"# Heading\n\nSync invoices from the billing API":              "Sync invoices from the billing API",
		"- first bullet\nsecond line":                                  "first bullet",
		"Is it done? Not yet.":                                         "Is it done?",
		"No terminator at all":                                         "No terminator at all",
		"Version 2.1 ships soon. Then 3.":                              "Version 2.1 ships soon.",
		"":                                                             "",
		"  \n  ":                                                       "",
	}
	for in, want := range cases {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}

// factsBoardIssues is one epic with every kind of child the cell counts:
// a ready issue, an issue waiting on an open blocker, one in progress, one
// deferred, one closed, and an urgent (P1) one. The epic itself carries the
// description, labels, owner and due date.
func factsBoardIssues() []model.Issue {
	now := time.Now()
	old := now.Add(-72 * time.Hour)
	recent := now.Add(-2 * time.Hour)
	due := now.Add(48 * time.Hour)
	mk := func(id string, status model.Status, prio int, updated time.Time, deps ...*model.Dependency) model.Issue {
		return model.Issue{ID: id, Title: id, IssueType: model.TypeTask, Status: status, Priority: prio,
			Dependencies: deps, CreatedAt: old, UpdatedAt: updated}
	}
	child := func(id string) *model.Dependency {
		return &model.Dependency{IssueID: id, DependsOnID: "p-e1", Type: model.DepParentChild}
	}
	blocks := func(id, on string) *model.Dependency {
		return &model.Dependency{IssueID: id, DependsOnID: on, Type: model.DepBlocks}
	}
	epic := model.Issue{ID: "p-e1", Title: "Epic: Events", IssueType: model.TypeEpic, Status: model.StatusOpen, Priority: 1,
		Description: "Live activity feed from bd events. Covers the TUI and web.",
		Labels:      []string{"tui", "web"}, Assignee: "andre", DueDate: &due, CreatedAt: old, UpdatedAt: old}
	return []model.Issue{
		epic,
		mk("p-e1.1", model.StatusOpen, 2, old, child("p-e1.1")),                       // ready
		mk("p-e1.2", model.StatusOpen, 1, recent, child("p-e1.2"), blocks("p-e1.2", "p-e1.1")), // waiting, urgent
		mk("p-e1.3", model.StatusInProgress, 2, old, child("p-e1.3")),                 // wip
		mk("p-e1.4", model.StatusDeferred, 3, old, child("p-e1.4")),                   // neither ready nor waiting
		mk("p-e1.5", model.StatusClosed, 2, old, child("p-e1.5")),                     // done
		mk("p-e1.6", model.StatusOpen, 2, old, child("p-e1.6"), blocks("p-e1.6", "p-e1.5")), // blocker closed: ready
	}
}

func TestBoardEpicFacts_CountsOverTheEpicUniverse(t *testing.T) {
	b := NewBoardModel(factsBoardIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("p")
	e := b.epics["p-e1"]
	if e == nil {
		t.Fatal("no epic p-e1")
	}
	want := boardEpic{Done: 1, Total: 6, Urgent: 1, InProgress: 1, Waiting: 1, Ready: 2,
		Description: "Live activity feed from bd events.", Labels: []string{"tui", "web"}, Owner: "andre"}
	got := *e
	if got.Done != want.Done || got.Total != want.Total || got.Urgent != want.Urgent ||
		got.InProgress != want.InProgress || got.Waiting != want.Waiting || got.Ready != want.Ready {
		t.Fatalf("counts: got done %d/%d urgent %d wip %d waiting %d ready %d", got.Done, got.Total, got.Urgent, got.InProgress, got.Waiting, got.Ready)
	}
	if got.Description != want.Description || got.Owner != want.Owner || len(got.Labels) != 2 || got.Due == nil {
		t.Fatalf("facts: got %q owner %q labels %v due %v", got.Description, got.Owner, got.Labels, got.Due)
	}
	if time.Since(got.LastActivity) > 3*time.Hour {
		t.Fatalf("last activity must be the newest child's update, got %v", got.LastActivity)
	}
}

func TestBoardEpicFacts_OwnerFallsBackToOwnerField(t *testing.T) {
	issues := factsBoardIssues()
	issues[0].Assignee = ""
	issues[0].Owner = "polly"
	b := NewBoardModel(issues, DefaultTheme(lipgloss.DefaultRenderer()))
	if got := b.epics["p-e1"].Owner; got != "polly" {
		t.Fatalf("owner = %q, want polly", got)
	}
}
```

**Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/ui/ -run 'TestFirstSentence|TestBoardEpicFacts' -v`
Expected: build error, `undefined: firstSentence` and unknown fields on `boardEpic`.

**Step 3: Write the minimal implementation**

`pkg/ui/board_epic_facts.go`:

```go
package ui

import "strings"

// firstSentence returns the first sentence of a description: the text up to
// the first ". ", "! " or "? " (or that mark at the end), or up to the first
// blank line, whichever comes first. A leading Markdown heading or list
// marker is dropped, so "# Goal" never reads as the sentence. A period
// inside a word, as in "2.1", does not end the sentence.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if para, _, ok := strings.Cut(s, "\n\n"); ok {
		s = strings.TrimSpace(para)
	}
	for _, mark := range []string{"#", "-", "*", "+"} {
		if strings.HasPrefix(s, mark) {
			s = strings.TrimLeft(s, mark+" ")
			break
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '.' || c == '!' || c == '?' {
			if i+1 == len(s) || s[i+1] == ' ' {
				return s[:i+1]
			}
		}
	}
	return s
}
```

In `pkg/ui/board_epics.go`, extend the struct:

```go
type boardEpic struct {
	ID    string
	Title string
	Done  int
	Total int
	// The facts the epic cell shows (docs/adr/0030), all over the epic
	// universe, so a board filter never changes them.
	Description  string   // first sentence, sanitised
	Labels       []string // the epic issue's own labels
	Owner        string   // assignee, or owner when there is no assignee
	Due          *time.Time
	Urgent       int // open P0 and P1 issues under the epic
	InProgress   int
	Waiting      int // open, with an open blocker
	Ready        int // open, no open blocker, not deferred
	LastActivity time.Time // newest UpdatedAt of the epic and its issues
	rank         int
	color        lipgloss.AdaptiveColor
}
```

Add `"time"` to the imports. In `rebuildEpicIndex`, where `info` is created from `e := byID[epic]`:

```go
			e := sanitizeIssueForTerminal(*byID[epic])
			info = &boardEpic{ID: epic, Title: withoutOwnIDPrefix(e.Title, epic), rank: 99, color: epicColor(epic),
				Description: firstSentence(e.Description), Labels: e.Labels, Owner: e.Assignee, Due: e.DueDate,
				LastActivity: e.UpdatedAt}
			if info.Owner == "" {
				info.Owner = e.Owner
			}
```

And after the `is.ID == epic` continue, beside the `Total`/`Done` counting:

```go
		if is.UpdatedAt.After(info.LastActivity) {
			info.LastActivity = is.UpdatedAt
		}
		info.Total++
		switch {
		case isClosedLikeStatus(is.Status):
			info.Done++
		default:
			if is.Priority <= 1 {
				info.Urgent++
			}
			switch {
			case is.Status == model.StatusInProgress:
				info.InProgress++
			case b.openBlockerIn(is, byID) != "":
				info.Waiting++
			case is.Status != model.StatusDeferred:
				info.Ready++
			}
		}
```

The `LastActivity` update goes before the `is.ID == epic` continue so the epic's own update counts. `openBlocker` reads `b.issueMap`, which holds only the filtered board, so add a variant that reads the universe map in `pkg/ui/board_layout.go` beside `openBlocker`:

```go
// openBlockerIn is openBlocker over an explicit issue map, for counts taken
// over the epic universe rather than the filtered board.
func (b *BoardModel) openBlockerIn(issue model.Issue, byID map[string]*model.Issue) string {
	for _, dep := range issue.Dependencies {
		if dep == nil || !dep.Type.IsBlocking() {
			continue
		}
		if blocker, ok := byID[dep.DependsOnID]; ok && blocker != nil && isClosedLikeStatus(blocker.Status) {
			continue
		}
		return dep.DependsOnID
	}
	return ""
}

func (b *BoardModel) openBlocker(issue model.Issue) string { return b.openBlockerIn(issue, b.issueMap) }
```

Note: the existing `rebuildEpicIndex` reads `rank` from open issues before the `is.ID == epic` check. Keep that order.

**Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/ui/ -run 'TestFirstSentence|TestBoardEpicFacts|TestBoardEpics' -v`
Expected: PASS, and the existing epic tests still pass.

**Step 5: Commit**

```bash
git add pkg/ui/board_epic_facts.go pkg/ui/board_epic_facts_test.go pkg/ui/board_epics.go pkg/ui/board_layout.go
git commit -m "feat(board): compute the epic facts the compact cell shows

Refs: <task-id>"
```

---

### Task 2: Identities reach the board

**Files:**
- Modify: `pkg/ui/board.go` (field and setter), `pkg/ui/identities.go:76-79`
- Test: `pkg/ui/board_epic_facts_test.go`

**Step 1: Write the failing test**

```go
func TestBoardEpicFacts_OwnerShowsThroughTheIdentityRegistry(t *testing.T) {
	b := NewBoardModel(factsBoardIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("p")
	reg := identity.NewRegistry([]identity.Identity{{Name: "André", Aliases: []string{"andre"}}})
	b.SetIdentities(reg)
	if got := b.ownerLabel("andre"); got != "@André" {
		t.Fatalf("ownerLabel = %q, want @André", got)
	}
	if got := b.ownerLabel(""); got != "" {
		t.Fatalf("ownerLabel of nobody = %q, want empty", got)
	}
}
```

Check the registry constructor's real name and the `Identity` fields first: `grep -n "^func New\|^type Identity struct" -A6 pkg/identity/identity.go`. Adjust the test to the real constructor.

**Step 2: Run to verify it fails**

Run: `go test ./pkg/ui/ -run TestBoardEpicFacts_OwnerShows -v`
Expected: `undefined: (BoardModel).SetIdentities`.

**Step 3: Implement**

In `pkg/ui/board.go`, in the struct after `activeProjectName`:

```go
	// identities resolves an epic's owner to its display name (ADR 0014).
	identities *identity.Registry
```

Add the import `"github.com/vanderheijden86/b9s/pkg/identity"`, and:

```go
// SetIdentities gives the board the alias registry, so an epic's owner shows
// as the configured name.
func (b *BoardModel) SetIdentities(reg *identity.Registry) { b.identities = reg }

// ownerLabel is "@name" for an owner, or "" for nobody. A nil registry
// resolves every name to itself.
func (b *BoardModel) ownerLabel(owner string) string {
	if owner == "" {
		return ""
	}
	return "@" + b.identities.DisplayName(owner)
}
```

`Registry.DisplayName` on a nil receiver returns the alias itself (see `Resolve`). In `pkg/ui/identities.go` `setIdentities`, add `m.board.SetIdentities(reg)`.

**Step 4: Run to verify it passes**

Run: `go test ./pkg/ui/ -run 'TestBoardEpicFacts' -v` → PASS.

**Step 5: Commit**

```bash
git add pkg/ui/board.go pkg/ui/identities.go pkg/ui/board_epic_facts_test.go
git commit -m "feat(board): resolve an epic's owner through the identity registry

Refs: <task-id>"
```

---

### Task 3: The epic cell's five lines

**Files:**
- Modify: `pkg/ui/board_lanes.go` (`railWidth`, `railLines`, constants)
- Test: `pkg/ui/board_epic_facts_test.go`

**Step 1: Write the failing tests**

```go
func newFactsBoard(issues []model.Issue) BoardModel {
	b := NewBoardModel(issues, DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("p")
	return b
}

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.TrimRight(stripANSI(l), " ")
	}
	return out
}

func TestBoardEpicCell_FiveLinesWithEveryFact(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	got := plainLines(b.railLines("p-e1", 34, false, false))
	want := []string{
		"▾ ◆ e1 Epic: Events",
		"Live activity feed from bd",
		"events.",
		"━━━━━━━━━━━━──────── 1/6 · P1 1",
		"wip 1 · waiting 1 · ready 2",
		"#tui #web · @andre · due",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for i := range want {
		if i == 3 {
			if !strings.HasSuffix(got[i], " 1/6 · P1 1") || !strings.HasPrefix(got[i], "━") {
				t.Errorf("line %d = %q, want a bar ending in 1/6 · P1 1", i, got[i])
			}
			continue
		}
		if i == 5 {
			if !strings.HasPrefix(got[i], "#tui #web · @andre · due ") || !strings.HasSuffix(got[i], " · 2h") {
				t.Errorf("line %d = %q, want labels, owner, due and activity", i, got[i])
			}
			continue
		}
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBoardEpicCell_LeavesOutEmptyLines(t *testing.T) {
	issues := factsBoardIssues()
	issues[0].Description, issues[0].Labels, issues[0].Assignee, issues[0].DueDate = "", nil, "", nil
	b := newFactsBoard(issues)
	got := plainLines(b.railLines("p-e1", 34, false, false))
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3 (title, bar, counts):\n%s", len(got), strings.Join(got, "\n"))
	}
	if !strings.HasSuffix(got[2], "ready 2 · 2h") {
		t.Fatalf("without a fifth line the activity ends the counts line, got %q", got[2])
	}
}

func TestBoardEpicCell_NoUrgentNoP1(t *testing.T) {
	issues := factsBoardIssues()
	issues[2].Priority = 2
	b := newFactsBoard(issues)
	got := plainLines(b.railLines("p-e1", 34, false, false))
	if strings.Contains(got[3], "P1") {
		t.Fatalf("no P0/P1 open: the bar line must not say P1, got %q", got[3])
	}
}

func TestBoardEpicCell_FoldedShowsTitleAndBar(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	got := plainLines(b.railLines("p-e1", 34, true, false))
	if len(got) != 2 || !strings.HasPrefix(got[0], "▸ ◆ e1 Epic: Events") || !strings.Contains(got[1], "1/6") {
		t.Fatalf("folded cell = %q, want the folded title line and the bar", got)
	}
}

func TestBoardEpicCell_PastDueIsRed(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	issues := factsBoardIssues()
	past := time.Now().Add(-24 * time.Hour)
	issues[0].DueDate = &past
	b := NewBoardModel(issues, DefaultTheme(renderer))
	b.SetActiveProjectName("p")
	lines := b.railLines("p-e1", 34, false, false)
	red := renderer.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}).Render("due")
	if !strings.Contains(lines[5], red[:strings.Index(red, "due")]+"due") {
		t.Fatalf("a past due date must be red: %q", lines[5])
	}
}

func TestBoardRailWidth_IsAFifthBetween16And34(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	for _, c := range []struct{ width, want int }{{60, 16}, {100, 20}, {170, 34}, {240, 34}} {
		if got := b.railWidth(c.width); got != c.want {
			t.Errorf("railWidth(%d) = %d, want %d", c.width, got, c.want)
		}
	}
}
```

**Step 2: Run to verify they fail**

Run: `go test ./pkg/ui/ -run 'TestBoardEpicCell|TestBoardRailWidth' -v`
Expected: FAIL on line counts and the rail width.

**Step 3: Implement**

In `pkg/ui/board_lanes.go`:

```go
const (
	laneRailMin        = 16
	laneRailMax        = 34
	laneRailTitleMax   = 2
	laneRailSummaryMax = 2
	cardTitleMaxLines  = 2
)

func (b *BoardModel) railWidth(width int) int {
	if b.epicView != BoardEpicRail {
		return 0
	}
	return min(max(width/5, laneRailMin), laneRailMax)
}
```

Replace `railLines`:

```go
// railLines returns the content of one lane's epic cell, which railBox
// frames (docs/adr/0030):
//
//	▾ ◆ eg0 Stream capture
//	pipeline
//	Capture bd events as a stream.
//	━━━━──────── 1/4 · P1 2
//	wip 1 · waiting 2 · ready 1
//	#tui · @andre · due 14 Oct · 4d
//
// The description and the last line are left out when empty, and the
// activity then ends the counts line. A folded cell is the title and the bar.
func (b *BoardModel) railLines(epic string, width int, folded, selected bool) []string {
	t := b.theme
	muted := b.fg(selected, t.Secondary)
	inner := railInner(width)
	if epic == "" {
		lines := []string{muted.Bold(true).Render(truncateRunesHelper("◇ No epic", inner, "…"))}
		if !folded {
			lines = append(lines, muted.Render(truncateRunesHelper(issueCount(b.laneIssueCount(epic)), inner, "…")))
		}
		return lines
	}
	info := b.epics[epic]
	marker := "▾ "
	if folded {
		marker = "▸ "
	}
	head := marker + "◆ " + b.displayID(epic)
	maxTitle := laneRailTitleMax
	if folded {
		maxTitle = 1
	}
	var lines []string
	for i, l := range clampLines(wrapTitleLines(head+" "+info.Title, inner), maxTitle, inner) {
		if i == 0 && strings.HasPrefix(l, head) {
			l = b.fg(selected, info.color).Bold(true).Render(head) + b.fg(selected, t.Base.GetForeground()).Bold(true).Render(l[len(head):])
		} else {
			l = b.fg(selected, t.Base.GetForeground()).Bold(true).Render(l)
		}
		lines = append(lines, l)
	}
	if !folded && info.Description != "" {
		for _, l := range clampLines(wrapTitleLines(info.Description, inner), laneRailSummaryMax, inner) {
			lines = append(lines, muted.Render(l))
		}
	}
	lines = append(lines, b.epicBarLine(info, inner, selected))
	if folded {
		return lines
	}
	counts := fmt.Sprintf("wip %d · waiting %d · ready %d", info.InProgress, info.Waiting, info.Ready)
	meta := b.epicMetaLine(info, selected)
	activity := b.fg(selected, getAgeColor(info.LastActivity)).Render(compactActivity(info.LastActivity))
	if meta == "" {
		lines = append(lines, muted.Render(truncateRunesHelper(counts+" · ", inner-lipgloss.Width(activity)+3, "…"))+activity)
		return lines
	}
	lines = append(lines, muted.Render(truncateRunesHelper(counts, inner, "…")))
	metaW := inner - lipgloss.Width(" · ") - lipgloss.Width(compactActivity(info.LastActivity))
	lines = append(lines, b.fitStyled(meta, metaW)+muted.Render(" · ")+activity)
	return lines
}

// epicBarLine is the progress bar, done/total and the urgent count.
func (b *BoardModel) epicBarLine(info *boardEpic, inner int, selected bool) string {
	muted := b.fg(selected, b.theme.Secondary)
	tail := fmt.Sprintf(" %d/%d", info.Done, info.Total)
	if info.Urgent > 0 {
		tail += fmt.Sprintf(" · P1 %d", info.Urgent)
	}
	if barW := inner - lipgloss.Width(tail); barW >= 3 {
		return b.fg(selected, info.color).Render(progressBar(info.Done, info.Total, barW)) + muted.Render(tail)
	}
	return muted.Render(strings.TrimSpace(tail))
}

// epicMetaLine joins the labels, the owner and the due date, or returns ""
// when the epic has none of them.
func (b *BoardModel) epicMetaLine(info *boardEpic, selected bool) string {
	t := b.theme
	var parts []string
	if len(info.Labels) > 0 {
		labels := make([]string, len(info.Labels))
		for i, l := range info.Labels {
			labels[i] = "#" + l
		}
		parts = append(parts, b.fg(selected, t.Secondary).Render(strings.Join(labels, " ")))
	}
	if owner := b.ownerLabel(info.Owner); owner != "" {
		parts = append(parts, b.fg(selected, t.Secondary).Render(owner))
	}
	if info.Due != nil {
		c := lipgloss.TerminalColor(t.Secondary)
		if info.Due.Before(time.Now()) {
			c = lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}
		}
		parts = append(parts, b.fg(selected, c).Render("due "+info.Due.Format("2 Jan")))
	}
	return strings.Join(parts, b.fg(selected, t.Secondary).Render(" · "))
}

// fitStyled clips a styled line to width cells with an ellipsis.
func (b *BoardModel) fitStyled(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return b.theme.Renderer.NewStyle().MaxWidth(max(width-1, 0)).Render(s) + "…"
}

// compactActivity is the time since t in at most four cells.
func compactActivity(t time.Time) string {
	return compactAge(model.Issue{UpdatedAt: t})
}
```

Add `"time"` to the imports of `board_lanes.go`. The clipping of the fifth line keeps the activity visible and cuts the labels first, which is the cheapest fact to lose.

Note the title now wraps together with the `▾ ◆ id` head, so a short title shares line one with the ID, as the design shows. The first-line split styles the head in the epic colour and the rest in the base colour.

**Step 4: Run to verify they pass**

Run: `go test ./pkg/ui/ -run 'TestBoardEpicCell|TestBoardRailWidth|TestBoardEpic' -v`
Expected: PASS for the new tests. `TestBoardEpicRail_EpicSitsInALeftRail` expects `"issues"` and `"▾ ◆ eg0"`; both still render (the No epic lane says `1 issue`, the head is unchanged). If `TestBoardEpicRail_EpicCellIsABoxSpanningItsLane` fails on `rule-head < 6`, that is Task 5's concern (cards become one line); leave it until then.

**Step 5: Commit**

```bash
git add pkg/ui/board_lanes.go pkg/ui/board_epic_facts_test.go
git commit -m "feat(board): show description, counts, labels, owner, due and activity in the epic cell

Refs: <task-id>"
```

---

### Task 4: One-line cards, the selected card expands

**Files:**
- Modify: `pkg/ui/board_lanes.go` (`cardHeight`, `cardLines`, `lanesBody` call sites), `pkg/ui/board_render.go` (`renderColumn`)
- Modify tests: `pkg/ui/board_epics_test.go:143-151` (`TestBoardEpicRail_CardsAreBoxes`), `:496-516` (`TestBoardCardHeight_MatchesDrawnCard`), `pkg/ui/board_layout_test.go:232-256`
- Test: `pkg/ui/board_card_line_test.go`

**Step 1: Write the failing tests**

```go
package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

func cardIssue() model.Issue {
	return model.Issue{ID: "spectroscope-eg0.4.2", Title: "Wire the flow subscription transport", IssueType: model.TypeTask,
		Status: model.StatusOpen, Priority: 1, Labels: []string{"lane:reviewing"},
		Dependencies: []*model.Dependency{{IssueID: "spectroscope-eg0.4.2", DependsOnID: "spectroscope-eg0.4.1", Type: model.DepBlocks}}}
}

func TestBoardCard_UnselectedIsOneLine(t *testing.T) {
	b := newSparseBoard()
	lines := b.cardLines(cardIssue(), 60, false, 0, 0)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	got := strings.TrimRight(stripANSI(lines[0]), " ")
	if !strings.HasPrefix(got, "✔ eg0.4.2 Wire the flow subscription transport") {
		t.Fatalf("line must start with icon, ID and title, got %q", got)
	}
	if !strings.HasSuffix(got, "blocked eg0.4.1 · lane: reviewing · blocks 1 · P1") {
		t.Fatalf("tags must be right-aligned in order, got %q", got)
	}
	if lipgloss.Width(lines[0]) != 60 {
		t.Fatalf("line width %d, want 60", lipgloss.Width(lines[0]))
	}
}

func TestBoardCard_TagsDropFromTheRightBeforeTheTitle(t *testing.T) {
	b := newSparseBoard()
	got := strings.TrimRight(stripANSI(b.cardLines(cardIssue(), 36, false, 0, 0)[0]), " ")
	if strings.Contains(got, "P1") || strings.Contains(got, "blocks") {
		t.Fatalf("at 36 cells the later tags must go first, got %q", got)
	}
	if !strings.Contains(got, "blocked eg0.4.1") || !strings.Contains(got, "Wire the") {
		t.Fatalf("the blocker and part of the title must stay, got %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("a cut title ends in an ellipsis, got %q", got)
	}
}

func TestBoardCard_LowPriorityHasNoPriorityTag(t *testing.T) {
	b := newSparseBoard()
	is := cardIssue()
	is.Priority, is.Labels, is.Dependencies = 3, nil, nil
	got := strings.TrimRight(stripANSI(b.cardLines(is, 60, false, 0, 0)[0]), " ")
	if strings.Contains(got, "P3") || strings.Contains(got, "·") {
		t.Fatalf("a P3 card with no tags is icon, ID and title only, got %q", got)
	}
}

func TestBoardCard_SelectedIsTheBox(t *testing.T) {
	b := newSparseBoard()
	lines := plainLines(b.cardLines(cardIssue(), 40, true, 0, 0))
	if len(lines) < 5 || !strings.HasPrefix(lines[0], "╭") || !strings.HasPrefix(lines[len(lines)-1], "╰") {
		t.Fatalf("selected card must be the rounded box:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(strings.Join(lines, "\n"), "blocked by eg0.4.1") {
		t.Fatalf("the box keeps the long tag line:\n%s", strings.Join(lines, "\n"))
	}
}

func TestBoardCardHeight_FollowsTheSelection(t *testing.T) {
	b := newSparseBoard()
	for w := 8; w <= 80; w += 9 {
		if got := b.cardHeight(cardIssue(), w, false); got != 1 {
			t.Errorf("unselected at %d: height %d, want 1", w, got)
		}
		if got, want := b.cardHeight(cardIssue(), w, true), len(b.cardLines(cardIssue(), w, true, 0, 0)); got != want {
			t.Errorf("selected at %d: height %d, drawn %d", w, got, want)
		}
	}
}

// The selected card's box grows its lane; the other columns of that lane
// stay level with it, and the lane below starts after the box.
func TestBoardLanes_SelectedCardGrowsItsLane(t *testing.T) {
	b := newEpicBoard(BoardEpicRail)
	b.SelectIssueByID("spectroscope-x1")
	short := strings.Split(stripANSI(b.View(200, 50)), "\n")
	b.SelectIssueByID("spectroscope-eg0.1")
	long := strings.Split(stripANSI(b.View(200, 50)), "\n")
	at := func(lines []string, s string) int {
		for i, l := range lines {
			if strings.Contains(l, s) {
				return i
			}
		}
		return -1
	}
	if at(long, "◆ k3s")-at(long, "◆ eg0") <= at(short, "◆ k3s")-at(short, "◆ eg0") {
		t.Fatalf("selecting eg0.1 must make the eg0 lane taller:\nshort:\n%s\nlong:\n%s", strings.Join(short, "\n"), strings.Join(long, "\n"))
	}
}
```

Move `plainLines` from Task 3's test file into a shared spot if the compiler complains about a duplicate; it lives in `board_epic_facts_test.go` and the package is one, so no duplicate is needed.

**Step 2: Run to verify they fail**

Run: `go test ./pkg/ui/ -run 'TestBoardCard|TestBoardLanes_Selected' -v`
Expected: build error on `cardHeight` arity, then failing line counts.

**Step 3: Implement**

In `pkg/ui/board_lanes.go`, replace `cardHeight` and `cardLines`:

```go
// cardHeight is the number of lines cardLines draws for issue, found
// without styling anything, so the lanes can be laid out before any card is
// drawn. An unselected card is one line; the selected card is its box.
func (b *BoardModel) cardHeight(issue model.Issue, width int, selected bool) int {
	if !selected {
		return 1
	}
	card := b.cardView(issue)
	inner := max(width-4, 1)
	h := 3 + min(len(wrapTitleLines(card.Title, inner)), cardTitleMaxLines)
	if len(b.cardTags(card, inner)) > 0 {
		h++
	}
	return h
}

// cardLines draws one issue. Unselected, it is one line (docs/adr/0030):
//
//	✔ eg0.4.2 Wire the flow subscription transport   blocked eg0.4.1 · P1
//
// Selected, it is the rounded box with the tag line and the footer:
//
//	╭──────────────────────────────╮
//	│ Wire the flow subscription   │
//	│ transport                    │
//	│ blocked by eg0.4.1           │
//	│ ✔ eg0.4.2          P1 · 2d   │
//	╰──────────────────────────────╯
func (b *BoardModel) cardLines(issue model.Issue, width int, selected bool, col, row int) []string {
	if !selected {
		return []string{b.cardLine(issue, width, col, row)}
	}
	return b.cardBox(issue, width, col, row)
}

// lineTag is one right-aligned tag of a one-line card.
type lineTag struct {
	text  string
	color lipgloss.TerminalColor
	bold  bool
}

// lineTags lists a card's tags for the one-line shape, in priority order:
// the blocker, the dispatcher lane, the downstream count, and P0 or P1.
func (b *BoardModel) lineTags(issue model.Issue, card boardCardView) []lineTag {
	t := b.theme
	var tags []lineTag
	if card.BlockedBy != "" {
		tags = append(tags, lineTag{"blocked " + card.BlockedBy, t.Blocked, false})
	}
	if card.LaneStage != "" {
		tags = append(tags, lineTag{"lane: " + card.LaneStage, t.InProgress, false})
	}
	if card.BlocksCount > 0 {
		tags = append(tags, lineTag{fmt.Sprintf("blocks %d", card.BlocksCount), t.Feature, false})
	}
	if issue.Priority <= 1 {
		tags = append(tags, lineTag{card.Priority, lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"}, true})
	}
	return tags
}

const cardLineTitleMin = 8

// cardLine draws the one-line card: icon, ID, the title cut to what the tags
// leave, and the tags right-aligned. Tags drop from the right until the
// title keeps cardLineTitleMin cells; the ID is never dropped.
func (b *BoardModel) cardLine(issue model.Issue, width, col, row int) string {
	t := b.theme
	card := b.cardView(issue)
	idColor := lipgloss.TerminalColor(t.Primary)
	if b.IsSearchMatch(col, row) {
		idColor = lipgloss.AdaptiveColor{Light: "#1565c0", Dark: "#64b5f6"}
	}
	icon, iconColor := t.GetTypeIcon(string(issue.IssueType))
	head := b.fg(false, iconColor).Render(icon) + " " + b.fg(false, idColor).Bold(true).Render(card.ShortID) + " "
	headW := lipgloss.Width(icon) + 1 + lipgloss.Width(card.ShortID) + 1

	tags := b.lineTags(issue, card)
	tagsW := func() int {
		w := 0
		for i, tag := range tags {
			if i > 0 {
				w += 3
			}
			w += lipgloss.Width(tag.text)
		}
		return w
	}
	for len(tags) > 0 && width-headW-tagsW()-2 < cardLineTitleMin {
		tags = tags[:len(tags)-1]
	}
	titleW := width - headW
	if len(tags) > 0 {
		titleW -= tagsW() + 2
	}
	title := b.fg(false, t.Base.GetForeground()).Render(truncateRunesHelper(card.Title, max(titleW, 0), "…"))
	line := head + title
	if len(tags) > 0 {
		parts := make([]string, len(tags))
		for i, tag := range tags {
			parts[i] = b.fg(false, tag.color).Bold(tag.bold).Render(tag.text)
		}
		right := strings.Join(parts, b.fg(false, t.Secondary).Render(" · "))
		if gap := width - lipgloss.Width(line) - tagsW(); gap >= 1 {
			line += strings.Repeat(" ", gap) + right
		}
	}
	return surface(line, width, b.rowSurface(false, col, row))
}
```

Rename the old `cardLines` body to `cardBox(issue, width, col, row)` with `selected := true` at the top (keep `bg := b.rowSurface(true, col, row)` and the rest unchanged).

Update the three call sites of `cardHeight`:
- `board_lanes.go` in `lanesBody`: `for off := range b.cardHeight(is, r.width, selected)`.
- Any other: `grep -n "cardHeight(" pkg/ui/*.go`.

`renderColumn` in `board_render.go` calls `cardLines` with `focusedCol && row == sel`; it needs no change.

Update the existing tests:
- `TestBoardEpicRail_CardsAreBoxes` → rename to `TestBoardEpicRail_CardsAreOneLine`: assert the view contains `"✔ eg0.1 Parse capture headers"` on one line and that `"╭"` appears only as often as there are epic cells plus the selected card. Simplest: count lines starting with `╭` after the rail is stripped; with two epics and one selected card the count is 3.
- `TestBoardCardHeight_MatchesDrawnCard`: pass `sel` to `cardHeight`.
- `TestBoardAdaptiveView_ShowsBoxedCardsAndRails` → `TestBoardAdaptiveView_ShowsCardsAndRails`: the selected card `eg0.4.2` is a box, so `"blocked by eg0.4.1"` and `"╭"` still appear. `"lane: reviewing"` and `"blocks 1"` belong to `eg0.4.1`, which is unselected: they are now right-aligned tags on one line, same text, so the assertions hold. Run it and fix only what fails.
- `TestBoardEpicRail_EpicCellIsABoxSpanningItsLane` asserts `rule-head >= 6` because two stacked cards made the lane tall. Now the cell's own content makes it tall. Change the comment and the bound to `rule-head < 4`.

**Step 4: Run to verify they pass**

Run: `go test ./pkg/ui/`
Expected: PASS. Then run `go test ./pkg/ui/ -run 'FitEveryWidth' -v` to confirm no width overflows.

**Step 5: Commit**

```bash
git add pkg/ui/board_lanes.go pkg/ui/board_render.go pkg/ui/board_card_line_test.go pkg/ui/board_epics_test.go pkg/ui/board_layout_test.go
git commit -m "feat(board): one-line cards, the selected card expands to its box

Refs: <task-id>"
```

---

### Task 5: Epic rows header on two lines

**Files:**
- Modify: `pkg/ui/board_lanes.go` (`laneBox`, `laneRow`, the `top`/`lane.head` logic in `lanesBody`, the cut-lane header line `lanes[l.lane].head[1]`)
- Test: `pkg/ui/board_epic_facts_test.go`

**Step 1: Write the failing test**

```go
func TestBoardEpicRows_HeaderHasTheFactsOnTwoLines(t *testing.T) {
	b := newFactsBoard(factsBoardIssues())
	b.SetEpicView(BoardEpicRows)
	lines := plainLines(b.laneBox("p-e1", 120, false, false))
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want top, two content lines, bottom:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, want := range []string{"▾ ◆ e1 Epic: Events", "wip 1 · waiting 1 · ready 2", "#tui #web", "@andre", "1/6 · P1 1"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("line one misses %q: %q", want, lines[1])
		}
	}
	if !strings.Contains(lines[2], "Live activity feed from bd events.") {
		t.Errorf("line two is the description, got %q", lines[2])
	}
	issues := factsBoardIssues()
	issues[0].Description = ""
	b = newFactsBoard(issues)
	b.SetEpicView(BoardEpicRows)
	if got := b.laneBox("p-e1", 120, false, false); len(got) != 3 {
		t.Fatalf("without a description the header is one content line, got %d lines", len(got))
	}
	if got := b.laneBox("p-e1", 120, true, false); len(got) != 3 {
		t.Fatalf("a folded row is one content line, got %d lines", len(got))
	}
}
```

**Step 2: Run to verify it fails**

Run: `go test ./pkg/ui/ -run TestBoardEpicRows_HeaderHasTheFacts -v` → FAIL (3 lines, no counts).

**Step 3: Implement**

Replace `laneBox` and extend `laneRow`:

```go
// laneBox draws the full-width epic header of the rows design as a box of
// one or two content lines: the facts, then the description when there is
// one. A folded row is the facts line alone.
func (b *BoardModel) laneBox(epic string, width int, folded, selected bool) []string {
	side, edge, bg := b.epicFrame(epic, selected)
	inner := max(width-4, 1)
	content := []string{b.laneRow(epic, inner, folded, selected)}
	if !folded && epic != "" && b.epics[epic].Description != "" {
		content = append(content, b.fg(selected, b.theme.Secondary).Render(truncateRunesHelper(b.epics[epic].Description, inner, "…")))
	}
	return frameLines(content, width, side, edge, bg)
}
```

In `laneRow`, after `head`, build `count` from the facts instead of the lane issue count, and keep the bar on the right:

```go
	info := b.epics[epic]
	marker := "▾ "
	if folded {
		marker = "▸ "
	}
	head := marker + "◆ " + b.displayID(epic)
	facts := fmt.Sprintf("  wip %d · waiting %d · ready %d", info.InProgress, info.Waiting, info.Ready)
	if meta := b.epicMetaLine(info, selected); meta != "" {
		facts += " · " + stripANSI(meta)
	}
	facts += " · " + compactActivity(info.LastActivity)
	count := facts
```

Then the existing width arithmetic uses `count` as before (`titleW := width - head - 1 - count - rightW - 2`). When `titleW < 8`, the existing code drops the bar; add one more step: when it is still under 8, cut `count` to what fits with `truncateRunesHelper(count, width-lipgloss.Width(head)-1-8, "…")`. The `done` tail gets ` · P1 n` as in `epicBarLine`: replace `done := fmt.Sprintf(" %d/%d", ...)` with the same tail logic.

`stripANSI` is a test helper; check `grep -n "func stripANSI" pkg/ui/`. If it is only in a `_test.go` file, build `meta` plain here instead: write a `epicMetaText(info) string` that returns the unstyled text, and have `epicMetaLine` style it. Prefer that split from the start.

In `lanesBody`, the rows design's cut-lane line reads `lanes[l.lane].head[1]`; that stays the facts line, which is right. Lane height counting uses `len(lane.head)`, which already adapts.

**Step 4: Run to verify it passes**

Run: `go test ./pkg/ui/ -run 'TestBoardEpicRows' -v` → PASS, including `TestBoardEpicRows_EpicHeaderIsAFullWidthBox` and `TestBoardEpicRows_HeaderRowAboveTheCards`.

**Step 5: Commit**

```bash
git add pkg/ui/board_lanes.go pkg/ui/board_epic_facts_test.go
git commit -m "feat(board): epic rows header carries the epic facts and description

Refs: <task-id>"
```

---

### Task 6: Package, race and E2E runs

**Files:** none new.

**Step 1: Unit and vet**

```bash
go vet ./... && go test ./pkg/ui/ && go test ./... -skip DoltIntegration -p 2
```
Expected: all `ok`. The `-p 2` is a known Mac stall workaround (`bd memories b9s`).

**Step 2: Race on the UI package**

```bash
go test ./pkg/ui/ -race
```

**Step 3: E2E board tests, bounded**

```bash
go test ./tests/e2e/ -run 'TestBoard' -v -timeout 600s 2>&1 | tail -30
```
Expected: every `TestBoard*` passes. `TestBoardEpicDesignsCycleWithV` wants `"╭"`, which the epic cell box still provides. If a test hangs past its own timeout, kill the run and read the fixture; do not raise the timeout.

**Step 4: Look at it**

```bash
make build && BEADS_DIR="$(git rev-parse --git-common-dir)/../.beads" ./b9s
```
Press `b`, look at both designs with `v`, fold with `Tab`, select cards with `j`/`k`, then `Ctrl-C`. Fix what reads wrong before the docs task. The test fixture never shows a description, so this is the only check of the real wrap and clipping.

No commit for this task unless a fix was needed.

---

### Task 7: Docs and ADR status

**Files:**
- Modify: `README.md:158`, `docs/ARCHITECTURE.md` (the `pkg/ui` paragraph near line 70), `docs/adr/0030-...md` (status), `docs/adr/0018-...md` (status only)

**Step 1: README**

In the board paragraph at line 158, replace `Each issue is a boxed card, and empty columns fold into rails.` with:

> Each issue is one line: its type, ID and title, with `blocked`, `lane`, `blocks` and `P0`/`P1` tags at the right end. The selected issue expands to a card with its tags and age. Empty columns fold into rails.

And after `the epic rail shows each epic with its completion in a column on the left`, add:

> The epic's cell also shows the first sentence of its description, how many of its issues are in progress, waiting on a blocker or ready, its open P0 and P1 count, its labels, owner and due date, and when it last changed.

**Step 2: ARCHITECTURE.md**

In the `pkg/ui` file list sentence, after `board.go the board`, add `, board_lanes.go the epic lanes and cards, board_epics.go the epic index and the facts each epic cell shows (ADR 0030)`.

**Step 3: ADR status**

- `docs/adr/0030-...md`: `status: proposed` → `status: active`.
- `docs/adr/0018-...md`: `status: active` → `status: superseded`, and add `superseded_by: "0030"` under `date`. No other edit.

**Step 4: Commit**

```bash
git add README.md docs/ARCHITECTURE.md docs/adr/0030-compact-board-with-one-line-cards-and-epic-context.md docs/adr/0018-show-epics-as-a-rail-or-rows-and-hide-closed.md
git commit -m "docs(board): describe the compact board and activate ADR 0030

Refs: <task-id>"
```

---

### Task 8: Follow-up ticket and landing

**Step 1: File the web follow-up**

```bash
bd create --title="Compact cards and epic facts on the web board" --type=feature --parent=bd-e5u3.20 --priority=2 \
  --deps discovered-from:bd-e5u3.20 \
  --description="The TUI board shows one-line cards and an epic cell with description, counts, labels, owner, due and activity (ADR 0030). web/src/render.ts keeps the boxed cards and the four-fact epic cell. Bring the wide web board and the phone lane headers to the same facts."
```
Prefix its title with its short path as the convention requires.

**Step 2: Land the branch**

Follow `AGENTS.md` "Landing a branch": update local `main`, `git reset --soft $(git merge-base HEAD main)`, one commit `feat(board): compact one-line cards and an epic cell with context` with `Refs:` for every task id, `git rebase main`, rerun `go test ./pkg/ui/` and the board E2E tests on the rebased commit, `git push --force-with-lease`, then `git merge --ff-only` on main. Ask before pushing `main`.
