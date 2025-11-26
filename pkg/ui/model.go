package ui

import (
	"fmt"
	"sort"
	"strings"

	"beads_viewer/pkg/model"
	"beads_viewer/pkg/updater"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const (
	SplitViewThreshold = 100
	WideViewThreshold  = 140
	UltraWideViewThreshold = 180
)

type focus int

const (
	focusList focus = iota
	focusDetail
	focusBoard
)

type UpdateMsg struct {
	TagName string
	URL     string
}

func CheckUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		tag, url, err := updater.CheckForUpdates()
		if err == nil && tag != "" {
			return UpdateMsg{TagName: tag, URL: url}
		}
		return nil
	}
}

type Model struct {
	issues        []model.Issue
	issueMap      map[string]*model.Issue
	list          list.Model
	viewport      viewport.Model
	renderer      *glamour.TermRenderer
	board         BoardModel
	
	// Update State
	updateAvailable bool
	updateTag       string
	updateURL       string
	
	// State
	focused       focus
	isSplitView   bool
	isBoardView   bool
	showDetails   bool
	ready         bool
	width         int
	height        int
	
	// Filter state
	currentFilter string // "all", "open", "closed", "ready"
	searchTerm    string // simple fuzzy search buffer (future enhancement)
	
	// Stats
	countOpen    int
	countReady   int
	countBlocked int
	countClosed  int
}

