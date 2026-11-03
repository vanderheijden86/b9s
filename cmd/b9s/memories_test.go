package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/bdrun"
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
case "$*" in *--help) [ "$B9S_MEMORY_TEST_HELP" = ready ] || exit 1; echo "records-json"; exit 0 ;; esac
printf '%s\n' "$*" >> "$B9S_MEMORY_TEST_CALLS"
case "$1" in
list)
  [ -n "$B9S_MEMORY_TEST_INVENTORY" ] || { echo 'inventory forbidden' >&2; exit 1; }
  printf '%s\n' "$B9S_MEMORY_TEST_INVENTORY"
  ;;
memories|graph) echo 'inventory forbidden' >&2; exit 1 ;;
show)
  case "$2" in b|beads/b|https://example.test/team/beads/b) ;; *) echo 'unrelated record forbidden' >&2; exit 1 ;; esac
  ` + show + `
  ;;
links)
  if [ "$2" = https://example.test/team/beads/a ] && [ -n "$B9S_MEMORY_TEST_INVENTORY" ]; then
    echo '{"preview":true,"result":[]}'
    exit 0
  fi
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
	t.Setenv("B9S_MEMORY_TEST_HELP", "ready")
	t.Setenv("B9S_MEMORY_TEST_INVENTORY", "")
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

func TestPrintOneMemoryReadFailures(t *testing.T) {
	tests := []struct {
		name, show, links, want string
	}{
		{"missing", `echo '{"code":"not_found","message":"Memory b does not exist"}'; exit 1`, "", "not_found"},
		{"issue", `echo '{"preview":true,"result":{"id":"https://example.test/team/beads/b","type":"types/preview-issue-v2"}}'`, "", "not a Memory"},
		{"other type", `echo '{"preview":true,"result":{"id":"https://example.test/team/beads/b","type":"types/unknown"}}'`, "", "not a Memory"},
		{"malformed record", `echo '{'`, "", "parse graph record"},
		{"unsupported record", `echo '{"preview":false,"result":{}}'`, "", "did not return a graph record"},
		{"null record", `echo '{"preview":true,"result":null}'`, "", "graph record has no ID"},
		{"missing record ID", `echo '{"preview":true,"result":{"type":"types/preview-memory-v2"}}'`, "", "graph record has no ID"},
		{"different record", `echo '{"preview":true,"result":{"id":"https://example.test/team/beads/other","type":"types/preview-memory-v2"}}'`, "", `bd returned record`},
		{"malformed links", "", `echo '{'`, "parse graph preview links"},
		{"unsupported links", "", `echo '{"preview":false,"result":[]}'`, "did not return graph preview links"},
		{"wrong links shape", "", `echo '{"preview":true,"result":{}}'`, "parse graph preview links"},
		{"refused links", "", `echo '{"code":"denied","message":"Links unavailable"}'; exit 1`, "denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			show, links := tt.show, tt.links
			if show == "" {
				show = "echo '" + selectedMemoryRecord + "'"
			}
			if links == "" {
				links = "echo '" + selectedMemoryLinks + "'"
			}
			project, log := memoryCommandFixture(t, show, links)
			var stdout, stderr strings.Builder
			if code := runMemories([]string{"--project", project, "--id", "b"}, &stdout, &stderr); code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stdout=%q stderr=%q, want failure containing %q", stdout.String(), stderr.String(), tt.want)
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := "show b --json\n"
			if tt.show == "" {
				wantCalls += "links https://example.test/team/beads/b --json\n"
			}
			if string(calls) != wantCalls {
				t.Fatalf("calls = %q, want %q", calls, wantCalls)
			}
		})
	}
}

