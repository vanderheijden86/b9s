package agents

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// importRe matches a Claude instruction-file import: "@path" at the start of
// a line or after whitespace, where path starts with ~, / or a dot.
var importRe = regexp.MustCompile(`(?m)(?:^|\s)@((?:~|\.{1,2})?/[^\s)]+)`)

// InstructionFiles lists the instruction files an agent loads for cwd, in
// load order. Claude reads ~/.claude/CLAUDE.md, then CLAUDE.md,
// .claude/CLAUDE.md and CLAUDE.local.md in every directory from the root down
// to cwd, following @imports. Codex reads ~/.codex/AGENTS.md and AGENTS.md
// in every directory from the root down to cwd.
func InstructionFiles(tool Tool, home, cwd string) []Source {
	var out []Source
	seen := map[string]bool{}
	var add func(path, scope string, depth int)
	add = func(path, scope string, depth int) {
		real, err := filepath.EvalSymlinks(path)
		if err != nil || seen[real] || depth > 4 {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		seen[real] = true
		info, _ := os.Stat(path)
		s := Source{Layer: LayerInstructions, Name: displayPath(home, path), Path: path, Scope: scope, Tokens: EstimateTokens(len(data))}
		if info != nil {
			s.ChangedAt = info.ModTime()
		}
		out = append(out, s)
		if tool != ToolClaude {
			return
		}
		for _, m := range importRe.FindAllStringSubmatch(string(data), -1) {
			p := m[1]
			switch {
			case strings.HasPrefix(p, "~/"):
				p = filepath.Join(home, p[2:])
			case !filepath.IsAbs(p):
				p = filepath.Join(filepath.Dir(path), p)
			}
			add(p, scope, depth+1)
		}
	}
	dirs := ancestors(cwd)
	switch tool {
	case ToolClaude:
		add(filepath.Join(home, ".claude", "CLAUDE.md"), "global", 0)
		for _, d := range dirs {
			for _, name := range []string{"CLAUDE.md", filepath.Join(".claude", "CLAUDE.md"), "CLAUDE.local.md"} {
				add(filepath.Join(d, name), "project", 0)
			}
		}
	case ToolCodex:
		add(filepath.Join(home, ".codex", "AGENTS.md"), "global", 0)
		for _, d := range dirs {
			add(filepath.Join(d, "AGENTS.md"), "project", 0)
		}
	}
	return out
}

// ClaudeMemoryFile is the auto-memory index Claude Code loads for cwd.
func ClaudeMemoryFile(home, cwd string) string {
	return filepath.Join(home, ".claude", "projects", ClaudeProjectDir(cwd), "memory", "MEMORY.md")
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// ClaudeProjectDir is the directory name Claude Code keeps a cwd's
// transcripts under: every character that is not a letter or digit becomes a
// dash.
func ClaudeProjectDir(cwd string) string {
	return nonAlnum.ReplaceAllString(cwd, "-")
}

// ancestors lists cwd and its parents, root first, stopping below the home
// directory's parent so /Users itself is never searched.
func ancestors(cwd string) []string {
	if cwd == "" {
		return nil
	}
	var out []string
	for d := filepath.Clean(cwd); ; d = filepath.Dir(d) {
		out = append([]string{d}, out...)
		if d == filepath.Dir(d) {
			break
		}
	}
	return out
}

func displayPath(home, p string) string {
	if home != "" && strings.HasPrefix(p, home+"/") {
		return "~/" + strings.TrimPrefix(p, home+"/")
	}
	return p
}

// BuildContext combines the files an agent loads at start with the sources
// its transcript recorded. A file is stale when it changed after the agent
// loaded it: the running session still works from the old text.
func BuildContext(s Session, recorded []Source, home string, compactedAt time.Time) ContextReport {
	loaded := s.Started
	if compactedAt.After(loaded) {
		loaded = compactedAt
	}
	var sources []Source
	if s.Tool == ToolClaude || s.Tool == ToolCodex {
		for _, f := range InstructionFiles(s.Tool, home, s.Cwd) {
			if s.Tool == ToolCodex && f.Scope == "project" && hasAgentsMessage(recorded, filepath.Dir(f.Path)) {
				continue
			}
			f.LoadedAt = loaded
			f.Stale = !loaded.IsZero() && f.ChangedAt.After(loaded)
			sources = append(sources, f)
		}
	}
	if s.Tool == ToolClaude {
		mem := ClaudeMemoryFile(home, s.Cwd)
		if data, err := os.ReadFile(mem); err == nil {
			info, _ := os.Stat(mem)
			src := Source{Layer: LayerMemories, Name: "MEMORY.md (auto memory)", Path: mem, Scope: "project", Tokens: EstimateTokens(len(data)), LoadedAt: loaded}
			if info != nil {
				src.ChangedAt = info.ModTime()
				src.Stale = !loaded.IsZero() && src.ChangedAt.After(loaded)
			}
			sources = append(sources, src)
		}
	}
	for _, r := range recorded {
		if r.Layer == LayerInstructions && r.Path != "" && s.Tool == ToolCodex {
			if info, err := os.Stat(filepath.Join(r.Path, "AGENTS.md")); err == nil {
				r.ChangedAt = info.ModTime()
				r.Stale = r.ChangedAt.After(r.LoadedAt)
			}
		}
		if r.Layer == LayerFiles && r.Path != "" {
			if info, err := os.Stat(r.Path); err == nil {
				r.ChangedAt = info.ModTime()
				r.Stale = r.ChangedAt.After(r.LoadedAt)
			}
		}
		sources = append(sources, r)
	}
	totals := map[Layer]int{}
	sum := 0
	for _, src := range sources {
		totals[src.Layer] += src.Tokens
		sum += src.Tokens
	}
	if rest := s.ContextTokens - sum; rest > 0 {
		totals[LayerSystem] += rest
		sources = append(sources, Source{Layer: LayerSystem, Name: "system prompt, tool schemas and unattributed", Scope: "session", Tokens: rest, LoadedAt: s.Started})
	}
	rep := ContextReport{SessionID: s.ID, Window: s.ContextTokens, Limit: s.ContextLimit, Sources: sources}
	for _, l := range Layers {
		rep.Layers = append(rep.Layers, LayerTotal{Layer: l, Tokens: totals[l]})
	}
	return rep
}

func hasAgentsMessage(recorded []Source, dir string) bool {
	for _, r := range recorded {
		if r.Layer == LayerInstructions && r.Path == dir {
			return true
		}
	}
	return false
}
