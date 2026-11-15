package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/pkg/config"
)

func pressFKey(m Model, n int) (Model, tea.Cmd) {
	keys := map[int]tea.KeyType{1: tea.KeyF1, 2: tea.KeyF2, 3: tea.KeyF3, 4: tea.KeyF4,
		5: tea.KeyF5, 6: tea.KeyF6, 7: tea.KeyF7, 8: tea.KeyF8}
	updated, cmd := m.Update(tea.KeyMsg{Type: keys[n]})
	return updated.(Model), cmd
}

// F1-F8 open the eight fleet screens the way k9s function keys open
// resources (ADR 0068). F4 is the :projects table.
func TestFunctionKeysOpenTheFleetScreens(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	want := map[int]fleetView{1: fleetAgents, 2: fleetLogs, 3: fleetContext,
		5: fleetXray, 6: fleetPulse, 7: fleetCommits, 8: fleetCommit}
	for n, view := range want {
		m := keybindingTestModel(t, nil)
		m, _ = pressFKey(m, n)
		if !m.FleetVisible() {
			t.Fatalf("F%d did not open the fleet", n)
		}
		got := m.fleet.view()
		if got != view && !(m.fleet.hasPending && m.fleet.pending == view) {
			t.Errorf("F%d opened view %v, want %v", n, got, view)
		}
	}
	m := keybindingTestModel(t, nil)
	_, cmd := pressFKey(m, 4)
	if cmd == nil {
		t.Fatal("F4 returned no command")
	}
	if _, ok := cmd().(OpenProjectTableMsg); !ok {
		t.Errorf("F4 message = %T, want OpenProjectTableMsg", cmd())
	}
}

// Inside the fleet, a function key jumps straight to another screen.
func TestFunctionKeysSwitchScreensInsideTheFleet(t *testing.T) {
	b, _ := newFleetTest(t, fleetAgents)
	b, _ = fleetDrive(t, b, "f2")
	if b.view() != fleetLogs || len(b.events) == 0 {
		t.Fatalf("F2: view %v with %d events", b.view(), len(b.events))
	}
	b, _ = fleetDrive(t, b, "f3")
	if b.view() != fleetContext {
		t.Fatalf("F3: view %v", b.view())
	}
	b, _ = fleetDrive(t, b, "f6")
	if b.view() != fleetPulse || len(b.stack) != 1 {
		t.Fatalf("F6: view %v stack %v", b.view(), b.stack)
	}
	_, out := fleetDrive(t, b, "f4")
	if !out.close || out.command != "projects" {
		t.Errorf("F4 outcome = %+v", out)
	}
}

// F2 before the first scan opens the logs as soon as the scan names a session.
func TestFunctionKeyWaitsForTheFirstScan(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	c, _ := fleetFixture(t)
	b := NewFleetBrowser(FleetDeps{Collector: c}, Theme{Renderer: lipgloss.DefaultRenderer()}, 120, 30, fleetAgents)
	b, _ = fleetDrive(t, b, "f2")
	if !b.hasPending || b.pending != fleetLogs {
		t.Fatalf("F2 before a scan: pending %v %v", b.hasPending, b.pending)
	}
	b = runFleetCmd(b, b.scanCmd())
	if b.view() != fleetLogs || b.hasPending {
		t.Errorf("after the scan: view %v pending %v", b.view(), b.hasPending)
	}
}

// Help is only ?: F1 belongs to the agents screen.
func TestF1DoesNotOpenHelp(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	m := keybindingTestModel(t, nil)
	m, _ = pressFKey(m, 1)
	if m.showHelp {
		t.Fatal("F1 opened the help overlay")
	}
}

func TestFunctionKeysAreBuiltIn(t *testing.T) {
	for _, key := range []string{"F2", "F8"} {
		_, errs := ResolveHotkeys(config.Hotkeys{{Name: "c", ShortCut: key, Command: "chore"}})
		if len(errs) != 1 {
			t.Fatalf("%s is built in, errs = %v", key, errs)
		}
	}
}
