package agents

import (
	"context"
	"os/exec"
	"regexp"
	"strings"
)

// Pane is one tmux pane.
type Pane struct {
	ID      string `json:"id"`     // %12
	Target  string `json:"target"` // session:window.pane
	Window  string `json:"window"`
	PID     string `json:"pid"`
	Path    string `json:"path"`
	Command string `json:"command"`
}

// PaneFormat is the tmux list-panes format ParsePanes reads.
const PaneFormat = "#{pane_id}|#{session_name}:#{window_index}.#{pane_index}|#{window_name}|#{pane_pid}|#{pane_current_path}|#{pane_current_command}"

// ListPanes asks the tmux server for every pane. No tmux server is not an
// error: the result is empty.
func ListPanes(ctx context.Context) []Pane {
	out, err := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", PaneFormat).Output()
	if err != nil {
		return nil
	}
	return ParsePanes(string(out))
}

// ParsePanes reads list-panes output in PaneFormat.
func ParsePanes(out string) []Pane {
	var panes []Pane
	for _, ln := range strings.Split(out, "\n") {
		f := strings.SplitN(ln, "|", 6)
		if len(f) != 6 {
			continue
		}
		panes = append(panes, Pane{ID: f[0], Target: f[1], Window: f[2], PID: f[3], Path: f[4], Command: f[5]})
	}
	return panes
}

// Claude Code runs as a process named after its version ("2.1.325").
var versionCmdRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// paneTool says which agent a pane's foreground command looks like.
func paneTool(cmd string) (Tool, bool) {
	c := strings.ToLower(cmd)
	switch {
	case versionCmdRe.MatchString(cmd), strings.Contains(c, "claude"):
		return ToolClaude, true
	case strings.Contains(c, "codex"):
		return ToolCodex, true
	case c == "node":
		return "", true
	}
	return "", false
}

// MatchPanes gives each agent pane to at most one session in the same working
// directory: the one whose name equals the window name, else the most recently
// active one. sessions must be sorted most recent first.
func MatchPanes(sessions []*Session, panes []Pane) {
	taken := map[*Session]bool{}
	pick := func(p Pane, tool Tool, byName bool) bool {
		for _, s := range sessions {
			if taken[s] || s.Cwd != p.Path || (tool != "" && s.Tool != tool) {
				continue
			}
			if byName && s.Name != p.Window {
				continue
			}
			taken[s] = true
			s.Where = Where{Kind: WhereTmux, Ref: p.ID, Label: p.Target}
			return true
		}
		return false
	}
	var rest []Pane
	for _, p := range panes {
		tool, ok := paneTool(p.Command)
		if !ok {
			continue
		}
		if !pick(p, tool, true) {
			rest = append(rest, p)
		}
	}
	for _, p := range rest {
		tool, _ := paneTool(p.Command)
		pick(p, tool, false)
	}
}
