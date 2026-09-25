package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// detailUpdateNotice carries a pending self-update into the detail pane.
type detailUpdateNotice struct {
	Tag string
}

// detailRelation is one row of the RELATIONS section.
type detailRelation struct {
	label string
	arrow string
	id    string
	issue *model.Issue
}

// renderIssueDetail draws the detail pane: a lipgloss header in the style of
// the board cards, then the free-text sections through glamour. The clipboard
// copy keeps formatIssueMarkdown, because markdown is what gets pasted.
func renderIssueDetail(item model.Issue, issueMap map[string]*model.Issue, t Theme, width int, md *MarkdownRenderer, update *detailUpdateNotice) string {
	if width < 20 {
		width = 20
	}
	rawID := item.ID
	item = sanitizeIssueForTerminal(item)
	r := t.Renderer
	muted := r.NewStyle().Foreground(t.Muted)
	wrap := r.NewStyle().Width(width)

	var blocks []string

	if update != nil {
		tag := sanitizeTerminalLine(update.Tag)
		blocks = append(blocks, wrap.Render(
			r.NewStyle().Foreground(t.Primary).Render("↑ "+tag+" available")+
				muted.Render(" · press U to update")))
	}

	// The header card: type, ID and age, the title, then status, priority
	// and people, framed in the status color like a board card.
	statusColor := detailStatusColor(t, item.Status)
	inner := width - 4
	icon, iconColor := t.GetTypeIcon(string(item.IssueType))
	typeName := string(item.IssueType)
	if typeName == "" {
		typeName = "issue"
	}
	left := r.NewStyle().Foreground(iconColor).Render(icon+" "+typeName) +
		muted.Render(" · ") +
		r.NewStyle().Bold(true).Foreground(t.Secondary).Render(item.ID)
	right := ""
	if !item.UpdatedAt.IsZero() {
		right = r.NewStyle().Foreground(getAgeColor(item.UpdatedAt)).Render("updated " + FormatTimeRel(item.UpdatedAt))
	}
	head := left
	if gap := inner - lipgloss.Width(left) - lipgloss.Width(right); right != "" && gap >= 2 {
		head = left + strings.Repeat(" ", gap) + right
	}
	title := r.NewStyle().Bold(true).Foreground(t.Base.GetForeground()).Render(item.Title)

	chip := r.NewStyle().Bold(true).Padding(0, 1).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Background(statusColor).
		Render(strings.ToUpper(strings.ReplaceAll(string(item.Status), "_", " ")))
	prioStyle := r.NewStyle().Bold(true).Foreground(t.Secondary)
	if item.Priority <= 1 {
		prioStyle = prioStyle.Foreground(lipgloss.AdaptiveColor{Light: "#c62828", Dark: "#ef5350"})
	}
	people := muted.Render("creator ") + detailPerson(r, t, item.CreatedBy, "unknown") +
		muted.Render("  assignee ") + detailPerson(r, t, item.Assignee, "none")
	meta := chip + "  " + prioStyle.Render(formatPriority(item.Priority)) + "  " + people

	card := r.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(statusColor).
		Padding(0, 1).
		Width(width - 2)
	blocks = append(blocks, card.Render(strings.Join([]string{head, title, "", meta}, "\n")))

	facts := []string{}
	if !item.CreatedAt.IsZero() {
		facts = append(facts, "created "+item.CreatedAt.Format("2006-01-02"))
	}
	if item.Owner != "" {
		facts = append(facts, item.Owner)
	}
	if len(facts) > 0 {
		blocks = append(blocks, wrap.Render(muted.Render(strings.Join(facts, " · "))))
	}
	if item.DeferUntil != nil {
		blocks = append(blocks, wrap.Render(r.NewStyle().Foreground(t.Deferred).Render("deferred until "+item.DeferUntil.Format("2006-01-02 15:04"))))
	}
	if len(item.Labels) > 0 {
		tags := make([]string, len(item.Labels))
		for i, l := range item.Labels {
			tags[i] = r.NewStyle().Foreground(t.Primary).Render("#" + sanitizeTerminalLine(l))
		}
		blocks = append(blocks, wrap.Render(strings.Join(tags, " ")))
	}
	blocks = append(blocks, "")

	section := func(label string) string {
		return r.NewStyle().Bold(true).Foreground(t.Muted).Render(label)
	}
	markdownSection := func(label, body string) {
		if strings.TrimSpace(body) == "" {
			return
		}
		rendered := body
		if md != nil {
			if out, err := md.Render(body); err == nil {
				rendered = strings.Trim(out, "\n")
			}
		}
		blocks = append(blocks, section(label), rendered, "")
	}
	markdownSection("DESCRIPTION", item.Description)
	markdownSection("DESIGN", item.Design)
	markdownSection("ACCEPTANCE", item.AcceptanceCriteria)
	markdownSection("NOTES", item.Notes)

	rels, children := detailRelations(rawID, item, issueMap)
	if len(children) > 0 {
		done := 0
		for _, c := range children {
			if c.issue != nil && c.issue.Status == model.StatusClosed {
				done++
			}
		}
		label := fmt.Sprintf("CHILDREN · %d/%d", done, len(children))
		barW := min(30, width-lipgloss.Width(label)-2)
		line := section(label)
		if barW >= 4 {
			line += "  " + r.NewStyle().Foreground(t.Open).Render(progressBar(done, len(children), barW))
		}
		blocks = append(blocks, line)
		idWidth := 0
		for _, c := range children {
			idWidth = max(idWidth, lipgloss.Width(detailShortID(c.id)))
		}
		idWidth = min(idWidth, width/3)
		for _, c := range children {
			blocks = append(blocks, renderDetailRelation(r, t, c, width, idWidth, false))
		}
		blocks = append(blocks, "")
	}
	if len(rels) > 0 {
		blocks = append(blocks, section(fmt.Sprintf("RELATIONS · %d", len(rels))))
		idWidth := 0
		for _, rel := range rels {
			idWidth = max(idWidth, lipgloss.Width(detailShortID(rel.id)))
		}
		idWidth = min(idWidth, width/3)
		for _, rel := range rels {
			blocks = append(blocks, renderDetailRelation(r, t, rel, width, idWidth, true))
		}
		blocks = append(blocks, "")
	}

	if len(item.Comments) > 0 {
		blocks = append(blocks, section(fmt.Sprintf("COMMENTS · %d", len(item.Comments))))
		body := r.NewStyle().Width(width - 2).PaddingLeft(2)
		for _, c := range item.Comments {
			if c == nil {
				continue
			}
			head := r.NewStyle().Bold(true).Foreground(t.Primary).Render(sanitizeTerminalLine(c.Author)) +
				muted.Render(" · "+FormatTimeRel(c.CreatedAt))
			blocks = append(blocks, wrap.Render(head), body.Render(sanitizeTerminalText(c.Text)), "")
		}
	}

	return strings.TrimRight(strings.Join(blocks, "\n"), "\n") + "\n"
}

