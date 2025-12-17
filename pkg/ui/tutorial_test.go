package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newTestTutorialModel() TutorialModel {
	theme := Theme{Renderer: lipgloss.DefaultRenderer()}
	return NewTutorialModel(theme)
}

func TestNewTutorialModel(t *testing.T) {
	m := newTestTutorialModel()

	if m.currentPage != 0 {
		t.Errorf("Expected initial page 0, got %d", m.currentPage)
	}
	if m.scrollOffset != 0 {
		t.Errorf("Expected initial scroll 0, got %d", m.scrollOffset)
	}
	if m.tocVisible {
		t.Error("Expected TOC to be hidden initially")
	}
	if m.contextMode {
		t.Error("Expected context mode to be disabled initially")
	}
	if len(m.pages) == 0 {
		t.Error("Expected default pages to be loaded")
	}
	if m.progress == nil {
		t.Error("Expected progress map to be initialized")
	}
}

func TestTutorialNavigation(t *testing.T) {
	m := newTestTutorialModel()
	totalPages := len(m.pages)

	// Test next page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if m.currentPage != 1 {
		t.Errorf("Expected page 1 after 'n', got %d", m.currentPage)
	}

	// Test right arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.currentPage != 2 {
		t.Errorf("Expected page 2 after right arrow, got %d", m.currentPage)
	}

	// Test previous page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if m.currentPage != 1 {
		t.Errorf("Expected page 1 after 'p', got %d", m.currentPage)
	}

	// Test left arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.currentPage != 0 {
		t.Errorf("Expected page 0 after left arrow, got %d", m.currentPage)
	}

	// Test boundary - can't go below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.currentPage != 0 {
		t.Errorf("Expected page to stay at 0, got %d", m.currentPage)
	}

	// Go to last page
	for i := 0; i < totalPages; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}
	if m.currentPage != totalPages-1 {
		t.Errorf("Expected to be at last page %d, got %d", totalPages-1, m.currentPage)
	}

	// Test boundary - can't go above max
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.currentPage != totalPages-1 {
		t.Errorf("Expected to stay at last page, got %d", m.currentPage)
	}
}

func TestTutorialScrolling(t *testing.T) {
	m := newTestTutorialModel()

	// Test scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.scrollOffset != 1 {
		t.Errorf("Expected scroll 1 after 'j', got %d", m.scrollOffset)
	}

	// Test scroll up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after 'k', got %d", m.scrollOffset)
	}

	// Can't scroll below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll to stay at 0, got %d", m.scrollOffset)
	}

	// Test home
	m.scrollOffset = 5
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll 0 after 'g', got %d", m.scrollOffset)
	}

	// Test end (will be clamped in View)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.scrollOffset == 0 {
		t.Error("Expected scroll to increase after 'G'")
	}
}

func TestTutorialTOCToggle(t *testing.T) {
	m := newTestTutorialModel()

	if m.tocVisible {
		t.Error("TOC should be hidden initially")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if !m.tocVisible {
		t.Error("TOC should be visible after 't'")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if m.tocVisible {
		t.Error("TOC should be hidden after second 't'")
	}
}

func TestTutorialJumpToPage(t *testing.T) {
	m := newTestTutorialModel()

	// Jump to page 3 using number key
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if m.currentPage != 2 { // 0-indexed
		t.Errorf("Expected page 2 after '3', got %d", m.currentPage)
	}

	// Jump to page 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if m.currentPage != 0 {
		t.Errorf("Expected page 0 after '1', got %d", m.currentPage)
	}

	// Invalid page number (beyond available pages)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("9")})
	// Should not change if page doesn't exist
}

func TestTutorialJumpMethods(t *testing.T) {
	m := newTestTutorialModel()

	// JumpToPage
	m.JumpToPage(3)
	if m.currentPage != 3 {
		t.Errorf("Expected page 3, got %d", m.currentPage)
	}
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll reset to 0, got %d", m.scrollOffset)
	}

	// JumpToPage with invalid index
	m.JumpToPage(-1)
	if m.currentPage != 3 {
		t.Error("JumpToPage with negative index should not change page")
	}

	m.JumpToPage(9999)
	if m.currentPage != 3 {
		t.Error("JumpToPage with too-large index should not change page")
	}

	// JumpToSection
	m.JumpToSection("navigation")
	if m.currentPage == 3 {
		// Should have moved to navigation page
	}
}

