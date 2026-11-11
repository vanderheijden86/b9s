package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestDetailSchedulingDates(t *testing.T) {
	date := time.Date(2026, 10, 20, 23, 45, 0, 0, time.FixedZone("source", -7*3600))
	for _, field := range []string{"due", "deferred"} {
		t.Run(field, func(t *testing.T) {
			issue := model.Issue{ID: "x-1", Title: "Scheduled issue", Status: model.StatusDeferred}
			label, markdownLabel := "due ", "**Due:** "
			if field == "due" {
				issue.DueDate = &date
			} else {
				issue.DeferUntil = &date
				label, markdownLabel = "deferred until ", "**Deferred until:** "
			}
			want := date.Local().Format("2006-01-02 15:04")
			theme := DefaultTheme(lipgloss.NewRenderer(nil))
			out := stripANSI(renderIssueDetail(issue, nil, nil, theme, 70, nil, nil, nil, false))
			if !strings.Contains(out, label+want) {
				t.Errorf("detail missing local scheduling date %q:\n%s", label+want, out)
			}
			if md := formatIssueMarkdown(issue, nil); !strings.Contains(md, markdownLabel+want) {
				t.Errorf("copied Markdown missing local scheduling date %q:\n%s", markdownLabel+want, md)
			}
		})
	}
}

func TestDetailOmitsUnsetSchedulingDates(t *testing.T) {
	issue := model.Issue{ID: "x-1", Title: "Unscheduled issue", Status: model.StatusDeferred}
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	out := stripANSI(renderIssueDetail(issue, nil, nil, theme, 70, nil, nil, nil, false))
	for _, label := range []string{"due ", "deferred until "} {
		if strings.Contains(out, label) {
			t.Errorf("detail shows unset scheduling field %q:\n%s", label, out)
		}
	}
	if md := formatIssueMarkdown(issue, nil); strings.Contains(md, "**Due:**") {
		t.Errorf("copied Markdown shows an unset due date:\n%s", md)
	}
}

func TestFormatIssueMarkdownShowsDeferUntil(t *testing.T) {
	until := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	issue := model.Issue{ID: "x-1", Title: "t", Status: model.StatusDeferred, DeferUntil: &until}

	md := formatIssueMarkdown(issue, nil)

	if !strings.Contains(md, "Deferred until") || !strings.Contains(md, "2026-10-01") {
		t.Errorf("detail markdown lacks the defer-until date:\n%s", md)
	}
}

func TestFormatIssueMarkdownOmitsDeferUntilWhenUnset(t *testing.T) {
	issue := model.Issue{ID: "x-1", Title: "t", Status: model.StatusOpen}

	md := formatIssueMarkdown(issue, nil)

	if strings.Contains(md, "Deferred until") {
		t.Errorf("detail markdown shows a defer-until line for an issue without one:\n%s", md)
	}
}
