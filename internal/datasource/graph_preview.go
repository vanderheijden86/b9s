package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

// LoadGraphPreview invokes the matching bd binary for a new graph workspace.
// It does not open the preview's Dolt store or alter its contents.
func LoadGraphPreview(projectDir string) (GraphPreview, error) {
	beadsDir := filepath.Join(projectDir, ".beads")
	metadata, err := os.ReadFile(filepath.Join(beadsDir, "metadata.json"))
	if err != nil {
		return GraphPreview{}, fmt.Errorf("read graph workspace metadata: %w", err)
	}
	var meta struct {
		GraphMode  string `json:"graph_mode"`
		GraphReady bool   `json:"graph_ready"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return GraphPreview{}, fmt.Errorf("parse graph workspace metadata: %w", err)
	}
	if meta.GraphMode != "link" || !meta.GraphReady {
		return GraphPreview{}, fmt.Errorf("%s is not a ready Memory Beads graph workspace", projectDir)
	}
	bd, ok := bdrun.Resolve()
	if !ok {
		return GraphPreview{}, ErrBDNotFound
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, stderr, err := bdrun.Output(ctx, bd, projectDir, []string{"BEADS_DIR=" + beadsDir}, "list", "--format", "records-json", "--all")
	if err != nil {
		return GraphPreview{}, fmt.Errorf("bd graph list: %w: %s", err, stderr)
	}
	graph, err := ParseGraphPreview(strings.NewReader(string(out)))
	if err != nil {
		return GraphPreview{}, err
	}
	links := make(map[string]GraphLink)
	for _, link := range graph.Links {
		links[link.ID] = link
	}
	for _, bead := range graph.Beads {
		if bead.Kind != "memory" {
			continue
		}
		out, stderr, err := bdrun.Output(ctx, bd, projectDir, []string{"BEADS_DIR=" + beadsDir}, "links", bead.ID, "--json")
		if err != nil {
			return GraphPreview{}, fmt.Errorf("bd links for %s: %w: %s", bead.ID, err, stderr)
		}
		incident, err := ParseGraphPreviewLinks(strings.NewReader(string(out)))
		if err != nil {
			return GraphPreview{}, err
		}
		for _, link := range incident {
			links[link.ID] = link
		}
	}
	graph.Links = graph.Links[:0]
	for _, link := range links {
		graph.Links = append(graph.Links, link)
	}
	sort.Slice(graph.Links, func(i, j int) bool { return graph.Links[i].ID < graph.Links[j].ID })
	return graph, nil
}