func NewModel(issues []model.Issue) Model {
	// Build map
	issueMap := make(map[string]*model.Issue)
	
	// Count stats
	var cOpen, cReady, cBlocked, cClosed int
	
	// Sort issues: Open first, then by Priority
	sort.Slice(issues, func(i, j int) bool {
		iClosed := issues[i].Status == "closed"
		jClosed := issues[j].Status == "closed"
		if iClosed != jClosed {
			return !iClosed
		}
		if issues[i].Priority != issues[j].Priority {
			return issues[i].Priority < issues[j].Priority
		}
		return issues[i].CreatedAt.After(issues[j].CreatedAt)
	})

	items := make([]list.Item, len(issues))
	for i, issue := range issues {
		issueMap[issue.ID] = &issues[i]
		items[i] = IssueItem{Issue: issue}
		
		// Stats
		if issue.Status == model.StatusClosed {
			cClosed++
		} else if issue.Status == model.StatusBlocked {
			cBlocked++
			cOpen++ // Blocked is technically open
		} else {
			cOpen++
			// Check if ready
			// isBlocked logic here is complex during initialization as map isn't full
		}
	}
	
	// Re-calc stats accurately
	cOpen, cReady, cBlocked, cClosed = 0, 0, 0, 0
	for _, issue := range issues {
		if issue.Status == model.StatusClosed {
			cClosed++
		} else {
			cOpen++
			if issue.Status == model.StatusBlocked {
				cBlocked++
			} else {
				// Check if blocked by dependencies
				isBlocked := false
				for _, dep := range issue.Dependencies {
					if dep.Type == model.DepBlocks {
						blocker, exists := issueMap[dep.DependsOnID]
						if exists && blocker.Status != model.StatusClosed {
							isBlocked = true
							break
						}
					}
				}
				if !isBlocked {
					cReady++
				}
			}
		}
	}

	// Default delegate
	delegate := IssueDelegate{Tier: TierCompact}
	l := list.New(items, delegate, 0, 0)
	l.Title = "Beads"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true) // Enable default fuzzy filter
	l.DisableQuitKeybindings()
	
	// Glamour renderer with Dark Style
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	
	// Initialize Board
	board := NewBoardModel(issues)

	m := Model{
		issues:        issues,
		issueMap:      issueMap,
		list:          l,
		renderer:      r,
		board:         board,
		currentFilter: "all",
		focused:       focusList,
		countOpen:     cOpen,
		countReady:    cReady,
		countBlocked:  cBlocked,
		countClosed:   cClosed,
	}
	
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd {
	return CheckUpdateCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case UpdateMsg:
		m.updateAvailable = true
		m.updateTag = msg.TagName
		m.updateURL = msg.URL
		// Maybe show a toast or just status bar indicator?
		// For "asks the user", we can trigger a modal or just show prominently.
		// A slick way is to change the Title or Footer.
		
	case tea.KeyMsg:
		// Filtering Keybindings (only when not filtering in list)
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "ctrl+c", "q":
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					return m, nil
				}
				return m, tea.Quit
			case "esc":
				if m.showDetails && !m.isSplitView {
					m.showDetails = false
					return m, nil
				}
			case "tab":
				if m.isSplitView && !m.isBoardView {
					if m.focused == focusList {
						m.focused = focusDetail
					} else {
						m.focused = focusList
					}
				}
			case "b":
				m.isBoardView = !m.isBoardView
				if m.isBoardView {
					m.focused = focusBoard
				} else {
					m.focused = focusList
				}
			}

			// Focus-specific
			if m.focused == focusBoard {
				switch msg.String() {
				case "h", "left":
					m.board.MoveLeft()
				case "l", "right":
					m.board.MoveRight()
				case "j", "down":
					m.board.MoveDown()
				case "k", "up":
					m.board.MoveUp()
				case "enter":
					// Switch to detail view of selected issue
					selected := m.board.SelectedIssue()
					if selected != nil {
						// Find index in list? Or just force viewport update
						// Ideally sync list selection
						// For now, just toggle off board and try to select in list?
						// Too complex. Let's just show details overlay?
						// Let's reuse SplitView logic: set selected item in list
						// Finding index is slow.
						// Let's just update viewport manually and switch focus to detail if split view.
						// Or just switch back to list view but filtered?
						
						// Simple hack: Switch back to list view, select correct item
						// requires linear scan
						for i, item := range m.list.Items() {
							if item.(IssueItem).Issue.ID == selected.ID {
								m.list.Select(i)
								break
							}
						}
						m.isBoardView = false
						m.focused = focusList
						if m.isSplitView {
							m.focused = focusDetail
						} else {
							m.showDetails = true
						}
						m.updateViewportContent()
					}
				}
			} else if m.focused == focusList {
				switch msg.String() {
				case "enter":
					if !m.isSplitView {
						m.showDetails = true
						m.updateViewportContent()
					}
				case "o":
					m.currentFilter = "open"
					m.applyFilter()
				case "c":
					m.currentFilter = "closed"
					m.applyFilter()
				case "r":
					m.currentFilter = "ready"
					m.applyFilter()
				case "a":
					m.currentFilter = "all"
					m.applyFilter()
				}
			} else {
				m.viewport, cmd = m.viewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.isSplitView = msg.Width > SplitViewThreshold
		
		m.ready = true
		
		// Layout calculations
		headerHeight := 1 // Status bar
		availableHeight := msg.Height - headerHeight
		
		var listWidth int
		
		if m.isSplitView {
			listWidth = int(float64(msg.Width) * 0.4)
			detailWidth := msg.Width - listWidth - 4 // margins
			
			m.list.SetSize(listWidth, availableHeight)
			m.viewport = viewport.New(detailWidth, availableHeight-2) // -2 for border
		} else {
			listWidth = msg.Width
			m.list.SetSize(msg.Width, availableHeight)
			m.viewport = viewport.New(msg.Width, availableHeight-2)
		}
		
		// Adaptive Delegate Tier based on List Width
		var tier Tier
		if listWidth > 120 {
			tier = TierUltraWide
		} else if listWidth > 90 {
			tier = TierWide
		} else if listWidth > 60 {
			tier = TierNormal
		} else {
			tier = TierCompact
		}
		m.list.SetDelegate(IssueDelegate{Tier: tier})
		
		if m.isSplitView {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width),
			)
		}
		m.updateViewportContent()
	}
	
	// Always update list (handles filtering input)
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	
	// Update viewport if list selection changed in split view
	if m.isSplitView && m.focused == focusList {
		// Check if selection actually changed to avoid re-rendering cost? 
		// For now just update, it's fast enough.
		m.updateViewportContent()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var body string

	if m.isBoardView {
		body = m.board.View(m.width, m.height-1)
	} else if m.isSplitView {
		// Split View
		var listStyle, detailStyle lipgloss.Style
		
		if m.focused == focusList {
			listStyle = FocusedPanelStyle
			detailStyle = PanelStyle
		} else {
			listStyle = PanelStyle
			detailStyle = FocusedPanelStyle
		}
		
		listView := listStyle.Width(m.list.Width()).Height(m.height-2).Render(m.list.View())
		detailView := detailStyle.Width(m.viewport.Width+2).Height(m.height-2).Render(m.viewport.View())
		
		body = lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
	} else {
		// Mobile View
		if m.showDetails {
			body = m.viewport.View()
		} else {
			body = m.list.View()
		}
	}
	
	// Footer / Status Bar
	footer := m.renderFooter()
	
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m *Model) renderFooter() string {
	filterStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(ColorSubtext)
	countStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Padding(0, 1)
	
	var filterTxt string
	switch m.currentFilter {
	case "all": filterTxt = "ALL"
	case "open": filterTxt = "OPEN"
	case "closed": filterTxt = "CLOSED"
	case "ready": filterTxt = "READY"
	}
	
	status := fmt.Sprintf(" Filter: %s ", filterTxt) 
	count := fmt.Sprintf("%d issues", len(m.list.Items()))
	
	// Stats block
	stats := fmt.Sprintf(" Open:%d Ready:%d Blocked:%d Closed:%d ", m.countOpen, m.countReady, m.countBlocked, m.countClosed)
	
	// Update block
	updateTxt := ""
	if m.updateAvailable {
		updateTxt = " ⭐ UPDATE AVAILABLE "
	}

	var keys string
	if m.isBoardView {
		keys = "h/j/k/l: nav • enter: view • b: list • q: quit"
	} else if m.list.FilterState() == list.Filtering {
		keys = "esc: cancel filter • enter: select"
	} else {
		if m.isSplitView {
			keys = "tab: focus • b: board • o/c/r/a: filter • /: search • q: quit"
		} else {
			if m.showDetails {
				keys = "esc: back • j/k: scroll • q: quit"
			} else {
				keys = "enter: details • b: board • o/c/r/a: filter • /: search • q: quit"
			}
		}
	}
	
	statusSection := filterStyle.Background(ColorPrimary).Padding(0, 1).Render(status)
	updateSection := lipgloss.NewStyle().Background(ColorTypeFeature).Foreground(ColorBg).Bold(true).Render(updateTxt)
	statsSection := lipgloss.NewStyle().Background(ColorBgHighlight).Foreground(ColorText).Render(stats)
	countSection := countStyle.Render(count)
	keysSection := helpStyle.Padding(0, 1).Render(keys)
	
	// Fill remaining space
	barWidth := m.width
	// left: status + update + stats
	// right: count + keys
	// middle: filler
	
	leftWidth := lipgloss.Width(statusSection) + lipgloss.Width(updateSection) + lipgloss.Width(statsSection)
	rightWidth := lipgloss.Width(countSection) + lipgloss.Width(keysSection)
	
	remaining := barWidth - leftWidth - rightWidth
	if remaining < 0 { remaining = 0 }
	filler := lipgloss.NewStyle().Background(ColorBgDark).Width(remaining).Render("")
	
	return lipgloss.JoinHorizontal(lipgloss.Bottom, statusSection, updateSection, statsSection, filler, countSection, keysSection)
}

