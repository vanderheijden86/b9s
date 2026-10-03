// Package skin reads k9s skin files (https://k9scli.io/topics/skins/) into the
// palette b9s renders with, so one skin file colours both tools.
//
// A k9s skin names colours by k9s widget. b9s has concepts k9s lacks (issue
// status, priority, type, badge tints), so each b9s token is mapped from the
// k9s slot that plays the closest role. An optional top-level `b9s:` block in
// the same file sets any token directly; k9s ignores that block.
package skin

import (
	"bytes"
	"embed"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/image/colornames"
	"gopkg.in/yaml.v3"
)

// AccentCount is the number of accents that colour repos, epics and help
// panels. Hashing picks among them, so the count is fixed.
const AccentCount = 8

// TintWeight is the share of an accent blended into the background for a
// badge tint.
const TintWeight = 0.2

// Color is an upper-case "#RRGGBB" hex value, or empty for the terminal's
// default colour.
type Color string

// UnmarshalYAML accepts what k9s accepts: hex, a W3C colour name, or
// "default".
func (c *Color) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	v, err := ParseColor(s)
	if err != nil {
		return fmt.Errorf("line %d: %w", node.Line, err)
	}
	*c = v
	return nil
}

// ParseColor normalises a k9s colour value.
func ParseColor(s string) (Color, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "" || s == "default" || s == "reset":
		return "", nil
	case strings.HasPrefix(s, "#"):
		hex := s[1:]
		if len(hex) == 3 {
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
		}
		if len(hex) != 6 {
			return "", fmt.Errorf("invalid colour %q", s)
		}
		if _, err := strconv.ParseUint(hex, 16, 32); err != nil {
			return "", fmt.Errorf("invalid colour %q", s)
		}
		return Color("#" + strings.ToUpper(hex)), nil
	}
	if rgba, ok := colornames.Map[s]; ok {
		return hexOf(rgba), nil
	}
	return "", fmt.Errorf("unknown colour name %q", s)
}

func hexOf(c color.RGBA) Color {
	return Color(fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B))
}

// Palette is every colour b9s renders with. The yaml tags are the keys of a
// skin file's `b9s:` block.
type Palette struct {
	Name string `yaml:"-"`
	// Dark reports a light-on-dark skin. It decides which theme slot the
	// skin fills and which board layout renders it.
	Dark bool `yaml:"-"`

	Bg             Color `yaml:"bg"`
	BgDeep         Color `yaml:"bgDeep"`
	BgSubtle       Color `yaml:"bgSubtle"`
	Selection      Color `yaml:"selection"`
	SelectionText  Color `yaml:"selectionText"`
	BoardSelection Color `yaml:"boardSelection"`
	Match          Color `yaml:"match"`
	HeaderText     Color `yaml:"headerText"`

	Text      Color `yaml:"text"`
	Subtext   Color `yaml:"subtext"`
	Muted     Color `yaml:"muted"`
	Primary   Color `yaml:"primary"`
	Secondary Color `yaml:"secondary"`
	Info      Color `yaml:"info"`
	Success   Color `yaml:"success"`
	Warning   Color `yaml:"warning"`
	Danger    Color `yaml:"danger"`
	Border    Color `yaml:"border"`

	Open       Color `yaml:"open"`
	InProgress Color `yaml:"inProgress"`
	Blocked    Color `yaml:"blocked"`
	Deferred   Color `yaml:"deferred"`
	Pinned     Color `yaml:"pinned"`
	Hooked     Color `yaml:"hooked"`
	Review     Color `yaml:"review"`
	Closed     Color `yaml:"closed"`
	Tombstone  Color `yaml:"tombstone"`

	OpenTint       Color `yaml:"openTint"`
	InProgressTint Color `yaml:"inProgressTint"`
	BlockedTint    Color `yaml:"blockedTint"`
	DeferredTint   Color `yaml:"deferredTint"`
	PinnedTint     Color `yaml:"pinnedTint"`
	HookedTint     Color `yaml:"hookedTint"`
	ReviewTint     Color `yaml:"reviewTint"`
	ClosedTint     Color `yaml:"closedTint"`
	TombstoneTint  Color `yaml:"tombstoneTint"`

	PrioCritical     Color `yaml:"prioCritical"`
	PrioHigh         Color `yaml:"prioHigh"`
	PrioMedium       Color `yaml:"prioMedium"`
	PrioLow          Color `yaml:"prioLow"`
	PrioCriticalTint Color `yaml:"prioCriticalTint"`
	PrioHighTint     Color `yaml:"prioHighTint"`
	PrioMediumTint   Color `yaml:"prioMediumTint"`
	PrioLowTint      Color `yaml:"prioLowTint"`

	Bug     Color `yaml:"bug"`
	Feature Color `yaml:"feature"`
	Task    Color `yaml:"task"`
	Epic    Color `yaml:"epic"`
	Chore   Color `yaml:"chore"`

	Accents []Color `yaml:"accents"`
}

