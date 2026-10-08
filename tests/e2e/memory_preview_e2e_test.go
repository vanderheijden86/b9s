package main_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
)

// The Memory preview tests need the integration-branch bd, which no release
// ships. B9S_TEST_MEMORY_BD names that binary; without it the tests skip.
// Each test seeds its own embedded graph workspace in a temporary directory
// and runs bd with a minimal environment, so no inherited BEADS_* or Dolt
// setting can point a write at a shared server.
const memoryPreviewBdEnv = "B9S_TEST_MEMORY_BD"

const memoryTUIDeadline = 60 * time.Second

type memoryWorkspace struct {
	dir  string
	env  []string
	bd   string
	work string // canonical path of the Issue that follows the policy
	// Versions of the policy and guide Memories, oldest first.
	policyVersions []string
	guideVersions  []string
}

func memoryPreviewBd(t *testing.T) string {
	t.Helper()
	bd := os.Getenv(memoryPreviewBdEnv)
	if bd == "" {
		t.Skipf("skipping: set %s to the Memory Beads preview bd binary", memoryPreviewBdEnv)
	}
	if _, err := os.Stat(bd); err != nil {
		t.Fatalf("%s: %v", memoryPreviewBdEnv, err)
	}
	return bd
}

// seedMemoryWorkspace builds this graph:
//
//	policy  v1 "Policy body one", v2 "Policy body two"
//	guide   v1 without Links, v2 owning guide -cites-> policy
//	Issue   beads/e2e-work owning e2e-work -follows-> policy
func seedMemoryWorkspace(t *testing.T) memoryWorkspace {
	t.Helper()
	bd := memoryPreviewBd(t)
	dir := t.TempDir()
	home := t.TempDir()
	env := []string{
		"HOME=" + home,
		"PATH=" + filepath.Dir(bd) + ":/usr/bin:/bin:/usr/sbin:/sbin",
		"TMPDIR=" + os.TempDir(),
		"BEADS_DOLT_AUTO_START=0",
		"BD_NON_INTERACTIVE=1",
		"BEADS_ACTOR=e2e",
		"GIT_AUTHOR_NAME=e2e", "GIT_AUTHOR_EMAIL=e2e@example.invalid",
		"GIT_COMMITTER_NAME=e2e", "GIT_COMMITTER_EMAIL=e2e@example.invalid",
	}
	ws := memoryWorkspace{dir: dir, env: env, bd: bd, work: "beads/e2e-work"}

	ws.run(t, "git", "init", "-q")
	ws.run(t, bd, "init", "--graph-mode", "link", "--scope-url", "http://127.0.0.1:8765/e2e/",
		"--prefix", "e2e", "--non-interactive", "--skip-hooks")
	ws.env = append(ws.env, "BEADS_DIR="+filepath.Join(dir, ".beads"))

	ws.run(t, bd, "remember", "Policy body one", "--id", "policy", "--title", "Policy")
	ws.run(t, bd, "remember", "Policy body two", "--update", "policy")
	ws.run(t, bd, "remember", "Guide body", "--id", "guide", "--title", "Guide")
	ws.run(t, bd, "link", "guide", "policy", "--link-type", "types/example-cites",
		"--id", "links/guide-cites-policy", "--properties", `{"note":"Guide cites policy"}`)
	ws.run(t, bd, "create", "Named work", "--id", "e2e-work")
	ws.run(t, bd, "link", ws.work, "policy", "--link-type", "types/example-follows",
		"--properties", `{"note":"Work follows policy"}`)

	client := ws.client(t)
	ws.policyVersions = ws.versions(t, client, "policy")
	ws.guideVersions = ws.versions(t, client, "guide")
	return ws
}

func (ws memoryWorkspace) run(t *testing.T, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = ws.dir
	cmd.WaitDelay = time.Second
	cmd.Env = ws.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", filepath.Base(name), strings.Join(args, " "), err, out)
	}
	return string(out)
}

// client opens the workspace the way b9s memories does, with the preview bd
// first on PATH.
func (ws memoryWorkspace) client(t *testing.T) *datasource.GraphPreviewClient {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "BEADS_") || strings.HasPrefix(key, "BD_") {
			t.Setenv(key, "")
		}
	}
	t.Setenv("PATH", filepath.Dir(ws.bd)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BEADS_DOLT_AUTO_START", "0")
	client, err := datasource.OpenGraphPreview(ws.dir)
	if err != nil {
		t.Fatalf("open graph preview: %v", err)
	}
	return client
}

