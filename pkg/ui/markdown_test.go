package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestNewMarkdownRenderer(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	if mr == nil {
		t.Fatal("NewMarkdownRenderer returned nil")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
	if mr.useTheme {
		t.Error("expected useTheme to be false for NewMarkdownRenderer")
	}
	if mr.theme != nil {
		t.Error("expected theme to be nil for NewMarkdownRenderer")
	}
}

func TestNewMarkdownRendererWithTheme(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	mr := NewMarkdownRendererWithTheme(80, theme)
	if mr == nil {
		t.Fatal("NewMarkdownRendererWithTheme returned nil")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("expected useTheme to be true for NewMarkdownRendererWithTheme")
	}
	if mr.theme == nil {
		t.Error("expected theme to be stored")
	}
}

func TestMarkdownRenderer_Render(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	result, err := mr.Render("# Hello\n\nWorld")
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	// Should contain "Hello" somewhere in the rendered output
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected result to contain 'Hello', got: %s", result)
	}
}

func TestMarkdownRenderer_RenderRemovesTerminalControlPayloads(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	result, err := mr.Render("safe\x1b]52;c;YXR0YWNr\x07text\u009b31mred")
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	for _, forbidden := range []string{"\x1b]52", "YXR0YWNr", "[31m"} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("Render retained terminal control payload %q: %q", forbidden, result)
		}
	}
}

func TestSanitizeTerminalText_StripsCSIAndOSC(t *testing.T) {
	input := "safe\x1b[31mred\x1b[0m\x1b]52;c;YXR0YWNr\x07tail"
	want := "saferedtail"
	if got := sanitizeTerminalText(input); got != want {
		t.Fatalf("sanitizeTerminalText() = %q, want %q", got, want)
	}
}

func TestSanitizeTerminalText_StripsC0AndC1Controls(t *testing.T) {
	input := "one\x00two\rthree\u0085four\u009b31mred"
	want := "onetwothreefourred"
	if got := sanitizeTerminalText(input); got != want {
		t.Fatalf("sanitizeTerminalText() = %q, want %q", got, want)
	}
}

func TestSanitizeTerminalText_StripsSingleByteC1Sequence(t *testing.T) {
	input := string([]byte{'s', 'a', 'f', 'e', 0x9b, '3', '1', 'm', 'r', 'e', 'd'})
	if got := sanitizeTerminalText(input); got != "safered" {
		t.Fatalf("sanitizeTerminalText() = %q, want %q", got, "safered")
	}
}

func TestSanitizeTerminalText_PreservesUnicodeAndLayout(t *testing.T) {
	input := "日本語\nsecond\tcolumn"
	if got := sanitizeTerminalText(input); got != input {
		t.Fatalf("sanitizeTerminalText() = %q, want %q", got, input)
	}
}

func TestSanitizeTerminalLine_RemovesInjectedRows(t *testing.T) {
	input := "first\nsecond\tcolumn"
	want := "first second column"
	if got := sanitizeTerminalLine(input); got != want {
		t.Fatalf("sanitizeTerminalLine() = %q, want %q", got, want)
	}
}

func TestSanitizeIssueForTerminal_SanitizesDependencies(t *testing.T) {
	issue := model.Issue{Dependencies: []*model.Dependency{{
		IssueID:     "bd-1\x1b[2J",
		DependsOnID: "bd-2\x1b]52;c;YXR0YWNr\x07",
		Type:        model.DependencyType("blocks\u009b31m"),
		CreatedBy:   "agent\nforged",
	}}}

	clean := sanitizeIssueForTerminal(issue)
	dep := clean.Dependencies[0]
	for _, value := range []string{dep.IssueID, dep.DependsOnID, string(dep.Type), dep.CreatedBy} {
		for _, forbidden := range []string{"\x1b", "YXR0YWNr", "[31m", "\n"} {
			if strings.Contains(value, forbidden) {
				t.Fatalf("dependency retained terminal payload %q: %q", forbidden, value)
			}
		}
	}
}

func TestMarkdownRenderer_RenderNilRenderer(t *testing.T) {
	mr := &MarkdownRenderer{
		renderer: nil,
		width:    80,
	}
	result, err := mr.Render("# Test")
	if err != nil {
		t.Fatalf("Render with nil renderer should not error: %v", err)
	}
	if result != "# Test" {
		t.Errorf("expected raw markdown when renderer is nil, got: %s", result)
	}
}

