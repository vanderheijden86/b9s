package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// StatusPickerModel provides a quick status selection modal
type StatusPickerModel struct {
	statuses      []model.Status // All valid statuses
	currentStatus model.Status   // Currently selected issue's status
	selectedIndex int            // Which status is highlighted
	width         int
	height        int
	theme         Theme
}

// NewStatusPickerModel creates a new status picker
func NewStatusPickerModel(currentStatus string, theme Theme) StatusPickerModel {
	// All valid statuses (excluding tombstone which is internal)
	statuses := []model.Status{
		model.StatusOpen,
		model.StatusInProgress,
		model.StatusBlocked,
		model.StatusDeferred,
		model.StatusPinned,
		model.StatusHooked,
		model.StatusReview,
		model.StatusClosed,
	}

	// Find index of current status
	current := model.Status(currentStatus)
	selectedIdx := 0
	for i, s := range statuses {
		if s == current {
			selectedIdx = i
			break
		}
	}

	return StatusPickerModel{
		statuses:      statuses,
		currentStatus: current,
		selectedIndex: selectedIdx,
		theme:         theme,
	}
}

// SetSize updates the picker dimensions
func (m *StatusPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// MoveUp moves selection up
func (m *StatusPickerModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down
func (m *StatusPickerModel) MoveDown() {
	if m.selectedIndex < len(m.statuses)-1 {
		m.selectedIndex++
	}
}

// SelectedStatus returns the currently selected status
func (m *StatusPickerModel) SelectedStatus() string {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.statuses) {
		return string(m.statuses[m.selectedIndex])
	}
	return ""
}

// View renders the status picker overlay
func (m *StatusPickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	// Calculate box dimensions
	boxWidth := 35
	if m.width < 45 {
		boxWidth = m.width - 10
	}
	if boxWidth < 25 {
		boxWidth = 25
	}

	var lines []string

	// Title
	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		MarginBottom(1)
	lines = append(lines, titleStyle.Render("Change Status"))
	lines = append(lines, "")

	// Status list
	for i, status := range m.statuses {
		isSelected := i == m.selectedIndex
		isCurrent := status == m.currentStatus

		itemStyle := t.Renderer.NewStyle()
		if isSelected {
			itemStyle = itemStyle.Foreground(t.Primary).Bold(true)
		} else {
			itemStyle = itemStyle.Foreground(t.Base.GetForeground())
		}

		prefix := "  "
		if isSelected {
			prefix = "> "
		}

		// Add checkmark for current status
		suffix := ""
		if isCurrent {
			checkStyle := t.Renderer.NewStyle().Foreground(t.Secondary)
			suffix = " " + checkStyle.Render("✓")
		}

		// Format status name (convert underscores to spaces, capitalize)
		displayName := formatStatusName(string(status))
		lines = append(lines, itemStyle.Render(prefix+displayName)+suffix)
	}

	// Footer with keybindings
	lines = append(lines, "")
	footerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	lines = append(lines, footerStyle.Render("j/k: navigate | enter: apply | esc: cancel"))

	content := strings.Join(lines, "\n")

	// Box style
	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(boxWidth)

	box := boxStyle.Render(content)

	// Center in viewport
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)
}

// formatStatusName converts a status string to a display name
// Example: "in_progress" -> "In Progress"
func formatStatusName(status string) string {
	parts := strings.Split(status, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
