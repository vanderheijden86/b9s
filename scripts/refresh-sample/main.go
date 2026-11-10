package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vanderheijden86/b9s/internal/bdrun"
)

// A closed projection prevents future export fields from publishing private
// text, identities, attachment references or environment metadata by default.
type sampleIssue struct {
	ID               string             `json:"id"`
	Title            string             `json:"title"`
	Status           string             `json:"status"`
	Priority         int                `json:"priority"`
	IssueType        string             `json:"issue_type"`
	CreatedAt        string             `json:"created_at,omitempty"`
	UpdatedAt        string             `json:"updated_at,omitempty"`
	StartedAt        string             `json:"started_at,omitempty"`
	ClosedAt         string             `json:"closed_at,omitempty"`
	DueAt            string             `json:"due_at,omitempty"`
	DueDate          string             `json:"due_date,omitempty"`
	DeferUntil       string             `json:"defer_until,omitempty"`
	EstimatedMinutes *int               `json:"estimated_minutes,omitempty"`
	Labels           []string           `json:"labels,omitempty"`
	Dependencies     []sampleDependency `json:"dependencies,omitempty"`
}

type sampleDependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
	CreatedAt   string `json:"created_at,omitempty"`
}

var (
	issueID = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)
	contact = regexp.MustCompile(`[[:alnum:]._%+-]+@[[:alnum:].-]+`)
	url     = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://|www\.)[^\s<>"` + "`" + `]+`)
	winPath = regexp.MustCompile(`[A-Za-z]:\\[^\s<>"` + "`" + `]+`)
	absPath = regexp.MustCompile(`/[[:alnum:]_.~-]+[^\s<>"` + "`" + `]*`)
)

func scrubDisplay(text string) string {
	for _, pattern := range []*regexp.Regexp{url, contact, winPath, absPath} {
		text = pattern.ReplaceAllString(text, "[redacted]")
	}
	return text
}

func main() {
	source := flag.String("source", ".", "b9s checkout to export (read-only)")
	output := flag.String("output", "tests/testdata/real/b9s.jsonl", "sanitized JSONL destination")
	flag.Parse()
	if flag.NArg() != 0 || !strings.HasSuffix(*output, ".jsonl") {
		fmt.Fprintln(os.Stderr, "use -source DIR -output FILE.jsonl")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	count, err := refresh(ctx, *source, *output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %d issues to %s and %s\n", count, *output, countPath(*output))
}

func countPath(output string) string {
	return strings.TrimSuffix(output, ".jsonl") + ".count"
}

func refresh(ctx context.Context, source, output string) (int, error) {
	project, err := filepath.Abs(source)
	if err != nil {
		return 0, fmt.Errorf("resolving source checkout: %w", err)
	}
	bd, ok := bdrun.Resolve()
	if !ok {
		return 0, fmt.Errorf("exporting b9s issues: bd is not installed or not on PATH")
	}
	raw, stderr, err := bdrun.Output(ctx, bd, project,
		[]string{"BEADS_DIR=" + filepath.Join(project, ".beads")},
		"-C", project, "export", "--readonly")
	if err != nil {
		return 0, fmt.Errorf("exporting b9s issues: %w: %s", err, stderr)
	}
	data, count, err := sanitizeExport(bytes.NewReader(raw))
	if err != nil {
		return 0, fmt.Errorf("sanitizing b9s export: %w", err)
	}
	if err := os.WriteFile(output, data, 0644); err != nil {
		return 0, fmt.Errorf("writing fixture: %w", err)
	}
	if err := os.WriteFile(countPath(output), []byte(strconv.Itoa(count)+"\n"), 0644); err != nil {
		return 0, fmt.Errorf("writing issue count: %w", err)
	}
	return count, nil
}

func sanitizeExport(input io.Reader) ([]byte, int, error) {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var issues []sampleIssue
	ids := map[string]bool{}
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var issue sampleIssue
		if err := json.Unmarshal(scanner.Bytes(), &issue); err != nil {
			return nil, 0, fmt.Errorf("parsing issue %d: %w", len(issues)+1, err)
		}
		if !issueID.MatchString(issue.ID) || strings.TrimSpace(issue.Title) == "" {
			return nil, 0, fmt.Errorf("issue %d needs a portable ID and title", len(issues)+1)
		}
		if ids[issue.ID] {
			return nil, 0, fmt.Errorf("duplicate issue ID %s", issue.ID)
		}
		ids[issue.ID] = true
		issue.Title = scrubDisplay(issue.Title)
		labels := make([]string, 0, len(issue.Labels))
		for _, label := range issue.Labels {
			if strings.HasPrefix(label, "dispatch") || strings.HasPrefix(label, "lane-") {
				continue
			}
			labels = append(labels, scrubDisplay(label))
		}
		issue.Labels = labels
		issues = append(issues, issue)
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading export: %w", err)
	}
	if len(issues) == 0 {
		return nil, 0, fmt.Errorf("export has no issues")
	}
	for _, issue := range issues {
		for _, dep := range issue.Dependencies {
			if dep.IssueID != issue.ID || !ids[dep.DependsOnID] {
				return nil, 0, fmt.Errorf("issue %s has a dependency with a wrong source or missing target", issue.ID)
			}
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].ID < issues[j].ID })
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, issue := range issues {
		if err := encoder.Encode(issue); err != nil {
			return nil, 0, fmt.Errorf("encoding issue %s: %w", issue.ID, err)
		}
	}
	return output.Bytes(), len(issues), nil
}
