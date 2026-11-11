package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/identity"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func creatorFixture() model.Issue {
	return model.Issue{
		ID: "bd-c1", Title: "Creator fixture", Status: model.StatusOpen,
		IssueType: model.TypeTask, Priority: 2, CreatedAt: time.Now(),
		CreatedBy: "alice", Owner: "alice@example.com", Assignee: "BrownBear",
	}
}

func TestFormatIssueMarkdown_ShowsCreatorSeparateFromAssignee(t *testing.T) {
	md := formatIssueMarkdown(creatorFixture(), nil)
	if !strings.Contains(md, "| Creator |") {
		t.Fatalf("meta table has no Creator column:\n%s", md)
	}
	if !strings.Contains(md, "@alice") || !strings.Contains(md, "@BrownBear") {
		t.Fatalf("meta table must show creator and assignee as separate values:\n%s", md)
	}
}

func TestFormatIssueMarkdown_ShowsOwnerEmail(t *testing.T) {
	md := formatIssueMarkdown(creatorFixture(), nil)
	if !strings.Contains(md, "alice@example.com") {
		t.Fatalf("detail should show the creator's git email:\n%s", md)
	}
}

func TestSanitizeIssueForTerminal_CleansCreatorFields(t *testing.T) {
	issue := creatorFixture()
	issue.CreatedBy = "alice\x1b]52;c;YXR0YWNr\x07"
	issue.Owner = "a@b\x1b[2J"
	clean := sanitizeIssueForTerminal(issue)
	for _, v := range []string{clean.CreatedBy, clean.Owner} {
		if strings.Contains(v, "\x1b") {
			t.Fatalf("creator field kept an escape sequence: %q", v)
		}
	}
}

func TestEditModal_ShowsCreatorReadOnly(t *testing.T) {
	issue := creatorFixture()
	modal := NewEditModal(&issue, DefaultTheme(lipgloss.DefaultRenderer()))
	modal.SetSize(100, 40)
	view := stripANSI(modal.View())
	if !strings.Contains(view, "created by @alice") {
		t.Fatalf("edit modal omits the creator:\n%s", view)
	}
}

func TestTreeColumnPopup_ListsCreatorAndAssignee(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.OpenColumnPopup()
	popup := stripANSI(tree.RenderColumnPopup())
	for _, label := range []string{"Creator", "Assignee"} {
		if !strings.Contains(popup, label) {
			t.Fatalf("column popup missing %q: %q", label, popup)
		}
	}
}

func TestTreeCreatorColumn_HiddenByDefault(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build([]model.Issue{creatorFixture()})
	tree.SetSize(160, 10)
	if strings.Contains(stripANSI(tree.View()), "CREATOR") {
		t.Fatal("creator column should be hidden unless asked for")
	}
}

func TestTreeCreatorColumn_ShowRendersCreator(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build([]model.Issue{creatorFixture()})
	tree.SetSize(160, 10)
	tree.SetColumnPreference(TreeColumnCreator, ColumnShow)
	view := stripANSI(tree.View())
	if !strings.Contains(view, "CREATOR") || !strings.Contains(view, "@alice") {
		t.Fatalf("creator column not rendered:\n%s", view)
	}
}

func TestTreeAssigneeColumn_ShowRendersAssignee(t *testing.T) {
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build([]model.Issue{creatorFixture()})
	tree.SetSize(160, 10)
	tree.SetColumnPreference(TreeColumnAssignee, ColumnShow)
	view := stripANSI(tree.View())
	if !strings.Contains(view, "ASSIGNEE") || !strings.Contains(view, "@BrownBear") {
		t.Fatalf("assignee column not rendered:\n%s", view)
	}
}