func (ws memoryWorkspace) versions(t *testing.T, client *datasource.GraphPreviewClient, id string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	versions, err := client.Versions(ctx, id)
	if err != nil {
		t.Fatalf("versions %s: %v", id, err)
	}
	if len(versions) != 2 {
		t.Fatalf("%s retains %d versions, want 2: %+v", id, len(versions), versions)
	}
	return []string{versions[1].Version, versions[0].Version}
}

func TestMemoryPreviewClientReadsRealWorkspace(t *testing.T) {
	ws := seedMemoryWorkspace(t)
	client := ws.client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("body changes produce a new current version", func(t *testing.T) {
		current, err := client.Memory(ctx, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		old, err := client.Memory(ctx, "policy", ws.policyVersions[0])
		if err != nil {
			t.Fatal(err)
		}
		if body := current.Properties.Body; body != "Policy body two" {
			t.Errorf("current body = %q", body)
		}
		if body := old.Properties.Body; body != "Policy body one" {
			t.Errorf("retained body = %q", body)
		}
		if current.Version != ws.policyVersions[1] {
			t.Errorf("current version %s, want %s", current.Version, ws.policyVersions[1])
		}
	})

	t.Run("a retained version carries the Links it owned then", func(t *testing.T) {
		old, err := client.Memory(ctx, "guide", ws.guideVersions[0])
		if err != nil {
			t.Fatal(err)
		}
		current, err := client.Memory(ctx, "guide", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(old.Owned) != 0 {
			t.Errorf("guide v1 owns %d Links, want 0", len(old.Owned))
		}
		if len(current.Owned) != 1 {
			t.Errorf("current guide owns %d Links, want 1", len(current.Owned))
		}
	})

	t.Run("Links reports both directions including Issue sources", func(t *testing.T) {
		links, err := client.Links(ctx, "policy")
		if err != nil {
			t.Fatal(err)
		}
		var sources []string
		for _, link := range links {
			sources = append(sources, link.Source)
		}
		joined := strings.Join(sources, " ")
		if !strings.Contains(joined, "/beads/guide") || !strings.Contains(joined, "/beads/e2e-work") {
			t.Errorf("incoming sources = %v", sources)
		}
	})

	t.Run("search is literal over title and body", func(t *testing.T) {
		hits, err := client.SearchMemories(ctx, "Policy body")
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || !strings.HasSuffix(hits[0].ID, "/beads/policy") {
			t.Errorf("hits = %+v", hits)
		}
		none, err := client.SearchMemories(ctx, "regulation")
		if err != nil {
			t.Fatal(err)
		}
		if len(none) != 0 {
			t.Errorf("a synonym matched: %+v", none)
		}
	})

	t.Run("unknown version is refused with bd's code", func(t *testing.T) {
		_, err := client.Memory(ctx, "policy", "00000000000000000000000000000000")
		if err == nil || !strings.Contains(err.Error(), "revision_unknown") {
			t.Fatalf("err = %v, want revision_unknown", err)
		}
	})

	t.Run("the Issue stays an ordinary open Issue", func(t *testing.T) {
		graph, err := client.Inventory(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, bead := range graph.Beads {
			if strings.HasSuffix(bead.ID, "/"+ws.work) {
				if bead.Kind != "issue" || bead.Properties.Status != "open" || bead.Properties.Title != "Named work" {
					t.Errorf("Issue = %+v", bead)
				}
				return
			}
		}
		t.Errorf("Issue %s missing from inventory", ws.work)
	})
}

func TestMemoryPreviewRefusesOrdinaryWorkspace(t *testing.T) {
	memoryPreviewBd(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "metadata.json"), []byte(`{"backend":"dolt"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, buildBvBinary(t), "memories", "--project", dir, "--print")
	cmd.Env = append(os.Environ(), "BEADS_DOLT_AUTO_START=0")
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 ||
		!strings.Contains(string(out), "Memories unavailable: this project is not a Memory graph workspace") {
		t.Fatalf("err = %v, output:\n%s", err, out)
	}
}

func TestMemoryPreviewPrintsOneMemory(t *testing.T) {
	ws := seedMemoryWorkspace(t)
	out := ws.run(t, buildBvBinary(t), "memories", "--project", ws.dir, "--id", "policy")
	for _, want := range []string{"Policy body two", "example-cites", "example-follows", "Guide", "Named work"} {
		if !strings.Contains(out, want) {
			t.Errorf("printed view lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Guide body") {
		t.Errorf("--id printed another Memory's body:\n%s", out)
	}
}

// memoryStep sends key once the screen shows ready.
type memoryStep struct {
	ready string
	key   string
}

func TestMemoryPreviewBrowserE2E(t *testing.T) {
	skipIfNoScript(t)
	ws := seedMemoryWorkspace(t)
	out := runMemoryBrowser(t, ws, []memoryStep{
		{ready: "2 Memories", key: "/"},
		{ready: "enter search", key: "Policy body"},
		{ready: "/Policy body", key: "\r"},
		{ready: "1 of 2 Memories", key: "\r"},
		{ready: "Policy body two", key: "v"},
		{ready: "#1", key: "j"},
		{ready: "#1", key: "\r"},
		{ready: "not current", key: "q"},
	})
	screen := stripTerminal(out)
	for _, want := range []string{
		"Informational Links",
		"no effect on scheduling",
		"Incoming",
		"example-cites",
		"Memory · Guide",
		"example-follows",
		"Issue · open · Named work",
		"literal search",
		"Retained versions of policy",
		"This is not full history.",
		"Policy body one",
		"retained version " + ws.policyVersions[0][:8],
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("browser never showed %q", want)
		}
	}
}

func runMemoryBrowser(t *testing.T, ws memoryWorkspace, steps []memoryStep) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), memoryTUIDeadline)
	defer cancel()

	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	cmd := scriptTUICommand(ctx, "/bin/sh", "-c",
		"stty rows 40 cols 160; exec "+quote(buildBvBinary(t))+" memories --project "+quote(ws.dir))
	if cmd == nil {
		t.Skip("skipping: script command not available on this platform")
	}
	cmd.Dir = ws.dir
	// screen-256color stops termenv from asking the terminal for its
	// background colour: script never answers, and the five-second wait
	// would swallow the first keys.
	cmd.Env = append(append([]string{}, ws.env...),
		"TERM=screen-256color",
		fmt.Sprintf("B9S_TUI_AUTOCLOSE_MS=%d", (memoryTUIDeadline-5*time.Second).Milliseconds()))

	stdinR, stdinW := io.Pipe()
	cmd.Stdin = stdinR
	t.Cleanup(func() {
		_ = stdinW.Close()
		_ = stdinR.Close()
	})

	outPath := filepath.Join(t.TempDir(), "memories.out")
	stepErr := make(chan error, 1)
	go func() {
		defer stdinW.Close()
		for _, step := range steps {
			if err := waitForScreen(ctx, outPath, step.ready); err != nil {
				stepErr <- err
				_ = stdinW.Close()
				return
			}
			if _, err := io.WriteString(stdinW, step.key); err != nil {
				stepErr <- err
				return
			}
		}
		stepErr <- nil
	}()

	out, err := runCmdToPath(cmd, outPath)
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("b9s memories did not exit within %s; screen:\n%s", memoryTUIDeadline, stripTerminal(out))
	}
	if err != nil {
		t.Fatalf("b9s memories: %v; screen:\n%s", err, stripTerminal(out))
	}
	select {
	case err := <-stepErr:
		if err != nil {
			t.Fatalf("%v; screen:\n%s", err, stripTerminal(out))
		}
	default:
		t.Fatalf("b9s memories exited before every key was sent; screen:\n%s", stripTerminal(out))
	}
	return out
}

// waitForScreen polls the captured output after the alternate screen opens
// until it contains text, bounded by ctx.
func waitForScreen(ctx context.Context, outPath, text string) error {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if out, err := os.ReadFile(outPath); err == nil {
			if i := bytes.Index(out, []byte("\x1b[?1049h")); i >= 0 && strings.Contains(stripTerminal(out[i:]), text) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %q", text)
		case <-tick.C:
		}
	}
}

var terminalControl = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07|\x1b[()][A-Z0-9]|\r`)

func stripTerminal(out []byte) string {
	return terminalControl.ReplaceAllString(string(out), "")
}
