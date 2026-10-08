package web

import (
	"context"
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
)

// AssetsChanged tells every open browser that the SPA files changed, so it
// reloads. Only b9s web --dev-assets calls it: the embedded bundle cannot
// change while the server runs.
func (s *Server) AssetsChanged() {
	s.etagMu.Lock()
	clear(s.etags)
	s.etagMu.Unlock()

	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	for ch := range s.assetSubs {
		// A stream that has not taken the last signal yet reloads anyway.
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Server) subscribeAssets() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.streamMu.Lock()
	s.assetSubs[ch] = struct{}{}
	s.streamMu.Unlock()
	return ch, func() {
		s.streamMu.Lock()
		delete(s.assetSubs, ch)
		s.streamMu.Unlock()
	}
}

// WatchAssets calls changed once per burst of writes in dir, after debounce
// passes with no further write. esbuild writes several files per rebuild, and
// a reload between two of them would load a mismatched pair. ready, when not
// nil, is closed once the watch is in place. It returns when ctx ends.
func WatchAssets(ctx context.Context, dir string, debounce time.Duration, changed func(), ready chan<- struct{}) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		return fmt.Errorf("watch %s: %w", dir, err)
	}
	if ready != nil {
		close(ready)
	}
	timer := time.NewTimer(debounce)
	timer.Stop()
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				timer.Reset(debounce)
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			return err
		case <-timer.C:
			changed()
		}
	}
}
