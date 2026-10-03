package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/skin"
)

// Selected cards print the skin's selection text on its selection surfaces,
// so every built-in skin must keep that text readable.
func TestSelectedCardTextColor_HighContrast(t *testing.T) {
	if ColorSelectionText.Light != string(lightSkin.SelectionText) || ColorSelectionText.Dark != string(darkSkin.SelectionText) {
		t.Fatalf("ColorSelectionText = %+v, want the slots' selection text", ColorSelectionText)
	}
	for _, p := range []skin.Palette{lightSkin, darkSkin} {
		for _, background := range []skin.Color{p.Selection, p.BoardSelection} {
			if ratio := contrastRatio(string(p.SelectionText), string(background)); ratio < 4.5 {
				t.Fatalf("%s: selected card text contrast on %s = %.2f:1, want at least 4.5:1", p.Name, background, ratio)
			}
		}
	}
}

func TestBuildBoardColumnHeaderText_Narrow(t *testing.T) {
	got := buildBoardColumnHeaderText("OPEN (3)", ColumnStats{
		Total:     3,
		P0Count:   1,
		P1Count:   2,
		OldestAge: 10 * 24 * time.Hour,
	}, 80, SwimByStatus, ColOpen)

	if got != "OPEN (3)" {
		t.Fatalf("header = %q, want %q", got, "OPEN (3)")
	}
}

func TestBuildBoardColumnHeaderText_Medium(t *testing.T) {
	got := buildBoardColumnHeaderText("OPEN (4)", ColumnStats{
		Total:   4,
		P0Count: 2,
		P1Count: 1,
	}, 120, SwimByStatus, ColOpen)

	if !strings.Contains(got, "P0:2") || !strings.Contains(got, "P1:1") {
		t.Fatalf("header = %q, want P0/P1 tokens", got)
	}
	if strings.ContainsAny(got, "🔴🟡⚠️⏱") {
		t.Fatalf("header should use plain text tokens, got %q", got)
	}
}

func TestBuildBoardColumnHeaderText_WideWithBlockedAndAge(t *testing.T) {
	got := buildBoardColumnHeaderText("IN PROGRESS (5)", ColumnStats{
		Total:        5,
		P0Count:      1,
		P1Count:      2,
		BlockedCount: 3,
		OldestAge:    14 * 24 * time.Hour,
	}, 160, SwimByStatus, ColInProgress)

	for _, token := range []string{"P0:1", "P1:2", "BLK:3", "AGE:2w"} {
		if !strings.Contains(got, token) {
			t.Fatalf("header = %q, missing %q", got, token)
		}
	}
	if strings.ContainsAny(got, "🔴🟡⚠️⏱") {
		t.Fatalf("header should use plain text tokens, got %q", got)
	}
}
