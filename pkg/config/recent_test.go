package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func recent(name string) RecentProject {
	return RecentProject{Name: name, Database: name, Host: "127.0.0.1:3306"}
}

func recentNames(projects []RecentProject) []string {
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	return names
}

func TestTouchRecent_PrependsNewProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b")}

	changed := cfg.TouchRecent(recent("c"))

	if !changed {
		t.Error("expected TouchRecent to report a change")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"c", "a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestForgetRecent_MatchesIdentityAndKeepsOtherProjectsInOrder(t *testing.T) {
	target := recent("a")
	otherHost := target
	otherHost.Host = "127.0.0.1:3307"
	cfg := Config{RecentProjects: []RecentProject{target, recent("b"), otherHost}}
	before := cfg.RecentProjects
	renamed := target
	renamed.Name = "renamed"

	if !cfg.ForgetRecent(renamed) {
		t.Fatal("ForgetRecent did not report removal by server identity")
	}
	if want := []RecentProject{recent("b"), otherHost}; !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Fatalf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
	if before[0] != target {
		t.Fatal("ForgetRecent changed a retained copy of the list")
	}
}

func TestForgetRecent_RespectsLock(t *testing.T) {
	cfg := Config{RecentProjects: []RecentProject{recent("a")}, LockRecent: true}
	if cfg.ForgetRecent(recent("a")) || len(cfg.RecentProjects) != 1 {
		t.Fatal("ForgetRecent changed locked list")
	}
}

func TestForgetRecent_MatchesCheckoutPath(t *testing.T) {
	cfg := Config{RecentProjects: []RecentProject{{Name: "local", Path: "/work/local"}}}
	if !cfg.ForgetRecent(RecentProject{Path: "/work/local"}) || len(cfg.RecentProjects) != 0 {
		t.Fatalf("recent = %+v, want checkout forgotten", cfg.RecentProjects)
	}
}

func TestForgetRecent_IgnoresUnknownProject(t *testing.T) {
	cfg := Config{RecentProjects: []RecentProject{recent("a")}}
	if cfg.ForgetRecent(recent("unknown")) || len(cfg.RecentProjects) != 1 {
		t.Fatal("ForgetRecent changed the list for an unknown project")
	}
}

func TestSaveRecentTo_ForgetsOnlyTargetWhileMergingAnotherWindow(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveRecentTo(cfgPath, []RecentProject{recent("a"), recent("b"), recent("from-other-window")}, false); err != nil {
		t.Fatal(err)
	}

	if err := SaveRecentTo(cfgPath, []RecentProject{recent("b")}, false, recent("a")); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"b", "from-other-window"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("recent = %v, want %v", got, want)
	}
}

func TestSaveRecentTo_LockPreventsForget(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	projects := []RecentProject{recent("a")}
	if err := SaveRecentTo(cfgPath, projects, true, recent("a")); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.RecentProjects, projects) {
		t.Fatalf("recent = %+v, want locked project retained", cfg.RecentProjects)
	}
}

func TestTouchRecent_KeepsSlotOfExistingProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("a"), recent("b"), recent("c")}

	changed := cfg.TouchRecent(recent("c"))

	if changed {
		t.Error("expected no change for a project already in the list")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestTouchRecent_UpdatesCheckoutDatabase(t *testing.T) {
	path := writeDoltProject(t, t.TempDir(), "checkout", "current")
	current, ok := RecentFromCheckout("checkout", path)
	if !ok {
		t.Fatal("RecentFromCheckout did not discover the checkout")
	}
	stale := current
	stale.Database = "stale"
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("first"), stale, recent("last")}

	if !cfg.TouchRecent(current) {
		t.Fatal("TouchRecent must report the database change")
	}
	want := []RecentProject{recent("first"), current, recent("last")}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Fatalf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
	if cfg.TouchRecent(current) {
		t.Error("touching the current entry must report no change")
	}
}

