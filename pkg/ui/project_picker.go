package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// ProjectEntry holds display data for one project in the picker.
type ProjectEntry struct {
	Project     config.Project
	FavoriteNum int  // 0 = not favorited, 1-9 = key
	IsActive    bool // Currently loaded project
	OpenCount   int
	ReadyCount  int
}

// SwitchProjectMsg is sent when the user selects a project to switch to.
type SwitchProjectMsg struct {
	Project config.Project
}

// ToggleFavoriteMsg is sent when the user toggles a project's favorite slot.
type ToggleFavoriteMsg struct {
	ProjectName string
	SlotNumber  int // 0 = remove, 1-9 = assign
}

// ProjectPickerModel is the overlay for selecting and switching between projects.
type ProjectPickerModel struct {
	entries     []ProjectEntry
	filtered    []int // indices into entries
	cursor      int
	width       int
	height      int
	filterInput textinput.Model
	filtering   bool
	theme       Theme
}

// NewProjectPicker creates a new project picker overlay.
func NewProjectPicker(entries []ProjectEntry, theme Theme) ProjectPickerModel {
	ti := textinput.New()
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 50
	ti.Width = 30

	indices := make([]int, len(entries))
	for i := range entries {
		indices[i] = i
	}

	return ProjectPickerModel{
		entries:     entries,
		filtered:    indices,
		cursor:      0,
		filterInput: ti,
		filtering:   false,
		theme:       theme,
	}
}

// SetSize updates the picker dimensions.
func (m *ProjectPickerModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles keyboard input for the project picker.
func (m ProjectPickerModel) Update(msg tea.Msg) (ProjectPickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// In filter mode, most keys go to the text input
		if m.filtering {
			return m.updateFiltering(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

// updateNormal handles keys when not in filter mode.
func (m ProjectPickerModel) updateNormal(msg tea.KeyMsg) (ProjectPickerModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "enter":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			entry := m.entries[m.filtered[m.cursor]]
			return m, func() tea.Msg {
				return SwitchProjectMsg{Project: entry.Project}
			}
		}
	case "u":
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			entry := m.entries[m.filtered[m.cursor]]
			newSlot := m.nextAvailableFavoriteSlot(entry)
			return m, func() tea.Msg {
				return ToggleFavoriteMsg{
					ProjectName: entry.Project.Name,
					SlotNumber:  newSlot,
				}
			}
		}
	case "/":
		m.filtering = true
		m.filterInput.SetValue("")
		m.filterInput.Focus()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		n := int(msg.String()[0] - '0')
		for _, entry := range m.entries {
			if entry.FavoriteNum == n {
				return m, func() tea.Msg {
					return SwitchProjectMsg{Project: entry.Project}
				}
			}
		}
	case "esc":
		// Close signal: the parent model handles this by checking for esc
		// and toggling the showProjectPicker flag. Return nil cmd.
		return m, nil
	}
	return m, nil
}

// updateFiltering handles keys when in filter mode.
func (m ProjectPickerModel) updateFiltering(msg tea.KeyMsg) (ProjectPickerModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.filtering = false
		m.filterInput.SetValue("")
		m.filterInput.Blur()
		m.applyFilter()
		return m, nil
	case "enter":
		m.filtering = false
		m.filterInput.Blur()
		// Keep the filter applied, select the highlighted entry
		if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
			entry := m.entries[m.filtered[m.cursor]]
			return m, func() tea.Msg {
				return SwitchProjectMsg{Project: entry.Project}
			}
		}
		return m, nil
	case "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down":
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.applyFilter()
		return m, cmd
	}
}

