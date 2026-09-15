package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func sanitizeTerminalText(s string) string {
	var clean strings.Builder
	clean.Grow(len(s))

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			i = skipEscapeSequence(s, i)
			continue
		}
		if s[i] >= 0x80 && s[i] <= 0x9f {
			switch s[i] {
			case 0x9b:
				i = skipCSISequence(s, i+1)
			case 0x90, 0x98, 0x9d, 0x9e, 0x9f:
				i = skipStringControl(s, i+1)
			default:
				i++
			}
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\u009b':
			i = skipCSISequence(s, i+size)
			continue
		case r == '\u0090' || r == '\u0098' || r == '\u009d' || r == '\u009e' || r == '\u009f':
			i = skipStringControl(s, i+size)
			continue
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			if r == '\n' || r == '\t' {
				clean.WriteRune(r)
			}
			i += size
			continue
		default:
			clean.WriteString(s[i : i+size])
			i += size
		}
	}

	return clean.String()
}

func sanitizeTerminalLine(s string) string {
	clean := sanitizeTerminalText(s)
	clean = strings.ReplaceAll(clean, "\n", " ")
	return strings.ReplaceAll(clean, "\t", " ")
}

func sanitizeIssueForTerminal(issue model.Issue) model.Issue {
	clean := issue.Clone()
	clean.ID = sanitizeTerminalLine(clean.ID)
	clean.Title = sanitizeTerminalLine(clean.Title)
	clean.Description = sanitizeTerminalText(clean.Description)
	clean.Design = sanitizeTerminalText(clean.Design)
	clean.AcceptanceCriteria = sanitizeTerminalText(clean.AcceptanceCriteria)
	clean.Notes = sanitizeTerminalText(clean.Notes)
	clean.Status = model.Status(sanitizeTerminalLine(string(clean.Status)))
	clean.IssueType = model.IssueType(sanitizeTerminalLine(string(clean.IssueType)))
	clean.Assignee = sanitizeTerminalLine(clean.Assignee)
	clean.SourceRepo = sanitizeTerminalLine(clean.SourceRepo)
	if clean.ExternalRef != nil {
		value := sanitizeTerminalLine(*clean.ExternalRef)
		clean.ExternalRef = &value
	}
	for i := range clean.Labels {
		clean.Labels[i] = sanitizeTerminalLine(clean.Labels[i])
	}
	for _, dependency := range clean.Dependencies {
		if dependency == nil {
			continue
		}
		dependency.IssueID = sanitizeTerminalLine(dependency.IssueID)
		dependency.DependsOnID = sanitizeTerminalLine(dependency.DependsOnID)
		dependency.Type = model.DependencyType(sanitizeTerminalLine(string(dependency.Type)))
		dependency.CreatedBy = sanitizeTerminalLine(dependency.CreatedBy)
	}
	for _, comment := range clean.Comments {
		if comment == nil {
			continue
		}
		comment.ID = sanitizeTerminalLine(comment.ID)
		comment.IssueID = sanitizeTerminalLine(comment.IssueID)
		comment.Author = sanitizeTerminalLine(comment.Author)
		comment.Text = sanitizeTerminalText(comment.Text)
	}
	return clean
}

func skipEscapeSequence(s string, start int) int {
	i := start + 1
	if i >= len(s) {
		return i
	}
	switch s[i] {
	case '[':
		return skipCSISequence(s, i+1)
	case ']', 'P', 'X', '^', '_':
		return skipStringControl(s, i+1)
	}
	for i < len(s) {
		c := s[i]
		i++
		if c >= 0x30 && c <= 0x7e {
			break
		}
	}
	return i
}

func skipCSISequence(s string, start int) int {
	for i := start; i < len(s); i++ {
		if s[i] >= 0x40 && s[i] <= 0x7e {
			return i + 1
		}
	}
	return len(s)
}

