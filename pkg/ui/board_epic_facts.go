package ui

import "strings"

// FirstSentence returns the first sentence of a description: the text up to
// the first ". ", "! " or "? " (or that mark at the end), or up to the first
// blank line, whichever comes first. Leading Markdown heading lines are
// skipped, so "# Goal" never reads as the sentence, and a leading list marker
// is dropped. A period inside a word, as in "2.1", does not end the sentence.
func FirstSentence(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for len(lines) > 0 {
		l := strings.TrimSpace(lines[0])
		if l != "" && !strings.HasPrefix(l, "#") {
			break
		}
		lines = lines[1:]
	}
	s = strings.Join(lines, "\n")
	if para, _, ok := strings.Cut(s, "\n\n"); ok {
		s = strings.TrimSpace(para)
	}
	for _, mark := range []string{"-", "*", "+"} {
		if strings.HasPrefix(s, mark) {
			s = strings.TrimLeft(s, mark+" ")
			break
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '.' || c == '!' || c == '?' {
			if i+1 == len(s) || s[i+1] == ' ' {
				return s[:i+1]
			}
		}
	}
	return s
}
