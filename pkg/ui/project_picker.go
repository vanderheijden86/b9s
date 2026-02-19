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
	Project         config.Project
	FavoriteNum     int  // 0 = not favorited, 1-9 = key
	IsActive        bool // Currently loaded project
	OpenCount       int
	InProgressCount int
	ReadyCount      int
	BlockedCount    int
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

// ProjectPickerModel is an always-visible k9s-style header for selecting projects.
// It renders as a multi-column panel: stats | project table (# NAME O P R) | shortcuts | B9s logo.
// Project switching is done via number keys 1-9 or filter mode.
type ProjectPickerModel struct {
	entries     []ProjectEntry
	filtered    []int // indices into entries
	cursor      int   // only used during filter mode for selecting results
	width       int
	height      int
	filterInput textinput.Model
	filtering   bool
	theme       Theme
}

// panelRows is the fixed number of content rows in the picker panel.
// Matches the B9s logo height (6 lines). Title bar adds 1 more.
const panelRows = 6

// maxVisibleProjects is the max number of projects shown in the table.
// Row 0 = column headers, so 5 project rows fit in 6 panel rows.
const maxVisibleProjects = 5

// NewProjectPicker creates a new project picker.
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
		if m.filtering {
			return m.updateFiltering(msg)
		}
		return m.updateNormal(msg)
	}
	return m, nil
}

