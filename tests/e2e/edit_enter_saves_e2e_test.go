package main_test

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestEditDeferDateSavesE2E(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   string
		existing string
		date     string
		want     []string
	}{
		{name: "set", status: "open", date: "2099-12-01", want: []string{"--until=2099-12-01", "defer", "schedule-1"}},
		{name: "reschedule", status: "deferred", existing: "2099-11-20T16:45:30Z", date: "+3d", want: []string{"--until=+3d", "defer", "schedule-1"}},
		{name: "clear", status: "deferred", existing: "2099-11-20T16:45:30Z", want: []string{"--defer=", "--status=open", "schedule-1", "update"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTreeFixture(t, dir, []treeFixtureIssue{{
				ID: "schedule-1", Title: "Scheduled", Status: tc.status, Priority: 2,
				IssueType: "task", CreatedAt: time.Now().Format(time.RFC3339), DeferUntil: tc.existing,
			}})
			pathEnv, logPath := fakeBd(t)
			out, err := runTreeTUIWithEnv(t, dir, 2500, []keyStep{
				kd("e", 300*time.Millisecond),
				kd("\t", 150*time.Millisecond),
				kd("\t", 150*time.Millisecond),
				kd("\x01\x0b"+tc.date, 150*time.Millisecond),
				kd("\x13", 300*time.Millisecond),
			}, pathEnv)
			if err != nil {
				t.Fatalf("run TUI: %v\n%s", err, out)
			}
			logged, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("save never called bd: %v\n%s", err, out)
			}
			got := strings.Split(strings.TrimSpace(string(logged)), "\n")
			sort.Strings(got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("save arguments = %q, want %q\n%s", got, tc.want, out)
			}
		})
	}
}

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
