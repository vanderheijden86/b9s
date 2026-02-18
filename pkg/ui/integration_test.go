package ui_test

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// View Transition Integration Tests (bv-i3ls)
// Tests verifying state preservation and behavior across view switches

// Helper to create a KeyMsg for a string key
func integrationKeyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	}
}

// Helper to create special key messages
func integrationSpecialKey(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

// createTestIssues creates a set of test issues for integration tests
func createTestIssues(count int) []model.Issue {
	issues := make([]model.Issue, count)
	statuses := []model.Status{model.StatusOpen, model.StatusInProgress, model.StatusBlocked, model.StatusClosed}
	priorities := []int{0, 1, 2, 3}

	for i := 0; i < count; i++ {
		issues[i] = model.Issue{
			ID:        "test-" + string(rune('a'+i%26)) + string(rune('0'+i/26)),
			Title:     "Test Issue",
			Status:    statuses[i%len(statuses)],
			Priority:  priorities[i%len(priorities)],
			IssueType: model.TypeTask,
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return issues
}

// Basic View Switching Tests

// TestTreeViewPermanent verifies tree is the permanent view (bd-8hw.4)
func TestTreeViewPermanent(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Should start in tree view (bd-dxc)
	if m.FocusState() != "tree" {
		t.Errorf("Expected initial focus 'tree', got %q", m.FocusState())
	}

	// 'E' is a no-op from tree (bd-8hw.4: tree is permanent)
	newM, _ := m.Update(integrationKeyMsg("E"))
	m = newM.(ui.Model)

	if m.FocusState() != "tree" {
		t.Errorf("After 'E', expected 'tree' (permanent), got %q", m.FocusState())
	}
}

// TestViewTransitionTreeToBoard verifies Tree -> Board -> Tree transition (bd-8hw.4)
func TestViewTransitionTreeToBoard(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Press 'b' to toggle board view from tree
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if !m.IsBoardView() {
		t.Error("IsBoardView should be true after 'b'")
	}

	// Press 'b' again to toggle back to tree
	newM, _ = m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if m.IsBoardView() {
		t.Error("IsBoardView should be false after second 'b'")
	}
	if m.FocusState() != "tree" {
		t.Errorf("Expected focus 'tree' after board toggle, got %q", m.FocusState())
	}
}

// TestViewTransitionBoardCycle verifies Tree -> Board -> Tree cycle (bd-8hw.4)
func TestViewTransitionBoardCycle(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Enter board view from tree
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)
	if !m.IsBoardView() {
		t.Error("Should be in board view")
	}
	if m.FocusState() != "board" {
		t.Errorf("Should be in board focus, got %q", m.FocusState())
	}

	// Toggle board off, returns to tree
	newM, _ = m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)
	if m.IsBoardView() {
		t.Error("Board should be toggled off")
	}
	if m.FocusState() != "tree" {
		t.Errorf("After toggling board off, expected 'tree', got %q", m.FocusState())
	}
}

// State Preservation Tests

// TestViewTransitionClearsOtherViews verifies toggling board off returns to tree (bd-8hw.4)
func TestViewTransitionClearsOtherViews(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Enter board view from tree
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	if !m.IsBoardView() {
		t.Error("Should be in board view")
	}

	// Esc from board returns to tree
	newM, _ = m.Update(integrationSpecialKey(tea.KeyEsc))
	m = newM.(ui.Model)

	if m.IsBoardView() {
		t.Error("Board view should be cleared after Esc")
	}
	if m.FocusState() != "tree" {
		t.Error("Should be in tree view after Esc from board")
	}
}

// TestViewTransitionFilterPreserved verifies filter state is preserved across board toggle (bd-8hw.4)
func TestViewTransitionFilterPreserved(t *testing.T) {
	issues := createTestIssues(10)
	m := ui.NewModel(issues, "")

	// Apply a filter from tree
	m.SetFilter("open")
	initialCount := len(m.FilteredIssues())

	// Switch to board and back
	newM, _ := m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	newM, _ = m.Update(integrationKeyMsg("b"))
	m = newM.(ui.Model)

	// Filter should still be active
	afterCount := len(m.FilteredIssues())
	if afterCount != initialCount {
		t.Errorf("Filter not preserved: before=%d, after=%d", initialCount, afterCount)
	}
}

// Edge Case Tests

// TestViewTransitionEmptyIssues verifies view switching with no issues doesn't panic
func TestViewTransitionEmptyIssues(t *testing.T) {
	m := ui.NewModel([]model.Issue{}, "")

	// Should not panic on any view transition
	keys := []string{"E", "b", "g", "a", "i", "?"}
	for _, k := range keys {
		newM, _ := m.Update(integrationKeyMsg(k))
		m = newM.(ui.Model)
	}
}

// TestViewTransitionEscBehavior verifies Esc behavior varies by view (bd-8hw.4)
func TestViewTransitionEscBehavior(t *testing.T) {
	issues := createTestIssues(10)

	t.Run("tree_is_permanent", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		if m.FocusState() != "tree" {
			t.Fatalf("Expected tree, got %q", m.FocusState())
		}

		// 'E' from tree is a no-op (tree is permanent, bd-8hw.4)
		newM, _ := m.Update(integrationKeyMsg("E"))
		m = newM.(ui.Model)

		if m.FocusState() != "tree" {
			t.Errorf("'E' from tree should stay in tree, got %q", m.FocusState())
		}
	})

	t.Run("board_toggle_exits_board", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		newM, _ := m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)

		// Press 'b' again to toggle off board
		newM, _ = m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)

		if m.IsBoardView() {
			t.Error("'b' should toggle off board view")
		}
		if m.FocusState() != "tree" {
			t.Errorf("Expected tree after board toggle, got %q", m.FocusState())
		}
	})

	t.Run("esc_from_board_returns_to_tree", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		newM, _ := m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)

		newM, _ = m.Update(integrationSpecialKey(tea.KeyEsc))
		m = newM.(ui.Model)

		if m.FocusState() != "tree" {
			t.Errorf("Esc from board should return to tree, got %q", m.FocusState())
		}
	})
}