// updateNormal handles keys in display-only mode.
func (m ProjectPickerModel) updateNormal(msg tea.KeyMsg) (ProjectPickerModel, tea.Cmd) {
	switch msg.String() {
	case "/":
		m.filtering = true
		m.cursor = 0
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
func (m *ProjectPickerModel) nextAvailableFavoriteSlot(entry ProjectEntry) int {
	if entry.FavoriteNum > 0 {
		return 0
	}
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
	return 0
}

// b9sLogo returns the ASCII art logo lines.
func b9sLogo() []string {
	return []string{
		`__________  ________`,
		`\______   \/   __   \______`,
		` |    |  _/\____    /  ___/`,
		` |    |   \   /    /\___ \`,
		` |______  /  /____//____  >`,
		`        \/              \/`,
	}
}

// pickerShortcuts returns the shortcut definitions for the picker panel.
func pickerShortcuts() []struct{ key, desc string } {
	return []struct{ key, desc string }{
		{"</>", "Filter"},
		{"<e>", "Edit"},
		{"<b>", "Board"},
		{"<?>", "Help"},
		{"<s>", "Sort"},
		{"<;>", "Shortcuts"},
	}
}

// View renders the k9s-style multi-column project picker panel (bd-b4u).
// Layout: [stats] [project table with O P R columns] [shortcuts] [B9s logo]
// Bottom: title bar divider.
func (m *ProjectPickerModel) View() string {
	if m.width == 0 {
		m.width = 80
	}

	w := m.width
	t := m.theme

	// --- Build each column as []string of panelRows lines ---

	// Column 1: Stats
	statsLines := m.renderStatsColumn()

	// Column 2: Project table (# NAME ... O P R)
	tableLines := m.renderProjectTable()

	// Column 3: Shortcuts
	shortcutLines := m.renderShortcutsColumn()

	// Column 4: B9s logo
	logoLines := m.renderLogoColumn()

	// --- Determine column widths ---
	statsWidth := m.maxLineWidth(statsLines)
	if statsWidth < 28 {
		statsWidth = 28
	}
	shortcutsWidth := m.maxLineWidth(shortcutLines)
	if shortcutsWidth < 16 {
		shortcutsWidth = 16
	}
	logoWidth := m.maxLineWidth(logoLines)
	gap := 2 // gap between columns

	// Table gets remaining space
	tableWidth := w - statsWidth - shortcutsWidth - logoWidth - gap*3
	if tableWidth < 30 {
		tableWidth = 30
	}

	// --- Style each column with fixed width ---
	statsStyle := t.Renderer.NewStyle().Width(statsWidth)
	tableStyle := t.Renderer.NewStyle().Width(tableWidth)
	shortcutsStyle := t.Renderer.NewStyle().Width(shortcutsWidth)
	logoStyle := t.Renderer.NewStyle().Width(logoWidth)
	gapStr := strings.Repeat(" ", gap)

	// --- Join columns row by row ---
	var rows []string
	for i := 0; i < panelRows; i++ {
		row := statsStyle.Render(safeIndex(statsLines, i)) +
			gapStr +
			tableStyle.Render(safeIndex(tableLines, i)) +
			gapStr +
			shortcutsStyle.Render(safeIndex(shortcutLines, i)) +
			gapStr +
			logoStyle.Render(safeIndex(logoLines, i))
		rows = append(rows, row)
	}

	// --- Title bar at bottom ---
	rows = append(rows, m.renderTitleBar(w))

	return strings.Join(rows, "\n")
}

// Height returns the number of terminal lines the picker panel uses.
func (m *ProjectPickerModel) Height() int {
	return panelRows + 1 // content rows + title bar
}

// renderStatsColumn renders the left stats column.
// Lines: Project, Path, Open, In Prog, Ready, Blocked.
func (m *ProjectPickerModel) renderStatsColumn() []string {
	t := m.theme

	labelStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	valueStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	// Find active project
	var activeName, activePath string
	var activeOpen, activeProg, activeReady, activeBlocked int
	for _, entry := range m.entries {
		if entry.IsActive {
			activeName = entry.Project.Name
			activePath = abbreviatePath(entry.Project.Path)
			activeOpen = entry.OpenCount
			activeProg = entry.InProgressCount
			activeReady = entry.ReadyCount
			activeBlocked = entry.BlockedCount
			break
		}
	}
	if activeName == "" && len(m.entries) > 0 {
		activeName = m.entries[0].Project.Name
		activePath = abbreviatePath(m.entries[0].Project.Path)
	}

	// Filter mode: replace path line with filter input
	pathLine := labelStyle.Render(" Path:    ") + valueStyle.Render(activePath)
	if m.filtering {
		pathLine = labelStyle.Render(" Filter:  ") + t.Renderer.NewStyle().Foreground(t.Primary).Render(m.filterInput.View())
	}

	return []string{
		labelStyle.Render(" Project: ") + valueStyle.Render(activeName),
		pathLine,
		labelStyle.Render(" Open:    ") + valueStyle.Render(fmt.Sprintf("%d", activeOpen)),
		labelStyle.Render(" In Prog: ") + valueStyle.Render(fmt.Sprintf("%d", activeProg)),
		labelStyle.Render(" Ready:   ") + valueStyle.Render(fmt.Sprintf("%d", activeReady)),
		labelStyle.Render(" Blocked: ") + valueStyle.Render(fmt.Sprintf("%d", activeBlocked)),
	}
}

// renderProjectTable renders the project list with # NAME and O P R columns.
func (m *ProjectPickerModel) renderProjectTable() []string {
	t := m.theme

	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true)
	numStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	activeStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	normalStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())
	cursorStyle := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)
	dimStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)

	// Find max name width for alignment
	nameW := 12 // minimum
	for _, idx := range m.filtered {
		entry := m.entries[idx]
		if len(entry.Project.Name) > nameW {
			nameW = len(entry.Project.Name)
		}
	}
	if nameW > 20 {
		nameW = 20
	}

	lines := make([]string, panelRows)

	// Row 0: column headers (right-aligned O P R above the number columns)
	lines[0] = headerStyle.Render(fmt.Sprintf("     %-*s  %3s %3s %3s", nameW, "", "O", "P", "R"))

	if len(m.filtered) == 0 {
		lines[1] = dimStyle.Render(" No projects found")
		return lines
	}

	// Rows 1-5: project entries
	visible := len(m.filtered)
	if visible > maxVisibleProjects {
		visible = maxVisibleProjects
	}

	for i := 0; i < visible; i++ {
		entry := m.entries[m.filtered[i]]
		isCursor := m.filtering && i == m.cursor

		// Number
		numStr := " "
		if entry.FavoriteNum > 0 {
			numStr = fmt.Sprintf("%d", entry.FavoriteNum)
		}

		// Name (truncated if needed)
		name := entry.Project.Name
		if len(name) > nameW {
			name = name[:nameW-3] + "..."
		}

		// Build the row text with fixed-width columns
		rowText := fmt.Sprintf(" <%s> %-*s  %3d %3d %3d",
			numStr, nameW, name,
			entry.OpenCount, entry.InProgressCount, entry.ReadyCount)

		switch {
		case isCursor:
			lines[i+1] = cursorStyle.Render(rowText)
		case entry.IsActive:
			lines[i+1] = activeStyle.Render(rowText)
		default:
			// Style number separately for color
			numPart := numStyle.Render(fmt.Sprintf(" <%s>", numStr))
			restText := fmt.Sprintf(" %-*s  %3d %3d %3d",
				nameW, name,
				entry.OpenCount, entry.InProgressCount, entry.ReadyCount)
			lines[i+1] = numPart + normalStyle.Render(restText)
		}
	}

	if len(m.filtered) > maxVisibleProjects {
		remaining := len(m.filtered) - maxVisibleProjects
		if visible < panelRows-1 {
			lines[visible+1] = dimStyle.Render(fmt.Sprintf("      ... +%d more", remaining))
		}
	}

	return lines
}