func TestTouchRecent_CollapsesStaleCheckoutEntries(t *testing.T) {
	current := RecentProject{Name: "checkout", Path: t.TempDir(), Host: "localhost:3306", Database: "current"}
	stale := current
	stale.Database = "stale"
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("first"), stale, recent("last"), current}

	cfg.TouchRecent(current)

	want := []RecentProject{recent("first"), current, recent("last")}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
}

func TestTouchRecent_UpdatesCheckoutBehindCatalogEntry(t *testing.T) {
	current := RecentProject{Name: "checkout", Path: t.TempDir(), Host: "localhost:3306", Database: "current"}
	stale := current
	stale.Database = "stale"
	catalog := current
	catalog.Path = ""
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{catalog, recent("other"), stale}

	if !cfg.TouchRecent(current) {
		t.Fatal("TouchRecent must refresh the checkout even when its catalog entry comes first")
	}
	want := []RecentProject{current, recent("other")}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
}

func TestTouchRecent_UpdatesSymlinkedCheckoutDatabase(t *testing.T) {
	root := t.TempDir()
	path := writeDoltProject(t, root, "checkout", "current")
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	current := RecentProject{Name: "checkout", Path: path, Host: "localhost:3306", Database: "current"}
	stale := current
	stale.Path, stale.Database = alias, "stale"
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{stale}

	cfg.TouchRecent(current)

	if !reflect.DeepEqual(cfg.RecentProjects, []RecentProject{current}) {
		t.Errorf("recent = %+v, want only %+v", cfg.RecentProjects, current)
	}
}

func TestRecentProject_SameProjectIdentity(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Chdir(root)
	local := RecentProject{Path: filepath.Join(root, "checkout"), Host: "localhost:3306", Database: "db"}
	cases := []struct {
		name  string
		other RecentProject
		same  bool
	}{
		{"changed database", RecentProject{Path: local.Path, Host: local.Host, Database: "new"}, true},
		{"different host", RecentProject{Path: local.Path, Host: "remote:3306", Database: local.Database}, false},
		{"different path", RecentProject{Path: filepath.Join(root, "other"), Host: local.Host, Database: local.Database}, false},
		{"pathless same database", RecentProject{Host: local.Host, Database: local.Database}, true},
		{"pathless different database", RecentProject{Host: local.Host, Database: "other"}, false},
		{"pathless different host", RecentProject{Host: "remote:3306", Database: local.Database}, false},
		{"missing path cleaned", RecentProject{Path: local.Path + "/../checkout", Host: local.Host}, true},
		{"home expanded", RecentProject{Path: "~/checkout", Host: local.Host}, true},
		{"relative path", RecentProject{Path: "checkout", Host: local.Host}, true},
		{"empty identity", RecentProject{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := local.SameProject(tc.other); got != tc.same {
				t.Errorf("SameProject = %v, want %v", got, tc.same)
			}
			if got := tc.other.SameProject(local); got != tc.same {
				t.Errorf("reverse SameProject = %v, want %v", got, tc.same)
			}
		})
	}
}

func TestTouchRecent_LockedListKeepsCheckoutDatabase(t *testing.T) {
	stale := RecentProject{Path: t.TempDir(), Host: "localhost:3306", Database: "stale"}
	current := stale
	current.Database = "current"
	cfg := DefaultConfig()
	cfg.LockRecent = true
	cfg.RecentProjects = []RecentProject{stale}
	if cfg.TouchRecent(current) || !reflect.DeepEqual(cfg.RecentProjects, []RecentProject{stale}) {
		t.Errorf("locked list changed: %+v", cfg.RecentProjects)
	}
}

func TestTouchRecent_PreservesCheckoutOpenedAt(t *testing.T) {
	stale := RecentProject{Path: t.TempDir(), Host: "localhost:3306", Database: "stale", OpenedAt: time.Unix(100, 0)}
	current := stale
	current.Database, current.OpenedAt = "current", time.Time{}
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{stale}
	cfg.TouchRecent(current)
	if len(cfg.RecentProjects) != 1 || !cfg.RecentProjects[0].OpenedAt.Equal(stale.OpenedAt) {
		t.Errorf("recent = %+v, want the existing opened_at", cfg.RecentProjects)
	}
}

