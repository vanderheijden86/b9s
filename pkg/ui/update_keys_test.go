package ui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestArrowKeysPaginateVisiblePage(t *testing.T) {
	for _, view := range []string{"tree", "tree-detail", "list"} {
		for _, width := range []int{80, 160} {
			for _, direction := range []tea.KeyType{tea.KeyRight, tea.KeyLeft} {
				t.Run(fmt.Sprintf("%s/width%d/%s", view, width, tea.KeyMsg{Type: direction}.String()), func(t *testing.T) {
					issues := make([]model.Issue, 100)
					for i := range issues {
						issues[i] = model.Issue{
							ID: fmt.Sprintf("page-%02d", i), Title: fmt.Sprintf("Paging task %02d", i),
							Status: model.StatusOpen, IssueType: model.TypeTask,
							CreatedAt: time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
						}
					}
					m := NewModel(issues, "")
					if view == "list" {
						m.focused = focusList
						m.treeViewActive = false
					}
					updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 20})
					m = updated.(Model)
					if view == "tree-detail" && m.isSplitView {
						updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
						m = updated.(Model)
					}

					pageSize := m.list.Paginator.PerPage
					if view != "list" {
						match := regexp.MustCompile(`Page 1/\d+ \(1-(\d+) of 100\)`).FindStringSubmatch(m.View())
						if len(match) != 2 {
							t.Fatalf("missing initial tree page indicator:\n%s", m.View())
						}
						var err error
						pageSize, err = strconv.Atoi(match[1])
						if err != nil {
							t.Fatal(err)
						}
					}
					if pageSize < 2 || pageSize >= len(issues) {
						t.Fatalf("fixture needs multiple pages, got page size %d", pageSize)
					}

					wantIndex := pageSize
					if direction == tea.KeyLeft {
						// Start on page three independently of the Right key handler.
						if view == "list" {
							m.list.Select(2 * pageSize)
						} else {
							m.tree.SelectByID(issues[2*pageSize].ID)
							m.tree.viewportOffset = 2 * pageSize
						}
					}
					_ = m.View()
					updated, _ = m.Update(tea.KeyMsg{Type: direction})
					m = updated.(Model)
					selected := m.getSelectedIssue()
					if selected == nil || selected.ID != issues[wantIndex].ID {
						t.Fatalf("%s must move one visible page (%d rows) to %s, got %+v", tea.KeyMsg{Type: direction}.String(), pageSize, issues[wantIndex].ID, selected)
					}
					if !strings.Contains(m.View(), issues[wantIndex].Title) {
						t.Fatalf("paged selection must be visible:\n%s", m.View())
					}
					if !strings.Contains(m.View(), "Page 2") {
						t.Fatalf("paging must render the second page:\n%s", m.View())
					}
				})
			}
		}
	}
}

func TestShiftKClosesSelectedIssueAfterConfirmation(t *testing.T) {
	issues := []model.Issue{
		{ID: "bd-123", Title: "Close this ticket", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")
	m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("Shift+K must wait for confirmation before closing")
	}
	view := m.View()
	if !strings.Contains(view, "Close issue?") {
		t.Fatalf("expected close confirmation, got:\n%s", view)
	}
	if !strings.Contains(view, "bd-123") || !strings.Contains(view, "Close this ticket") {
		t.Fatal("close confirmation must identify the selected issue")
	}
	if !strings.Contains(view, "[Y] Close") || !strings.Contains(view, "[Esc] Cancel") {
		t.Fatal("close confirmation must show explicit confirm and cancel buttons")
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("confirming close must return a command")
	}
	msg := cmd()
	result, ok := msg.(BdResultMsg)
	if !ok {
		t.Fatalf("expected BdResultMsg, got %T", msg)
	}
	if result.Operation != BdOpClose || result.IssueID != "bd-123" || !result.Success {
		t.Fatalf("unexpected close result: %#v", result)
	}
	if strings.Contains(m.View(), "Close issue?") {
		t.Fatal("close confirmation must dismiss after confirmation")
	}
}

func TestShiftKCloseConfirmationCanBeCancelled(t *testing.T) {
	m := NewModel([]model.Issue{
		{ID: "bd-123", Title: "Keep this ticket open", Status: model.StatusOpen},
	}, "")
	m.issueWriter = &IssueWriter{bdPath: "/bin/echo", available: true}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("cancelling close must not run a command")
	}
	if strings.Contains(m.View(), "Close issue?") {
		t.Fatal("close confirmation must dismiss after cancellation")
	}
}

