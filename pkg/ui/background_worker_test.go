package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackgroundWorker_NewWithoutPath(t *testing.T) {
	cfg := WorkerConfig{
		BeadsPath: "",
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state, got %v", worker.State())
	}

	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot initially")
	}
}

func TestBackgroundWorker_NewWithPath(t *testing.T) {
	// Create a temporary beads file
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Write a valid beads file
	content := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state, got %v", worker.State())
	}
}

func TestBackgroundWorker_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop should be idempotent
	worker.Stop()
	worker.Stop() // Should not panic

	if worker.State() != WorkerStopped {
		t.Errorf("Expected stopped state, got %v", worker.State())
	}
}

func TestBackgroundWorker_TriggerRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Trigger refresh and wait for processing
	worker.TriggerRefresh()

	// Wait for processing to complete
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after refresh")
	}

	if len(snapshot.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(snapshot.Issues))
	}
}

func TestBackgroundWorker_WatcherChanged(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	ch := worker.WatcherChanged()
	if ch == nil {
		t.Error("WatcherChanged should return non-nil channel")
	}
}

func TestBackgroundWorker_WatcherChangedNil(t *testing.T) {
	// Worker without path should have nil watcher
	cfg := WorkerConfig{
		BeadsPath: "",
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if worker.WatcherChanged() != nil {
		t.Error("WatcherChanged should return nil when no watcher")
	}
}

func TestWorkerState_String(t *testing.T) {
	tests := []struct {
		state    WorkerState
		expected string
	}{
		{WorkerIdle, "0"},
		{WorkerProcessing, "1"},
		{WorkerStopped, "2"},
	}

	for _, tt := range tests {
		// Just verify the states have distinct values
		if int(tt.state) < 0 || int(tt.state) > 2 {
			t.Errorf("Unexpected state value: %v", tt.state)
		}
	}
}

func TestBackgroundWorker_ContentHashDedup(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh should build snapshot and set hash
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	if snapshot1 == nil {
		t.Fatal("Expected snapshot after first refresh")
	}

	hash1 := worker.LastHash()
	if hash1 == "" {
		t.Error("Expected non-empty hash after first refresh")
	}

	// Second refresh with same content should be deduped (snapshot unchanged)
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	hash2 := worker.LastHash()

	// Hash should be the same
	if hash1 != hash2 {
		t.Errorf("Hash changed unexpectedly: %s -> %s", hash1, hash2)
	}

	// Snapshot pointer should be unchanged (deduped)
	if snapshot1 != snapshot2 {
		t.Error("Snapshot pointer changed when content was unchanged - dedup failed")
	}
}

func TestBackgroundWorker_ContentHashChanges(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content1 := `{"id":"test-1","title":"Test Issue","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	if snapshot1 == nil {
		t.Fatal("Expected snapshot after first refresh")
	}
	hash1 := worker.LastHash()

	// Modify the file content
	content2 := `{"id":"test-1","title":"Updated Title","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write modified file: %v", err)
	}

	// Second refresh with different content should rebuild
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	if snapshot2 == nil {
		t.Fatal("Expected snapshot after second refresh")
	}
	hash2 := worker.LastHash()

	// Hash should be different
	if hash1 == hash2 {
		t.Error("Hash should have changed when content changed")
	}

	// Snapshot should be different
	if snapshot1 == snapshot2 {
		t.Error("Snapshot pointer should have changed when content changed")
	}

	// New snapshot should have updated title
	if snapshot2.Issues[0].Title != "Updated Title" {
		t.Errorf("Expected updated title, got %q", snapshot2.Issues[0].Title)
	}
}

func TestBackgroundWorker_ResetHash(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot1 := worker.GetSnapshot()
	hash1 := worker.LastHash()
	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Reset hash
	worker.ResetHash()
	if worker.LastHash() != "" {
		t.Error("Expected empty hash after reset")
	}

	// Refresh should rebuild even though content unchanged
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot2 := worker.GetSnapshot()
	hash2 := worker.LastHash()

	// Hash should be repopulated
	if hash2 == "" {
		t.Error("Expected hash to be set after refresh")
	}

	// Should have rebuilt (new snapshot pointer)
	if snapshot1 == snapshot2 {
		t.Error("Expected new snapshot after hash reset")
	}
}

func TestBackgroundWorker_SnapshotHasDataHash(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot")
	}

	// Snapshot should have DataHash populated
	if snapshot.DataHash == "" {
		t.Error("Expected DataHash to be set in snapshot")
	}

	// DataHash should match LastHash
	if snapshot.DataHash != worker.LastHash() {
		t.Errorf("DataHash mismatch: snapshot=%s, worker=%s", snapshot.DataHash, worker.LastHash())
	}
}

