// Package attachref reads and writes the versioned attachment reference
// lines that b9s stores in ordinary Beads comments (ADR 0024). A reference
// never touches the Beads schema: it is text, so any bd version and any b9s
// data source can carry it, and Dolt merges never conflict on it.
package attachref

import (
	"fmt"
	"mime"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vanderheijden86/b9s/pkg/debug"
	"github.com/vanderheijden86/b9s/pkg/model"
)

const (
	attachMarker = "beads-attachment/v1 "
	detachMarker = "beads-attachment-detach/v1 "

	maxNameBytes = 255
)

// Ref is one attachment: the content hash that names its blob in the store,
// plus the metadata carried alongside it in the comment.
type Ref struct {
	SHA256 string
	Size   int64
	Type   string
	Name   string
}

// Attachment is a Ref folded from the comment history of an issue, together
// with who attached it, when, and through which comment.
type Attachment struct {
	Ref
	CommentID string
	AddedBy   string
	AddedAt   time.Time
}

// Format renders r as a reference comment: a human line for stock `bd show`,
// then the machine line that Parse reads back. It validates r with the same
// rules Parse applies to a machine line, so a value Format accepts always
// round-trips through Parse unchanged.
func Format(r Ref) (string, error) {
	if err := validate(r); err != nil {
		return "", fmt.Errorf("attachref: %w", err)
	}
	human := fmt.Sprintf("\U0001F4CE %s (%s, %s)", r.Name, r.Type, humanSize(r.Size))
	machine := fmt.Sprintf("%ssha256=%s size=%d type=%s name=%s",
		attachMarker, r.SHA256, r.Size, r.Type, url.PathEscape(r.Name))
	return human + "\n" + machine, nil
}

// FormatDetach renders a detach comment for the attachment identified by
// r.SHA256. Only the hash reaches the machine line; Name is used for the
// human line only, so it is validated the same way a name is on attach.
func FormatDetach(r Ref) (string, error) {
	if err := validateSHA256(r.SHA256); err != nil {
		return "", fmt.Errorf("attachref: %w", err)
	}
	if err := validateName(r.Name); err != nil {
		return "", fmt.Errorf("attachref: %w", err)
	}
	human := fmt.Sprintf("\U0001F4CE detached %s", r.Name)
	machine := fmt.Sprintf("%ssha256=%s", detachMarker, r.SHA256)
	return human + "\n" + machine, nil
}

// Parse scans text line by line for machine lines and returns the attach
// references and detach hashes found, in the order their lines appear. A
// line that starts with a marker but breaks one of its rules, including a
// field token with no "=" (which invalidates the whole line), is skipped
// rather than returned, and never causes the rest of the text to be
// rejected. An unrecognised version (not "/v1") never matches a marker, so
// it is skipped the same way.
func Parse(text string) (refs []Ref, detaches []string) {
	for _, e := range parseLines(text) {
		if e.isAttach {
			refs = append(refs, e.ref)
		} else {
			detaches = append(detaches, e.sha)
		}
	}
	return refs, detaches
}

// lineEvent is one machine line, in the order it appeared in a comment's
// text. Collect needs this order preserved (a detach and a re-attach can
// both land in the same comment), which is why parseLines exists separately
// from Parse's two flat, un-interleaved slices.
type lineEvent struct {
	isAttach bool
	ref      Ref    // set when isAttach
	sha      string // set when !isAttach
}

func parseLines(text string) []lineEvent {
	var events []lineEvent
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, attachMarker):
			r, err := parseAttachLine(line)
			if err != nil {
				debug.Log("attachref: ignoring invalid attach line %q: %v", line, err)
				continue
			}
			events = append(events, lineEvent{isAttach: true, ref: r})
		case strings.HasPrefix(line, detachMarker):
			sha, err := parseDetachLine(line)
			if err != nil {
				debug.Log("attachref: ignoring invalid detach line %q: %v", line, err)
				continue
			}
			events = append(events, lineEvent{isAttach: false, sha: sha})
		}
	}
	return events
}

