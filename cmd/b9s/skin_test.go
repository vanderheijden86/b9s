package main

import (
	"path/filepath"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/config"
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

func TestWebSkinsFollowTheConfiguredSkin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	skins := webSkins()

	if l, d := skins(); l.Name != "sepia" || d.Name != "dracula" {
		t.Fatalf("without a config: %s/%s, want sepia/dracula", l.Name, d.Name)
	}

	nord, err := filepath.Abs(filepath.Join("..", "..", "pkg", "skin", "testdata", "nord.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.UI.Skin = nord
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if l, d := skins(); l.Name != "sepia" || d.Name != "nord" {
		t.Fatalf("after :skins chose nord: %s/%s, want sepia/nord", l.Name, d.Name)
	}

	cfg.UI.Skin = "neon"
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if l, d := skins(); l.Name != "sepia" || d.Name != "dracula" {
		t.Fatalf("unknown skin: %s/%s, want the built-in sepia/dracula", l.Name, d.Name)
	}
}
