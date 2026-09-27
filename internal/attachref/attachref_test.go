package attachref

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/beadwork/pkg/model"
)

func validRef() Ref {
	return Ref{
		SHA256: strings.Repeat("a", 64),
		Size:   120832,
		Type:   "image/png",
		Name:   "screenshot.png",
	}
}

func TestFormatHumanLineMatchesADRExample(t *testing.T) {
	text, err := Format(validRef())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	lines := strings.Split(text, "\n")
	if len(lines) != 2 {
		t.Fatalf("Format produced %d lines, want 2: %q", len(lines), text)
	}
	wantHuman := "\U0001F4CE screenshot.png (image/png, 118 KB)"
	if lines[0] != wantHuman {
		t.Errorf("human line = %q, want %q", lines[0], wantHuman)
	}
	wantMachine := "beads-attachment/v1 sha256=" + strings.Repeat("a", 64) + " size=120832 type=image/png name=screenshot.png"
	if lines[1] != wantMachine {
		t.Errorf("machine line = %q, want %q", lines[1], wantMachine)
	}
}

func TestFormatDetachHumanLine(t *testing.T) {
	text, err := FormatDetach(validRef())
	if err != nil {
		t.Fatalf("FormatDetach: %v", err)
	}
	lines := strings.Split(text, "\n")
	if len(lines) != 2 {
		t.Fatalf("FormatDetach produced %d lines, want 2: %q", len(lines), text)
	}
	wantHuman := "\U0001F4CE detached screenshot.png"
	if lines[0] != wantHuman {
		t.Errorf("human line = %q, want %q", lines[0], wantHuman)
	}
	wantMachine := "beads-attachment-detach/v1 sha256=" + strings.Repeat("a", 64)
	if lines[1] != wantMachine {
		t.Errorf("machine line = %q, want %q", lines[1], wantMachine)
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	sha := strings.Repeat("b", 64)
	cases := []struct {
		name string
		ref  Ref
	}{
		{"plain", Ref{SHA256: sha, Size: 42, Type: "text/plain", Name: "notes.txt"}},
		{"spaces", Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "my screenshot.png"}},
		{"percent", Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "100% done.png"}},
		{"unicode", Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "\u00e9cran.png"}},
		{"quotes", Ref{SHA256: sha, Size: 1, Type: "image/png", Name: `he said "hi".txt`}},
		{"zero size", Ref{SHA256: sha, Size: 0, Type: "application/octet-stream", Name: "empty"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, err := Format(tc.ref)
			if err != nil {
				t.Fatalf("Format: %v", err)
			}
			refs, detaches := Parse(text)
			if len(detaches) != 0 {
				t.Fatalf("Parse returned detaches for an attach line: %v", detaches)
			}
			if len(refs) != 1 {
				t.Fatalf("Parse returned %d refs, want 1: %q", len(refs), text)
			}
			if refs[0] != tc.ref {
				t.Errorf("round trip: got %+v, want %+v", refs[0], tc.ref)
			}
		})
	}
}

