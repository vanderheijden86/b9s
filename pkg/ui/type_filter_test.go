package ui

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestTypeQuickFilterTogglesTypeTermsInSharedQuery(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressNamedKey(m, "Y")
	if m.pickerMode != pickerModeTypes {
		t.Fatal("Y should open the type picker")
	}
	m = pressNamedKey(m, "4") // epic
	if got := m.QueryText(); got != "type:epic" {
		t.Fatalf("query = %q, want type:epic", got)
	}
	m = pressNamedKey(m, "1") // bug, alternative to epic
	if got := m.QueryText(); got != "type:epic type:bug" {
		t.Fatalf("query = %q, want both types", got)
	}
	if n := len(m.FilteredIssues()); n != 2 {
		t.Errorf("list shows %d issues, want the epic and the bug", n)
	}
	m = pressNamedKey(m, "4")
	if got := m.QueryText(); got != "type:bug" {
		t.Fatalf("toggling epic again must remove it, got %q", got)
	}
	m = pressNamedKey(m, "0")
	if got := m.QueryText(); got != "" {
		t.Fatalf("0 must clear the type filter, got %q", got)
	}
}

func TestTypeQuickFilterComposesWithOtherTerms(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m.setQueryText("status:open")
	m.queryState.Accept()
	m = pressNamedKey(m, "Y")
	m = pressNamedKey(m, "3") // task
	if got := m.QueryText(); got != "status:open type:task" {
		t.Fatalf("query = %q", got)
	}
	m = pressNamedKey(m, "0")
	if got := m.QueryText(); got != "status:open" {
		t.Fatalf("0 must keep the other terms, got %q", got)
	}
}

func TestTypeQuickFilterWorksInTreeAndBoard(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressNamedKey(m, "Y")
	m = pressNamedKey(m, "4")
	if got := m.TreeNodeCount(); got != 1 {
		t.Errorf("tree shows %d nodes, want only the epic", got)
	}
	m = pressNamedKey(m, "b")
	if !m.IsBoardView() {
		t.Fatal("b should open the board")
	}
	if got := m.QueryText(); got != "type:epic" {
		t.Errorf("board lost the shared query: %q", got)
	}
	m = pressNamedKey(m, "Y")
	m = pressNamedKey(m, "1")
	if got := m.QueryText(); got != "type:epic type:bug" {
		t.Errorf("board type chip query = %q", got)
	}
}

func TestTypeBarShowsCountsAndActiveType(t *testing.T) {
	m := keybindingTestModel(t, nil)
	m = pressNamedKey(m, "Y")
	m = pressNamedKey(m, "4")
	view := m.View()
	for _, want := range []string{"<1>", "<4>", "epic", "bug", "type:epic"} {
		if !strings.Contains(view, want) {
			t.Errorf("view misses %q", want)
		}
	}
}

func TestTypeKeyIsNotShadowedByCustomBindingsWithoutOverride(t *testing.T) {
	_, errs := ResolveKeybindings(config.Keybindings{{Key: "Y", Query: "type:epic"}})
	if len(errs) != 1 {
		t.Fatalf("Y is built in, errs = %v", errs)
	}
}
