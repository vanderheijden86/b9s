package preview_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestPreviewReadiness(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	render := exec.CommandContext(ctx, "bash", "-c",
		`source ./lib.sh; preview_parse_args bd-ready 1111111111111111111111111111111111111111; preview_render_manifests /dev/stdout`)
	render.Env = append(os.Environ(),
		"PREVIEW_CMD=readiness-test", "B9S_PREVIEW_KUBECONFIG=../../go.mod",
		"PREVIEW_HOST_SUFFIX=preview.example.test", "PREVIEW_LABEL_DOMAIN=preview.example",
		"PREVIEW_RUNNER_NAMESPACE=runners", "PREVIEW_WORKLOAD_ROLE=preview-workloads",
		"PREVIEW_DEPLOYER_SA=platform/preview-deployer-b9s", "PREVIEW_READER_REF=b9s-reader",
		"PREVIEW_VIEWER_HEADER=X-Preview-Viewer", "PREVIEW_VIEWER=viewer@example.invalid",
		"B9S_PREVIEW_EVIDENCE_DIR="+t.TempDir())
	manifest, err := render.Output()
	if err != nil {
		t.Fatalf("render preview: %v", err)
	}
	var command []string
	decoder := yaml.NewDecoder(bytes.NewReader(manifest))
	for {
		var document struct {
			Kind string `yaml:"kind"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							ReadinessProbe struct {
								Exec struct {
									Command []string `yaml:"command"`
								} `yaml:"exec"`
								HTTPGet struct {
									Path string `yaml:"path"`
									Port string `yaml:"port"`
								} `yaml:"httpGet"`
							} `yaml:"readinessProbe"`
						} `yaml:"containers"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
		}
		if err := decoder.Decode(&document); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if document.Kind == "Deployment" {
			probe := document.Spec.Template.Spec.Containers[0].ReadinessProbe
			command = probe.Exec.Command
			// Execute the identity-only probe too, so the regression demonstrates
			// that identity can answer while the user-facing process is absent.
			if probe.HTTPGet.Port == "identity" {
				command = []string{"wget", "-q", "-T", "1", "-O", "/dev/null", "http://127.0.0.1:7682" + probe.HTTPGet.Path}
			}
		}
	}
	if len(command) == 0 {
		t.Fatal("deployment must declare an executable readiness check")
	}
	for _, scenario := range []struct {
		name  string
		ready bool
	}{
		{"healthy", true},
		{"identity-down", false},
		{"web-down", false},
		{"viewer-refused", false},
		{"tui-missing", false},
		{"tui-blank", false},
		{"web-recovers", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, source := range map[string]string{
				"wget": `#!/bin/sh
set -eu
[ "$1" = -q ] && [ "$2" = -T ] && [ "$3" = 1 ] &&
[ "$4" = -O ] && [ "$5" = /dev/null ] || exit 90
shift 5
case "$*" in
  http://127.0.0.1:7682/__preview) [ "$SCENARIO" != identity-down ] ;;
  "--header=X-Preview-Viewer: viewer@example.invalid http://127.0.0.1:7681/api/session")
    [ "$SCENARIO" != web-down ] && [ "$SCENARIO" != viewer-refused ] ;;
  *) exit 91 ;;
esac
`,
				"tmux": `#!/bin/sh
[ "$*" = "capture-pane -p -t b9s" ] || exit 92
case "$SCENARIO" in
  tui-missing) exit 1 ;;
  tui-blank) printf ' \n\t\n' ;;
  *) printf 'b9s terminal UI\n' ;;
esac
`,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			run := func(state string) bool {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				executable := command[0]
				if executable == "wget" {
					executable = filepath.Join(dir, "wget")
				}
				cmd := exec.CommandContext(ctx, executable, command[1:]...)
				cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "SCENARIO="+state,
					"PREVIEW_VIEWER_HEADER=X-Preview-Viewer", "PREVIEW_VIEWER=viewer@example.invalid")
				err := cmd.Run()
				if ctx.Err() != nil {
					t.Fatal("readiness check exceeded its deadline")
				}
				return err == nil
			}
			if scenario.name == "web-recovers" && run("web-down") {
				t.Fatal("identity must not approve readiness before the web process starts")
			}
			if got := run(scenario.name); got != scenario.ready {
				t.Fatalf("readiness=%v, want %v: gate identity, authenticated web and a nonblank TUI in lib.sh", got, scenario.ready)
			}
		})
	}
}
