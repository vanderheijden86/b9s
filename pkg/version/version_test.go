package version

import (
	"runtime/debug"
	"testing"
)

func buildInfo(mainVersion string, settings map[string]string) *debug.BuildInfo {
	bi := &debug.BuildInfo{Main: debug.Module{Path: "github.com/vanderheijden86/b9s", Version: mainVersion}}
	for k, v := range settings {
		bi.Settings = append(bi.Settings, debug.BuildSetting{Key: k, Value: v})
	}
	return bi
}

const sha = "0704a347f00dbabe0123456789abcdef01234567"

func TestResolve(t *testing.T) {
	tests := []struct {
		name                string
		ldVersion, ldCommit string
		bi                  *debug.BuildInfo
		want                Info
	}{
		{
			name:      "release build: ldflags win over build info",
			ldVersion: "v1.3.3", ldCommit: sha,
			bi:   buildInfo("v1.3.3", map[string]string{"vcs.revision": "ffff", "vcs.modified": "false"}),
			want: Info{Version: "v1.3.3", Commit: sha},
		},
		{
			name: "go install module@tag: module version, no vcs settings",
			bi:   buildInfo("v1.3.3", nil),
			want: Info{Version: "v1.3.3"},
		},
		{
			name: "local build on a tag: vcs stamp",
			bi:   buildInfo("v1.3.3", map[string]string{"vcs.revision": sha, "vcs.modified": "false"}),
			want: Info{Version: "v1.3.3", Commit: sha},
		},
		{
			name: "local build after a tag, uncommitted changes",
			bi:   buildInfo("v1.3.4-0.20261010120000-0704a347f00d+dirty", map[string]string{"vcs.revision": sha, "vcs.modified": "true"}),
			want: Info{Version: "v1.3.4-0.20261010120000-0704a347f00d+dirty", Commit: sha, Modified: true},
		},
		{
			name: "no build info at all",
			bi:   nil,
			want: Info{Version: "dev"},
		},
		{
			name: "devel module version",
			bi:   buildInfo("(devel)", map[string]string{"vcs.revision": sha}),
			want: Info{Version: "dev", Commit: sha},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.ldVersion, tt.ldCommit, tt.bi); got != tt.want {
				t.Fatalf("resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInfoDisplay(t *testing.T) {
	tests := []struct {
		in                         Info
		label, short, url, string_ string
	}{
		{
			in:    Info{Version: "v1.3.3", Commit: sha},
			label: "v1.3.3", short: "0704a347",
			url:     "https://github.com/vanderheijden86/b9s/commit/" + sha,
			string_: "v1.3.3 (0704a347)",
		},
		{
			in:    Info{Version: "v1.3.4-0.20261010120000-0704a347f00d+dirty", Commit: sha, Modified: true},
			label: "dev after v1.3.3", short: "0704a347",
			url:     "https://github.com/vanderheijden86/b9s/commit/" + sha,
			string_: "dev after v1.3.3 (0704a347, modified)",
		},
		{
			in:    Info{Version: "v1.4.0-rc.1.0.20261010120000-0704a347f00d", Commit: sha},
			label: "dev after v1.4.0-rc.1", short: "0704a347",
			url:     "https://github.com/vanderheijden86/b9s/commit/" + sha,
			string_: "dev after v1.4.0-rc.1 (0704a347)",
		},
		{
			in:    Info{Version: "v0.0.0-20261010120000-0704a347f00d", Commit: sha},
			label: "dev", short: "0704a347",
			url:     "https://github.com/vanderheijden86/b9s/commit/" + sha,
			string_: "dev (0704a347)",
		},
		{
			in:    Info{Version: "v1.3.3"},
			label: "v1.3.3", short: "", url: "", string_: "v1.3.3",
		},
		{
			in:    Info{Version: "dev", Commit: "not-a-sha"},
			label: "dev", short: "", url: "", string_: "dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.in.Version, func(t *testing.T) {
			if got := tt.in.Label(); got != tt.label {
				t.Errorf("Label() = %q, want %q", got, tt.label)
			}
			if got := tt.in.ShortCommit(); got != tt.short {
				t.Errorf("ShortCommit() = %q, want %q", got, tt.short)
			}
			if got := tt.in.CommitURL(); got != tt.url {
				t.Errorf("CommitURL() = %q, want %q", got, tt.url)
			}
			if got := tt.in.String(); got != tt.string_ {
				t.Errorf("String() = %q, want %q", got, tt.string_)
			}
		})
	}
}
