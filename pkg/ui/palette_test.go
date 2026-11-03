package ui

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/skin"
)

func colorTestRenderer(t *testing.T, dark bool) *lipgloss.Renderer {
	t.Helper()
	profile := TermProfile
	t.Cleanup(func() { TermProfile = profile })
	TermProfile = colorprofile.TrueColor
	r := lipgloss.NewRenderer(nil)
	r.SetColorProfile(termenv.TrueColor)
	r.SetHasDarkBackground(dark)
	return r
}

func renderedRGB(t *testing.T, rendered string, code int) string {
	t.Helper()
	pattern := regexp.MustCompile(fmt.Sprintf(`(?:\x1b\[|;)%d;2;(\d+);(\d+);(\d+)`, code))
	match := pattern.FindStringSubmatch(rendered)
	if match == nil {
		t.Fatalf("missing RGB code %d in %q", code, rendered)
	}
	r, _ := strconv.Atoi(match[1])
	g, _ := strconv.Atoi(match[2])
	b, _ := strconv.Atoi(match[3])
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func TestSepiaTriageSymbolsMeetAA(t *testing.T) {
	th := DefaultTheme(colorTestRenderer(t, false))
	for name, style := range map[string]lipgloss.Style{
		"up": th.PriorityUpArrow, "down": th.PriorityDownArrow,
		"star": th.TriageStar, "unblocks": th.TriageUnblocks, "alternate": th.TriageUnblocksAlt,
	} {
		t.Run(name, func(t *testing.T) {
			fg := renderedRGB(t, style.Render("↑"), 38)
			if ratio := contrastRatio(fg, string(lightSkin.Bg)); ratio < 4.5 {
				t.Errorf("%s on paper: %.2f:1, want >= 4.5:1", fg, ratio)
			}
		})
	}
}

func TestSepiaHeatmapMeetsAA(t *testing.T) {
	r := colorTestRenderer(t, false)
	th := DefaultTheme(r)
	for _, intensity := range []float64{0, 0.01, 0.15, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1} {
		t.Run(fmt.Sprint(intensity), func(t *testing.T) {
			block := r.NewStyle().Foreground(GetHeatGradientColor(intensity, th)).Render("█")
			if ratio := contrastRatio(renderedRGB(t, block, 38), string(lightSkin.Bg)); ratio < 4.5 {
				t.Errorf("heat glyph on paper: %.2f:1, want >= 4.5:1", ratio)
			}
			bg, fg := GetHeatGradientColorBg(intensity)
			cell := r.NewStyle().Background(bg).Foreground(fg).Render("12")
			if ratio := contrastRatio(renderedRGB(t, cell, 38), renderedRGB(t, cell, 48)); ratio < 4.5 {
				t.Errorf("heat cell text: %.2f:1, want >= 4.5:1", ratio)
			}
		})
	}
}

func TestPickerTextFollowsTerminalPalette(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			r := colorTestRenderer(t, dark)
			th := DefaultTheme(r)
			picker := NewProjectPicker([]ProjectEntry{
				{Project: config.Project{Name: "sample"}, FavoriteNum: 1, IsActive: true},
				{Project: config.Project{Name: "other"}},
			}, th)
			picker.SetSize(120, 40)
			m := Model{
				theme: th, width: 120, height: 40, projectPicker: picker,
				labelEntries:    []LabelEntry{{Label: "sample", Number: 1, Count: 1}},
				assigneeEntries: []AssigneeEntry{{Assignee: "sample", Number: 1, Count: 1}},
			}
			fg := string(lightSkin.Text)
			if dark {
				fg = "#F3F3F3"
			}
			style := r.NewStyle().Foreground(lipgloss.Color(fg)).Bold(true)
			for name, tt := range map[string]struct{ view, token string }{
				"project":  {strings.Join(picker.renderProjectTable(), "\n"), "<2>"},
				"title":    {picker.renderTitleBar(120), " <1>"},
				"label":    {m.renderLabelBar(), " <1>"},
				"assignee": {m.renderAssigneeBar(), " <1>"},
			} {
				t.Run(name, func(t *testing.T) {
					if !strings.Contains(tt.view, style.Render(tt.token)) {
						t.Errorf("%s should render %q with %s: %q", name, tt.token, fg, tt.view)
					}
				})
			}
		})
	}
}