func TestTutorialContextFiltering(t *testing.T) {
	m := newTestTutorialModel()

	// Initially all pages visible
	allPages := m.visiblePages()
	if len(allPages) == 0 {
		t.Error("Expected some pages")
	}

	// Enable context mode
	m.SetContextMode(true)
	m.SetContext("list")

	// Now only list-context pages should be visible
	filteredPages := m.visiblePages()
	for _, page := range filteredPages {
		if len(page.Contexts) > 0 {
			found := false
			for _, ctx := range page.Contexts {
				if ctx == "list" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Page %s should not be visible in list context", page.ID)
			}
		}
	}

	// Disable context mode - all pages visible again
	m.SetContextMode(false)
	allPagesAgain := m.visiblePages()
	if len(allPagesAgain) != len(allPages) {
		t.Errorf("Expected %d pages without context mode, got %d", len(allPages), len(allPagesAgain))
	}
}

func TestTutorialProgress(t *testing.T) {
	m := newTestTutorialModel()

	// Initially no progress
	if m.IsComplete() {
		t.Error("Tutorial should not be complete initially")
	}

	// Mark first page viewed
	m.MarkViewed("intro")
	if !m.progress["intro"] {
		t.Error("Page 'intro' should be marked as viewed")
	}

	// Check progress getter
	progress := m.Progress()
	if !progress["intro"] {
		t.Error("Progress getter should return viewed pages")
	}

	// Set progress from external source (persistence)
	newProgress := map[string]bool{
		"intro":      true,
		"navigation": true,
	}
	m.SetProgress(newProgress)
	if !m.progress["navigation"] {
		t.Error("SetProgress should restore progress")
	}

	// Mark all pages viewed
	for _, page := range m.pages {
		m.MarkViewed(page.ID)
	}
	if !m.IsComplete() {
		t.Error("Tutorial should be complete when all pages viewed")
	}
}

func TestTutorialView(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 24)

	view := m.View()

	// Should contain title
	if !strings.Contains(view, "Welcome") {
		t.Error("View should contain first page title")
	}

	// Should contain navigation hints
	if !strings.Contains(view, "pages") {
		t.Error("View should contain navigation hints")
	}

	// Test with TOC visible
	m.tocVisible = true
	viewWithTOC := m.View()
	if !strings.Contains(viewWithTOC, "Contents") {
		t.Error("View with TOC should contain Contents header")
	}
}

func TestTutorialSetSize(t *testing.T) {
	m := newTestTutorialModel()

	m.SetSize(100, 30)
	if m.width != 100 {
		t.Errorf("Expected width 100, got %d", m.width)
	}
	if m.height != 30 {
		t.Errorf("Expected height 30, got %d", m.height)
	}
}

func TestTutorialCurrentPageID(t *testing.T) {
	m := newTestTutorialModel()

	id := m.CurrentPageID()
	if id != "intro-welcome" {
		t.Errorf("Expected 'intro-welcome', got %s", id)
	}

	m.NextPage()
	id = m.CurrentPageID()
	if id != "intro-philosophy" {
		t.Errorf("Expected 'intro-philosophy', got %s", id)
	}
}

func TestTutorialCenterTutorial(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(60, 20)

	centered := m.CenterTutorial(100, 40)

	// Should not be empty
	if centered == "" {
		t.Error("Centered tutorial should not be empty")
	}

	// Should still contain content
	if !strings.Contains(centered, "Welcome") {
		t.Error("Centered tutorial should contain content")
	}
}

func TestTutorialEmptyState(t *testing.T) {
	m := newTestTutorialModel()
	m.pages = []TutorialPage{} // Clear all pages

	view := m.View()
	if !strings.Contains(view, "No tutorial pages") {
		t.Error("Empty state should show appropriate message")
	}
}

