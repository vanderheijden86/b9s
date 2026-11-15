// Package agents finds the coding-agent sessions that run on this machine
// (Claude Code and Codex transcripts, tmux panes, optional Omnigent lane
// pods) and turns each transcript into a session summary, a log of events and
// an estimate of what fills its context window.
//
// Everything here is read-only. Transcripts are parsed incrementally from a
// remembered offset, so a rescan costs only the bytes appended since the last
// one.
package agents

import "time"

// Tool names the agent program that wrote a transcript.
type Tool string

const (
	ToolClaude Tool = "claude"
	ToolCodex  Tool = "codex"
	ToolLane   Tool = "lane"
)

// State is the closed set of states a session can be in. The collector
// derives it from the transcript tail and from liveness (a tmux pane or a
// pod), never stores it.
type State string

const (
	StateRunning State = "Running" // a turn is in progress
	StateWaiting State = "Waiting" // the turn ended and the agent waits for a person
	StateIdle    State = "Idle"    // live, but nothing happened for IdleAfter
	StateFailed  State = "Failed"  // the last turn ended in an API error
	StatePending State = "Pending" // a lane pod that has not started
	StateDone    State = "Done"    // no pane, no pod and no recent activity
)

// WhereKind says what hosts a session.
type WhereKind string

const (
	WhereNone WhereKind = ""
	WhereTmux WhereKind = "tmux"
	WherePod  WhereKind = "pod"
)

// Where locates the process that runs a session.
type Where struct {
	Kind  WhereKind `json:"kind"`
	Ref   string    `json:"ref"`   // tmux pane id (%12) or namespace/pod
	Label string    `json:"label"` // tmux session:window.pane or the pod name
}

// Session is one agent conversation.
type Session struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Tool          Tool      `json:"tool"`
	Project       string    `json:"project"`
	Cwd           string    `json:"cwd"`
	Branch        string    `json:"branch"`
	Role          string    `json:"role"`
	TaskID        string    `json:"taskId"`
	State         State     `json:"state"`
	Model         string    `json:"model"`
	ContextTokens int       `json:"contextTokens"`
	ContextLimit  int       `json:"contextLimit"`
	TokensPerMin  int       `json:"tokensPerMin"`
	TotalTokens   int       `json:"totalTokens"`
	Compactions   int       `json:"compactions"`
	Started       time.Time `json:"started"`
	LastActivity  time.Time `json:"lastActivity"`
	Where         Where     `json:"where"`
	Transcript    string    `json:"transcript"`
	Tools         int       `json:"toolCalls"`
	Errors        int       `json:"errors"`
}

// ContextPercent is the share of the window in use, 0 to 100.
func (s Session) ContextPercent() int {
	if s.ContextLimit <= 0 {
		return 0
	}
	p := s.ContextTokens * 100 / s.ContextLimit
	if p > 100 {
		return 100
	}
	return p
}

// Live reports whether a process still hosts the session.
func (s Session) Live() bool {
	return s.State != StateDone
}

// EventKind is the closed set of log line kinds the logs view filters on.
type EventKind string

const (
	EventTask   EventKind = "TASK"
	EventUser   EventKind = "USER"
	EventAI     EventKind = "AI"
	EventTool   EventKind = "TOOL"
	EventResult EventKind = "RESULT"
	EventBD     EventKind = "BD"
	EventCtx    EventKind = "CTX"
	EventErr    EventKind = "ERR"
)

// Event is one line of an agent log.
type Event struct {
	Time   time.Time `json:"time"`
	Kind   EventKind `json:"kind"`
	Tool   string    `json:"tool,omitempty"`
	Text   string    `json:"text"`
	Detail string    `json:"detail,omitempty"`
}

// Layer groups context sources the way the context view stacks them.
type Layer string

const (
	LayerInstructions Layer = "Instructions"
	LayerMemories     Layer = "Memories"
	LayerSkills       Layer = "Skills"
	LayerTask         Layer = "Task"
	LayerFiles        Layer = "Files read"
	LayerToolOutput   Layer = "Tool output"
	LayerConversation Layer = "Conversation"
	LayerSystem       Layer = "System and tools"
)

// Layers lists every layer in display order.
var Layers = []Layer{
	LayerInstructions, LayerMemories, LayerSkills, LayerTask,
	LayerFiles, LayerToolOutput, LayerConversation, LayerSystem,
}

// Source is one thing loaded into a context window.
type Source struct {
	Layer     Layer     `json:"layer"`
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`
	Scope     string    `json:"scope,omitempty"` // global, project, role, task, session
	Tokens    int       `json:"tokens"`
	LoadedAt  time.Time `json:"loadedAt"`
	ChangedAt time.Time `json:"changedAt,omitempty"`
	Stale     bool      `json:"stale"`
}

// LayerTotal is the token estimate for one layer.
type LayerTotal struct {
	Layer  Layer `json:"layer"`
	Tokens int   `json:"tokens"`
}

// ContextReport estimates what fills a session's context window. Window is
// the measured size from the last model call; the per-source numbers are
// estimates (bytes / 4), and the system layer takes the remainder.
type ContextReport struct {
	SessionID string       `json:"sessionId"`
	Window    int          `json:"window"`
	Limit     int          `json:"limit"`
	Layers    []LayerTotal `json:"layers"`
	Sources   []Source     `json:"sources"`
}

// EstimateTokens approximates a token count from text length.
func EstimateTokens(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + 3) / 4
}
