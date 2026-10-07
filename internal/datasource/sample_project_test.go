package datasource

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// The sample project is what the README tells a newcomer to open, so it must
// open without a Dolt server, still show every kind of work the screens
// draw, and carry no identity of whoever regenerated it.
func TestSampleProjectOpensAsJSONL(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "sample-project")
	opened, failure := OpenProject(OpenTarget{Name: "sample", Dir: dir})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if opened.Source.Type != SourceTypeJSONLLocal {
		t.Fatalf("source = %s, want %s", opened.Source.Type, SourceTypeJSONLLocal)
	}

	byID := map[string]model.Issue{}
	for _, is := range opened.Issues {
		byID[is.ID] = is
	}
	parentOf := func(is model.Issue) (model.Issue, bool) {
		for _, d := range is.Dependencies {
			if d.Type == model.DepParentChild {
				p, ok := byID[d.DependsOnID]
				return p, ok
			}
		}
		return model.Issue{}, false
	}

	count := map[string]int{}
	for _, is := range opened.Issues {
		if is.IssueType == "milestone" {
			count["milestone"]++
		}
		// The tree view is the first screen a visitor sees; it must show an
		// epic holding features that hold tasks, three levels deep.
		if f, ok := parentOf(is); ok && f.IssueType == model.TypeFeature {
			if e, ok := parentOf(f); ok && e.IssueType == model.TypeEpic {
				count["epic>feature>task"]++
			}
		}
		count[string(is.Status)]++
		if is.IssueType == model.TypeEpic {
			count["epic"]++
		}
		for _, d := range is.Dependencies {
			if d.Type == model.DepBlocks {
				count["blocks"]++
			}
		}
		if len(is.Comments) > 0 {
			count["commented"]++
		}
		for _, who := range []string{is.Owner, is.CreatedBy, is.Assignee} {
			if who != "" && !strings.HasSuffix(who, "@example.com") && strings.Contains(who, "@") {
				t.Errorf("%s names %q; the sample uses only made-up identities", is.ID, who)
			}
		}
	}
	for _, kind := range []string{"epic", "open", "in_progress", "blocked", "deferred", "closed", "blocks", "commented", "milestone", "epic>feature>task"} {
		if count[kind] == 0 {
			t.Errorf("the sample has no %s issue; regenerate it with examples/sample-project/generate.sh", kind)
		}
	}
}