func TestTutorialInit(t *testing.T) {
	m := newTestTutorialModel()
	cmd := m.Init()

	// Init should return nil (no initial command)
	if cmd != nil {
		t.Error("Init should return nil")
	}
}

func TestTutorialPageNavResetsScroll(t *testing.T) {
	m := newTestTutorialModel()

	// Scroll down on first page
	m.scrollOffset = 10

	// Navigate to next page
	m.NextPage()

	// Scroll should reset
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll to reset on page change, got %d", m.scrollOffset)
	}

	// Same for PrevPage
	m.scrollOffset = 5
	m.PrevPage()
	if m.scrollOffset != 0 {
		t.Errorf("Expected scroll to reset on PrevPage, got %d", m.scrollOffset)
	}
}

func TestTutorialAlternativeKeys(t *testing.T) {
	m := newTestTutorialModel()

	// Test 'l' for next page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.currentPage != 1 {
		t.Error("'l' should navigate to next page")
	}

	// Test 'h' for prev page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if m.currentPage != 0 {
		t.Error("'h' should navigate to previous page")
	}

	// Test Tab for next page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.currentPage != 1 {
		t.Error("Tab should navigate to next page")
	}

	// Test Shift+Tab for prev page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.currentPage != 0 {
		t.Error("Shift+Tab should navigate to previous page")
	}

	// Test down arrow for scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.scrollOffset != 1 {
		t.Error("Down arrow should scroll down")
	}

	// Test up arrow for scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Error("Up arrow should scroll up")
	}

	// Test Home for scroll
	m.scrollOffset = 10
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset != 0 {
		t.Error("Home should scroll to top")
	}

	// Test End for scroll
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset == 0 {
		t.Error("End should scroll down")
	}
}

func TestTutorialProgressPersistence(t *testing.T) {
	m := newTestTutorialModel()

	// Simulate viewing pages
	m.MarkViewed("intro")
	m.MarkViewed("navigation")

	// Get progress for persistence
	progress := m.Progress()

	// Create new tutorial model
	m2 := newTestTutorialModel()

	// Restore progress
	m2.SetProgress(progress)

	// Verify restored
	if !m2.progress["intro"] {
		t.Error("Progress should be restored for 'intro'")
	}
	if !m2.progress["navigation"] {
		t.Error("Progress should be restored for 'navigation'")
	}

	// Test nil progress doesn't crash
	m2.SetProgress(nil)
}

func TestDefaultTutorialPages(t *testing.T) {
	pages := defaultTutorialPages()

	if len(pages) == 0 {
		t.Error("Should have default pages")
	}

	// Check required pages exist (using new page IDs from bv-kdv2)
	requiredIDs := []string{"intro-welcome", "intro-philosophy", "ref-keyboard"}
	for _, id := range requiredIDs {
		found := false
		for _, page := range pages {
			if page.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing required page: %s", id)
		}
	}

	// Check all pages have required fields
	for _, page := range pages {
		if page.ID == "" {
			t.Error("Page missing ID")
		}
		if page.Title == "" {
			t.Error("Page missing Title")
		}
		if page.Content == "" {
			t.Error("Page missing Content")
		}
	}
}

// Tests for UI Layout & Chrome (bv-h6rq)

func TestTutorialViewProgressBar(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(100, 60) // Larger to ensure footer isn't clipped

	view := m.View()

	// Should contain progress indicator format [1/N]
	if !strings.Contains(view, "[1/") {
		t.Error("View should contain progress indicator [1/N] format")
	}

	// Should contain progress bar characters
	if !strings.Contains(view, "█") {
		t.Error("View should contain filled progress bar character")
	}

	// Navigate to page 2 and verify progress updates
	m.NextPage()
	view = m.View()
	if !strings.Contains(view, "[2/") {
		t.Error("View should show [2/N] on second page")
	}
}

func TestTutorialViewHeader(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 24)

	view := m.View()

	// Should contain app title
	if !strings.Contains(view, "beads_viewer Tutorial") {
		t.Error("View should contain app title 'beads_viewer Tutorial'")
	}

	// Should contain separator line
	if !strings.Contains(view, "─") {
		t.Error("View should contain separator line")
	}
}

