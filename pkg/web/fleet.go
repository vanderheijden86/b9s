package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/pkg/ui"
)

// Fleet board stages, left to right.
const (
	stageReady      = "ready"
	stageInProgress = "in_progress"
	stageReview     = "review"
	stageTest       = "test"
	stageDone       = "done"
)

var stageOrder = []string{stageReady, stageInProgress, stageReview, stageTest, stageDone}

const (
	// maxReadyCards and maxProgressCards bound the cards one cell lists. The
	// cell's count still covers every issue.
	maxReadyCards    = 6
	maxProgressCards = 12
	// probeTTL is how long the per-project counts of the other projects are
	// reused: probing opens every project's database.
	probeTTL = 15 * time.Second
)

func (s *Server) now() time.Time {
	if s.opts.Now != nil {
		return s.opts.Now()
	}
	return time.Now()
}

// projectProbe caches ui.ProbeProjects, which reads every project.
type projectProbe struct {
	mu     sync.Mutex
	at     time.Time
	counts map[string]ui.ProjectCounts
	reach  map[string]string
}

func (s *Server) probe() (map[string]ui.ProjectCounts, map[string]string) {
	p := &s.probed
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.counts != nil && time.Since(p.at) < probeTTL {
		return p.counts, p.reach
	}
	counts, reach := ui.ProbeProjects(s.opts.Projects(), s.opts.StartupUser)
	p.counts, p.reach, p.at = counts, map[string]string{}, time.Now()
	for k, v := range reach {
		p.reach[k] = v.String()
	}
	return p.counts, p.reach
}

// fleetRow is a project row under construction.
type fleetRow struct {
	FleetProject
	issues   []Issue
	sessions []agents.Session
}

func (s *Server) handleFleet(w http.ResponseWriter, r *http.Request) {
	snap, _ := s.feed.snapshot(r.Context())
	writeJSON(w, r, FleetResponse{
		At:       formatTime(snap.At),
		Projects: s.buildFleet(snap.Sessions),
		Sessions: convertSessions(snap.Sessions),
		PodError: snap.PodError,
	})
}

// buildFleet joins three sources: the project list (counts from a cached
// probe), the open project's issues (exact, live counts and cards) and the
// agent sessions (matched to a project by name).
func (s *Server) buildFleet(sessions []agents.Session) []FleetProject {
	var rows []*fleetRow
	byName := map[string]*fleetRow{}
	ensure := func(key, name string) *fleetRow {
		if row, ok := byName[strings.ToLower(name)]; ok {
			if row.Key == "" {
				row.Key = key
			}
			return row
		}
		row := &fleetRow{FleetProject: FleetProject{Key: key, Name: name, Reach: "unknown"}}
		byName[strings.ToLower(name)] = row
		rows = append(rows, row)
		return row
	}

	if !s.opts.Public {
		projects := s.opts.Projects()
		if len(projects) > 0 {
			counts, reach := s.probe()
			for _, p := range projects {
				key := ui.ProjectKey(p)
				row := ensure(key, p.Name)
				if c, ok := counts[key]; ok {
					row.Open, row.InProgress, row.Ready, row.Blocked = c.Open, c.InProgress, c.Ready, c.Blocked
				}
				if v, ok := reach[key]; ok {
					row.Reach = v
				}
			}
		}
	}

	info := s.opts.Store.Info()
	loaded, _ := s.opts.Store.Issues()
	issues := convertIssues(loaded)
	if info.All {
		for _, issue := range issues {
			name := issue.Project
			if name == "" {
				name = "unknown"
			}
			row := ensure("", name)
			row.issues = append(row.issues, issue)
		}
	} else if info.Name != "" {
		row := ensure(info.Key, info.Name)
		row.Current, row.Reach, row.issues = true, "ok", issues
	}

	for _, sess := range sessions {
		if !sess.Live() {
			continue
		}
		name := sess.Project
		if name == "" {
			name = filepath.Base(sess.Cwd)
		}
		row := ensure("", name)
		row.sessions = append(row.sessions, sess)
	}

	today := s.now().Local()
	out := make([]FleetProject, 0, len(rows))
	for _, row := range rows {
		if row.issues != nil {
			row.Open, row.InProgress, row.Ready, row.Blocked = countIssues(row.issues)
		}
		row.LiveAgents = len(row.sessions)
		row.Stages, row.ClosedToday = buildStages(row.issues, row.sessions, row.Ready, today)
		out = append(out, row.FleetProject)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.LiveAgents > 0) != (b.LiveAgents > 0) {
			return a.LiveAgents > 0
		}
		if a.Open != b.Open {
			return a.Open > b.Open
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return out
}

// countIssues uses the rules of the project sheet: open counts every issue
// that is not closed, ready only issues with status open that nothing blocks.
func countIssues(issues []Issue) (open, inProgress, ready, blocked int) {
	for _, issue := range issues {
		if issue.ClosedLike {
			continue
		}
		open++
		if issue.Status == "in_progress" {
			inProgress++
		}
		if issue.Blocked {
			blocked++
		}
		if issue.Ready && issue.Status == "open" {
			ready++
		}
	}
	return
}

// roleStage maps a lane role to the stage its agent works in.
func roleStage(role string) string {
	r := strings.ToLower(role)
	switch {
	case strings.Contains(r, "review"):
		return stageReview
	case strings.Contains(r, "test"), strings.Contains(r, "deploy"):
		return stageTest
	}
	return stageInProgress
}

func closedOn(issue Issue, day time.Time) bool {
	if !issue.ClosedLike || issue.ClosedAt == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, issue.ClosedAt)
	if err != nil {
		return false
	}
	at = at.In(day.Location())
	return at.Year() == day.Year() && at.YearDay() == day.YearDay()
}