// applyFilter updates the filtered indices based on the current filter input.
func (m *ProjectPickerModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.filtered = make([]int, len(m.entries))
		for i := range m.entries {
			m.filtered[i] = i
		}
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		return
	}

	type scored struct {
		index int
		score int
	}
	var matches []scored
	for i, entry := range m.entries {
		name := strings.ToLower(entry.Project.Name)
		path := strings.ToLower(entry.Project.Path)
		// Check name and path for fuzzy match
		nameScore := fuzzyScore(name, query)
		pathScore := fuzzyScore(path, query)
		best := nameScore
		if pathScore > best {
			best = pathScore
		}
		if best > 0 {
			matches = append(matches, scored{i, best})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	m.filtered = make([]int, len(matches))
	for i, match := range matches {
		m.filtered[i] = match.index
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// nextAvailableFavoriteSlot cycles through favorite slots for the given entry.
// If already favorited, removes the favorite (returns 0).
// If not favorited, assigns the next available slot 1-9.
func (m *ProjectPickerModel) nextAvailableFavoriteSlot(entry ProjectEntry) int {
	if entry.FavoriteNum > 0 {
		// Already favorited: remove it
		return 0
	}
	// Find the first available slot
	used := make(map[int]bool)
	for _, e := range m.entries {
		if e.FavoriteNum > 0 {
			used[e.FavoriteNum] = true
		}
	}
	for n := 1; n <= 9; n++ {
		if !used[n] {
			return n
		}
	}
	return 0 // All slots taken
}

// View renders the project picker overlay.
func (m *ProjectPickerModel) View() string {
	if m.width == 0 {
		m.width = 70
	}
	if m.height == 0 {
		m.height = 20
	}

	t := m.theme

	// Calculate box dimensions
	boxWidth := 64
	if m.width < 74 {
		boxWidth = m.width - 10
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	maxVisible := 12
	if m.height < 18 {
		maxVisible = m.height - 8
	}
	if maxVisible < 3 {
		maxVisible = 3
	}

	var lines []string

	// Title
	titleStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	lines = append(lines, titleStyle.Render("Select Project"))
	lines = append(lines, "")

	// Filter input (shown when filtering)
	if m.filtering {
		inputStyle := t.Renderer.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(t.Secondary).
			Padding(0, 1).
			Width(boxWidth - 6)
		lines = append(lines, inputStyle.Render(m.filterInput.View()))
		lines = append(lines, "")
	}

	// Column header
	headerLine := m.renderHeaderLine(boxWidth)
	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	lines = append(lines, headerStyle.Render(headerLine))

	// Separator
	sepStyle := t.Renderer.NewStyle().Foreground(t.Border)
	lines = append(lines, sepStyle.Render(strings.Repeat("\u2500", boxWidth-4)))

	// Project list with scroll
	if len(m.filtered) == 0 {
		dimStyle := t.Renderer.NewStyle().
			Foreground(t.Secondary).
			Italic(true)
		lines = append(lines, dimStyle.Render("  No matching projects"))
	} else {
		// Calculate visible window
		start := 0
		if m.cursor >= maxVisible {
			start = m.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := start; i < end; i++ {
			entry := m.entries[m.filtered[i]]
			isCursor := i == m.cursor
			lines = append(lines, m.renderProjectLine(entry, isCursor, boxWidth))
		}

		// Show scroll indicator
		if len(m.filtered) > maxVisible {
			countStyle := t.Renderer.NewStyle().
				Foreground(t.Secondary).
				Italic(true)
			lines = append(lines, "")
			lines = append(lines, countStyle.Render(
				fmt.Sprintf("  %d/%d projects", m.cursor+1, len(m.filtered)),
			))
		}
	}

	// Footer
	lines = append(lines, "")
	footerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	lines = append(lines, footerStyle.Render("* = active | u:favorite | enter:switch | /:search | esc"))

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

// renderHeaderLine renders the column header row.
func (m *ProjectPickerModel) renderHeaderLine(boxWidth int) string {
	// Layout: " #  Project          Path                    Open  Ready"
	nameWidth, pathWidth := m.columnWidths(boxWidth)
	return fmt.Sprintf(" %-2s %-*s  %-*s  %5s %5s",
		"#", nameWidth, "Project", pathWidth, "Path", "Open", "Ready")
}

// renderProjectLine renders a single project entry row.
func (m *ProjectPickerModel) renderProjectLine(entry ProjectEntry, isCursor bool, boxWidth int) string {
	t := m.theme
	nameWidth, pathWidth := m.columnWidths(boxWidth)

	// Favorite number column
	favStr := "-"
	if entry.FavoriteNum > 0 {
		favStr = fmt.Sprintf("%d", entry.FavoriteNum)
	}

	// Project name with active marker
	name := entry.Project.Name
	if entry.IsActive {
		name += " *"
	}
	name = truncateRunesHelper(name, nameWidth, "...")

	// Path (abbreviated with ~)
	path := abbreviatePath(entry.Project.Path)
	path = truncateRunesHelper(path, pathWidth, "...")

	// Counts
	openStr := fmt.Sprintf("%d", entry.OpenCount)
	readyStr := fmt.Sprintf("%d", entry.ReadyCount)

	line := fmt.Sprintf(" %-2s %-*s  %-*s  %5s %5s",
		favStr, nameWidth, name, pathWidth, path, openStr, readyStr)

	style := t.Renderer.NewStyle()
	if isCursor {
		style = style.Foreground(t.Primary).Bold(true)
		line = ">" + line[1:] // Replace leading space with cursor
	} else {
		style = style.Foreground(t.Base.GetForeground())
	}

	return style.Render(line)
}

// columnWidths calculates name and path column widths based on box width.
// Layout: " #  Name  Path  Open  Ready" with padding and fixed-width count columns.
func (m *ProjectPickerModel) columnWidths(boxWidth int) (nameWidth, pathWidth int) {
	// Fixed columns: " # " (4) + "  " (2 gap) + "  " (2 gap) + " Open" (6) + " Ready" (6) = ~20 fixed
	available := boxWidth - 24
	if available < 20 {
		available = 20
	}
	// Split roughly 40/60 between name and path
	nameWidth = available * 2 / 5
	pathWidth = available - nameWidth
	if nameWidth < 8 {
		nameWidth = 8
	}
	if pathWidth < 10 {
		pathWidth = 10
	}
	return nameWidth, pathWidth
}

// abbreviatePath replaces the user's home directory with ~ in a path.
func abbreviatePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// Filtering returns whether the picker is in filter mode.
func (m *ProjectPickerModel) Filtering() bool {
	return m.filtering
}

// Cursor returns the current cursor position.
func (m *ProjectPickerModel) Cursor() int {
	return m.cursor
}

// FilteredCount returns the number of entries matching the current filter.
func (m *ProjectPickerModel) FilteredCount() int {
	return len(m.filtered)
}

// SelectedEntry returns the currently highlighted project entry, or nil if none.
func (m *ProjectPickerModel) SelectedEntry() *ProjectEntry {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	entry := m.entries[m.filtered[m.cursor]]
	return &entry
}
