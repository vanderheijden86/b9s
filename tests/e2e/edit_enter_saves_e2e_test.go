package main_test

import (
	"os"
	"strings"
	"testing"
	"time"
)

// GitHub issue 21: e, Tab to Status, Down to closed, Enter must save.
func TestEditEnterOnStatusSavesE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeMarkFixture())
	pathEnv, logPath := fakeBd(t)

	steps := []keyStep{kd("e", 300*time.Millisecond), kd("\t", 150*time.Millisecond)}
	for i := 0; i < 5; i++ {
		steps = append(steps, kd("\x1b[B", 80*time.Millisecond))
	}
	steps = append(steps, kd("\r", 300*time.Millisecond))

	out, err := runTreeTUIWithEnv(t, dir, 2500, steps, pathEnv)
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"enter or ctrl+s save", "esc cancel"})

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("bd was never called: %v", err)
	}
	args := string(logged)
	if !strings.Contains(args, "update\n") || !strings.Contains(args, "closed") {
		t.Errorf("bd args lack an update to closed:\n%s", args)
	}
}

func TestEditCtrlSSavesE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeMarkFixture())
	pathEnv, logPath := fakeBd(t)

	out, err := runTreeTUIWithEnv(t, dir, 2500, []keyStep{
		kd("e", 300*time.Millisecond),
		kd("\x01", 150*time.Millisecond),
		kd("CtrlS proof ", 150*time.Millisecond),
		kd("\x13", 300*time.Millisecond),
	}, pathEnv)
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"ctrl+s save", "esc cancel"})

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("bd was never called: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(logged)), "\n")
	if args[0] != "update" {
		t.Fatalf("bd command = %q, want update", args[0])
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--title=CtrlS proof ") {
			return
		}
	}
	t.Fatalf("bd args lack the edited title: %q", args)
}
