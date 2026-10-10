package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/b9s/pkg/version"
)

func withBuildInfo(t *testing.T, info version.Info) {
	t.Helper()
	prev := buildInfo
	buildInfo = func() version.Info { return info }
	t.Cleanup(func() { buildInfo = prev })
}

func TestHelpOverlayShowsVersionAndLinkedCommit(t *testing.T) {
	sha := "0704a3472717dec01f2f3543317e06e4525dbb92"
	withBuildInfo(t, version.Info{Version: "v1.3.3", Commit: sha})
	updated, _ := NewModel(nil, "").Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	m := updated.(Model)

	raw := m.renderHelpOverlay()
	help := stripANSI(raw)

	if !strings.Contains(help, "b9s v1.3.3") || !strings.Contains(help, "0704a347") {
		t.Errorf("help overlay does not show the version and short commit:\n%s", help)
	}
	link := "\x1b]8;;https://github.com/vanderheijden86/b9s/commit/" + sha + "\x1b\\"
	if !strings.Contains(raw, link) {
		t.Errorf("help overlay does not link the commit to GitHub with OSC 8")
	}
}

func TestHelpOverlayVersionLineWithoutCommit(t *testing.T) {
	withBuildInfo(t, version.Info{Version: "dev"})
	updated, _ := NewModel(nil, "").Update(tea.WindowSizeMsg{Width: 180, Height: 60})
	m := updated.(Model)

	raw := m.renderHelpOverlay()
	if !strings.Contains(stripANSI(raw), "b9s dev") {
		t.Errorf("help overlay does not show the dev version")
	}
	if strings.Contains(raw, "\x1b]8;;https://github.com") {
		t.Errorf("help overlay links a commit it does not know")
	}
}
