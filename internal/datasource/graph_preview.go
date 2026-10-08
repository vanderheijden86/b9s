package datasource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanderheijden86/b9s/internal/bdrun"
	"github.com/vanderheijden86/b9s/pkg/debug"
)

// ErrBDNotGraphPreview identifies missing commands, independent of release names.
var ErrBDNotGraphPreview = errors.New("this bd has no compatible Memory Beads graph commands; use a bd installation with Memory support on PATH")

var ErrMemoryNotEnabled = errors.New("not a ready Memory Beads graph workspace")

// ExplainMemoryFailure uses explicit causes first, then recognizable CLI diagnostics.
// The original diagnostic remains available when the evidence is inconclusive.
func ExplainMemoryFailure(err error) string {
	if err == nil {
		return ""
	}
	category, hint := "Memory read failed", "Check the details below and retry."
	detail := strings.ToLower(err.Error())
	var syntax *json.SyntaxError
	switch {
	case errors.Is(err, ErrBDNotFound):
		category, hint = "bd missing", "Put the project's bd installation on PATH."
	case errors.Is(err, ErrMemoryNotEnabled), errors.Is(err, os.ErrNotExist):
		category, hint = "Memories not enabled", "This project needs a ready Memory graph workspace."
	case errors.Is(err, ErrBDNotGraphPreview), strings.Contains(detail, "unknown command"), strings.Contains(detail, "unknown flag"):
		category, hint = "Memory commands unavailable", "Use the project's bd installation with Memory support."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, bdrun.ErrTimeout):
		category, hint = "Read timed out", "Check the database connection and retry."
	case errors.Is(err, os.ErrPermission), strings.Contains(detail, "access denied"), strings.Contains(detail, "permission denied"), strings.Contains(detail, "authentication failed"):
		category, hint = "Access denied", "Check this project's database credentials and read permissions."
	case strings.Contains(detail, "schema version mismatch"):
		category, hint = "Beads schema mismatch", "Use a bd binary compatible with this project's database schema."
	case strings.Contains(detail, "connection refused"), strings.Contains(detail, "unreachable"), strings.Contains(detail, "no such host"):
		category, hint = "Database unreachable", "Check the project's database connection or tunnel."
	case errors.As(err, &syntax), strings.Contains(detail, "did not return graph preview"), strings.Contains(detail, "exceeds the CLI limit"):
		category, hint = "Unsupported Memory response", "Check that bd returns a complete, compatible Memory graph."
	}
	return category + ": " + hint + "\nDetails: " + err.Error()
}

// GraphPreview is the current, bounded Bead inventory returned by the preview CLI.
type GraphPreview struct {
	Beads []GraphBead
	Links []GraphLink
}

type GraphBead struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    string          `json:"version"`
	Properties GraphProperties `json:"properties"`
	Owned      []GraphLink     `json:"owned"`
	Kind       string          `json:"-"`
	// rawProperties keeps every property, including Issue fields that
	// GraphProperties does not name.
	rawProperties json.RawMessage
}

type GraphProperties struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type GraphLink struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Source     string          `json:"source"`
	Target     string          `json:"target"`
	Properties GraphProperties `json:"properties"`
}

// ParseGraphPreview rejects partial snapshots because a missing edge would make
// the displayed graph misleading. The preview CLI exposes no continuation cursor.
func ParseGraphPreview(r io.Reader) (GraphPreview, error) {
	var payload struct {
		Preview bool `json:"preview"`
		Result  struct {
			Items   []GraphBead `json:"items"`
			HasMore bool        `json:"hasMore"`
		} `json:"result"`
	}
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return GraphPreview{}, fmt.Errorf("parse graph preview: %w", err)
	}
	if !payload.Preview {
		return GraphPreview{}, fmt.Errorf("bd did not return graph preview records")
	}
	if payload.Result.HasMore {
		return GraphPreview{}, fmt.Errorf("graph preview inventory exceeds the CLI limit; no complete graph is available")
	}
	for i := range payload.Result.Items {
		bead := &payload.Result.Items[i]
		switch {
		case strings.HasSuffix(bead.Type, "/preview-memory-v2"):
			bead.Kind = "memory"
		case strings.HasSuffix(bead.Type, "/preview-issue-v2"):
			bead.Kind = "issue"
		default:
			bead.Kind = "other"
		}
	}
	graph := GraphPreview{Beads: payload.Result.Items}
	for _, bead := range graph.Beads {
		graph.Links = append(graph.Links, bead.Owned...)
	}
	return graph, nil
}

