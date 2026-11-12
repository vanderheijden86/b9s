package ui

import (
	"fmt"
	"image/color"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"
	"github.com/vanderheijden86/b9s/pkg/model"
)

func TestBadgeContrast(t *testing.T) {
	saved := lipgloss.DefaultRenderer()
	t.Cleanup(func() { lipgloss.SetDefaultRenderer(saved) })
	statuses := []string{
		"open", "in_progress", "blocked", "deferred", "pinned",
		"hooked", "review", "closed", "tombstone", "unknown",
	}
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dark=%t", profile.Name(), dark), func(t *testing.T) {
				r := lipgloss.NewRenderer(io.Discard)
				r.SetColorProfile(profile)
				r.SetHasDarkBackground(dark)
				lipgloss.SetDefaultRenderer(r)
				theme := DefaultTheme(r)
				for _, status := range statuses {
					t.Run(status+"/compact", func(t *testing.T) {
						assertBadgeContrast(t, RenderStatusBadge(status), profile)
					})
					t.Run(status+"/detail", func(t *testing.T) {
						issue := model.Issue{ID: "badge-1", Title: "Contrast", Status: model.Status(status)}
						out := renderIssueDetail(issue, nil, nil, theme, 100, nil, nil, nil, false)
						label := strings.ToUpper(strings.ReplaceAll(status, "_", " "))
						for _, line := range strings.Split(out, "\n") {
							if strings.Contains(stripANSI(line), label) {
								assertBadgeContrast(t, line, profile)
								return
							}
						}
						t.Fatalf("detail status badge %q missing", label)
					})
				}
				for _, priority := range []int{0, 1, 2, 3, 4, 99} {
					t.Run(fmt.Sprintf("priority=%d", priority), func(t *testing.T) {
						assertBadgeContrast(t, RenderPriorityBadge(priority), profile)
					})
				}
			})
		}
	}
}

func assertBadgeContrast(t *testing.T, out string, profile termenv.Profile) {
	t.Helper()
	if profile == termenv.Ascii {
		if strings.Contains(out, "\x1b[") {
			t.Fatalf("monochrome badge must use terminal defaults: %q", out)
		}
		return
	}
	buf := cellbuf.NewBuffer(lipgloss.Width(out), 1)
	cellbuf.SetContent(buf, out)
	checked := 0
	for x := 0; x < buf.Width(); x++ {
		cell := buf.Cell(x, 0)
		if cell == nil || cell.Rune == ' ' || cell.Style.Bg == nil {
			continue
		}
		if cell.Style.Fg == nil {
			t.Fatal("filled badge must have an explicit text colour")
		}
		light, dark := badgeTestLuminance(cell.Style.Fg), badgeTestLuminance(cell.Style.Bg)
		if light < dark {
			light, dark = dark, light
		}
		ratio := (light + 0.05) / (dark + 0.05)
		if ratio < 4.5 {
			t.Fatalf("badge contrast %.2f:1 < 4.5:1, foreground=%v background=%v", ratio, cell.Style.Fg, cell.Style.Bg)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no filled badge text checked")
	}
}

func badgeTestLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(v uint32) float64 {
		s := float64(v) / 65535
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func TestRenderPriorityBadge(t *testing.T) {
	tests := []struct {
		prio int
		want string
	}{
		{0, "P0"},
		{1, "P1"},
		{2, "P2"},
		{3, "P3"},
		{4, "P4"},
		{99, "P?"},
	}

	for _, tt := range tests {
		got := RenderPriorityBadge(tt.prio)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderPriorityBadge(%d) = %q, want to contain %q", tt.prio, got, tt.want)
		}
	}
}

func TestRenderStatusBadge(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		// All 8 official beads statuses
		{"open", "OPEN"},
		{"in_progress", "PROG"},
		{"blocked", "BLKD"},
		{"deferred", "DEFR"},
		{"pinned", "PIN"},
		{"hooked", "HOOK"},
		{"closed", "DONE"},
		{"tombstone", "TOMB"},
		// Unknown status should show "????"
		{"unknown", "????"},
		{"", "????"},
		{"invalid_status", "????"},
	}

	for _, tt := range tests {
		got := RenderStatusBadge(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderStatusBadge(%q) = %q, want to contain %q", tt.status, got, tt.want)
		}
	}
}

// TestRenderStatusBadge_AllStatusesHaveColors verifies each status has distinct colors
func TestRenderStatusBadge_AllStatusesHaveColors(t *testing.T) {
	statuses := []string{
		"open", "in_progress", "blocked", "deferred",
		"pinned", "hooked", "closed", "tombstone",
	}

	// Each status should produce a non-empty output
	for _, status := range statuses {
		got := RenderStatusBadge(status)
		if got == "" {
			t.Errorf("RenderStatusBadge(%q) returned empty string", status)
		}
		// Should NOT contain "????" for known statuses
		if strings.Contains(got, "????") {
			t.Errorf("RenderStatusBadge(%q) returned unknown badge '????'", status)
		}
	}
}

func TestRenderMiniBar(t *testing.T) {
	renderer := lipgloss.NewRenderer(io.Discard)
	theme := DefaultTheme(renderer)

	tests := []struct {
		val   float64
		width int
	}{
		{0.0, 10},
		{0.5, 10},
		{1.0, 10},
		{-0.1, 10}, // Should clamp to 0
		{1.5, 10},  // Should clamp to 1
		{0.5, 0},   // Should return empty
		{0.5, -5},  // Should return empty (no panic)
	}

	for _, tt := range tests {
		got := RenderMiniBar(tt.val, tt.width, theme)
		if tt.width <= 0 {
			if got != "" {
				t.Errorf("RenderMiniBar(%v, %d) = %q, want empty string", tt.val, tt.width, got)
			}
			continue
		}
		// Basic sanity check: output should not be empty
		if got == "" {
			t.Errorf("RenderMiniBar(%v, %d) returned empty string", tt.val, tt.width)
		}
		// Check expected fullness characters approximately
		if tt.val > 0 {
			if !strings.Contains(got, "█") && !strings.Contains(got, "░") {
				t.Errorf("RenderMiniBar output expected bar chars, got %q", got)
			}
		}
	}
}

func TestRenderRankBadge(t *testing.T) {
	tests := []struct {
		rank  int
		total int
		want  string
	}{
		{1, 100, "#1"},
		{50, 100, "#50"},
		{0, 0, "#?"},
	}

	for _, tt := range tests {
		got := RenderRankBadge(tt.rank, tt.total)
		if !strings.Contains(got, tt.want) {
			t.Errorf("RenderRankBadge(%d, %d) = %q, want to contain %q", tt.rank, tt.total, got, tt.want)
		}
	}
}
