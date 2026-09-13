package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// GraphModel renders the blocking neighborhood around one issue.
type GraphModel struct {
	issues       []model.Issue
	issueMap     map[string]*model.Issue
	blockers     map[string][]string
	dependents   map[string][]string
	sortedIDs    []string
	selectedIdx  int
	scrollOffset int
	theme        Theme
}

func NewGraphModel(issues []model.Issue, theme Theme) GraphModel {
	g := GraphModel{theme: theme}
	g.SetIssues(issues)
	return g
}

// SetIssues rebuilds the dependency indexes while preserving selection.
func (g *GraphModel) SetIssues(issues []model.Issue) {
	selectedID := ""
	if selected := g.SelectedIssue(); selected != nil {
		selectedID = selected.ID
	}

	g.issues = issues
	g.issueMap = make(map[string]*model.Issue, len(issues))
	g.blockers = make(map[string][]string, len(issues))
	g.dependents = make(map[string][]string, len(issues))
	g.sortedIDs = make([]string, 0, len(issues))

	for i := range g.issues {
		issue := &g.issues[i]
		g.issueMap[issue.ID] = issue
		g.sortedIDs = append(g.sortedIDs, issue.ID)
	}
	for i := range g.issues {
		issue := &g.issues[i]
		for _, dep := range issue.Dependencies {
			if dep == nil || !dep.Type.IsBlocking() {
				continue
			}
			g.blockers[issue.ID] = appendUnique(g.blockers[issue.ID], dep.DependsOnID)
			g.dependents[dep.DependsOnID] = appendUnique(g.dependents[dep.DependsOnID], issue.ID)
		}
	}

	sort.Strings(g.sortedIDs)
	for id := range g.blockers {
		sort.Strings(g.blockers[id])
	}
	for id := range g.dependents {
		sort.Strings(g.dependents[id])
	}

	if selectedID != "" && g.SelectIssue(selectedID) {
		return
	}
	if g.selectedIdx >= len(g.sortedIDs) {
		g.selectedIdx = 0
	}
	if g.selectedIdx < 0 {
		g.selectedIdx = 0
	}
}

func appendUnique(ids []string, id string) []string {
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func (g *GraphModel) SelectIssue(id string) bool {
	for i, candidate := range g.sortedIDs {
		if candidate == id {
			g.selectedIdx = i
			g.ensureVisible(1)
			return true
		}
	}
	return false
}

func (g *GraphModel) SelectedIssue() *model.Issue {
	if g.selectedIdx < 0 || g.selectedIdx >= len(g.sortedIDs) {
		return nil
	}
	return g.issueMap[g.sortedIDs[g.selectedIdx]]
}

func (g *GraphModel) MoveUp() {
	if g.selectedIdx > 0 {
		g.selectedIdx--
	}
}

func (g *GraphModel) MoveDown() {
	if g.selectedIdx < len(g.sortedIDs)-1 {
		g.selectedIdx++
	}
}

func (g *GraphModel) JumpToTop() {
	g.selectedIdx = 0
}

func (g *GraphModel) JumpToBottom() {
	if len(g.sortedIDs) > 0 {
		g.selectedIdx = len(g.sortedIDs) - 1
	}
}

func (g *GraphModel) PageUp() {
	g.selectedIdx -= 10
	if g.selectedIdx < 0 {
		g.selectedIdx = 0
	}
}

func (g *GraphModel) PageDown() {
	g.selectedIdx += 10
	if g.selectedIdx >= len(g.sortedIDs) {
		g.selectedIdx = len(g.sortedIDs) - 1
	}
	if g.selectedIdx < 0 {
		g.selectedIdx = 0
	}
}

func (g *GraphModel) ensureVisible(visibleRows int) {
	if visibleRows < 1 {
		visibleRows = 1
	}
	if g.selectedIdx < g.scrollOffset {
		g.scrollOffset = g.selectedIdx
	}
	if g.selectedIdx >= g.scrollOffset+visibleRows {
		g.scrollOffset = g.selectedIdx - visibleRows + 1
	}
	if g.scrollOffset < 0 {
		g.scrollOffset = 0
	}
}

func (g *GraphModel) View(width, height int) string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	if len(g.sortedIDs) == 0 {
		return g.theme.Renderer.NewStyle().Width(width).Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(g.theme.Secondary).
			Render("No dependency data")
	}

	selected := g.SelectedIssue()
	if selected == nil {
		return "Unable to select an issue"
	}
	if width < 80 {
		return g.renderNeighborhood(selected, width)
	}

	listWidth := 30
	if width >= 120 {
		listWidth = 36
	}
	detailWidth := width - listWidth - 1
	left := g.renderNodeList(listWidth, height)
	right := g.renderNeighborhood(selected, detailWidth)
	separator := g.theme.Renderer.NewStyle().Foreground(g.theme.Border).
		Render(strings.Repeat("│\n", max(1, height-1)) + "│")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, separator, right)
}

