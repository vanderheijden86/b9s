package ui

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestModelStylePlaceholder(t *testing.T) {
	// Placeholder to keep file non-empty after reverting footer-style experiments.
}

func TestTreeFooterShowsManualRefreshShortcut(t *testing.T) {
	m := NewModel([]model.Issue{{ID: "test-1", Title: "Test", Status: model.StatusOpen}}, "")
	m.width = 200

	footer := stripANSI(m.renderFooter())
	if !strings.Contains(footer, "^R:refresh") {
		t.Fatalf("expected tree footer to expose manual refresh shortcut, got %q", footer)
	}
}

func TestModelChromeRemovesTerminalControlPayloads(t *testing.T) {
	m := NewModel(nil, "")
	m.width = 120
	m.height = 20
	m.activeProjectName = "safe\x1b]52;c;YXR0YWNr\x07project"
	m.statusMsg = "safe\x1b[2Jstatus"
	m.issueConfirm = issueConfirmation{id: "bd-1\x1b[2J", title: "title\x1b]52;c;YXR0YWNr\x07"}

	out := m.renderGlobalHeader() + m.renderFooter() + m.renderIssueConfirm()
	for _, forbidden := range []string{"\x1b]52", "YXR0YWNr", "[2J"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("model chrome retained terminal payload %q: %q", forbidden, out)
		}
	}
}