func ParseGraphPreviewLinks(r io.Reader) ([]GraphLink, error) {
	var payload struct {
		Preview bool        `json:"preview"`
		Result  []GraphLink `json:"result"`
	}
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse graph preview links: %w", err)
	}
	if !payload.Preview {
		return nil, fmt.Errorf("bd did not return graph preview links")
	}
	return payload.Result, nil
}

// GraphPreviewClient runs read-only preview bd commands against one graph
// workspace. It never opens the preview's Dolt store, which an open embedded
// store would lock against bd itself.
type GraphPreviewClient struct {
	projectDir string
	beadsDir   string
	bd         string
	// traversalWorkers bounds concurrent traversals in Graph; zero means
	// defaultTraversalWorkers.
	traversalWorkers int
}

// MemorySummary is one compact result of bd memories. Search is a literal
// title and body match, so a summary carries which fields matched.
type MemorySummary struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Version       string   `json:"version"`
	MatchedFields []string `json:"matchedFields"`
	Excerpt       struct {
		Field     string `json:"field"`
		Text      string `json:"text"`
		Truncated bool   `json:"truncated"`
	} `json:"excerpt"`
	Details struct {
		OwnedLinkCount int `json:"ownedLinkCount"`
	} `json:"details"`
}

// GraphVersion is one retained version of a graph Resource. The token is the
// only stable reference; the ordinal orders versions within this store.
type GraphVersion struct {
	Version  string    `json:"version"`
	Ordinal  int       `json:"ordinal"`
	ChangeAt time.Time `json:"change_at"`
	Actor    string    `json:"actor"`
	Removed  bool      `json:"removed"`
}

// maxGraphSchemaVersion is the newest graph workspace format the preview-v2
// reader understands. A newer workspace may encode Types or Links this reader
// would misdraw, so it reads as not enabled rather than as a parse failure.
const maxGraphSchemaVersion = 6

// MemoryState says whether Memory views can open for a project.
type MemoryState string

const (
	// MemoryReady: the workspace is a ready graph workspace and the bd on
	// PATH speaks a Memory dialect b9s reads.
	MemoryReady MemoryState = "ready"
	// MemoryUnsupported: the bd on PATH is missing or has no Memory dialect.
	MemoryUnsupported MemoryState = "unsupported"
	// MemoryNotEnabled: the project is not a ready graph workspace.
	MemoryNotEnabled MemoryState = "not_enabled"
	// MemoryOff: the user turned Memory views off with MemoryLeverEnv.
	MemoryOff MemoryState = "off"
)

// MemoryLeverEnv turns Memory views off when set to "off". It can only
// subtract: no value forces Memory on against a failing detection.
const MemoryLeverEnv = "B9S_MEMORY"

// MemoryCapability is the outcome of DetectMemory. Err is nil only when
// State is MemoryReady, and ExplainMemoryFailure turns it into the reason.
type MemoryCapability struct {
	State   MemoryState
	Dialect bdrun.MemoryDialect
	BD      string
	Err     error
}

// MemoryUnavailableError is DetectMemory's refusal. Reason is one line for a
// status bar; the wrapped cause keeps errors.Is and the full diagnostic.
type MemoryUnavailableError struct {
	Reason string
	cause  error
}

func (e *MemoryUnavailableError) Error() string { return e.cause.Error() }
func (e *MemoryUnavailableError) Unwrap() error { return e.cause }

func unavailable(reason string, cause error) error {
	return &MemoryUnavailableError{Reason: reason, cause: cause}
}

// MemoryUnavailableReason returns the one-line reason when err is a
// DetectMemory refusal, and false for any other error, such as a failed read.
func MemoryUnavailableReason(err error) (string, bool) {
	var refusal *MemoryUnavailableError
	if errors.As(err, &refusal) {
		return refusal.Reason, true
	}
	return "", false
}

