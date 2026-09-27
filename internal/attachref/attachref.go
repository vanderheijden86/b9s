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

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/model"
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
// references and detach hashes found. A line that starts with a marker but
// breaks one of its rules is skipped rather than returned, and never causes
// the rest of the text to be rejected. An unrecognised version (not "/v1")
// never matches a marker, so it is skipped the same way.
func Parse(text string) (refs []Ref, detaches []string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, attachMarker):
			r, err := parseAttachLine(line)
			if err != nil {
				debug.Log("attachref: ignoring invalid attach line %q: %v", line, err)
				continue
			}
			refs = append(refs, r)
		case strings.HasPrefix(line, detachMarker):
			sha, err := parseDetachLine(line)
			if err != nil {
				debug.Log("attachref: ignoring invalid detach line %q: %v", line, err)
				continue
			}
			detaches = append(detaches, sha)
		}
	}
	return refs, detaches
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
// comment ID as tie-break. Position in the result reflects when a hash was
// first attached, not when it was last re-attached after a detach, so a
// re-attach updates the author and time shown but not the list order.
func Collect(comments []*model.Comment) []Attachment {
	sorted := make([]*model.Comment, len(comments))
	copy(sorted, comments)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !sorted[i].CreatedAt.Equal(sorted[j].CreatedAt) {
			return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
		}
		return sorted[i].ID < sorted[j].ID
	})

	live := make(map[string]Attachment)
	firstSeen := make(map[string]bool)
	var order []string
	for _, c := range sorted {
		refs, detaches := Parse(c.Text)
		for _, r := range refs {
			if !firstSeen[r.SHA256] {
				firstSeen[r.SHA256] = true
				order = append(order, r.SHA256)
			}
			live[r.SHA256] = Attachment{
				Ref:       r,
				CommentID: c.ID,
				AddedBy:   c.Author,
				AddedAt:   c.CreatedAt,
			}
		}
		for _, sha := range detaches {
			delete(live, sha)
		}
	}

	result := make([]Attachment, 0, len(order))
	for _, sha := range order {
		if a, ok := live[sha]; ok {
			result = append(result, a)
		}
	}
	return result
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

// validateType rejects whitespace outright: the machine line is a
// space-delimited list of key=value fields and the type is not
// percent-encoded, so a type containing a space (a MIME parameter such as
// "; charset=utf-8") would corrupt the fields that follow it on re-parse.
// mime.ParseMediaType alone is not enough: it also accepts a bare token with
// no slash, which is not a MIME type.
func validateType(t string) error {
	if t == "" || strings.ContainsAny(t, " \t\r\n") {
		return fmt.Errorf("type must be a single token with no whitespace: %q", t)
	}
	if !strings.Contains(t, "/") {
		return fmt.Errorf("type must be a MIME type in type/subtype form: %q", t)
	}
	if _, _, err := mime.ParseMediaType(t); err != nil {
		return fmt.Errorf("type %q is not a valid MIME type: %w", t, err)
	}
	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("name must not be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("name must not be %q", name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("name must not contain a path separator: %q", name)
	}
	if len(name) > maxNameBytes {
		return fmt.Errorf("name must be at most %d bytes, got %d", maxNameBytes, len(name))
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("name must not contain control characters: %q", name)
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
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return Ref{}, fmt.Errorf("size %q is not a decimal integer: %w", sizeStr, err)
	}
	if err := validateSize(size); err != nil {
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