func skipStringControl(s string, start int) int {
	for i := start; i < len(s); {
		if s[i] == 0x07 {
			return i + 1
		}
		if s[i] == 0x9c {
			return i + 1
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '\u009c' {
			return i + size
		}
		i += size
	}
	return len(s)
}

// FormatTimeRel returns a relative time string (e.g., "2h ago", "3d ago")
func FormatTimeRel(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	d := time.Since(t)
	if d < 0 {
		// Future timestamps treated as now
		return "now"
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}

// truncateRunesHelper truncates a string to max visual width (cells), adding suffix if needed.
// Uses go-runewidth to handle wide characters correctly.
func truncateRunesHelper(s string, maxWidth int, suffix string) string {
	if maxWidth <= 0 {
		return ""
	}

	width := runewidth.StringWidth(s)
	if width <= maxWidth {
		return s
	}

	suffixWidth := runewidth.StringWidth(suffix)
	if suffixWidth > maxWidth {
		// Even suffix is too wide, truncate suffix
		return runewidth.Truncate(suffix, maxWidth, "")
	}

	targetWidth := maxWidth - suffixWidth
	return runewidth.Truncate(s, targetWidth, "") + suffix
}

// padRight pads string s with spaces on the right to reach visual width.
// Uses go-runewidth to handle wide characters (emojis, CJK) correctly,
// consistent with truncateRunesHelper which also uses visual width.
// padRight pads a string to the given visual width using plain spaces.
// Uses lipgloss.Width for ANSI-safe measurement (bd-qyr).
func padRight(s string, width int) string {
	visualWidth := lipgloss.Width(s)
	if visualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visualWidth)
}

// truncate truncates string s to maxRunes
func truncate(s string, maxRunes int) string {
	return truncateRunesHelper(s, maxRunes, "…")
}

// DependencyNode represents a visual node in the dependency tree
type DependencyNode struct {
	ID       string
	Title    string
	Status   string
	Type     string // "root", "blocks", "related", etc.
	Children []*DependencyNode
}

// BuildDependencyTree constructs a tree from dependencies for visualization.
// maxDepth limits recursion to prevent infinite loops and performance issues.
// Set maxDepth to 0 for unlimited depth (use with caution).
func BuildDependencyTree(rootID string, issueMap map[string]*model.Issue, maxDepth int) *DependencyNode {
	visited := make(map[string]bool)
	return buildTreeRecursive(rootID, issueMap, "root", visited, 0, maxDepth)
}

func buildTreeRecursive(id string, issueMap map[string]*model.Issue, depType string, visited map[string]bool, depth, maxDepth int) *DependencyNode {
	// Check depth limit (0 = unlimited)
	if maxDepth > 0 && depth > maxDepth {
		return nil
	}

	// Cycle detection
	if visited[id] {
		return &DependencyNode{
			ID:     id,
			Title:  "(cycle)",
			Status: "?",
			Type:   depType,
		}
	}

	issue, exists := issueMap[id]
	if !exists {
		return &DependencyNode{
			ID:     id,
			Title:  "(not found)",
			Status: "?",
			Type:   depType,
		}
	}

	visited[id] = true
	defer func() { visited[id] = false }() // Allow revisiting in different branches

	node := &DependencyNode{
		ID:     sanitizeTerminalLine(issue.ID),
		Title:  sanitizeTerminalLine(issue.Title),
		Status: sanitizeTerminalLine(string(issue.Status)),
		Type:   sanitizeTerminalLine(depType),
	}

	// Recursively add children (dependencies)
	for _, dep := range issue.Dependencies {
		childNode := buildTreeRecursive(dep.DependsOnID, issueMap, string(dep.Type), visited, depth+1, maxDepth)
		if childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}

	return node
}

// RenderDependencyTree renders a dependency tree as a formatted string
func RenderDependencyTree(node *DependencyNode) string {
	if node == nil {
		return "No dependency data."
	}

	var sb strings.Builder
	sb.WriteString("Dependency Graph:\n")
	renderTreeNode(&sb, node, "", true, true) // isRoot=true for root node
	return sb.String()
}

func renderTreeNode(sb *strings.Builder, node *DependencyNode, prefix string, isLast bool, isRoot bool) {
	if node == nil {
		return
	}

	// Determine the connector
	var connector string
	if isRoot {
		connector = "" // Root has no connector
	} else if isLast {
		connector = "└── "
	} else {
		connector = "├── "
	}

	// Get icons
	status := sanitizeTerminalLine(node.Status)
	depType := sanitizeTerminalLine(node.Type)
	id := sanitizeTerminalLine(node.ID)
	statusIcon := GetStatusIcon(status)
	typeIcon := getDepTypeIcon(depType)

	// Truncate title if too long (UTF-8 safe)
	title := truncateRunesHelper(sanitizeTerminalLine(node.Title), 40, "...")

	// Render this node
	sb.WriteString(fmt.Sprintf("%s%s%s %s %s %s (%s) [%s]\n",
		prefix,
		connector,
		statusIcon,
		typeIcon,
		id,
		title,
		status,
		depType,
	))

	// Calculate prefix for children
	var childPrefix string
	if isRoot {
		childPrefix = "" // Children of root start with no prefix
	} else if isLast {
		childPrefix = prefix + "    "
	} else {
		childPrefix = prefix + "│   "
	}

	// Render children
	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		renderTreeNode(sb, child, childPrefix, isChildLast, false) // isRoot=false for children
	}
}