func TestFormatRejectsInvalid(t *testing.T) {
	base := validRef()
	cases := []struct {
		name string
		ref  Ref
	}{
		{"short sha", func() Ref { r := base; r.SHA256 = "abc"; return r }()},
		{"uppercase sha", func() Ref { r := base; r.SHA256 = strings.ToUpper(r.SHA256); return r }()},
		{"non-hex sha", func() Ref { r := base; r.SHA256 = strings.Repeat("g", 64); return r }()},
		{"negative size", func() Ref { r := base; r.Size = -1; return r }()},
		{"empty type", func() Ref { r := base; r.Type = ""; return r }()},
		{"unparseable type", func() Ref { r := base; r.Type = "not-a-mime-type"; return r }()},
		{"type with space", func() Ref { r := base; r.Type = "text/plain; charset=utf-8"; return r }()},
		{"type with vertical tab", func() Ref { r := base; r.Type = "image/png\v"; return r }()},
		{"type with form feed", func() Ref { r := base; r.Type = "image/png\f"; return r }()},
		{"type with nbsp", func() Ref { r := base; r.Type = "image/png\u00a0"; return r }()},
		{"type with leading em space", func() Ref { r := base; r.Type = "\u2003image/png"; return r }()},
		{"type with nel", func() Ref { r := base; r.Type = "image/png\u0085"; return r }()},
		{"type with two slashes", func() Ref { r := base; r.Type = "image/png/extra"; return r }()},
		{"type with tspecial", func() Ref { r := base; r.Type = "image/png;q=1"; return r }()},
		{"empty name", func() Ref { r := base; r.Name = ""; return r }()},
		{"name with slash", func() Ref { r := base; r.Name = "a/b.png"; return r }()},
		{"name with backslash", func() Ref { r := base; r.Name = `a\b.png`; return r }()},
		{"name dot", func() Ref { r := base; r.Name = "."; return r }()},
		{"name dotdot", func() Ref { r := base; r.Name = ".."; return r }()},
		{"name with newline", func() Ref { r := base; r.Name = "a\nb.png"; return r }()},
		{"name with control char", func() Ref { r := base; r.Name = "a\x01b.png"; return r }()},
		{"name too long", func() Ref { r := base; r.Name = strings.Repeat("a", 256); return r }()},
		{"name with invalid UTF-8", func() Ref { r := base; r.Name = "a\xffb.png"; return r }()},
		{"name with LRM", func() Ref { r := base; r.Name = "a\u200eb.png"; return r }()},
		{"name with RLM", func() Ref { r := base; r.Name = "a\u200fb.png"; return r }()},
		{"name with Arabic letter mark", func() Ref { r := base; r.Name = "a\u061cb.png"; return r }()},
		{"name with LRO", func() Ref { r := base; r.Name = "a\u202db.png"; return r }()},
		{"name with RLO", func() Ref { r := base; r.Name = "a\u202eb.png"; return r }()},
		{"name with LRI", func() Ref { r := base; r.Name = "a\u2066b.png"; return r }()},
		{"name with PDI", func() Ref { r := base; r.Name = "a\u2069b.png"; return r }()},
		{"name with zero-width space", func() Ref { r := base; r.Name = "a\u200bb.png"; return r }()},
		{"name with zero-width non-joiner", func() Ref { r := base; r.Name = "a\u200cb.png"; return r }()},
		{"name with BOM", func() Ref { r := base; r.Name = "a" + "\xef\xbb\xbf" + "b.png"; return r }()},
		{"name starting with dash", func() Ref { r := base; r.Name = "-rf.png"; return r }()},
		{"name with line separator", func() Ref { r := base; r.Name = "a\u2028b.png"; return r }()},
		{"name with paragraph separator", func() Ref { r := base; r.Name = "a\u2029b.png"; return r }()},
		{"name with tag character", func() Ref { r := base; r.Name = "a\U000E0020b.png"; return r }()},
		{"name with hangul filler", func() Ref { r := base; r.Name = "a\u3164b.png"; return r }()},
		{"name with hangul choseong filler", func() Ref { r := base; r.Name = "a\u115fb.png"; return r }()},
		{"name with hangul jungseong filler", func() Ref { r := base; r.Name = "a\u1160b.png"; return r }()},
		{"name ending in space", func() Ref { r := base; r.Name = "screenshot.png "; return r }()},
		{"name ending in dot", func() Ref { r := base; r.Name = "screenshot.png."; return r }()},
		{"name only whitespace", func() Ref { r := base; r.Name = "   "; return r }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Format(tc.ref); err == nil {
				t.Errorf("Format(%+v) succeeded, want error", tc.ref)
			}
		})
	}
}

func TestFormatAllowsZeroWidthJoiner(t *testing.T) {
	// ZWJ is what joins emoji into a single glyph (e.g. family emoji sequences),
	// so a name built from one must not be treated the same as the other,
	// visually similar zero-width and bidi formatting characters that are rejected.
	r := validRef()
	r.Name = "a\u200db.png"
	text, err := Format(r)
	if err != nil {
		t.Fatalf("Format with a ZWJ in the name failed: %v", err)
	}
	refs, _ := Parse(text)
	if len(refs) != 1 || refs[0].Name != r.Name {
		t.Fatalf("round trip of a ZWJ name failed: got %+v", refs)
	}
}

