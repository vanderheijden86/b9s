package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMouseCapturePersistsE2E(t *testing.T) {
	skipIfNoScript(t)
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeFilterFixture(t))
	configHome := writeConfigFile(t, "config.yaml", `# personal settings
ui:
  columns:
    priority: hide
custom:
  value: keep-me
`)
	configPath := filepath.Join(configHome, "b9s", "config.yaml")
	run := func(keys []keyStep) string {
		t.Helper()
		out, err := runTreeTUIWithEnv(t, dir, 1800, keys, "XDG_CONFIG_HOME="+configHome)
		if err != nil {
			t.Fatalf("run TUI: %v\n%s", err, out)
		}
		return string(out)
	}
	checkConfig := func(mouse string) {
		t.Helper()
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"mouse: " + mouse, "# personal settings", "priority: hide", "value: keep-me"} {
			if !strings.Contains(string(data), want) {
				t.Errorf("config misses %q:\n%s", want, data)
			}
		}
	}
	const enableCellMotion = "\x1b[?1002h"
	toggle := []keyStep{kd(":mouse\r", 200*time.Millisecond)}

	out := run(toggle)
	if !strings.Contains(out, enableCellMotion) || !strings.Contains(out, "Mouse released") {
		t.Fatalf("default capture or runtime release missing:\n%s", out)
	}
	checkConfig("false")

	if out := run(nil); strings.Contains(out, enableCellMotion) {
		t.Fatalf("restart with mouse: false enabled capture:\n%s", out)
	}
	checkConfig("false")

	if out := run(toggle); !strings.Contains(out, enableCellMotion) || !strings.Contains(out, "Mouse captured") {
		t.Fatalf("runtime capture missing:\n%s", out)
	}
	checkConfig("true")

	if out := run(nil); !strings.Contains(out, enableCellMotion) {
		t.Fatalf("restart with mouse: true did not enable capture:\n%s", out)
	}
	checkConfig("true")
}