// buildStages places a project's issues and live sessions on the board. A
// project without loaded issues still shows its ready count, and its
// sessions appear as cards named after the session.
func buildStages(issues []Issue, sessions []agents.Session, readyCount int, today time.Time) ([]FleetStage, int) {
	cells := map[string]*FleetStage{}
	for _, name := range stageOrder {
		cells[name] = &FleetStage{Stage: name, Cards: []FleetCard{}}
	}
	if len(issues) == 0 {
		cells[stageReady].Count = readyCount
	}

	byTask := map[string][]agents.Session{}
	for _, sess := range sessions {
		if sess.TaskID != "" {
			byTask[sess.TaskID] = append(byTask[sess.TaskID], sess)
		}
	}
	placed := map[string]bool{}
	closedToday := 0
	sorted := append([]Issue(nil), issues...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sorted[i].ID < sorted[j].ID
	})
	for _, issue := range sorted {
		working := byTask[issue.ID]
		stage := ""
		switch {
		case closedOn(issue, today):
			stage = stageDone
			closedToday++
		case issue.ClosedLike:
		case len(working) > 0:
			stage = roleStage(working[0].Role)
		case issue.Status == "review":
			stage = stageReview
		case issue.Status == "in_progress":
			stage = stageInProgress
		case issue.Ready && issue.Status == "open":
			stage = stageReady
		}
		if stage == "" {
			continue
		}
		cell := cells[stage]
		cell.Count++
		limit := maxProgressCards
		if stage == stageReady {
			limit = maxReadyCards
		}
		if len(working) == 0 && len(cell.Cards) >= limit {
			continue
		}
		card := FleetCard{ID: issue.ID, Title: issue.Title, Status: issue.Status, Agents: []string{}}
		for _, sess := range working {
			card.Agents = append(card.Agents, sess.ID)
			placed[sess.ID] = true
		}
		cell.Cards = append(cell.Cards, card)
	}
	for _, sess := range sessions {
		if placed[sess.ID] {
			continue
		}
		cell := cells[roleStage(sess.Role)]
		cell.Cards = append(cell.Cards, FleetCard{ID: sess.TaskID, Title: sess.Name, Agents: []string{sess.ID}})
	}
	out := make([]FleetStage, len(stageOrder))
	for i, name := range stageOrder {
		out[i] = *cells[name]
	}
	return out, closedToday
}

// aggregateLayers are the layers a session fills without naming a source.
// Each gets one row per layer, so the matrix shows where the window goes as well as what was loaded.
var aggregateLayers = map[agents.Layer]bool{
	agents.LayerToolOutput: true, agents.LayerConversation: true, agents.LayerSystem: true,
}

func (s *Server) handleContextMatrix(w http.ResponseWriter, r *http.Request) {
	snap, src := s.feed.snapshot(r.Context())
	var live []agents.Session
	for _, sess := range snap.Sessions {
		if sess.Live() {
			live = append(live, sess)
		}
	}
	reports := make([]agents.ContextReport, len(live))
	for i, sess := range live {
		reports[i], _ = src.Context(sess.ID)
	}
	writeJSON(w, r, buildMatrix(formatTime(snap.At), live, reports))
}