func (m *Model) applyFilter() {
	var filtered []list.Item
	for _, issue := range m.issues {
		include := false
		switch m.currentFilter {
		case "all":
			include = true
		case "open":
			if issue.Status != model.StatusClosed {
				include = true
			}
		case "closed":
			if issue.Status == model.StatusClosed {
				include = true
			}
		case "ready":
			// Ready = Open/InProgress AND No Open Blocks
			if issue.Status != model.StatusClosed && issue.Status != model.StatusBlocked {
				isBlocked := false
				for _, dep := range issue.Dependencies {
					if dep.Type == model.DepBlocks {
						blocker, exists := m.issueMap[dep.DependsOnID]
						if exists && blocker.Status != model.StatusClosed {
							isBlocked = true
							break
						}
					}
				}
				if !isBlocked {
					include = true
				}
			}
		}

		if include {
			filtered = append(filtered, IssueItem{Issue: issue})
		}
	}
	m.list.SetItems(filtered)
	
	if len(filtered) > 0 {
		if m.list.Index() >= len(filtered) {
			m.list.Select(0)
		}
	}
	m.updateViewportContent()
}

func (m *Model) updateViewportContent() {
	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.viewport.SetContent("No issues selected")
		return
	}
	item := selectedItem.(IssueItem).Issue
	
	var sb strings.Builder

	if m.updateAvailable {
		sb.WriteString(fmt.Sprintf("⭐ **Update Available:** [%s](%s)\n\n", m.updateTag, m.updateURL))
	}

	// Title Block
	sb.WriteString(fmt.Sprintf("# %s %s\n", GetTypeIconMD(string(item.IssueType)), item.Title))
	
	// Meta Table
	sb.WriteString(fmt.Sprintf("| ID | Status | Priority | Assignee | Created |\n|---|---|---|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| **%s** | **%s** | %s | @%s | %s |\n\n", 
		item.ID, 
		strings.ToUpper(string(item.Status)), 
		GetPriorityIcon(item.Priority),
		item.Assignee,
		item.CreatedAt.Format("2006-01-02"),
	))
	
	// Description
	if item.Description != "" {
		sb.WriteString("### Description\n")
		sb.WriteString(item.Description + "\n\n")
	}

	// Acceptance Criteria
	if item.AcceptanceCriteria != "" {
		sb.WriteString("### Acceptance Criteria\n")
		sb.WriteString(item.AcceptanceCriteria + "\n\n")
	}
	
	// Notes
	if item.Notes != "" {
		sb.WriteString("### Notes\n")
		sb.WriteString(item.Notes + "\n\n")
	}
	
	// Dependency Graph (Tree)
	if len(item.Dependencies) > 0 {
		// We build a small tree rooted at current issue
		rootNode := BuildDependencyTree(item.ID, m.issueMap)
		treeStr := RenderDependencyTree(rootNode)
		sb.WriteString("```\n" + treeStr + "```\n\n")
	}

	// Comments
	if len(item.Comments) > 0 {
		sb.WriteString(fmt.Sprintf("### Comments (%d)\n", len(item.Comments)))
		for _, comment := range item.Comments {
			sb.WriteString(fmt.Sprintf("> **%s** (%s)\n> \n> %s\n\n", 
				comment.Author, 
				FormatTimeRel(comment.CreatedAt), 
				strings.ReplaceAll(comment.Text, "\n", "\n> ")))
		}
	}

	rendered, err := m.renderer.Render(sb.String())
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering markdown: %v", err))
	} else {
		m.viewport.SetContent(rendered)
	}
}

func GetTypeIconMD(t string) string {
	switch t {
	case "bug": return "🐛"
	case "feature": return "✨"
	case "task": return "📋"
	case "epic": return "🏔️"
	case "chore": return "🧹"
	default: return "•"
	}
}

// SetFilter sets the current filter and applies it (exposed for testing/control)
func (m *Model) SetFilter(f string) {
	m.currentFilter = f
	m.applyFilter()
}

// FilteredIssues returns the currently visible issues (exposed for testing)
func (m Model) FilteredIssues() []model.Issue {
	items := m.list.Items()
	issues := make([]model.Issue, len(items))
	for i, item := range items {
		issues[i] = item.(IssueItem).Issue
	}
	return issues
}