func TestSaveRecentTo_KeepsCurrentCheckoutDatabase(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	stale := RecentProject{Name: "checkout", Path: t.TempDir(), Host: "localhost:3306", Database: "stale"}
	if err := SaveRecentTo(configPath, []RecentProject{stale}, false); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	current := stale
	current.Database = "current"
	cfg.TouchRecent(current)
	if err := SaveRecentTo(configPath, cfg.RecentProjects, false); err != nil {
		t.Fatal(err)
	}
	saved, err := LoadFrom(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.RecentProjects, []RecentProject{current}) {
		t.Errorf("saved recent = %+v, want only %+v", saved.RecentProjects, current)
	}
}

func TestTouchRecent_IdentifiesProjectByHostAndDatabase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{{Name: "a", Database: "a", Host: "127.0.0.1:3306"}}

	cfg.TouchRecent(RecentProject{Name: "a", Database: "a", Host: "10.0.0.5:3306"})

	if got := len(cfg.RecentProjects); got != 2 {
		t.Errorf("expected the same database on another host to be a new entry, got %d entries", got)
	}
}

func TestTouchRecent_DropsProjectBeyondCap(t *testing.T) {
	cfg := DefaultConfig()
	for i := 1; i <= MaxRecentProjects; i++ {
		cfg.RecentProjects = append(cfg.RecentProjects, recent(fmt.Sprintf("p%d", i)))
	}

	cfg.TouchRecent(recent("new"))

	if got := len(cfg.RecentProjects); got != MaxRecentProjects {
		t.Fatalf("expected %d entries, got %d", MaxRecentProjects, got)
	}
	if first, last := cfg.RecentProjects[0].Name, cfg.RecentProjects[MaxRecentProjects-1].Name; first != "new" || last != "p8" {
		t.Errorf("expected new first and p8 last, got %s ... %s", first, last)
	}
}

func TestTouchRecent_LockedListIgnoresNewProject(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LockRecent = true
	cfg.RecentProjects = []RecentProject{recent("a")}

	changed := cfg.TouchRecent(recent("b"))

	if changed {
		t.Error("expected a locked list not to change")
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestLoadFrom_MigratesFavoritesIntoRecentProjects(t *testing.T) {
	dir := t.TempDir()
	alpha := writeDoltProject(t, dir, "alpha", "alpha_db")
	beta := writeDoltProject(t, dir, "beta", "beta_db")
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, fmt.Sprintf(`projects:
  - name: alpha
    path: %s
  - name: beta
    path: %s
favorites:
  2: alpha
  1: beta
`, alpha, beta))

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	want := []RecentProject{
		{Name: "beta", Database: "beta_db", Host: "127.0.0.1:3306", Path: beta},
		{Name: "alpha", Database: "alpha_db", Host: "127.0.0.1:3306", Path: alpha},
	}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
}

func TestLoadFrom_KeepsExistingRecentProjectsOverFavorites(t *testing.T) {
	dir := t.TempDir()
	alpha := writeDoltProject(t, dir, "alpha", "alpha_db")
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, fmt.Sprintf(`projects:
  - name: alpha
    path: %s
favorites:
  1: alpha
recent_projects:
  - name: gamma
    database: gamma
    host: 127.0.0.1:3306
`, alpha))

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if got, want := recentNames(cfg.RecentProjects), []string{"gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestSaveTo_MergesRecentProjectsWrittenByAnotherWindow(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	other := DefaultConfig()
	other.TouchRecent(recent("other"))
	if err := SaveTo(other, cfgPath); err != nil {
		t.Fatalf("SaveTo other: %v", err)
	}

	cfg.TouchRecent(recent("mine"))
	if err := SaveTo(cfg, cfgPath); err != nil {
		t.Fatalf("SaveTo mine: %v", err)
	}

	saved, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom saved: %v", err)
	}
	if got, want := recentNames(saved.RecentProjects), []string{"mine", "other"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestSaveTo_LeavesOnlyTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.TouchRecent(recent("a"))

	if err := SaveTo(cfg, filepath.Join(dir, "config.yaml")); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.yaml" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only config.yaml, found %v", names)
	}
}

