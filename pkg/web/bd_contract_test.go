package web

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/model"
	"github.com/vanderheijden86/b9s/pkg/ui"
)

// TestBDContract_EveryWriteB9sMakes runs each write the TUI and b9s web make
// through the bd on PATH, in a throwaway embedded project, and reads the
// result back the way b9s reads it. It is how a bd release is checked
// against b9s: put that bd first on PATH and run this test (docs/testing.md).
//
// Every flag in updateFields and createFields is exercised, so a flag added
// to the browser's allowlist is tested against the real bd without further
// edits here.
func TestBDContract_EveryWriteB9sMakes(t *testing.T) {
	if testing.Short() {
		t.Skip("starts the real bd")
	}
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH")
	}
	projectDir := contractProject(t)
	w := ui.NewIssueWriter()
	checkout, ok := ui.NewCheckout(projectDir)
	if !ok {
		t.Fatal("the temp project is not a checkout")
	}
	w.SetCheckout(checkout)

	parent := contractCreate(t, w, map[string]string{"title": "Parent", "type": "epic"})

	createValues := map[string]string{
		"title": "Child", "description": "The description", "design": "The design",
		"acceptance": "The criteria", "notes": "The notes", "priority": "1",
		"type": "bug", "assignee": "alice", "parent": parent, "labels": "one,two",
	}
	contractCoversEveryFlag(t, "createFields", createFields, createValues)
	child := contractCreate(t, w, createValues)
	got := contractIssue(t, projectDir, child)
	contractExpect(t, "after create", got, map[string]string{
		"title": "Child", "description": "The description", "design": "The design",
		"acceptance": "The criteria", "notes": "The notes", "priority": "1",
		"type": "bug", "assignee": "alice", "parent": parent, "labels": "one,two",
	})

	other := contractCreate(t, w, map[string]string{"title": "Other", "type": "task"})
	updateValues := map[string]string{
		"title": "Child renamed", "description": "New description", "design": "New design",
		"acceptance": "New criteria", "notes": "New notes", "status": "in_progress",
		"priority": "3", "type": "feature", "assignee": contractActor, "parent": other,
		"add-label": "three", "remove-label": "one",
	}
	contractCoversEveryFlag(t, "updateFields", updateFields, updateValues)
	contractRun(t, "update", w.UpdateIssue(child, updateValues))
	got = contractIssue(t, projectDir, child)
	contractExpect(t, "after update", got, map[string]string{
		"title": "Child renamed", "description": "New description", "design": "New design",
		"acceptance": "New criteria", "notes": "New notes", "status": "in_progress",
		"priority": "3", "type": "feature", "assignee": contractActor, "parent": other,
		"labels": "three,two",
	})

	// The TUI edit form replaces labels with --set-labels and creates with
	// --actor, two flags the browser does not send.
	contractRun(t, "set-labels", w.UpdateIssue(child, map[string]string{"set-labels": "four"}))
	contractExpect(t, "after set-labels", contractIssue(t, projectDir, child), map[string]string{"labels": "four"})
	byActor := contractCreate(t, w, map[string]string{"title": "By actor", "actor": "carol"})
	if got := contractIssue(t, projectDir, byActor); got.CreatedBy != "carol" {
		t.Errorf("--actor: created_by = %q, want carol", got.CreatedBy)
	}

	contractRun(t, "comment", w.AddComment(child, "-starts with a dash"))
	if got := contractIssue(t, projectDir, child); len(got.Comments) != 1 || got.Comments[0].Text != "-starts with a dash" {
		t.Errorf("comment: %+v, want one comment with the dashed text", got.Comments)
	}

	until := time.Now().Add(72 * time.Hour).Format("2006-01-02")
	contractRun(t, "defer", w.DeferIssue(other, until))
	if got := contractIssue(t, projectDir, other); got.Status != model.StatusDeferred || got.DeferUntil == nil {
		t.Errorf("defer: status %q, defer_until %v; want deferred with a date", got.Status, got.DeferUntil)
	}

	contractRun(t, "set status", w.SetStatus(other, "open"))
	contractRun(t, "set statuses", w.SetStatuses([]string{other, byActor}, "blocked"))
	for _, id := range []string{other, byActor} {
		if got := contractIssue(t, projectDir, id); got.Status != model.StatusBlocked {
			t.Errorf("set statuses: %s is %q, want blocked", id, got.Status)
		}
	}

	contractRun(t, "close", w.CloseIssue(child, "done here"))
	if got := contractIssue(t, projectDir, child); got.Status != model.StatusClosed {
		t.Errorf("close with a reason: status %q, want closed", got.Status)
	}
	contractRun(t, "close many", w.CloseIssues([]string{other, byActor}))
	for _, id := range []string{other, byActor} {
		if got := contractIssue(t, projectDir, id); got.Status != model.StatusClosed {
			t.Errorf("close many: %s is %q, want closed", id, got.Status)
		}
	}

	spare1 := contractCreate(t, w, map[string]string{"title": "Spare one"})
	spare2 := contractCreate(t, w, map[string]string{"title": "Spare two"})
	spare3 := contractCreate(t, w, map[string]string{"title": "Spare three"})
	contractRun(t, "delete", w.DeleteIssue(spare1))
	contractRun(t, "delete many", w.DeleteIssues([]string{spare2, spare3}))
	issues := contractLoad(t, projectDir)
	for _, id := range []string{spare1, spare2, spare3} {
		for _, is := range issues {
			if is.ID == id && is.Status != model.StatusTombstone {
				t.Errorf("delete: %s is still %q", id, is.Status)
			}
		}
	}
}

