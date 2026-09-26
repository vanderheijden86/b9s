package web

// These types are the JSON contract between b9s web and the browser. The
// TypeScript in web/src/api.gen.ts is generated from them (see tsgen_test.go), so a
// change here that is not regenerated fails TestGeneratedTypesAreCurrent.

// Issue is one issue as the browser shows it. Blocked, Ready and ClosedLike
// are derived on the server with the TUI's rules, so both UIs count alike.
type Issue struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Design         string    `json:"design"`
	Acceptance     string    `json:"acceptance"`
	Notes          string    `json:"notes"`
	Status         string    `json:"status"`
	Priority       int       `json:"priority"`
	Type           string    `json:"type"`
	Assignee       string    `json:"assignee"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
	ClosedAt       string    `json:"closed_at"`
	DeferUntil     string    `json:"defer_until"`
	Labels         []string  `json:"labels"`
	Parent         string    `json:"parent"`
	BlockedBy      []string  `json:"blocked_by"`
	Related        []string  `json:"related"`
	DiscoveredFrom []string  `json:"discovered_from"`
	Comments       []Comment `json:"comments"`
	Project        string    `json:"project"`
	Blocked        bool      `json:"blocked"`
	Ready          bool      `json:"ready"`
	ClosedLike     bool      `json:"closed_like"`
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
}

// Event is one Server-Sent Event: "hello" on connect, "changed" after the
// project's data changed, "project" after another project was opened.
type Event struct {
	Type    string `json:"type"`
	Version uint64 `json:"version"`
}
