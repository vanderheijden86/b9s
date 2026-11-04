package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/ui"
	"github.com/vanderheijden86/b9s/pkg/web"
)

func TestWebDoltDiscoveryAddsRuntimeDatabase(t *testing.T) {
	root := t.TempDir()
	names := []string{"alpha"}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list: func(context.Context, datasource.DataSource) ([]string, error) {
			return names, nil
		},
	}
	discovery.projects(nil)
	names = append(names, "beta")
	got := discovery.projects(nil)
	if len(got) != 2 || got[1].Name != "beta" {
		t.Fatalf("projects after grant = %+v, want alpha and beta", got)
	}
	recent, ok := config.RecentFromCheckout("beta", filepath.Join(root, "beta"))
	if !ok || recent.Database != "beta" || recent.Host != discovery.source.Path {
		t.Fatalf("new checkout = %+v, ok=%v", recent, ok)
	}
}

func TestWebDoltDiscoveryDropsUnreadableDatabase(t *testing.T) {
	root := t.TempDir()
	names := []string{"alpha", "beta"}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list:   func(context.Context, datasource.DataSource) ([]string, error) { return names, nil },
	}
	initial := discovery.projects(nil)
	names = []string{"beta"}
	got := discovery.projects(initial)
	if len(got) != 1 || got[0].Name != "beta" {
		t.Fatalf("projects after loss = %+v, want beta only", got)
	}
	names = nil
	if got := discovery.projects(initial); len(got) != 0 {
		t.Fatalf("projects after all databases disappear = %+v", got)
	}
}

func TestWebDoltDiscoveryRetainsProjectsOnTransientFailure(t *testing.T) {
	root := t.TempDir()
	var failure error
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list: func(ctx context.Context, _ datasource.DataSource) ([]string, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("discovery needs a deadline")
			}
			return []string{"alpha"}, failure
		},
	}
	discovery.projects(nil)
	failure = errors.New("temporary network failure")
	if got := discovery.projects(nil); len(got) != 1 || got[0].Name != "alpha" {
		t.Fatalf("projects during outage = %+v, want last readable list", got)
	}
	failure = nil
	if got := discovery.projects(nil); len(got) != 1 {
		t.Fatalf("projects after recovery = %+v", got)
	}
}

func TestWebDoltDiscoveryPreservesLocalCheckout(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "local")
	if err := os.MkdirAll(filepath.Join(local, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list:   func(context.Context, datasource.DataSource) ([]string, error) { return nil, nil },
	}
	got := discovery.projects([]config.Project{{Name: "local", Path: local}})
	if len(got) != 1 || got[0].Path != local {
		t.Fatalf("local checkout = %+v", got)
	}
}

func TestWebDoltDiscoveryDropsProjectsOnLoginDenial(t *testing.T) {
	var failure error
	discovery := &webDoltDiscovery{
		root:   t.TempDir(),
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list: func(context.Context, datasource.DataSource) ([]string, error) {
			return []string{"alpha"}, failure
		},
	}
	initial := discovery.projects(nil)
	if len(initial) != 1 {
		t.Fatal("initial project missing")
	}
	failure = &mysql.MySQLError{Number: 1045, Message: "Access denied"}
	if got := discovery.projects(initial); len(got) != 0 {
		t.Fatalf("projects after login denied = %+v", got)
	}
}

func TestWebDoltDiscoveryDoesNotOverwriteLocalCheckout(t *testing.T) {
	root := t.TempDir()
	beads := filepath.Join(root, "alpha", ".beads")
	if err := os.MkdirAll(beads, 0o700); err != nil {
		t.Fatal(err)
	}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list: func(context.Context, datasource.DataSource) ([]string, error) {
			return []string{"alpha"}, nil
		},
	}
	discovery.projects(nil)
	if _, err := os.Stat(filepath.Join(beads, "metadata.json")); !os.IsNotExist(err) {
		t.Fatalf("local checkout gained generated metadata: %v", err)
	}
}

func TestWebDoltDiscoveryCollisionDoesNotBlockOtherProjects(t *testing.T) {
	root := t.TempDir()
	names := []string{"alpha"}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:13306", User: "reader"},
		list: func(context.Context, datasource.DataSource) ([]string, error) {
			return names, nil
		},
	}
	initial := discovery.projects(nil)
	if len(initial) != 1 || initial[0].Name != "alpha" {
		t.Fatalf("initial projects = %+v, want alpha", initial)
	}
	localBeads := filepath.Join(root, "beta", ".beads")
	if err := os.MkdirAll(localBeads, 0o700); err != nil {
		t.Fatal(err)
	}
	localIssues := filepath.Join(localBeads, "issues.jsonl")
	const fixture = "{\"id\":\"local-1\",\"title\":\"Local issue\",\"status\":\"open\"}\n"
	if err := os.WriteFile(localIssues, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}

	names = []string{"beta", "gamma"}
	for attempt := 0; attempt < 2; attempt++ {
		got := discovery.projects(initial)
		var projectNames []string
		for _, project := range got {
			projectNames = append(projectNames, project.Name)
		}
		if strings.Join(projectNames, ",") != "beta,gamma" {
			t.Errorf("attempt %d: projects = %v, want local beta and new gamma without disappeared alpha", attempt, projectNames)
		}
	}
	if _, err := os.Stat(filepath.Join(localBeads, "metadata.json")); !os.IsNotExist(err) {
		t.Fatalf("local beta gained generated metadata: %v", err)
	}
	content, err := os.ReadFile(localIssues)
	if err != nil || string(content) != fixture {
		t.Fatalf("local beta content = %q, error = %v", content, err)
	}
}

