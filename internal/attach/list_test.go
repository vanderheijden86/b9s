package attach

import (
	"errors"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

func TestList_ReturnsEmptyForAnIssueWithNoComments(t *testing.T) {
	got, err := List(fakeComments{}, "bd-1")

	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %v, want empty", got)
	}
}

func TestList_FoldsAttachAndDetachComments(t *testing.T) {
	ref := attachref.Ref{SHA256: testSHA, Size: 3, Type: "text/plain", Name: "notes.txt"}
	comments := fakeComments{
		"bd-1": {attachComment(t, "1", "alice", time.Unix(100, 0), ref)},
	}

	got, err := List(comments, "bd-1")

	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].SHA256 != testSHA {
		t.Fatalf("List = %+v", got)
	}
}

type erroringCommentLoader struct{}

func (erroringCommentLoader) Comments(string) ([]*model.Comment, error) {
	return nil, errors.New("boom")
}

func TestList_SurfacesTheCommentLoaderError(t *testing.T) {
	_, err := List(erroringCommentLoader{}, "bd-1")

	if err == nil {
		t.Fatal("List: err = nil, want the loader's error surfaced")
	}
}
