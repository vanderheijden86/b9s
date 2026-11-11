package main_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClipboardOSC52AfterY(t *testing.T) {
	dir := detailFooterFixture(t)
	configHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(configHome, "b9s"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "b9s", "config.yaml"),
		[]byte("clipboard: osc52\nui:\n  default_view: board\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runTreeTUIWithEnv(t, dir, 2500, []keyStep{
		kd("y", 100*time.Millisecond),
	}, "XDG_CONFIG_HOME="+configHome, "TERM=xterm-256color")
	if err != nil {
		t.Fatalf("TUI failed: %v\n%s", err, out)
	}
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("detail-1")) + "\a"
	if !strings.Contains(string(out), want) {
		t.Fatalf("y must emit OSC52 with the selected issue ID, missing %q:\n%s", want, out)
	}
	if !strings.Contains(string(out), "Copied detail-1 to clipboard") {
		t.Fatalf("copy did not report success:\n%s", out)
	}
}
