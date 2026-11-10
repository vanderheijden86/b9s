package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/loader"
)

func projectExport(t *testing.T, input string) []map[string]any {
	t.Helper()
	output, _, err := sanitizeExport(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte("\n")) {
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("invalid JSONL: %v", err)
		}
		rows = append(rows, row)
	}
	return rows
}

func TestProjectionDropsPrivateFields(t *testing.T) {
	rows := projectExport(t, `{"id":"bd-root","title":"Epic","status":"open","priority":1,"issue_type":"epic","assignee":"person@private.test","owner":"owner","created_by":"actor","description":"secret","notes":"secret","design":"secret","acceptance_criteria":"secret","comments":[{"text":"https://private.test/attachment?token=secret"}],"attachments":[{"url":"s3://private/file"}],"metadata":{"token":"secret"},"external_ref":"file:///home/person/doc","unknown_future_field":"secret"}`)
	for _, field := range []string{"assignee", "owner", "created_by", "description", "notes", "design", "acceptance_criteria", "comments", "attachments", "metadata", "external_ref", "unknown_future_field"} {
		if _, exists := rows[0][field]; exists {
			t.Errorf("private field %q survived the projection", field)
		}
	}
}

func TestProjectionRedactsTitlesAndLabels(t *testing.T) {
	rows := projectExport(t, `{"id":"bd-root","title":"Contact person@private.test https://private.test/file s3://bucket/file file:///home/person/doc /Users/person/repo C:\\Users\\person\\repo","status":"open","priority":1,"issue_type":"epic","labels":["feature","person@private.test","https://private.test/file","dispatch-session=private","lane-running"]}`)
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"person@", "private.test", "bucket", "/Users/", `Users\\person`, "dispatch-session", "lane-running"} {
		if bytes.Contains(encoded, []byte(private)) {
			t.Errorf("retained private value %q", private)
		}
	}
	if !strings.Contains(rows[0]["title"].(string), "Contact") {
		t.Error("lost useful title text")
	}
}

func TestProjectionPreservesIDsAndRelationships(t *testing.T) {
	input := `{"id":"bd-root.1.1","title":"Task","status":"blocked","priority":2,"issue_type":"task","dependencies":[{"issue_id":"bd-root.1.1","depends_on_id":"bd-root.1","type":"parent-child","created_by":"person@private.test","metadata":{"path":"/home/person"}},{"issue_id":"bd-root.1.1","depends_on_id":"bd-root","type":"blocks"}]}
{"id":"bd-root","title":"Epic","status":"open","priority":1,"issue_type":"epic"}
{"id":"bd-root.1","title":"Feature","status":"in_progress","priority":1,"issue_type":"feature","dependencies":[{"issue_id":"bd-root.1","depends_on_id":"bd-root","type":"parent-child"}]}`
	rows := projectExport(t, input)
	if len(rows) != 3 {
		t.Fatalf("got %d issues, want 3", len(rows))
	}
	if rows[0]["id"] != "bd-root" || rows[1]["id"] != "bd-root.1" || rows[2]["id"] != "bd-root.1.1" {
		t.Fatal("IDs must be preserved and sorted")
	}
	deps := rows[2]["dependencies"].([]any)
	parent := deps[0].(map[string]any)
	if parent["issue_id"] != "bd-root.1.1" || parent["depends_on_id"] != "bd-root.1" || parent["type"] != "parent-child" {
		t.Fatalf("lost hierarchy: %v", parent)
	}
	if deps[1].(map[string]any)["type"] != "blocks" {
		t.Fatal("lost blocking dependency")
	}
	if _, exists := parent["created_by"]; exists {
		t.Error("dependency retains identity")
	}
	if _, exists := parent["metadata"]; exists {
		t.Error("dependency retains metadata")
	}
}

