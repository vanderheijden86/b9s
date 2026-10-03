package skin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T, name string) Palette {
	t.Helper()
	p, err := Load(filepath.Join("testdata", name+".yaml"))
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return p
}

func TestDraculaSkinMapsK9sSlots(t *testing.T) {
	p := load(t, "dracula")
	want := map[string]Color{
		"Name":       Color(p.Name),
		"Bg":         p.Bg,
		"Text":       p.Text,
		"Primary":    p.Primary,
		"Blocked":    p.Blocked,
		"Open":       p.Open,
		"InProgress": p.InProgress,
		"Closed":     p.Closed,
		"Border":     p.Border,
	}
	expect := map[string]Color{
		"Name":       "dracula",
		"Bg":         "#282A36",
		"Text":       "#F8F8F2",
		"Primary":    "#BD93F9",
		"Blocked":    "#FF5555",
		"Open":       "#50FA7B",
		"InProgress": "#8BE9FD",
		"Closed":     "#6272A4",
		"Border":     "#44475A",
	}
	for k, v := range expect {
		if want[k] != v {
			t.Errorf("%s = %q, want %q", k, want[k], v)
		}
	}
	if !p.Dark {
		t.Error("dracula must classify as dark")
	}
}

func TestLightSkinClassifiesAsLight(t *testing.T) {
	for _, name := range []string{"gruvbox-light", "solarized-light"} {
		if load(t, name).Dark {
			t.Errorf("%s classified as dark", name)
		}
	}
	if !load(t, "nord").Dark {
		t.Error("nord classified as light")
	}
}

func TestNamedColoursResolveToHex(t *testing.T) {
	p := load(t, "stock")
	if p.Text != "#1E90FF" { // dodgerblue
		t.Errorf("Text = %q, want dodgerblue #1E90FF", p.Text)
	}
	if p.Bg != "#000000" {
		t.Errorf("Bg = %q, want black #000000", p.Bg)
	}
	if p.Selection != "#00FFFF" { // table cursorBgColor aqua
		t.Errorf("Selection = %q, want aqua", p.Selection)
	}
}

// k9s merges a partial skin onto its stock styles, so a skin that only clears
// backgrounds still has every foreground.
func TestPartialSkinInheritsStockSlots(t *testing.T) {
	p := load(t, "transparent")
	if p.Bg != "" {
		t.Errorf("Bg = %q, want terminal default", p.Bg)
	}
	if p.Blocked != "#FF4500" { // stock errorColor orangered
		t.Errorf("Blocked = %q, want stock orangered", p.Blocked)
	}
	if p.OpenTint == "" {
		t.Error("tints need a background to blend into even when bg is default")
	}
}

func TestEveryTokenIsFilled(t *testing.T) {
	for _, name := range []string{"dracula", "nord", "gruvbox-light", "solarized-light", "stock"} {
		p := load(t, name)
		for token, v := range p.Tokens() {
			if v == "" {
				t.Errorf("%s: token %s is empty", name, token)
			}
		}
		if len(p.Accents) != AccentCount {
			t.Errorf("%s: %d accents, want %d", name, len(p.Accents), AccentCount)
		}
	}
}

func TestTintBlendsAccentIntoBackground(t *testing.T) {
	p := load(t, "dracula")
	if got := Blend("#FF5555", "#282A36", 0.2); p.BlockedTint != got {
		t.Errorf("BlockedTint = %q, want %q", p.BlockedTint, got)
	}
	if got := Blend("#FFFFFF", "#000000", 0.5); got != "#808080" {
		t.Errorf("Blend half white into black = %q, want #808080", got)
	}
}

func TestB9sBlockOverridesTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mine.yaml")
	src := `k9s:
  body:
    fgColor: "#111111"
    bgColor: "#fafafa"
b9s:
  blocked: "#aa0000"
  selection: steelblue
  accents: ["#010101", "#020202", "#030303", "#040404", "#050505", "#060606", "#070707", "#080808"]
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "mine" || p.Dark {
		t.Errorf("name/dark = %q/%v", p.Name, p.Dark)
	}
	if p.Blocked != "#AA0000" || p.Selection != "#4682B4" || p.Accents[7] != "#080808" {
		t.Errorf("overrides not applied: blocked %q selection %q accents %v", p.Blocked, p.Selection, p.Accents)
	}
	if p.BlockedTint != Blend("#AA0000", "#FAFAFA", TintWeight) {
		t.Errorf("tint must derive from the overridden accent, got %q", p.BlockedTint)
	}
}

func TestBuiltinsAreCompleteAndNamed(t *testing.T) {
	for _, name := range Builtins() {
		p, err := Resolve(name)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", name, err)
		}
		if p.Name != name {
			t.Errorf("built-in %q reports name %q", name, p.Name)
		}
		for token, v := range p.Tokens() {
			if v == "" {
				t.Errorf("%s: token %s is empty", name, token)
			}
		}
	}
	if s, _ := Resolve("sepia"); s.Dark {
		t.Error("sepia must be light")
	}
	if d, _ := Resolve("dracula"); !d.Dark {
		t.Error("dracula must be dark")
	}
}

func TestResolveRejectsUnknownSkin(t *testing.T) {
	_, err := Resolve("no-such-skin")
	if err == nil || !strings.Contains(err.Error(), "sepia") {
		t.Fatalf("err = %v, want one that lists the built-in skins", err)
	}
}

func TestUnknownColourNameIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typo.yaml")
	if err := os.WriteFile(path, []byte("k9s:\n  body:\n    fgColor: dodgerblu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "dodgerblu") {
		t.Fatalf("err = %v, want one naming the bad colour", err)
	}
}
