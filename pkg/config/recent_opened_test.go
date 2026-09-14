package config

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var (
	earlier = time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	later   = time.Date(2026, 9, 14, 15, 30, 0, 0, time.UTC)
)

func TestMarkOpened_RecordsTimeWithoutMovingProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b")}

	changed := cfg.MarkOpened(recent("b"), later)

	if !changed {
		t.Error("MarkOpened reported no change")
	}
	if got := recentNames(cfg.RecentProjects); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("order = %v, want [a b]", got)
	}
	if got := cfg.RecentProjects[1].OpenedAt; !got.Equal(later) {
		t.Errorf("b opened_at = %v, want %v", got, later)
	}
}

func TestMarkOpened_IgnoresProjectNotInList(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a")}

	if cfg.MarkOpened(recent("ghost"), later) {
		t.Error("MarkOpened changed the list for a project that is not in it")
	}
	if len(cfg.RecentProjects) != 1 {
		t.Errorf("list has %d entries, want 1", len(cfg.RecentProjects))
	}
}

func TestMarkOpened_RecordsTimeInLockedList(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LockRecent = true
	cfg.RecentProjects = []RecentProject{recent("a")}

	cfg.MarkOpened(recent("a"), later)

	if got := cfg.RecentProjects[0].OpenedAt; !got.Equal(later) {
		t.Errorf("opened_at = %v, want %v: a lock freezes membership and order, not success times", got, later)
	}
}

func TestLastOpened_ReturnsNewestOpenedProject(t *testing.T) {
	cfg := DefaultConfig()
	a, b := recent("a"), recent("b")
	a.OpenedAt, b.OpenedAt = earlier, later
	cfg.RecentProjects = []RecentProject{a, b}

	got, ok := cfg.LastOpened(RecentProject{})

	if !ok || got.Name != "b" {
		t.Errorf("LastOpened = %+v, %v; want b", got, ok)
	}
}

func TestLastOpened_SkipsTheFailedProject(t *testing.T) {
	cfg := DefaultConfig()
	a, b := recent("a"), recent("b")
	a.OpenedAt, b.OpenedAt = earlier, later
	cfg.RecentProjects = []RecentProject{a, b}

	got, ok := cfg.LastOpened(recent("b"))

	if !ok || got.Name != "a" {
		t.Errorf("LastOpened(except b) = %+v, %v; want a", got, ok)
	}
}

func TestLastOpened_NeverPicksProjectThatNeverOpened(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b")}

	if got, ok := cfg.LastOpened(RecentProject{}); ok {
		t.Errorf("LastOpened = %+v, want none: no entry has opened_at", got)
	}
}

func TestSaveRecentTo_RoundTripsOpenedAt(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	a := recent("a")
	a.OpenedAt = later

	if err := SaveRecentTo(cfgPath, []RecentProject{a}, false); err != nil {
		t.Fatalf("SaveRecentTo: %v", err)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if len(cfg.RecentProjects) != 1 || !cfg.RecentProjects[0].OpenedAt.Equal(later) {
		t.Errorf("loaded %+v, want a with opened_at %v", cfg.RecentProjects, later)
	}
}

func TestSaveRecentTo_KeepsNewerOpenedAtFromAnotherWindow(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	other := recent("a")
	other.OpenedAt = later
	if err := SaveRecentTo(cfgPath, []RecentProject{other}, false); err != nil {
		t.Fatalf("SaveRecentTo (other window): %v", err)
	}

	mine := recent("a")
	mine.OpenedAt = earlier
	if err := SaveRecentTo(cfgPath, []RecentProject{mine}, false); err != nil {
		t.Fatalf("SaveRecentTo (this window): %v", err)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if len(cfg.RecentProjects) != 1 || !cfg.RecentProjects[0].OpenedAt.Equal(later) {
		t.Errorf("loaded %+v, want the newer opened_at %v kept", cfg.RecentProjects, later)
	}
}
