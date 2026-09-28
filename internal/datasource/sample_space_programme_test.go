package datasource

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/pkg/model"
)

// The space programme is the larger of the two sample projects, the one the
// screens are shown with, so beyond opening without a Dolt server it must keep
// its size, its releases and its blocked work, and carry no identity of
// whoever regenerated it.
func TestSampleSpaceProgrammeOpensAsJSONL(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "space-programme")
	opened, failure := OpenProject(OpenTarget{Name: "space-programme", Dir: dir})
	if failure != nil {
		t.Fatalf("OpenProject: %v", failure)
	}
	if opened.Source.Type != SourceTypeJSONLLocal {
		t.Fatalf("source = %s, want %s", opened.Source.Type, SourceTypeJSONLLocal)
	}
	if got, least := len(opened.Issues), 45; got < least {
		t.Errorf("the space programme has %d issues, want at least %d", got, least)
	}

	count := map[string]int{}
	for _, is := range opened.Issues {
		count[string(is.Status)]++
		count[string(is.IssueType)]++
		for _, d := range is.Dependencies {
			if d.Type == model.DepBlocks {
				count["blocks"]++
			}
		}
		if len(is.Comments) > 0 {
			count["commented"]++
		}
		if len(is.Labels) > 0 {
			count["labelled"]++
		}
		if !strings.HasPrefix(is.ID, "mars-") {
			t.Errorf("%s does not carry the mars prefix", is.ID)
		}
		for _, who := range []string{is.Owner, is.CreatedBy, is.Assignee} {
			if who != "" && !strings.HasSuffix(who, "@example.com") && strings.Contains(who, "@") {
				t.Errorf("%s names %q; the sample uses only made-up identities", is.ID, who)
			}
		}
	}
	kinds := []string{
		string(model.TypeEpic), "milestone", string(model.TypeFeature), string(model.TypeBug), string(model.TypeChore),
		"open", "in_progress", string(model.StatusBlocked), "deferred", "closed",
		"blocks", "commented", "labelled",
	}
	for _, kind := range kinds {
		if count[kind] == 0 {
			t.Errorf("the space programme has no %s issue; regenerate it with examples/space-programme/generate.sh", kind)
		}
	}
}
