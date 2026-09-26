package web

import (
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// convertIssues builds the browser's issues, deriving blocked and ready with
// the rules of the TUI's counts and its r filter: an issue is blocked by its
// status or by a blocking dependency on an issue that is not closed.
func convertIssues(issues []model.Issue) []Issue {
	status := make(map[string]model.Status, len(issues))
	for i := range issues {
		status[issues[i].ID] = issues[i].Status
	}

	out := make([]Issue, len(issues))
	for i := range issues {
		src := &issues[i]
		dst := Issue{
			ID:             src.ID,
			Title:          src.Title,
			Description:    src.Description,
			Design:         src.Design,
			Acceptance:     src.AcceptanceCriteria,
			Notes:          src.Notes,
			Status:         string(src.Status),
			Priority:       src.Priority,
			Type:           string(src.IssueType),
			Assignee:       src.Assignee,
			CreatedBy:      src.CreatedBy,
			CreatedAt:      formatTime(src.CreatedAt),
			UpdatedAt:      formatTime(src.UpdatedAt),
			ClosedAt:       formatTimePtr(src.ClosedAt),
			DeferUntil:     formatTimePtr(src.DeferUntil),
			Labels:         nonNil(src.Labels),
			BlockedBy:      []string{},
			Related:        []string{},
			DiscoveredFrom: []string{},
			Comments:       make([]Comment, 0, len(src.Comments)),
			Project:        src.SourceRepo,
			ClosedLike:     ui.IsClosedLike(src.Status),
		}
		if dst.Project == "" || dst.Project == "." {
			dst.Project = ui.ExtractRepoPrefix(src.ID)
		}

		openBlocker := false
		for _, dep := range src.Dependencies {
			if dep == nil || dep.DependsOnID == "" {
				continue
			}
			switch {
			case dep.Type == model.DepParentChild:
				if dst.Parent == "" {
					dst.Parent = dep.DependsOnID
				}
			case dep.Type.IsBlocking():
				dst.BlockedBy = append(dst.BlockedBy, dep.DependsOnID)
				if s, ok := status[dep.DependsOnID]; ok && !ui.IsClosedLike(s) {
					openBlocker = true
				}
			case dep.Type == model.DepRelated:
				dst.Related = append(dst.Related, dep.DependsOnID)
			case dep.Type == model.DepDiscoveredFrom:
				dst.DiscoveredFrom = append(dst.DiscoveredFrom, dep.DependsOnID)
			}
		}
		for _, c := range src.Comments {
			if c == nil {
				continue
			}
			dst.Comments = append(dst.Comments, Comment{Author: c.Author, Text: c.Text, CreatedAt: formatTime(c.CreatedAt)})
		}

		if !dst.ClosedLike {
			dst.Blocked = src.Status == model.StatusBlocked || openBlocker
			dst.Ready = !dst.Blocked
		}
		out[i] = dst
	}
	return out
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}