func TestParseRejectsEveryRuleViolation(t *testing.T) {
	sha := strings.Repeat("c", 64)
	cases := []struct {
		name string
		line string
	}{
		{"short sha", "beads-attachment/v1 sha256=abc size=1 type=image/png name=a.png"},
		{"uppercase sha", "beads-attachment/v1 sha256=" + strings.ToUpper(sha) + " size=1 type=image/png name=a.png"},
		{"missing sha", "beads-attachment/v1 size=1 type=image/png name=a.png"},
		{"negative size", "beads-attachment/v1 sha256=" + sha + " size=-1 type=image/png name=a.png"},
		{"non-numeric size", "beads-attachment/v1 sha256=" + sha + " size=big type=image/png name=a.png"},
		{"missing size", "beads-attachment/v1 sha256=" + sha + " type=image/png name=a.png"},
		{"size with leading plus", "beads-attachment/v1 sha256=" + sha + " size=+5 type=image/png name=a.png"},
		{"size with leading zero", "beads-attachment/v1 sha256=" + sha + " size=0005 type=image/png name=a.png"},
		{"size negative zero", "beads-attachment/v1 sha256=" + sha + " size=-0 type=image/png name=a.png"},
		{"bad type", "beads-attachment/v1 sha256=" + sha + " size=1 type=nope name=a.png"},
		{"missing type", "beads-attachment/v1 sha256=" + sha + " size=1 name=a.png"},
		{"missing name", "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png"},
		{"name is dotdot", "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=.."},
		{"name has slash", "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a%2Fb.png"},
		{"duplicate key", "beads-attachment/v1 sha256=" + sha + " sha256=" + sha + " size=1 type=image/png name=a.png"},
		{"malformed field", "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png junk"},
		{"bad percent escape", "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=%zz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseAttachLine(tc.line); err == nil {
				t.Errorf("parseAttachLine(%q) succeeded, want error", tc.line)
			}
			refs, _ := Parse(tc.line)
			if len(refs) != 0 {
				t.Errorf("Parse(%q) returned refs, want none: %+v", tc.line, refs)
			}
		})
	}
}

func TestParseDetachRejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"short sha", "beads-attachment-detach/v1 sha256=abc"},
		{"missing sha", "beads-attachment-detach/v1 "},
		{"duplicate key", "beads-attachment-detach/v1 sha256=" + strings.Repeat("a", 64) + " sha256=" + strings.Repeat("b", 64)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseDetachLine(tc.line); err == nil {
				t.Errorf("parseDetachLine(%q) succeeded, want error", tc.line)
			}
			_, detaches := Parse(tc.line)
			if len(detaches) != 0 {
				t.Errorf("Parse(%q) returned detaches, want none: %v", tc.line, detaches)
			}
		})
	}
}

func TestParseIgnoresUnknownKeys(t *testing.T) {
	sha := strings.Repeat("d", 64)
	line := "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png future=stuff"
	refs, _ := Parse(line)
	if len(refs) != 1 {
		t.Fatalf("Parse returned %d refs, want 1", len(refs))
	}
	want := Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "a.png"}
	if refs[0] != want {
		t.Errorf("got %+v, want %+v", refs[0], want)
	}
}

func TestParseUnknownVersionIgnored(t *testing.T) {
	sha := strings.Repeat("e", 64)
	line := "beads-attachment/v2 sha256=" + sha + " size=1 type=image/png name=a.png"
	refs, detaches := Parse(line)
	if len(refs) != 0 || len(detaches) != 0 {
		t.Errorf("Parse of a v2 line returned %+v / %v, want nothing", refs, detaches)
	}
}

func TestParseTrailingCR(t *testing.T) {
	sha := strings.Repeat("f", 64)
	text := "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png\r\n"
	refs, _ := Parse(text)
	if len(refs) != 1 {
		t.Fatalf("Parse with CRLF returned %d refs, want 1", len(refs))
	}
}

func TestParseCommentWithSurroundingText(t *testing.T) {
	sha := strings.Repeat("1", 64)
	text := "\U0001F4CE a.png (image/png, 1 B)\nbeads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png\n"
	refs, _ := Parse(text)
	if len(refs) != 1 {
		t.Fatalf("Parse returned %d refs, want 1", len(refs))
	}
	if refs[0].SHA256 != sha {
		t.Errorf("got sha %q, want %q", refs[0].SHA256, sha)
	}
}

func TestStripMachineLines(t *testing.T) {
	sha := strings.Repeat("2", 64)
	human := "\U0001F4CE a.png (image/png, 1 B)"
	machine := "beads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png"
	text := human + "\n" + machine
	got := StripMachineLines(text)
	if got != human {
		t.Errorf("StripMachineLines(%q) = %q, want %q", text, got, human)
	}

	// An invalid machine-looking line is left alone: it is not a parsed reference.
	invalid := "beads-attachment/v1 sha256=nothex size=1 type=image/png name=a.png"
	textWithInvalid := human + "\n" + invalid
	if got := StripMachineLines(textWithInvalid); got != textWithInvalid {
		t.Errorf("StripMachineLines stripped an invalid line: got %q, want %q", got, textWithInvalid)
	}

	if got := StripMachineLines("just text"); got != "just text" {
		t.Errorf("StripMachineLines changed plain text: %q", got)
	}
}