func TestWebDoltDiscoveryHTTPKeepsSession(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	root := t.TempDir()
	names := []string{"alpha"}
	discovery := &webDoltDiscovery{
		root:   root,
		source: datasource.DataSource{Type: datasource.SourceTypeDolt, Path: "127.0.0.1:1", User: "reader"},
		list: func(context.Context, datasource.DataSource) ([]string, error) {
			return names, nil
		},
	}
	store := web.NewStore(time.Second)
	defer store.Close()
	auth, err := web.NewHeaderAuth([]byte("0123456789abcdef0123456789abcdef"), "X-Forwarded-Email", "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := web.NewServer(web.Options{
		Store: store, Auth: auth,
		Projects: func() []config.Project { return discovery.projects(nil) },
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	client := &http.Client{Timeout: 2 * time.Second}
	get := func(path string, target interface{}) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-Email", "owner@example.com")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatal(err)
		}
	}
	var before, after web.Session
	get("/api/session", &before)
	for _, want := range [][]string{{"alpha"}, {"alpha", "beta"}, {"beta"}, {}} {
		names = want
		var list web.ProjectList
		get("/api/projects", &list)
		var got []string
		for _, p := range list.Projects {
			got = append(got, p.Name)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("HTTP projects = %v, want %v", got, want)
		}
	}
	get("/api/session", &after)
	if before.CSRF == "" || before.CSRF != after.CSRF {
		t.Fatalf("CSRF changed during discovery: %q -> %q", before.CSRF, after.CSRF)
	}
}

func TestWebRefusesNoTokenOffLoopback(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, addr := range []string{"0.0.0.0:0", ":0", "100.64.0.1:0"} {
		var out, errOut bytes.Buffer
		code := runWeb([]string{"--no-token", "--listen", addr}, &out, &errOut)
		if code != 2 || !strings.Contains(errOut.String(), "without a pairing token") {
			t.Errorf("%s: exit %d, stderr %q", addr, code, errOut.String())
		}
	}
}

func TestWebTrustHeaderNeedsOwnerAndAToken(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, args := range [][]string{
		{"--trust-header", "X-Forwarded-Email"},
		{"--owner", "a@b.c"},
		{"--trust-header", "X-Forwarded-Email", "--owner", "a@b.c", "--no-token"},
		{"--discover-dolt"},
	} {
		var out, errOut bytes.Buffer
		if code := runWeb(args, &out, &errOut); code != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}

func TestWebHelpExitsCleanly(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runWeb([]string{"--help"}, &out, &errOut); code != 0 || !strings.Contains(errOut.String(), "pairing link") {
		t.Fatalf("exit %d, usage %q", code, errOut.String())
	}
}

func TestPairURLCarriesTokenOnlyWithAuth(t *testing.T) {
	if got := pairURL("http://localhost:7979", nil); got != "http://localhost:7979/" {
		t.Fatalf("no auth: %q", got)
	}
}

func TestWithCheckoutsUnderAddsEveryCheckoutOnce(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"beta", "alpha", "gamma"} {
		if err := os.MkdirAll(filepath.Join(root, name, ".beads"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "no-beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	recent := []config.Project{{Name: "gamma", Path: filepath.Join(root, "gamma")}}

	got := withCheckoutsUnder(recent, root)
	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "gamma,alpha,beta" {
		t.Fatalf("projects = %v, want the recent one first, then the others by name", names)
	}
	if again := withCheckoutsUnder(recent, ""); len(again) != 1 {
		t.Fatalf("no root added projects: %v", again)
	}
}

func TestWebPublicRejectsPairingAndProjectFlags(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	for _, extra := range [][]string{
		{"--no-token"}, {"--new-token"}, {"--projects-root", "/tmp"},
		{"--discover-dolt"},
		{"--trust-header", "X-Forwarded-Email", "--owner", "a@b.c"},
	} {
		var out, errOut bytes.Buffer
		args := append([]string{"--public", "--listen", "127.0.0.1:0"}, extra...)
		if code := runWeb(args, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "--public cannot be combined") {
			t.Errorf("%v: exit %d, stderr %q", extra, code, errOut.String())
		}
	}
}

func TestWebBannerLinkMustBeAWebAddress(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	var out, errOut bytes.Buffer
	if code := runWeb([]string{"--public", "--banner-link", "javascript:alert(1)"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
}

func TestRecentFromTarget_ReadsServerOfCheckoutSoHeaderListsItOnce(t *testing.T) {
	checkout := filepath.Join(t.TempDir(), "b9s")
	if err := os.MkdirAll(filepath.Join(checkout, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"backend":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"root","dolt_database":"b9s"}`
	if err := os.WriteFile(filepath.Join(checkout, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	recent := []config.RecentProject{{Name: "b9s", Database: "b9s", Host: "127.0.0.1:3306"}}

	got := ui.HeaderProjects(recent, recentFromTarget(datasource.OpenTarget{Name: "b9s", Dir: checkout}))

	if len(got) != 1 || got[0].Path != checkout {
		t.Errorf("header = %+v, want one b9s row with path %s", got, checkout)
	}
}

func TestWebDevAssetsNeedsLoopbackAndNoPublic(t *testing.T) {
	t.Setenv("B9S_TEST_MODE", "1")
	dir := t.TempDir()
	for _, args := range [][]string{
		{"--dev-assets", dir, "--listen", "0.0.0.0:0"},
		{"--dev-assets", dir, "--public", "--listen", "127.0.0.1:0"},
		{"--dev-assets", filepath.Join(dir, "missing"), "--listen", "127.0.0.1:0"},
	} {
		var out, errOut bytes.Buffer
		if code := runWeb(args, &out, &errOut); code != 2 {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}
