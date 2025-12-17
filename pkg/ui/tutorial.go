package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TutorialPage represents a single page of tutorial content.
type TutorialPage struct {
	ID       string   // Unique identifier (e.g., "intro", "navigation")
	Title    string   // Page title displayed in header
	Content  string   // Markdown content
	Section  string   // Parent section for TOC grouping
	Contexts []string // Which view contexts this page applies to (empty = all)
}

// tutorialFocus tracks which element has focus (bv-wdsd)
type tutorialFocus int

const (
	focusTutorialContent tutorialFocus = iota
	focusTutorialTOC
)

// TutorialModel manages the tutorial overlay state.
type TutorialModel struct {
	pages        []TutorialPage
	currentPage  int
	scrollOffset int
	tocVisible   bool
	progress     map[string]bool // Tracks which pages have been viewed
	width        int
	height       int
	theme        Theme
	contextMode  bool   // If true, filter pages by current context
	context      string // Current view context (e.g., "list", "board", "graph")

	// Markdown rendering with Glamour (bv-lb0h)
	markdownRenderer *MarkdownRenderer

	// Keyboard navigation state (bv-wdsd)
	focus       tutorialFocus // Current focus: content or TOC
	shouldClose bool          // Signal to parent to close tutorial
	tocCursor   int           // Cursor position in TOC when focused
}

// NewTutorialModel creates a new tutorial model with default pages.
func NewTutorialModel(theme Theme) TutorialModel {
	// Calculate initial content width for markdown renderer
	contentWidth := 80 - 6 // default width minus padding
	if contentWidth < 40 {
		contentWidth = 40
	}

	return TutorialModel{
		pages:            defaultTutorialPages(),
		currentPage:      0,
		scrollOffset:     0,
		tocVisible:       false,
		progress:         make(map[string]bool),
		width:            80,
		height:           24,
		theme:            theme,
		contextMode:      false,
		context:          "",
		markdownRenderer: NewMarkdownRendererWithTheme(contentWidth, theme),
		focus:            focusTutorialContent,
		shouldClose:      false,
		tocCursor:        0,
	}
}

// Init initializes the tutorial model.
func (m TutorialModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the tutorial with focus management (bv-wdsd).
func (m TutorialModel) Update(msg tea.Msg) (TutorialModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keys (work in any focus mode)
		switch msg.String() {
		case "esc", "q":
			// Mark current page as viewed before closing
			pages := m.visiblePages()
			if m.currentPage >= 0 && m.currentPage < len(pages) {
				m.progress[pages[m.currentPage].ID] = true
			}
			m.shouldClose = true
			return m, nil

		case "t":
			// Toggle TOC and switch focus
			m.tocVisible = !m.tocVisible
			if m.tocVisible {
				m.focus = focusTutorialTOC
				m.tocCursor = m.currentPage // Sync TOC cursor with current page
			} else {
				m.focus = focusTutorialContent
			}
			return m, nil

		case "tab":
			// Switch focus between content and TOC (if visible)
			if m.tocVisible {
				if m.focus == focusTutorialContent {
					m.focus = focusTutorialTOC
					m.tocCursor = m.currentPage
				} else {
					m.focus = focusTutorialContent
				}
			} else {
				// If TOC not visible, tab advances page
				m.NextPage()
			}
			return m, nil
		}

		// Route to focus-specific handlers
		if m.focus == focusTutorialTOC && m.tocVisible {
			return m.handleTOCKeys(msg), nil
		}
		return m.handleContentKeys(msg), nil
	}
	return m, nil
}

// handleContentKeys handles keys when content area has focus (bv-wdsd).
func (m TutorialModel) handleContentKeys(msg tea.KeyMsg) TutorialModel {
	switch msg.String() {
	// Page navigation
	case "right", "l", "n", " ": // Space added for next page
		m.NextPage()
	case "left", "h", "p", "shift+tab":
		m.PrevPage()

	// Content scrolling
	case "j", "down":
		m.scrollOffset++
	case "k", "up":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}

	// Half-page scrolling
	case "ctrl+d":
		visibleHeight := m.height - 10
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		m.scrollOffset += visibleHeight / 2
	case "ctrl+u":
		visibleHeight := m.height - 10
		if visibleHeight < 5 {
			visibleHeight = 5
		}
		m.scrollOffset -= visibleHeight / 2
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}

	// Jump to top/bottom
	case "g", "home":
		m.scrollOffset = 0
	case "G", "end":
		m.scrollOffset = 9999 // Will be clamped in View()

	// Jump to specific page (1-9)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		pageNum := int(msg.String()[0] - '0')
		pages := m.visiblePages()
		if pageNum > 0 && pageNum <= len(pages) {
			m.JumpToPage(pageNum - 1)
		}
	}
	return m
}

// handleTOCKeys handles keys when TOC has focus (bv-wdsd).
func (m TutorialModel) handleTOCKeys(msg tea.KeyMsg) TutorialModel {
	pages := m.visiblePages()

	switch msg.String() {
	case "j", "down":
		if m.tocCursor < len(pages)-1 {
			m.tocCursor++
		}
	case "k", "up":
		if m.tocCursor > 0 {
			m.tocCursor--
		}
	case "g", "home":
		m.tocCursor = 0
	case "G", "end":
		m.tocCursor = len(pages) - 1
	case "enter", " ":
		// Jump to selected page in TOC
		m.JumpToPage(m.tocCursor)
		m.focus = focusTutorialContent
	case "h", "left":
		// Switch back to content
		m.focus = focusTutorialContent
	}
	return m
}

