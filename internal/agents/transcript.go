package agents

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// maxEvents bounds the log kept per session. The logs view shows the tail of
// a conversation, and a long lane session can write tens of thousands of
// lines.
const maxEvents = 4000

// transcript is the parse state of one transcript file. feed is called once
// per JSONL line, in file order, so the collector can resume from an offset.
type transcript struct {
	tool    Tool
	sess    Session
	events  []Event
	sources map[string]*Source
	order   []string
	calls   map[string]toolCall
	samples []tokenSample
	// lastStop is the stop reason of the last model reply ("end_turn",
	// "tool_use"), or a Codex turn marker ("task_complete", "task_started").
	lastStop    string
	apiError    bool
	conversed   int // bytes of conversation text since the last compaction
	toolOut     int // bytes of tool output since the last compaction
	taskFromBD  bool
	sawPrompt   bool
	compactedAt time.Time
}

type toolCall struct {
	name string
	path string
}

type tokenSample struct {
	at     time.Time
	tokens int
}

func newTranscript(tool Tool, path string) *transcript {
	return &transcript{
		tool:    tool,
		sess:    Session{Tool: tool, Transcript: path, ContextLimit: defaultLimit(tool)},
		sources: map[string]*Source{},
		calls:   map[string]toolCall{},
	}
}

func defaultLimit(tool Tool) int {
	if tool == ToolCodex {
		return 272000
	}
	return 200000
}

func (t *transcript) feed(line []byte) {
	if len(line) == 0 {
		return
	}
	switch t.tool {
	case ToolClaude:
		t.feedClaude(line)
	case ToolCodex:
		t.feedCodex(line)
	}
}

func (t *transcript) touch(at time.Time) {
	if at.IsZero() {
		return
	}
	if t.sess.Started.IsZero() || at.Before(t.sess.Started) {
		t.sess.Started = at
	}
	if at.After(t.sess.LastActivity) {
		t.sess.LastActivity = at
	}
}

func (t *transcript) setCwd(cwd string) {
	if cwd == "" || t.sess.Cwd == cwd {
		return
	}
	t.sess.Cwd = cwd
	t.sess.Project = ProjectFromCwd(cwd)
}

func (t *transcript) setBranch(branch string) {
	if branch == "" || branch == "HEAD" {
		return
	}
	t.sess.Branch = branch
	if t.sess.TaskID == "" {
		if m := branchTaskRe.FindStringSubmatch(branch); m != nil {
			t.sess.TaskID = m[1]
		}
	}
}

func (t *transcript) emit(e Event) {
	e.Text = oneLine(e.Text, 400)
	if len(e.Detail) > 8000 {
		e.Detail = e.Detail[:8000] + "\n…"
	}
	t.events = append(t.events, e)
	if len(t.events) > maxEvents {
		t.events = append(t.events[:0:0], t.events[len(t.events)-maxEvents:]...)
	}
}

// addSource records a context source; a repeat with the same key replaces the
// earlier one, so a re-read file counts once.
func (t *transcript) addSource(key string, s Source) {
	if old, ok := t.sources[key]; ok {
		*old = s
		return
	}
	cp := s
	t.sources[key] = &cp
	t.order = append(t.order, key)
}

// compact drops what a compaction folds into its summary: files read, tool
// output and conversation. Instructions, memories, skills and the task stay,
// because the agent reloads or keeps them.
func (t *transcript) compact(at time.Time, summaryTokens int) {
	t.sess.Compactions++
	t.compactedAt = at
	keep := t.order[:0:0]
	for _, k := range t.order {
		switch t.sources[k].Layer {
		case LayerFiles, LayerToolOutput, LayerConversation:
			delete(t.sources, k)
		default:
			keep = append(keep, k)
		}
	}
	t.order = keep
	t.conversed, t.toolOut = 0, 0
	if summaryTokens > 0 {
		t.addSource("compaction", Source{Layer: LayerConversation, Name: "compaction summary", Scope: "session", Tokens: summaryTokens, LoadedAt: at})
	}
	t.emit(Event{Time: at, Kind: EventCtx, Text: "context compacted"})
}