func TestMarkdownRenderer_SetWidth(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	originalRenderer := mr.renderer

	// Same width should not recreate renderer
	mr.SetWidth(80)
	if mr.renderer != originalRenderer {
		t.Error("SetWidth with same width should not recreate renderer")
	}

	// Invalid width should not change anything
	mr.SetWidth(0)
	if mr.width != 80 {
		t.Error("SetWidth with 0 should not change width")
	}
	mr.SetWidth(-1)
	if mr.width != 80 {
		t.Error("SetWidth with negative should not change width")
	}

	// Different width should update
	mr.SetWidth(100)
	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
}

func TestMarkdownRenderer_SetWidthPreservesTheme(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	mr := NewMarkdownRendererWithTheme(80, theme)

	if !mr.useTheme {
		t.Fatal("expected useTheme to be true")
	}

	// SetWidth should preserve theme
	mr.SetWidth(100)
	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("SetWidth should preserve useTheme flag")
	}
	if mr.theme == nil {
		t.Error("SetWidth should preserve theme")
	}
}

func TestMarkdownRenderer_SetWidthWithTheme(t *testing.T) {
	mr := NewMarkdownRenderer(80)

	if mr.useTheme {
		t.Fatal("expected useTheme to be false initially")
	}

	theme := DefaultTheme(lipgloss.DefaultRenderer())
	mr.SetWidthWithTheme(100, theme)

	if mr.width != 100 {
		t.Errorf("expected width 100, got %d", mr.width)
	}
	if !mr.useTheme {
		t.Error("SetWidthWithTheme should set useTheme to true")
	}
	if mr.theme == nil {
		t.Error("SetWidthWithTheme should store theme")
	}
}

func TestMarkdownRenderer_SetWidthWithThemeSameWidth(t *testing.T) {
	// SetWidthWithTheme should allow updating theme even with same width
	theme := DefaultTheme(lipgloss.DefaultRenderer())
	mr := NewMarkdownRendererWithTheme(80, theme)

	originalRenderer := mr.renderer

	// Same width but (conceptually) different theme should recreate renderer
	mr.SetWidthWithTheme(80, theme)

	// Renderer should be recreated (different instance)
	if mr.renderer == originalRenderer {
		t.Error("SetWidthWithTheme with same width should still recreate renderer")
	}
	if mr.width != 80 {
		t.Errorf("expected width 80, got %d", mr.width)
	}
}

func TestMarkdownRenderer_SetWidthWithThemeInvalidWidth(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	originalRenderer := mr.renderer

	mr.SetWidthWithTheme(0, DefaultTheme(lipgloss.DefaultRenderer()))
	if mr.width != 80 {
		t.Error("SetWidthWithTheme with width 0 should not change width")
	}
	if mr.renderer != originalRenderer {
		t.Error("SetWidthWithTheme with width 0 should not change renderer")
	}

	mr.SetWidthWithTheme(-1, DefaultTheme(lipgloss.DefaultRenderer()))
	if mr.width != 80 {
		t.Error("SetWidthWithTheme with negative width should not change width")
	}
}

func TestMarkdownRenderer_IsDarkMode(t *testing.T) {
	mr := NewMarkdownRenderer(80)
	// Just verify it returns a boolean without panicking
	_ = mr.IsDarkMode()
}

func TestExtractHex(t *testing.T) {
	ac := lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#000000"}

	lightHex := extractHex(ac, false)
	if lightHex != "#ffffff" {
		t.Errorf("expected #ffffff for light mode, got %s", lightHex)
	}

	darkHex := extractHex(ac, true)
	if darkHex != "#000000" {
		t.Errorf("expected #000000 for dark mode, got %s", darkHex)
	}
}

func TestBuildStyleFromTheme(t *testing.T) {
	theme := DefaultTheme(lipgloss.DefaultRenderer())

	// Test dark mode
	darkConfig := buildStyleFromTheme(theme, true)
	if darkConfig.Document.Color == nil {
		t.Error("expected Document.Color to be set")
	}
	if *darkConfig.Document.Color != "#f8f8f2" {
		t.Errorf("expected dark mode doc color #f8f8f2, got %s", *darkConfig.Document.Color)
	}
	// Dark mode background should be nil (transparent) to avoid Solarized/16-color
	// terminal issues where hex colors get downconverted to wrong ANSI slots (#101)
	if darkConfig.Document.BackgroundColor != nil {
		t.Errorf("expected dark mode BackgroundColor to be nil (transparent), got %v", *darkConfig.Document.BackgroundColor)
	}

	// Test light mode
	lightConfig := buildStyleFromTheme(theme, false)
	if *lightConfig.Document.Color != "#000000" {
		t.Errorf("expected light mode doc color #000000, got %s", *lightConfig.Document.Color)
	}
	// Light mode should have nil background (use terminal default)
	if lightConfig.Document.BackgroundColor != nil {
		t.Errorf("expected light mode BackgroundColor to be nil, got %v", lightConfig.Document.BackgroundColor)
	}
}