func TestIdentityNamesInPersonViews(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	reg, err := identity.Parse(`[
		{"name":"Al","kind":"human","aliases":["a","a_b"]},
		{"name":"Bo","kind":"agent","aliases":["b","b_b"]},
		{"name":"André","kind":"human","aliases":["andre@example.invalid"]}
	]`, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range []string{"detail", "board_detail", "tree_columns"} {
		for _, tc := range []struct {
			name, creator, assignee, wantCreator, wantAssignee string
			registry                                           *identity.Registry
		}{
			{"aliases", "a", "b", "@Al (@a)", "@Bo (@b)", reg},
			{"canonical_aliases", "a.b", "b-b", "@Al (@a.b)", "@Bo (@b-b)", reg},
			{"email_alias", "andre@example.invalid", "b", "@André (@andre@example.invalid)", "@Bo (@b)", reg},
			{"same_names", "Al", "Bo", "@Al", "@Bo", reg},
			{"unknown_names", "guest", "crew", "@guest", "@crew", reg},
			{"no_registry", "a", "b", "@a", "@b", nil},
			{"empty_names", "", "", "", "", reg},
		} {
			t.Run(view+"/"+tc.name, func(t *testing.T) {
				issue := creatorFixture()
				issue.CreatedBy, issue.Assignee, issue.Owner = tc.creator, tc.assignee, ""
				m := NewModel([]model.Issue{issue}, "")
				next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
				m = next.(Model)
				m.setIdentities(tc.registry)
				m.tree.SetColumnPreference(TreeColumnCreator, ColumnShow)
				m.tree.SetColumnPreference(TreeColumnAssignee, ColumnShow)
				m.treeViewActive, m.treeDetailHidden = true, true
				m.isSplitView = false
				m.focused = focusTree
				if view == "board_detail" {
					m.isBoardView = true
					m.focused = focusBoard
					m.board.SelectIssueByID(issue.ID)
				}
				if view != "tree_columns" {
					next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
					m = next.(Model)
					if m.focused != focusDetail {
						t.Fatalf("Enter did not open the %s", view)
					}
				}
				out := stripANSI(m.View())
				t.Logf("Rendered %s:\n%s", view, out)
				for _, want := range []string{tc.wantCreator, tc.wantAssignee} {
					if want != "" && !strings.Contains(out, want) {
						t.Errorf("%s missing %q:\n%s", view, want, out)
					}
				}
				for _, duplicate := range []string{"@Al (@Al)", "@Bo (@Bo)", "@guest (@guest)", "@crew (@crew)"} {
					if strings.Contains(out, duplicate) {
						t.Errorf("%s duplicates an unchanged name: %q", view, duplicate)
					}
				}
				if tc.creator == "" && tc.assignee == "" {
					if strings.Contains(out, "@") {
						t.Errorf("%s shows a bare @ for an empty person:\n%s", view, out)
					}
					if view != "tree_columns" && (!strings.Contains(out, "unknown") || !strings.Contains(out, "none")) {
						t.Errorf("%s lost the empty-person labels:\n%s", view, out)
					}
				}
				if got := m.issueMap[issue.ID]; got.CreatedBy != tc.creator || got.Assignee != tc.assignee {
					t.Fatal("rendering changed the stored aliases")
				}
			})
		}
	}
}

func TestPersonLabelSanitizesResolvedIdentity(t *testing.T) {
	reg, err := identity.Parse(`[{"name":"Al\u001b[2J","kind":"human","aliases":["a\u001b[1m"]}]`, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := personLabel(reg, "a\x1b[1m"); got != "@Al (@a)" {
		t.Fatalf("person label must resolve the raw alias and strip terminal controls: %q", got)
	}
}

func TestTreePersonColumnsKeepWidthsAndAlignment(t *testing.T) {
	reg, err := identity.Parse(`[{"name":"李","kind":"human","aliases":["li@example.invalid"]}]`, "")
	if err != nil {
		t.Fatal(err)
	}
	issue := creatorFixture()
	issue.CreatedBy, issue.Assignee = "li@example.invalid", "li@example.invalid"
	for _, width := range []int{96, 160, 240} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			tree := NewTreeModel(newTreeTestTheme())
			tree.identities = reg
			tree.Build([]model.Issue{issue})
			tree.SetSize(width, 10)
			for _, column := range []TreeColumn{TreeColumnPriority, TreeColumnDispatch, TreeColumnCreated} {
				tree.SetColumnPreference(column, ColumnHide)
			}
			tree.SetColumnPreference(TreeColumnCreator, ColumnShow)
			tree.SetColumnPreference(TreeColumnAssignee, ColumnShow)
			layout := tree.resolveColumnLayout()
			labelWidth := lipgloss.Width("@李 (@li@example.invalid)")
			wantWidth := min(max(treePersonColumnWidth, width/4), labelWidth)
			if layout.creatorWidth != wantWidth || layout.assigneeWidth != wantWidth {
				t.Fatalf("person widths = %d, %d, want %d", layout.creatorWidth, layout.assigneeWidth, wantWidth)
			}
			lines := strings.Split(stripANSI(tree.View()), "\n")
			header, row := lines[0], lines[1]
			for _, line := range []string{header, row} {
				if lipgloss.Width(line) > width {
					t.Fatalf("tree line exceeds terminal width %d: %q", width, line)
				}
			}
			for i, title := range []string{"CREATOR", "ASSIGNEE"} {
				headerColumn := lipgloss.Width(header[:strings.Index(header, title)])
				personIndex := strings.Index(row, "@李")
				if i == 1 {
					personIndex = strings.LastIndex(row, "@李")
				}
				if personIndex < 0 {
					t.Fatalf("missing person column:\n%s", row)
				}
				if column := lipgloss.Width(row[:personIndex]); column != headerColumn {
					t.Errorf("%s starts at %d, value at %d", title, headerColumn, column)
				}
			}
		})
	}
}

func TestIdentityReloadRefreshesVisiblePeople(t *testing.T) {
	issue := creatorFixture()
	issue.CreatedBy, issue.Assignee, issue.Owner = "a", "a", ""
	m := NewModel([]model.Issue{issue}, "").
		WithSourceType(datasource.SourceTypeDolt).
		WithDoltSource(datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", Database: "fixture"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	m = next.(Model)
	m.tree.SetColumnPreference(TreeColumnCreator, ColumnShow)
	m.tree.SetColumnPreference(TreeColumnAssignee, ColumnShow)
	m.treeViewActive, m.treeDetailHidden, m.focused = true, true, focusTree
	initial := m.tree.resolveColumnLayout()
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	reg, err := identity.Parse(`[{"name":"Alexandria","kind":"human","aliases":["a"]}]`, "")
	if err != nil {
		t.Fatal(err)
	}
	next, _ = m.Update(identitiesLoadedMsg{source: m.doltSource, registry: reg})
	m = next.(Model)
	if out := stripANSI(m.View()); !strings.Contains(out, "@Alexandria (@a)") {
		t.Fatalf("identity reload did not refresh the open detail:\n%s", out)
	}
	updated := m.tree.resolveColumnLayout()
	if updated.creatorWidth <= initial.creatorWidth || updated.assigneeWidth <= initial.assigneeWidth {
		t.Fatalf("identity reload kept stale person widths: before %+v, after %+v", initial, updated)
	}
}
