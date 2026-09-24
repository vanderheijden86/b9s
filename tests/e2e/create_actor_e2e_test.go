package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBd puts a bd on PATH that records its arguments, so a create reaches no
// real database.
func fakeBd(t *testing.T) (pathEnv, logPath string) {
	t.Helper()
	binDir := t.TempDir()
	logPath = filepath.Join(binDir, "bd-args.log")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done >> '" + logPath + "'\necho 'Created issue: fake-1'\n"
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return "PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"), logPath
}

func TestCreateModalNamesActorE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeMarkFixture())
	pathEnv, _ := fakeBd(t)

	out, err := runTreeTUIWithEnv(t, dir, 1500, []keyStep{k("\x0e")}, pathEnv, "BEADS_ACTOR=e2e-actor")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"as @e2e-actor"})
}

func TestCreateSendsActorAsCreatorAndAssigneeE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeMarkFixture())
	pathEnv, logPath := fakeBd(t)

	_, err := runTreeTUIWithEnv(t, dir, 2500, []keyStep{
		k("\x0e"), kd("Actor check", 300*time.Millisecond), kd("\x13", 300*time.Millisecond),
	}, pathEnv, "BEADS_ACTOR=e2e-actor")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("bd was never called: %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(logged)), "\n")
	for _, want := range []string{"create", "--actor=e2e-actor", "--assignee=e2e-actor", "--title=Actor check"} {
		found := false
		for _, a := range args {
			found = found || a == want
		}
		if !found {
			t.Errorf("bd args %q lack %q", args, want)
		}
	}
}

func TestDetailShowsCreatorApartFromAssigneeE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PTY e2e test in -short mode")
	}
	dir := t.TempDir()
	writeTreeFixture(t, dir, []treeFixtureIssue{{
		ID: "who-1", Title: "Handed over", Status: "open", Priority: 2, IssueType: "task",
		CreatedAt: time.Now().Format(time.RFC3339), CreatedBy: "maker-e2e", Assignee: "holder-e2e",
	}})

	out, err := runTreeTUI(t, dir, 1500, []keyStep{k("\r")})
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	containsAll(t, out, []string{"Creator", "@maker-e2e", "Assignee", "@holder-e2e"})
}