func getDepTypeIcon(depType string) string {
	switch depType {
	case "root":
		return "📍"
	case "blocks":
		return "⛔"
	case "related":
		return "🔗"
	case "parent-child":
		return "📦"
	case "discovered-from":
		return "🔍"
	default:
		return "•"
	}
}

// GetStatusIcon returns a colored icon for a status
func GetStatusIcon(s string) string {
	switch s {
	case "open":
		return "🟢"
	case "in_progress":
		return "🔵"
	case "blocked":
		return "🔴"
	case "closed":
		return "⚫"
	default:
		return "⚪"
	}
}

// GetPriorityIcon returns the emoji for a priority level
func GetPriorityIcon(priority int) string {
	switch priority {
	case 0:
		return "🔥" // Critical
	case 1:
		return "⚡" // High
	case 2:
		return "🔹" // Medium
	case 3:
		return "☕" // Low
	case 4:
		return "💤" // Backlog
	default:
		return "  "
	}
}

// GetPriorityLabel returns a compact text label for priority (P0, P1, etc.)
func GetPriorityLabel(priority int) string {
	if priority >= 0 && priority <= 4 {
		return fmt.Sprintf("P%d", priority)
	}
	return "P?"
}

// GetAgeDays returns the number of days since the given time
func GetAgeDays(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(time.Since(t).Hours() / 24)
}

// GetAgeColor returns a color based on staleness:
// green (<7 days), yellow (7-30 days), red (>30 days)
func GetAgeColor(t time.Time) lipgloss.AdaptiveColor {
	days := GetAgeDays(t)
	switch {
	case days < 7:
		return ColorSuccess // Green - fresh
	case days < 30:
		return ColorWarning // Yellow/Orange - aging
	default:
		return ColorDanger // Red - stale
	}
}

// FormatAgeBadge returns a compact age string with timer emoji (e.g., "3d ⏱")
func FormatAgeBadge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	days := GetAgeDays(t)
	switch {
	case days == 0:
		return "<1d"
	case days < 7:
		return fmt.Sprintf("%dd", days)
	case days < 30:
		return fmt.Sprintf("%dw", days/7)
	default:
		return fmt.Sprintf("%dmo", days/30)
	}
}

// shortIDSuffix returns the short suffix of an issue ID, stripping the project prefix.
// For example, "agents-config-oa9" returns "oa9", "bd-3eh" returns "3eh".
// If the ID has no separator, returns the full ID.
func shortIDSuffix(id string) string {
	if idx := strings.LastIndex(id, "-"); idx >= 0 && idx < len(id)-1 {
		return id[idx+1:]
	}
	return id
}
