package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PromptMode is the ':' command prompt's input lifecycle.
type PromptMode uint8

const (
	PromptIdle PromptMode = iota
	PromptEditing
)

// OpenProjectTableMsg asks the model to show the :project table.
type OpenProjectTableMsg struct{}

// CommandPrompt owns the ':' prompt text.
type CommandPrompt struct {
	mode PromptMode
	text string
}

func (p CommandPrompt) Mode() PromptMode { return p.mode }

func (p CommandPrompt) Text() string { return p.text }

// Suggestion is the canonical command the typed text is a prefix of.
func (p CommandPrompt) Suggestion() string { return commandSuggestion(p.text) }

func (p *CommandPrompt) Start() {
	p.mode = PromptEditing
	p.text = ""
}

func (p *CommandPrompt) Cancel() {
	p.mode = PromptIdle
	p.text = ""
}

func (p *CommandPrompt) Append(runes ...rune) {
	p.text += string(runes)
}

func (p *CommandPrompt) Backspace() {
	runes := []rune(p.text)
	if len(runes) > 0 {
		p.text = string(runes[:len(runes)-1])
	}
}

func (p *CommandPrompt) AcceptSuggestion() {
	if suggestion := p.Suggestion(); suggestion != "" {
		p.text = suggestion
	}
}

func (m Model) handleCommandPromptKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commandPrompt.Cancel()
	case "enter":
		text := m.commandPrompt.Text()
		m.commandPrompt.Cancel()
		if strings.TrimSpace(text) == "" {
			return m, nil
		}
		command, err := ResolveCommand(text)
		if err != nil {
			m.statusMsg = err.Error()
			m.statusIsError = true
			return m, nil
		}
		return m.executeCommand(command)
	case "tab", "right":
		m.commandPrompt.AcceptSuggestion()
	case "backspace":
		// Backspace on an empty prompt closes it, as in k9s.
		if m.commandPrompt.Text() == "" {
			m.commandPrompt.Cancel()
		} else {
			m.commandPrompt.Backspace()
		}
	default:
		if runes := typedRunes(msg); len(runes) > 0 {
			m.commandPrompt.Append(runes...)
		}
	}
	return m, nil
}

// typedRunes returns the text a key press contributes to a text input.
// Bubbletea reports the space bar as KeySpace rather than KeyRunes, so a
// KeyRunes-only check silently drops every space.
func typedRunes(msg tea.KeyMsg) []rune {
	if msg.Alt {
		return nil
	}
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		return msg.Runes
	}
	return nil
}

func (m Model) executeCommand(command Command) (Model, tea.Cmd) {
	switch command.Kind {
	case CommandTypeFilter:
		m.setQueryText(withTypeFilter(m.queryState.Text(), command.IssueType))
	case CommandClearType:
		m.setQueryText(withTypeFilter(m.queryState.Text(), ""))
	case CommandProjects:
		return m, func() tea.Msg { return OpenProjectTableMsg{} }
	case CommandMouse:
		return m.toggleMouseCapture()
	case CommandLayout:
		return m.toggleSplitLayout(), nil
	case CommandWrap:
		return m.toggleTitleWrap(), nil
	}
	return m, nil
}

// MouseCaptured reports whether b9s receives mouse events. The program starts
// with cell-motion capture on, for wheel scrolling.
func (m Model) MouseCaptured() bool { return !m.mouseReleased }

// toggleMouseCapture hands the mouse to the terminal, or takes it back. While
// b9s captures it, tmux with "mouse on" forwards drags to b9s instead of
// starting copy mode, so text cannot be selected.
func (m Model) toggleMouseCapture() (Model, tea.Cmd) {
	m.mouseReleased = !m.mouseReleased
	if m.mouseReleased {
		m.statusMsg = "Mouse released: drag to select text, :mouse to restore wheel scroll"
		return m, tea.DisableMouse
	}
	m.statusMsg = "Mouse captured: wheel scrolls b9s, :mouse to select text"
	return m, tea.EnableMouseCellMotion
}

// toggleSplitLayout switches the split between the detail pane to the right
// of the list and the detail pane below it.
func (m Model) toggleSplitLayout() Model {
	m.splitStacked = !m.splitStacked
	m.recalculateSplitPaneSizes()
	m.syncTreeSize()
	if m.treeViewActive && !m.treeDetailHidden {
		m.syncTreeToDetail()
	}
	if m.splitStacked {
		m.statusMsg = "Layout stacked: detail below the list, \\ for side by side"
	} else {
		m.statusMsg = "Layout side by side: detail right of the list, \\ to stack"
	}
	m.statusIsError = false
	return m
}

// toggleTitleWrap switches tree titles between one truncated line and the
// full title wrapped under the title column.
func (m Model) toggleTitleWrap() Model {
	m.tree.ToggleWrapTitles()
	if m.tree.WrapTitles() {
		m.statusMsg = "Titles wrapped: v or :wrap for one line per issue"
	} else {
		m.statusMsg = "Titles truncated: v or :wrap to show full titles"
	}
	m.statusIsError = false
	return m
}