// StripMachineLines removes every valid machine line from text, leaving the
// human line(s) a UI should show. A line that merely starts with a marker
// but fails validation is not a reference, so it is left in place rather
// than silently dropped.
func StripMachineLines(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(trimmed, attachMarker) {
			if _, err := parseAttachLine(trimmed); err == nil {
				continue
			}
		}
		if strings.HasPrefix(trimmed, detachMarker) {
			if _, err := parseDetachLine(trimmed); err == nil {
				continue
			}
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// Collect folds the attach and detach references of comments into the
// attachment list of the issue they belong to, in created_at order with the
// comment ID as tie-break. A comment ID that parses as an integer on both
// sides of the comparison is compared numerically, since legacy Beads IDs
// are decimal and "9" must sort before "10". A tie is broken deterministically
// this way, but the ID order is not necessarily the real-world order of two
// writes landing in the same second, so it is a convention, not a causal
// order. Within one comment, lines are folded in the order they appear.
//
// An attach of a hash that is not currently in the list appends it to the
// end; an attach of a hash that is already live replaces its entry in place,
// keeping its existing position. A detach removes the hash from the list, so
// a later re-attach appends it again at the end rather than restoring its
// old position. A nil comment is skipped.
func Collect(comments []*model.Comment) []Attachment {
	sorted := make([]*model.Comment, 0, len(comments))
	for _, c := range comments {
		if c != nil {
			sorted = append(sorted, c)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return lessCommentID(sorted[i].ID, sorted[j].ID)
	})

	live := make(map[string]Attachment)
	var order []string
	for _, c := range sorted {
		for _, e := range parseLines(c.Text) {
			if e.isAttach {
				if _, exists := live[e.ref.SHA256]; !exists {
					order = append(order, e.ref.SHA256)
				}
				live[e.ref.SHA256] = Attachment{
					Ref:       e.ref,
					CommentID: c.ID,
					AddedBy:   c.Author,
					AddedAt:   c.CreatedAt,
				}
				continue
			}
			if _, exists := live[e.sha]; exists {
				delete(live, e.sha)
				order = removeString(order, e.sha)
			}
		}
	}

	result := make([]Attachment, 0, len(order))
	for _, sha := range order {
		result = append(result, live[sha])
	}
	return result
}

// lessCommentID orders two comment IDs numerically when both parse as
// integers (the common case for legacy Beads IDs), lexically when neither
// does (bd v0.63 UUID IDs), and with the numeric one first when only one
// does. That last case matters for transitivity: comparing "10" and "1a2b"
// lexically would put "10" first ('0' < 'a'), while "1a2b" and "9" lexically
// put "1a2b" first ('1' < '9'), so together with "9" < "10" numerically the
// three would form a cycle. Numeric-first for a mixed pair breaks the cycle
// by giving every legacy integer id a lower rank than every non-numeric one.
func lessCommentID(a, b string) bool {
	ai, aErr := strconv.ParseInt(a, 10, 64)
	bi, bErr := strconv.ParseInt(b, 10, 64)
	switch {
	case aErr == nil && bErr == nil:
		return ai < bi
	case aErr == nil:
		return true
	case bErr == nil:
		return false
	default:
		return a < b
	}
}

// removeString returns s with the first occurrence of v removed, preserving
// the order of the rest.
func removeString(s []string, v string) []string {
	for i, x := range s {
		if x == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func validate(r Ref) error {
	if err := validateSHA256(r.SHA256); err != nil {
		return err
	}
	if err := validateSize(r.Size); err != nil {
		return err
	}
	if err := validateType(r.Type); err != nil {
		return err
	}
	return validateName(r.Name)
}

func validateSHA256(sha string) error {
	if len(sha) != 64 {
		return fmt.Errorf("sha256 must be 64 hex characters, got %d in %q", len(sha), sha)
	}
	for _, r := range sha {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return fmt.Errorf("sha256 must be lowercase hex: %q", sha)
		}
	}
	return nil
}

func validateSize(size int64) error {
	if size < 0 {
		return fmt.Errorf("size must be >= 0, got %d", size)
	}
	return nil
}

// parseSize reads the size field as plain decimal digits: no sign (so
// neither "+5" nor "-0"), and no leading zero unless the whole value is "0".
// strconv.ParseInt alone accepts both, which would let two different machine
// lines ("size=5" and "size=+5") describe the same Ref, breaking the
// round-trip guarantee that Format's canonical form is the only spelling
// Parse accepts.
func parseSize(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("size must not be empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("size must contain only decimal digits: %q", s)
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("size must not have a leading zero: %q", s)
	}
	size, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q is out of range: %w", s, err)
	}
	return size, nil
}

// mimeTspecials are the RFC 2045 "tspecials": token characters reserved as
// MIME syntax (parameter separators, quoting, the type/subtype separator
// itself) and so excluded from the token charset.
const mimeTspecials = `()<>@,;:\"/[]?=`

// isMIMETokenChar reports whether r is legal in an RFC 2045 token: printable
// US-ASCII, excluding space, control characters and the tspecials. This is
// intentionally an ASCII-only charset, so it rejects every non-ASCII space,
// separator or format character (NBSP, EM SPACE, NEL, ...) in one check
// rather than naming each one.
func isMIMETokenChar(r rune) bool {
	if r < 0x21 || r > 0x7e {
		return false
	}
	return !strings.ContainsRune(mimeTspecials, r)
}

// validateType requires a bare "type/subtype" of RFC 2045 tokens, stricter
// than mime.ParseMediaType alone: that parser also accepts a bare token with
// no slash (not a MIME type) and a type with trailing "; param=value" pairs,
// whose spaces would corrupt the fields that follow it on the space-delimited,
// unescaped machine line.
func validateType(t string) error {
	slash := strings.IndexByte(t, '/')
	if slash < 0 {
		return fmt.Errorf("type must be a MIME type in type/subtype form: %q", t)
	}
	typePart, subtypePart := t[:slash], t[slash+1:]
	if typePart == "" || subtypePart == "" || strings.ContainsRune(subtypePart, '/') {
		return fmt.Errorf("type must contain exactly one '/': %q", t)
	}
	for _, part := range [2]string{typePart, subtypePart} {
		for _, r := range part {
			if !isMIMETokenChar(r) {
				return fmt.Errorf("type contains a character not allowed in a MIME token: %q", t)
			}
		}
	}
	if _, _, err := mime.ParseMediaType(t); err != nil {
		return fmt.Errorf("type %q is not a valid MIME type: %w", t, err)
	}
	return nil
}

// isDisallowedNameFormatChar reports whether r is a bidi- or width-control
// character that lets a name display differently from how it reads: the
// bidi override/isolate/mark family (which can make "cod.exe" *look* like an
// image extension) and the zero-width joiner's siblings, which are otherwise
// invisible. U+200D ZERO WIDTH JOINER is deliberately excluded: it is what
// composes emoji sequences, so a name built from one is legitimate text
// rather than a spoofing attempt, unlike its zero-width relatives here.
//
// This list is named explicitly, in addition to the general category check
// in isDisallowedNameRune, because it gives a clearer, more specific error
// for the most common spoofing characters rather than the generic "format or
// separator character" message.
func isDisallowedNameFormatChar(r rune) bool {
	// Written as hex code points, not rune literals: several of these
	// (notably U+FEFF) are invisible or actively hostile to render in a
	// source file, which defeats the point of writing them out at all.
	switch r {
	case 0x200B, // ZERO WIDTH SPACE
		0x200C, // ZERO WIDTH NON-JOINER
		0xFEFF, // ZERO WIDTH NO-BREAK SPACE / BOM
		0x200E, // LEFT-TO-RIGHT MARK
		0x200F, // RIGHT-TO-LEFT MARK
		0x061C, // ARABIC LETTER MARK
		0x202A, // LEFT-TO-RIGHT EMBEDDING
		0x202B, // RIGHT-TO-LEFT EMBEDDING
		0x202C, // POP DIRECTIONAL FORMATTING
		0x202D, // LEFT-TO-RIGHT OVERRIDE
		0x202E, // RIGHT-TO-LEFT OVERRIDE
		0x2066, // LEFT-TO-RIGHT ISOLATE
		0x2067, // RIGHT-TO-LEFT ISOLATE
		0x2068, // FIRST STRONG ISOLATE
		0x2069: // POP DIRECTIONAL ISOLATE
		return true
	default:
		return false
	}
}

// isDisallowedNameRune reports whether r belongs to a Unicode category that
// lets a name render differently from how it reads, or that renders as
// nothing at all: Cf (Format, the broader family isDisallowedNameFormatChar
// only partially names), Zl (Line Separator) and Zp (Paragraph Separator),
// the tag character block U+E0000-U+E007F (invisible modifiers once used for
// subdivision-flag emoji, a use this rejection deliberately gives up), and
// three Hangul filler characters that render as blank space. U+200D ZERO
// WIDTH JOINER is exempted for the same reason isDisallowedNameFormatChar
// exempts it: it legitimately composes emoji sequences.
func isDisallowedNameRune(r rune) bool {
	if r == 0x200D { // ZERO WIDTH JOINER, exempted
		return false
	}
	if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
		return true
	}
	if r >= 0xE0000 && r <= 0xE007F {
		return true
	}
	switch r {
	case 0x3164, // HANGUL FILLER
		0x115F, // HANGUL CHOSEONG FILLER
		0x1160: // HANGUL JUNGSEONG FILLER
		return true
	}
	return false
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("name must be valid UTF-8: %q", name)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must not be only whitespace: %q", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("name must not be %q", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name must not contain a path separator: %q", name)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("name must not start with '-': %q", name)
	}
	// Windows cannot use a name that ends in a space or a dot as a file name.
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("name must not end with a space or a dot: %q", name)
	}
	if len(name) > maxNameBytes {
		return fmt.Errorf("name must be at most %d bytes, got %d", maxNameBytes, len(name))
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("name must not contain control characters: %q", name)
		}
		if isDisallowedNameFormatChar(r) {
			return fmt.Errorf("name must not contain a bidi or zero-width formatting character: %q", name)
		}
		if isDisallowedNameRune(r) {
			return fmt.Errorf("name must not contain a format, line, paragraph, tag or filler character: %q", name)
		}
	}
	return nil
}

