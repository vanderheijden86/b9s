package agents

import (
	"strings"
	"testing"
)

func TestContainsBlurb(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "no blurb",
			content:  "# My AGENTS.md\n\nSome other content.",
			expected: false,
		},
		{
			name:     "has blurb v1",
			content:  "# My AGENTS.md\n\n<!-- bv-agent-instructions-v1 -->\nSome content\n<!-- end-bv-agent-instructions -->",
			expected: true,
		},
		{
			name:     "has blurb v2 (future)",
			content:  "# My AGENTS.md\n\n<!-- bv-agent-instructions-v2 -->\nSome content\n<!-- end-bv-agent-instructions -->",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsBlurb(tt.content)
			if result != tt.expected {
				t.Errorf("ContainsBlurb() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetBlurbVersion(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "no blurb",
			content:  "# My AGENTS.md",
			expected: 0,
		},
		{
			name:     "version 1",
			content:  "<!-- bv-agent-instructions-v1 -->",
			expected: 1,
		},
		{
			name:     "version 2 (future)",
			content:  "<!-- bv-agent-instructions-v2 -->",
			expected: 2,
		},
		{
			name:     "version 10 (multi-digit)",
			content:  "<!-- bv-agent-instructions-v10 -->",
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBlurbVersion(tt.content)
			if result != tt.expected {
				t.Errorf("GetBlurbVersion() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAppendBlurb(t *testing.T) {
	content := "# My AGENTS.md\n\nSome existing content."
	result := AppendBlurb(content)

	// Should contain the start marker
	if !strings.Contains(result, BlurbStartMarker) {
		t.Error("AppendBlurb() result missing start marker")
	}

	// Should contain the end marker
	if !strings.Contains(result, BlurbEndMarker) {
		t.Error("AppendBlurb() result missing end marker")
	}

	// Should contain key content
	if !strings.Contains(result, "bd ready") {
		t.Error("AppendBlurb() result missing 'bd ready' command")
	}

	// Should preserve original content
	if !strings.Contains(result, "Some existing content.") {
		t.Error("AppendBlurb() did not preserve original content")
	}

	// Original content should come first
	origIdx := strings.Index(result, "Some existing content.")
	blurbIdx := strings.Index(result, BlurbStartMarker)
	if origIdx >= blurbIdx {
		t.Error("AppendBlurb() should place blurb after original content")
	}
}

func TestRemoveBlurb(t *testing.T) {
	// Content with blurb
	withBlurb := "# My AGENTS.md\n\nSome content.\n\n" + AgentBlurb + "\n"
	result := RemoveBlurb(withBlurb)

	// Should not contain markers
	if strings.Contains(result, BlurbStartMarker) {
		t.Error("RemoveBlurb() result still contains start marker")
	}
	if strings.Contains(result, BlurbEndMarker) {
		t.Error("RemoveBlurb() result still contains end marker")
	}

	// Should preserve original content
	if !strings.Contains(result, "Some content.") {
		t.Error("RemoveBlurb() did not preserve original content")
	}
}

func TestRemoveBlurbNoBlurb(t *testing.T) {
	content := "# My AGENTS.md\n\nNo blurb here."
	result := RemoveBlurb(content)

	// Should be unchanged
	if result != content {
		t.Errorf("RemoveBlurb() modified content without blurb: got %q, want %q", result, content)
	}
}

func TestUpdateBlurb(t *testing.T) {
	// Start with content containing old blurb
	oldContent := "# My AGENTS.md\n\n<!-- bv-agent-instructions-v1 -->\nOld blurb content\n<!-- end-bv-agent-instructions -->\n"
	result := UpdateBlurb(oldContent)

	// Should have exactly one blurb
	count := strings.Count(result, BlurbStartMarker)
	if count != 1 {
		t.Errorf("UpdateBlurb() resulted in %d blurbs, want 1", count)
	}

	// Should have current blurb content
	if !strings.Contains(result, "bd ready") {
		t.Error("UpdateBlurb() result missing current blurb content")
	}

	// Should preserve header
	if !strings.Contains(result, "# My AGENTS.md") {
		t.Error("UpdateBlurb() did not preserve original header")
	}
}

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no blurb",
			content:  "# No blurb",
			expected: false,
		},
		{
			name:     "current version",
			content:  "<!-- bv-agent-instructions-v1 -->",
			expected: false, // v1 is current, no update needed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsUpdate(tt.content)
			if result != tt.expected {
				t.Errorf("NeedsUpdate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAgentBlurbContent(t *testing.T) {
	// Verify blurb contains essential commands
	essentials := []string{
		"bd ready",
		"bd list",
		"bd show",
		"bd create",
		"bd update",
		"bd close",
		"bd sync",
		"bd dep add",
	}

	for _, cmd := range essentials {
		if !strings.Contains(AgentBlurb, cmd) {
			t.Errorf("AgentBlurb missing essential command: %s", cmd)
		}
	}

	// Verify markers
	if !strings.HasPrefix(AgentBlurb, BlurbStartMarker) {
		t.Error("AgentBlurb should start with BlurbStartMarker")
	}
	if !strings.HasSuffix(strings.TrimSpace(AgentBlurb), BlurbEndMarker) {
		t.Error("AgentBlurb should end with BlurbEndMarker")
	}
}

func TestSupportedAgentFiles(t *testing.T) {
	// Should support common variations
	expected := map[string]bool{
		"AGENTS.md": true,
		"CLAUDE.md": true,
		"agents.md": true,
		"claude.md": true,
	}

	for _, file := range SupportedAgentFiles {
		if !expected[file] {
			t.Errorf("Unexpected file in SupportedAgentFiles: %s", file)
		}
		delete(expected, file)
	}

	for missing := range expected {
		t.Errorf("Missing expected file in SupportedAgentFiles: %s", missing)
	}
}

// LegacyBlurbContent is a sample of the old-format blurb (pre-v1, without HTML markers)
const LegacyBlurbContent = `### Using bv as an AI sidecar

If you're an AI agent (like Claude, GPT, Codex, etc.), bv can serve as your
external memory and decision-support system for handling complex multi-part
coding tasks.

**Entry point**: Always start with ` + "`" + `bv --robot-triage` + "`" + `

**Available robot flags**:
- ` + "`" + `--robot-triage` + "`" + ` - Get structured task overview and priorities
- ` + "`" + `--robot-insights` + "`" + ` - Deep analysis with recommendations
- ` + "`" + `--robot-plan` + "`" + ` - Generate actionable task breakdown

**Why use robot flags?**
bv already computes the hard parts for you.
` + "```"

func TestContainsLegacyBlurb(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "no blurb",
			content:  "# My AGENTS.md\n\nSome other content.",
			expected: false,
		},
		{
			name:     "has legacy blurb",
			content:  "# My AGENTS.md\n\n" + LegacyBlurbContent,
			expected: true,
		},
		{
			name:     "has current blurb (not legacy)",
			content:  "# My AGENTS.md\n\n" + AgentBlurb,
			expected: false,
		},
		{
			name:     "partial legacy (missing patterns)",
			content:  "# My AGENTS.md\n\n### Using bv as an AI sidecar\nJust a header.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsLegacyBlurb(tt.content)
			if result != tt.expected {
				t.Errorf("ContainsLegacyBlurb() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestContainsAnyBlurb(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no blurb",
			content:  "# My AGENTS.md",
			expected: false,
		},
		{
			name:     "has current blurb",
			content:  "# AGENTS.md\n\n" + AgentBlurb,
			expected: true,
		},
		{
			name:     "has legacy blurb",
			content:  "# AGENTS.md\n\n" + LegacyBlurbContent,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsAnyBlurb(tt.content)
			if result != tt.expected {
				t.Errorf("ContainsAnyBlurb() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRemoveLegacyBlurb(t *testing.T) {
	// Content with legacy blurb
	withLegacy := "# My AGENTS.md\n\nSome content.\n\n" + LegacyBlurbContent + "\n\n## Other Section\n"
	result := RemoveLegacyBlurb(withLegacy)

	// Should not contain legacy markers
	if strings.Contains(result, "### Using bv as an AI sidecar") {
		t.Error("RemoveLegacyBlurb() result still contains legacy header")
	}
	if strings.Contains(result, "--robot-insights") {
		t.Error("RemoveLegacyBlurb() result still contains robot flags")
	}

	// Should preserve original content before and after
	if !strings.Contains(result, "Some content.") {
		t.Error("RemoveLegacyBlurb() did not preserve content before blurb")
	}
	if !strings.Contains(result, "## Other Section") {
		t.Error("RemoveLegacyBlurb() did not preserve content after blurb")
	}
}

func TestRemoveLegacyBlurbNoLegacy(t *testing.T) {
	content := "# My AGENTS.md\n\nNo legacy blurb here."
	result := RemoveLegacyBlurb(content)

	// Should be unchanged
	if result != content {
		t.Errorf("RemoveLegacyBlurb() modified content without legacy: got %q, want %q", result, content)
	}
}

func TestRemoveLegacyBlurbNoTrailingBackticks(t *testing.T) {
	// Legacy content WITHOUT trailing triple backticks (regression test for regex fix)
	legacyNoBackticks := `# My AGENTS.md

### Using bv as an AI sidecar

Some description here.

**Available robot flags**:
- --robot-insights - Analysis
- --robot-plan - Planning

bv already computes the hard parts for you.

## Next Section
`
	result := RemoveLegacyBlurb(legacyNoBackticks)

	// Should not contain legacy markers
	if strings.Contains(result, "### Using bv as an AI sidecar") {
		t.Error("RemoveLegacyBlurb() did not remove legacy header (no trailing backticks case)")
	}
	if strings.Contains(result, "--robot-insights") {
		t.Error("RemoveLegacyBlurb() did not remove robot flags (no trailing backticks case)")
	}
	if strings.Contains(result, "bv already computes the hard parts") {
		t.Error("RemoveLegacyBlurb() did not remove end phrase (no trailing backticks case)")
	}

	// Should preserve surrounding content
	if !strings.Contains(result, "# My AGENTS.md") {
		t.Error("RemoveLegacyBlurb() did not preserve header")
	}
	if !strings.Contains(result, "## Next Section") {
		t.Error("RemoveLegacyBlurb() did not preserve next section")
	}
}

func TestUpdateBlurbFromLegacy(t *testing.T) {
	// Start with content containing legacy blurb
	legacyContent := "# My AGENTS.md\n\n" + LegacyBlurbContent + "\n"
	result := UpdateBlurb(legacyContent)

	// Should have exactly one current blurb
	count := strings.Count(result, BlurbStartMarker)
	if count != 1 {
		t.Errorf("UpdateBlurb() from legacy resulted in %d blurbs, want 1", count)
	}

	// Should have current blurb content
	if !strings.Contains(result, "bd ready") {
		t.Error("UpdateBlurb() from legacy missing current blurb content")
	}

	// Should NOT have legacy content
	if strings.Contains(result, "--robot-insights") {
		t.Error("UpdateBlurb() from legacy still contains legacy content")
	}

	// Should preserve header
	if !strings.Contains(result, "# My AGENTS.md") {
		t.Error("UpdateBlurb() from legacy did not preserve original header")
	}
}

func TestNeedsUpdateLegacy(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "legacy blurb needs update",
			content:  "# AGENTS.md\n\n" + LegacyBlurbContent,
			expected: true,
		},
		{
			name:     "current blurb no update",
			content:  "# AGENTS.md\n\n" + AgentBlurb,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsUpdate(tt.content)
			if result != tt.expected {
				t.Errorf("NeedsUpdate() = %v, want %v", result, tt.expected)
			}
		})
	}
}
