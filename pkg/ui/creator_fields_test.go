package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
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

func TestBoardDetail_ShowsCreator(t *testing.T) {
	board := NewBoardModel([]model.Issue{creatorFixture()}, DefaultTheme(lipgloss.DefaultRenderer()))
	board.SetLayout(BoardLayoutInspector)
	view := stripANSI(board.View(180, 40))
	if !strings.Contains(view, "Creator") || !strings.Contains(view, "@alice") {
		t.Fatalf("board detail omits the creator:\n%s", view)
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