func TestCollectFoldsAttachAndDetach(t *testing.T) {
	shaA := strings.Repeat("a", 64)
	shaB := strings.Repeat("b", 64)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	attachA, err := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a.png"})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	attachB, err := Format(Ref{SHA256: shaB, Size: 2, Type: "image/png", Name: "b.png"})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	detachA, err := FormatDetach(Ref{SHA256: shaA, Name: "a.png"})
	if err != nil {
		t.Fatalf("FormatDetach: %v", err)
	}
	reattachA, err := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a.png"})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}

	comments := []*model.Comment{
		{ID: "c3", IssueID: "i1", Author: "alice", Text: detachA, CreatedAt: t0.Add(3 * time.Hour)},
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attachA, CreatedAt: t0.Add(1 * time.Hour)},
		{ID: "c2", IssueID: "i1", Author: "bob", Text: attachB, CreatedAt: t0.Add(2 * time.Hour)},
		{ID: "c4", IssueID: "i1", Author: "carol", Text: reattachA, CreatedAt: t0.Add(4 * time.Hour)},
	}

	got := Collect(comments)
	if len(got) != 2 {
		t.Fatalf("Collect returned %d attachments, want 2: %+v", len(got), got)
	}
	// A was detached and re-attached, so it moves to the end of the list: a
	// re-attach is a new attachment event, not a restore of the old position.
	if got[0].SHA256 != shaB {
		t.Errorf("got[0].SHA256 = %q, want %q (B was never detached)", got[0].SHA256, shaB)
	}
	if got[1].SHA256 != shaA {
		t.Errorf("got[1].SHA256 = %q, want %q (re-attach appends at the end)", got[1].SHA256, shaA)
	}
	if got[1].CommentID != "c4" || got[1].AddedBy != "carol" {
		t.Errorf("got[1] = %+v, want the re-attach comment's author/id", got[1])
	}
	if !got[1].AddedAt.Equal(t0.Add(4 * time.Hour)) {
		t.Errorf("got[1].AddedAt = %v, want %v", got[1].AddedAt, t0.Add(4*time.Hour))
	}
}

func TestCollectSecondAttachWhileLiveReplacesInPlace(t *testing.T) {
	shaA := strings.Repeat("a", 64)
	shaB := strings.Repeat("b", 64)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	attachA1, _ := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a.png"})
	attachB, _ := Format(Ref{SHA256: shaB, Size: 2, Type: "image/png", Name: "b.png"})
	attachA2, _ := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a-renamed.png"})

	comments := []*model.Comment{
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attachA1, CreatedAt: t0},
		{ID: "c2", IssueID: "i1", Author: "bob", Text: attachB, CreatedAt: t0.Add(time.Hour)},
		{ID: "c3", IssueID: "i1", Author: "carol", Text: attachA2, CreatedAt: t0.Add(2 * time.Hour)},
	}

	got := Collect(comments)
	if len(got) != 2 {
		t.Fatalf("Collect returned %d attachments, want 2: %+v", len(got), got)
	}
	// A was never detached between c1 and c3, so the second attach replaces
	// it in place: A keeps its original (first) position ahead of B.
	if got[0].SHA256 != shaA || got[0].Name != "a-renamed.png" || got[0].CommentID != "c3" {
		t.Errorf("got[0] = %+v, want A updated in place by c3", got[0])
	}
	if got[1].SHA256 != shaB {
		t.Errorf("got[1].SHA256 = %q, want %q", got[1].SHA256, shaB)
	}
}

func TestCollectDetachThenAttachSameCommentIsLive(t *testing.T) {
	sha := strings.Repeat("d", 64)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	attach, _ := Format(Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "a.png"})
	detach, _ := FormatDetach(Ref{SHA256: sha, Name: "a.png"})

	comments := []*model.Comment{
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attach, CreatedAt: t0},
		// One comment carries both lines, detach first: line order within
		// the comment must be honoured, not "all detaches after all attaches".
		{ID: "c2", IssueID: "i1", Author: "bob", Text: detach + "\n" + strings.SplitN(attach, "\n", 2)[1], CreatedAt: t0.Add(time.Hour)},
	}

	got := Collect(comments)
	if len(got) != 1 || got[0].SHA256 != sha {
		t.Fatalf("Collect returned %+v, want the attachment live", got)
	}
}