func TestTouchRecent_IdentifiesCheckoutWithoutDatabaseByPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{{Name: "a", Path: "/work/a"}, {Name: "b", Path: "/work/b"}}

	sameChanged := cfg.TouchRecent(RecentProject{Name: "a", Path: "/work/a"})
	otherChanged := cfg.TouchRecent(RecentProject{Name: "a", Path: "/elsewhere/a"})

	if sameChanged {
		t.Error("expected the same checkout path to keep its slot")
	}
	if !otherChanged {
		t.Error("expected a different checkout path with the same name to be a new entry")
	}
	if got, want := len(cfg.RecentProjects), 3; got != want {
		t.Errorf("expected %d entries, got %d", want, got)
	}
}

func TestSameProject_ResolvesCheckoutSymlinks(t *testing.T) {
	checkout, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(checkout, link); err != nil {
		t.Fatal(err)
	}
	saved := RecentProject{Name: "saved", Path: link}
	startup := RecentProject{Name: "startup", Path: checkout, Database: "project", Host: "127.0.0.1:3306"}

	if !saved.SameProject(startup) || !startup.SameProject(saved) {
		t.Fatalf("symlink %q and resolved checkout %q must identify the same project", link, checkout)
	}
}

func TestSameProject_CleansMissingCheckoutPaths(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	saved := RecentProject{Path: missing + string(filepath.Separator) + "."}
	startup := RecentProject{Path: missing}

	if !saved.SameProject(startup) || !startup.SameProject(saved) {
		t.Fatalf("missing checkout paths %q and %q must match after cleaning", saved.Path, startup.Path)
	}
}

func TestSameProject_DistinguishesMissingCheckoutPaths(t *testing.T) {
	root := t.TempDir()
	first := RecentProject{Path: filepath.Join(root, "first")}
	second := RecentProject{Path: filepath.Join(root, "second")}

	if first.SameProject(second) {
		t.Fatal("different missing checkout paths must remain distinct")
	}
	if (RecentProject{}).SameProject(RecentProject{Path: "."}) {
		t.Fatal("an empty checkout path must not identify the working directory")
	}
}

func TestTouchRecent_KeepsSlotForSymlinkedCheckout(t *testing.T) {
	checkout := writeJSONLProject(t, t.TempDir(), "project")
	link := filepath.Join(t.TempDir(), "checkout")
	if err := os.Symlink(checkout, link); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{recent("other"), {Name: "project", Path: link}}
	want := append([]RecentProject(nil), cfg.RecentProjects...)

	if cfg.TouchRecent(RecentProject{Name: "project", Path: checkout}) {
		t.Fatal("a symlinked checkout must keep its existing recent slot")
	}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Fatalf("recent = %+v, want unchanged %+v", cfg.RecentProjects, want)
	}
}

func TestRecentFromCheckout_DoltProjectUsesServerIdentity(t *testing.T) {
	path := writeDoltProject(t, t.TempDir(), "alpha", "alpha_db")

	got, ok := RecentFromCheckout("alpha", path)

	want := RecentProject{Name: "alpha", Database: "alpha_db", Host: "127.0.0.1:3306", Path: path}
	if !ok || got != want {
		t.Errorf("RecentFromCheckout = %+v, %v; want %+v, true", got, ok, want)
	}
}

func TestRecentFromCheckout_JSONLProjectUsesPath(t *testing.T) {
	path := writeJSONLProject(t, t.TempDir(), "gamma")

	got, ok := RecentFromCheckout("gamma", path)

	want := RecentProject{Name: "gamma", Path: path}
	if !ok || got != want {
		t.Errorf("RecentFromCheckout = %+v, %v; want %+v, true", got, ok, want)
	}
}

func TestRecentFromCheckout_RejectsDirectoryWithoutBeads(t *testing.T) {
	if got, ok := RecentFromCheckout("empty", t.TempDir()); ok {
		t.Errorf("RecentFromCheckout = %+v, true; want false for a directory without .beads", got)
	}
}

