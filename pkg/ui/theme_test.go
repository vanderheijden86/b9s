package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestDefaultTheme(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	if theme.Renderer != renderer {
		t.Error("DefaultTheme renderer mismatch")
	}
	// Check a few known colors are set (not zero value)
	if isColorEmpty(theme.Primary) {
		t.Error("DefaultTheme Primary color is empty")
	}
	if isColorEmpty(theme.Open) {
		t.Error("DefaultTheme Open color is empty")
	}
}

func TestThemeModeCycle(t *testing.T) {
	if nextThemeMode("auto") != "light" || nextThemeMode("light") != "dark" || nextThemeMode("dark") != "auto" {
		t.Fatal("theme mode must cycle Automatic, Sepia and Dracula")
	}
}

func TestTerminalSequenceForThemeMode(t *testing.T) {
	if got := terminalSequenceForThemeMode("light", true); got != "\x1b]11;#E9DFCB\x07" {
		t.Fatalf("forced Sepia on dark terminal = %q", got)
	}
	if got := terminalSequenceForThemeMode("dark", false); got != "\x1b]11;#282A36\x07" {
		t.Fatalf("forced Dracula on light terminal = %q", got)
	}
	if got := terminalSequenceForThemeMode("auto", true); got != "\x1b]111\x07" {
		t.Fatalf("automatic dark must restore terminal default, got %q", got)
	}
}

func TestAutomaticDarkDoesNotTouchPaperUntilThemeChanges(t *testing.T) {
	originalGlobalDark := lipgloss.HasDarkBackground()
	defer lipgloss.SetHasDarkBackground(originalGlobalDark)
	m := Model{theme: DefaultTheme(lipgloss.NewRenderer(nil)), originalDark: true, paperTouched: new(bool)}
	m.appConfig.UI.Theme = config.ThemeAuto
	if got := m.TerminalPaperSequence(); got != "" {
		t.Fatalf("initial automatic dark changed paper: %q", got)
	}
	m.applyThemeMode(config.ThemeLight)
	m.applyThemeMode(config.ThemeAuto)
	if got := m.TerminalPaperSequence(); got != "\x1b]111\x07" || !m.PaperTouched() {
		t.Fatalf("returning to automatic dark did not restore paper: %q", got)
	}
}

func TestThemeShortcutDoesNotReplaceUnreadableConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := config.ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const original = "ui: [invalid\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel(nil, "").WithConfig(config.DefaultConfig(), "", "").WithConfigLoadError(errors.New("bad yaml"))
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlT})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("unreadable config was replaced: %q", data)
	}
}

func isColorEmpty(c lipgloss.AdaptiveColor) bool {
	return c.Light == "" && c.Dark == ""
}

func TestGetStatusColor(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	tests := []struct {
		status string
		want   lipgloss.AdaptiveColor
	}{
		{"open", theme.Open},
		{"in_progress", theme.InProgress},
		{"blocked", theme.Blocked},
		{"closed", theme.Closed},
		{"unknown", theme.Subtext},
		{"", theme.Subtext},
	}

	for _, tt := range tests {
		got := theme.GetStatusColor(tt.status)
		if got != tt.want {
			t.Errorf("GetStatusColor(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestGetTypeIcon(t *testing.T) {
	renderer := lipgloss.NewRenderer(nil)
	theme := DefaultTheme(renderer)

	tests := []struct {
		typ      string
		wantIcon string
		wantCol  lipgloss.AdaptiveColor
	}{
		{"bug", "●", theme.Bug},
		{"feature", "▲", theme.Feature},
		{"task", "✔", theme.Task},
		{"epic", "♦", theme.Epic},
		{"chore", "○", theme.Chore},
		{"unknown", "·", theme.Subtext},
	}

	for _, tt := range tests {
		icon, col := theme.GetTypeIcon(tt.typ)
		if icon != tt.wantIcon {
			t.Errorf("GetTypeIcon(%q) icon = %q, want %q", tt.typ, icon, tt.wantIcon)
		}
		if col != tt.wantCol {
			t.Errorf("GetTypeIcon(%q) color = %v, want %v", tt.typ, col, tt.wantCol)
		}
	}
}

// ── Color profile detection tests (bd-2rih) ─────────────────────────────

func TestColorProfile_Detection(t *testing.T) {
	// TermProfile is set at init(); just verify it's a valid value
	valid := map[colorprofile.Profile]bool{
		colorprofile.Unknown:   true,
		colorprofile.NoTTY:     true,
		colorprofile.ASCII:     true,
		colorprofile.ANSI:      true,
		colorprofile.ANSI256:   true,
		colorprofile.TrueColor: true,
	}
	if !valid[TermProfile] {
		t.Errorf("TermProfile has unexpected value: %d", TermProfile)
	}
}

func TestThemeBg_TrueColor(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.TrueColor

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); ok {
		t.Error("ThemeBg should return hex color in TrueColor mode, got NoColor")
	}
}

func TestThemeBg_ANSI(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); !ok {
		t.Errorf("ThemeBg should return NoColor in ANSI mode, got %T", got)
	}
}

func TestThemeBg_ANSI256(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI256

	got := ThemeBg("#282A36")
	if _, ok := got.(lipgloss.NoColor); !ok {
		t.Errorf("ThemeBg should return NoColor in ANSI256 mode (only TrueColor gets hex bg), got %T", got)
	}
}

func TestThemeFg_TrueColor(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.TrueColor

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); ok {
		t.Error("ThemeFg should return hex color in TrueColor mode, got ANSIColor")
	}
}

func TestThemeFg_ANSI256(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI256

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); ok {
		t.Error("ThemeFg should return hex color in ANSI256 mode, got ANSIColor")
	}
}

func TestThemeFg_ANSI(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.ANSI

	got := ThemeFg("#FF6B6B")
	ansiColor, ok := got.(lipgloss.ANSIColor)
	if !ok {
		t.Errorf("ThemeFg should return ANSIColor in ANSI mode, got %T", got)
	} else if ansiColor != 7 {
		t.Errorf("ThemeFg should return ANSI white (7) in ANSI mode, got %d", ansiColor)
	}
}

func TestThemeFg_NoTTY(t *testing.T) {
	saved := TermProfile
	defer func() { TermProfile = saved }()

	TermProfile = colorprofile.NoTTY

	got := ThemeFg("#FF6B6B")
	if _, ok := got.(lipgloss.ANSIColor); !ok {
		t.Errorf("ThemeFg should return ANSIColor in NoTTY mode, got %T", got)
	}
}
