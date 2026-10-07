package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/skin"
	"github.com/vanderheijden86/b9s/pkg/ui"
)

func TestSkinStylesheetCarriesBothSlots(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	resp, err := ts.Client().Get(ts.URL + "/skin.css")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/css") {
		t.Fatalf("skin.css: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	css := string(body)
	dark, light, ok := strings.Cut(css, `:root[data-theme="light"]`)
	if !ok {
		t.Fatalf("no light block in %s", css)
	}
	for _, want := range []string{`--skin-light: "sepia";`, `--skin-dark: "dracula";`, "--bg: #282A36;", "--text: #F8F8F2;", "--hi: #4FC1E9;", "color-scheme: dark;"} {
		if !strings.Contains(dark, want) {
			t.Errorf("dark block lacks %q", want)
		}
	}
	for _, want := range []string{"--bg: #E9DFCB;", "--text: #3A2F22;", "--hi: #2F4F6F;", "--card-sel: #D2D5CC;", "color-scheme: light;"} {
		if !strings.Contains(light, want) {
			t.Errorf("light block lacks %q", want)
		}
	}
}

// A skin path in ui.skin reaches the browser too, so the web board looks like
// the terminal.
func TestSkinStylesheetUsesConfiguredSkin(t *testing.T) {
	gruvbox, err := skin.Load(filepath.Join("..", "skin", "testdata", "gruvbox-light.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	dracula, _ := skin.Resolve("dracula")
	_, light, _ := strings.Cut(SkinCSS(gruvbox, dracula), `:root[data-theme="light"]`)
	if gruvbox.Bg == "" || !strings.Contains(light, "--bg: "+string(gruvbox.Bg)+";") {
		t.Fatalf("gruvbox background %q missing from %s", gruvbox.Bg, light)
	}
}

// A skin that keeps the terminal background still gives the page a colour.
func TestSkinStylesheetFillsTransparentBackground(t *testing.T) {
	p, _ := skin.Resolve("dracula")
	p.Bg = ""
	sepia, _ := skin.Resolve("sepia")
	if css := SkinCSS(sepia, p); !strings.Contains(css, "--bg: #000000;") {
		t.Fatalf("transparent dark skin must fall back to black: %s", css)
	}
}

// The skin name goes into a CSS string, and a file name may hold anything.
func TestSkinNameCannotBreakOutOfTheStylesheet(t *testing.T) {
	p, _ := skin.Resolve("sepia")
	p.Name = `x"; } body { display: none } :root { --a: "`
	dracula, _ := skin.Resolve("dracula")
	css := SkinCSS(p, dracula)
	if strings.Contains(css, "display: none") && strings.Count(css, "{") != 2 {
		t.Fatalf("skin name escaped its string: %s", css)
	}
	if strings.Count(css, "{") != 2 || strings.Count(css, "}") != 2 {
		t.Fatalf("want two blocks, got %s", css)
	}
}

// :skins in the TUI rewrites ui.skin while b9s web runs. The next page load
// must show it, and a cached stylesheet must not survive the change.
func TestSkinStylesheetFollowsSkinChanges(t *testing.T) {
	sepia, _ := skin.Resolve("sepia")
	dracula, _ := skin.Resolve("dracula")
	gruvbox, err := skin.Load(filepath.Join("..", "skin", "testdata", "gruvbox-light.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	light := sepia
	dir := newFixtureProject(t)
	store := NewStore(50 * time.Millisecond)
	t.Cleanup(store.Close)
	if failure := store.Open(datasource.OpenTarget{Name: "fixture", Dir: dir}, dir); failure != nil {
		t.Fatalf("open fixture: %s", failure.Message())
	}
	srv, err := NewServer(Options{Store: store, Writer: ui.NewIssueWriter(), Assets: testAssets,
		Skins: func() (skin.Palette, skin.Palette) { return light, dracula }})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	get := func() (string, string) {
		t.Helper()
		resp, err := ts.Client().Get(ts.URL + "/skin.css")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), resp.Header.Get("ETag")
	}
	before, tagBefore := get()
	light = gruvbox
	after, tagAfter := get()
	if strings.Contains(before, "gruvbox") || !strings.Contains(after, `--skin-light: "gruvbox-light";`) {
		t.Fatalf("stylesheet did not follow the skin change:\n%s", after)
	}
	if tagBefore == tagAfter {
		t.Errorf("ETag %s unchanged after the skin changed", tagAfter)
	}
}