const contractActor = "contract-bot"

// contractProject makes an embedded project with the bd on PATH. Every
// BEADS_* and BD_* variable of the calling shell is blanked, so no server
// setting or password can point bd at a shared database, and the test
// refuses to write unless bd made an embedded project.
func contractProject(t *testing.T) string {
	t.Helper()
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "BEADS_") || strings.HasPrefix(name, "BD_") {
			t.Setenv(name, "")
		}
	}
	projectDir := t.TempDir()
	t.Setenv("BEADS_DIR", filepath.Join(projectDir, ".beads"))
	t.Setenv("BEADS_DOLT_AUTO_START", "0")
	t.Setenv("BD_NON_INTERACTIVE", "1")
	// bd refuses to close an issue assigned to anyone but the actor, and the
	// actor otherwise comes from the machine's git config.
	t.Setenv("BEADS_ACTOR", contractActor)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bd", "init", "--prefix", "ct", "--quiet", "--skip-hooks")
	cmd.Dir = projectDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd init: %v\n%s", err, out)
	}
	var meta struct {
		DoltMode string `json:"dolt_mode"`
	}
	raw, err := os.ReadFile(filepath.Join(projectDir, ".beads", "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &meta); err != nil || meta.DoltMode != "embedded" {
		t.Fatalf("bd init did not make an embedded project (dolt_mode=%q, err=%v); refusing to write", meta.DoltMode, err)
	}
	return projectDir
}

// contractRun runs one writer command to its result and fails on a bd error.
func contractRun(t *testing.T, what string, cmd tea.Cmd) ui.BdResultMsg {
	t.Helper()
	result, ok := cmd().(ui.BdResultMsg)
	if !ok {
		t.Fatalf("%s: the writer returned no bd result", what)
	}
	if !result.Success {
		t.Fatalf("%s: bd failed: %v", what, result.Error)
	}
	return result
}

// contractCreate creates an issue and returns the ID b9s read from bd's
// output, which is how the TUI selects the new issue.
func contractCreate(t *testing.T, w *ui.IssueWriter, fields map[string]string) string {
	t.Helper()
	result := contractRun(t, "create "+fields["title"], w.CreateIssue(fields))
	if result.IssueID == "" {
		t.Fatalf("create %s: no issue ID in bd's output %q", fields["title"], result.Output)
	}
	return result.IssueID
}

func contractLoad(t *testing.T, projectDir string) []model.Issue {
	t.Helper()
	opened, failure := datasource.OpenProject(datasource.OpenTarget{Name: "ct", Dir: projectDir})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if opened.Source.Type != datasource.SourceTypeDoltEmbedded {
		t.Fatalf("read %s, want %s", opened.Source.Type, datasource.SourceTypeDoltEmbedded)
	}
	return opened.Issues
}

func contractIssue(t *testing.T, projectDir, id string) model.Issue {
	t.Helper()
	for _, is := range contractLoad(t, projectDir) {
		if is.ID == id {
			return is
		}
	}
	t.Fatalf("%s is missing from what b9s reads", id)
	return model.Issue{}
}

// contractCoversEveryFlag fails when the browser may send a flag this test
// does not send, so the allowlist cannot grow past what bd was tested with.
func contractCoversEveryFlag(t *testing.T, name string, allowed map[string]bool, values map[string]string) {
	t.Helper()
	for flag := range allowed {
		if _, ok := values[flag]; !ok {
			t.Errorf("%s allows --%s, which this contract test does not send to bd", name, flag)
		}
	}
}

// contractExpect compares the fields of got that want names.
func contractExpect(t *testing.T, when string, got model.Issue, want map[string]string) {
	t.Helper()
	parent := ""
	for _, d := range got.Dependencies {
		if d.Type == model.DepParentChild {
			parent = d.DependsOnID
		}
	}
	labels := append([]string(nil), got.Labels...)
	sort.Strings(labels)
	actual := map[string]string{
		"title": got.Title, "description": got.Description, "design": got.Design,
		"acceptance": got.AcceptanceCriteria, "notes": got.Notes, "status": string(got.Status),
		"priority": strconv.Itoa(got.Priority), "type": string(got.IssueType),
		"assignee": got.Assignee, "parent": parent, "labels": strings.Join(labels, ","),
	}
	for field, value := range want {
		if actual[field] != value {
			t.Errorf("%s: %s = %q, want %q", when, field, actual[field], value)
		}
	}
}