// parseFields splits the space-separated key=value tail of a machine line.
// A token with no "=", or a key seen twice, invalidates the whole line: both
// are signs the line was hand-edited or truncated, not a v1 reference with
// unfamiliar additive fields.
func parseFields(rest string) (map[string]string, error) {
	fields := make(map[string]string)
	for _, tok := range strings.Fields(rest) {
		key, value, ok := strings.Cut(tok, "=")
		if !ok {
			return nil, fmt.Errorf("field %q has no '='", tok)
		}
		if _, dup := fields[key]; dup {
			return nil, fmt.Errorf("duplicate key %q", key)
		}
		fields[key] = value
	}
	return fields, nil
}

func parseAttachLine(line string) (Ref, error) {
	rest, ok := strings.CutPrefix(line, attachMarker)
	if !ok {
		return Ref{}, fmt.Errorf("missing %q marker", attachMarker)
	}
	fields, err := parseFields(rest)
	if err != nil {
		return Ref{}, err
	}

	sha, ok := fields["sha256"]
	if !ok {
		return Ref{}, fmt.Errorf("missing sha256 field")
	}
	if err := validateSHA256(sha); err != nil {
		return Ref{}, err
	}

	sizeStr, ok := fields["size"]
	if !ok {
		return Ref{}, fmt.Errorf("missing size field")
	}
	size, err := parseSize(sizeStr)
	if err != nil {
		return Ref{}, err
	}

	typ, ok := fields["type"]
	if !ok {
		return Ref{}, fmt.Errorf("missing type field")
	}
	if err := validateType(typ); err != nil {
		return Ref{}, err
	}

	nameEnc, ok := fields["name"]
	if !ok {
		return Ref{}, fmt.Errorf("missing name field")
	}
	name, err := url.PathUnescape(nameEnc)
	if err != nil {
		return Ref{}, fmt.Errorf("name %q does not percent-decode: %w", nameEnc, err)
	}
	if err := validateName(name); err != nil {
		return Ref{}, err
	}

	return Ref{SHA256: sha, Size: size, Type: typ, Name: name}, nil
}

func parseDetachLine(line string) (string, error) {
	rest, ok := strings.CutPrefix(line, detachMarker)
	if !ok {
		return "", fmt.Errorf("missing %q marker", detachMarker)
	}
	fields, err := parseFields(rest)
	if err != nil {
		return "", err
	}
	sha, ok := fields["sha256"]
	if !ok {
		return "", fmt.Errorf("missing sha256 field")
	}
	if err := validateSHA256(sha); err != nil {
		return "", err
	}
	return sha, nil
}

// humanSize renders size the way the human line shows it: binary units
// (1024-based) with the plain-letter suffix from the ADR example ("118 KB"),
// not the SI/IEC spelling ("118 kB" or "118 KiB").
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	const units = "KMGTPE"
	return fmt.Sprintf("%.0f %cB", float64(size)/float64(div), units[exp])
}
