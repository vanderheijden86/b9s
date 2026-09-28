package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
)

func newPublicServer(t *testing.T, opts Options) *testServer {
	t.Helper()
	dir := newFixtureProject(t)
	store := NewStore(50 * time.Millisecond)
	t.Cleanup(store.Close)
	if failure := store.Open(datasource.OpenTarget{Name: "fixture", Dir: dir}, dir); failure != nil {
		t.Fatalf("open fixture: %s", failure.Message())
	}
	opts.Store, opts.Public, opts.Assets, opts.Heartbeat = store, true, testAssets, 20*time.Millisecond
	opts.Projects = func() []config.Project {
		t.Error("a public server must not list other projects")
		return nil
	}
	srv, err := NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return &testServer{Server: ts, store: store, dir: dir}
}

func TestPublicServerRefusesPairing(t *testing.T) {
	store := NewStore(time.Second)
	t.Cleanup(store.Close)
	if _, err := NewServer(Options{Store: store, Public: true, Auth: testAuth(t)}); err == nil {
		t.Fatal("a public server with an Auth was accepted")
	}
}

func TestPublicServerServesWithoutPairingAndCarriesTheBanner(t *testing.T) {
	ts := newPublicServer(t, Options{Banner: "Public demo", BannerLink: "https://example.com/b9s"})
	var s Session
	if code := getJSON(t, ts.Client(), ts.URL+"/api/session", &s); code != http.StatusOK {
		t.Fatalf("session: %d", code)
	}
	if !s.Public || s.Banner != "Public demo" || s.BannerLink != "https://example.com/b9s" {
		t.Fatalf("session %+v", s)
	}
	var snap Snapshot
	if code := getJSON(t, ts.Client(), ts.URL+"/api/snapshot", &snap); code != http.StatusOK || len(snap.Issues) == 0 {
		t.Fatalf("snapshot: %d with %d issues", code, len(snap.Issues))
	}
}

func TestPublicServerShowsOnlyItsOwnProject(t *testing.T) {
	ts := newPublicServer(t, Options{})
	var list ProjectList
	if code := getJSON(t, ts.Client(), ts.URL+"/api/projects", &list); code != http.StatusOK {
		t.Fatalf("projects: %d", code)
	}
	if len(list.Projects) != 1 || !list.Projects[0].Active || list.Projects[0].Name != "fixture" {
		t.Fatalf("projects %+v", list.Projects)
	}
	for _, key := range []string{AllProjectsKey, list.Projects[0].Key, "/etc"} {
		resp, err := ts.Client().Post(ts.URL+"/api/projects/open", "application/json", strings.NewReader(`{"key":"`+key+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("open %q: %d, want 403", key, resp.StatusCode)
		}
	}
}

func TestPublicServerSharesOneWriteBudget(t *testing.T) {
	ts := newPublicServer(t, Options{})
	post := func() int {
		resp, err := ts.Client().Post(ts.URL+"/api/reload", "application/json", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := 0; i < publicWritesPerMinute; i++ {
		if code := post(); code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, code)
		}
	}
	if code := post(); code != http.StatusTooManyRequests {
		t.Fatalf("request past the budget: %d, want 429", code)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	now := time.Unix(0, 0)
	l := newRateLimiter(2, time.Minute)
	l.now, l.last = func() time.Time { return now }, now
	if !l.allow() || !l.allow() || l.allow() {
		t.Fatal("a full bucket of 2 did not allow exactly 2")
	}
	now = now.Add(30 * time.Second)
	if !l.allow() || l.allow() {
		t.Fatal("half a period did not refill exactly one")
	}
}

func TestEventStreamsAreCapped(t *testing.T) {
	ts := newPublicServer(t, Options{MaxStreams: 1})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	open := func() *http.Response {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	first := open()
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first stream: %d", first.StatusCode)
	}
	second := open()
	second.Body.Close()
	if second.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("second stream: %d, want 503", second.StatusCode)
	}
}
