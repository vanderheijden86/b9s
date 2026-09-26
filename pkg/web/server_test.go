package web

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// fixtureIssues covers every derivation rule: a parent-child link, an open
// blocker, a closed blocker and a status-blocked issue.
const fixtureIssues = `{"id":"t-1","title":"Epic one","status":"open","priority":1,"issue_type":"epic","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z"}
{"id":"t-1.1","title":"Child waiting on blocker","status":"open","priority":2,"issue_type":"task","labels":["web"],"created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z","dependencies":[{"issue_id":"t-1.1","depends_on_id":"t-1","type":"parent-child"},{"issue_id":"t-1.1","depends_on_id":"t-2","type":"blocks"}]}
{"id":"t-2","title":"Open blocker","status":"in_progress","priority":0,"issue_type":"bug","assignee":"alice","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z"}
{"id":"t-3","title":"Waits on closed work","status":"open","priority":3,"issue_type":"task","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z","dependencies":[{"issue_id":"t-3","depends_on_id":"t-4","type":"blocks"}]}
{"id":"t-4","title":"Done already","status":"closed","priority":2,"issue_type":"task","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z","closed_at":"2026-09-02T10:00:00Z"}
{"id":"t-5","title":"Blocked by status","status":"blocked","priority":2,"issue_type":"task","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z","comments":[{"id":1,"issue_id":"t-5","author":"bob","text":"stuck on review","created_at":"2026-09-03T10:00:00Z"}]}
`

var testAssets = fstest.MapFS{
	"index.html": {Data: []byte("<!doctype html><title>b9s</title>")},
	"app.js":     {Data: []byte("console.log('b9s')")},
}

// newFixtureProject writes a JSONL checkout and returns its directory.
func newFixtureProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	beads := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(beads, "issues.jsonl"), []byte(fixtureIssues), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

type testServer struct {
	*httptest.Server
	store *Store
	auth  *Auth
	dir   string
}

