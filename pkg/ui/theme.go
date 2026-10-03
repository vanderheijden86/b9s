package ui

import (
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/skin"
)

// formTheme colours huh forms from the skin slot on screen.
func formTheme(theme Theme) *huh.Theme {
	p := lightSkin
	if theme.Renderer.HasDarkBackground() {
		p = darkSkin
	}
	c := func(v skin.Color) lipgloss.Color { return lipgloss.Color(string(v)) }
	t := huh.ThemeBase()
	t.Focused.Base = t.Focused.Base.BorderForeground(c(p.Primary))
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(c(p.Epic)).Bold(true)
	t.Focused.NoteTitle = t.Focused.Title
	t.Focused.Description = t.Focused.Description.Foreground(c(p.Subtext))
	t.Focused.Option = t.Focused.Option.Foreground(c(p.Text))
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(c(p.Success))
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(c(p.Primary))
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(c(p.Primary))
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(c(p.Primary))
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(c(p.Subtext))
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(c(p.HeaderText)).Background(c(p.Primary))
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(c(p.Text)).Background(c(p.BgDeep))
	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	return t
}

func nextThemeMode(mode config.ThemeMode) config.ThemeMode {
	switch mode {
	case config.ThemeAuto:
		return config.ThemeLight
	case config.ThemeLight:
		return config.ThemeDark
	default:
		return config.ThemeAuto
	}
}

// themeHint names the skin a forced theme shows. Automatic depends on the
// terminal, so it keeps its own name.
func themeHint(mode config.ThemeMode) string {
	switch mode {
	case config.ThemeLight:
		return "theme:" + lightSkin.Name
	case config.ThemeDark:
		return "theme:" + darkSkin.Name
	default:
		return "theme:auto"
	}
}

// terminalSequenceForThemeMode paints the terminal background with the chosen
// slot's skin. A skin with no background keeps the terminal's own.
func terminalSequenceForThemeMode(mode config.ThemeMode, originalDark bool) string {
	paint := func(p skin.Palette) string {
		if p.Bg == "" {
			return "\x1b]111\x07"
		}
		return "\x1b]11;" + string(p.Bg) + "\x07"
	}
	switch mode {
	case config.ThemeLight:
		return paint(lightSkin)
	case config.ThemeDark:
		return paint(darkSkin)
	default:
		if originalDark {
			return "\x1b]111\x07"
		}
		return paint(lightSkin)
	}
}

// TermProfile holds the detected terminal color profile. Computed once at
// package init so every style helper can branch without re-detecting.
var TermProfile colorprofile.Profile

func init() {
	TermProfile = colorprofile.Detect(os.Stdout, os.Environ())
}

