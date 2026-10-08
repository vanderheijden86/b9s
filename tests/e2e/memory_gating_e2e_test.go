// memory_gating_e2e_test.go - End-to-end proof that one b9s binary opens the
// Memory views with a Memory-capable bd and explains their absence otherwise
// (ADR 0045).
package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gatingStockBd answers like a released bd: no versions or links command, and
// the Issues come from bd export.
const gatingStockBd = `#!/bin/sh
case "$1" in
versions|links) echo "Error: unknown command \"$1\" for \"bd\"" >&2; exit 1 ;;
esac
case "$*" in *--help) exit 0 ;; esac
[ "$1" = "export" ] || exit 2
echo '{"id":"gate-1","title":"Ship the gate","status":"open","priority":1,"issue_type":"task"}'
`

// gatingPreviewBd answers like the Memory Beads preview: help mentions
// records-json, and the Issues and one Memory come from the graph read.
const gatingPreviewBd = `#!/bin/sh
case "$*" in *--help) echo "records-json"; exit 0 ;; esac
case "$1" in
list) printf '%s\n' '{"preview":true,"result":{"hasMore":false,"items":[{"id":"s/beads/gate-1","type":"s/types/preview-issue-v2","properties":{"title":"Ship the gate","status":"open","issue_type":"task","description":"Issue body"},"owned":[]},{"id":"s/beads/m","type":"s/types/preview-memory-v2","properties":{"title":"Gate decision","body":"Decision body"},"owned":[]}]}}' ;;
graph) echo '{"preview":true,"result":{"complete":true,"nodes":[{"id":"s/beads/gate-1"},{"id":"s/beads/m"}],"links":[{"id":"s/links/g-follows-m","source":"s/beads/gate-1","target":"s/beads/m","type":"s/types/example-follows","properties":{"note":"why"}}]}}' ;;
*) exit 3 ;;
esac
`

// graphWorkspaceFixture lays out an embedded Memory graph workspace with the
// given bd on a private PATH, and returns the project and the env for b9s.
func graphWorkspaceFixture(t *testing.T, bd string) (dir string, env []string) {
	t.Helper()
	dir = resolvedTempDir(t)
	beads := filepath.Join(dir, ".beads")
	noms := filepath.Join(beads, "embeddeddolt", "emb", ".dolt", "noms")
	if err := os.MkdirAll(noms, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	files := map[string]string{
		filepath.Join(beads, "metadata.json"):                   `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"emb","graph_mode":"link","graph_ready":true}`,
		filepath.Join(noms, "manifest"):                         "manifest-1",
		filepath.Join(noms, "vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv"): "journal",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(bd), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, []string{
		"PATH=" + bin + string(os.PathListSeparator) + "/usr/bin:/bin",
		"XDG_CACHE_HOME=" + t.TempDir(),
	}
}

// runGatingTUI shows the Issue detail, which requests the Memory graph
// (ADR 0051), and then presses M.
func runGatingTUI(t *testing.T, bd string, extraEnv ...string) string {
	t.Helper()
	dir, env := graphWorkspaceFixture(t, bd)
	keys := []keyStep{kd("d", 800*time.Millisecond), kd("M", 1200*time.Millisecond)}
	out, err := runTreeTUIWithEnv(t, dir, 3500, keys, append(env, extraEnv...)...)
	if err != nil {
		t.Fatalf("TUI run failed: %v\noutput:\n%s", err, truncateOutput(string(out), 3000))
	}
	return stripTerminalCodes(out)
}

func TestMemoryKeyExplainsStockBdE2E(t *testing.T) {
	screen := runGatingTUI(t, gatingStockBd)

	for _, want := range []string{"Ship the gate", "Memories unavailable: bd at", "has no Memory Beads support"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q\nscreen:\n%s", want, truncateOutput(screen, 3000))
		}
	}
	if strings.Contains(screen, "MEMORY LINKS") {
		t.Errorf("stock bd shows the MEMORY LINKS column\nscreen:\n%s", truncateOutput(screen, 3000))
	}
}

func TestMemoryKeyOpensWithPreviewBdE2E(t *testing.T) {
	screen := runGatingTUI(t, gatingPreviewBd)

	for _, want := range []string{"Ship the gate", "MEMORY LINKS", "Gate decision"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q\nscreen:\n%s", want, truncateOutput(screen, 3000))
		}
	}
	if strings.Contains(screen, "Memories unavailable") {
		t.Errorf("preview bd still explains Memories away\nscreen:\n%s", truncateOutput(screen, 3000))
	}
}

func TestMemoryLeverTurnsPreviewOffE2E(t *testing.T) {
	screen := runGatingTUI(t, gatingPreviewBd, "B9S_MEMORY=off")

	// A graph workspace refuses bd export, so its Issues must still come
	// from the graph read while the Memory views stay off.
	for _, want := range []string{"Ship the gate", "Memory views are turned off (B9S_MEMORY=off)"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q\nscreen:\n%s", want, truncateOutput(screen, 3000))
		}
	}
	if strings.Contains(screen, "Gate decision") || strings.Contains(screen, "MEMORY LINKS") {
		t.Errorf("lever left the Memory visible\nscreen:\n%s", truncateOutput(screen, 3000))
	}
}