func detailPerson(r *lipgloss.Renderer, t Theme, name, empty string) string {
	if name == "" {
		return r.NewStyle().Foreground(t.Muted).Italic(true).Render(empty)
	}
	return r.NewStyle().Foreground(t.Base.GetForeground()).Render("@" + name)
}

func detailStatusColor(t Theme, s model.Status) lipgloss.AdaptiveColor {
	switch s {
	case model.StatusDeferred:
		return t.Deferred
	case model.StatusPinned:
		return t.Pinned
	case model.StatusHooked:
		return t.Hooked
	case model.StatusTombstone:
		return t.Tombstone
	}
	return t.GetStatusColor(string(s))
}

// detailRelations lists the issue's own dependencies and the issues that
// point back at it, so children and dependents show without opening them.
// Children come back apart from the other relations: they get their own
// section with a progress bar.
func detailRelations(rawID string, item model.Issue, issueMap map[string]*model.Issue) (rels, children []detailRelation) {
	var parents, blockedBy, related, blocks []detailRelation
	for _, dep := range item.Dependencies {
		if dep == nil {
			continue
		}
		rel := detailRelation{id: dep.DependsOnID, issue: issueMap[dep.DependsOnID]}
		switch dep.Type {
		case model.DepParentChild:
			rel.arrow, rel.label = "↑", "parent"
			parents = append(parents, rel)
		case model.DepBlocks:
			rel.arrow, rel.label = "⊘", "blocked by"
			blockedBy = append(blockedBy, rel)
		case model.DepDiscoveredFrom:
			rel.arrow, rel.label = "↖", "found in"
			related = append(related, rel)
		default:
			rel.arrow, rel.label = "~", "related"
			related = append(related, rel)
		}
	}
	for id, other := range issueMap {
		if other == nil || id == rawID {
			continue
		}
		for _, dep := range other.Dependencies {
			if dep == nil || dep.DependsOnID != rawID {
				continue
			}
			rel := detailRelation{id: id, issue: other}
			switch dep.Type {
			case model.DepParentChild:
				rel.arrow, rel.label = "↓", "child"
				children = append(children, rel)
			case model.DepBlocks:
				rel.arrow, rel.label = "→", "blocks"
				blocks = append(blocks, rel)
			}
		}
	}
	byID := func(rs []detailRelation) []detailRelation {
		sort.Slice(rs, func(i, j int) bool { return rs[i].id < rs[j].id })
		return rs
	}
	out := append(parents, blockedBy...)
	out = append(out, byID(blocks)...)
	return append(out, related...), byID(children)
}

