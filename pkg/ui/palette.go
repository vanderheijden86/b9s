package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/skin"
)

// Every colour comes from two skins. The light slot renders on a light
// terminal background and the dark slot on a dark one. lipgloss resolves each
// AdaptiveColor when it renders, from the renderer's background, so Ctrl-T
// moves between slots without rebuilding a style.
var lightSkin, darkSkin skin.Palette

func init() {
	applySkins(mustBuiltin("sepia"), mustBuiltin("dracula"))
}

func mustBuiltin(name string) skin.Palette {
	p, err := skin.Resolve(name)
	if err != nil {
		panic("built-in skin " + name + ": " + err.Error())
	}
	return p
}

// UseSkin puts p into the slot its background belongs to. The other slot
// keeps its built-in skin, so Ctrl-T still reaches a theme for the other kind
// of terminal. It must run before the model and its styles are built.
func UseSkin(p skin.Palette) {
	if p.Dark {
		applySkins(lightSkin, p)
		return
	}
	applySkins(p, darkSkin)
}

// SkinNames reports the skins in the light and dark slots.
func SkinNames() (light, dark string) { return lightSkin.Name, darkSkin.Name }

// Skins returns the palettes in the light and dark slots.
func Skins() (light, dark skin.Palette) { return lightSkin, darkSkin }

func pair(token func(skin.Palette) skin.Color) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: string(token(lightSkin)), Dark: string(token(darkSkin))}
}

func accentPairs() []lipgloss.AdaptiveColor {
	out := make([]lipgloss.AdaptiveColor, skin.AccentCount)
	for i := range out {
		out[i] = lipgloss.AdaptiveColor{Light: string(lightSkin.Accents[i]), Dark: string(darkSkin.Accents[i])}
	}
	return out
}

type tokenFn = func(skin.Palette) skin.Color

func applySkins(light, dark skin.Palette) {
	lightSkin, darkSkin = light, dark
	for _, t := range []struct {
		dst   *lipgloss.AdaptiveColor
		token tokenFn
	}{
		{&ColorBg, func(p skin.Palette) skin.Color { return p.Bg }},
		{&ColorBgDark, func(p skin.Palette) skin.Color { return p.BgDeep }},
		{&ColorBgSubtle, func(p skin.Palette) skin.Color { return p.BgSubtle }},
		{&ColorBgHighlight, func(p skin.Palette) skin.Color { return p.Selection }},
		{&ColorBoardSelection, func(p skin.Palette) skin.Color { return p.BoardSelection }},
		{&ColorSelectionText, func(p skin.Palette) skin.Color { return p.SelectionText }},
		{&ColorMatchBg, func(p skin.Palette) skin.Color { return p.Match }},
		{&ColorHeaderText, func(p skin.Palette) skin.Color { return p.HeaderText }},
		{&ColorText, func(p skin.Palette) skin.Color { return p.Text }},
		{&ColorSubtext, func(p skin.Palette) skin.Color { return p.Subtext }},
		{&ColorMuted, func(p skin.Palette) skin.Color { return p.Muted }},
		{&ColorBorder, func(p skin.Palette) skin.Color { return p.Border }},
		{&ColorPrimary, func(p skin.Palette) skin.Color { return p.Primary }},
		{&ColorSecondary, func(p skin.Palette) skin.Color { return p.Secondary }},
		{&ColorInfo, func(p skin.Palette) skin.Color { return p.Info }},
		{&ColorSuccess, func(p skin.Palette) skin.Color { return p.Success }},
		{&ColorWarning, func(p skin.Palette) skin.Color { return p.Warning }},
		{&ColorDanger, func(p skin.Palette) skin.Color { return p.Danger }},

		{&ColorStatusOpen, func(p skin.Palette) skin.Color { return p.Open }},
		{&ColorStatusInProgress, func(p skin.Palette) skin.Color { return p.InProgress }},
		{&ColorStatusBlocked, func(p skin.Palette) skin.Color { return p.Blocked }},
		{&ColorStatusDeferred, func(p skin.Palette) skin.Color { return p.Deferred }},
		{&ColorStatusPinned, func(p skin.Palette) skin.Color { return p.Pinned }},
		{&ColorStatusHooked, func(p skin.Palette) skin.Color { return p.Hooked }},
		{&ColorStatusReview, func(p skin.Palette) skin.Color { return p.Review }},
		{&ColorStatusClosed, func(p skin.Palette) skin.Color { return p.Closed }},
		{&ColorStatusTombstone, func(p skin.Palette) skin.Color { return p.Tombstone }},

		{&ColorStatusOpenBg, func(p skin.Palette) skin.Color { return p.OpenTint }},
		{&ColorStatusInProgressBg, func(p skin.Palette) skin.Color { return p.InProgressTint }},
		{&ColorStatusBlockedBg, func(p skin.Palette) skin.Color { return p.BlockedTint }},
		{&ColorStatusDeferredBg, func(p skin.Palette) skin.Color { return p.DeferredTint }},
		{&ColorStatusPinnedBg, func(p skin.Palette) skin.Color { return p.PinnedTint }},
		{&ColorStatusHookedBg, func(p skin.Palette) skin.Color { return p.HookedTint }},
		{&ColorStatusReviewBg, func(p skin.Palette) skin.Color { return p.ReviewTint }},
		{&ColorStatusClosedBg, func(p skin.Palette) skin.Color { return p.ClosedTint }},
		{&ColorStatusTombstoneBg, func(p skin.Palette) skin.Color { return p.TombstoneTint }},

		{&ColorPrioCritical, func(p skin.Palette) skin.Color { return p.PrioCritical }},
		{&ColorPrioHigh, func(p skin.Palette) skin.Color { return p.PrioHigh }},
		{&ColorPrioMedium, func(p skin.Palette) skin.Color { return p.PrioMedium }},
		{&ColorPrioLow, func(p skin.Palette) skin.Color { return p.PrioLow }},
		{&ColorPrioCriticalBg, func(p skin.Palette) skin.Color { return p.PrioCriticalTint }},
		{&ColorPrioHighBg, func(p skin.Palette) skin.Color { return p.PrioHighTint }},
		{&ColorPrioMediumBg, func(p skin.Palette) skin.Color { return p.PrioMediumTint }},
		{&ColorPrioLowBg, func(p skin.Palette) skin.Color { return p.PrioLowTint }},

		{&ColorTypeBug, func(p skin.Palette) skin.Color { return p.Bug }},
		{&ColorTypeFeature, func(p skin.Palette) skin.Color { return p.Feature }},
		{&ColorTypeTask, func(p skin.Palette) skin.Color { return p.Task }},
		{&ColorTypeEpic, func(p skin.Palette) skin.Color { return p.Epic }},
		{&ColorTypeChore, func(p skin.Palette) skin.Color { return p.Chore }},

		// The query bar border stays quieter than the success colour on a
		// dark background, where a saturated green frame dominates the screen.
		{&unifiedQueryBorder, func(p skin.Palette) skin.Color {
			if p.Dark {
				return skin.Blend(p.Success, p.Bg, 0.35)
			}
			return p.Success
		}},
	} {
		*t.dst = pair(t.token)
	}
	RepoColors = accentPairs()
	epicPalette = accentPairs()

	PanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorMuted)
	FocusedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary)
}

// PaperSequences changes the terminal's default background only for a light
// theme. OSC 111 restores the user's default when b9s exits normally.
func PaperSequences(dark bool) (set, reset string) {
	if dark || lightSkin.Bg == "" {
		return "", ""
	}
	return "\x1b]11;" + string(lightSkin.Bg) + "\x07", "\x1b]111\x07"
}