// View renders the tutorial overlay.
func (m TutorialModel) View() string {
	pages := m.visiblePages()
	if len(pages) == 0 {
		return m.renderEmptyState()
	}

	// Clamp current page
	if m.currentPage >= len(pages) {
		m.currentPage = len(pages) - 1
	}
	if m.currentPage < 0 {
		m.currentPage = 0
	}

	currentPage := pages[m.currentPage]

	// Mark as viewed
	m.progress[currentPage.ID] = true

	r := m.theme.Renderer

	// Calculate dimensions
	contentWidth := m.width - 6 // padding and borders
	if m.tocVisible {
		contentWidth -= 24 // TOC sidebar width
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Build the view
	var b strings.Builder

	// Header
	header := m.renderHeader(currentPage, len(pages))
	b.WriteString(header)
	b.WriteString("\n")

	// Separator line
	sepStyle := r.NewStyle().Foreground(m.theme.Border)
	b.WriteString(sepStyle.Render(strings.Repeat("─", contentWidth+4)))
	b.WriteString("\n")

	// Page title and section
	pageTitleStyle := r.NewStyle().Bold(true).Foreground(m.theme.Primary)
	sectionStyle := r.NewStyle().Foreground(m.theme.Subtext).Italic(true)
	pageTitle := pageTitleStyle.Render(currentPage.Title)
	if currentPage.Section != "" {
		pageTitle += sectionStyle.Render(" — " + currentPage.Section)
	}
	b.WriteString(pageTitle)
	b.WriteString("\n\n")

	// Content area (with optional TOC)
	if m.tocVisible {
		toc := m.renderTOC(pages)
		content := m.renderContent(currentPage, contentWidth)
		// Join TOC and content horizontally
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, toc, "  ", content))
	} else {
		content := m.renderContent(currentPage, contentWidth)
		b.WriteString(content)
	}

	b.WriteString("\n\n")

	// Footer with navigation hints
	footer := m.renderFooter(len(pages))
	b.WriteString(footer)

	// Wrap in modal style
	modalStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(m.width).
		MaxHeight(m.height)

	return modalStyle.Render(b.String())
}

// renderHeader renders the tutorial header with title and progress bar.
func (m TutorialModel) renderHeader(page TutorialPage, totalPages int) string {
	r := m.theme.Renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	// Progress indicator: [2/15] ███░░░
	pageNum := m.currentPage + 1
	progressText := r.NewStyle().
		Foreground(m.theme.Subtext).
		Render(fmt.Sprintf("[%d/%d]", pageNum, totalPages))

	// Visual progress bar
	barWidth := 10
	filledWidth := 0
	if totalPages > 0 {
		filledWidth = (pageNum * barWidth) / totalPages
		// Ensure at least 1 filled bar when on any page
		if filledWidth < 1 && pageNum > 0 {
			filledWidth = 1
		}
	}
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	progressBar := r.NewStyle().
		Foreground(m.theme.Open). // Using Open (green) for progress
		Render(strings.Repeat("█", filledWidth)) +
		r.NewStyle().
			Foreground(m.theme.Muted).
			Render(strings.Repeat("░", barWidth-filledWidth))

	// Title
	title := titleStyle.Render("📚 beads_viewer Tutorial")

	// Calculate spacing to align progress to the right
	headerContent := title + "  " + progressText + " " + progressBar

	return headerContent
}

// renderContent renders the page content with Glamour markdown and scroll handling.
func (m TutorialModel) renderContent(page TutorialPage, width int) string {
	r := m.theme.Renderer

	// Render markdown content using Glamour
	var renderedContent string
	if m.markdownRenderer != nil {
		rendered, err := m.markdownRenderer.Render(page.Content)
		if err == nil {
			renderedContent = strings.TrimSpace(rendered)
		} else {
			// Fallback to raw content on error
			renderedContent = page.Content
		}
	} else {
		renderedContent = page.Content
	}

	// Split rendered content into lines for scrolling
	lines := strings.Split(renderedContent, "\n")

	// Calculate visible lines based on height
	visibleHeight := m.height - 10 // header, footer, padding
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	// Clamp scroll offset
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}

	// Get visible lines
	endLine := m.scrollOffset + visibleHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}
	visibleLines := lines[m.scrollOffset:endLine]

	// Join visible lines (already styled by Glamour)
	content := strings.Join(visibleLines, "\n")

	// Add scroll indicators
	if m.scrollOffset > 0 {
		scrollUpHint := r.NewStyle().Foreground(m.theme.Muted).Render("↑ more above")
		content = scrollUpHint + "\n" + content
	}
	if endLine < len(lines) {
		scrollDownHint := r.NewStyle().Foreground(m.theme.Muted).Render("↓ more below")
		content = content + "\n" + scrollDownHint
	}

	return content
}

// renderTOC renders the table of contents sidebar with focus indication (bv-wdsd).
func (m TutorialModel) renderTOC(pages []TutorialPage) string {
	r := m.theme.Renderer

	// Use different border style when TOC has focus
	borderColor := m.theme.Border
	if m.focus == focusTutorialTOC {
		borderColor = m.theme.Primary
	}

	tocStyle := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(22)

	headerStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	sectionStyle := r.NewStyle().
		Foreground(m.theme.Secondary).
		Bold(true)

	itemStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	selectedStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	// TOC cursor style (when TOC has focus and cursor is on this item)
	cursorStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.InProgress).
		Background(m.theme.Highlight)

	viewedStyle := r.NewStyle().
		Foreground(m.theme.Open)

	var b strings.Builder
	b.WriteString(headerStyle.Render("Contents"))
	if m.focus == focusTutorialTOC {
		b.WriteString(r.NewStyle().Foreground(m.theme.Primary).Render(" ●"))
	}
	b.WriteString("\n")

	currentSection := ""
	for i, page := range pages {
		// Show section header if changed
		if page.Section != currentSection && page.Section != "" {
			currentSection = page.Section
			b.WriteString("\n")
			b.WriteString(sectionStyle.Render("▸ " + currentSection))
			b.WriteString("\n")
		}

		// Determine style based on cursor position and current page
		prefix := "   "
		style := itemStyle

		// TOC has focus and cursor is on this item
		if m.focus == focusTutorialTOC && i == m.tocCursor {
			prefix = " → "
			style = cursorStyle
		} else if i == m.currentPage {
			// Current page indicator (but not cursor)
			prefix = " ▶ "
			style = selectedStyle
		}

		// Truncate long titles
		title := page.Title
		if len(title) > 14 {
			title = title[:12] + "…"
		}

		// Viewed indicator
		viewed := ""
		if m.progress[page.ID] {
			viewed = viewedStyle.Render(" ✓")
		}

		b.WriteString(style.Render(prefix+title) + viewed)
		b.WriteString("\n")
	}

	return tocStyle.Render(b.String())
}

