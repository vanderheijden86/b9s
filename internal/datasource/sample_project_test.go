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

	count := map[string]int{}
	for _, is := range opened.Issues {
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
	for _, kind := range []string{"epic", "open", "in_progress", "blocked", "deferred", "closed", "blocks", "commented"} {
		if count[kind] == 0 {
			t.Errorf("the sample has no %s issue; regenerate it with examples/sample-project/generate.sh", kind)
		}
	}
}
