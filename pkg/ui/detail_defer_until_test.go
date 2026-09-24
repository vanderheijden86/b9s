package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

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