// renderFooter renders context-sensitive navigation hints (bv-wdsd).
func (m TutorialModel) renderFooter(totalPages int) string {
	r := m.theme.Renderer

	keyStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	descStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	sepStyle := r.NewStyle().
		Foreground(m.theme.Muted)

	var hints []string

	if m.focus == focusTutorialTOC && m.tocVisible {
		// TOC-focused hints
		hints = []string{
			keyStyle.Render("j/k") + descStyle.Render(" select"),
			keyStyle.Render("Enter") + descStyle.Render(" go to page"),
			keyStyle.Render("Tab") + descStyle.Render(" back to content"),
			keyStyle.Render("t") + descStyle.Render(" hide TOC"),
			keyStyle.Render("q") + descStyle.Render(" close"),
		}
	} else {
		// Content-focused hints
		hints = []string{
			keyStyle.Render("←/→/Space") + descStyle.Render(" pages"),
			keyStyle.Render("j/k") + descStyle.Render(" scroll"),
			keyStyle.Render("Ctrl+d/u") + descStyle.Render(" half-page"),
			keyStyle.Render("t") + descStyle.Render(" TOC"),
			keyStyle.Render("q") + descStyle.Render(" close"),
		}
	}

	sep := sepStyle.Render(" │ ")
	return strings.Join(hints, sep)
}

// renderEmptyState renders a message when no pages are available.
func (m TutorialModel) renderEmptyState() string {
	r := m.theme.Renderer

	style := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(2, 4).
		Width(m.width)

	return style.Render("No tutorial pages available for this context.")
}

// NextPage advances to the next page.
func (m *TutorialModel) NextPage() {
	pages := m.visiblePages()
	if m.currentPage < len(pages)-1 {
		m.currentPage++
		m.scrollOffset = 0
	}
}

// PrevPage goes to the previous page.
func (m *TutorialModel) PrevPage() {
	if m.currentPage > 0 {
		m.currentPage--
		m.scrollOffset = 0
	}
}

// JumpToPage jumps to a specific page index.
func (m *TutorialModel) JumpToPage(index int) {
	pages := m.visiblePages()
	if index >= 0 && index < len(pages) {
		m.currentPage = index
		m.scrollOffset = 0
	}
}

// JumpToSection jumps to the first page in a section.
func (m *TutorialModel) JumpToSection(sectionID string) {
	pages := m.visiblePages()
	for i, page := range pages {
		if page.ID == sectionID || page.Section == sectionID {
			m.currentPage = i
			m.scrollOffset = 0
			return
		}
	}
}

// SetContext sets the current view context for filtering.
func (m *TutorialModel) SetContext(ctx string) {
	m.context = ctx
	// Reset to first page when context changes
	m.currentPage = 0
	m.scrollOffset = 0
}

// SetContextMode enables or disables context-based filtering.
func (m *TutorialModel) SetContextMode(enabled bool) {
	m.contextMode = enabled
	if enabled {
		m.currentPage = 0
		m.scrollOffset = 0
	}
}