func TestDarkTriageAndHeatmapColorsStayUnchanged(t *testing.T) {
	r := colorTestRenderer(t, true)
	th := DefaultTheme(r)
	for _, tt := range []struct {
		style lipgloss.Style
		want  string
	}{
		{th.PriorityUpArrow, "#FF6B6B"}, {th.PriorityDownArrow, "#4ECDC4"},
		{th.TriageStar, "#FFD700"}, {th.TriageUnblocks, "#50FA7B"}, {th.TriageUnblocksAlt, "#6272A4"},
	} {
		want := r.NewStyle().Foreground(lipgloss.Color(tt.want)).Bold(tt.style.GetBold()).Render("↑")
		if got := tt.style.Render("↑"); got != want {
			t.Errorf("dark triage = %q, want %q", got, want)
		}
	}
	for _, tt := range []struct {
		intensity float64
		bg, fg    string
	}{
		{0, "#1A1A2E", "#6272A4"}, {0.1, "#16213E", "#BBE1FA"},
		{0.2, "#3282B8", "#FFFFFF"}, {0.4, "#F7DC6F", "#1A1A2E"},
		{0.6, "#E94560", "#FFFFFF"}, {0.8, "#FF2E63", "#FFFFFF"},
	} {
		bg, fg := GetHeatGradientColorBg(tt.intensity)
		cell := r.NewStyle().Background(bg).Foreground(fg).Render("12")
		want := r.NewStyle().Background(lipgloss.Color(tt.bg)).Foreground(lipgloss.Color(tt.fg)).Render("12")
		if cell != want {
			t.Errorf("dark heat cell = %q, want %q", cell, want)
		}
	}
	for i, want := range []string{"#1A1A2E", "#16213E", "#0F4C75", "#3282B8", "#BBE1FA", "#F7DC6F", "#E94560", "#FF2E63"} {
		wantRender := r.NewStyle().Foreground(lipgloss.Color(want)).Render("█")
		if got := r.NewStyle().Foreground(HeatmapGradientColors[i]).Render("█"); got != wantRender {
			t.Errorf("dark heat gradient %d = %q, want %q", i, got, wantRender)
		}
	}
}

func TestHealthTextFollowsTerminalPalette(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			r := colorTestRenderer(t, dark)
			m := Model{
				theme: DefaultTheme(r), width: 100, height: 40,
				dbHealth: DatabaseHealth{DoltAttempt: &DoltFailure{Error: "connection refused"}},
			}
			warning, failure := string(lightSkin.Warning), string(lightSkin.Danger)
			if dark {
				warning, failure = "#FF8800", "#FF4444"
			}
			view := m.renderDBHealthModal()
			for text, color := range map[string]string{
				"Dolt configured but not connected:": warning, "connection refused": failure,
			} {
				want := r.NewStyle().Foreground(lipgloss.Color(color)).Render(text)
				if !strings.Contains(view, want) {
					t.Errorf("health text %q should use %s: %q", text, color, view)
				}
			}
		})
	}
}

func TestCommandSuggestionFollowsTerminalPalette(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			r := colorTestRenderer(t, dark)
			m := Model{theme: DefaultTheme(r)}
			m.commandPrompt.Start()
			m.commandPrompt.Append('e', 'p')
			fg := string(lightSkin.Subtext)
			if dark {
				fg = "#6C7086"
			}
			want := r.NewStyle().Foreground(lipgloss.Color(fg)).Render("ic")
			if view := m.renderUnifiedTitleBar(80); !strings.Contains(view, want) {
				t.Errorf("suggestion should use %s: %q", fg, view)
			}
		})
	}
}