func TestDeleteKeyRetainsDeleteConfirmation(t *testing.T) {
	m := NewModel([]model.Issue{
		{ID: "bd-123", Title: "Discard this ticket", Status: model.StatusOpen},
	}, "")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	m = updated.(Model)
	if cmd != nil {
		t.Fatal("Delete must wait for confirmation before deleting")
	}
	if !strings.Contains(m.View(), "Delete issue?") {
		t.Fatalf("expected delete confirmation, got:\n%s", m.View())
	}
}

func TestShiftKTargetsBoardSelection(t *testing.T) {
	m := NewModel([]model.Issue{
		{ID: "bd-tree", Title: "Tree selection", Status: model.StatusOpen},
		{ID: "bd-board", Title: "Board selection", Status: model.StatusBlocked},
	}, "")
	m.tree.SelectByID("bd-tree")
	m.board.SelectIssueByID("bd-board")
	m.isBoardView = true
	m.focused = focusBoard

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("K")})
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "bd-board") || strings.Contains(view, "bd-tree") {
		t.Fatalf("close confirmation must target the active board selection, got:\n%s", view)
	}
}

// Cover additional branches in Model.Update for quit/help/tab handling and update notices (bd-8hw.4).
func TestUpdateHelpQuitAndTabFocus(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "One", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Make model ready and split view
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Help toggle via ? then dismiss with another key
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp || m.focused != focusHelp {
		t.Fatalf("expected help overlay shown")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.showHelp || m.focused != focusTree {
		t.Fatalf("expected help overlay dismissed back to tree, got focus %v", m.focused)
	}

	// Tab always folds (CycleNodeVisibility), never switches focus (bd-lt1l).
	if m.focused != focusTree {
		t.Fatalf("expected tree focus, got %v", m.focused)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focused != focusTree {
		t.Fatalf("expected tree focus to remain after tab (fold), got %v", m.focused)
	}

	// Escape should show quit confirm, 'y' should issue tea.Quit
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if !m.showQuitConfirm {
		t.Fatalf("expected quit confirm after esc")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatalf("expected quit command on confirm quit")
	}
}

func TestUpdateMsgSetsUpdateAvailable(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "1", Title: "One", Status: model.StatusOpen}}, "")
	updated, _ := m.Update(UpdateMsg{TagName: "v9.9.9", URL: "https://example"})
	m = updated.(Model)
	if !m.updateAvailable || m.updateTag != "v9.9.9" {
		t.Fatalf("update flag not set")
	}
}

// TestNarrowWindowTreeDetailHidden verifies that in a narrow window (width <= SplitViewThreshold),
// treeDetailHidden is true so Enter opens full-screen detail (bd-6eg, bd-1of).
func TestNarrowWindowTreeDetailHidden(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Narrow window: below SplitViewThreshold (100)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)

	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden=true in narrow window")
	}
	if m.focused != focusTree {
		t.Fatalf("expected focusTree, got %v", m.focused)
	}

	// Enter should open detail-only view (bd-1of: Enter=detail, Tab=fold)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if m.focused != focusDetail {
		t.Fatalf("expected Enter to open detail view in narrow window, got focus %v", m.focused)
	}

	// Esc should return to tree
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	if m.focused != focusTree {
		t.Fatalf("expected Esc to return to tree, got focus %v", m.focused)
	}
}

// TestResizeNarrowToWideStaysManual verifies that resizing from narrow to wide
// does NOT auto-show the detail panel (bd-6eg).
func TestResizeNarrowToWideStaysManual(t *testing.T) {
	issues := []model.Issue{
		{ID: "1", Title: "Test issue", Status: model.StatusOpen},
	}
	m := NewModel(issues, "")

	// Start narrow
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = updated.(Model)

	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden=true in narrow window")
	}

	// Resize to wide
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = updated.(Model)

	// Should stay hidden - user must press d to restore
	if !m.treeDetailHidden {
		t.Fatal("expected treeDetailHidden to stay true after resize to wide (manual mode)")
	}
}
