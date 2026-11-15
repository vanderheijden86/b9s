package web

// MemoryGraphResponse is the Memory graph of the open project. The graph is
// read on the first request for it (ADR 0051), so a response may report the
// read in progress instead.
type MemoryGraphResponse struct {
	Version   uint64 `json:"version"`
	Available bool   `json:"available"`
	// Reason says in one line why Available is false.
	Reason string `json:"reason,omitempty"`
	// Loading is true while the first read runs. Done of Total counts the
	// Issue components the latest read traversed, and Total is 0 while it
	// reads the inventory.
	Loading bool              `json:"loading"`
	Done    int               `json:"done"`
	Total   int               `json:"total"`
	Nodes   []MemoryGraphNode `json:"nodes"`
	Edges   []MemoryGraphEdge `json:"edges"`
}

type MemoryGraphNode struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Type    string `json:"type"`
	Version string `json:"record_version"`
	Body    string `json:"body"`
}

type MemoryGraphEdge struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Note   string `json:"note"`
}

type IssueMemoryLink struct {
	Link     MemoryGraphEdge `json:"link"`
	Memory   MemoryGraphNode `json:"memory"`
	Outgoing bool            `json:"outgoing"`
}

// These types are the JSON contract between b9s web and the browser. The
// TypeScript in web/src/api.gen.ts is generated from them (see tsgen_test.go), so a
// change here that is not regenerated fails TestGeneratedTypesAreCurrent.

// Issue is one issue as the browser shows it. A snapshot leaves out
// Description, Design, Acceptance, Notes, CloseReason, Metadata and Comments,
// which make up most of its size, and the detail view reads them from GET
// api/issue. Blocked, Ready and ClosedLike are derived on the server with the
// TUI's rules, so both UIs count alike.
type Issue struct {
	MemoryAvailable bool              `json:"memory_available"`
	MemoryLoading   bool              `json:"memory_loading"`
	MemoryLinks     []IssueMemoryLink `json:"memory_links"`
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Description     string            `json:"description"`
	Design          string            `json:"design"`
	Acceptance      string            `json:"acceptance"`
	Notes           string            `json:"notes"`
	CloseReason     string            `json:"close_reason"`
	Metadata        string            `json:"metadata"`
	Status          string            `json:"status"`
	Priority        int               `json:"priority"`
	Type            string            `json:"type"`
	Assignee        string            `json:"assignee"`
	CreatedBy       string            `json:"created_by"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
	ClosedAt        string            `json:"closed_at"`
	DeferUntil      string            `json:"defer_until"`
	Due             string            `json:"due"`
	// Summary is the first sentence of an epic's description, for the board's
	// epic cell. A snapshot keeps it while it drops Description.
	Summary        string   `json:"summary"`
	Labels         []string `json:"labels"`
	Parent         string   `json:"parent"`
	BlockedBy      []string `json:"blocked_by"`
	Related        []string `json:"related"`
	DiscoveredFrom []string `json:"discovered_from"`
	// Comments is empty in a snapshot, which carries only CommentCount.
	// GET api/issue returns them with the other long text fields.
	Comments     []Comment `json:"comments"`
	CommentCount int       `json:"comment_count"`
	Project      string    `json:"project"`
	Blocked      bool      `json:"blocked"`
	Ready        bool      `json:"ready"`
	ClosedLike   bool      `json:"closed_like"`
}

// Comment is one comment on an issue.
type Comment struct {
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// Snapshot is everything the browser needs to draw one project.
type Snapshot struct {
	Version uint64      `json:"version"`
	Project ProjectInfo `json:"project"`
	Issues  []Issue     `json:"issues"`
	Actor   string      `json:"actor"`
	People  []Person    `json:"people"`
	Health  Health      `json:"health"`
}

// ProjectInfo describes the project on screen.
type ProjectInfo struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	// ReadOnly is set for a project without a checkout and for the combined
	// all-projects view: bd needs a checkout to write through.
	ReadOnly bool `json:"read_only"`
	All      bool `json:"all"`
}

// Health is the data source report behind the header dot (TUI key D).
type Health struct {
	OK       bool   `json:"ok"`
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Watching string `json:"watching"`
	LoadedAt string `json:"loaded_at"`
	Issues   int    `json:"issues"`
	// Fallback explains why a local export is shown instead of the Dolt
	// server named in the project's metadata.
	Fallback string `json:"fallback"`
	Error    string `json:"error"`
	BdFound  bool   `json:"bd_found"`
}

// Person is one identity for assignee pickers (ADR 0014).
type Person struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Aliases []string `json:"aliases"`
}

// ProjectEntry is one row of the project sheet. Slot is its number key, 1-9,
// or 0 beyond the ninth.
type ProjectEntry struct {
	Slot       int    `json:"slot"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	Location   string `json:"location"`
	Active     bool   `json:"active"`
	Reach      string `json:"reach"`
	Open       int    `json:"open"`
	InProgress int    `json:"in_progress"`
	Ready      int    `json:"ready"`
	Blocked    int    `json:"blocked"`
}

