package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func decisionTree(t *testing.T, width int, withGraph bool) *TreeModel {
	t.Helper()
	g := fixtureMemoryGraph()
	tree := NewTreeModel(newTreeTestTheme())
	if withGraph {
		tree.SetDecisionLookup(func() (datasource.MemoryGraph, bool) { return g, true })
	}
	tree.SetSize(width, 20)
	tree.Build(g.Issues())
	tree.ExpandAll()
	return &tree
}

func TestDecisionCellNamesEachDecisionAndItsState(t *testing.T) {
	g := fixtureMemoryGraph()
	cases := map[string]string{
		"sample-e.1":  "0009✗ (0027)",
		"sample-e.2":  "0026?",
		"sample-lone": "·  none",
		"sample-e":    "",
	}
	for id, want := range cases {
		if got, _ := decisionCell(g, id); got != want {
			t.Errorf("decisionCell(%s) = %q, want %q", id, got, want)
		}
	}
}

func TestTreeShowsDecisionsColumnOnlyForAGraphWorkspace(t *testing.T) {
	with := stripANSI(decisionTree(t, 140, true).View())
	if !strings.Contains(with, "DECISIONS") || !strings.Contains(with, "0009✗ (0027)") || !strings.Contains(with, "0026?") {
		t.Fatalf("graph workspace tree lacks the DECISIONS column:\n%s", with)
	}
	without := stripANSI(decisionTree(t, 140, false).View())
	if strings.Contains(without, "DECISIONS") {
		t.Fatalf("a project without Memory Links must not spend width on DECISIONS:\n%s", without)
	}
}

func TestDecisionsColumnKeepsRowsAlignedAndYieldsOnNarrowScreens(t *testing.T) {
	tree := decisionTree(t, 140, true)
	lines := strings.Split(stripANSI(tree.View()), "\n")
	header := stripANSI(tree.RenderHeader())
	col := lipgloss.Width(header[:strings.Index(header, "DECISIONS")])
	for _, l := range lines {
		if i := strings.Index(l, "0026?"); i >= 0 && lipgloss.Width(l[:i]) != col {
			t.Fatalf("cell at %d, header at %d:\n%s\n%s", i, col, header, l)
		}
	}
	if narrow := stripANSI(decisionTree(t, 90, true).RenderHeader()); strings.Contains(narrow, "DECISIONS") {
		t.Fatalf("auto layout keeps the title width on a narrow screen: %s", narrow)
	}
	tree.SetColumnPreference(TreeColumnDecisions, ColumnHide)
	if strings.Contains(stripANSI(tree.RenderHeader()), "DECISIONS") {
		t.Fatal("Hide must remove the column")
	}
}

func TestTreeRereadsDecisionsOnEveryBuild(t *testing.T) {
	g := fixtureMemoryGraph()
	served := false
	tree := NewTreeModel(newTreeTestTheme())
	tree.SetDecisionLookup(func() (datasource.MemoryGraph, bool) { return g, served })
	tree.SetSize(140, 20)
	tree.Build(g.Issues())
	if strings.Contains(stripANSI(tree.RenderHeader()), "DECISIONS") {
		t.Fatal("no graph read yet")
	}
	served = true
	tree.Build([]model.Issue(g.Issues()))
	if !strings.Contains(stripANSI(tree.RenderHeader()), "DECISIONS") {
		t.Fatal("a reload that brings a graph must show the column")
	}
}
