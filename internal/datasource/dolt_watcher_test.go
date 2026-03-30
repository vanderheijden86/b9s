package datasource

import (
	"testing"
	"time"
)

// TestDoltWatcher_NewFailsWithBadAddress verifies that NewDoltWatcher returns a
// non-nil error when the target address has no Dolt server listening. This test
// does NOT require B9S_TEST_DOLT_DSN to be set.
func TestDoltWatcher_NewFailsWithBadAddress(t *testing.T) {
	source := DataSource{
		Type: SourceTypeDolt,
		Path: "127.0.0.1:19999",
	}

	_, err := NewDoltWatcher(source, 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected error for unreachable address, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

// TestDoltWatcher_NewAndStop verifies that a DoltWatcher can be created against
// a live Dolt server, that Changed() returns a non-nil channel, and that Stop
// does not panic.
func TestDoltWatcher_NewAndStop(t *testing.T) {
	skipIfNoDolt(t)

	w, err := NewDoltWatcher(doltSource(), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}

	if w.Changed() == nil {
		t.Error("Changed() returned nil channel")
	}

	if !w.IsPolling() {
		t.Error("IsPolling() should always return true for DoltWatcher")
	}

	// Stop must not panic or block.
	w.Stop()
}

// TestDoltWatcher_StartIsIdempotent verifies that calling Start twice does not
// return an error or cause a panic.
func TestDoltWatcher_StartIsIdempotent(t *testing.T) {
	skipIfNoDolt(t)

	w, err := NewDoltWatcher(doltSource(), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer w.Stop()

	if err := w.Start(); err != nil {
		t.Fatalf("first Start() failed: %v", err)
	}
	// Second call must be a no-op.
	if err := w.Start(); err != nil {
		t.Fatalf("second Start() failed: %v", err)
	}
}

// TestDoltWatcher_DetectsNoChangeOnPoll starts a watcher against a quiescent
// Dolt server and waits for two poll cycles. It asserts that no spurious change
// notifications are delivered when the database has not changed.
func TestDoltWatcher_DetectsNoChangeOnPoll(t *testing.T) {
	skipIfNoDolt(t)

	const interval = 200 * time.Millisecond

	w, err := NewDoltWatcher(doltSource(), interval)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer w.Stop()

	if err := w.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Wait for two full poll cycles to elapse.
	time.Sleep(interval * 3)

	// The channel should be empty — no database changes occurred.
	select {
	case <-w.Changed():
		t.Error("received unexpected change notification on quiescent database")
	default:
		// Expected: no notification.
	}
}

// TestDoltWatcher_SetOnChangeNotCalledOnQuiescentDB verifies that the onChange
// callback is not invoked when the database has not changed.
func TestDoltWatcher_SetOnChangeNotCalledOnQuiescentDB(t *testing.T) {
	skipIfNoDolt(t)

	const interval = 200 * time.Millisecond

	w, err := NewDoltWatcher(doltSource(), interval)
	if err != nil {
		t.Fatalf("NewDoltWatcher failed: %v", err)
	}
	defer w.Stop()

	called := false
	w.SetOnChange(func() { called = true })

	if err := w.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	time.Sleep(interval * 3)

	if called {
		t.Error("onChange callback was invoked on quiescent database")
	}
}
