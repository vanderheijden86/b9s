// tree_find_mode_e2e_test.go - End-to-end proof that Ctrl-F switches the tree
// search from filter to find, and that & toggles occur (ADR 0057).
package main_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"
)

// sgrPattern matches colour and style escapes. A find hit is underlined, and
// lipgloss styles each word of an underlined title on its own.
var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plainFinalFrame(out []byte) string {
	return sgrPattern.ReplaceAllString(treeFinalFrame(out), "")
}

func searchAlphaKeys(extra ...keyStep) []keyStep {
	keys := []keyStep{kd("/", 150*time.Millisecond)}
	for _, r := range "alpha" {
		keys = append(keys, kd(string(r), 80*time.Millisecond))
	}
	keys = append(keys, kd("\r", 150*time.Millisecond))
	return append(keys, extra...)
}

func TestTreeFindModeKeepsEveryRowE2E(t *testing.T) {
	skipIfNoScript(t)
	tempDir := t.TempDir()
	writeTreeFixture(t, tempDir, makeLabelSearchFixture(t))
	pass, fail := 0, 0
	check := func(ok bool, format string, args ...any) {
		if ok {
			pass++
			return
		}
		fail++
		t.Errorf(format, args...)
	}

	out, err := runTreeTUI(t, tempDir, 2600, searchAlphaKeys())
	if err != nil {
		t.Fatalf("filter run failed: %v\noutput:\n%s", err, out)
	}
	frame := plainFinalFrame(out)
	check(strings.Contains(frame, "Alpha bug work") && !strings.Contains(frame, "Beta bug work"),
		"/ alpha did not filter the tree:\n%s", frame)

	out, err = runTreeTUI(t, tempDir, 2800, searchAlphaKeys(kd("\x06", 200*time.Millisecond)))
	if err != nil {
		t.Fatalf("find run failed: %v\noutput:\n%s", err, out)
	}
	frame = plainFinalFrame(out)
	check(strings.Contains(frame, "find / alpha"), "query bar does not name find mode:\n%s", frame)
	for _, title := range []string{"Alpha bug work", "Beta bug work", "Gamma bug work", "Alpha feature work", "Beta feature work"} {
		check(strings.Contains(frame, title), "find mode hid %q:\n%s", title, frame)
	}

	out, err = runTreeTUI(t, tempDir, 2800, searchAlphaKeys(kd("&", 200*time.Millisecond)))
	if err != nil {
		t.Fatalf("occur run failed: %v\noutput:\n%s", err, out)
	}
	frame = plainFinalFrame(out)
	check(strings.Contains(frame, "OCCUR[alpha]"), "& did not enter occur mode:\n%s", frame)

	fmt.Printf("=== TREE FIND MODE DONE pass=%d fail=%d ===\n", pass, fail)
}

func TestTreeOccurEscapeRoutingE2E(t *testing.T) {
	dir := t.TempDir()
	writeTreeFixture(t, dir, makeLabelSearchFixture(t))
	out, err := runTreeTUI(t, dir, 2800, searchAlphaKeys(
		kd("&", 150*time.Millisecond),
		kd("\x1b", 200*time.Millisecond),
	))
	if err != nil {
		t.Fatalf("TUI failed: %v\n%s", err, out)
	}
	frame := plainFinalFrame(out)
	if strings.Contains(frame, "OCCUR[") || strings.Contains(frame, "Quit b9s?") || !strings.Contains(frame, "alpha") {
		t.Fatalf("Esc must leave occur and retain the query:\n%s", frame)
	}
}
