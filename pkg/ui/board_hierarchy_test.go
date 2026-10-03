package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// hierarchyIssue builds an open P2 issue; a later seq is a newer issue.
func hierarchyIssue(id, parent string, typ model.IssueType, status model.Status, seq int) model.Issue {
	at := time.Date(2026, 10, 1, 12, seq, 0, 0, time.UTC)
	is := model.Issue{
		ID: id, Title: "Title of " + id, Status: status, IssueType: typ, Priority: 2,
		CreatedAt: at, UpdatedAt: at,
	}
	if parent != "" {
		is.Dependencies = []*model.Dependency{{IssueID: id, DependsOnID: parent, Type: model.DepParentChild}}
	}
	return is
}

// hierarchyIssues is an epic with two features, each with tasks, one subtask
// and one task that has left its feature's column.
func hierarchyIssues() []model.Issue {
	return []model.Issue{
		hierarchyIssue("mdu-1", "", model.TypeEpic, model.StatusOpen, 0),
		hierarchyIssue("mdu-1.1", "mdu-1", model.TypeFeature, model.StatusOpen, 1),
		hierarchyIssue("mdu-1.1.1", "mdu-1.1", model.TypeTask, model.StatusOpen, 2),
		hierarchyIssue("mdu-1.1.2", "mdu-1.1", model.TypeTask, model.StatusOpen, 3),
		hierarchyIssue("mdu-1.1.2.1", "mdu-1.1.2", model.TypeTask, model.StatusOpen, 4),
		hierarchyIssue("mdu-1.2", "mdu-1", model.TypeFeature, model.StatusOpen, 5),
		hierarchyIssue("mdu-1.2.1", "mdu-1.2", model.TypeTask, model.StatusOpen, 6),
		hierarchyIssue("mdu-1.2.2", "mdu-1.2", model.TypeTask, model.StatusInProgress, 7),
	}
}

func TestBoardHierarchy_ParentsSortAboveTheirChildren(t *testing.T) {
	for _, tc := range []struct {
		name   string
		issues []model.Issue
		want   []string
	}{
		{"epic lanes", hierarchyIssues(), []string{
			"mdu-1",
			"mdu-1.2", "mdu-1.2.1",
			"mdu-1.1", "mdu-1.1.2", "mdu-1.1.2.1", "mdu-1.1.1",
		}},
		// Without an epic the board has no lanes, and the order still holds.
		{"no epic", hierarchyIssues()[1:], []string{
			"mdu-1.2", "mdu-1.2.1",
			"mdu-1.1", "mdu-1.1.2", "mdu-1.1.2.1", "mdu-1.1.1",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBoardModel(tc.issues, DefaultTheme(lipgloss.DefaultRenderer()))
			if got := columnIDs(b, ColOpen); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("open column order\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}

func TestBoardHierarchy_DepthCountsParentsInTheSameCell(t *testing.T) {
	b := NewBoardModel(hierarchyIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	for id, want := range map[string]int{
		"mdu-1.1":     0, // the lane's epic is the header, not a level
		"mdu-1.1.1":   1,
		"mdu-1.1.2.1": 2,
		"mdu-1.2.2":   0, // its feature is in another column
	} {
		if got := b.cardDepth[id]; got != want {
			t.Errorf("depth of %s = %d, want %d", id, got, want)
		}
	}
}

func TestBoardHierarchy_CardLineIndentsByDepth(t *testing.T) {
	b := NewBoardModel(hierarchyIssues(), DefaultTheme(lipgloss.DefaultRenderer()))
	b.SetActiveProjectName("mdu")
	line := func(id string, width int) string {
		t.Helper()
		got := b.cardLines(*b.issueMap[id], width, false, 0, 0)[0]
		if w := lipgloss.Width(got); w != width {
			t.Fatalf("%s at %d cells is %d wide", id, width, w)
		}
		return strings.TrimRight(stripANSI(got), " ")
	}
	leadingSpaces := func(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

	for id, want := range map[string]int{"mdu-1.1": 0, "mdu-1.1.1": 2, "mdu-1.1.2.1": 4, "mdu-1.2.2": 0} {
		if got := leadingSpaces(line(id, 80)); got != want {
			t.Errorf("%s indents %d cells, want %d: %q", id, got, want, line(id, 80))
		}
	}
	// A narrow column keeps the ID and title rather than the indent.
	for w := 4; w < cardLineIndentMinWidth; w += 3 {
		if got := leadingSpaces(line("mdu-1.1.2.1", w)); got != 0 {
			t.Errorf("at %d cells the card must not indent, got %d", w, got)
		}
	}
}