// TestViewToggleExitBehavior verifies toggle keys (bd-8hw.4: tree permanent)
func TestViewToggleExitBehavior(t *testing.T) {
	issues := createTestIssues(10)

	t.Run("tree_permanent", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		if m.FocusState() != "tree" {
			t.Errorf("Expected tree, got %q", m.FocusState())
		}
		// 'E' is no-op (tree is permanent)
		newM, _ := m.Update(integrationKeyMsg("E"))
		m = newM.(ui.Model)
		if m.FocusState() != "tree" {
			t.Errorf("'E' should stay in tree, got %q", m.FocusState())
		}
	})

	t.Run("board_b_toggle", func(t *testing.T) {
		m := ui.NewModel(issues, "")
		newM, _ := m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)
		if !m.IsBoardView() {
			t.Error("Should be in board view")
		}
		newM, _ = m.Update(integrationKeyMsg("b"))
		m = newM.(ui.Model)
		if m.IsBoardView() {
			t.Error("'b' should toggle off board")
		}
		if m.FocusState() != "tree" {
			t.Errorf("After board toggle, expected 'tree', got %q", m.FocusState())
		}
	})
}

// Rapid Switching Stress Tests

// TestRapidViewSwitching verifies no panics during rapid view changes
func TestRapidViewSwitching(t *testing.T) {
	issues := createTestIssues(50)
	m := ui.NewModel(issues, "")

	keys := []string{"E", "b", "g", "a", "i", "E", "b", "g"}

	for i := 0; i < 100; i++ {
		for _, k := range keys {
			newM, _ := m.Update(integrationKeyMsg(k))
			m = newM.(ui.Model)
		}
	}
}

// TestRapidViewSwitchingWithNavigation verifies navigation during rapid switches
func TestRapidViewSwitchingWithNavigation(t *testing.T) {
	issues := createTestIssues(50)
	m := ui.NewModel(issues, "")

	actions := []tea.KeyMsg{
		integrationKeyMsg("E"),            // Toggle tree (exits default tree)
		integrationKeyMsg("j"),            // Move down in list
		integrationKeyMsg("j"),            // Move down in list
		integrationKeyMsg("b"),            // Enter board
		integrationKeyMsg("l"),            // Move right in board
		integrationKeyMsg("g"),            // Enter graph
		integrationKeyMsg("j"),            // Move down in graph
		integrationSpecialKey(tea.KeyEsc), // Exit to list
		integrationKeyMsg("j"),            // Move down in list
	}

	for i := 0; i < 50; i++ {
		for _, k := range actions {
			newM, _ := m.Update(k)
			m = newM.(ui.Model)
		}
	}
}

