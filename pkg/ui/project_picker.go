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
	Project      config.Project
	FavoriteNum  int  // 0 = not favorited, 1-9 = key
	IsActive     bool // Currently loaded project
	OpenCount    int
	ReadyCount   int
	BlockedCount int
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

// ProjectPickerModel is a full-screen k9s-style view for selecting projects.
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
	case "g":
		m.cursor = 0
	case "G":
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
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

// View renders the full-screen k9s-style project picker.
func (m *ProjectPickerModel) View() string {
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	t := m.theme
	w := m.width

	var sections []string

	// --- Shortcut hints (k9s style: colored key + description grid) ---
	sections = append(sections, m.renderShortcutBar(w))

	// --- Title bar: " projects(filtered)[count] " ---
	sections = append(sections, m.renderTitleBar(w))

	// --- Column headers ---
	sections = append(sections, m.renderColumnHeaders(w))

	// --- Filter input (shown inline when filtering) ---
	if m.filtering {
		filterStyle := t.Renderer.NewStyle().
			Foreground(t.Primary).
			Width(w)
		sections = append(sections, filterStyle.Render("  / "+m.filterInput.View()))
	}

	// --- Project rows ---
	headerLines := len(sections) + 1 // +1 for footer
	maxVisible := m.height - headerLines - 1
	if maxVisible < 3 {
		maxVisible = 3
	}

	if len(m.filtered) == 0 {
		dimStyle := t.Renderer.NewStyle().
			Foreground(t.Secondary).
			Italic(true)
		sections = append(sections, dimStyle.Render("  No projects found. Configure scan_paths in ~/.config/bw/config.yaml"))
	} else {
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
			sections = append(sections, m.renderRow(entry, isCursor, w))
		}
	}

	// Pad remaining vertical space
	usedLines := len(sections)
	remaining := m.height - usedLines
	if remaining > 0 {
		sections = append(sections, strings.Repeat("\n", remaining-1))
	}

	return strings.Join(sections, "\n")
}

// renderShortcutBar renders the k9s-style shortcut hints at the top.
func (m *ProjectPickerModel) renderShortcutBar(w int) string {
	t := m.theme

	keyStyle := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
		Bold(true)
	descStyle := t.Renderer.NewStyle().
		Foreground(t.Subtext)

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"<enter>", "Switch"},
		{"<1-9>", "Quick Switch"},
		{"<u>", "Favorite"},
		{"</>", "Filter"},
		{"<esc>", "Back"},
		{"<g>", "Top"},
		{"<G>", "Bottom"},
	}

	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, keyStyle.Render(s.key)+" "+descStyle.Render(s.desc))
	}

	line := strings.Join(parts, "  ")

	barStyle := t.Renderer.NewStyle().
		Width(w).
		Background(ColorBgHighlight).
		Padding(0, 1)

	return barStyle.Render(line)
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
	}

	title := titleText.Render(label) + countText.Render(fmt.Sprintf("[%d]", len(m.filtered)))

	// Center the title with separator lines
	sepChar := "\u2500"
	sepStyle := t.Renderer.NewStyle().Foreground(t.Border)

	// Calculate padding (approximate since styled text has zero-width codes)
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

// renderColumnHeaders renders the table column header row.
func (m *ProjectPickerModel) renderColumnHeaders(w int) string {
	t := m.theme
	nameW, pathW := m.columnWidths(w)

	header := fmt.Sprintf("  %-2s %-*s %-*s %6s %6s %8s",
		"#", nameW, "NAME", pathW, "PATH", "OPEN", "READY", "BLOCKED")

	headerStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Bold(true).
		Width(w)

	return headerStyle.Render(header)
}

// renderRow renders a single project entry row in k9s table style.
func (m *ProjectPickerModel) renderRow(entry ProjectEntry, isCursor bool, w int) string {
	t := m.theme
	nameW, pathW := m.columnWidths(w)

	// Favorite number
	favStr := " "
	if entry.FavoriteNum > 0 {
		favStr = fmt.Sprintf("%d", entry.FavoriteNum)
	}

	// Project name
	name := entry.Project.Name
	name = truncateRunesHelper(name, nameW, "...")

	// Path (abbreviated)
	path := abbreviatePath(entry.Project.Path)
	path = truncateRunesHelper(path, pathW, "...")

	// Counts
	openStr := fmt.Sprintf("%d", entry.OpenCount)
	readyStr := fmt.Sprintf("%d", entry.ReadyCount)
	blockedStr := fmt.Sprintf("%d", entry.BlockedCount)

	line := fmt.Sprintf("  %-2s %-*s %-*s %6s %6s %8s",
		favStr, nameW, name, pathW, path, openStr, readyStr, blockedStr)

	if isCursor {
		// Active row highlight: full-width background like k9s
		if entry.IsActive {
			// Active + selected: bright cyan/green
			return t.Renderer.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
				Background(ColorBgHighlight).
				Bold(true).
				Width(w).
				Render(line)
		}
		return t.Renderer.NewStyle().
			Foreground(t.Primary).
			Background(ColorBgHighlight).
			Bold(true).
			Width(w).
			Render(line)
	}

	if entry.IsActive {
		// Active project (not selected): cyan text
		return t.Renderer.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"}).
			Width(w).
			Render(line)
	}

	// Normal row
	return t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground()).
		Width(w).
		Render(line)
}

// columnWidths calculates name and path column widths based on terminal width.
func (m *ProjectPickerModel) columnWidths(totalWidth int) (nameWidth, pathWidth int) {
	// Fixed columns: "  # " (4) + " " (gaps) + "  OPEN" (7) + " READY" (7) + " BLOCKED" (9) = ~27 fixed
	available := totalWidth - 30
	if available < 20 {
		available = 20
	}
	// Split 35/65 between name and path
	nameWidth = available * 35 / 100
	pathWidth = available - nameWidth
	if nameWidth < 10 {
		nameWidth = 10
	}
	if pathWidth < 15 {
		pathWidth = 15
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
