package attach

import (
	"fmt"

	"github.com/vanderheijden86/beadwork/internal/attachref"
)

// Detach appends a detach comment for sha256, provided it is currently
// attached to issueID. Beads has no verb to edit or delete a comment, so
// this never touches the earlier attach comment: the attachment list is a
// fold of both kinds of comment in time order (ADR 0024).
func Detach(bd BdRunner, cl CommentLoader, issueID, sha256Hash string) error {
	if !isSHA256(sha256Hash) {
		return fmt.Errorf("%q is not a 64 character lowercase hex sha256", sha256Hash)
	}
	comments, err := cl.Comments(issueID)
	if err != nil {
		return err
	}
	attachments := attachref.Collect(comments)
	var found *attachref.Attachment
	for i := range attachments {
		if attachments[i].SHA256 == sha256Hash {
			found = &attachments[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("%s is not currently attached to %s", sha256Hash, issueID)
	}

	text, err := attachref.FormatDetach(attachref.Ref{SHA256: sha256Hash, Name: found.Name})
	if err != nil {
		return err
	}
	if _, err := bd.Run("comments", "add", issueID, text); err != nil {
		return fmt.Errorf("bd comments add: %w", err)
	}
	return nil
}

// isSHA256 reports whether s is 64 lowercase hex characters, the same shape
// attachref requires of a machine line's sha256 field.
func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
