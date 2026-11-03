package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestMemoryPreviewLauncherIsolatesConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("preview launcher requires a POSIX shell")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("preview launcher requires jq")
	}
	repo := t.TempDir()
	workspace := filepath.Join(repo, "preview")
	bin := filepath.Join(workspace, ".memory-preview", "bin")
	for _, dir := range []string{filepath.Join(repo, "scripts"), filepath.Join(workspace, ".beads"), bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	launcher, err := os.ReadFile("../../scripts/memory-preview")
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, "scripts", "memory-preview"), string(launcher))
	write(filepath.Join(workspace, ".beads", "metadata.json"), `{"graph_mode":"link","graph_ready":true,"dolt_mode":"embedded"}`)
	probe := "#!/bin/sh\nprintf '%s\\n' \"${XDG_CONFIG_HOME:-missing}\" \"${BEADS_DOLT_PASSWORD:-absent}\" \"$#\"\n"
	write(filepath.Join(bin, "bd"), probe)
	write(filepath.Join(repo, "b9s"), probe)
	write(filepath.Join(bin, "make"), "#!/bin/sh\nexit 0\n")
	t.Setenv("B9S_PREVIEW_WORKSPACE", workspace)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(repo, "personal-config"))
	t.Setenv("BEADS_DOLT_PASSWORD", "unrelated-workspace-password")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, args := range [][]string{{"bd", "status"}, {"b9s"}, {"b9s", "tui"}} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, filepath.Join(repo, "scripts", "memory-preview"), args...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("launcher: %v: %s", err, output)
			}
			want := filepath.Join(workspace, ".memory-preview", "config") + "\nabsent\n"
			if args[0] == "bd" {
				want += "1\n"
			} else {
				want += "0\n"
			}
			if string(output) != want {
				t.Fatalf("isolated environment = %q, want %q", output, want)
			}
		})
	}
}

func TestRenderMemoriesShowsTypedEdgesAndBodies(t *testing.T) {
	graph := datasource.GraphPreview{Beads: []datasource.GraphBead{
		{ID: "https://example.test/team/beads/policy", Kind: "memory", Version: "v2", Properties: datasource.GraphProperties{Title: "Policy", Body: "Use integration."}, Owned: []datasource.GraphLink{{Type: "https://example.test/team/types/preview-related-v2", Source: "https://example.test/team/beads/policy", Target: "https://example.test/team/beads/work", Properties: datasource.GraphProperties{Note: "applies to"}}}},
		{ID: "https://example.test/team/beads/work", Kind: "issue", Properties: datasource.GraphProperties{Title: "Adopt policy"}},
	}, Links: []datasource.GraphLink{
		{Type: "types/preview-related-v2", Source: "https://example.test/team/beads/policy", Target: "https://example.test/team/beads/work", Properties: datasource.GraphProperties{Note: "applies to"}},
		{Type: "types/preview-related-v2", Source: "https://example.test/team/beads/work", Target: "https://example.test/team/beads/policy", Properties: datasource.GraphProperties{Note: "cites policy"}},
	}}
	output, err := renderMemories(graph, "policy")
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Policy", "Use integration.", "v2", "related", "applies to", "cites policy", "Adopt policy"} {
		if !strings.Contains(output, text) {
			t.Errorf("missing %q in %s", text, output)
		}
	}
}

func TestRenderMemoriesRejectsUnknownSelection(t *testing.T) {
	_, err := renderMemories(datasource.GraphPreview{}, "missing")
	if err == nil {
		t.Fatal("expected missing memory error")
	}
}

const selectedMemoryRecord = `{"preview":true,"result":{"id":"https://example.test/team/beads/b","type":"https://example.test/team/types/preview-memory-v2","version":"v2","properties":{"title":"Beta","body":"B body"},"owned":[{"id":"links/owned","type":"types/preview-related-v2","source":"https://example.test/team/beads/b","target":"https://example.test/team/beads/a","properties":{"note":"owned note"}},{"id":"links/shared","type":"types/preview-related-v2","source":"https://example.test/team/beads/b","target":"https://example.test/team/beads/a","properties":{"note":"shared note"}}]}}`

