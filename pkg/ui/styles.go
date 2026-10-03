package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ══════════════════════════════════════════════════════════════════════════════
// DESIGN TOKENS - Consistent spacing, colors, and visual language
// ══════════════════════════════════════════════════════════════════════════════

// Spacing constants for consistent layout (in characters)
const (
	SpaceXS = 1
	SpaceSM = 2
	SpaceMD = 3
	SpaceLG = 4
	SpaceXL = 6
)

// ══════════════════════════════════════════════════════════════════════════════
// COLOR PALETTE - each colour pairs the light and dark skin slots. applySkins
// in palette.go assigns them; see the skin package for the token meanings.
// ══════════════════════════════════════════════════════════════════════════════

var (
	ColorBg, ColorBgDark, ColorBgSubtle, ColorBgHighlight lipgloss.AdaptiveColor
	ColorBoardSelection, ColorSelectionText, ColorMatchBg lipgloss.AdaptiveColor
	ColorHeaderText, ColorBorder                          lipgloss.AdaptiveColor
	ColorText, ColorSubtext, ColorMuted                   lipgloss.AdaptiveColor

	ColorPrimary, ColorSecondary, ColorInfo lipgloss.AdaptiveColor
	ColorSuccess, ColorWarning, ColorDanger lipgloss.AdaptiveColor

	ColorStatusOpen, ColorStatusInProgress, ColorStatusBlocked lipgloss.AdaptiveColor
	ColorStatusDeferred, ColorStatusPinned, ColorStatusHooked  lipgloss.AdaptiveColor
	ColorStatusReview, ColorStatusClosed, ColorStatusTombstone lipgloss.AdaptiveColor

	ColorStatusOpenBg, ColorStatusInProgressBg, ColorStatusBlockedBg lipgloss.AdaptiveColor
	ColorStatusDeferredBg, ColorStatusPinnedBg, ColorStatusHookedBg  lipgloss.AdaptiveColor
	ColorStatusReviewBg, ColorStatusClosedBg, ColorStatusTombstoneBg lipgloss.AdaptiveColor

	ColorPrioCritical, ColorPrioHigh, ColorPrioMedium, ColorPrioLow         lipgloss.AdaptiveColor
	ColorPrioCriticalBg, ColorPrioHighBg, ColorPrioMediumBg, ColorPrioLowBg lipgloss.AdaptiveColor

	ColorTypeBug, ColorTypeFeature, ColorTypeTask, ColorTypeEpic, ColorTypeChore lipgloss.AdaptiveColor

	// RepoColors gives each repo prefix a stable accent.
	RepoColors []lipgloss.AdaptiveColor
)

// ══════════════════════════════════════════════════════════════════════════════
// PANEL STYLES - For split view layouts
// ══════════════════════════════════════════════════════════════════════════════

var (
	// PanelStyle is the default style for unfocused panels. Its border is
	// muted so the focused panel is the only bright one on screen.
	PanelStyle lipgloss.Style

	// FocusedPanelStyle is the style for focused panels
	FocusedPanelStyle lipgloss.Style
)

// ══════════════════════════════════════════════════════════════════════════════
// BADGE RENDERING - Polished, consistent badge styles
// ══════════════════════════════════════════════════════════════════════════════

// RenderPriorityBadge returns a styled priority badge
// Priority values: 0=Critical, 1=High, 2=Medium, 3=Low, 4=Backlog
func RenderPriorityBadge(priority int) string {
	var fg, bg lipgloss.AdaptiveColor
	var label string

	switch priority {
	case 0:
		fg, bg, label = ColorPrioCritical, ColorPrioCriticalBg, "P0"
	case 1:
		fg, bg, label = ColorPrioHigh, ColorPrioHighBg, "P1"
	case 2:
		fg, bg, label = ColorPrioMedium, ColorPrioMediumBg, "P2"
	case 3:
		fg, bg, label = ColorPrioLow, ColorPrioLowBg, "P3"
	case 4:
		fg, bg, label = ColorMuted, ColorBgSubtle, "P4"
	default:
		fg, bg, label = ColorMuted, ColorBgSubtle, "P?"
	}

	return lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Padding(0, 0).
		Render(label)
}