// Tokens returns every single-colour token by its `b9s:` key. Bg may be empty
// for a skin that keeps the terminal's background.
func (p Palette) Tokens() map[string]Color {
	out := map[string]Color{}
	v := reflect.ValueOf(p)
	ty := v.Type()
	for i := 0; i < ty.NumField(); i++ {
		if ty.Field(i).Type != reflect.TypeOf(Color("")) {
			continue
		}
		key := ty.Field(i).Tag.Get("yaml")
		if key == "bg" && p.Bg == "" {
			continue
		}
		out[key] = v.Field(i).Interface().(Color)
	}
	return out
}

// colorSet is the subset of the k9s skin schema b9s reads.
type colorSet struct {
	Body struct {
		Fg   Color `yaml:"fgColor"`
		Bg   Color `yaml:"bgColor"`
		Logo Color `yaml:"logoColor"`
	} `yaml:"body"`
	Prompt struct {
		Suggest Color `yaml:"suggestColor"`
	} `yaml:"prompt"`
	Info struct {
		Fg Color `yaml:"fgColor"`
	} `yaml:"info"`
	Frame struct {
		Border struct {
			Fg    Color `yaml:"fgColor"`
			Focus Color `yaml:"focusColor"`
		} `yaml:"border"`
		Menu struct {
			Key Color `yaml:"keyColor"`
		} `yaml:"menu"`
		Status struct {
			New       Color `yaml:"newColor"`
			Modify    Color `yaml:"modifyColor"`
			Add       Color `yaml:"addColor"`
			Error     Color `yaml:"errorColor"`
			Pending   Color `yaml:"pendingColor"`
			Highlight Color `yaml:"highlightColor"`
			Kill      Color `yaml:"killColor"`
			Completed Color `yaml:"completedColor"`
		} `yaml:"status"`
		Title struct {
			Counter Color `yaml:"counterColor"`
		} `yaml:"title"`
	} `yaml:"frame"`
	Views struct {
		Table struct {
			CursorFg Color `yaml:"cursorFgColor"`
			CursorBg Color `yaml:"cursorBgColor"`
		} `yaml:"table"`
		YAML struct {
			Key Color `yaml:"keyColor"`
		} `yaml:"yaml"`
	} `yaml:"views"`
}

type skinFile struct {
	K9s colorSet `yaml:"k9s"`
	B9s *struct {
		Palette `yaml:",inline"`
		Dark    *bool `yaml:"dark"`
	} `yaml:"b9s"`
}

//go:embed builtin/*.yaml
var builtinFS embed.FS

// Builtins lists the skins compiled into b9s.
func Builtins() []string {
	entries, _ := builtinFS.ReadDir("builtin")
	var names []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".yaml")
		if name != stockName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// stockName is k9s's own default skin. k9s lays every skin over it, so b9s
// does too; it is a base, not a choice.
const stockName = "k9s-stock"