const selectedMemoryLinks = `{"preview":true,"result":[{"id":"links/incoming","type":"types/preview-related-v2","source":"https://example.test/team/beads/work","target":"https://example.test/team/beads/b","properties":{"note":"incoming note"}},{"id":"links/shared","type":"types/preview-related-v2","source":"https://example.test/team/beads/b","target":"https://example.test/team/beads/a","properties":{"note":"shared note"}}]}`

func memoryCommandFixture(t *testing.T, show, links string) (project, log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bd script needs a POSIX shell")
	}
	project = t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".beads", "metadata.json"), []byte(`{"graph_mode":"link","graph_ready":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log = filepath.Join(bin, "calls.log")
	script := `#!/bin/sh
case "$*" in *--help) echo "records-json"; exit 0 ;; esac
printf '%s\n' "$*" >> "$B9S_MEMORY_TEST_CALLS"
case "$1" in
list|memories|graph) echo 'inventory forbidden' >&2; exit 1 ;;
show)
  case "$2" in b|beads/b|https://example.test/team/beads/b) ;; *) echo 'unrelated record forbidden' >&2; exit 1 ;; esac
  ` + show + `
  ;;
links)
  [ "$2" = https://example.test/team/beads/b ] || { echo 'unrelated links forbidden' >&2; exit 1; }
  ` + links + `
  ;;
*) echo 'unexpected command' >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("B9S_MEMORY_TEST_CALLS", log)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("B9S_MEMORY", "")
	return project, log
}

func TestPrintOneMemoryWithoutInventory(t *testing.T) {
	for _, id := range []string{"b", "beads/b", "https://example.test/team/beads/b"} {
		t.Run(id, func(t *testing.T) {
			project, log := memoryCommandFixture(t, "echo '"+selectedMemoryRecord+"'", "echo '"+selectedMemoryLinks+"'")
			var stdout, stderr strings.Builder
			if code := runMemories([]string{"--project", project, "--id", id}, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			for _, want := range []string{"Memory Beads (1)", "Beta", "B body", "version v2", "→", "←", "beads/a", "beads/work", "owned note", "incoming note"} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("output lacks %q: %s", want, stdout.String())
				}
			}
			if strings.Count(stdout.String(), "shared note") != 1 {
				t.Errorf("shared Link must appear once: %s", stdout.String())
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := "show " + id + " --json\nlinks https://example.test/team/beads/b --json\n"
			if string(calls) != wantCalls {
				t.Errorf("calls = %q, want %q", calls, wantCalls)
			}
		})
	}
}

// Someone who runs b9s memories expects the feature, so it says why it cannot
// open and exits with the usage-class code rather than a read failure.
func TestMemoriesExplainsUnavailableAndExitsTwo(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var stdout, stderr strings.Builder

	code := runMemories([]string{"--project", t.TempDir(), "--print"}, &stdout, &stderr)

	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if want := "Memories unavailable: this project is not a Memory graph workspace"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestMemoryLeverFromConfigTurnsMemoryOff(t *testing.T) {
	t.Setenv("B9S_MEMORY", "")
	applyMemoryLever(config.Config{Memory: "off"})
	if got := os.Getenv("B9S_MEMORY"); got != "off" {
		t.Errorf("B9S_MEMORY = %q, want off", got)
	}
}

func TestMemoryLeverFromConfigCannotTurnMemoryOn(t *testing.T) {
	t.Setenv("B9S_MEMORY", "off")
	applyMemoryLever(config.Config{Memory: "on"})
	if got := os.Getenv("B9S_MEMORY"); got != "off" {
		t.Errorf("B9S_MEMORY = %q, want off to stay", got)
	}
}