// RenderStatusBadge returns a styled status badge
func RenderStatusBadge(status string) string {
	var fg, bg lipgloss.AdaptiveColor
	var label string

	switch status {
	case "open":
		fg, bg, label = ColorStatusOpen, ColorStatusOpenBg, "OPEN"
	case "in_progress":
		fg, bg, label = ColorStatusInProgress, ColorStatusInProgressBg, "PROG"
	case "blocked":
		fg, bg, label = ColorStatusBlocked, ColorStatusBlockedBg, "BLKD"
	case "deferred":
		fg, bg, label = ColorStatusDeferred, ColorStatusDeferredBg, "DEFR"
	case "pinned":
		fg, bg, label = ColorStatusPinned, ColorStatusPinnedBg, "PIN"
	case "hooked":
		fg, bg, label = ColorStatusHooked, ColorStatusHookedBg, "HOOK"
	case "review":
		fg, bg, label = ColorStatusReview, ColorStatusReviewBg, "REVW"
	case "closed":
		fg, bg, label = ColorStatusClosed, ColorStatusClosedBg, "DONE"
	case "tombstone":
		fg, bg, label = ColorStatusTombstone, ColorStatusTombstoneBg, "TOMB"
	default:
		fg, bg, label = ColorMuted, ColorBgSubtle, "????"
	}

	style := lipgloss.NewStyle().Foreground(fg).Padding(0, 0)
	if status == "closed" && !lipgloss.HasDarkBackground() {
		return style.Render(label)
	}
	return style.Background(bg).Render(label)
}

// ══════════════════════════════════════════════════════════════════════════════
// METRIC VISUALIZATION - Mini-bars and rank badges
// ══════════════════════════════════════════════════════════════════════════════

// RenderMiniBar renders a mini horizontal bar for a value between 0 and 1
func RenderMiniBar(value float64, width int, t Theme) string {
	if width <= 0 {
		return ""
	}
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}

	filled := int(value * float64(width))
	if filled > width {
		filled = width
	}

	// Choose color based on value
	var barColor lipgloss.AdaptiveColor
	if value >= 0.75 {
		barColor = t.Open // Green/Success
	} else if value >= 0.5 {
		barColor = t.Feature // Orange/Warning
	} else if value >= 0.25 {
		barColor = t.InProgress // Cyan/Info
	} else {
		barColor = t.Secondary // Muted
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return t.Renderer.NewStyle().Foreground(barColor).Render(bar)
}

// RenderRankBadge renders a rank badge like "#1" with color based on percentile
func RenderRankBadge(rank, total int) string {
	if total == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("#?")
	}

	percentile := float64(rank) / float64(total)

	var color lipgloss.AdaptiveColor
	if percentile <= 0.1 {
		color = ColorSuccess // Top 10%
	} else if percentile <= 0.25 {
		color = ColorInfo // Top 25%
	} else if percentile <= 0.5 {
		color = ColorWarning // Top 50%
	} else {
		color = ColorMuted // Bottom 50%
	}

	return lipgloss.NewStyle().
		Foreground(color).
		Render(fmt.Sprintf("#%d", rank))
}

// ══════════════════════════════════════════════════════════════════════════════
// DIVIDERS AND SEPARATORS
// ══════════════════════════════════════════════════════════════════════════════

// RenderDivider renders a horizontal divider line
func RenderDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(ColorBgHighlight).
		Render(strings.Repeat("─", width))
}

// RenderSubtleDivider renders a more subtle divider using dots
func RenderSubtleDivider(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render(strings.Repeat("·", width))
}
