package ui

import (
	"reflect"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func newMarkTestTree(t *testing.T) *TreeModel {
	t.Helper()
	now := time.Now()
	issues := []model.Issue{
		{ID: "item-1", Title: "Item 1", Priority: 1, IssueType: model.TypeTask, CreatedAt: now.Add(4 * time.Hour)},
		{ID: "item-2", Title: "Item 2", Priority: 2, IssueType: model.TypeTask, CreatedAt: now.Add(3 * time.Hour)},
		{ID: "item-3", Title: "Item 3", Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(2 * time.Hour)},
		{ID: "item-4", Title: "Item 4", Priority: 3, IssueType: model.TypeTask, CreatedAt: now.Add(time.Hour)},
		{ID: "item-5", Title: "Item 5", Priority: 3, IssueType: model.TypeTask, CreatedAt: now},
	}
	tree := NewTreeModel(newTreeTestTheme())
	tree.Build(issues)
	if got := tree.GetSelectedID(); got != "item-1" {
		t.Fatalf("fixture expects cursor on item-1, got %q", got)
	}
	return &tree
}

func TestTreeSpanMarkExtendsBackToPreviousMark(t *testing.T) {
	tree := newMarkTestTree(t)
	tree.MoveDown() // item-2
	tree.ToggleMark()
	tree.MoveDown()
	tree.MoveDown() // item-4

	tree.SpanMark()

	want := []string{"item-2", "item-3", "item-4"}
	if got := tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v marked, got %v", want, got)
	}
}

func TestTreeSpanMarkExtendsForwardToNextMark(t *testing.T) {
	tree := newMarkTestTree(t)
	tree.MoveDown()
	tree.MoveDown()
	tree.MoveDown() // item-4
	tree.ToggleMark()
	tree.MoveUp()
	tree.MoveUp() // item-2

	tree.SpanMark()

	want := []string{"item-2", "item-3", "item-4"}
	if got := tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v marked, got %v", want, got)
	}
}

func TestTreeSpanMarkWithoutAnchorMarksCursorRow(t *testing.T) {
	tree := newMarkTestTree(t)
	tree.MoveDown() // item-2

	tree.SpanMark()

	want := []string{"item-2"}
	if got := tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v marked, got %v", want, got)
	}
}

func TestTreeUnmarkRemovesOnlyGivenIDs(t *testing.T) {
	tree := newMarkTestTree(t)
	for i := 0; i < 3; i++ {
		tree.ToggleMark()
		tree.MoveDown()
	}

	tree.Unmark("item-1", "item-3", "not-marked")

	want := []string{"item-2"}
	if got := tree.TreeMarkedIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v marked, got %v", want, got)
	}
	if got := tree.MarkedCount(); got != 1 {
		t.Fatalf("expected MarkedCount 1, got %d", got)
	}
}
