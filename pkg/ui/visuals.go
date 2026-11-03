package ui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderSparkline creates a textual bar chart of value (0.0 - 1.0)
func RenderSparkline(val float64, width int) string {
	if width <= 0 {
		return ""
	}

	chars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	if math.IsNaN(val) {
		val = 0
	}
	if val < 0 {
		val = 0
	}
	if val > 1 {
		val = 1
	}

	// Calculate fullness
	fullChars := int(val * float64(width))
	remainder := (val * float64(width)) - float64(fullChars)

	var sb strings.Builder
	for i := 0; i < fullChars; i++ {
		sb.WriteString("█")
	}

	if fullChars < width {
		idx := int(remainder * float64(len(chars)))
		// Ensure non-zero values are visible
		if idx == 0 && remainder > 0 {
			idx = 1
		}
		if idx >= len(chars) {
			idx = len(chars) - 1
		}
		if idx > 0 {
			sb.WriteString(chars[idx])
		} else {
			sb.WriteString(" ")
		}
	}

	// Pad
	padding := width - fullChars - 1
	if padding > 0 {
		sb.WriteString(strings.Repeat(" ", padding))
	}

	return sb.String()
}

// GetHeatmapColor returns a color based on score (0-1)
func GetHeatmapColor(score float64, t Theme) lipgloss.TerminalColor {
	if score > 0.8 {
		return t.Primary // Peak/High
	} else if score > 0.5 {
		return t.Feature // Mid-High
	} else if score > 0.2 {
		return t.InProgress // Low-Mid
	}
	return t.Secondary // Low
}

// HeatmapGradientColors defines the color gradient for enhanced heatmap (bv-t4yg)
// Ordered from cold (low count) to hot (high count).
// Each entry pairs the light and dark skin slots.
var HeatmapGradientColors []lipgloss.TerminalColor

// GetHeatGradientColor returns an interpolated color for heatmap intensity (0-1) (bv-t4yg)
func GetHeatGradientColor(intensity float64, t Theme) lipgloss.TerminalColor {
	if intensity <= 0 {
		return adaptiveThemeFg(HeatmapGradientColors[0])
	}
	if intensity >= 1 {
		return adaptiveThemeFg(HeatmapGradientColors[len(HeatmapGradientColors)-1])
	}

	// Map intensity to gradient index
	idx := int(intensity * float64(len(HeatmapGradientColors)-1))
	if idx >= len(HeatmapGradientColors)-1 {
		idx = len(HeatmapGradientColors) - 2
	}

	return adaptiveThemeFg(HeatmapGradientColors[idx+1]) // +1 because 0 is for empty cells
}

// GetHeatGradientColorBg returns a background-friendly color for heatmap cell (bv-t4yg)
// Returns both the background color and appropriate foreground for contrast.
// On 16-color terminals, backgrounds are transparent and foreground uses ANSI-safe colors.
func GetHeatGradientColorBg(intensity float64) (bg lipgloss.TerminalColor, fg lipgloss.TerminalColor) {
	index := 0
	switch {
	case intensity <= 0:
	case intensity >= 0.8:
		index = 5
	case intensity >= 0.6:
		index = 4
	case intensity >= 0.4:
		index = 3
	case intensity >= 0.2:
		index = 2
	default:
		index = 1
	}
	return adaptiveThemeBg(heatBackgrounds[index]), adaptiveThemeFg(heatForegrounds[index])
}

// GetRepoColor returns a consistent color for a repo prefix based on hash
func GetRepoColor(prefix string) lipgloss.AdaptiveColor {
	if prefix == "" {
		return ColorMuted
	}
	// Simple hash based on prefix characters
	hash := 0
	for _, c := range prefix {
		hash = (hash*31 + int(c)) % len(RepoColors)
	}
	if hash < 0 {
		hash = -hash
	}
	return RepoColors[hash%len(RepoColors)]
}

// RenderRepoBadge creates a compact colored badge for a repository prefix
// Example: "api" -> "[API]" with distinctive color
func RenderRepoBadge(prefix string) string {
	if prefix == "" {
		return ""
	}
	// Uppercase and limit to 4 chars for compactness
	display := strings.ToUpper(prefix)
	if len(display) > 4 {
		display = display[:4]
	}

	color := GetRepoColor(prefix)
	return lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render("[" + display + "]")
}
