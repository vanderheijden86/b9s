package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/model"
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

func TestMarkdownRendererMermaidFlowchartDirections(t *testing.T) {
	for _, header := range []string{"graph TD", "flowchart LR"} {
		t.Run(header, func(t *testing.T) {
			out, err := NewMarkdownRenderer(80).Render("```mermaid\n" + header + "\nA[Start] --> B[Done]\n```\n")
			if err != nil {
				t.Fatal(err)
			}
			plain := stripANSI(out)
			if !strings.Contains(plain, "┌") || !strings.Contains(plain, "Start") || !strings.Contains(plain, "Done") {
				t.Fatalf("flowchart did not render: %q", plain)
			}
			if strings.Contains(plain, "graph TD") || strings.Contains(plain, "flowchart LR") {
				t.Fatalf("rendered diagram retained source: %q", plain)
			}
		})
	}
}

func TestMarkdownRendererMermaidSequence(t *testing.T) {
	out, err := NewMarkdownRenderer(80).Render("```mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n```")
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "┌") || !strings.Contains(plain, "Hello") || strings.Contains(plain, "sequenceDiagram") {
		t.Fatalf("sequence did not render: %q", plain)
	}
}

func TestMarkdownRendererMermaidFallbacks(t *testing.T) {
	for name, source := range map[string]string{
		"unsupported":    "classDiagram\nA <|-- B",
		"bad header":     "graph WRONG\nA --> B",
		"malformed body": "sequenceDiagram\nAlice->>Bob",
		"missing target": "graph LR\nA -->",
	} {
		t.Run(name, func(t *testing.T) {
			out, err := NewMarkdownRenderer(80).Render("```mermaid\n" + source + "\n```")
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(source, "\n") {
				if !strings.Contains(stripANSI(out), line) {
					t.Fatalf("source line %q was not retained: %q", line, stripANSI(out))
				}
			}
		})
	}
}

func TestMarkdownRendererMultipleMermaidBlocks(t *testing.T) {
	source := "Before\n\n```mermaid\ngraph LR\nA --> B\n```\n\nBetween\n\n```mermaid\nsequenceDiagram\nA->>B: Hi\n```\n\nAfter"
	out, err := NewMarkdownRenderer(80).Render(source)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	for _, want := range []string{"Before", "Between", "After", "┌", "Hi"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in %q", want, plain)
		}
	}
	if strings.Contains(plain, "sequenceDiagram") || strings.Contains(plain, "graph LR") {
		t.Fatalf("source retained in %q", plain)
	}
}

func TestMarkdownRendererMermaidWidth(t *testing.T) {
	source := "sequenceDiagram\nAlice->>Bob: Hello"
	diagram, err := renderMermaidDiagram(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	width := lipgloss.Width(diagram)
	markdown := "```mermaid\n" + source + "\n```"
	for _, paneWidth := range []int{width, width + 5} {
		out, err := NewMarkdownRenderer(paneWidth).Render(markdown)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stripANSI(out), "┌") {
			t.Fatalf("diagram should fit width %d: %q", paneWidth, stripANSI(out))
		}
	}
	out, err := NewMarkdownRenderer(width - 1).Render(markdown)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "sequenceDiagram") || !strings.Contains(plain, "diagram too wide") || strings.Contains(plain, "┌") {
		t.Fatalf("expected width fallback: %q", plain)
	}
}

func TestMarkdownRendererMermaidResize(t *testing.T) {
	source := "sequenceDiagram\nAlice->>Bob: Hello"
	diagram, err := renderMermaidDiagram(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	width := lipgloss.Width(diagram)
	markdown := "```mermaid\n" + source + "\n```"
	mr := NewMarkdownRenderer(width - 1)
	before, err := mr.Render(markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(before), "diagram too wide") {
		t.Fatalf("expected narrow fallback: %q", stripANSI(before))
	}
	mr.SetWidth(width)
	after, err := mr.Render(markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stripANSI(after), "┌") {
		t.Fatalf("expected diagram after resize: %q", stripANSI(after))
	}
}

func TestMarkdownRendererMermaidNarrowHintFits(t *testing.T) {
	source := "```mermaid\nsequenceDiagram\nAlice->>Bob: Hello\n```"
	out, err := NewMarkdownRenderer(20).Render(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "wide") && lipgloss.Width(line) > 20 {
			t.Fatalf("hint exceeds pane width: %q", line)
		}
	}
}

func TestMarkdownRendererDoesNotRenderMermaidInsideAnotherFence(t *testing.T) {
	source := "```text\n    ```\n```mermaid\ngraph LR\nA --> B\n```\n```"
	out, err := NewMarkdownRenderer(80).Render(source)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if strings.Contains(plain, "┌") || !strings.Contains(plain, "graph LR") {
		t.Fatalf("literal Mermaid source was rendered: %q", plain)
	}
}

func TestMarkdownRendererUnsupportedGraphStatementFallsBack(t *testing.T) {
	source := "```mermaid\ngraph LR\nA --> B\nclick A https://example.org\n```"
	out, err := NewMarkdownRenderer(80).Render(source)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if strings.Contains(plain, "┌") || !strings.Contains(plain, "click A") {
		t.Fatalf("unsupported statement changed meaning: %q", plain)
	}
}

func TestMarkdownRendererKeepsListContext(t *testing.T) {
	source := "1. First\n\n   ```mermaid\n   graph LR\n   A --> B\n   ```\n\n2. Second"
	mr := NewMarkdownRenderer(80)
	out, err := mr.Render(source)
	if err != nil {
		t.Fatal(err)
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "┌") || !strings.Contains(plain, "2. Second") {
		t.Fatalf("list context lost: %q", plain)
	}
	if !strings.Contains(plain, "\n   ┌") {
		t.Fatalf("nested diagram lost list indentation: %q", plain)
	}
}

func TestMarkdownRendererMermaidCopyKeepsSource(t *testing.T) {
	source := "```mermaid\ngraph LR\nA --> B\n```"
	issue := model.Issue{ID: "bd-diagram", Title: "Diagram", Description: source}
	if copied := formatIssueMarkdown(issue, nil); !strings.Contains(copied, source) {
		t.Fatalf("copy lost Mermaid source: %q", copied)
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