// renderShortcutsColumn renders the shortcuts column.
func (m *ProjectPickerModel) renderShortcutsColumn() []string {
	t := m.theme

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#BD93F9"}).
		Bold(true)
	descStyle := t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground())

	shortcuts := pickerShortcuts()
	lines := make([]string, panelRows)

	for i := 0; i < len(shortcuts) && i < panelRows; i++ {
		s := shortcuts[i]
		lines[i] = keyStyle.Render(fmt.Sprintf("%-7s", s.key)) + " " + descStyle.Render(s.desc)
	}

	return lines
}

// renderLogoColumn renders the B9s ASCII art logo.
func (m *ProjectPickerModel) renderLogoColumn() []string {
	t := m.theme
	logoStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"})

	logo := b9sLogo()
	lines := make([]string, panelRows)
	for i := 0; i < len(logo) && i < panelRows; i++ {
		lines[i] = logoStyle.Render(logo[i])
	}

	return lines
}

// renderTitleBar renders the k9s-style title bar with resource type and count.
func (m *ProjectPickerModel) renderTitleBar(w int) string {
	t := m.theme

	titleText := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	countText := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"})

	label := "projects"
	if m.filtering && m.filterInput.Value() != "" {
		label = fmt.Sprintf("projects(%s)", m.filterInput.Value())
	} else {
		for _, entry := range m.entries {
			if entry.IsActive {
				label = fmt.Sprintf("projects(%s)", entry.Project.Name)
				break
			}
		}
	}

	title := titleText.Render(label) + countText.Render(fmt.Sprintf("[%d]", len(m.filtered)))

	sepChar := "\u2500"
	sepStyle := t.Renderer.NewStyle().Foreground(t.Border)

	titleLen := len(label) + len(fmt.Sprintf("[%d]", len(m.filtered)))
	leftPad := (w - titleLen - 4) / 2
	rightPad := w - titleLen - 4 - leftPad
	if leftPad < 1 {
		leftPad = 1
	}
	if rightPad < 1 {
		rightPad = 1
	}

	return sepStyle.Render(strings.Repeat(sepChar, leftPad)) + " " + title + " " + sepStyle.Render(strings.Repeat(sepChar, rightPad))
}

// maxLineWidth returns the max visible width across a set of pre-rendered lines.
// Uses lipgloss.Width to account for ANSI escape codes.
func (m *ProjectPickerModel) maxLineWidth(lines []string) int {
	maxW := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

// safeIndex returns lines[i] or empty string if out of bounds.
func safeIndex(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
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