func (t *transcript) sourceList() []Source {
	out := make([]Source, 0, len(t.order)+2)
	for _, k := range t.order {
		out = append(out, *t.sources[k])
	}
	if t.toolOut > 0 {
		out = append(out, Source{Layer: LayerToolOutput, Name: "tool results", Scope: "session", Tokens: EstimateTokens(t.toolOut)})
	}
	if t.conversed > 0 {
		out = append(out, Source{Layer: LayerConversation, Name: "messages", Scope: "session", Tokens: EstimateTokens(t.conversed)})
	}
	return out
}

func (t *transcript) sample(at time.Time, tokens int) {
	t.sess.TotalTokens += tokens
	t.samples = append(t.samples, tokenSample{at, tokens})
	if len(t.samples) > 512 {
		t.samples = t.samples[len(t.samples)-512:]
	}
}

// tokensPerMin averages output tokens over the ten minutes before now.
func (t *transcript) tokensPerMin(now time.Time) int {
	const window = 10 * time.Minute
	sum := 0
	for i := len(t.samples) - 1; i >= 0; i-- {
		if now.Sub(t.samples[i].at) > window {
			break
		}
		sum += t.samples[i].tokens
	}
	return sum / 10
}

// prompt handles a person's (or a lane dispatcher's) message.
func (t *transcript) prompt(at time.Time, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	t.conversed += len(text)
	first := !t.sawPrompt
	t.sawPrompt = true
	if m := roleRe.FindStringSubmatch(text); m != nil && t.sess.Role == "" {
		t.sess.Role = strings.ToLower(m[1])
	}
	if t.sess.TaskID == "" || (first && !t.taskFromBD) {
		if id := taskInPrompt(text); id != "" {
			t.sess.TaskID = id
		}
	}
	kind := EventUser
	if first {
		kind = EventTask
		if t.sess.Name == "" {
			t.sess.Name = oneLine(text, 40)
		}
		t.addSource("task-prompt", Source{Layer: LayerTask, Name: "first prompt", Scope: "task", Tokens: EstimateTokens(len(text)), LoadedAt: at})
	}
	t.emit(Event{Time: at, Kind: kind, Text: text, Detail: text})
}

// bdCommand notes a bd invocation; a claim names the session's task.
func (t *transcript) bdCommand(cmd string) {
	for _, seg := range cmdSegmentRe.Split(cmd, -1) {
		if m := bdClaimRe.FindStringSubmatch(strings.TrimSpace(seg)); m != nil {
			t.sess.TaskID = m[1]
			t.taskFromBD = true
		}
	}
}

var (
	roleRe       = regexp.MustCompile(`(?i)\byou are (?:the |an? )?(polly|implementer|reviewer|tester|deployer|beads-reader)\b`)
	// bdClaimRe matches one shell command segment that claims a task.
	bdClaimRe    = regexp.MustCompile(`^bd update ([A-Za-z][\w-]*-[a-z0-9]{3,6}(?:\.\d+)*)(?:\s[^\n]*)?\s(?:--claim|--status[= ]in_progress)\b`)
	cmdSegmentRe = regexp.MustCompile(`&&|\|\||[;|\n]`)
	laneTaskRe   = regexp.MustCompile(`\b([A-Za-z][\w-]*-([a-z0-9]{3,6}(?:\.\d+)*)) \[([a-z0-9]{3,6}(?:\.\d+)*)\]`)
	branchTaskRe = regexp.MustCompile(`\b(bd-[a-z0-9]{3,6}(?:\.\d+)*)`)
	bdCmdRe      = regexp.MustCompile(`(?:^|[\s;&|(])bd\s`)
)

// taskInPrompt finds a Beads id in a prompt. A lane prompt writes
// "proj-abc.1 [abc.1]"; an interactive prompt usually names "bd-abc".
func taskInPrompt(text string) string {
	if m := laneTaskRe.FindStringSubmatch(text); m != nil && m[2] == m[3] {
		return m[1]
	}
	if m := branchTaskRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// ProjectFromCwd names the project a working directory belongs to: the base
// name of the checkout, with a .worktrees/<slug> suffix removed.
func ProjectFromCwd(cwd string) string {
	if cwd == "" {
		return ""
	}
	if i := strings.Index(cwd, "/.worktrees/"); i >= 0 {
		cwd = cwd[:i]
	}
	return filepath.Base(filepath.Clean(cwd))
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
