package ui

import (
	"fmt"
	"sort"
	
	"beads_viewer/pkg/model"
	"github.com/charmbracelet/lipgloss"
)

type BoardModel struct {
	columns [4][]model.Issue
	focusedCol int
	selectedRow [4]int // Store selection for each column
	ready bool
	width int
	height int
}

func NewBoardModel(issues []model.Issue) BoardModel {
	var cols [4][]model.Issue
	
	// Distribute issues
	for _, i := range issues {
		switch i.Status {
		case model.StatusOpen:
			cols[0] = append(cols[0], i)
		case model.StatusInProgress:
			cols[1] = append(cols[1], i)
		case model.StatusBlocked:
			cols[2] = append(cols[2], i)
		case model.StatusClosed:
			cols[3] = append(cols[3], i)
		}
	}
	
	// Sort each column by priority then date
	sortFunc := func(list []model.Issue) {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Priority != list[j].Priority {
				return list[i].Priority < list[j].Priority
			}
			return list[i].CreatedAt.After(list[j].CreatedAt)
		})
	}
	
	for i := 0; i < 4; i++ {
		sortFunc(cols[i])
	}

	return BoardModel{
		columns: cols,
		focusedCol: 0,
	}
}

func (b *BoardModel) MoveDown() {
	count := len(b.columns[b.focusedCol])
	if count == 0 { return }
	if b.selectedRow[b.focusedCol] < count - 1 {
		b.selectedRow[b.focusedCol]++
	}
}

func (b *BoardModel) MoveUp() {
	if b.selectedRow[b.focusedCol] > 0 {
		b.selectedRow[b.focusedCol]--
	}
}

func (b *BoardModel) MoveRight() {
	if b.focusedCol < 3 {
		b.focusedCol++
	}
}

func (b *BoardModel) MoveLeft() {
	if b.focusedCol > 0 {
		b.focusedCol--
	}
}

func (b *BoardModel) SelectedIssue() *model.Issue {
	col := b.columns[b.focusedCol]
	row := b.selectedRow[b.focusedCol]
	if len(col) > 0 && row < len(col) {
		return &col[row]
	}
	return nil
}

// View renders the board
func (b BoardModel) View(width, height int) string {
	colWidth := (width - 6) / 4 // -6 for borders/gaps
	if colWidth < 20 { colWidth = 20 } // Min width
	
colHeight := height - 2 // Header
	
	var renderedCols []string
	titles := []string{"OPEN", "IN PROGRESS", "BLOCKED", "CLOSED"}
	colors := []lipgloss.Color{ColorStatusOpen, ColorStatusInProgress, ColorStatusBlocked, ColorStatusClosed}
	
	for i := 0; i < 4; i++ {
		isFocused := b.focusedCol == i
		
		// Header
		headerStyle := lipgloss.NewStyle().
			Width(colWidth).
			Align(lipgloss.Center).
			Background(colors[i]).
			Foreground(ColorBg).
			Bold(true)
			
		if !isFocused {
			headerStyle = headerStyle.Background(ColorBgDark).Foreground(colors[i])
		}
		
		header := headerStyle.Render(titles[i])
		
		// Rows
		var rows []string
		start := 0 
		// Simple scrolling logic: keep selected in view
		// If row > height-header, offset start
		// Very rudimentary scrolling
		visibleRows := colHeight - 2
		if visibleRows < 1 { visibleRows = 1 }
		
		sel := b.selectedRow[i]
		if sel >= len(b.columns[i]) && len(b.columns[i]) > 0 { sel = len(b.columns[i]) - 1 }
		
		if sel >= visibleRows {
			start = sel - visibleRows + 1
		}
		
		end := start + visibleRows
		if end > len(b.columns[i]) { end = len(b.columns[i]) }
		
		for r := start; r < end; r++ {
			issue := b.columns[i][r]
			
			style := lipgloss.NewStyle().
				Width(colWidth).
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(ColorBg)
				
			if isFocused && r == sel {
				style = style.Background(ColorBgHighlight).BorderForeground(ColorPrimary)
			}
			
			icon, iconColor := GetTypeIcon(string(issue.IssueType))
			prio := GetPriorityIcon(issue.Priority)
			
			// Content:
			// 🐛 P0
			// Title...
			
			line1 := fmt.Sprintf("%s %s %s", 
				lipgloss.NewStyle().Foreground(iconColor).Render(icon),
				lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(issue.ID),
				prio,
			)
			
			line2 := lipgloss.NewStyle().Foreground(ColorText).Render(truncate(issue.Title, colWidth-4))
			
			rows = append(rows, style.Render(line1 + "\n" + line2))
		}
		
		// Fill rest
		content := lipgloss.JoinVertical(lipgloss.Left, rows...)
		colStyle := lipgloss.NewStyle().
			Width(colWidth).
			Height(colHeight).
			Border(lipgloss.RoundedBorder())			
		if isFocused {
			colStyle = colStyle.BorderForeground(ColorPrimary)
		} else {
			colStyle = colStyle.BorderForeground(ColorSecondary)
		}
		
		renderedCols = append(renderedCols, lipgloss.JoinVertical(lipgloss.Center, header, colStyle.Render(content)))
	}
	
	return lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)
}

func truncate(s string, w int) string {
	if len(s) > w {
		return s[:w-1] + "…"
	}
	return s
}