// ProjectList is the project sheet.
type ProjectList struct {
	Projects []ProjectEntry `json:"projects"`
}

// OpenProjectRequest names the project to open by its key, or "*" for the
// read-only all-projects view.
type OpenProjectRequest struct {
	Key string `json:"key"`
}

// QueryResult lists the IDs that match a query in the current snapshot.
type QueryResult struct {
	Version uint64   `json:"version"`
	IDs     []string `json:"ids"`
	Error   string   `json:"error"`
}

// WriteOp is one of the writes the browser may request. Each maps to one bd
// command through the TUI's IssueWriter.
type WriteOp string

const (
	OpStatus  WriteOp = "status"
	OpClose   WriteOp = "close"
	OpDelete  WriteOp = "delete"
	OpUpdate  WriteOp = "update"
	OpCreate  WriteOp = "create"
	OpComment WriteOp = "comment"
	OpDefer   WriteOp = "defer"
)

// WriteRequest is a write. IDs holds the marked issues, or the one under the
// cursor (ADR 0011). Key makes a retried request return the first result
// instead of running bd twice.
type WriteRequest struct {
	Op     WriteOp           `json:"op"`
	IDs    []string          `json:"ids,omitempty"`
	Status string            `json:"status,omitempty"`
	Reason string            `json:"reason,omitempty"`
	Until  string            `json:"until,omitempty"`
	Text   string            `json:"text,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
	Key    string            `json:"key,omitempty"`
}

// WriteResult reports a write. Error carries bd's own message on failure.
type WriteResult struct {
	OK      bool     `json:"ok"`
	IDs     []string `json:"ids"`
	Created string   `json:"created"`
	Error   string   `json:"error"`
	Output  string   `json:"output"`
	Version uint64   `json:"version"`
}

// Session tells a paired browser the CSRF value every write must echo, and
// the query it starts with.
type Session struct {
	CSRF  string `json:"csrf"`
	Query string `json:"query"`
	// Public is true on a server anyone may open (b9s web --public). The
	// browser then offers no project switching.
	Public bool `json:"public"`
	// Banner is a short line to show above the board, or "".
	Banner string `json:"banner"`
	// BannerLink is an http(s) URL the banner links to, or "".
	BannerLink string `json:"banner_link"`
	// Version names the running b9s build for people to read: a release tag,
	// "dev after <tag>" or "dev". The bundle is embedded in the binary, so it
	// is also the web app's version.
	Version string `json:"version"`
	// Commit is the full commit hash the build came from, or "".
	Commit string `json:"commit"`
	// CommitURL links to Commit on GitHub, or is "".
	CommitURL string `json:"commit_url"`
	// Modified is true when the build had uncommitted changes.
	Modified bool `json:"modified"`
}

// Event is one Server-Sent Event: "hello" on connect, "changed" after the
// project's data changed, "project" after another project was opened, and
// "assets" after the SPA files changed under b9s web --dev-assets.
type Event struct {
	Type    string `json:"type"`
	Version uint64 `json:"version"`
}

// AgentSession is one coding-agent conversation, as the fleet views show it.
// Times are RFC 3339 in UTC, like the issue fields.
type AgentSession struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Tool           string `json:"tool"`
	Project        string `json:"project"`
	Cwd            string `json:"cwd"`
	Branch         string `json:"branch"`
	Role           string `json:"role"`
	TaskID         string `json:"task_id"`
	State          string `json:"state"`
	Model          string `json:"model"`
	ContextTokens  int    `json:"context_tokens"`
	ContextLimit   int    `json:"context_limit"`
	ContextPercent int    `json:"context_percent"`
	TokensPerMin   int    `json:"tokens_per_min"`
	TotalTokens    int    `json:"total_tokens"`
	Compactions    int    `json:"compactions"`
	Started        string `json:"started"`
	LastActivity   string `json:"last_activity"`
	// WhereKind is "tmux", "pod" or "" for a session no process hosts.
	WhereKind  string `json:"where_kind"`
	WhereLabel string `json:"where_label"`
	ToolCalls  int    `json:"tool_calls"`
	Errors     int    `json:"errors"`
	// Live is false only for a finished session.
	Live bool `json:"live"`
}

// AgentsResponse is GET api/agents: every session of the last scan, live
// ones first.
type AgentsResponse struct {
	At       string         `json:"at"`
	Sessions []AgentSession `json:"sessions"`
	// PodError says why lane pods are missing from the list, or is "".
	PodError string `json:"pod_error"`
}

// AgentEvent is one line of an agent's log.
type AgentEvent struct {
	Time   string `json:"time"`
	Kind   string `json:"kind"`
	Tool   string `json:"tool"`
	Text   string `json:"text"`
	Detail string `json:"detail"`
}

// AgentLogResponse is GET api/agent/log.
type AgentLogResponse struct {
	ID     string       `json:"id"`
	Events []AgentEvent `json:"events"`
}

// ContextSource is one thing loaded into a session's context window.
type ContextSource struct {
	Layer     string `json:"layer"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Scope     string `json:"scope"`
	Tokens    int    `json:"tokens"`
	LoadedAt  string `json:"loaded_at"`
	ChangedAt string `json:"changed_at"`
	// Stale is true when the file changed after the session loaded it.
	Stale bool `json:"stale"`
}

