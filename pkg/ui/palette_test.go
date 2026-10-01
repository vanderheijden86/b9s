package ui

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every light colour must come from the sepia palette (ADR 0030). A hex
// literal in a Light field elsewhere bypasses the paper the palette was tuned
// against, and nothing else checks its contrast.
func TestLightColoursComeFromSepiaPalette(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`Light:\s*"#`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "palette.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if literal.MatchString(line) {
				t.Errorf("%s:%d: light colour literal; use a sepia* constant from palette.go: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestLightTextMeetsAAOnPaper(t *testing.T) {
	th := TestTheme()
	text := map[string]lipgloss.AdaptiveColor{
		"Primary": th.Primary, "Secondary": th.Secondary, "Subtext": th.Subtext, "Muted": th.Muted,
		"Open": th.Open, "InProgress": th.InProgress, "Blocked": th.Blocked, "Deferred": th.Deferred,
		"Pinned": th.Pinned, "Hooked": th.Hooked, "Closed": th.Closed,
		"Bug": th.Bug, "Feature": th.Feature, "Task": th.Task, "Epic": th.Epic, "Chore": th.Chore,
		"ColorText": ColorText, "ColorSubtext": ColorSubtext, "ColorMuted": ColorMuted,
		"ColorPrimary": ColorPrimary, "ColorInfo": ColorInfo, "ColorSuccess": ColorSuccess,
		"ColorWarning": ColorWarning, "ColorDanger": ColorDanger,
		"ColorStatusReview": ColorStatusReview,
		"ColorPrioCritical": ColorPrioCritical, "ColorPrioHigh": ColorPrioHigh,
		"ColorPrioMedium": ColorPrioMedium, "ColorPrioLow": ColorPrioLow,
		"ColorTypeBug": ColorTypeBug, "ColorTypeFeature": ColorTypeFeature, "ColorTypeTask": ColorTypeTask,
		"ColorTypeEpic": ColorTypeEpic, "ColorTypeChore": ColorTypeChore,
	}
	for name, c := range text {
		if r := contrastRatio(c.Light, sepiaPaper); r < 4.5 {
			t.Errorf("%s %s on paper %s: contrast %.2f:1, want >= 4.5:1", name, c.Light, sepiaPaper, r)
		}
	}
	for i, c := range sepiaAccents {
		if r := contrastRatio(c, sepiaPaper); r < 4.5 {
			t.Errorf("sepiaAccents[%d] %s on paper: contrast %.2f:1, want >= 4.5:1", i, c, r)
		}
	}
	if r := contrastRatio(th.Highlight.Light, sepiaInk); r < 4.5 {
		t.Errorf("ink on the selection bar %s: contrast %.2f:1, want >= 4.5:1", th.Highlight.Light, r)
	}
}

func TestPaperSequencesPaintOnlyALightTerminal(t *testing.T) {
	set, reset := PaperSequences(false)
	if set != "\x1b]11;"+sepiaPaper+"\x07" {
		t.Errorf("light terminal set = %q", set)
	}
	if reset != "\x1b]111\x07" {
		t.Errorf("light terminal reset = %q", reset)
	}
	if set, reset := PaperSequences(true); set != "" || reset != "" {
		t.Errorf("dark terminal must be left alone, got %q / %q", set, reset)
	}
}

func contrastRatio(a, b string) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

func relativeLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	ch := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(0) + 0.7152*ch(2) + 0.0722*ch(4)
}