// SetSize sets the tutorial dimensions and updates the markdown renderer.
func (m *TutorialModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Update markdown renderer width to match content area
	contentWidth := width - 6 // padding and borders
	if m.tocVisible {
		contentWidth -= 24 // TOC sidebar width
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.markdownRenderer != nil {
		m.markdownRenderer.SetWidthWithTheme(contentWidth, m.theme)
	}
}

// MarkViewed marks a page as viewed.
func (m *TutorialModel) MarkViewed(pageID string) {
	m.progress[pageID] = true
}

// Progress returns the progress map for persistence.
func (m TutorialModel) Progress() map[string]bool {
	return m.progress
}

// SetProgress restores progress from persistence.
func (m *TutorialModel) SetProgress(progress map[string]bool) {
	if progress != nil {
		m.progress = progress
	}
}

// CurrentPageID returns the ID of the current page.
func (m TutorialModel) CurrentPageID() string {
	pages := m.visiblePages()
	if m.currentPage >= 0 && m.currentPage < len(pages) {
		return pages[m.currentPage].ID
	}
	return ""
}

// IsComplete returns true if all pages have been viewed.
func (m TutorialModel) IsComplete() bool {
	pages := m.visiblePages()
	for _, page := range pages {
		if !m.progress[page.ID] {
			return false
		}
	}
	return len(pages) > 0
}

// ShouldClose returns true if user requested to close the tutorial (bv-wdsd).
func (m TutorialModel) ShouldClose() bool {
	return m.shouldClose
}

// ResetClose resets the close flag (call after handling close) (bv-wdsd).
func (m *TutorialModel) ResetClose() {
	m.shouldClose = false
}

// visiblePages returns pages filtered by context if contextMode is enabled.
func (m TutorialModel) visiblePages() []TutorialPage {
	if !m.contextMode || m.context == "" {
		return m.pages
	}

	var filtered []TutorialPage
	for _, page := range m.pages {
		// Include if no context restriction or matches current context
		if len(page.Contexts) == 0 {
			filtered = append(filtered, page)
			continue
		}
		for _, ctx := range page.Contexts {
			if ctx == m.context {
				filtered = append(filtered, page)
				break
			}
		}
	}
	return filtered
}

// CenterTutorial returns the tutorial view centered in the terminal.
func (m TutorialModel) CenterTutorial(termWidth, termHeight int) string {
	tutorial := m.View()

	// Get actual rendered dimensions
	tutorialWidth := lipgloss.Width(tutorial)
	tutorialHeight := lipgloss.Height(tutorial)

	// Calculate padding
	padTop := (termHeight - tutorialHeight) / 2
	padLeft := (termWidth - tutorialWidth) / 2

	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	r := m.theme.Renderer

	centered := r.NewStyle().
		MarginTop(padTop).
		MarginLeft(padLeft).
		Render(tutorial)

	return centered
}

// defaultTutorialPages returns the built-in tutorial content.
// Content organized by section - see bv-kdv2, bv-sbib, bv-36wz, etc.
func defaultTutorialPages() []TutorialPage {
	return []TutorialPage{
		// =============================================================
		// INTRODUCTION & PHILOSOPHY (bv-kdv2)
		// =============================================================
		{
			ID:      "intro-welcome",
			Title:   "Welcome",
			Section: "Introduction",
			Content: introWelcomeContent,
		},
		{
			ID:      "intro-philosophy",
			Title:   "The Beads Philosophy",
			Section: "Introduction",
			Content: introPhilosophyContent,
		},
		{
			ID:      "intro-audience",
			Title:   "Who Is This For?",
			Section: "Introduction",
			Content: introAudienceContent,
		},
		{
			ID:      "intro-quickstart",
			Title:   "Quick Start",
			Section: "Introduction",
			Content: introQuickstartContent,
		},

		// =============================================================
		// CORE CONCEPTS (bv-sbib)
		// =============================================================
		{
			ID:      "concepts-beads",
			Title:   "What Are Beads?",
			Section: "Core Concepts",
			Content: conceptsBeadsContent,
		},
		{
			ID:      "concepts-dependencies",
			Title:   "Dependencies & Blocking",
			Section: "Core Concepts",
			Content: conceptsDependenciesContent,
		},
		{
			ID:      "concepts-labels",
			Title:   "Labels & Organization",
			Section: "Core Concepts",
			Content: conceptsLabelsContent,
		},
		{
			ID:      "concepts-priorities",
			Title:   "Priorities & Status",
			Section: "Core Concepts",
			Content: conceptsPrioritiesContent,
		},
		{
			ID:      "concepts-graph",
			Title:   "The Dependency Graph",
			Section: "Core Concepts",
			Content: conceptsGraphContent,
		},

		// =============================================================
		// VIEWS & NAVIGATION (bv-36wz, bv-wra5, bv-h6jw)
		// =============================================================
		{
			ID:      "views-nav-fundamentals",
			Title:   "Navigation Fundamentals",
			Section: "Views",
			Content: viewsNavFundamentalsContent,
		},
		{
			ID:       "views-list",
			Title:    "List View",
			Section:  "Views",
			Contexts: []string{"list"},
			Content:  viewsListContent,
		},
		{
			ID:       "views-detail",
			Title:    "Detail View",
			Section:  "Views",
			Contexts: []string{"detail"},
			Content:  viewsDetailContent,
		},
		{
			ID:       "views-split",
			Title:    "Split View",
			Section:  "Views",
			Contexts: []string{"split"},
			Content:  viewsSplitContent,
		},
		{
			ID:       "views-board",
			Title:    "Board View",
			Section:  "Views",
			Contexts: []string{"board"},
			Content:  viewsBoardContent,
		},
		{
			ID:       "views-graph",
			Title:    "Graph View",
			Section:  "Views",
			Contexts: []string{"graph"},
			Content:  viewsGraphContent,
		},
		{
			ID:       "views-insights",
			Title:    "Insights Panel",
			Section:  "Views",
			Contexts: []string{"insights"},
			Content:  viewsInsightsContent,
		},
		{
			ID:       "views-history",
			Title:    "History View",
			Section:  "Views",
			Contexts: []string{"history"},
			Content:  viewsHistoryContent,
		},

		// =============================================================
		// ADVANCED FEATURES (placeholder - bv-19gf)
		// =============================================================
		{
			ID:      "advanced-ai",
			Title:   "AI Agent Integration",
			Section: "Advanced",
			Content: `## AI Agent Integration

bv works with **AI coding agents** through robot mode:

` + "```bash\nbv --robot-triage   # Prioritized recommendations\nbv --robot-next     # Top priority item\nbv --robot-plan     # Parallel execution tracks\n```" + `

See ` + "`AGENTS.md`" + ` for the complete AI integration guide.

> More detailed content coming in bv-19gf.`,
		},

		// =============================================================
		// REFERENCE
		// =============================================================
		{
			ID:      "ref-keyboard",
			Title:   "Keyboard Reference",
			Section: "Reference",
			Content: `## Quick Keyboard Reference

### Global
| Key | Action |
|-----|--------|
| **?** | Help overlay |
| **q** | Quit |
| **Esc** | Close/go back |
| **b/g/i/h** | Switch views |

### Navigation
| Key | Action |
|-----|--------|
| **j/k** | Move down/up |
| **h/l** | Move left/right |
| **g/G** | Top/bottom |
| **Enter** | Select |

### Filtering
| Key | Action |
|-----|--------|
| **/** | Fuzzy search |
| **~** | Semantic search |
| **o/c/r/a** | Status filter |

> Press **?** in any view for context help.`,
		},
	}
}

// =============================================================================
// INTRODUCTION & PHILOSOPHY CONTENT (bv-kdv2)
// =============================================================================

// introWelcomeContent is Page 1 of the Introduction section.
const introWelcomeContent = `## Welcome to beads_viewer

` + "```" + `
    ╭──────────────────────────────────────╮
    │      beads_viewer (bv)               │
    │  Issue tracking that lives in code   │
    ╰──────────────────────────────────────╯
` + "```" + `

**The problem:** You're deep in flow, coding away, when you need to check an issue.
You switch to a browser, navigate to your issue tracker, lose context,
and break your concentration.

**The solution:** ` + "`bv`" + ` brings issue tracking *into your terminal*, where you already work.
No browser tabs. No context switching. No cloud dependencies.

### The 30-Second Value Proposition

1. **Issues live in your repo** — version controlled, diffable, greppable
2. **Works offline** — no internet required, no accounts to manage
3. **AI-native** — designed for both humans and coding agents
4. **Zero dependencies** — just a single binary and your git repo

> Press **→** or **Space** to continue.`

// introPhilosophyContent is Page 2 of the Introduction section.
const introPhilosophyContent = `## The Beads Philosophy

Why "beads"? Think of git commits as **beads on a string** — each one a
discrete, meaningful step in your project's history.

Issues are beads too. They're snapshots of work: what needs doing, what's
in progress, what's complete. They belong *with your code*, not in some
external system.

### Core Principles

**1. Issues as First-Class Citizens**
Your ` + "`.beads/`" + ` directory is just as important as your ` + "`src/`" + `.
Issues get the same git treatment as code: branching, merging, history.

**2. No External Dependencies**
No servers to run. No accounts to create. No API keys to manage.
If you have git and a terminal, you have everything you need.

**3. Diffable and Greppable**
Issues are stored as plain JSONL. You can ` + "`git diff`" + ` your backlog.
You can ` + "`grep`" + ` for patterns across all issues.

**4. Human and Agent Readable**
The same data works for both humans (via ` + "`bv`" + `) and AI agents (via ` + "`--robot-*`" + ` flags).

> Press **→** to continue.`

// introAudienceContent is Page 3 of the Introduction section.
const introAudienceContent = `## Who Is This For?

### Solo Developers

Managing personal projects? Keep your TODO lists organized without
the overhead of heavyweight tools. Everything stays in your repo,
backs up with your code, and travels wherever you push.

### Small Teams

Want lightweight issue tracking without the subscription fees?
Share your ` + "`.beads/`" + ` directory through git. Everyone sees the same
state. No sync issues. No "who has the latest?"

### AI Coding Agents

This is where bv shines. AI agents like Claude, Cursor, and Codex
need structured task management. The ` + "`--robot-*`" + ` flags output
machine-readable formats perfect for agent consumption:

` + "```bash\nbv --robot-triage    # What should I work on?\nbv --robot-plan      # How can work be parallelized?\n```" + `

### Anyone Tired of Context-Switching

If you've ever lost your train of thought switching between your
editor and a web-based issue tracker, bv is for you. Stay in the
terminal. Stay in flow.

> Press **→** to continue.`

// introQuickstartContent is Page 4 of the Introduction section.
const introQuickstartContent = `## Quick Start

You're already running ` + "`bv`" + ` — you're ahead of the game!

### Basic Navigation

| Key | Action |
|-----|--------|
| **j / k** | Move down / up |
| **Enter** | Open issue details |
| **Esc** | Close overlay / go back |
| **q** | Quit bv |

### Switching Views

| Key | View |
|-----|------|
| **Esc** | Return to List |
| **b** | Board (Kanban) |
| **g** | Graph (dependencies) |
| **i** | Insights panel |
| **h** | History |

### Getting Help

| Key | What You Get |
|-----|--------------|
| **?** | Quick help overlay |
| **Space** (in help) | This tutorial |
| **` + "`" + `** (backtick) | Jump to tutorial |
| **~** (tilde) | Context-sensitive help |

### Next Steps

Try pressing **t** to see the Table of Contents for this tutorial.
Or press **q** to exit and start exploring!

> **Tip:** Press **?** anytime you need a quick reference.`

// =============================================================================
// CORE CONCEPTS CONTENT (bv-sbib)
// =============================================================================

// conceptsBeadsContent is Page 1 of the Core Concepts section.
const conceptsBeadsContent = `## What Are Beads?

A **bead** is an issue, task, or unit of work in your project. Think of your
project's work as beads on a string — discrete items that together form
the complete picture.

### Anatomy of a Bead

` + "```" + `
┌─────────────────────────────────────────────────────────┐
│ bv-abc123                               ← Unique ID     │
├─────────────────────────────────────────────────────────┤
│ Title: Fix authentication timeout                       │
│ Type: bug                  Status: open                 │
│ Priority: P1               Created: 2025-01-15          │
├─────────────────────────────────────────────────────────┤
│ Labels: auth, security, urgent                          │
├─────────────────────────────────────────────────────────┤
│ Description:                                            │
│ Users report being logged out after 5 minutes...        │
├─────────────────────────────────────────────────────────┤
│ Dependencies:                                           │
│   Blocks: bv-xyz789 (Production deploy)                 │
│   Blocked-by: (none)                                    │
└─────────────────────────────────────────────────────────┘
` + "```" + `

### Issue Types

| Type | When to Use |
|------|-------------|
| **bug** | Something broken that needs fixing |
| **feature** | New functionality to add |
| **task** | General work item |
| **epic** | Large initiative containing sub-tasks |
| **chore** | Maintenance, cleanup, tech debt |
| **docs** | Documentation work |

### How Beads Are Stored

Your issues live in ` + "`.beads/issues.jsonl`" + ` — a simple JSON Lines file:

` + "```" + `json
{"id":"bv-abc123","title":"Fix auth","type":"bug","priority":1,...}
{"id":"bv-def456","title":"Add dark mode","type":"feature",...}
` + "```" + `

This means your issues are:
- **Version controlled** — branch, merge, history
- **Diffable** — see exactly what changed
- **Greppable** — search with standard tools

> Press **→** to continue.`

// conceptsDependenciesContent is Page 2 of the Core Concepts section.
const conceptsDependenciesContent = `## Dependencies & Blocking

Not all work can happen in parallel. Some issues must wait for others.
This is where **dependencies** come in.

### The Relationship

` + "```" + `
    ┌───────────┐         ┌───────────┐
    │  bv-abc1  │ ──────► │  bv-def2  │
    │  (Auth)   │ blocks  │ (Deploy)  │
    └───────────┘         └───────────┘

    bv-abc1 BLOCKS bv-def2
    bv-def2 is BLOCKED BY bv-abc1
` + "```" + `

In plain terms: **You can't deploy until auth is fixed.**

### Visual Indicators

Throughout bv, blocking relationships are shown:

| Indicator | Meaning |
|-----------|---------|
| 🔴 | Blocked — waiting on something else |
| 🟢 | Ready — no blockers, can start now |
| **→** in detail | Shows what this issue blocks |
| **←** in detail | Shows what blocks this issue |

### The "Ready" Filter

Press **r** in List view to filter to **ready** issues only:

` + "```" + `
  Ready = Open + Zero Blockers
` + "```" + `

This is your **actionable work queue**. These issues have no dependencies
blocking them — you can start any of them right now.

> **Tip:** Start your day with ` + "`bd ready`" + ` to see what you can tackle.

### Adding Dependencies

From the command line:

` + "```bash\nbd dep add bv-def2 bv-abc1   # def2 depends on abc1\n```" + `

This creates the blocking relationship shown above.

> Press **→** to continue.`

// conceptsLabelsContent is Page 3 of the Core Concepts section.
const conceptsLabelsContent = `## Labels & Organization

Labels provide **flexible categorization** that cuts across types
and priorities. Use them for anything that doesn't fit elsewhere.

### Common Label Patterns

| Category | Example Labels |
|----------|----------------|
| **Area** | frontend, backend, api, database |
| **Owner** | team-alpha, @alice, contractor |
| **Scope** | mvp, v2, tech-debt, nice-to-have |
| **State** | needs-review, blocked-external, waiting-response |

### Multi-Label Support

Issues can have multiple labels:

` + "```" + `
bv-abc123  [bug] [P1]  auth, security, needs-review
` + "```" + `

This issue is a high-priority auth bug that needs security review.

### Working with Labels

| Key | Action |
|-----|--------|
| **L** | Open label picker (apply labels) |
| **Shift+L** | Filter by label |
| **[** | Switch to Labels dashboard view |

### Label Analytics

The **Labels view** (press **[**) shows:
- Issue count per label
- Health indicators (stale issues, blockers)
- Distribution across priorities

` + "```" + `
┌─────────────────────────────────────────┐
│ Label         │ Open │ In Progress │ %  │
├───────────────┼──────┼─────────────┼────┤
│ frontend      │   12 │           3 │ 28%│
│ backend       │    8 │           5 │ 24%│
│ needs-review  │    6 │           0 │ 11%│
└─────────────────────────────────────────┘
` + "```" + `

> **Tip:** Keep your label set small. Too many labels = no one uses them.

> Press **→** to continue.`

// conceptsPrioritiesContent is Page 4 of the Core Concepts section.
const conceptsPrioritiesContent = `## Priorities & Status

Every issue has a **priority** and a **status**. Together, they answer:
"How important is this?" and "Where is it in the workflow?"

### Priority Levels

| Level | Meaning | Response Time |
|-------|---------|---------------|
| **P0** | Critical/emergency | Drop everything |
| **P1** | High priority | This sprint/week |
| **P2** | Medium priority | This cycle/month |
| **P3** | Low priority | When you have time |
| **P4** | Backlog | Someday/maybe |

> **Guideline:** If everything is P0, nothing is P0.

### Status Flow

` + "```" + `
   ┌──────┐     ┌─────────────┐     ┌────────┐
   │ open │ ──► │ in_progress │ ──► │ closed │
   └──────┘     └─────────────┘     └────────┘
                      │
                      ▼
                ┌─────────┐
                │ blocked │ (auto-detected from deps)
                └─────────┘
` + "```" + `

| Status | When to Use |
|--------|-------------|
| **open** | New or not yet started |
| **in_progress** | Actively being worked |
| **blocked** | Waiting on dependencies |
| **closed** | Complete or won't fix |

### Priority in the UI

The **Insights panel** (press **i**) calculates a priority score:

` + "```" + `
Priority Score = Base Priority + Blocking Factor + Freshness
` + "```" + `

- **Blocking Factor**: How many issues are waiting on this?
- **Freshness**: How long since last update?

This surfaces issues that are both important AND blocking other work.

### Changing Priority/Status

| Key | Action |
|-----|--------|
| **p** | Change priority |
| **s** | Change status |

Or from the command line:

` + "```bash\nbd update bv-abc123 --priority=P1\nbd update bv-abc123 --status=in_progress\n```" + `

> Press **→** to continue.`

// conceptsGraphContent is Page 5 of the Core Concepts section.
const conceptsGraphContent = `## The Dependency Graph

Your issues form a **directed acyclic graph (DAG)**. That sounds complex,
but the concept is simple: work flows in one direction, with no cycles.

### Mental Model

` + "```" + `
                    ┌─────────┐
                    │ bv-001  │  (Epic: User Auth)
                    │  EPIC   │
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
     ┌─────────┐   ┌─────────┐   ┌─────────┐
     │ bv-002  │   │ bv-003  │   │ bv-004  │
     │ Login   │   │ Signup  │   │ Reset   │
     └────┬────┘   └────┬────┘   └─────────┘
          │              │
          ▼              ▼
     ┌─────────┐   ┌─────────┐
     │ bv-005  │   │ bv-006  │
     │ Tests   │   │ Tests   │
     └─────────┘   └─────────┘

     Arrows flow DOWN toward what's blocked.
` + "```" + `

### Key Insights from the Graph

1. **Root nodes** (no arrows in) — Can be started immediately
2. **Leaf nodes** (no arrows out) — Nothing depends on them
3. **High fan-out** — Completing this unblocks many items
4. **Critical path** — The longest chain determines minimum time

### Graph View (Press g)

The **Graph view** visualizes these relationships:

| Visual | Meaning |
|--------|---------|
| Node size | Priority (bigger = higher) |
| Green node | Closed |
| Blue node | In progress |
| Red node | Blocked |
| Arrow A→B | A blocks B |

### Navigation in Graph View

| Key | Action |
|-----|--------|
| **j/k** | Move between nodes vertically |
| **h/l** | Move between siblings |
| **f** | Focus on selected subgraph |
| **Enter** | View selected issue |
| **Esc** | Return to list |

### Why This Matters

The graph reveals:
- **Bottlenecks**: One issue blocking many
- **Parallel tracks**: Independent work streams
- **Priority inversions**: Low-priority blocking high-priority

> **Tip:** Use ` + "`bd blocked`" + ` to quickly see all blocked issues.

> Press **→** to continue to Views & Navigation.`

// =============================================================================
// VIEWS & NAVIGATION CONTENT (bv-36wz)
// =============================================================================

// viewsNavFundamentalsContent is Page 1 of the Views section.
const viewsNavFundamentalsContent = `## Navigation Fundamentals

bv uses **vim-style navigation** throughout. If you know vim, you're already
at home. If not, you'll pick it up in minutes.

### Core Movement

| Key | Action |
|-----|--------|
| **j** | Move down |
| **k** | Move up |
| **h** | Move left (in multi-column views) |
| **l** | Move right (in multi-column views) |

### Jump Commands

| Key | Action |
|-----|--------|
| **g** | Jump to top |
| **G** | Jump to bottom |
| **Ctrl+d** | Half-page down |
| **Ctrl+u** | Half-page up |

### Universal Keys

These work in every view:

| Key | Action |
|-----|--------|
| **?** | Help overlay |
| **Esc** | Close overlay / go back |
| **Enter** | Select / open |
| **q** | Quit bv |

### The Shortcuts Sidebar

Press **;** (semicolon) to toggle a floating sidebar showing all available
shortcuts for your current view. It updates as you navigate.

> Press **→** to continue.`

// viewsListContent is the List View page content.
const viewsListContent = `## List View

The **List view** is your issue inbox — where you'll spend most of your time.

` + "```" + `
┌─────────────────────────────────────────────────────┐
│ bv-abc1  [P1] [bug] Fix login timeout              │ ← selected
│ bv-def2  [P2] [feature] Add dark mode              │
│ bv-ghi3  [P2] [task] Update dependencies           │
│ bv-jkl4  [P3] [chore] Clean up test fixtures       │
└─────────────────────────────────────────────────────┘
` + "```" + `

### Filtering

Quickly narrow down what you see:

| Key | Filter |
|-----|--------|
| **o** | Open issues only |
| **c** | Closed issues only |
| **r** | Ready issues (no blockers) |
| **a** | All issues (reset filter) |

### Searching

| Key | Search Type |
|-----|-------------|
| **/** | Fuzzy search (fast, typo-tolerant) |
| **~** | Semantic search (AI-powered, finds related concepts) |
| **n/N** | Next/previous search result |

### Sorting

Press **s** to cycle through sort modes: priority → created → updated.
Press **S** (shift+s) to reverse the current sort order.

### When to Use List View

- Daily triage: filter to ` + "`r`" + ` (ready) and work top-down
- Quick status check: filter to ` + "`o`" + ` (open) to see backlog size
- Finding specific issues: use **/** or **~** to search

> Press **→** to continue.`

// viewsDetailContent is the Detail View page content.
const viewsDetailContent = `## Detail View

Press **Enter** on any issue to see its full details.

` + "```" + `
┌─────────────────────────────────────────────────────┐
│ bv-abc1: Fix login timeout                         │
├─────────────────────────────────────────────────────┤
│ Status: open          Priority: P1                 │
│ Type: bug             Created: 2025-01-15          │
├─────────────────────────────────────────────────────┤
│                                                     │
│ ## Description                                      │
│                                                     │
│ Users report being logged out after 5 minutes      │
│ of inactivity. Should be 30 minutes per spec.      │
│                                                     │
│ ## Dependencies                                     │
│ Blocks: bv-xyz9 (Deploy to production)             │
│                                                     │
└─────────────────────────────────────────────────────┘
` + "```" + `

### Detail View Actions

| Key | Action |
|-----|--------|
| **O** | Open/edit in external editor |
| **C** | Copy issue ID to clipboard |
| **j/k** | Scroll content up/down |
| **Esc** | Return to list |

### Markdown Rendering

Issue descriptions are rendered with full markdown support:
- Headers, bold, italic, code blocks
- Lists and tables
- Links (displayed but not clickable in terminal)

> Press **→** to continue.`

// viewsSplitContent is the Split View page content.
const viewsSplitContent = `## Split View

Press **Tab** from Detail view to enter Split view — list and detail side by side.

` + "```" + `
┌────────────────────┬────────────────────────────────┐
│ bv-abc1 [P1] bug   │ bv-abc1: Fix login timeout     │
│ bv-def2 [P2] feat  │ ────────────────────────────── │
│ bv-ghi3 [P2] task  │ Status: open    Priority: P1   │
│ bv-jkl4 [P3] chore │                                │
│                    │ ## Description                 │
│                    │ Users report being logged...   │
└────────────────────┴────────────────────────────────┘
` + "```" + `

### Split View Navigation

| Key | Action |
|-----|--------|
| **Tab** | Switch focus between panes |
| **j/k** | Navigate in focused pane |
| **Esc** | Return to full list |

### When to Use Split View

- **Code review**: Quickly scan multiple related issues
- **Triage session**: Read details without losing list context
- **Dependency analysis**: Navigate while viewing relationships

> **Tip:** The detail pane auto-updates as you navigate the list.

> Press **→** to continue.`

// viewsBoardContent is the Board View page content.
const viewsBoardContent = `## Board View

Press **b** to switch to the Kanban-style board.

` + "```" + `
┌─────────────┬─────────────┬─────────────┬─────────────┐
│    OPEN     │ IN PROGRESS │   BLOCKED   │   CLOSED    │
├─────────────┼─────────────┼─────────────┼─────────────┤
│ bv-abc1     │ bv-mno7     │ bv-stu0     │ bv-vwx1     │
│ bv-def2     │             │             │ bv-yza2     │
│ bv-ghi3     │             │             │ bv-bcd3     │
│ bv-jkl4     │             │             │             │
└─────────────┴─────────────┴─────────────┴─────────────┘
` + "```" + `

### Board Navigation

| Key | Action |
|-----|--------|
| **h/l** | Move between columns |
| **j/k** | Move within a column |
| **Enter** | View issue details |
| **m** | Move issue to different status |

### Visual Indicators

- Card height indicates description length
- Priority shown with color intensity
- Blocked issues appear in the BLOCKED column automatically

### When to Use Board View

- **Sprint planning**: Visualize work distribution
- **Standups**: Quick status overview
- **Bottleneck detection**: Spot column imbalances

> Press **→** to continue.`

// viewsGraphContent is the Graph View page content.
const viewsGraphContent = `## Graph View

Press **g** to visualize issue dependencies as a graph.

` + "```" + `
                    ┌─────────┐
                    │ bv-abc1 │
                    └────┬────┘
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
         ┌─────────┐ ┌─────────┐ ┌─────────┐
         │ bv-def2 │ │ bv-ghi3 │ │ bv-jkl4 │
         └────┬────┘ └─────────┘ └────┬────┘
              │                       │
              ▼                       ▼
         ┌─────────┐            ┌─────────┐
         │ bv-mno5 │            │ bv-pqr6 │
         └─────────┘            └─────────┘
` + "```" + `

### Reading the Graph

- **Arrows point TO dependencies** (A → B means A *blocks* B)
- **Node size** reflects priority
- **Color** indicates status (green=closed, blue=in_progress, etc.)
- **Highlighted node** is your current selection

### Graph Navigation

| Key | Action |
|-----|--------|
| **j/k** | Navigate between connected nodes |
| **h/l** | Navigate siblings |
| **Enter** | Select node and view details |
| **f** | Focus: show only this node's subgraph |
| **Esc** | Exit focus / return to list |

### When to Use Graph View

- **Critical path analysis**: Find what's blocking important work
- **Dependency planning**: Understand execution order
- **Impact assessment**: See what closing an issue unblocks

> Press **→** to continue.`

// viewsInsightsContent is the Insights Panel page content.
const viewsInsightsContent = `## Insights Panel

Press **i** to open the Insights panel — AI-powered prioritization assistance.

### Priority Score Algorithm

Each issue gets a computed **priority score** based on:

1. **Explicit priority** (P0-P4)
2. **Blocking factor** — how many issues it unblocks
3. **Freshness** — recently updated issues score higher
4. **Type weight** — bugs often prioritized over features

### Attention Scores

The panel highlights issues that may need attention:

- **Stale issues**: Open for too long without updates
- **Blocked chains**: Issues creating bottlenecks
- **Priority inversions**: Low-priority items blocking high-priority

### Visual Heatmap

Press **m** to toggle heatmap mode, which colors the list by:
- Red = high attention needed
- Yellow = moderate
- Green = on track

### When to Use Insights

- **Weekly review**: Find neglected issues
- **Sprint planning**: Data-driven prioritization
- **Bottleneck hunting**: Identify blocking patterns

> Press **→** to continue.`

// viewsHistoryContent is the History View page content.
const viewsHistoryContent = `## History View

Press **h** to see the git-integrated timeline of your project.

` + "```" + `
┌─────────────────────────────────────────────────────┐
│ 2025-01-15 14:32  abc1234  feat: Add login flow    │
│   └─ bv-abc1 opened, bv-def2 closed                │
│                                                     │
│ 2025-01-15 10:15  def5678  fix: Timeout issue      │
│   └─ bv-ghi3 status → in_progress                  │
│                                                     │
│ 2025-01-14 16:45  ghi9012  chore: Bump deps        │
│   └─ (no bead changes)                             │
└─────────────────────────────────────────────────────┘
` + "```" + `

### History Features

- **Git commits** with associated bead changes
- **Bead-only changes** from ` + "`bd`" + ` commands
- **Time-travel preview**: See project state at any point

### History Navigation

| Key | Action |
|-----|--------|
| **j/k** | Navigate timeline |
| **Enter** | Preview project state at that commit |
| **d** | Show diff for selected commit |
| **Esc** | Return to current state |

### Time Travel

When you press **Enter** on a historical commit, bv shows you:
- What issues existed at that moment
- Their status at that time
- The dependency graph as it was

This is read-only — you're viewing the past, not changing it.

> **Use case:** "What was our backlog like before the big refactor?"`