func (g *GraphModel) renderNodeList(width, height int) string {
	visibleRows := height - 3
	if visibleRows < 1 {
		visibleRows = 1
	}
	g.ensureVisible(visibleRows)
	end := min(len(g.sortedIDs), g.scrollOffset+visibleRows)

	lines := []string{
		g.theme.Renderer.NewStyle().Bold(true).Foreground(g.theme.Primary).Width(width).
			Render(fmt.Sprintf("Dependency Graph (%d)", len(g.sortedIDs))),
		g.theme.Renderer.NewStyle().Foreground(g.theme.Border).Render(strings.Repeat("─", width)),
	}
	for i := g.scrollOffset; i < end; i++ {
		issue := g.issueMap[g.sortedIDs[i]]
		if issue == nil {
			continue
		}
		line := fmt.Sprintf("%s %s", GetStatusIcon(string(issue.Status)), issue.ID)
		style := g.theme.Renderer.NewStyle().Width(width)
		if i == g.selectedIdx {
			style = style.Bold(true).Foreground(g.theme.Primary).Background(g.theme.Highlight)
		} else {
			style = style.Foreground(g.theme.Secondary)
		}
		lines = append(lines, style.Render(truncateRunesHelper(line, width, "…")))
	}
	if len(g.sortedIDs) > visibleRows {
		lines = append(lines, g.theme.Renderer.NewStyle().Foreground(g.theme.Muted).
			Render(fmt.Sprintf("%d-%d of %d", g.scrollOffset+1, end, len(g.sortedIDs))))
	}
	return strings.Join(lines, "\n")
}

func (g *GraphModel) renderNeighborhood(issue *model.Issue, width int) string {
	if width < 1 {
		width = 1
	}
	var lines []string
	heading := g.theme.Renderer.NewStyle().Bold(true).Foreground(g.theme.Primary)
	muted := g.theme.Renderer.NewStyle().Foreground(g.theme.Secondary)

	lines = append(lines, heading.Render("Dependency Graph"))
	lines = append(lines, heading.Render(fmt.Sprintf("%s %s", GetStatusIcon(string(issue.Status)), issue.ID)))
	lines = append(lines, muted.Render(truncateRunesHelper(issue.Title, max(1, width-2), "…")), "")
	lines = append(lines, g.renderRelations("BLOCKED BY", g.blockers[issue.ID], width)...)
	lines = append(lines, "")
	lines = append(lines, g.renderRelations("BLOCKS", g.dependents[issue.ID], width)...)
	lines = append(lines, "", muted.Render("j/k navigate  enter details  esc return"))
	return strings.Join(lines, "\n")
}

func (g *GraphModel) renderRelations(label string, ids []string, width int) []string {
	lines := []string{g.theme.Renderer.NewStyle().Bold(true).Foreground(g.theme.Feature).
		Render(fmt.Sprintf("%s (%d)", label, len(ids)))}
	if len(ids) == 0 {
		return append(lines, g.theme.Renderer.NewStyle().Foreground(g.theme.Muted).Render("  none"))
	}
	for _, id := range ids {
		issue := g.issueMap[id]
		if issue == nil {
			lines = append(lines, g.theme.Renderer.NewStyle().Foreground(g.theme.Muted).
				Render(truncateRunesHelper("  ? "+id+" (not loaded)", width, "…")))
			continue
		}
		line := fmt.Sprintf("  %s %s  %s", GetStatusIcon(string(issue.Status)), id, issue.Title)
		lines = append(lines, g.theme.Renderer.NewStyle().Foreground(g.theme.Secondary).
			Render(truncateRunesHelper(line, width, "…")))
	}
	return lines
}