func TestLoadOneMemoryDeadline(t *testing.T) {
	for _, stage := range []string{"show", "links"} {
		t.Run(stage, func(t *testing.T) {
			show, links := "echo '"+selectedMemoryRecord+"'", "echo '"+selectedMemoryLinks+"'"
			if stage == "show" {
				show = "sleep 30"
			} else {
				links = "sleep 30"
			}
			project, _ := memoryCommandFixture(t, show, links)
			client, err := datasource.OpenGraphPreview(project)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, err = loadMemoryGraph(ctx, client, "b")
			if !errors.Is(err, bdrun.ErrTimeout) {
				t.Fatalf("error = %v, want bd timeout", err)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("read took %s, want bounded deadline", elapsed)
			}
		})
	}
}

func TestLoadOneMemoryPreservesOwnedRecord(t *testing.T) {
	project, _ := memoryCommandFixture(t, "echo '"+selectedMemoryRecord+"'", "echo '"+selectedMemoryLinks+"'")
	client, err := datasource.OpenGraphPreview(project)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	graph, err := loadMemoryGraph(ctx, client, "b")
	if err != nil {
		t.Fatal(err)
	}
	owned := graph.Beads[0].Owned
	if len(owned) != 2 || owned[0].ID != "links/owned" || owned[1].ID != "links/shared" {
		t.Fatalf("owned Links changed while merging incident Links: %+v", owned)
	}
}

func TestPrintOneMemoryCapabilityGating(t *testing.T) {
	for _, state := range []string{"off", "not_enabled", "unsupported"} {
		t.Run(state, func(t *testing.T) {
			project, log := memoryCommandFixture(t, "exit 99", "exit 99")
			switch state {
			case "off":
				t.Setenv("B9S_MEMORY", "off")
			case "not_enabled":
				project = t.TempDir()
			case "unsupported":
				t.Setenv("B9S_MEMORY_TEST_HELP", "unsupported")
			}
			var stdout, stderr strings.Builder
			if code := runMemories([]string{"--project", project, "--id", "b"}, &stdout, &stderr); code != 2 {
				t.Fatalf("exit %d, want 2: %s", code, stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Memories unavailable:") || !strings.Contains(stderr.String(), "("+state+")") {
				t.Fatalf("stdout=%q stderr=%q, want unavailable (%s)", stdout.String(), stderr.String(), state)
			}
			if calls, err := os.ReadFile(log); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected data commands %q (read error %v)", calls, err)
			}
		})
	}
}

func TestPrintAllMemoriesUsesInventory(t *testing.T) {
	for _, printFlag := range []bool{false, true} {
		name := "nonterminal"
		if printFlag {
			name = "print"
		}
		t.Run(name, func(t *testing.T) {
			project, log := memoryCommandFixture(t, "exit 99", "echo '"+selectedMemoryLinks+"'")
			t.Setenv("B9S_MEMORY_TEST_INVENTORY", `{"preview":true,"result":{"items":[{"id":"https://example.test/team/beads/a","type":"types/preview-memory-v2","properties":{"title":"Alpha","body":"A body"}},{"id":"https://example.test/team/beads/b","type":"types/preview-memory-v2","properties":{"title":"Beta","body":"B body"},"owned":[{"id":"links/owned","type":"types/preview-related-v2","source":"https://example.test/team/beads/b","target":"https://example.test/team/beads/a","properties":{"note":"owned note"}}]},{"id":"https://example.test/team/beads/work","type":"types/preview-issue-v2","properties":{"title":"Work item"}}],"hasMore":false}}`)
			args := []string{"--project", project}
			if printFlag {
				args = append(args, "--print")
			}
			var stdout, stderr strings.Builder
			if code := runMemories(args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr.String())
			}
			for _, want := range []string{"Memory Beads (2)", "Alpha", "Beta", "A body", "B body", "owned note", "incoming note", "Work item"} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("output lacks %q: %s", want, stdout.String())
				}
			}
			calls, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := "list --format records-json --all\nlinks https://example.test/team/beads/a --json\nlinks https://example.test/team/beads/b --json\n"
			if string(calls) != wantCalls {
				t.Fatalf("calls = %q, want %q", calls, wantCalls)
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