func TestProjectionRejectsInvalidExport(t *testing.T) {
	for _, tc := range []struct {
		name, input string
	}{
		{"empty", ""},
		{"malformed", "{broken"},
		{"missing ID", `{"title":"Issue"}`},
		{"duplicate ID", "{\"id\":\"bd-a\",\"title\":\"A\"}\n{\"id\":\"bd-a\",\"title\":\"B\"}"},
		{"missing target", `{"id":"bd-a","title":"A","dependencies":[{"issue_id":"bd-a","depends_on_id":"bd-missing","type":"parent-child"}]}`},
		{"wrong source", `{"id":"bd-a","title":"A","dependencies":[{"issue_id":"bd-other","depends_on_id":"bd-a","type":"blocks"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := sanitizeExport(strings.NewReader(tc.input)); err == nil {
				t.Fatal("invalid export accepted")
			}
		})
	}
}

func fakeBD(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\nset -eu\n"+body), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRefreshUsesReadOnlyExportAndRecordsCount(t *testing.T) {
	source := t.TempDir()
	t.Setenv("SAMPLE_SOURCE", source)
	fakeBD(t, `[ "$#" = 4 ]
[ "$1" = "-C" ]
[ "$2" = "$SAMPLE_SOURCE" ]
[ "$3" = "export" ]
[ "$4" = "--readonly" ]
printf '%s\n' '{"id":"bd-a","title":"Task","status":"open","priority":2,"issue_type":"task"}'
`)
	output := filepath.Join(t.TempDir(), "sample.jsonl")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count, err := refresh(ctx, source, output)
	if err != nil {
		t.Fatal(err)
	}
	recorded, err := os.ReadFile(countPath(output))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || string(recorded) != "1\n" {
		t.Fatalf("count=%d recorded=%q, want 1", count, recorded)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(data, []byte("\n")) || !json.Valid(bytes.TrimSpace(data)) {
		t.Fatal("refresh did not write newline-terminated JSONL")
	}
}

func TestRefreshLeavesFixtureOnExportFailure(t *testing.T) {
	for _, body := range []string{"exit 7\n", "printf '%s\\n' '{broken'\n"} {
		t.Run(strings.TrimSpace(body), func(t *testing.T) {
			fakeBD(t, body)
			output := filepath.Join(t.TempDir(), "sample.jsonl")
			for path, content := range map[string]string{output: "original\n", countPath(output): "99\n"} {
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := refresh(ctx, ".", output); err == nil {
				t.Fatal("failed export accepted")
			}
			for path, want := range map[string]string{output: "original\n", countPath(output): "99\n"} {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Errorf("failed refresh changed %s", path)
				}
			}
		})
	}
}

func TestRecordedFixtureLoadsEveryIssue(t *testing.T) {
	path := filepath.Join("..", "..", "tests", "testdata", "real", "b9s.jsonl")
	recorded, err := os.ReadFile(strings.TrimSuffix(path, ".jsonl") + ".count")
	if err != nil {
		t.Fatal(err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(recorded)))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	projected, actual, err := sanitizeExport(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, projected) {
		t.Error("fixture differs from its sanitized, sorted projection")
	}
	issues, err := loader.LoadIssuesFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if count < 1000 || actual != count || len(issues) != count {
		t.Fatalf("recorded=%d JSONL=%d loaded=%d, want the same large issue count", count, actual, len(issues))
	}
	parents := map[string]string{}
	epics, blocks := 0, 0
	for _, issue := range issues {
		if issue.IssueType == "epic" {
			epics++
		}
		for _, dep := range issue.Dependencies {
			if dep.Type == "parent-child" {
				parents[issue.ID] = dep.DependsOnID
			}
			if dep.Type == "blocks" {
				blocks++
			}
		}
	}
	deep := false
	for _, parent := range parents {
		if parents[parent] != "" {
			deep = true
		}
	}
	if epics == 0 || blocks == 0 || !deep {
		t.Fatalf("fixture lacks graph coverage: epics=%d blocks=%d deep=%t", epics, blocks, deep)
	}
	t.Logf("recorded=%d loaded=%d epics=%d parent-child=%d blocks=%d", count, len(issues), epics, len(parents), blocks)
}
