// Package gitlog reads commit history with the git CLI and draws a compact
// one-row-per-commit lane graph. It links commits to Beads issues through the
// Refs: trailer the commit strategy requires.
package gitlog

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Commit is one commit.
type Commit struct {
	Repo    string    `json:"repo,omitempty"`
	Hash    string    `json:"hash"`
	Short   string    `json:"short"`
	Parents []string  `json:"parents"`
	Time    time.Time `json:"time"`
	Author  string    `json:"author"`
	Email   string    `json:"email"`
	Refs    []string  `json:"refs,omitempty"`
	Subject string    `json:"subject"`
	Body    string    `json:"body,omitempty"`
	Issues  []string  `json:"issues,omitempty"`
	Graph   string    `json:"graph,omitempty"`
}

// Options bounds a log read.
type Options struct {
	Max int  // default 400
	All bool // every branch, not only HEAD
	// Grep keeps commits whose message matches (git log --grep).
	Grep string
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
	logFormat = "%H%x1f%P%x1f%at%x1f%an%x1f%ae%x1f%D%x1f%s%x1f%b%x1e"
)

// Read runs git log in dir.
func Read(ctx context.Context, dir string, opts Options) ([]Commit, error) {
	if opts.Max <= 0 {
		opts.Max = 400
	}
	args := []string{"-C", dir, "log", "--topo-order", "--no-color", "-n", strconv.Itoa(opts.Max), "--format=" + logFormat}
	if opts.All {
		args = append(args, "--branches", "--remotes", "--tags", "HEAD")
	}
	if opts.Grep != "" {
		args = append(args, "--fixed-strings", "--grep", opts.Grep)
	}
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, gitErr(err)
	}
	commits := Parse(string(out))
	Draw(commits)
	return commits, nil
}

func gitErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("git: %s", strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}

// Parse reads output in logFormat.
func Parse(out string) []Commit {
	var commits []Commit
	for _, rec := range strings.Split(out, recordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, fieldSep, 8)
		if len(f) < 8 {
			continue
		}
		secs, _ := strconv.ParseInt(f[2], 10, 64)
		c := Commit{
			Hash:    f[0],
			Short:   short(f[0]),
			Parents: strings.Fields(f[1]),
			Time:    time.Unix(secs, 0),
			Author:  f[3],
			Email:   f[4],
			Refs:    splitRefs(f[5]),
			Subject: f[6],
			Body:    strings.TrimSpace(f[7]),
		}
		c.Issues = Issues(c.Body)
		commits = append(commits, c)
	}
	return commits
}

func short(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func splitRefs(d string) []string {
	var out []string
	for _, r := range strings.Split(d, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

var refsRe = regexp.MustCompile(`(?mi)^\s*(?:refs|closes|fixes)\s*:\s*(.+)$`)

// Issues reads the Beads ids from Refs:, Closes: and Fixes: trailers.
func Issues(body string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range refsRe.FindAllStringSubmatch(body, -1) {
		for _, id := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
			id = strings.Trim(id, "#.;")
			if id != "" && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// Merge interleaves several repositories' logs by commit time, newest first.
// Graphs are dropped: lanes of different repositories do not connect.
func Merge(logs map[string][]Commit) []Commit {
	var all []Commit
	for repo, cs := range logs {
		for _, c := range cs {
			c.Repo = repo
			c.Graph = ""
			all = append(all, c)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Time.After(all[j].Time) })
	return all
}

// FileStat is one changed file of a commit.
type FileStat struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary,omitempty"`
}

// Detail is a commit with its files and diff.
type Detail struct {
	Commit Commit     `json:"commit"`
	Files  []FileStat `json:"files"`
	Diff   string     `json:"diff"`
	// Truncated says the diff was cut at maxDiffLines.
	Truncated bool `json:"truncated,omitempty"`
}

const maxDiffLines = 4000

// Show reads one commit's message, numstat and patch.
func Show(ctx context.Context, dir, hash string) (Detail, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "show", "--no-color", "--numstat", "--format="+logFormat, hash).Output()
	if err != nil {
		return Detail{}, gitErr(err)
	}
	head, stats, _ := strings.Cut(string(out), recordSep)
	cs := Parse(head + recordSep)
	if len(cs) == 0 {
		return Detail{}, fmt.Errorf("git show %s: no commit", hash)
	}
	d := Detail{Commit: cs[0], Files: ParseNumstat(stats)}
	patch, err := exec.CommandContext(ctx, "git", "-C", dir, "show", "--no-color", "--format=", "--patch", hash).Output()
	if err == nil {
		lines := strings.Split(string(patch), "\n")
		if len(lines) > maxDiffLines {
			lines, d.Truncated = lines[:maxDiffLines], true
		}
		d.Diff = strings.Join(lines, "\n")
	}
	return d, nil
}

// ParseNumstat reads git --numstat lines.
func ParseNumstat(s string) []FileStat {
	var out []FileStat
	for _, ln := range strings.Split(s, "\n") {
		f := strings.SplitN(ln, "\t", 3)
		if len(f) != 3 {
			continue
		}
		fs := FileStat{Path: f[2]}
		if f[0] == "-" {
			fs.Binary = true
		} else {
			fs.Added, _ = strconv.Atoi(f[0])
			fs.Deleted, _ = strconv.Atoi(f[1])
		}
		out = append(out, fs)
	}
	return out
}

// Root returns the top directory of the work tree that holds dir.
func Root(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", gitErr(err)
	}
	return strings.TrimSpace(string(out)), nil
}