// buildMatrix pivots one report per session into rows of sources. Two files
// are the same source when their layer and path (or name) agree.
func buildMatrix(at string, live []agents.Session, reports []agents.ContextReport) MatrixResponse {
	out := MatrixResponse{At: at, Sessions: make([]MatrixSession, len(live)), Rows: []MatrixRow{}, Layers: []ContextLayer{}, Insights: []Insight{}}
	for i, sess := range live {
		out.Sessions[i] = MatrixSession{ID: sess.ID, Name: sess.Name, Project: sess.Project, State: string(sess.State), Tool: string(sess.Tool)}
	}
	rows := map[string]*MatrixRow{}
	var order []string
	cell := func(layer agents.Layer, name, path, scope string, aggregate bool, i, tokens int, stale bool) {
		id := string(layer) + "|" + path
		if path == "" {
			id = string(layer) + "|" + name
		}
		row := rows[id]
		if row == nil {
			row = &MatrixRow{Key: id, Layer: string(layer), Name: name, Path: path, Scope: scope, Aggregate: aggregate, Cells: make([]MatrixCell, len(live))}
			rows[id] = row
			order = append(order, id)
		}
		if row.Cells[i].Loaded {
			row.Cells[i].Tokens += tokens
			row.Cells[i].Stale = row.Cells[i].Stale || stale
			return
		}
		row.Cells[i] = MatrixCell{Loaded: true, Tokens: tokens, Stale: stale}
	}
	layerTotals := map[agents.Layer]int{}
	for i, rep := range reports {
		for _, src := range rep.Sources {
			if aggregateLayers[src.Layer] {
				continue
			}
			cell(src.Layer, src.Name, src.Path, src.Scope, false, i, src.Tokens, src.Stale)
		}
		for _, l := range rep.Layers {
			layerTotals[l.Layer] += l.Tokens
			if aggregateLayers[l.Layer] && l.Tokens > 0 {
				cell(l.Layer, string(l.Layer), "", "all calls", true, i, l.Tokens, false)
			}
		}
	}
	for _, layer := range agents.Layers {
		if layerTotals[layer] > 0 {
			out.Layers = append(out.Layers, ContextLayer{Layer: string(layer), Tokens: layerTotals[layer]})
		}
	}
	for _, id := range order {
		row := rows[id]
		for _, c := range row.Cells {
			if c.Loaded {
				row.Sessions++
				row.Tokens += c.Tokens
			}
			if c.Stale {
				row.Stale++
			}
		}
		out.Rows = append(out.Rows, *row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if a.Aggregate != b.Aggregate {
			return !a.Aggregate
		}
		if a.Sessions != b.Sessions {
			return a.Sessions > b.Sessions
		}
		if a.Tokens != b.Tokens {
			return a.Tokens > b.Tokens
		}
		return a.Name < b.Name
	})
	out.Insights = matrixInsights(out)
	return out
}

func matrixInsights(m MatrixResponse) []Insight {
	insights := []Insight{}
	if len(m.Sessions) == 0 {
		return insights
	}
	var shared *MatrixRow
	staleRows, staleCells := 0, 0
	var worst *MatrixRow
	for i := range m.Rows {
		row := &m.Rows[i]
		if row.Aggregate {
			continue
		}
		if shared == nil || row.Sessions > shared.Sessions {
			shared = row
		}
		if row.Stale > 0 {
			staleRows++
			staleCells += row.Stale
			if worst == nil || row.Stale > worst.Stale {
				worst = row
			}
		}
	}
	if shared != nil && shared.Sessions > 1 {
		insights = append(insights, Insight{
			Title: fmt.Sprintf("%s is loaded by %d of %d sessions", shared.Name, shared.Sessions, len(m.Sessions)),
			Text:  fmt.Sprintf("The most shared source, about %s tokens per session.", kTokens(shared.Tokens/shared.Sessions)),
		})
	}
	if worst != nil {
		insights = append(insights, Insight{
			Title: fmt.Sprintf("%d sessions run on an old copy of %s", worst.Stale, worst.Name),
			Text:  fmt.Sprintf("%d stale cells across %d sources. A restarted session loads the current file.", staleCells, staleRows),
		})
	} else {
		insights = append(insights, Insight{Title: "No stale sources", Text: "Every session holds the current copy of what it loaded."})
	}
	total := 0
	for _, l := range m.Layers {
		total += l.Tokens
	}
	var biggest ContextLayer
	for _, l := range m.Layers {
		if l.Tokens > biggest.Tokens {
			biggest = l
		}
	}
	if total > 0 {
		insights = append(insights, Insight{
			Title: fmt.Sprintf("%s fills %d%% of all windows", biggest.Layer, (biggest.Tokens*100+total/2)/total),
			Text:  fmt.Sprintf("%s of %s tokens across %d live sessions.", kTokens(biggest.Tokens), kTokens(total), len(m.Sessions)),
		})
	}
	return insights
}

// kTokens formats a token count the way the views do: 6.1k, 91k, 640.
func kTokens(n int) string {
	switch {
	case n >= 100_000:
		return fmt.Sprintf("%dk", n/1000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
