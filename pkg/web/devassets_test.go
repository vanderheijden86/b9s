package web

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAssetsChangedReachesOpenStreamsAndServesNewFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "app.css"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("body{color:red}")
	base := newTestServer(t, nil, nil)
	srv, err := NewServer(Options{Store: base.store, Assets: os.DirFS(dir), Heartbeat: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	get := func() string {
		t.Helper()
		resp, err := ts.Client().Get(ts.URL + "/app.css")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	if got := get(); got != "body{color:red}" {
		t.Fatalf("first css %q", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	events := make(chan string)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if l := sc.Text(); strings.HasPrefix(l, "event:") {
				events <- l
			}
		}
		close(events)
	}()
	next := func() string {
		select {
		case l := <-events:
			return l
		case <-ctx.Done():
			t.Fatal("no event within the deadline")
			return ""
		}
	}
	if l := next(); l != "event: hello" {
		t.Fatalf("first event %q", l)
	}

	write("body{color:blue}")
	srv.AssetsChanged()
	if l := next(); l != "event: assets" {
		t.Fatalf("after AssetsChanged: %q", l)
	}
	if got := get(); got != "body{color:blue}" {
		t.Fatalf("css after change %q, want the new file", got)
	}
}

func TestWatchAssetsCallsBackOnceForABurstOfWrites(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	calls := make(chan struct{}, 10)
	ready := make(chan struct{})
	go func() {
		_ = WatchAssets(ctx, dir, 100*time.Millisecond, func() { calls <- struct{}{} }, ready)
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("watcher never started")
	}
	for _, name := range []string{"app.js", "app.css", "index.html"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-calls:
	case <-ctx.Done():
		t.Fatal("no callback after writes")
	}
	select {
	case <-calls:
		t.Fatal("a burst of writes called back twice")
	case <-time.After(400 * time.Millisecond):
	}
}

func TestWatchAssetsFailsOnAMissingDir(t *testing.T) {
	err := WatchAssets(context.Background(), filepath.Join(t.TempDir(), "nope"), time.Millisecond, func() {}, nil)
	if err == nil {
		t.Fatal("watching a missing dir succeeded")
	}
}
