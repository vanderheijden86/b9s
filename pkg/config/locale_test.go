package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocaleMonthFirst(t *testing.T) {
	cases := map[string]bool{
		"en_US.UTF-8":        true,
		"en_US":              true,
		"en-US":              true,
		"es_PR.UTF-8":        true,
		"fil_PH":             true,
		"en_CA":              true,
		"en_NL":              false,
		"nl_NL.UTF-8":        false,
		"en_GB.UTF-8@euro":   false,
		"de_DE@euro":         false,
		"zh_Hans_CN":         false,
		"en_US@rg=nlzzzz":    false, // macOS region override: English text, Dutch formats
		"nl_NL@rg=uszzzz":    true,
		"C":                  false,
		"POSIX":              false,
		"":                   false,
		"en":                 false,
		"en_US.UTF-8@rg=gbz": false, // a malformed override still names a region
	}
	for locale, want := range cases {
		if got := LocaleMonthFirst(locale); got != want {
			t.Errorf("LocaleMonthFirst(%q) = %v, want %v", locale, got, want)
		}
	}
}

func TestPickLocale_AppleLocaleBeatsTerminalEnvironment(t *testing.T) {
	env := map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "en_US"}
	if got := pickLocale("en_NL", mapEnv(env)); got != "en_NL" {
		t.Fatalf("pickLocale = %q, want the macOS region en_NL", got)
	}
}

func TestPickLocale_EnvironmentOrder(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LC_ALL": "en_US", "LC_TIME": "nl_NL", "LANG": "de_DE"}, "en_US"},
		{map[string]string{"LC_TIME": "nl_NL", "LANG": "en_US"}, "nl_NL"},
		{map[string]string{"LANG": "en_US"}, "en_US"},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := pickLocale("", mapEnv(c.env)); got != c.want {
			t.Errorf("pickLocale(env=%v) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestDateOrderMonthFirst(t *testing.T) {
	calls := 0
	us := func() string { calls++; return "en_US" }

	if !DateOrderMDY.MonthFirst(us) || DateOrderDMY.MonthFirst(us) {
		t.Fatal("an explicit date order must win over the locale")
	}
	if calls != 0 {
		t.Fatalf("an explicit date order must not look up the locale, looked %d times", calls)
	}
	if !DateOrderAuto.MonthFirst(us) || !DateOrder("").MonthFirst(us) {
		t.Fatal("auto and unset must follow the locale")
	}
	if DateOrderAuto.MonthFirst(func() string { return "nl_NL" }) {
		t.Fatal("auto with a Dutch locale must put the day first")
	}
}

func TestLoadFrom_DateOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("ui:\n  date_order: mdy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFrom(path)
	if err != nil || cfg.UI.DateOrder != DateOrderMDY {
		t.Fatalf("date_order: mdy loaded as %q, err %v", cfg.UI.DateOrder, err)
	}

	if err := os.WriteFile(path, []byte("ui:\n  date_order: ymd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(path); err == nil {
		t.Fatal("an unknown date_order must be rejected")
	}
}

func mapEnv(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}