// detailShortID drops the project prefix, as the board cards do.
func detailShortID(id string) string {
	id = sanitizeTerminalLine(id)
	if prefix := ExtractRepoPrefix(id); prefix != "" && strings.HasPrefix(id, prefix+"-") {
		return strings.TrimPrefix(id, prefix+"-")
	}
	return id
}

// renderDetailRelation draws one row: the relation label when withLabel is
// set, then the short ID, the title and the status on the right edge.
func renderDetailRelation(r *lipgloss.Renderer, t Theme, rel detailRelation, width, idWidth int, withLabel bool) string {
	muted := r.NewStyle().Foreground(t.Muted)
	label := ""
	if withLabel {
		label = fmt.Sprintf("%s %-10s ", rel.arrow, rel.label)
	}
	idWidth = max(min(idWidth, width-lipgloss.Width(label)-1), 4)
	id := truncateRunesHelper(detailShortID(rel.id), idWidth, "…")
	id += strings.Repeat(" ", max(idWidth-lipgloss.Width(id), 0))
	head := muted.Render(label) + r.NewStyle().Bold(true).Foreground(t.Secondary).Render(id) + "  "

	status, title := "", ""
	if rel.issue != nil {
		st := sanitizeTerminalLine(string(rel.issue.Status))
		status = r.NewStyle().Foreground(detailStatusColor(t, rel.issue.Status)).Render("● " + strings.ReplaceAll(st, "_", " "))
		title = sanitizeTerminalLine(rel.issue.Title)
	}
	room := width - lipgloss.Width(head) - lipgloss.Width(status) - 1
	if room < 4 {
		// Too narrow for a title beside the status: keep the ID and status.
		status, title, room = "", "", 0
		if rel.issue != nil {
			status = r.NewStyle().Foreground(detailStatusColor(t, rel.issue.Status)).Render("●")
		}
	}
	title = truncateRunesHelper(title, room, "…")
	pad := width - lipgloss.Width(head) - lipgloss.Width(title) - lipgloss.Width(status)
	if pad < 1 {
		pad = 1
	}
	return head + title + strings.Repeat(" ", pad) + status
}