// DetectMemory decides whether Memory views can open for projectDir. The
// workspace check comes first because it reads one small file: an ordinary
// project, the common case, never runs bd. The bd check reads only help
// output, cached per binary (ADR 0044, docs/plans/2026-10-07-memory-capability-gating.md).
func DetectMemory(ctx context.Context, projectDir string) MemoryCapability {
	capability := detectMemory(ctx, projectDir)
	reason, _ := MemoryUnavailableReason(capability.Err)
	debug.Log("memory: %s state=%s dialect=%s bd=%s reason=%q", projectDir, capability.State, capability.Dialect, capability.BD, reason)
	return capability
}

func detectMemory(ctx context.Context, projectDir string) MemoryCapability {
	if memoryTurnedOff() {
		return MemoryCapability{State: MemoryOff, Err: unavailable("Memory views are turned off ("+MemoryLeverEnv+"=off)", ErrMemoryNotEnabled)}
	}
	return detectGraphReadable(ctx, projectDir)
}

func memoryTurnedOff() bool { return os.Getenv(MemoryLeverEnv) == "off" }

// detectGraphReadable is DetectMemory without the lever. The loader needs it:
// a graph workspace refuses bd export, so its Issues come only from the graph
// read, and turning the Memory views off must not take the Issues with them.
func detectGraphReadable(ctx context.Context, projectDir string) MemoryCapability {
	if err := checkGraphWorkspace(projectDir); err != nil {
		reason := "this project is not a Memory graph workspace"
		var schema *graphSchemaError
		if errors.As(err, &schema) {
			reason = fmt.Sprintf("this project uses graph schema %d, newer than this b9s reads", schema.version)
		}
		return MemoryCapability{State: MemoryNotEnabled, Err: unavailable(reason, err)}
	}
	bd, ok := bdrun.Resolve()
	if !ok {
		return MemoryCapability{State: MemoryUnsupported, Err: unavailable("bd is not on PATH", ErrBDNotFound)}
	}
	dialect, _ := bdrun.ProbeMemoryCached(ctx, bd, bdrun.DefaultProbeCacheFile())
	if dialect != bdrun.MemoryDialectPreviewV2 {
		return MemoryCapability{State: MemoryUnsupported, Dialect: dialect, BD: bd,
			Err: unavailable("bd at "+bd+" has no Memory Beads support", fmt.Errorf("%s: %w", bd, ErrBDNotGraphPreview))}
	}
	return MemoryCapability{State: MemoryReady, Dialect: dialect, BD: bd}
}

// OpenGraphPreview opens a reader for projectDir when DetectMemory finds it
// ready, and returns DetectMemory's reason otherwise.
func OpenGraphPreview(projectDir string) (*GraphPreviewClient, error) {
	capability := DetectMemory(context.Background(), projectDir)
	if capability.Err != nil {
		return nil, capability.Err
	}
	return newGraphPreviewClient(projectDir, capability.BD)
}

func newGraphPreviewClient(projectDir, bd string) (*GraphPreviewClient, error) {
	if err := checkGraphWorkspace(projectDir); err != nil {
		return nil, err
	}
	return &GraphPreviewClient{projectDir: projectDir, beadsDir: filepath.Join(projectDir, ".beads"), bd: bd}, nil
}