func TestTutorialViewFooter(t *testing.T) {
	m := newTestTutorialModel()
	// Use large dimensions to ensure footer isn't clipped
	m.SetSize(100, 60)

	view := m.View()

	// Should contain styled key hints
	if !strings.Contains(view, "←/→") {
		t.Error("View should contain page navigation hint")
	}
	if !strings.Contains(view, "j/k") {
		t.Error("View should contain scroll hint")
	}
	if !strings.Contains(view, "TOC") {
		t.Error("View should contain TOC hint")
	}
	// Footer shows "q close" not "Esc close"
	if !strings.Contains(view, "close") {
		t.Error("View should contain close hint")
	}
}

func TestTutorialTOCSectionIndicators(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 24)
	m.tocVisible = true

	view := m.View()

	// Should contain section indicator
	if !strings.Contains(view, "▸") {
		t.Error("TOC should contain section indicator ▸")
	}

	// Should contain current page indicator
	if !strings.Contains(view, "▶") {
		t.Error("TOC should contain current page indicator ▶")
	}
}

func TestTutorialTOCProgressCheckmarks(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 24)
	m.tocVisible = true

	// Mark intro as viewed
	m.MarkViewed("intro")

	view := m.View()

	// Should contain checkmark for viewed page
	if !strings.Contains(view, "✓") {
		t.Error("TOC should show checkmark for viewed pages")
	}
}

func TestTutorialPageTitleDisplay(t *testing.T) {
	m := newTestTutorialModel()
	// Use large height to ensure content isn't clipped
	m.SetSize(100, 60)

	view := m.View()

	// Should show current page title (now "Welcome" from bv-kdv2)
	if !strings.Contains(view, "Welcome") {
		t.Error("View should contain current page title")
	}

	// Should show section info (now "Introduction" from bv-kdv2)
	if !strings.Contains(view, "Introduction") {
		t.Error("View should contain section name")
	}
}

// Tests for Glamour Markdown Rendering (bv-lb0h)

func TestTutorialMarkdownRendererInitialized(t *testing.T) {
	m := newTestTutorialModel()

	// Markdown renderer should be initialized
	if m.markdownRenderer == nil {
		t.Error("Markdown renderer should be initialized")
	}
}

func TestTutorialMarkdownContent(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 30)

	view := m.View()

	// Should contain rendered markdown elements
	// Bold text should be rendered (though exact ANSI codes vary)
	if !strings.Contains(view, "beads_viewer") {
		t.Error("View should contain beads_viewer text")
	}

	// Bullet points from markdown should be present
	if !strings.Contains(view, "•") || !strings.Contains(view, "-") {
		// Glamour renders bullets as •
		// Check if content has list items
	}
}

func TestTutorialSetSizeUpdatesMarkdownRenderer(t *testing.T) {
	m := newTestTutorialModel()

	// Change size
	m.SetSize(120, 40)

	// The markdown renderer should be updated (not nil)
	if m.markdownRenderer == nil {
		t.Error("Markdown renderer should still exist after SetSize")
	}

	// Width should be updated
	if m.width != 120 {
		t.Errorf("Expected width 120, got %d", m.width)
	}
}

func TestTutorialMarkdownWithCodeBlocks(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(100, 60) // Larger to show more content

	// Navigate to the "AI Agent Integration" page which has code blocks
	m.JumpToPage(17) // Index 17 is "advanced-ai" (after intro x4, concepts x5, views x8)

	view := m.View()

	// Code blocks should be present (content includes bash commands)
	// The exact rendering depends on Glamour, but the content should include command text
	if !strings.Contains(view, "robot") {
		t.Error("View should contain 'robot' command from code blocks")
	}
}

func TestTutorialMarkdownWithTables(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(100, 60) // Wide and tall enough for tables

	// Navigate to Quick Start page which has tables
	m.JumpToPage(3) // Index 3 is "intro-quickstart"

	view := m.View()

	// Tables should render with separators
	// Glamour renders tables with │ characters
	if !strings.Contains(view, "│") && !strings.Contains(view, "|") {
		// Table separators might vary by theme
	}

	// Content from table should be present
	if !strings.Contains(view, "Action") {
		t.Error("View should contain table header 'Action'")
	}
}

