package web

import (
	"fmt"
	"strings"

	"github.com/vanderheijden86/b9s/pkg/skin"
)

// SkinCSS renders the light and dark skins as the custom properties app.css
// reads. The dark skin styles :root and the light skin :root[data-theme="light"],
// the same two selectors the browser's appearance switch toggles between.
func SkinCSS(light, dark skin.Palette) string {
	var b strings.Builder
	// Both names sit on :root, which the light block does not override, so
	// the appearance sheet can name either choice from either theme.
	names := fmt.Sprintf("  --skin-light: %q;\n  --skin-dark: %q;\n", cssName(light.Name), cssName(dark.Name))
	writeSkinBlock(&b, ":root", dark, names)
	writeSkinBlock(&b, `:root[data-theme="light"]`, light, "")
	return b.String()
}

// cssName keeps the characters a skin file name normally has, so the name
// cannot end its CSS string.
func cssName(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.', r == ' ':
			return r
		}
		return '-'
	}, name)
}

func writeSkinBlock(b *strings.Builder, selector string, p skin.Palette, extra string) {
	// A page has no terminal background to fall back to, so a skin that
	// keeps the terminal's own gets plain white or black.
	bg := p.Bg
	if bg == "" {
		bg = "#FFFFFF"
		if p.Dark {
			bg = "#000000"
		}
	}
	scheme := "light"
	if p.Dark {
		scheme = "dark"
	}
	// --hi marks the selection with a bar, so it needs the 3:1 non-text
	// contrast of WCAG 1.4.11. A paper skin selects with a quiet surface
	// and marks it with its primary colour instead.
	hi := p.Selection
	if contrast(hi, bg) < 3 {
		hi = p.Primary
	}
	vars := []struct {
		name  string
		value skin.Color
	}{
		{"page", bg},
		{"bg", bg},
		{"bg2", skin.Blend(p.Text, bg, 0.06)},
		{"bg3", skin.Blend(p.Text, bg, 0.12)},
		{"line", skin.Blend(p.Border, bg, 0.4)},
		{"text", p.Text},
		{"dim", p.Subtext},
		{"muted", p.Muted},
		{"accent", p.Epic},
		{"cyan", p.Info},
		{"hi", hi},
		{"green", p.Success},
		{"red", p.Danger},
		{"orange", p.Warning},
		{"blue", p.Task},
		{"feat", p.Feature},
		{"yellow", p.PrioMedium},
		{"ink", p.SelectionText},
		{"row-sel", skin.Blend(hi, bg, 0.15)},
		{"marked", skin.Blend(p.Epic, bg, 0.15)},
		{"flash", p.OpenTint},
		{"card-sel", p.BoardSelection},
		{"hover", skin.Blend(p.Text, bg, 0.03)},
	}
	fmt.Fprintf(b, "%s {\n  color-scheme: %s;\n%s", selector, scheme, extra)
	for _, v := range vars {
		fmt.Fprintf(b, "  --%s: %s;\n", v.name, v.value)
	}
	b.WriteString("}\n")
}

func contrast(a, b skin.Color) float64 {
	la, lb := skin.Luminance(a), skin.Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