// checkGraphWorkspace reports, from .beads/metadata.json alone, whether
// projectDir is a ready graph workspace in a format this reader understands.
func checkGraphWorkspace(projectDir string) error {
	metadata, err := os.ReadFile(filepath.Join(projectDir, ".beads", "metadata.json"))
	if err != nil {
		return fmt.Errorf("read graph workspace metadata: %w", err)
	}
	var meta struct {
		GraphMode          string `json:"graph_mode"`
		GraphReady         bool   `json:"graph_ready"`
		GraphSchemaVersion int    `json:"graph_schema_version"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return fmt.Errorf("parse graph workspace metadata: %w", err)
	}
	if meta.GraphMode != "link" || !meta.GraphReady {
		return fmt.Errorf("%s is %w", projectDir, ErrMemoryNotEnabled)
	}
	if meta.GraphSchemaVersion > maxGraphSchemaVersion {
		return &graphSchemaError{project: projectDir, version: meta.GraphSchemaVersion}
	}
	return nil
}

type graphSchemaError struct {
	project string
	version int
}

func (e *graphSchemaError) Error() string {
	return fmt.Sprintf("%s uses graph schema %d, newer than %d this b9s reads: %v",
		e.project, e.version, maxGraphSchemaVersion, ErrMemoryNotEnabled)
}

func (e *graphSchemaError) Unwrap() error { return ErrMemoryNotEnabled }

// run returns bd's stdout. A refused preview read prints a JSON error with a
// stable code on stdout and exits non-zero; that code is the useful part of
// the failure, so it leads the returned error.
func (c *GraphPreviewClient) run(ctx context.Context, args ...string) ([]byte, error) {
	out, stderr, err := bdrun.Output(ctx, c.bd, c.projectDir, []string{"BEADS_DIR=" + c.beadsDir}, args...)
	if err == nil {
		return out, nil
	}
	var refusal struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(out, &refusal) == nil && refusal.Code != "" {
		return nil, fmt.Errorf("bd %s: %s: %s", args[0], refusal.Code, refusal.Message)
	}
	if strings.Contains(stderr, "unknown flag") || strings.Contains(stderr, "unknown command") {
		return nil, fmt.Errorf("%s: %w", c.bd, ErrBDNotGraphPreview)
	}
	return nil, fmt.Errorf("bd %s: %w: %s", args[0], err, stderr)
}

// Inventory returns every current Bead with its owned Links. Incoming Links
// owned by other Beads appear only on their owners; use Links for one Bead.
func (c *GraphPreviewClient) Inventory(ctx context.Context) (GraphPreview, error) {
	out, err := c.run(ctx, "list", "--format", "records-json", "--all")
	if err != nil {
		return GraphPreview{}, err
	}
	return ParseGraphPreview(bytes.NewReader(out))
}

// SearchMemories lists Memory summaries whose title or body contains query
// literally. An empty query lists every Memory.
func (c *GraphPreviewClient) SearchMemories(ctx context.Context, query string) ([]MemorySummary, error) {
	args := []string{"memories"}
	if query != "" {
		args = append(args, query)
	}
	out, err := c.run(ctx, append(args, "--all", "--details", "--format", "records-json")...)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Preview bool `json:"preview"`
		Result  struct {
			Items   []MemorySummary `json:"items"`
			HasMore bool            `json:"hasMore"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse graph memory summaries: %w", err)
	}
	if !payload.Preview {
		return nil, fmt.Errorf("bd did not return graph memory summaries")
	}
	if payload.Result.HasMore {
		return nil, fmt.Errorf("graph memory search exceeds the CLI limit; no complete result is available")
	}
	return payload.Result.Items, nil
}

// Memory reads one Bead record: the current version when version is empty,
// otherwise that exact retained version. Owned holds the outgoing Links as
// of the version read.
func (c *GraphPreviewClient) Memory(ctx context.Context, id, version string) (GraphBead, error) {
	args := []string{"show", id, "--json"}
	if version != "" {
		args = append(args, "--version", version)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return GraphBead{}, err
	}
	var payload struct {
		Preview bool      `json:"preview"`
		Result  GraphBead `json:"result"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return GraphBead{}, fmt.Errorf("parse graph record: %w", err)
	}
	if !payload.Preview {
		return GraphBead{}, fmt.Errorf("bd did not return a graph record")
	}
	return payload.Result, nil
}

// Links returns the current Links in both directions for one Bead.
func (c *GraphPreviewClient) Links(ctx context.Context, id string) ([]GraphLink, error) {
	out, err := c.run(ctx, "links", id, "--json")
	if err != nil {
		return nil, err
	}
	return ParseGraphPreviewLinks(bytes.NewReader(out))
}

// Versions returns the retained versions of one Resource, newest first.
func (c *GraphPreviewClient) Versions(ctx context.Context, id string) ([]GraphVersion, error) {
	out, err := c.run(ctx, "versions", id, "--json")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Preview bool `json:"preview"`
		Result  struct {
			Versions []GraphVersion `json:"versions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse graph versions: %w", err)
	}
	if !payload.Preview {
		return nil, fmt.Errorf("bd did not return graph versions")
	}
	return payload.Result.Versions, nil
}
