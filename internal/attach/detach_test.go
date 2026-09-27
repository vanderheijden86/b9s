package attach

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// fakeComments is a CommentLoader backed by an in-memory map, standing in
// for a data source's issue comments.
type fakeComments map[string][]*model.Comment

func (f fakeComments) Comments(issueID string) ([]*model.Comment, error) {
	return f[issueID], nil
}

func attachComment(t *testing.T, id, author string, at time.Time, r attachref.Ref) *model.Comment {
	t.Helper()
	text, err := attachref.Format(r)
	if err != nil {
		t.Fatal(err)
	}
	return &model.Comment{ID: id, Author: author, Text: text, CreatedAt: at}
}

const testSHA = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

func TestDetach_AppendsDetachCommentForALiveAttachment(t *testing.T) {
	ref := attachref.Ref{SHA256: testSHA, Size: 3, Type: "text/plain", Name: "notes.txt"}
	comments := fakeComments{
		"bd-1": {attachComment(t, "1", "alice", time.Unix(100, 0), ref)},
	}
	bd := &fakeBd{}

	err := Detach(context.Background(), bd, comments, "bd-1", testSHA)

	if err != nil {
		t.Fatalf("Detach: %v", err)
	}
	if len(bd.calls) != 1 {
		t.Fatalf("bd calls = %d, want 1", len(bd.calls))
	}
	call := bd.calls[0]
	if call[0] != "comments" || call[1] != "add" || call[2] != "bd-1" {
		t.Fatalf("bd call = %v", call)
	}
	_, detaches := attachref.Parse(call[3])
	if len(detaches) != 1 || detaches[0] != testSHA {
		t.Errorf("Parse(%q) detaches = %v", call[3], detaches)
	}
}

func TestDetach_BdFailureErrorIncludesBdOutput(t *testing.T) {
	ref := attachref.Ref{SHA256: testSHA, Size: 3, Type: "text/plain", Name: "notes.txt"}
	comments := fakeComments{
		"bd-1": {attachComment(t, "1", "alice", time.Unix(100, 0), ref)},
	}
	bd := &fakeBd{err: errCommandFailed, output: "bd: issue bd-1 not found"}

	err := Detach(context.Background(), bd, comments, "bd-1", testSHA)

	if err == nil {
		t.Fatal("Detach: err = nil, want the bd failure reported")
	}
	if !strings.Contains(err.Error(), "bd: issue bd-1 not found") {
		t.Errorf("error = %v, want it to include bd's output", err)
	}
}

func TestDetach_RejectsMalformedHash(t *testing.T) {
	err := Detach(context.Background(), &fakeBd{}, fakeComments{}, "bd-1", "not-a-hash")

	if err == nil {
		t.Fatal("Detach: err = nil, want a malformed-hash error")
	}
}

func TestDetach_RejectsHashNotCurrentlyAttached(t *testing.T) {
	err := Detach(context.Background(), &fakeBd{}, fakeComments{}, "bd-1", testSHA)

	if err == nil {
		t.Fatal("Detach: err = nil, want an error for a hash never attached")
	}
}

// TestDetach_SurfacesTheCommentLoaderError guards the fail-closed contract
// bd-t8j5.18 depends on: a comments load failure must be reported as an
// error, never read as "this hash was never attached" (TestDetach_RejectsHashNotCurrentlyAttached's
// empty case), because that message is indistinguishable from success for a
// caller checking only the exit code.
func TestDetach_SurfacesTheCommentLoaderError(t *testing.T) {
	err := Detach(context.Background(), &fakeBd{}, erroringCommentLoader{}, "bd-1", testSHA)

	if err == nil {
		t.Fatal("Detach: err = nil, want the loader's error surfaced")
	}
	if strings.Contains(err.Error(), "not currently attached") {
		t.Errorf("error = %v, must not read as \"not currently attached\" when the load itself failed", err)
	}
}

func TestDetach_RejectsAnAlreadyDetachedHash(t *testing.T) {
	ref := attachref.Ref{SHA256: testSHA, Size: 3, Type: "text/plain", Name: "notes.txt"}
	detachText, err := attachref.FormatDetach(ref)
	if err != nil {
		t.Fatal(err)
	}
	comments := fakeComments{
		"bd-1": {
			attachComment(t, "1", "alice", time.Unix(100, 0), ref),
			{ID: "2", Author: "alice", Text: detachText, CreatedAt: time.Unix(200, 0)},
		},
	}

	err = Detach(context.Background(), &fakeBd{}, comments, "bd-1", testSHA)

	if err == nil {
		t.Fatal("Detach: err = nil, want an error: already detached")
	}
}
