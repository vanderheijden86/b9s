package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/ui"
)

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