// Resolve returns a built-in skin by name, or loads a skin file when ref is a
// path. A leading ~ is the home directory.
func Resolve(ref string) (Palette, error) {
	if src, err := builtinFS.ReadFile("builtin/" + ref + ".yaml"); err == nil && ref != stockName {
		return Parse(ref, src)
	}
	if strings.ContainsAny(ref, `/\`) || strings.HasSuffix(ref, ".yaml") || strings.HasSuffix(ref, ".yml") {
		return Load(expandHome(ref))
	}
	return Palette{}, fmt.Errorf("unknown skin %q: want a skin file path or one of %s", ref, strings.Join(Builtins(), ", "))
}

// Load reads a skin file. The file name without extension is the skin's name.
func Load(path string) (Palette, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, fmt.Errorf("reading skin: %w", err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	p, err := Parse(name, src)
	if err != nil {
		return Palette{}, fmt.Errorf("skin %s: %w", path, err)
	}
	return p, nil
}

// Parse reads skin YAML laid over the k9s stock skin.
func Parse(name string, src []byte) (Palette, error) {
	stock, err := builtinFS.ReadFile("builtin/" + stockName + ".yaml")
	if err != nil {
		return Palette{}, err
	}
	var f skinFile
	if err := yaml.Unmarshal(stock, &f); err != nil {
		return Palette{}, fmt.Errorf("stock skin: %w", err)
	}
	f.B9s = nil
	if len(bytes.TrimSpace(src)) > 0 {
		if err := yaml.Unmarshal(src, &f); err != nil {
			return Palette{}, err
		}
	}
	p := fromK9s(f.K9s)
	var dark *bool
	if f.B9s != nil {
		overlay(&p, f.B9s.Palette)
		dark = f.B9s.Dark
	}
	p.Name = name
	if dark != nil {
		p.Dark = *dark
	} else {
		p.Dark = isDark(p.Bg, p.Text)
	}
	derive(&p)
	if len(p.Accents) != AccentCount {
		return Palette{}, fmt.Errorf("b9s.accents has %d colours, want %d", len(p.Accents), AccentCount)
	}
	return p, nil
}

func fromK9s(k colorSet) Palette {
	s := k.Frame.Status
	return Palette{
		Bg:            k.Body.Bg,
		Text:          k.Body.Fg,
		Primary:       k.Body.Logo,
		Selection:     k.Views.Table.CursorBg,
		SelectionText: k.Views.Table.CursorFg,
		Muted:         s.Completed,
		Secondary:     s.Completed,
		Info:          s.New,
		Success:       s.Add,
		Warning:       s.Highlight,
		Danger:        s.Error,
		Border:        k.Frame.Border.Fg,

		Open:       s.Add,
		InProgress: s.New,
		Blocked:    s.Error,
		Deferred:   s.Pending,
		Pinned:     s.New,
		Hooked:     k.Views.YAML.Key,
		Review:     s.Modify,
		Closed:     s.Completed,
		Tombstone:  s.Kill,

		PrioCritical: s.Error,
		PrioHigh:     s.Highlight,
		PrioMedium:   k.Info.Fg,
		PrioLow:      s.Add,

		Bug:     s.Error,
		Feature: s.Add,
		Task:    s.New,
		Epic:    k.Body.Logo,
		Chore:   k.Frame.Menu.Key,

		Accents: []Color{
			k.Body.Logo, s.New, s.Add, s.Highlight,
			s.Modify, k.Info.Fg, k.Frame.Menu.Key, k.Prompt.Suggest,
		},
	}
}

// overlay copies every colour o sets onto p.
func overlay(p *Palette, o Palette) {
	pv, ov := reflect.ValueOf(p).Elem(), reflect.ValueOf(o)
	for i := 0; i < pv.NumField(); i++ {
		if f := ov.Field(i); f.Kind() == reflect.String && f.String() != "" {
			pv.Field(i).Set(f)
		}
	}
	if len(o.Accents) > 0 {
		p.Accents = o.Accents
	}
}

// derive fills the tokens no k9s slot carries from the ones that are set.
// Each runs only when the skin's `b9s:` block left it empty.
func derive(p *Palette) {
	ground := p.Bg
	if ground == "" {
		ground = "#FFFFFF"
		if p.Dark {
			ground = "#000000"
		}
	}
	text := p.Text
	if text == "" {
		text = "#000000"
		if p.Dark {
			text = "#FFFFFF"
		}
	}
	fill := func(c *Color, v Color) {
		if *c == "" {
			*c = v
		}
	}
	fill(&p.Subtext, Blend(text, ground, 0.75))
	fill(&p.BgDeep, Blend(text, ground, 0.06))
	fill(&p.BgSubtle, Blend(text, ground, 0.10))
	fill(&p.Selection, Blend(p.Primary, ground, 0.3))
	fill(&p.SelectionText, text)
	fill(&p.BoardSelection, p.Selection)
	fill(&p.HeaderText, ground)
	fill(&p.Match, Blend(p.Review, ground, 0.3))
	for _, t := range []struct {
		tint *Color
		of   Color
	}{
		{&p.OpenTint, p.Open}, {&p.InProgressTint, p.InProgress},
		{&p.BlockedTint, p.Blocked}, {&p.DeferredTint, p.Deferred},
		{&p.PinnedTint, p.Pinned}, {&p.HookedTint, p.Hooked},
		{&p.ReviewTint, p.Review}, {&p.ClosedTint, p.Closed},
		{&p.TombstoneTint, p.Tombstone},
		{&p.PrioCriticalTint, p.PrioCritical}, {&p.PrioHighTint, p.PrioHigh},
		{&p.PrioMediumTint, p.PrioMedium}, {&p.PrioLowTint, p.PrioLow},
	} {
		fill(t.tint, Blend(t.of, ground, TintWeight))
	}
	// A slot a skin set to "default" falls back to the text colour, so no
	// foreground is ever unset.
	pv := reflect.ValueOf(p).Elem()
	for i := 0; i < pv.NumField(); i++ {
		if f := pv.Field(i); f.Kind() == reflect.String && f.Type() == reflect.TypeOf(Color("")) && f.String() == "" && pv.Type().Field(i).Name != "Bg" {
			f.SetString(string(text))
		}
	}
	for i, a := range p.Accents {
		if a == "" {
			p.Accents[i] = text
		}
	}
}

// Blend mixes weight of c into ground. An empty colour blends as ground.
func Blend(c, ground Color, weight float64) Color {
	cr, cg, cb := rgb(c, ground)
	gr, gg, gb := rgb(ground, "#000000")
	mix := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a)*weight + float64(b)*(1-weight)))
	}
	return hexOf(color.RGBA{R: mix(cr, gr), G: mix(cg, gg), B: mix(cb, gb)})
}

func rgb(c, fallback Color) (uint8, uint8, uint8) {
	if c == "" {
		c = fallback
	}
	v, _ := strconv.ParseUint(strings.TrimPrefix(string(c), "#"), 16, 32)
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}

// Luminance is the WCAG relative luminance of c.
func Luminance(c Color) float64 {
	r, g, b := rgb(c, "#000000")
	ch := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(r) + 0.7152*ch(g) + 0.0722*ch(b)
}

// isDark judges by the background when the skin sets one. A skin that keeps
// the terminal background is judged by its text: light text implies a dark
// terminal.
func isDark(bg, text Color) bool {
	if bg != "" {
		return closerToBlack(bg)
	}
	if text != "" {
		return !closerToBlack(text)
	}
	return true
}

// closerToBlack reports whether c contrasts more with white than with black.
func closerToBlack(c Color) bool {
	l := Luminance(c)
	return (1.05)/(l+0.05) > (l+0.05)/0.05
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}
