package ui

import "github.com/vanderheijden86/b9s/pkg/version"

// buildInfo is the running build; tests replace it to render a known one.
var buildInfo = version.Get

// buildLine names the running build, with the commit as an OSC 8 hyperlink to
// GitHub so a report can be traced to the exact source. Terminals without
// OSC 8 support print the label and drop the link.
func buildLine(info version.Info) string {
	line := "b9s " + info.Label()
	short := info.ShortCommit()
	if short == "" {
		return line
	}
	line += " · commit " + osc8Hyperlink(info.CommitURL(), short)
	if info.Modified {
		line += " (modified)"
	}
	return line
}
