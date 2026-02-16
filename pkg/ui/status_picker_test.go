package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestNewStatusPickerModel(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("in_progress", theme)

	// Should have all statuses except tombstone
	expectedCount := 8 // open, in_progress, blocked, deferred, pinned, hooked, review, closed
	if len(picker.statuses) != expectedCount {
		t.Errorf("Expected %d statuses, got %d", expectedCount, len(picker.statuses))
	}

	// Should select the current status
	if picker.SelectedStatus() != "in_progress" {
		t.Errorf("Expected current status 'in_progress' to be selected, got %q", picker.SelectedStatus())
	}

	// Should store current status
	if picker.currentStatus != model.StatusInProgress {
		t.Errorf("Expected currentStatus to be in_progress, got %q", picker.currentStatus)
	}
}

func TestNewStatusPickerModelUnknownStatus(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("unknown_status", theme)

	// Should default to first status (open)
	if picker.selectedIndex != 0 {
		t.Errorf("Expected selectedIndex 0 for unknown status, got %d", picker.selectedIndex)
	}

	if picker.SelectedStatus() != "open" {
		t.Errorf("Expected default to 'open', got %q", picker.SelectedStatus())
	}
}

func TestStatusPickerNavigation(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("open", theme)

	// Start at index 0 (open)
	if picker.selectedIndex != 0 {
		t.Fatalf("Expected initial index 0, got %d", picker.selectedIndex)
	}

	// Move down
	picker.MoveDown()
	if picker.selectedIndex != 1 {
		t.Errorf("After MoveDown, expected index 1, got %d", picker.selectedIndex)
	}
	if picker.SelectedStatus() != "in_progress" {
		t.Errorf("After MoveDown, expected 'in_progress', got %q", picker.SelectedStatus())
	}

	// Move up
	picker.MoveUp()
	if picker.selectedIndex != 0 {
		t.Errorf("After MoveUp, expected index 0, got %d", picker.selectedIndex)
	}
	if picker.SelectedStatus() != "open" {
		t.Errorf("After MoveUp, expected 'open', got %q", picker.SelectedStatus())
	}

	// Try to move up past start (should stay at 0)
	picker.MoveUp()
	if picker.selectedIndex != 0 {
		t.Errorf("MoveUp at start should stay at 0, got %d", picker.selectedIndex)
	}

	// Move to end
	for i := 0; i < 10; i++ {
		picker.MoveDown()
	}
	expectedLast := len(picker.statuses) - 1
	if picker.selectedIndex != expectedLast {
		t.Errorf("After many MoveDown, expected index %d, got %d", expectedLast, picker.selectedIndex)
	}

	// Try to move down past end (should stay at last)
	picker.MoveDown()
	if picker.selectedIndex != expectedLast {
		t.Errorf("MoveDown at end should stay at %d, got %d", expectedLast, picker.selectedIndex)
	}
}

func TestStatusPickerSetSize(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("open", theme)

	picker.SetSize(100, 50)
	if picker.width != 100 || picker.height != 50 {
		t.Errorf("SetSize(100, 50) failed: got width=%d height=%d", picker.width, picker.height)
	}
}

func TestStatusPickerView(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("blocked", theme)
	picker.SetSize(80, 40)

	output := picker.View()

	// Check for key elements in output
	mustContain := []string{
		"Change Status",          // Title
		"Open",                   // First status
		"Closed",                 // Last status
		"Blocked",                // Current status
		"✓",                      // Checkmark for current
		"j/k: navigate",          // Footer
		"enter: apply",           // Footer
		"esc: cancel",            // Footer
		"> ",                     // Selection marker
	}

	for _, expected := range mustContain {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected View() to contain %q, but it didn't", expected)
		}
	}
}

func TestStatusPickerViewCurrentStatusHighlighted(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("review", theme)
	picker.SetSize(80, 40)

	output := picker.View()

	// Should contain checkmark next to review status
	lines := strings.Split(output, "\n")
	foundReviewWithCheck := false
	for _, line := range lines {
		if strings.Contains(line, "Review") && strings.Contains(line, "✓") {
			foundReviewWithCheck = true
			break
		}
	}

	if !foundReviewWithCheck {
		t.Errorf("Expected 'review' status to be marked with checkmark in View()")
	}
}

func TestFormatStatusName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"open", "Open"},
		{"in_progress", "In Progress"},
		{"blocked", "Blocked"},
		{"deferred", "Deferred"},
	}

	for _, tt := range tests {
		got := formatStatusName(tt.input)
		if got != tt.expected {
			t.Errorf("formatStatusName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStatusPickerAllValidStatuses(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	picker := NewStatusPickerModel("open", theme)

	// Check all expected statuses are present
	expectedStatuses := []model.Status{
		model.StatusOpen,
		model.StatusInProgress,
		model.StatusBlocked,
		model.StatusDeferred,
		model.StatusPinned,
		model.StatusHooked,
		model.StatusReview,
		model.StatusClosed,
	}

	if len(picker.statuses) != len(expectedStatuses) {
		t.Fatalf("Expected %d statuses, got %d", len(expectedStatuses), len(picker.statuses))
	}

	for i, expected := range expectedStatuses {
		if picker.statuses[i] != expected {
			t.Errorf("Status at index %d: expected %q, got %q", i, expected, picker.statuses[i])
		}
	}

	// Ensure tombstone is NOT included
	for _, status := range picker.statuses {
		if status == model.StatusTombstone {
			t.Errorf("StatusTombstone should not be included in picker")
		}
	}
}