// Performance Tests

// TestViewSwitchingPerformance verifies reasonable performance for view switching
func TestViewSwitchingPerformance(t *testing.T) {
	issues := createTestIssues(100)
	m := ui.NewModel(issues, "")

	keys := []string{"E", "b", "g", "E", "b", "g"}

	start := time.Now()

	for i := 0; i < 100; i++ {
		for _, k := range keys {
			newM, _ := m.Update(integrationKeyMsg(k))
			m = newM.(ui.Model)
		}
	}

	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("View switching too slow: %v for 600 switches", elapsed)
	}
}

// Help View Integration Tests

// TestHelpViewTransition verifies help view can be opened from tree and board (bd-8hw.4)
func TestHelpViewTransition(t *testing.T) {
	issues := createTestIssues(10)

	views := []struct {
		name     string
		enterKey string
	}{
		{"tree", ""},    // Default is tree (bd-dxc)
		{"board", "b"},  // Board accessible from tree (bd-8hw.4)
	}

	for _, v := range views {
		t.Run(v.name, func(t *testing.T) {
			m := ui.NewModel(issues, "")

			// Enter the base view
			if v.enterKey != "" {
				newM, _ := m.Update(integrationKeyMsg(v.enterKey))
				m = newM.(ui.Model)
			}

			// Open help with '?'
			newM, _ := m.Update(integrationKeyMsg("?"))
			m = newM.(ui.Model)

			if m.FocusState() != "help" {
				t.Errorf("Expected help focus from %s view, got %q", v.name, m.FocusState())
			}

			// Exit help with Esc
			newM, _ = m.Update(integrationSpecialKey(tea.KeyEsc))
			m = newM.(ui.Model)

			if m.FocusState() == "help" {
				t.Error("Should have exited help with Esc")
			}
		})
	}
}

// View Rendering Integration Tests

// TestAllViewsRenderWithoutPanic verifies all views can render without panic (bd-8hw.4)
func TestAllViewsRenderWithoutPanic(t *testing.T) {
	issues := createTestIssues(20)

	views := []struct {
		name     string
		enterKey string
	}{
		{"tree", ""},    // Default is tree (bd-dxc)
		{"board", "b"},  // Board from tree (bd-8hw.4)
		{"insights", "i"},
		{"help", "?"},
	}

	for _, v := range views {
		t.Run(v.name, func(t *testing.T) {
			m := ui.NewModel(issues, "")

			// Enter the view
			if v.enterKey != "" {
				newM, _ := m.Update(integrationKeyMsg(v.enterKey))
				m = newM.(ui.Model)
			}

			// Render should not panic
			output := m.View()
			if output == "" {
				t.Errorf("View() returned empty for %s view", v.name)
			}
		})
	}
}

// TestViewRenderingAtDifferentSizes verifies views render at various terminal sizes (bd-8hw.4)
func TestViewRenderingAtDifferentSizes(t *testing.T) {
	issues := createTestIssues(20)

	sizes := []struct {
		width, height int
	}{
		{80, 24},
		{120, 30},
		{160, 40},
		{40, 15},  // Narrow
		{200, 50}, // Wide
	}

	views := []struct {
		key  string
		name string
	}{
		{"", "tree"},   // Default is tree
		{"b", "board"}, // Board from tree (bd-8hw.4)
	}

	for _, size := range sizes {
		for _, v := range views {
			t.Run(fmt.Sprintf("%s_%dx%d", v.name, size.width, size.height), func(t *testing.T) {
				m := ui.NewModel(issues, "")

				// Set size
				newM, _ := m.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})
				m = newM.(ui.Model)

				// Enter view
				if v.key != "" {
					newM, _ = m.Update(integrationKeyMsg(v.key))
					m = newM.(ui.Model)
				}

				// Render should not panic
				output := m.View()
				if output == "" {
					t.Errorf("View() returned empty for %s at %dx%d", v.name, size.width, size.height)
				}
			})
		}
	}
}