func TestLoadFrom_MigratesJSONLFavoriteByPath(t *testing.T) {
	dir := t.TempDir()
	gamma := writeJSONLProject(t, dir, "gamma")
	cfgPath := filepath.Join(dir, "config.yaml")
	writeFile(t, cfgPath, fmt.Sprintf(`projects:
  - name: gamma
    path: %s
favorites:
  1: gamma
`, gamma))

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	want := []RecentProject{{Name: "gamma", Path: gamma}}
	if !reflect.DeepEqual(cfg.RecentProjects, want) {
		t.Errorf("recent = %+v, want %+v", cfg.RecentProjects, want)
	}
}

func TestTouchRecent_MatchesByPathWhenOnlyOneEntryHasDatabase(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RecentProjects = []RecentProject{{Name: "a", Path: "/work/a", Host: "127.0.0.1:3306"}}

	current := RecentProject{Name: "a", Database: "a", Host: "127.0.0.1:3306", Path: "/work/a"}
	changed := cfg.TouchRecent(current)

	if !changed || !reflect.DeepEqual(cfg.RecentProjects, []RecentProject{current}) {
		t.Errorf("expected the same checkout to refresh its database in place, recent = %+v", cfg.RecentProjects)
	}
}

func TestSaveRecentTo_ReplacesOnlyRecentProjects(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, cfgPath, `# my b9s settings
ui:
  default_view: tree # I like trees
recent_projects:
  - name: old
    path: /work/old
`)

	if err := SaveRecentTo(cfgPath, []RecentProject{{Name: "new", Path: "/work/new"}}, false); err != nil {
		t.Fatalf("SaveRecentTo: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, want := range []string{"# my b9s settings", "default_view: tree # I like trees", "name: new", "name: old"} {
		if !strings.Contains(text, want) {
			t.Errorf("saved config lacks %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"split_ratio", "poll_interval", "max_depth", "sort:"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("saved config gained default %q:\n%s", unwanted, text)
		}
	}
}

func TestSaveRecentTo_CreatesMissingFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "b9s", "config.yaml")

	if err := SaveRecentTo(cfgPath, []RecentProject{{Name: "new", Path: "/work/new"}}, false); err != nil {
		t.Fatalf("SaveRecentTo: %v", err)
	}

	cfg, err := LoadFrom(cfgPath)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got, want := recentNames(cfg.RecentProjects), []string{"new"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recent = %v, want %v", got, want)
	}
}

func TestRecentFromCheckout_RejectsEmptyPathInsideACheckout(t *testing.T) {
	t.Chdir(writeJSONLProject(t, t.TempDir(), "startup"))

	if got, ok := RecentFromCheckout("remote", ""); ok {
		t.Errorf("RecentFromCheckout with an empty path = %+v, true; it described the working directory", got)
	}
}

func writeJSONLProject(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name)
	writeFile(t, filepath.Join(path, ".beads", "issues.jsonl"),
		`{"id":"g-1","title":"Task","status":"open","issue_type":"task","priority":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`+"\n")
	return path
}

func writeDoltProject(t *testing.T, root, name, database string) string {
	t.Helper()
	path := filepath.Join(root, name)
	writeFile(t, filepath.Join(path, ".beads", "metadata.json"), fmt.Sprintf(
		`{"backend":"dolt","dolt_mode":"server","dolt_server_host":"127.0.0.1","dolt_server_port":3306,"dolt_server_user":"root","dolt_database":%q}`,
		database))
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestRecentFromCheckout_EmbeddedProjectIsIdentifiedByPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gamma")
	if err := os.MkdirAll(filepath.Join(path, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"database":"dolt","backend":"dolt","dolt_mode":"embedded","dolt_database":"gamma"}`
	if err := os.WriteFile(filepath.Join(path, ".beads", "metadata.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := RecentFromCheckout("gamma", path)

	want := RecentProject{Name: "gamma", Path: path}
	if !ok || got != want {
		t.Errorf("RecentFromCheckout = %+v, %v; want %+v, true: an embedded store lives in the checkout", got, ok, want)
	}
}