// ContextLayer is the token estimate of one layer of a context window.
type ContextLayer struct {
	Layer  string `json:"layer"`
	Tokens int    `json:"tokens"`
}

// AgentContext is GET api/agent/context. Window is the size measured at the
// last model call, and the layer and source numbers are estimates.
type AgentContext struct {
	SessionID string          `json:"session_id"`
	Window    int             `json:"window"`
	Limit     int             `json:"limit"`
	Layers    []ContextLayer  `json:"layers"`
	Sources   []ContextSource `json:"sources"`
}

// FleetCard is one issue (or one agent without a known issue) in a stage cell.
type FleetCard struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	// Agents lists the IDs of the live sessions working on the card.
	Agents []string `json:"agents"`
}

// FleetStage is one cell of the fleet board. Count is the number of issues
// in the stage, of which Cards lists the ones the server could name.
type FleetStage struct {
	Stage string      `json:"stage"`
	Count int         `json:"count"`
	Cards []FleetCard `json:"cards"`
}

// FleetProject is one row of the fleet board. Current marks the project on
// screen, the only one whose issues are loaded, so only its stages carry
// issue cards.
type FleetProject struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Current     bool         `json:"current"`
	Reach       string       `json:"reach"`
	Open        int          `json:"open"`
	InProgress  int          `json:"in_progress"`
	Ready       int          `json:"ready"`
	Blocked     int          `json:"blocked"`
	ClosedToday int          `json:"closed_today"`
	LiveAgents  int          `json:"live_agents"`
	Stages      []FleetStage `json:"stages"`
}

// FleetResponse is GET api/fleet.
type FleetResponse struct {
	At       string         `json:"at"`
	Projects []FleetProject `json:"projects"`
	Sessions []AgentSession `json:"sessions"`
	PodError string         `json:"pod_error"`
}

// MatrixSession is one column of the context matrix.
type MatrixSession struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Project string `json:"project"`
	State   string `json:"state"`
	Tool    string `json:"tool"`
}

// MatrixCell is one session's share of a context source. Loaded is false
// when the session does not hold the source.
type MatrixCell struct {
	Loaded bool `json:"loaded"`
	Tokens int  `json:"tokens"`
	Stale  bool `json:"stale"`
}

// MatrixRow is one context source across the sessions. Cells follow the
// order of MatrixResponse.Sessions.
type MatrixRow struct {
	Key   string `json:"key"`
	Layer string `json:"layer"`
	Name  string `json:"name"`
	Path  string `json:"path"`
	Scope string `json:"scope"`
	// Aggregate is true for a layer the sessions fill without a named
	// source, such as tool output.
	Aggregate bool         `json:"aggregate"`
	Cells     []MatrixCell `json:"cells"`
	Sessions  int          `json:"sessions"`
	Stale     int          `json:"stale"`
	Tokens    int          `json:"tokens"`
}

// Insight is one plain-language finding under a fleet view.
type Insight struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// MatrixResponse is GET api/context-matrix, built from the live sessions.
type MatrixResponse struct {
	At       string          `json:"at"`
	Sessions []MatrixSession `json:"sessions"`
	Rows     []MatrixRow     `json:"rows"`
	Layers   []ContextLayer  `json:"layers"`
	Insights []Insight       `json:"insights"`
}

// TimelineSegment is a stretch of one session's time in one phase: plan,
// implement, track (bd calls), fail or idle.
type TimelineSegment struct {
	Phase string `json:"phase"`
	Start string `json:"start"`
	End   string `json:"end"`
	Label string `json:"label"`
}

// TimelineMarker is a point event on a lane: a bd call, an error, a
// compaction or a commit.
type TimelineMarker struct {
	Time  string `json:"time"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// TimelineLane is one session's row of the timeline.
type TimelineLane struct {
	SessionID string            `json:"session_id"`
	Name      string            `json:"name"`
	Project   string            `json:"project"`
	State     string            `json:"state"`
	Segments  []TimelineSegment `json:"segments"`
	Markers   []TimelineMarker  `json:"markers"`
}

// TimelineResponse is GET api/timeline.
type TimelineResponse struct {
	From  string         `json:"from"`
	To    string         `json:"to"`
	Hours int            `json:"hours"`
	Lanes []TimelineLane `json:"lanes"`
}

// CommitInfo is one commit of the open project's repository.
type CommitInfo struct {
	Hash    string   `json:"hash"`
	Short   string   `json:"short"`
	Time    string   `json:"time"`
	Author  string   `json:"author"`
	Subject string   `json:"subject"`
	Refs    []string `json:"refs"`
	// Issues lists the Beads IDs the commit's Refs: trailer names.
	Issues []string `json:"issues"`
}

// CommitsResponse is GET api/commits.
type CommitsResponse struct {
	Repo    string       `json:"repo"`
	Commits []CommitInfo `json:"commits"`
}
