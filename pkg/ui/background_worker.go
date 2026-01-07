// Package ui provides the terminal user interface for beads_viewer.
// This file implements the BackgroundWorker for off-thread data processing.
package ui

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/loader"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
	"github.com/Dicklesworthstone/beads_viewer/pkg/watcher"
)

// WorkerState represents the current state of the background worker.
type WorkerState int

const (
	// WorkerIdle means the worker is waiting for file changes.
	WorkerIdle WorkerState = iota
	// WorkerProcessing means the worker is building a new snapshot.
	WorkerProcessing
	// WorkerStopped means the worker has been stopped.
	WorkerStopped
)

// WorkerError wraps errors with phase and retry context.
type WorkerError struct {
	Phase   string    // "load", "parse", "analyze_phase1", "analyze_phase2"
	Cause   error     // The underlying error
	Time    time.Time // When the error occurred
	Retries int       // Number of retry attempts
}

func (e WorkerError) Error() string {
	return fmt.Sprintf("%s failed: %v (retries: %d)", e.Phase, e.Cause, e.Retries)
}

func (e WorkerError) Unwrap() error {
	return e.Cause
}

// BackgroundWorker manages background processing of beads data.
// It owns the file watcher, implements coalescing, and builds snapshots
// off the UI thread.
type BackgroundWorker struct {
	// Configuration
	beadsPath     string
	debounceDelay time.Duration

	// State
	mu       sync.RWMutex
	state    WorkerState
	dirty    bool // True if a change came in while processing
	snapshot *DataSnapshot
	started  bool   // True if Start() has been called
	lastHash string // Content hash of last processed snapshot (for dedup)

	// Error tracking
	lastError *WorkerError // Most recent error (nil if last operation succeeded)
	errorCount int         // Consecutive error count for backoff

	// Components
	watcher *watcher.Watcher
	program *tea.Program

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

// WorkerConfig configures the BackgroundWorker.
type WorkerConfig struct {
	BeadsPath     string
	DebounceDelay time.Duration
	Program       *tea.Program
}

// NewBackgroundWorker creates a new background worker.
func NewBackgroundWorker(cfg WorkerConfig) (*BackgroundWorker, error) {
	ctx, cancel := context.WithCancel(context.Background())

	if cfg.DebounceDelay == 0 {
		cfg.DebounceDelay = 200 * time.Millisecond
	}

	w := &BackgroundWorker{
		beadsPath:     cfg.BeadsPath,
		debounceDelay: cfg.DebounceDelay,
		program:       cfg.Program,
		state:         WorkerIdle,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
	}

	// Initialize file watcher
	if cfg.BeadsPath != "" {
		fw, err := watcher.NewWatcher(cfg.BeadsPath,
			watcher.WithDebounceDuration(cfg.DebounceDelay),
		)
		if err != nil {
			cancel()
			return nil, err
		}
		w.watcher = fw
	}

	return w, nil
}

// Start begins watching for file changes and processing in the background.
// Start is idempotent - calling it multiple times has no effect.
func (w *BackgroundWorker) Start() error {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return nil // Already started
	}
	w.started = true
	w.mu.Unlock()

	if w.watcher != nil {
		if err := w.watcher.Start(); err != nil {
			return err
		}

		// Start the processing loop
		go w.processLoop()
	} else {
		// No watcher - close done channel immediately so Stop() doesn't block
		close(w.done)
	}

	return nil
}

// Stop halts the background worker and cleans up resources.
// Stop is idempotent - calling it multiple times has no effect.
func (w *BackgroundWorker) Stop() {
	w.mu.Lock()
	if w.state == WorkerStopped {
		w.mu.Unlock()
		return
	}
	w.state = WorkerStopped
	wasStarted := w.started
	w.mu.Unlock()

	w.cancel()

	if w.watcher != nil {
		w.watcher.Stop()
	}

	// Only wait for done if Start() was called
	if wasStarted {
		select {
		case <-w.done:
		case <-time.After(2 * time.Second):
			// Timeout waiting for graceful shutdown
		}
	}
}

// TriggerRefresh manually triggers a refresh of the data.
// Has no effect if the worker is stopped or already processing.
func (w *BackgroundWorker) TriggerRefresh() {
	w.mu.Lock()
	if w.state == WorkerStopped {
		w.mu.Unlock()
		return
	}
	if w.state == WorkerProcessing {
		w.dirty = true
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	// Trigger processing
	go w.process()
}

// GetSnapshot returns the current snapshot (may be nil).
func (w *BackgroundWorker) GetSnapshot() *DataSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.snapshot
}

// State returns the current worker state.
func (w *BackgroundWorker) State() WorkerState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// processLoop watches for file changes and triggers processing.
func (w *BackgroundWorker) processLoop() {
	defer close(w.done)

	if w.watcher == nil {
		return
	}

	for {
		select {
		case <-w.ctx.Done():
			return

		case <-w.watcher.Changed():
			w.process()
		}
	}
}

// process builds a new snapshot from the current file.
func (w *BackgroundWorker) process() {
	w.mu.Lock()
	if w.state != WorkerIdle {
		// Already stopped or processing
		if w.state == WorkerProcessing {
			// Mark dirty so current processor will re-run when done
			w.dirty = true
		}
		w.mu.Unlock()
		return
	}
	w.state = WorkerProcessing
	w.dirty = false
	w.mu.Unlock()

	// Load and build snapshot
	// Returns nil if content unchanged (dedup) or on error
	snapshot := w.buildSnapshot()

	w.mu.Lock()
	// Check if stopped while we were processing - don't overwrite stopped state
	if w.state == WorkerStopped {
		w.mu.Unlock()
		return
	}
	// Only update snapshot if we got a new one (nil means deduped or error)
	if snapshot != nil {
		w.snapshot = snapshot
	}
	wasDirty := w.dirty
	w.state = WorkerIdle
	w.mu.Unlock()

	// Notify UI only if we have a new snapshot
	if w.program != nil && snapshot != nil {
		w.program.Send(SnapshotReadyMsg{Snapshot: snapshot})
	}

	// If dirty, process again immediately
	if wasDirty {
		go w.process()
	}
}

