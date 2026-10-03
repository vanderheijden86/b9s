package ui

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/skin"
)

// Every adaptive colour must come from the skin slots (ADR 0033). A hex
// literal in a Light or Dark field ignores the skin the user chose, and
// nothing checks its contrast against that skin's background.
func TestAdaptiveColoursComeFromSkins(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`(Light|Dark):\s*"#`)
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
				t.Errorf("%s:%d: adaptive colour literal; derive it from a skin token in palette.go: %s", f, i+1, strings.TrimSpace(line))
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
	paper := string(lightSkin.Bg)
	for name, c := range text {
		if r := contrastRatio(c.Light, paper); r < 4.5 {
			t.Errorf("%s %s on paper %s: contrast %.2f:1, want >= 4.5:1", name, c.Light, paper, r)
		}
	}
	for i, c := range lightSkin.Accents {
		if r := contrastRatio(string(c), paper); r < 4.5 {
			t.Errorf("accent %d %s on paper: contrast %.2f:1, want >= 4.5:1", i, c, r)
		}
	}
	if r := contrastRatio(th.Highlight.Light, string(lightSkin.Text)); r < 4.5 {
		t.Errorf("ink on the selection bar %s: contrast %.2f:1, want >= 4.5:1", th.Highlight.Light, r)
	}
}

func TestPaperSequencesPaintOnlyALightTerminal(t *testing.T) {
	set, reset := PaperSequences(false)
	if set != "\x1b]11;"+string(lightSkin.Bg)+"\x07" {
		t.Errorf("light terminal set = %q", set)
	}
	if reset != "\x1b]111\x07" {
		t.Errorf("light terminal reset = %q", reset)
	}
	if set, reset := PaperSequences(true); set != "" || reset != "" {
		t.Errorf("dark terminal must be left alone, got %q / %q", set, reset)
	}
}

// A custom skin replaces only the slot its background belongs to, so Ctrl-T
// still reaches a built-in theme for the other kind of terminal.
func TestUseSkinFillsTheSlotOfItsBackground(t *testing.T) {
	light, dark := Skins()
	t.Cleanup(func() { applySkins(light, dark) })

	nord, err := skin.Load(filepath.Join("..", "skin", "testdata", "nord.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	UseSkin(nord)
	if l, d := SkinNames(); l != "sepia" || d != "nord" {
		t.Fatalf("slots after a dark skin = %s/%s, want sepia/nord", l, d)
	}
	if ColorText.Dark != string(nord.Text) || ColorText.Light != string(light.Text) {
		t.Errorf("ColorText = %+v, want nord text in the dark slot only", ColorText)
	}

	gruvbox, err := skin.Load(filepath.Join("..", "skin", "testdata", "gruvbox-light.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	UseSkin(gruvbox)
	if l, d := SkinNames(); l != "gruvbox-light" || d != "nord" {
		t.Fatalf("slots after a light skin = %s/%s, want gruvbox-light/nord", l, d)
	}
	if set, _ := PaperSequences(false); set != "\x1b]11;"+string(gruvbox.Bg)+"\x07" {
		t.Errorf("paper sequence = %q, want gruvbox background", set)
	}
}

// A skin that keeps the terminal's own background leaves the terminal alone.
func TestPaperSequencesKeepTransparentBackground(t *testing.T) {
	light, dark := Skins()
	t.Cleanup(func() { applySkins(light, dark) })
	transparent := light
	transparent.Bg = ""
	UseSkin(transparent)
	if set, reset := PaperSequences(false); set != "" || reset != "" {
		t.Errorf("transparent skin sequences = %q / %q, want none", set, reset)
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

// A skin that fails to load must not stop b9s, and the user must see why the
// built-in colours are on screen.
func TestSkinErrorShowsInStatusBar(t *testing.T) {
	m := NewModel(nil, "").WithSkinError(errors.New(`unknown skin "neon"`))
	if !m.statusIsError || !strings.Contains(m.statusMsg, "neon") || !strings.Contains(m.statusMsg, "skin") {
		t.Fatalf("status = %q (error %v), want the skin error", m.statusMsg, m.statusIsError)
	}
	if m := NewModel(nil, "").WithSkinError(nil); m.statusIsError {
		t.Fatalf("nil skin error set status %q", m.statusMsg)
	}
}
