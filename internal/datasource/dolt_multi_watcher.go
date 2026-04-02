package datasource

import (
	"context"
	"sync"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/debug"
)

// MultiDoltWatcher polls HEAD hashes across multiple Dolt databases and fires
// a change notification when any database has a new commit.
type MultiDoltWatcher struct {
	reader       *MultiDoltReader
	pollInterval time.Duration
	lastHashes   map[string]string

	ctx      context.Context
	cancel   context.CancelFunc
	changeCh chan struct{}
	started  bool
	mu       sync.RWMutex
}

// NewMultiDoltWatcher creates a watcher that monitors all databases in the reader.
func NewMultiDoltWatcher(reader *MultiDoltReader, pollInterval time.Duration) *MultiDoltWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &MultiDoltWatcher{
		reader:       reader,
		pollInterval: pollInterval,
		lastHashes:   make(map[string]string),
		changeCh:     make(chan struct{}, 1),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins polling. Captures initial hashes, then polls for changes.
func (w *MultiDoltWatcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return nil
	}

	// Capture initial hashes
	w.lastHashes = w.reader.GetHeadHashes()
	debug.Log("multi-dolt-watcher: initial hashes for %d databases", len(w.lastHashes))

	w.started = true
	go w.poll()
	return nil
}

// WaitForChange blocks until any database changes or the context is cancelled.
func (w *MultiDoltWatcher) WaitForChange() bool {
	select {
	case <-w.changeCh:
		return true
	case <-w.ctx.Done():
		return false
	}
}

// Stop stops the watcher.
func (w *MultiDoltWatcher) Stop() {
	w.cancel()
}

func (w *MultiDoltWatcher) poll() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			hashes := w.reader.GetHeadHashes()
			changed := false
			for db, hash := range hashes {
				if old, ok := w.lastHashes[db]; !ok || old != hash {
					debug.Log("multi-dolt-watcher: %s changed %s -> %s", db, old, hash)
					changed = true
				}
			}
			if changed {
				w.mu.Lock()
				w.lastHashes = hashes
				w.mu.Unlock()
				select {
				case w.changeCh <- struct{}{}:
				default:
				}
			}
		}
	}
}
