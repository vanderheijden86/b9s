package attach

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vanderheijden86/beadwork/internal/attachref"
)

// Resolve finds the one attachment query names: a full sha256, a unique
// hash prefix of at least 8 characters, or an exact name. Attachments
// matching by either means are collected together by hash, so a query that
// matches more than one attachment is reported as ambiguous instead of
// picking one silently.
func Resolve(attachments []attachref.Attachment, query string) (attachref.Attachment, error) {
	matched := make(map[string]attachref.Attachment)
	if isHexPrefix(query) {
		for _, a := range attachments {
			if strings.HasPrefix(a.SHA256, query) {
				matched[a.SHA256] = a
			}
		}
	}
	for _, a := range attachments {
		if a.Name == query {
			matched[a.SHA256] = a
		}
	}

	switch len(matched) {
	case 1:
		for _, a := range matched {
			return a, nil
		}
	case 0:
		return attachref.Attachment{}, fmt.Errorf("no attachment matches %q", query)
	}

	hashes := make([]string, 0, len(matched))
	for h := range matched {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	return attachref.Attachment{}, fmt.Errorf("%q matches more than one attachment: %s", query, strings.Join(hashes, ", "))
}

// isHexPrefix reports whether query is shaped like a sha256 prefix (8 to 64
// lowercase hex characters), so Resolve searches by hash as well as by name.
func isHexPrefix(query string) bool {
	if len(query) < 8 || len(query) > 64 {
		return false
	}
	for _, r := range query {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