func newTestServer(t *testing.T, auth *Auth, writer *ui.IssueWriter) *testServer {
	t.Helper()
	dir := newFixtureProject(t)
	store := NewStore(50 * time.Millisecond)
	t.Cleanup(store.Close)
	if failure := store.Open(datasource.OpenTarget{Name: "fixture", Dir: dir}, dir); failure != nil {
		t.Fatalf("open fixture: %s", failure.Message())
	}
	if writer == nil {
		writer = ui.NewIssueWriter()
	}
	srv, err := NewServer(Options{Store: store, Writer: writer, Auth: auth, Assets: testAssets, Heartbeat: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return &testServer{Server: ts, store: store, auth: auth, dir: dir}
}

func testAuth(t *testing.T) *Auth {
	t.Helper()
	a, err := NewAuth([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// pairedClient pairs a cookie-jar client the way a phone does: it follows
// the printed link once.
func (ts *testServer) pairedClient(t *testing.T) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	resp, err := c.Get(ts.URL + "/pair?t=" + url.QueryEscape(ts.auth.PairToken()))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pairing ended with %d", resp.StatusCode)
	}
	return c
}

func getJSON(t *testing.T, c *http.Client, u string, v any) int {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if v != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

func byID(issues []Issue) map[string]Issue {
	m := make(map[string]Issue, len(issues))
	for _, is := range issues {
		m[is.ID] = is
	}
	return m
}

func TestSnapshotDerivesBlockedReadyAndParentLikeTheTUI(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	var snap Snapshot
	if code := getJSON(t, ts.Client(), ts.URL+"/api/snapshot", &snap); code != http.StatusOK {
		t.Fatalf("snapshot: %d", code)
	}
	if len(snap.Issues) != 6 || snap.Version == 0 {
		t.Fatalf("got %d issues at version %d", len(snap.Issues), snap.Version)
	}
	got := byID(snap.Issues)
	cases := []struct {
		id                         string
		blocked, ready, closedLike bool
	}{
		{"t-1", false, true, false},
		{"t-1.1", true, false, false}, // open blocker t-2
		{"t-2", false, true, false},
		{"t-3", false, true, false}, // its blocker is closed
		{"t-4", false, false, true},
		{"t-5", true, false, false}, // status blocked
	}
	for _, c := range cases {
		is := got[c.id]
		if is.Blocked != c.blocked || is.Ready != c.ready || is.ClosedLike != c.closedLike {
			t.Errorf("%s: blocked=%v ready=%v closed_like=%v, want %v %v %v", c.id, is.Blocked, is.Ready, is.ClosedLike, c.blocked, c.ready, c.closedLike)
		}
	}
	if got["t-1.1"].Parent != "t-1" || len(got["t-1.1"].BlockedBy) != 1 || got["t-1.1"].BlockedBy[0] != "t-2" {
		t.Errorf("t-1.1 links: parent %q blocked_by %v", got["t-1.1"].Parent, got["t-1.1"].BlockedBy)
	}
	if got["t-5"].CommentCount != 1 {
		t.Errorf("t-5 comment_count %d, want 1", got["t-5"].CommentCount)
	}
	if got["t-4"].ClosedAt != "2026-09-02T10:00:00Z" {
		t.Errorf("closed_at %q", got["t-4"].ClosedAt)
	}
	if snap.Project.ReadOnly || snap.Health.Kind != string(datasource.SourceTypeJSONLLocal) || !snap.Health.OK {
		t.Errorf("project %+v health %+v", snap.Project, snap.Health)
	}
}

func TestSnapshotListsNeverNull(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	resp, err := ts.Client().Get(ts.URL + "/api/snapshot")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "null") {
		t.Fatalf("snapshot carries a null, which the TypeScript types do not allow: %.300s", body)
	}
}

func TestQueryUsesTheTUIQueryLanguage(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	cases := map[string][]string{
		"status:blocked": {"t-5"},
		"assignee:alice": {"t-2"},
		"label:web":      {"t-1.1"},
		"blocker":        {"t-1.1", "t-2"},
		"type:epic":      {"t-1"},
		"":               {"t-1", "t-1.1", "t-2", "t-3", "t-4", "t-5"},
	}
	for q, want := range cases {
		var res QueryResult
		getJSON(t, ts.Client(), ts.URL+"/api/query?q="+url.QueryEscape(q), &res)
		if strings.Join(res.IDs, ",") != strings.Join(want, ",") {
			t.Errorf("query %q: got %v, want %v", q, res.IDs, want)
		}
	}
}

func TestUnpairedRequestsAreRefused(t *testing.T) {
	ts := newTestServer(t, testAuth(t), nil)
	for _, path := range []string{"/api/snapshot", "/api/query?q=x", "/api/events", "/api/session", "/api/projects"} {
		if code := getJSON(t, ts.Client(), ts.URL+path, nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s unpaired: %d, want 401", path, code)
		}
	}
	resp, err := ts.Client().Post(ts.URL+"/api/write", "application/json", strings.NewReader(`{"op":"close","ids":["t-1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unpaired write: %d", resp.StatusCode)
	}
}

func TestPairingSetsAStrictHttpOnlyCookie(t *testing.T) {
	ts := newTestServer(t, testAuth(t), nil)
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	resp, err := noRedirect.Get(ts.URL + "/pair?t=wrong")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden || len(resp.Cookies()) != 0 {
		t.Fatalf("wrong token: %d with %d cookies", resp.StatusCode, len(resp.Cookies()))
	}

	resp, err = noRedirect.Get(ts.URL + "/pair?t=" + ts.auth.PairToken())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "./" {
		t.Fatalf("pair: %d to %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Error("the pairing response must not leak its URL in a Referer")
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Value == ts.auth.PairToken() {
		t.Fatalf("cookie: %+v", cookies)
	}

	c := ts.pairedClient(t)
	var session Session
	if code := getJSON(t, c, ts.URL+"/api/session", &session); code != http.StatusOK || session.CSRF != ts.auth.CSRF() {
		t.Fatalf("session: %d %+v", code, session)
	}
}

func TestWriteWithoutCSRFHeaderIsRefused(t *testing.T) {
	ts := newTestServer(t, testAuth(t), nil)
	c := ts.pairedClient(t)
	resp, err := c.Post(ts.URL+"/api/write", "application/json", strings.NewReader(`{"op":"close","ids":["t-1"]}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("write without CSRF: %d, want 403", resp.StatusCode)
	}
}

func TestPairTokenSurvivesRestartAndRenewUnpairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "web", "secret")
	first, err := LoadOrCreateSecret(path, false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("secret file mode %v (%v)", info.Mode(), err)
	}
	again, _ := LoadOrCreateSecret(path, false)
	renewed, _ := LoadOrCreateSecret(path, true)
	a1, _ := NewAuth(first)
	a2, _ := NewAuth(again)
	a3, _ := NewAuth(renewed)
	if a1.PairToken() != a2.PairToken() {
		t.Error("a restart must keep the pairing token")
	}
	if a1.PairToken() == a3.PairToken() || a1.sessionValue() == a3.sessionValue() {
		t.Error("renewing the secret must unpair every device")
	}
}

// A server without a pairing token must never be reachable from another
// machine: tailnet and LAN peers could read and write every issue.
func TestListenWithoutAuthOnlyOnLoopback(t *testing.T) {
	auth := testAuth(t)
	cases := []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:7979", true},
		{"localhost:7979", true},
		{"[::1]:7979", true},
		{":7979", false},
		{"0.0.0.0:7979", false},
		{"100.64.1.2:7979", false},
		{"192.168.1.10:7979", false},
	}
	for _, c := range cases {
		if err := CheckListen(c.addr, nil); (err == nil) != c.ok {
			t.Errorf("CheckListen(%q, nil) = %v, want ok=%v", c.addr, err, c.ok)
		}
		if err := CheckListen(c.addr, auth); err != nil {
			t.Errorf("CheckListen(%q, auth) = %v", c.addr, err)
		}
	}
}

func TestEventsStreamHelloThenChanges(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	next := func(prefix string) string {
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("stream ended before %q", prefix)
				}
				if strings.HasPrefix(l, prefix) {
					return l
				}
			case <-ctx.Done():
				t.Fatalf("no %q line within the deadline", prefix)
			}
		}
	}
	if l := next("event:"); l != "event: hello" {
		t.Fatalf("first event %q", l)
	}
	next(": ping")
	ts.store.Reload()
	if l := next("event:"); l != "event: changed" {
		t.Fatalf("after reload: %q", l)
	}
}

