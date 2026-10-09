// Package docs guards the shape of the repository's documentation.
//
// The README is a front page and docs/USER_GUIDE.md is the one reference
// (ADR 0052, docs/plans/2026-10-08-readme-restructure.md). The line cap keeps
// that decision from eroding one key table at a time, and the link check makes
// moving content between the two files safe: a heading renamed in the guide
// breaks the README link that points at it.
package docs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readmeMaxLines is the cap from ADR 0052. The README was 525 lines before the
// restructure and about 180 after it, so the cap leaves room for a section or
// two without letting the reference creep back in.
const readmeMaxLines = 300

var repoRoot = filepath.Join("..", "..")

// checkedFiles are the documents whose relative links and anchors must resolve.
var checkedFiles = []string{
	"README.md",
	filepath.Join("docs", "USER_GUIDE.md"),
}

func TestReadmeStaysAFrontPage(t *testing.T) {
	lines := readLines(t, filepath.Join(repoRoot, "README.md"))
	if len(lines) > readmeMaxLines {
		t.Fatalf("README.md has %d lines, the cap is %d; move reference material into docs/USER_GUIDE.md and link to it (ADR 0052)", len(lines), readmeMaxLines)
	}
}

func TestReadmeHasNoEmDashes(t *testing.T) {
	for _, path := range checkedFiles {
		for i, line := range readLines(t, filepath.Join(repoRoot, path)) {
			if strings.ContainsRune(line, '—') {
				t.Errorf("%s:%d contains an em dash; use a comma, a colon or two sentences", path, i+1)
			}
		}
	}
}

// markdownLink matches [text](target) and ![alt](target). Targets with spaces
// or nested parentheses do not occur in these documents.
var markdownLink = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]+)\)`)

// htmlSrc matches the src attribute of the inline <img> tags the README uses
// where Markdown cannot set a width.
var htmlSrc = regexp.MustCompile(`<img[^>]*\ssrc="([^"]+)"`)

func TestRelativeLinksResolve(t *testing.T) {
	for _, path := range checkedFiles {
		full := filepath.Join(repoRoot, path)
		lines := readLines(t, full)
		for i, line := range lines {
			for _, target := range linkTargets(line) {
				checkTarget(t, path, i+1, target)
			}
		}
	}
}

func linkTargets(line string) []string {
	var targets []string
	for _, m := range markdownLink.FindAllStringSubmatch(line, -1) {
		targets = append(targets, m[1])
	}
	for _, m := range htmlSrc.FindAllStringSubmatch(line, -1) {
		targets = append(targets, m[1])
	}
	return targets
}

func checkTarget(t *testing.T, from string, lineNo int, target string) {
	t.Helper()
	if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
		return
	}
	file, anchor, _ := strings.Cut(target, "#")
	var resolved string
	if file == "" {
		resolved = filepath.Join(repoRoot, from)
	} else {
		resolved = filepath.Join(repoRoot, filepath.Dir(from), file)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		t.Errorf("%s:%d links to %q, which does not exist (%v)", from, lineNo, target, err)
		return
	}
	if anchor == "" {
		return
	}
	if info.IsDir() || !strings.HasSuffix(resolved, ".md") {
		t.Errorf("%s:%d uses an anchor on %q, which is not a Markdown file", from, lineNo, target)
		return
	}
	if !headingSlugs(t, resolved)[anchor] {
		t.Errorf("%s:%d links to anchor %q, and %s has no heading with that slug", from, lineNo, anchor, file)
	}
}

// headingSlugs returns the GitHub anchor of every ATX heading in a Markdown
// file. Fenced code blocks are skipped, since a comment line that starts with
// # inside one is not a heading.
func headingSlugs(t *testing.T, path string) map[string]bool {
	t.Helper()
	slugs := map[string]bool{}
	seen := map[string]int{}
	inFence := false
	for _, line := range readLines(t, path) {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimLeft(line, "#")
		if text == "" || text[0] != ' ' {
			continue
		}
		slug := githubSlug(strings.TrimSpace(text))
		if n := seen[slug]; n > 0 {
			slugs[slug+"-"+itoa(n)] = true
		} else {
			slugs[slug] = true
		}
		seen[slug]++
	}
	return slugs
}

var slugDrop = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// githubSlug follows GitHub's heading-to-anchor rule: lowercase, drop
// punctuation other than hyphens and underscores, spaces become hyphens.
// Inline code and link markup are removed first so `bd` and [x](y) slug as
// their text.
func githubSlug(heading string) string {
	s := strings.ReplaceAll(heading, "`", "")
	s = markdownLink.ReplaceAllStringFunc(s, func(link string) string {
		return link[1:strings.Index(link, "](")]
	})
	s = strings.ToLower(s)
	s = slugDrop.ReplaceAllString(s, "")
	return strings.ReplaceAll(s, " ", "-")
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return lines
}
