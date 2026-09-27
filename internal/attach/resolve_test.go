package attach

import (
	"strings"
	"testing"

	"github.com/vanderheijden86/b9s/internal/attachref"
)

func attachment(sha256, name string) attachref.Attachment {
	return attachref.Attachment{Ref: attachref.Ref{SHA256: sha256, Name: name, Size: 1, Type: "text/plain"}}
}

func TestResolve_MatchesByFullSHA256(t *testing.T) {
	shaA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	attachments := []attachref.Attachment{attachment(shaA, "a.txt"), attachment(shaB, "b.txt")}

	got, err := Resolve(attachments, shaB)

	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SHA256 != shaB {
		t.Errorf("Resolve SHA256 = %s, want %s", got.SHA256, shaB)
	}
}

func TestResolve_MatchesByUniqueHashPrefix(t *testing.T) {
	shaA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	attachments := []attachref.Attachment{attachment(shaA, "a.txt"), attachment(shaB, "b.txt")}

	got, err := Resolve(attachments, "bbbbbbbb")

	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SHA256 != shaB {
		t.Errorf("Resolve SHA256 = %s, want %s", got.SHA256, shaB)
	}
}

func TestResolve_MatchesByExactName(t *testing.T) {
	shaA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	attachments := []attachref.Attachment{attachment(shaA, "notes.txt")}

	got, err := Resolve(attachments, "notes.txt")

	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.SHA256 != shaA {
		t.Errorf("Resolve SHA256 = %s, want %s", got.SHA256, shaA)
	}
}

func TestResolve_AmbiguousHashPrefixListsSortedCandidates(t *testing.T) {
	shaA := "aabbccddaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB := "aabbccddbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	attachments := []attachref.Attachment{attachment(shaB, "b.txt"), attachment(shaA, "a.txt")}

	_, err := Resolve(attachments, "aabbccdd")

	if err == nil {
		t.Fatal("Resolve: err = nil, want an ambiguous-prefix error")
	}
	if !strings.Contains(err.Error(), shaA) || !strings.Contains(err.Error(), shaB) {
		t.Errorf("Resolve error = %v, want both hashes listed", err)
	}
	if strings.Index(err.Error(), shaA) > strings.Index(err.Error(), shaB) {
		t.Errorf("Resolve error = %v, want hashes sorted", err)
	}
}

func TestResolve_AmbiguousNameAcrossTwoAttachments(t *testing.T) {
	shaA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	attachments := []attachref.Attachment{attachment(shaA, "notes.txt"), attachment(shaB, "notes.txt")}

	_, err := Resolve(attachments, "notes.txt")

	if err == nil {
		t.Fatal("Resolve: err = nil, want an ambiguous-name error")
	}
}

func TestResolve_NoMatch(t *testing.T) {
	_, err := Resolve(nil, "nothing")

	if err == nil {
		t.Fatal("Resolve: err = nil, want a no-match error")
	}
}

func TestResolve_ShortHexLikeQueryIsNameOnly(t *testing.T) {
	// "abc123" is under the 8-character prefix floor, so it must not match
	// any hash by prefix; it can still match a literal file named "abc123".
	shaA := "abc123aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	attachments := []attachref.Attachment{attachment(shaA, "unrelated.txt")}

	_, err := Resolve(attachments, "abc123")

	if err == nil {
		t.Fatal("Resolve: err = nil, want no match: query is shorter than the hash-prefix floor")
	}
}
