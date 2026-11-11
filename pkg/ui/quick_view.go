package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/model"
)

type quickViewMode uint8

const (
	quickViewChoosing quickViewMode = iota
	quickViewReading
)

// The preview owns its selection and viewport. The issue under it remains the
// action target, so preview input must never reach contextual key handlers.
type quickView struct {
	mode        quickViewMode
	returnFocus focus
	originID    string
	id          string
	input       textinput.Model
	cursor      int
	viewport    viewport.Model
	renderer    *MarkdownRenderer
}

func (m *Model) openQuickView() {
	issue := m.getSelectedIssue()
	if issue == nil {
		return
	}
	input := textinput.New()
	input.Prompt = "ID or title: "
	input.CharLimit = 256
	input.Focus()
	m.quickView = &quickView{
		returnFocus: m.focused, originID: issue.ID, input: input,
		viewport: viewport.New(1, 1),
		renderer: NewMarkdownRendererWithTheme(80, m.theme),
	}
	if m.focused != focusDetail {
		m.quickView.mode, m.quickView.id = quickViewReading, issue.ID
	} else if issue.Abridged {
		m.requestIssueText(*issue)
	}
	m.refreshQuickView()
}

// quickViewCandidates searches the full snapshot, including rows hidden by
// the original query. Relations and literal mentions lead the empty chooser.
func (m Model) quickViewCandidates() []string {
	q := m.quickView
	query := strings.ToLower(strings.TrimSpace(q.input.Value()))
	related := make(map[string]bool)
	if origin := m.issueMap[q.originID]; origin != nil {
		rels, children := detailRelations(origin.ID, *origin, m.issueMap)
		for _, rel := range append(rels, children...) {
			related[rel.id] = true
		}
		text := ""
		if !origin.Abridged {
			text = strings.Join([]string{origin.Description, origin.Design, origin.AcceptanceCriteria, origin.Notes}, "\n")
			for _, comment := range origin.Comments {
				if comment != nil {
					text += "\n" + comment.Text
				}
			}
		}
		for id, issue := range m.issueMap {
			if strings.Contains(text, id) {
				related[id] = true
			}
			if issue != nil {
				for _, dep := range issue.Dependencies {
					if dep != nil && dep.DependsOnID == origin.ID {
						related[id] = true
					}
				}
			}
		}
	}
	var ids []string
	for id, issue := range m.issueMap {
		if issue != nil && id != q.originID &&
			(query == "" || strings.Contains(strings.ToLower(id+" "+issue.Title), query)) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		if query == "" && related[ids[i]] != related[ids[j]] {
			return related[ids[i]]
		}
		return ids[i] < ids[j]
	})
	return ids
}

func (m *Model) refreshQuickView() {
	q := m.quickView
	if q == nil {
		return
	}
	q.viewport.Width = max(1, min(100, m.width-4)-4)
	q.viewport.Height = max(1, m.height-9)
	q.input.Width = max(1, q.viewport.Width-13)
	if q.mode != quickViewReading {
		return
	}
	issue := m.issueMap[q.id]
	if issue == nil {
		q.viewport.SetContent("Issue is no longer available. Esc returns to the original view.")
		return
	}
	item := *issue
	// Loading and errors belong inside the preview, leaving the original
	// detail's status line ready for the return from Esc.
	status, statusIsError := m.statusMsg, m.statusIsError
	whole := m.wholeIssue(issue)
	m.statusMsg, m.statusIsError = status, statusIsError
	if whole != nil {
		item = *whole
	} else {
		item = m.placeholderText(item)
	}
	q.renderer.SetWidthWithTheme(q.viewport.Width, m.theme)
	var decisions []string
	if graph, ok := m.tree.MemoryGraph(); ok {
		decisions = decisionSection(m.theme, graph, item.ID, max(20, q.viewport.Width))
	}
	q.viewport.SetContent(renderIssueDetail(item, m.issueMap, m.theme, q.viewport.Width, q.renderer, nil, decisions, m.commentsOldestFirst))
}

func (m *Model) cycleQuickViewSibling(delta int) {
	q := m.quickView
	issue := m.issueMap[q.id]
	if issue == nil {
		return
	}
	var parent string
	for _, dep := range issue.Dependencies {
		if dep != nil && dep.Type == model.DepParentChild {
			parent = dep.DependsOnID
			break
		}
	}
	if parent == "" {
		return
	}
	// The parent may be absent from a partial snapshot while its children exist.
	ids := detailChildIDs(model.Issue{ID: parent}, m.issueMap)
	for i, id := range ids {
		if id == q.id {
			q.id = ids[(i+delta+len(ids))%len(ids)]
			q.viewport.GotoTop()
			m.refreshQuickView()
			return
		}
	}
}

func (m Model) handleQuickViewKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	q := m.quickView
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.focused = q.returnFocus
		m.quickView = nil
		return m, nil
	}
	if q.mode == quickViewChoosing {
		ids := m.quickViewCandidates()
		switch msg.String() {
		case "up":
			q.cursor = max(0, q.cursor-1)
		case "down":
			q.cursor = min(max(0, len(ids)-1), q.cursor+1)
		case "enter":
			if q.cursor < len(ids) {
				q.mode, q.id = quickViewReading, ids[q.cursor]
				m.refreshQuickView()
			}
		default:
			var cmd tea.Cmd
			q.input, cmd = q.input.Update(msg)
			q.cursor = 0
			return m, cmd
		}
		return m, nil
	}
	switch msg.String() {
	case "n":
		m.cycleQuickViewSibling(1)
	case "p":
		m.cycleQuickViewSibling(-1)
	case "home":
		q.viewport.GotoTop()
	case "end":
		q.viewport.GotoBottom()
	case "j", "k", "up", "down", "pgup", "pgdown", "ctrl+d", "ctrl+u", "ctrl+b", "ctrl+f":
		q.viewport, _ = q.viewport.Update(msg)
	}
	return m, nil
}

func (m Model) renderQuickView() string {
	q := m.quickView
	title, content, hint := "Quick view · read-only", q.viewport.View(), "j/k: scroll  n/p: cycle siblings  Esc: return"
	if q.mode == quickViewChoosing {
		title = "Preview issue"
		ids := m.quickViewCandidates()
		lines := []string{q.input.View(), "", "Related and mentioned issues first. Type to search all loaded issues."}
		if len(ids) == 0 {
			lines = append(lines, "No matching issues")
		}
		rows := max(1, q.viewport.Height-4)
		start := max(0, min(q.cursor-rows+1, len(ids)-rows))
		for i := start; i < min(len(ids), start+rows); i++ {
			issue := m.issueMap[ids[i]]
			line := fmt.Sprintf("  %s  %s", sanitizeTerminalLine(issue.ID), sanitizeTerminalLine(issue.Title))
			if i == q.cursor {
				line = m.theme.Renderer.NewStyle().Foreground(m.theme.Primary).Bold(true).Render("> " + strings.TrimSpace(line))
			}
			lines = append(lines, line)
		}
		content, hint = strings.Join(lines, "\n"), "↑/↓: select  Enter: preview  Esc: return"
	}
	box := m.theme.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(m.theme.Primary).
		Padding(0, 1).Width(q.viewport.Width + 2).
		MaxWidth(max(1, m.width)).MaxHeight(max(1, m.height)).
		Render(m.theme.Renderer.NewStyle().Bold(true).Render(title) + "\n\n" + content + "\n\n" + hint)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