func TestCollectSkipsNilComments(t *testing.T) {
	sha := strings.Repeat("e", 64)
	attach, _ := Format(Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "a.png"})
	comments := []*model.Comment{
		nil,
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attach, CreatedAt: time.Now()},
		nil,
	}
	got := Collect(comments)
	if len(got) != 1 || got[0].SHA256 != sha {
		t.Fatalf("Collect with nil comments = %+v, want one attachment", got)
	}
}

func TestCollectDetachRemovesAttachment(t *testing.T) {
	sha := strings.Repeat("3", 64)
	t0 := time.Now()
	attach, _ := Format(Ref{SHA256: sha, Size: 1, Type: "image/png", Name: "a.png"})
	detach, _ := FormatDetach(Ref{SHA256: sha, Name: "a.png"})
	comments := []*model.Comment{
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attach, CreatedAt: t0},
		{ID: "c2", IssueID: "i1", Author: "alice", Text: detach, CreatedAt: t0.Add(time.Second)},
	}
	got := Collect(comments)
	if len(got) != 0 {
		t.Fatalf("Collect returned %+v, want none (detached)", got)
	}
}

func TestCollectOrdersByCreatedAtThenID(t *testing.T) {
	shaA := strings.Repeat("4", 64)
	shaB := strings.Repeat("5", 64)
	t0 := time.Now()
	attachA, _ := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a.png"})
	attachB, _ := Format(Ref{SHA256: shaB, Size: 1, Type: "image/png", Name: "b.png"})
	// Same timestamp: comment ID is the tie-break, so "c1" (attachB) is folded before "c2" (attachA).
	comments := []*model.Comment{
		{ID: "c2", IssueID: "i1", Author: "alice", Text: attachA, CreatedAt: t0},
		{ID: "c1", IssueID: "i1", Author: "alice", Text: attachB, CreatedAt: t0},
	}
	got := Collect(comments)
	if len(got) != 2 || got[0].SHA256 != shaB || got[1].SHA256 != shaA {
		t.Fatalf("Collect did not tie-break by comment ID: %+v", got)
	}
}

func TestCollectOrdersNumericIDsNumerically(t *testing.T) {
	sha9 := strings.Repeat("9", 64)
	sha10 := strings.Repeat("8", 64)
	sha11 := strings.Repeat("7", 64)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	attach9, _ := Format(Ref{SHA256: sha9, Size: 1, Type: "image/png", Name: "9.png"})
	attach10, _ := Format(Ref{SHA256: sha10, Size: 1, Type: "image/png", Name: "10.png"})
	attach11, _ := Format(Ref{SHA256: sha11, Size: 1, Type: "image/png", Name: "11.png"})

	// Same second for all three: a lexical id comparison would order "10" and
	// "11" before "9" ('1' < '9'), which is wrong once ids are legacy integers.
	comments := []*model.Comment{
		{ID: "10", IssueID: "i1", Author: "alice", Text: attach10, CreatedAt: t0},
		{ID: "9", IssueID: "i1", Author: "alice", Text: attach9, CreatedAt: t0},
		{ID: "11", IssueID: "i1", Author: "alice", Text: attach11, CreatedAt: t0},
	}

	got := Collect(comments)
	if len(got) != 3 {
		t.Fatalf("Collect returned %d attachments, want 3: %+v", len(got), got)
	}
	if got[0].SHA256 != sha9 || got[1].SHA256 != sha10 || got[2].SHA256 != sha11 {
		t.Fatalf("Collect did not order numeric ids numerically: %+v", got)
	}
}