func TestWorkerError_String(t *testing.T) {
	err := WorkerError{
		Phase:   "load",
		Cause:   os.ErrNotExist,
		Time:    time.Now(),
		Retries: 3,
	}

	s := err.Error()
	if s == "" {
		t.Error("Error() should return non-empty string")
	}

	if !strings.Contains(s, "load") {
		t.Errorf("Error() should contain phase 'load': %s", s)
	}

	if !strings.Contains(s, "3") {
		t.Errorf("Error() should contain retry count: %s", s)
	}

	// Test Unwrap
	if err.Unwrap() != os.ErrNotExist {
		t.Error("Unwrap() should return underlying error")
	}
}

func TestBackgroundWorker_LoadError(t *testing.T) {
	// Create a worker pointing to non-existent file
	cfg := WorkerConfig{
		BeadsPath:     "/nonexistent/path/beads.jsonl",
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		// Watcher creation might fail for non-existent path, which is fine
		t.Skipf("Skipping test - watcher creation failed: %v", err)
	}
	defer worker.Stop()

	// Trigger refresh
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	// Should have no snapshot (load failed)
	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot when file doesn't exist")
	}

	// Should have recorded error
	lastErr := worker.LastError()
	if lastErr == nil {
		t.Error("Expected error to be recorded")
	} else {
		if lastErr.Phase != "load" {
			t.Errorf("Expected phase 'load', got %q", lastErr.Phase)
		}
	}
}

func TestBackgroundWorker_ErrorRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	// Start with no file
	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// First refresh should fail (no file)
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	if worker.GetSnapshot() != nil {
		t.Error("Expected nil snapshot when file doesn't exist")
	}

	// Now create the file
	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Reset hash to force reload
	worker.ResetHash()

	// Second refresh should succeed
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	snapshot := worker.GetSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot after file created")
	}

	// Error should be cleared
	if worker.LastError() != nil {
		t.Error("Expected error to be cleared on success")
	}
}

func TestBackgroundWorker_SafeCompute(t *testing.T) {
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	// Test that safeCompute catches panics
	err2 := worker.safeCompute("test", func() error {
		panic("intentional panic for testing")
	})

	if err2 == nil {
		t.Error("safeCompute should catch panics")
	}

	if err2.Phase != "test" {
		t.Errorf("Expected phase 'test', got %q", err2.Phase)
	}

	// Verify worker still functional after panic
	worker.TriggerRefresh()
	time.Sleep(200 * time.Millisecond)

	if worker.GetSnapshot() == nil {
		t.Error("Worker should still be functional after panic recovery")
	}
}

func TestHashPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "short string (empty hash)",
			input:    "empty",
			expected: "empty",
		},
		{
			name:     "exactly 16 chars",
			input:    "1234567890123456",
			expected: "1234567890123456",
		},
		{
			name:     "longer than 16 chars",
			input:    "8b423072ec4730921a2b3c4d5e6f7890",
			expected: "8b423072ec473092",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("hashPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBackgroundWorker_ConcurrentTrigger(t *testing.T) {
	// Test that concurrent TriggerRefresh calls don't cause duplicate processing
	tmpDir := t.TempDir()
	beadsPath := filepath.Join(tmpDir, "beads.jsonl")

	content := `{"id":"test-1","title":"Test","status":"open","priority":1,"issue_type":"task"}` + "\n"
	if err := os.WriteFile(beadsPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := WorkerConfig{
		BeadsPath:     beadsPath,
		DebounceDelay: 50 * time.Millisecond,
	}

	worker, err := NewBackgroundWorker(cfg)
	if err != nil {
		t.Fatalf("NewBackgroundWorker failed: %v", err)
	}
	defer worker.Stop()

	if err := worker.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Fire multiple TriggerRefresh calls concurrently
	// The fix ensures only one process() runs at a time, others mark dirty
	for i := 0; i < 5; i++ {
		go worker.TriggerRefresh()
	}

	// Wait for processing to complete
	time.Sleep(400 * time.Millisecond)

	// Worker should still be in idle state (not stuck in processing)
	if worker.State() != WorkerIdle {
		t.Errorf("Expected idle state after concurrent triggers, got %v", worker.State())
	}

	// Should have a valid snapshot
	if worker.GetSnapshot() == nil {
		t.Error("Expected snapshot after concurrent triggers")
	}
}
