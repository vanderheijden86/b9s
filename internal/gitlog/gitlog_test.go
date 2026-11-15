package gitlog

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssuesFromTrailers(t *testing.T) {
	got := Issues("Body text.\n\nRefs: bd-m5js, bd-m5js.1\nCloses: bd-abc\nRefs: bd-m5js")
	if strings.Join(got, ",") != "bd-m5js,bd-m5js.1,bd-abc" {
		t.Errorf("issues = %v", got)
	}
}

func TestDrawBranchAndMerge(t *testing.T) {
	// m merges b into a; a and b both descend from r.
	cs := []Commit{
		{Hash: "m", Parents: []string{"a", "b"}},
		{Hash: "b", Parents: []string{"r"}},
		{Hash: "a", Parents: []string{"r"}},
		{Hash: "r"},
	}
	Draw(cs)
	want := []string{"●─╮", "│ ●", "● │", "●─╯"}
	for i, c := range cs {
		if c.Graph != want[i] {
			t.Errorf("row %d (%s) = %q, want %q", i, c.Hash, c.Graph, want[i])
		}
	}
}

func TestDrawLinear(t *testing.T) {
	cs := []Commit{{Hash: "c", Parents: []string{"b"}}, {Hash: "b", Parents: []string{"a"}}, {Hash: "a"}}
	Draw(cs)
	for _, c := range cs {
		if c.Graph != "●" {
			t.Errorf("%s = %q", c.Hash, c.Graph)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Ada", "GIT_AUTHOR_EMAIL=ada@example.com", "GIT_COMMITTER_NAME=Ada", "GIT_COMMITTER_EMAIL=ada@example.com", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestReadAndShowRealRepo(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "feat: first\n\nRefs: bd-aaa")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644)
	git(t, dir, "commit", "-q", "-am", "fix: second")
	cs, err := Read(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].Subject != "fix: second" || cs[1].Issues[0] != "bd-aaa" || cs[0].Author != "Ada" {
		t.Fatalf("commits = %+v", cs)
	}
	if !strings.Contains(strings.Join(cs[0].Refs, ","), "main") {
		t.Errorf("refs = %v", cs[0].Refs)
	}
	d, err := Show(context.Background(), dir, cs[0].Hash)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 || d.Files[0].Path != "a.txt" || d.Files[0].Deleted != 1 || !strings.Contains(d.Diff, "-two") {
		t.Errorf("detail = %+v", d)
	}
	grep, _ := Read(context.Background(), dir, Options{Grep: "bd-aaa"})
	if len(grep) != 1 {
		t.Errorf("grep = %d commits", len(grep))
	}
}
