package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWritesWaitWhileProjectOpens(t *testing.T) {
	// bdPath names a file that does not exist, so no real bd can ever run.
	w := &IssueWriter{available: true, bdPath: filepath.Join(t.TempDir(), "no-bd"), checkout: Checkout{dir: t.TempDir()}}
	w.SetOpening("beta")

	result, ok := w.CloseIssue("x-1", "")().(BdResultMsg)

	if !ok {
		t.Fatal("CloseIssue produced no BdResultMsg")
	}
	if result.Success || result.Error == nil || !strings.Contains(result.Error.Error(), "wait until beta has opened") {
		t.Errorf("result = %+v, want the write refused until beta has opened", result)
	}
}
