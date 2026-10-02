package datasource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanderheijden86/b9s/internal/bdrun"
)

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

// OpenGraphPreview checks that projectDir is a ready graph workspace and finds
// the bd binary to read it with.
func OpenGraphPreview(projectDir string) (*GraphPreviewClient, error) {
	bd, ok := bdrun.Resolve()
	if !ok {
		return nil, ErrBDNotFound
	}
	return newGraphPreviewClient(projectDir, bd)
}

func newGraphPreviewClient(projectDir, bd string) (*GraphPreviewClient, error) {
	beadsDir := filepath.Join(projectDir, ".beads")
	metadata, err := os.ReadFile(filepath.Join(beadsDir, "metadata.json"))
	if err != nil {
		return nil, fmt.Errorf("read graph workspace metadata: %w", err)
	}
	var meta struct {
		GraphMode  string `json:"graph_mode"`
		GraphReady bool   `json:"graph_ready"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return nil, fmt.Errorf("parse graph workspace metadata: %w", err)
	}
	if meta.GraphMode != "link" || !meta.GraphReady {
		return nil, fmt.Errorf("%s is not a ready Memory Beads graph workspace", projectDir)
	}
	return &GraphPreviewClient{projectDir: projectDir, beadsDir: beadsDir, bd: bd}, nil
}

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