// safeCompute executes fn and recovers from any panics.
// Returns a WorkerError if fn panics, nil otherwise.
func (w *BackgroundWorker) safeCompute(phase string, fn func() error) *WorkerError {
	var result *WorkerError
	func() {
		defer func() {
			if r := recover(); r != nil {
				result = &WorkerError{
					Phase: phase,
					Cause: fmt.Errorf("panic: %v\n%s", r, debug.Stack()),
					Time:  time.Now(),
				}
			}
		}()
		if err := fn(); err != nil {
			result = &WorkerError{
				Phase: phase,
				Cause: err,
				Time:  time.Now(),
			}
		}
	}()
	return result
}

// recordError tracks an error and updates error state.
func (w *BackgroundWorker) recordError(err *WorkerError) {
	w.mu.Lock()
	w.lastError = err
	if err != nil {
		w.errorCount++
		err.Retries = w.errorCount
	} else {
		w.errorCount = 0
	}
	w.mu.Unlock()
}

// LastError returns the most recent error (nil if last operation succeeded).
func (w *BackgroundWorker) LastError() *WorkerError {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastError
}

// buildSnapshot loads data and constructs a new DataSnapshot.
// This is called from the worker goroutine (NOT the UI thread).
// Returns nil if beadsPath is empty, loading fails, or content is unchanged.
func (w *BackgroundWorker) buildSnapshot() *DataSnapshot {
	if w.beadsPath == "" {
		return nil
	}

	start := time.Now()

	// Load issues from file with panic recovery
	var issues []model.Issue
	loadErr := w.safeCompute("load", func() error {
		var err error
		issues, err = loader.LoadIssuesFromFile(w.beadsPath)
		return err
	})

	if loadErr != nil {
		log.Printf("buildSnapshot: error loading %s: %v", w.beadsPath, loadErr)
		w.recordError(loadErr)

		// Send error to UI
		if w.program != nil {
			w.program.Send(SnapshotErrorMsg{
				Err:         loadErr,
				Recoverable: true, // File errors are usually recoverable
			})
		}
		return nil
	}

	loadDuration := time.Since(start)

	// Compute content hash for dedup
	hash := analysis.ComputeDataHash(issues)

	// Check if content is unchanged (dedup optimization)
	w.mu.RLock()
	lastHash := w.lastHash
	w.mu.RUnlock()

	if hash == lastHash && lastHash != "" {
		log.Printf("buildSnapshot: content unchanged (hash=%s), skipping rebuild", hashPrefix(hash))
		// Clear any previous error on successful dedup
		w.recordError(nil)
		return nil
	}

	// Build snapshot (includes Phase 1 analysis) with panic recovery
	var snapshot *DataSnapshot
	analyzeStart := time.Now()
	analyzeErr := w.safeCompute("analyze_phase1", func() error {
		builder := NewSnapshotBuilder(issues)
		snapshot = builder.Build()
		return nil
	})

	analyzeDuration := time.Since(analyzeStart)

	if analyzeErr != nil {
		log.Printf("buildSnapshot: analysis error: %v", analyzeErr)
		w.recordError(analyzeErr)

		// Send error to UI
		if w.program != nil {
			w.program.Send(SnapshotErrorMsg{
				Err:         analyzeErr,
				Recoverable: true,
			})
		}
		return nil
	}

	// Clear error on success
	w.recordError(nil)

	// Update lastHash for future dedup checks
	w.mu.Lock()
	w.lastHash = hash
	w.mu.Unlock()

	// Store hash in snapshot for external access
	if snapshot != nil {
		snapshot.DataHash = hash
	}

	totalDuration := time.Since(start)
	log.Printf("buildSnapshot: loaded %d issues (load=%v, analyze=%v, total=%v, hash=%s)",
		len(issues), loadDuration, analyzeDuration, totalDuration, hashPrefix(hash))

	return snapshot
}

// SnapshotReadyMsg is sent to the UI when a new snapshot is ready.
type SnapshotReadyMsg struct {
	Snapshot *DataSnapshot
}

// SnapshotErrorMsg is sent to the UI when snapshot building fails.
type SnapshotErrorMsg struct {
	Err         error
	Recoverable bool // True if we expect to recover on next file change
}

// Phase2UpdateMsg is sent when Phase 2 analysis completes.
// This allows the UI to update without waiting for full rebuild.
type Phase2UpdateMsg struct {
	// Phase 2 metrics are embedded in the GraphStats
}

// WatcherChanged returns the watcher's change notification channel.
// This is useful for integration with existing code.
func (w *BackgroundWorker) WatcherChanged() <-chan struct{} {
	if w.watcher == nil {
		return nil
	}
	return w.watcher.Changed()
}

// LastHash returns the content hash from the last successful snapshot build.
// Useful for testing and debugging.
func (w *BackgroundWorker) LastHash() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastHash
}

// hashPrefix returns a safe prefix of the hash for logging.
// Returns up to 16 characters, or the full hash if shorter.
func hashPrefix(hash string) string {
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

// ResetHash clears the stored content hash, forcing the next buildSnapshot
// to process even if content is unchanged. Useful for testing.
func (w *BackgroundWorker) ResetHash() {
	w.mu.Lock()
	w.lastHash = ""
	w.mu.Unlock()
}
