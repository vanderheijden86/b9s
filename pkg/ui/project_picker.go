package ui

import (
	"fmt"
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
// It renders as a compact horizontal layout: shortcut bar, project chips, and title bar.
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
// Only filter entry (/) and number-key quick-switch are active.
// Navigation (j/k/enter) is intentionally omitted — the picker is display-only.
// Project switching is done via number keys 1-9, handled at top priority in Model.Update.
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

// View renders the compact horizontal project picker (bd-ylz).
// Layout: shortcut bar + (optional filter line) + horizontal project chips + title bar at bottom.
func (m *ProjectPickerModel) View() string {
	if m.width == 0 {
		m.width = 80
	}

	w := m.width

	var sections []string

	// --- Shortcut hints (k9s style) ---
	sections = append(sections, m.renderShortcutBar(w))

	// --- Filter input (shown inline when filtering) ---
	if m.filtering {
		t := m.theme
		filterStyle := t.Renderer.NewStyle().
			Foreground(t.Primary).
			Width(w)
		sections = append(sections, filterStyle.Render("  / "+m.filterInput.View()))
	}

	// --- Project chips (horizontal flow, wrapping) ---
	if len(m.filtered) == 0 {
		t := m.theme
		dimStyle := t.Renderer.NewStyle().
			Foreground(t.Secondary).
			Italic(true)
		sections = append(sections, dimStyle.Render("  No projects found. Configure scan_paths in ~/.config/bw/config.yaml"))
	} else {
		chipLines := m.renderProjectChips(w)
		sections = append(sections, chipLines...)
	}

	// --- Title bar at bottom (divider between picker and tree) ---
	sections = append(sections, m.renderTitleBar(w))

	return strings.Join(sections, "\n")
}

// Height returns the number of terminal lines the compact picker uses.
func (m *ProjectPickerModel) Height() int {
	lines := 1 // shortcut bar
	if m.filtering {
		lines++ // filter input line
	}
	if len(m.filtered) == 0 {
		lines++ // "No projects found" message
	} else {
		lines += m.projectChipLineCount(m.width)
	}
	lines++ // title bar at bottom
	return lines
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
		{"<1-9>", "Quick Switch"},
		{"</>", "Filter"},
	}

	var parts []string
	for _, s := range shortcuts {
		parts = append(parts, keyStyle.Render(s.key)+" "+descStyle.Render(s.desc))
	}

	line := strings.Join(parts, "  ")

	return " " + line
}

// renderTitleBar renders the k9s-style title bar with resource type and count.
func (m *ProjectPickerModel) renderTitleBar(w int) string {
	t := m.theme

	titleText := t.Renderer.NewStyle().
		Foreground(t.Primary).
		Bold(true)

	countText := t.Renderer.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#006080", Dark: "#8BE9FD"})

	// Show active project in title bar like k9s: projects(active-name)[count]
	// When filtering, show the filter text instead.
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

// renderProjectChips flows project chips horizontally, wrapping at terminal width.
// Returns one string per line of chips.
func (m *ProjectPickerModel) renderProjectChips(w int) []string {
	var lines []string
	var currentLine strings.Builder
	currentLen := 0
	indent := "  " // 2-space indent
	indentLen := 2

	for i := 0; i < len(m.filtered); i++ {
		entry := m.entries[m.filtered[i]]
		isCursor := m.filtering && i == m.cursor
		chip := m.renderChip(entry, isCursor)
		chipTextLen := m.chipTextLen(entry)

		// Check if chip fits on current line
		if currentLen > indentLen && currentLen+chipTextLen+2 > w {
			// Wrap to next line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLen = 0
		}

		if currentLen == 0 {
			currentLine.WriteString(indent)
			currentLen = indentLen
		} else {
			currentLine.WriteString("  ") // gap between chips
			currentLen += 2
		}

		currentLine.WriteString(chip)
		currentLen += chipTextLen
	}

	if currentLen > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// renderChip renders a single project chip: "N name(open/prog/ready)"
func (m *ProjectPickerModel) renderChip(entry ProjectEntry, isCursor bool) string {
	t := m.theme

	// Favorite number
	numStr := " "
	if entry.FavoriteNum > 0 {
		numStr = fmt.Sprintf("%d", entry.FavoriteNum)
	}

	// Counts: (open/prog/ready)
	counts := fmt.Sprintf("(%d/%d/%d)", entry.OpenCount, entry.InProgressCount, entry.ReadyCount)

	text := fmt.Sprintf("%s %s%s", numStr, entry.Project.Name, counts)

	if isCursor {
		// Cursor highlight during filter mode
		return t.Renderer.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render(text)
	}

	if entry.IsActive {
		// Active project: bold + primary color
		return t.Renderer.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render(text)
	}

	// Normal project
	return t.Renderer.NewStyle().
		Foreground(t.Base.GetForeground()).
		Render(text)
}

// chipTextLen returns the visible character length of a chip (without ANSI codes).
func (m *ProjectPickerModel) chipTextLen(entry ProjectEntry) int {
	numStr := " "
	if entry.FavoriteNum > 0 {
		numStr = fmt.Sprintf("%d", entry.FavoriteNum)
	}
	counts := fmt.Sprintf("(%d/%d/%d)", entry.OpenCount, entry.InProgressCount, entry.ReadyCount)
	return len(numStr) + 1 + len(entry.Project.Name) + len(counts)
}

// projectChipLineCount predicts how many lines the chip section will use at the given width.
func (m *ProjectPickerModel) projectChipLineCount(w int) int {
	if len(m.filtered) == 0 {
		return 0
	}

	lines := 1
	currentLen := 2 // indent
	indentLen := 2

	for i := 0; i < len(m.filtered); i++ {
		entry := m.entries[m.filtered[i]]
		chipLen := m.chipTextLen(entry)

		if currentLen > indentLen && currentLen+chipLen+2 > w {
			lines++
			currentLen = indentLen
		}

		if currentLen == indentLen {
			currentLen += chipLen
		} else {
			currentLen += chipLen + 2
		}
	}

	return lines
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