func TestAdaptiveHeatmapTerminalProfiles(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, profile := range []colorprofile.Profile{colorprofile.ANSI, colorprofile.ANSI256} {
			t.Run(fmt.Sprintf("dark=%v/profile=%v", dark, profile), func(t *testing.T) {
				r := colorTestRenderer(t, dark)
				TermProfile = profile
				r.SetColorProfile(termenv.ANSI)
				if profile == colorprofile.ANSI256 {
					r.SetColorProfile(termenv.ANSI256)
				}
				bg, fg := GetHeatGradientColorBg(1)
				if _, ok := bg.(lipgloss.NoColor); !ok {
					t.Fatalf("profile %v paints heat background %T", profile, bg)
				}
				got := r.NewStyle().Background(bg).Foreground(fg).Render("12")
				if strings.Contains(got, "48;") {
					t.Fatalf("heatmap emitted background on profile %v: %q", profile, got)
				}
				if profile == colorprofile.ANSI {
					wantColor := lipgloss.ANSIColor(0)
					if dark {
						wantColor = lipgloss.ANSIColor(7)
					}
					want := r.NewStyle().Foreground(wantColor).Render("12")
					if got != want {
						t.Errorf("ANSI heat text = %q, want %q", got, want)
					}
					th := DefaultTheme(r)
					if got := th.TriageStar.Render("12"); got != want {
						t.Errorf("ANSI triage = %q, want %q", got, want)
					}
				}
			})
		}
	}
}

func TestTUIAccentsFollowCustomSkin(t *testing.T) {
	light, dark := Skins()
	t.Cleanup(func() { applySkins(light, dark) })
	custom := light
	custom.PickerText, custom.CommandSuggestion = "#123456", "#234567"
	custom.HealthWarning, custom.HealthError = "#345678", "#456789"
	custom.PriorityUp, custom.PriorityDown = "#56789A", "#6789AB"
	custom.TriageStar, custom.TriageUnblocks, custom.TriageUnblocksAlt = "#789ABC", "#89ABCD", "#9ABCDE"
	custom.HeatGradient = []skin.Color{"#123456", "#123456", "#123456", "#123456", "#123456", "#123456", "#123456", "#123456"}
	custom.HeatBackgrounds = []skin.Color{"#234567", "#234567", "#234567", "#234567", "#234567", "#234567"}
	custom.HeatForegrounds = []skin.Color{"#345678", "#345678", "#345678", "#345678", "#345678", "#345678"}
	UseSkin(custom)
	for _, tt := range []struct {
		got  lipgloss.AdaptiveColor
		want skin.Color
	}{
		{pickerText, custom.PickerText}, {commandSuggestionColor, custom.CommandSuggestion},
		{healthWarning, custom.HealthWarning}, {healthError, custom.HealthError},
		{priorityUp, custom.PriorityUp}, {priorityDown, custom.PriorityDown},
		{triageStar, custom.TriageStar}, {triageUnblocks, custom.TriageUnblocks}, {triageUnblocksAlt, custom.TriageUnblocksAlt},
	} {
		if tt.got.Light != string(tt.want) {
			t.Errorf("custom token = %+v, want light %s", tt.got, tt.want)
		}
	}
	r := colorTestRenderer(t, false)
	bg, fg := GetHeatGradientColorBg(0.8)
	want := r.NewStyle().Background(lipgloss.Color("#234567")).Foreground(lipgloss.Color("#345678")).Render("12")
	if got := r.NewStyle().Background(bg).Foreground(fg).Render("12"); got != want {
		t.Errorf("custom heat cell = %q, want %q", got, want)
	}
	want = r.NewStyle().Foreground(lipgloss.Color("#123456")).Render("█")
	if got := r.NewStyle().Foreground(GetHeatGradientColor(1, DefaultTheme(r))).Render("█"); got != want {
		t.Errorf("custom heat glyph = %q, want %q", got, want)
	}
}

// Every adaptive colour must come from the skin slots (ADR 0033). A hex
// literal in a Light or Dark field ignores the skin the user chose, and
// nothing checks its contrast against that skin's background.
func TestAdaptiveColoursComeFromSkins(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`(Light|Dark):\s*"#|(?:lipgloss\.Color|ThemeFg|ThemeBg)\("#`)
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
