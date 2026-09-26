package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// b9s ctl branch names an issue that bd has not written yet. The running b9s
// holds the request, and when a reload brings the issue in it selects it and
// limits the tree to its top-level branch.
func TestCtlBranchShowsAnIssueCreatedAfterTheRequestE2E(t *testing.T) {
	skipIfNoScript(t)
	bv := buildBvBinary(t)
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeFilterFixture(t))

	controlDir, err := os.MkdirTemp("/tmp", "b9se2e")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(controlDir)

	ctlResult := make(chan string, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for {
			if regs, _ := filepath.Glob(filepath.Join(controlDir, "*.json")); len(regs) > 0 {
				break
			}
			if time.Now().After(deadline) {
				ctlResult <- "b9s never registered a control socket"
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		ctl := exec.Command(bv, "ctl", "branch", "task-new")
		ctl.Env = append(os.Environ(), "B9S_CONTROL_DIR="+controlDir, "TMUX_PANE=")
		if out, err := ctl.CombinedOutput(); err != nil {
			ctlResult <- "b9s ctl failed: " + err.Error() + ": " + string(out)
			return
		}
		time.Sleep(300 * time.Millisecond)
		appendFixtureIssue(t, tempDir, treeFixtureIssue{
			ID: "task-new", Title: "Freshly created task", Status: "open", Priority: 2, IssueType: "task",
			CreatedAt:    time.Now().Format(time.RFC3339),
			Dependencies: []*treeFixtureDep{{IssueID: "task-new", DependsOnID: "epic-f1", Type: "parent-child"}},
		})
		ctlResult <- ""
	}()

	out, err := runTreeTUIWithEnv(t, tempDir, 4500, nil, "B9S_CONTROL_DIR="+controlDir, "TMUX_PANE=")
	if err != nil {
		t.Fatalf("run TUI: %v", err)
	}
	select {
	case msg := <-ctlResult:
		if msg != "" {
			t.Fatal(msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctl goroutine never finished")
	}
	containsAll(t, out, []string{"Waiting for task-new", "Freshly created task", "Branch epic-f1"})

	if regs, _ := filepath.Glob(filepath.Join(controlDir, "*")); len(regs) != 0 {
		t.Fatalf("b9s left control files behind: %v", regs)
	}
}

func appendFixtureIssue(t *testing.T, dir string, issue treeFixtureIssue) {
	data, err := json.Marshal(issue)
	if err != nil {
		t.Error(err)
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, ".beads", "beads.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Error(err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Error(err)
	}
}