// TestCollectCommentIDOrderIsTransitive guards a 3-cycle in lessCommentID:
// "9" < "10" numerically, but "10" < "1a2b..." and "1a2b..." < "9" lexically.
// A cycle makes sort.SliceStable's result depend on the input order, so every
// permutation of the same three same-second comments must fold to the same
// attachments.
func TestCollectCommentIDOrderIsTransitive(t *testing.T) {
	shaA := strings.Repeat("a", 64)
	shaB := strings.Repeat("b", 64)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	attachA, _ := Format(Ref{SHA256: shaA, Size: 1, Type: "image/png", Name: "a.png"})
	detachA, _ := FormatDetach(Ref{SHA256: shaA, Name: "a.png"})
	attachB, _ := Format(Ref{SHA256: shaB, Size: 1, Type: "image/png", Name: "b.png"})

	base := []*model.Comment{
		{ID: "9", IssueID: "i1", Author: "alice", Text: attachA, CreatedAt: t0},
		{ID: "10", IssueID: "i1", Author: "alice", Text: detachA, CreatedAt: t0},
		{ID: "1a2b3c4d", IssueID: "i1", Author: "alice", Text: attachB, CreatedAt: t0},
	}

	permutations := [][3]int{
		{0, 1, 2}, {0, 2, 1}, {1, 0, 2},
		{1, 2, 0}, {2, 0, 1}, {2, 1, 0},
	}

	var want []Attachment
	for i, perm := range permutations {
		comments := []*model.Comment{base[perm[0]], base[perm[1]], base[perm[2]]}
		got := Collect(comments)
		if i == 0 {
			want = got
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("permutation %v gave %+v, want the same result as permutation %v: %+v", perm, got, permutations[0], want)
		}
	}
}

// FuzzFormat covers the direction FuzzParse does not: for any Ref that Format
// accepts (however it was constructed), the text it produces must read back
// through Parse as that exact Ref, not merely as some valid Ref.
func FuzzFormat(f *testing.F) {
	sha := strings.Repeat("a", 64)
	f.Add(sha, int64(120832), "image/png", "screenshot.png")
	f.Add(sha, int64(0), "text/plain", "notes.txt")
	f.Add(sha, int64(1), "image/png\u00a0", "a.png")
	f.Add(sha, int64(1), "image/png", "a\u200eb.png")
	f.Add(sha, int64(1), "image/png", "-rf")
	f.Fuzz(func(t *testing.T, sha string, size int64, typ string, name string) {
		r := Ref{SHA256: sha, Size: size, Type: typ, Name: name}
		text, err := Format(r)
		if err != nil {
			return
		}
		refs, detaches := Parse(text)
		if len(detaches) != 0 {
			t.Fatalf("Format(%+v) = %q, which Parse read as a detach: %v", r, text, detaches)
		}
		if len(refs) != 1 || refs[0] != r {
			t.Fatalf("Format(%+v) = %q, round trip returned %+v", r, text, refs)
		}
	})
}

func FuzzParse(f *testing.F) {
	sha := strings.Repeat("a", 64)
	seeds := []string{
		"",
		"beads-attachment/v1 sha256=" + sha + " size=100 type=image/png name=a.png",
		"beads-attachment-detach/v1 sha256=" + sha,
		"beads-attachment/v1 sha256=short size=100 type=image/png name=a.png",
		"beads-attachment/v1 sha256=" + sha + " size=-1 type=image/png name=a.png",
		"beads-attachment/v1 sha256=" + sha + " size=100 type=notmime name=a.png",
		"beads-attachment/v1 sha256=" + sha + " size=100 type=image/png name=..",
		"beads-attachment/v1 sha256=" + sha + " size=100 type=image/png name=%zz",
		"random text with no marker at all",
		"beads-attachment/v2 sha256=" + sha + " size=100 type=image/png name=a.png",
		"\U0001F4CE a.png (image/png, 1 B)\r\nbeads-attachment/v1 sha256=" + sha + " size=1 type=image/png name=a.png\r\n",
		"beads-attachment/v1 ",
		"beads-attachment-detach/v1 ",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, text string) {
		refs, detaches := Parse(text)
		for _, r := range refs {
			if err := validate(r); err != nil {
				t.Fatalf("Parse returned an invalid ref %+v: %v", r, err)
			}
			formatted, err := Format(r)
			if err != nil {
				t.Fatalf("Format(%+v) failed: %v", r, err)
			}
			reparsed, reparsedDetaches := Parse(formatted)
			if len(reparsedDetaches) != 0 {
				t.Fatalf("re-parsing %q produced detaches: %v", formatted, reparsedDetaches)
			}
			if len(reparsed) != 1 || reparsed[0] != r {
				t.Fatalf("round trip failed for %+v: reparsed to %+v", r, reparsed)
			}
		}
		for _, sha := range detaches {
			if err := validateSHA256(sha); err != nil {
				t.Fatalf("Parse returned an invalid detach hash %q: %v", sha, err)
			}
		}
	})
}