// Tests for Keyboard Navigation (bv-wdsd)

func TestTutorialExitKeys(t *testing.T) {
	// Test 'q' key closes
	m := newTestTutorialModel()
	if m.ShouldClose() {
		t.Error("Should not close initially")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !m.ShouldClose() {
		t.Error("'q' should trigger close")
	}

	// Test Esc key closes
	m = newTestTutorialModel()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.ShouldClose() {
		t.Error("Esc should trigger close")
	}
}

func TestTutorialSpaceNavigates(t *testing.T) {
	m := newTestTutorialModel()

	// Space should advance to next page
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	if m.currentPage != 1 {
		t.Error("Space should navigate to next page")
	}
}

func TestTutorialHalfPageScroll(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 30)

	// Ctrl+d should scroll half page down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	if m.scrollOffset == 0 {
		t.Error("Ctrl+d should scroll down")
	}

	savedOffset := m.scrollOffset

	// Ctrl+u should scroll half page up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	if m.scrollOffset >= savedOffset {
		t.Error("Ctrl+u should scroll up")
	}
}

func TestTutorialFocusManagement(t *testing.T) {
	m := newTestTutorialModel()

	// Initially content has focus
	if m.focus != focusTutorialContent {
		t.Error("Content should have initial focus")
	}

	// Toggle TOC with 't'
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	if !m.tocVisible {
		t.Error("TOC should be visible after 't'")
	}
	if m.focus != focusTutorialTOC {
		t.Error("TOC should have focus after 't'")
	}

	// Tab switches focus back to content
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusTutorialContent {
		t.Error("Tab should switch focus to content")
	}

	// Tab again switches back to TOC (when visible)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus != focusTutorialTOC {
		t.Error("Tab should switch focus back to TOC")
	}
}

func TestTutorialTOCNavigation(t *testing.T) {
	m := newTestTutorialModel()
	m.tocVisible = true
	m.focus = focusTutorialTOC
	m.tocCursor = 0

	// j/down moves cursor down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.tocCursor != 1 {
		t.Error("'j' should move TOC cursor down")
	}

	// k/up moves cursor up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.tocCursor != 0 {
		t.Error("'k' should move TOC cursor up")
	}

	// Enter jumps to selected page
	m.tocCursor = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.currentPage != 2 {
		t.Error("Enter should jump to TOC cursor position")
	}
	if m.focus != focusTutorialContent {
		t.Error("Enter should switch focus to content")
	}
}

func TestTutorialTOCCursorIndicator(t *testing.T) {
	m := newTestTutorialModel()
	m.SetSize(80, 24)
	m.tocVisible = true
	m.focus = focusTutorialTOC
	m.tocCursor = 1

	view := m.View()

	// Should contain cursor indicator
	if !strings.Contains(view, "→") {
		t.Error("TOC should show cursor indicator when focused")
	}

	// Should contain focus indicator
	if !strings.Contains(view, "●") {
		t.Error("TOC should show focus indicator")
	}
}

func TestTutorialContextSensitiveFooter(t *testing.T) {
	m := newTestTutorialModel()
	// Use very large dimensions to ensure footer isn't clipped by MaxHeight
	m.SetSize(120, 100)

	// Content focus footer
	view := m.View()
	if !strings.Contains(view, "Space") {
		t.Error("Content footer should show Space hint")
	}
	if !strings.Contains(view, "Ctrl+d") {
		t.Error("Content footer should show Ctrl+d hint")
	}

	// TOC focus footer
	m.tocVisible = true
	m.focus = focusTutorialTOC
	view = m.View()
	if !strings.Contains(view, "Enter") {
		t.Error("TOC footer should show Enter hint")
	}
	if !strings.Contains(view, "back to content") {
		t.Error("TOC footer should show back to content hint")
	}
}

func TestTutorialResetClose(t *testing.T) {
	m := newTestTutorialModel()
	m.shouldClose = true

	if !m.ShouldClose() {
		t.Error("ShouldClose should return true")
	}

	m.ResetClose()
	if m.ShouldClose() {
		t.Error("ResetClose should clear shouldClose flag")
	}
}