func TestFileChangeReachesTheSnapshot(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	before := ts.store.Version()
	extra := `{"id":"t-6","title":"Added outside","status":"open","priority":2,"issue_type":"task","created_at":"2026-09-01T10:00:00Z","updated_at":"2026-09-01T10:00:00Z"}` + "\n"
	f, err := os.OpenFile(filepath.Join(ts.dir, ".beads", "issues.jsonl"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(extra)
	f.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ts.store.Version() > before {
			var snap Snapshot
			getJSON(t, ts.Client(), ts.URL+"/api/snapshot", &snap)
			if _, ok := byID(snap.Issues)["t-6"]; ok {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the watcher did not reload the changed file within 5s")
}

func TestAssetsRevalidateWithETag(t *testing.T) {
	ts := newTestServer(t, nil, nil)
	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if !strings.Contains(string(body), "<title>b9s</title>") || etag == "" || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("index: %q etag %q cache %q", body, etag, resp.Header.Get("Cache-Control"))
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	req.Header.Set("If-None-Match", etag)
	resp, err = ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Fatalf("revalidation: %d", resp.StatusCode)
	}
	if code := getJSON(t, ts.Client(), ts.URL+"/api/nothing", nil); code != http.StatusNotFound {
		t.Fatalf("unknown API path: %d", code)
	}
}

func TestEmbeddedBundleHasAnIndex(t *testing.T) {
	assets, err := EmbeddedAssets()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assets.Open("index.html"); err != nil {
		t.Fatalf("embedded bundle: %v", err)
	}
}

func TestSnapshotLeavesOutLongTextThatIssueReturns(t *testing.T) {
	ts := newTestServer(t, testAuth(t), nil)
	c := ts.pairedClient(t)
	var snap Snapshot
	getJSON(t, c, ts.URL+"/api/snapshot", &snap)
	lean := byID(snap.Issues)["t-5"]
	if len(lean.Comments) != 0 || lean.CommentCount != 1 {
		t.Fatalf("snapshot t-5: comments %+v count %d, want none and 1", lean.Comments, lean.CommentCount)
	}

	var full Issue
	if code := getJSON(t, c, ts.URL+"/api/issue?id=t-5", &full); code != http.StatusOK {
		t.Fatalf("issue: %d", code)
	}
	if len(full.Comments) != 1 || full.Comments[0].Text != "stuck on review" || full.CommentCount != 1 || !full.Blocked {
		t.Fatalf("full t-5: %+v", full)
	}
	if code := getJSON(t, c, ts.URL+"/api/issue?id=t-404", nil); code != http.StatusNotFound {
		t.Fatalf("missing issue: %d, want 404", code)
	}
	if code := getJSON(t, &http.Client{Timeout: 5 * time.Second}, ts.URL+"/api/issue?id=t-5", nil); code != http.StatusUnauthorized && code != http.StatusForbidden {
		t.Fatalf("unpaired issue read: %d", code)
	}
}
