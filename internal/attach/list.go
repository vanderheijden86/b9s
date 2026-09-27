package attach

import "github.com/vanderheijden86/beadwork/internal/attachref"

// List returns issueID's attachments, folded from its comments in the order
// attachref.Collect defines.
func List(cl CommentLoader, issueID string) ([]attachref.Attachment, error) {
	comments, err := cl.Comments(issueID)
	if err != nil {
		return nil, err
	}
	return attachref.Collect(comments), nil
}
