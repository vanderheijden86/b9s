package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

// cursorRenderIssues mirrors a sprint board: two epics, each with features
// holding enough subtasks that scrolling pushes their ancestors off-screen and
// sticky ancestor lines start taking viewport rows.
func cursorRenderIssues() []model.Issue {
	now := time.Now()
	var issues []model.Issue
	child := func(id, title, parent string, typ model.IssueType, offset int) model.Issue {
		return model.Issue{
			ID: id, Title: title, Priority: 2, IssueType: typ,
			CreatedAt: now.Add(time.Duration(offset) * time.Minute),
			Dependencies: []*model.Dependency{
				{IssueID: id, DependsOnID: parent, Type: model.DepParentChild},
			},
		}
	}
	offset := 0
	for e := 1; e <= 2; e++ {
		epicID := fmt.Sprintf("e%d", e)
		issues = append(issues, model.Issue{
			ID: epicID, Title: fmt.Sprintf("Epic-%d", e), Priority: 1,
			IssueType: model.TypeEpic, CreatedAt: now.Add(time.Duration(offset) * time.Minute),
		})
		offset++
		for f := 1; f <= 2; f++ {
			featID := fmt.Sprintf("%s.%d", epicID, f)
			issues = append(issues, child(featID, fmt.Sprintf("Feature-%d-%d", e, f), epicID, model.TypeFeature, offset))
			offset++
			for s := 1; s <= 6; s++ {
				subID := fmt.Sprintf("%s.%d", featID, s)
				issues = append(issues, child(subID, fmt.Sprintf("Subtask-%d-%d-%d", e, f, s), featID, model.TypeTask, offset))
				offset++
			}
		}
	}
	return issues
}

func newCursorRenderTree(t *testing.T, height int) TreeModel {
	t.Helper()
	tree := newIsolatedTree(t)
	tree.Build(cursorRenderIssues())
	tree.SetSize(120, height)
	tree.ExpandAll()
	if tree.NodeCount() <= height {
		t.Fatalf("fixture must overflow the viewport: %d nodes for height %d", tree.NodeCount(), height)
	}
	return tree
}

// assertSelectedRowRendered fails when the selected issue is absent from the
// rows View() draws, or when the frame is taller than the tree was given.
// Sticky lines only name ancestors of the first drawn row, and the cursor is
// never above that row, so a sticky line cannot stand in for the selection.
func assertSelectedRowRendered(t *testing.T, tree *TreeModel, height int, step string) {
	t.Helper()
	node := tree.SelectedNode()
	if node == nil || node.Issue == nil {
		t.Fatalf("%s: no selected node", step)
	}
	view := strings.TrimRight(tree.View(), "\n")
	if !strings.Contains(view, node.Issue.Title) {
		t.Fatalf("%s: selected %q (cursor %d, offset %d) is not rendered:\n%s",
			step, node.Issue.Title, tree.cursor, tree.viewportOffset, view)
	}
	if lines := strings.Count(view, "\n") + 1; lines > height {
		t.Fatalf("%s: View() is %d lines, taller than height %d", step, lines, height)
	}
}

func TestTreeSelectedRowStaysRendered(t *testing.T) {
	const height = 20

	t.Run("moving down through every row", func(t *testing.T) {
		tree := newCursorRenderTree(t, height)
		assertSelectedRowRendered(t, &tree, height, "start")
		for i := 1; i < tree.NodeCount(); i++ {
			tree.MoveDown()
			assertSelectedRowRendered(t, &tree, height, fmt.Sprintf("down %d", i))
		}
	})

	t.Run("moving up from the bottom", func(t *testing.T) {
		tree := newCursorRenderTree(t, height)
		tree.JumpToBottom()
		for i := 1; i < tree.NodeCount(); i++ {
			tree.MoveUp()
			assertSelectedRowRendered(t, &tree, height, fmt.Sprintf("up %d", i))
		}
	})

	t.Run("jumping to the bottom", func(t *testing.T) {
		tree := newCursorRenderTree(t, height)
		tree.JumpToBottom()
		assertSelectedRowRendered(t, &tree, height, "bottom")
	})

	t.Run("jumping between search matches", func(t *testing.T) {
		tree := newCursorRenderTree(t, height)
		for _, ch := range "Subtask" {
			tree.SearchAddChar(ch)
		}
		if tree.SearchMatchCount() < 2 {
			t.Fatalf("expected several search matches, got %d", tree.SearchMatchCount())
		}
		assertSelectedRowRendered(t, &tree, height, "first match")
		for i := 1; i < tree.SearchMatchCount(); i++ {
			tree.NextSearchMatch()
			assertSelectedRowRendered(t, &tree, height, fmt.Sprintf("match %d", i))
		}
	})

	t.Run("moving down inside XRay", func(t *testing.T) {
		const xrayHeight = 10
		tree := newCursorRenderTree(t, xrayHeight)
		tree.JumpToTop()
		tree.ToggleXRay()
		if !tree.IsXRayMode() {
			t.Fatal("expected XRay mode on the first epic")
		}
		for i := 1; i < tree.NodeCount(); i++ {
			tree.MoveDown()
			assertSelectedRowRendered(t, &tree, xrayHeight, fmt.Sprintf("xray down %d", i))
		}
	})
}
