// Package version reports which build of b9s is running: the release version
// and the commit it was built from.
package version

import (
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

// Version and Commit are set at release time through the linker, e.g.
//
//	go build -ldflags "-X github.com/vanderheijden86/b9s/pkg/version.Version=v1.2.3 \
//	  -X github.com/vanderheijden86/b9s/pkg/version.Commit=<sha>"
//
// A build without them (go install module@tag, make build) takes both from
// the Go build info, and init replaces Version with the resolved value so the
// updater compares against what actually runs.
var (
	Version = ""
	Commit  = ""
)

// RepoURL is the public repository the commit links point into.
const RepoURL = "https://github.com/vanderheijden86/b9s"

// Info identifies one build.
type Info struct {
	// Version is a semantic version: a release tag, a Go pseudo-version for an
	// untagged commit, or "dev" when the build carries neither.
	Version string
	// Commit is the full commit hash, or "" when the build does not know it.
	Commit string
	// Modified is true when the working tree had uncommitted changes, so the
	// commit alone does not reproduce the build.
	Modified bool
}

var current Info

func init() {
	bi, _ := debug.ReadBuildInfo()
	current = resolve(Version, Commit, bi)
	Version = current.Version
}

// Get returns the running build.
func Get() Info { return current }

func resolve(ldVersion, ldCommit string, bi *debug.BuildInfo) Info {
	info := Info{Version: ldVersion, Commit: ldCommit}
	if bi != nil {
		if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = s.Value
				}
			case "vcs.modified":
				// A linker-stamped commit is a release from a clean tag; the
				// local stamp describes the tree the release tool ran in.
				if ldCommit == "" {
					info.Modified = s.Value == "true"
				}
			}
		}
	}
	if info.Version == "" {
		info.Version = "dev"
	}
	return info
}

var (
	hexCommit = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	// A Go pseudo-version ends in a 14-digit UTC timestamp and a 12-character
	// commit prefix; what precedes them encodes the closest earlier tag.
	pseudoTail = regexp.MustCompile(`^(.*?)\d{14}-[0-9a-f]{12}$`)
)

// Label is the version for people to read. A pseudo-version becomes
// "dev after <tag>", since its own number names a release that does not exist.
func (i Info) Label() string {
	v, _, _ := strings.Cut(i.Version, "+")
	m := pseudoTail.FindStringSubmatch(v)
	if m == nil {
		return i.Version
	}
	prefix := m[1]
	// vX.Y.(Z+1)-0.<time>-<hash>: built after release tag vX.Y.Z.
	if core, ok := strings.CutSuffix(prefix, "-0."); ok {
		if dot := strings.LastIndex(core, "."); dot > 0 {
			if z, err := strconv.Atoi(core[dot+1:]); err == nil && z > 0 {
				return "dev after " + core[:dot+1] + strconv.Itoa(z-1)
			}
		}
		return "dev"
	}
	// vX.Y.Z-pre.0.<time>-<hash>: built after prerelease tag vX.Y.Z-pre.
	if base, ok := strings.CutSuffix(prefix, ".0."); ok && strings.Contains(base, "-") {
		return "dev after " + base
	}
	return "dev"
}

// ShortCommit is the first eight characters of the commit, or "".
func (i Info) ShortCommit() string {
	if !hexCommit.MatchString(i.Commit) {
		return ""
	}
	return i.Commit[:min(8, len(i.Commit))]
}

// CommitURL links to the commit on GitHub, or is "" without a commit.
func (i Info) CommitURL() string {
	if !hexCommit.MatchString(i.Commit) {
		return ""
	}
	return RepoURL + "/commit/" + i.Commit
}

// String is the one-line form: "v1.3.3 (0704a347)".
func (i Info) String() string {
	s := i.Label()
	if c := i.ShortCommit(); c != "" {
		if i.Modified {
			c += ", modified"
		}
		s += " (" + c + ")"
	}
	return s
}
