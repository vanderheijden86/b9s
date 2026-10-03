package main

import (
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/ui"
)

func TestApplyConfiguredSkin(t *testing.T) {
	light, dark := ui.Skins()
	t.Cleanup(func() { ui.UseSkin(light); ui.UseSkin(dark) })

	if err := applyConfiguredSkin(""); err != nil {
		t.Fatalf("empty skin: %v", err)
	}
	if l, d := ui.SkinNames(); l != "sepia" || d != "dracula" {
		t.Fatalf("default slots = %s/%s, want sepia/dracula", l, d)
	}
	if err := applyConfiguredSkin("neon"); err == nil {
		t.Fatal("unknown skin must be an error")
	}
	if err := applyConfiguredSkin(filepath.Join("..", "..", "pkg", "skin", "testdata", "nord.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, d := ui.SkinNames(); d != "nord" {
		t.Fatalf("dark slot = %s, want nord", d)
	}
}