// ThemeBg returns the given hex color for TrueColor terminals and
// lipgloss.NoColor{} otherwise, so 16/256-color terminals use the
// terminal's own background instead of a down-converted approximation
// that may clash with palettes like Solarized.
func ThemeBg(hex string) lipgloss.TerminalColor {
	if TermProfile < colorprofile.TrueColor {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(hex)
}

// ThemeFg returns the given hex color for ANSI256+ terminals and a safe
// ANSI white (color 7) for 16-color or lower terminals.
func ThemeFg(hex string) lipgloss.TerminalColor {
	if TermProfile < colorprofile.ANSI256 {
		return lipgloss.ANSIColor(7)
	}
	return lipgloss.Color(hex)
}

type Theme struct {
	Renderer *lipgloss.Renderer

	// Colors
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Subtext   lipgloss.AdaptiveColor

	// Status
	Open       lipgloss.AdaptiveColor
	InProgress lipgloss.AdaptiveColor
	Blocked    lipgloss.AdaptiveColor
	Deferred   lipgloss.AdaptiveColor
	Pinned     lipgloss.AdaptiveColor
	Hooked     lipgloss.AdaptiveColor
	Closed     lipgloss.AdaptiveColor
	Tombstone  lipgloss.AdaptiveColor

	// Types
	Bug     lipgloss.AdaptiveColor
	Feature lipgloss.AdaptiveColor
	Task    lipgloss.AdaptiveColor
	Epic    lipgloss.AdaptiveColor
	Chore   lipgloss.AdaptiveColor

	// UI Elements
	Border    lipgloss.AdaptiveColor
	Highlight lipgloss.AdaptiveColor
	Muted     lipgloss.AdaptiveColor

	// Styles
	Base     lipgloss.Style
	Selected lipgloss.Style
	Column   lipgloss.Style
	Header   lipgloss.Style

	// Pre-computed delegate styles (bv-o4cj optimization)
	// These are created once at startup instead of per-frame
	MutedText         lipgloss.Style // Age, muted info
	InfoText          lipgloss.Style // Comments
	InfoBold          lipgloss.Style // Search scores
	SecondaryText     lipgloss.Style // ID, assignee
	PrimaryBold       lipgloss.Style // Selection indicator
	PriorityUpArrow   lipgloss.Style // Priority hint ↑
	PriorityDownArrow lipgloss.Style // Priority hint ↓
	TriageStar        lipgloss.Style // Top pick ⭐
	TriageUnblocks    lipgloss.Style // Unblocks indicator 🔓
	TriageUnblocksAlt lipgloss.Style // Secondary unblocks ↪
}

// DefaultTheme returns the theme of the light and dark skin slots.
func DefaultTheme(r *lipgloss.Renderer) Theme {
	t := Theme{
		Renderer: r,

		Primary:   ColorPrimary,
		Secondary: ColorSecondary,
		Subtext:   ColorSubtext,

		Open:       ColorStatusOpen,
		InProgress: ColorStatusInProgress,
		Blocked:    ColorStatusBlocked,
		Deferred:   ColorStatusDeferred,
		Pinned:     ColorStatusPinned,
		Hooked:     ColorStatusHooked,
		Closed:     ColorStatusClosed,
		Tombstone:  ColorStatusTombstone,

		Bug:     ColorTypeBug,
		Feature: ColorTypeFeature,
		Epic:    ColorTypeEpic,
		Task:    ColorTypeTask,
		Chore:   ColorTypeChore,

		Border:    ColorBorder,
		Highlight: ColorBgHighlight,
		Muted:     ColorMuted,
	}

	t.Base = r.NewStyle().Foreground(ColorText)

	t.Selected = r.NewStyle().
		Background(t.Highlight).
		Bold(true)

	t.Header = r.NewStyle().
		Background(t.Primary).
		Foreground(ColorHeaderText).
		Bold(true).
		Padding(0, 1)

	// Pre-computed delegate styles (bv-o4cj optimization)
	// Reduces ~16 NewStyle() allocations per visible item per frame
	t.MutedText = r.NewStyle().Foreground(ColorMuted)
	t.InfoText = r.NewStyle().Foreground(ColorInfo)
	t.InfoBold = r.NewStyle().Foreground(ColorInfo).Bold(true)
	t.SecondaryText = r.NewStyle().Foreground(t.Secondary)
	t.PrimaryBold = r.NewStyle().Foreground(t.Primary).Bold(true)
	t.PriorityUpArrow = r.NewStyle().Foreground(ThemeFg("#FF6B6B")).Bold(true)
	t.PriorityDownArrow = r.NewStyle().Foreground(ThemeFg("#4ECDC4")).Bold(true)
	t.TriageStar = r.NewStyle().Foreground(ThemeFg("#FFD700"))
	t.TriageUnblocks = r.NewStyle().Foreground(ThemeFg("#50FA7B"))
	t.TriageUnblocksAlt = r.NewStyle().Foreground(ThemeFg("#6272A4"))

	return t
}

func (t Theme) GetStatusColor(s string) lipgloss.AdaptiveColor {
	switch s {
	case "open":
		return t.Open
	case "in_progress":
		return t.InProgress
	case "blocked":
		return t.Blocked
	case "closed":
		return t.Closed
	default:
		return t.Subtext
	}
}

func (t Theme) GetTypeIcon(typ string) (string, lipgloss.AdaptiveColor) {
	switch typ {
	case "bug":
		return "●", t.Bug
	case "feature":
		return "▲", t.Feature
	case "task":
		return "✔", t.Task
	case "epic":
		return "♦", t.Epic // 1-cell diamond (⚡ is 2 cells, misaligns)
	case "chore":
		return "○", t.Chore
	default:
		return "·", t.Subtext
	}
}

// TestTheme returns a theme suitable for use in tests (uses nil renderer).
func TestTheme() Theme {
	return DefaultTheme(lipgloss.NewRenderer(os.Stdout))
}
