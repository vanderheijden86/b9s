package main_test

import (
	"strings"
	"testing"
	"time"
)

// The detail footer advertises e:edit and O:open. Both must act on the detail
// screen (GitHub #15). B9S_TEST_MODE keeps O from starting a real editor.

func detailFooterFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTreeFixture(t, dir, []treeFixtureIssue{{
		ID: "detail-1", Title: "Detail issue", Status: "open", Priority: 2, IssueType: "task",
		CreatedAt: time.Now().Format(time.RFC3339), Description: "Some description.",
	}})
	return dir
}

func TestDetailEKeyOpensEditModalE2E(t *testing.T) {
	dir := detailFooterFixture(t)

	out, err := runTreeTUIWithEnv(t, dir, 3000, []keyStep{
		kd("\r", 300*time.Millisecond), // open the detail
		kd("e", 400*time.Millisecond),
	}, "B9S_TEST_MODE=1")
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
	frame := treeFinalFrame(out)
	if !strings.Contains(frame, "Edit Issue: detail-1") {
		t.Fatalf("expected e on the detail screen to open the edit modal:\n%s", frame)
	}
}

func TestDetailOKeyOpensBeadsFileE2E(t *testing.T) {
	dir := detailFooterFixture(t)

	out, err := runTreeTUIWithEnv(t, dir, 3000, []keyStep{
		kd("\r", 300*time.Millisecond), // open the detail
		kd("O", 400*time.Millisecond),
	}, "B9S_TEST_MODE=1")
	if err != nil {
		t.Fatalf("TUI run failed: %v\n%s", err, out)
	}
	frame := treeFinalFrame(out)
	if !strings.Contains(frame, "Would open beads.jsonl") {
		t.Fatalf("expected O on the detail screen to open the beads file:\n%s", frame)
	}
}
