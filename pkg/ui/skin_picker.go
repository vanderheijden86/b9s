package ui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/skin"
)

// skinsDir holds the skin files :skins offers next to the built-in skins.
// b9s never reads the k9s config, so k9s skins appear here only when the user
// copies or links them in (ADR 0034).
func skinsDir() string {
	if dir := config.ConfigDir(); dir != "" {
		return filepath.Join(dir, "skins")
	}
	return ""
}

// SkinPickerModel lists the skins :skins can switch to.
type SkinPickerModel struct {
	entries       []skin.Entry
	current       string
	selectedIndex int
	width         int
	height        int
	theme         Theme
}

// NewSkinPickerModel highlights the entry whose ref is current.
func NewSkinPickerModel(entries []skin.Entry, current string, theme Theme) SkinPickerModel {
	m := SkinPickerModel{entries: entries, current: current, theme: theme}
	for i, e := range entries {
		if e.Ref == current {
			m.selectedIndex = i
		}
	}
	return m
}

func (m *SkinPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *SkinPickerModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

func (m *SkinPickerModel) MoveDown() {
	if m.selectedIndex < len(m.entries)-1 {
		m.selectedIndex++
	}
}

// Selected returns the highlighted skin, or a zero Entry when there is none.
func (m SkinPickerModel) Selected() skin.Entry {
	if m.selectedIndex >= 0 && m.selectedIndex < len(m.entries) {
		return m.entries[m.selectedIndex]
	}
	return skin.Entry{}
}

func (m SkinPickerModel) View() string {
	width, height := m.width, m.height
	if width == 0 {
		width = 60
	}
	if height == 0 {
		height = 20
	}
	t := m.theme
	boxWidth := 40
	if width < 50 {
		boxWidth = max(width-10, 25)
	}

	lines := []string{t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render("Skins"), ""}
	for i, e := range m.entries {
		style := t.Renderer.NewStyle().Foreground(t.Base.GetForeground())
		prefix := "  "
		if i == m.selectedIndex {
			style = style.Foreground(t.Primary).Bold(true)
			prefix = "> "
		}
		line := style.Render(prefix + e.Name)
		if e.Ref != e.Name {
			line += t.MutedText.Render("  file")
		}
		if e.Ref == m.current {
			line += " " + t.Renderer.NewStyle().Foreground(t.Secondary).Render("✓")
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true).
		Render("j/k: navigate | enter: apply | esc: cancel"))

	box := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(boxWidth).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) openSkinPicker() Model {
	current := m.appConfig.UI.Skin
	if current == "" {
		current = lightSkin.Name
		if m.theme.Renderer.HasDarkBackground() {
			current = darkSkin.Name
		}
	}
	m.skinPicker = NewSkinPickerModel(skin.Available(skinsDir()), current, m.theme)
	m.skinPicker.SetSize(m.width, m.height-1)
	m.showSkinPicker = true
	return m
}

func (m Model) handleSkinPickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.skinPicker.MoveDown()
	case "k", "up":
		m.skinPicker.MoveUp()
	case "esc", "q":
		m.showSkinPicker = false
	case "enter":
		m.showSkinPicker = false
		entry := m.skinPicker.Selected()
		p, err := skin.Resolve(entry.Ref)
		if err != nil {
			m.statusMsg = "Skin not loaded: " + err.Error()
			m.statusIsError = true
			return m, nil
		}
		m.switchSkin(p, entry.Ref)
		m.statusMsg = "Skin: " + p.Name
		m.statusIsError = false
		m.saveUIChoice("skin")
	}
	return m, nil
}
