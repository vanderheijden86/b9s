package web

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/internal/gitlog"
)

// Timeline phases.
const (
	phasePlan      = "plan"
	phaseImplement = "implement"
	phaseTrack     = "track"
	phaseFail      = "fail"
	phaseIdle      = "idle"
)

const (
	// idleGap is the silence after which a lane counts as idle.
	idleGap = 5 * time.Minute
	// tailGap is how long the last event of a stretch stays visible.
	tailGap = 30 * time.Second
	// toolAdjacent is how close an AI line must sit to a tool call to count
	// as part of the work rather than as planning.
	toolAdjacent = time.Minute
	maxMarkers   = 40
	maxLabel     = 32
)

// phaseOf classifies one event. A model message between tool calls belongs
// to the implementation it narrates, so only a message with no tool call
// beside it is planning.
func phaseOf(events []agents.Event, i int) string {
	switch events[i].Kind {
	case agents.EventErr:
		return phaseFail
	case agents.EventTool, agents.EventResult:
		return phaseImplement
	case agents.EventBD:
		return phaseTrack
	case agents.EventAI:
		for j := i - 1; j >= 0 && events[i].Time.Sub(events[j].Time) <= toolAdjacent; j-- {
			if k := events[j].Kind; k == agents.EventTool || k == agents.EventResult {
				return phaseImplement
			}
		}
		for j := i + 1; j < len(events) && events[j].Time.Sub(events[i].Time) <= toolAdjacent; j++ {
			if k := events[j].Kind; k == agents.EventTool || k == agents.EventResult {
				return phaseImplement
			}
		}
	}
	return phasePlan
}

func label(e agents.Event) string {
	text := e.Tool
	if text == "" {
		text = strings.TrimSpace(e.Text)
	}
	if r := []rune(text); len(r) > maxLabel {
		text = string(r[:maxLabel-1]) + "…"
	}
	return text
}

// buildLane turns a session's events into phase segments and markers inside
// [from, to]. A silence of idleGap or more becomes an idle segment, and so
// does the stretch up to now of a session that still lives and has gone quiet.
func buildLane(sess agents.Session, events []agents.Event, from, to time.Time) TimelineLane {
	lane := TimelineLane{SessionID: sess.ID, Name: sess.Name, Project: sess.Project, State: string(sess.State),
		Segments: []TimelineSegment{}, Markers: []TimelineMarker{}}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	var inside []agents.Event
	for _, e := range events {
		if !e.Time.Before(from) && !e.Time.After(to) && e.Kind != agents.EventCtx {
			inside = append(inside, e)
		}
	}
	add := func(phase string, start, end time.Time, text string) {
		if end.After(to) {
			end = to
		}
		if !end.After(start) {
			return
		}
		lane.Segments = append(lane.Segments, TimelineSegment{Phase: phase, Start: formatTime(start), End: formatTime(end), Label: text})
	}

	var (
		cur      string
		curStart time.Time
		curLabel string
		last     time.Time
	)
	flush := func(end time.Time) {
		if cur != "" {
			add(cur, curStart, end, curLabel)
			cur = ""
		}
	}
	for i, e := range inside {
		phase := phaseOf(inside, i)
		if cur != "" && e.Time.Sub(last) >= idleGap {
			flush(last.Add(tailGap))
			add(phaseIdle, last.Add(tailGap), e.Time, "")
		}
		if cur != phase {
			flush(e.Time)
			cur, curStart, curLabel = phase, e.Time, label(e)
		}
		last = e.Time
	}
	flush(last.Add(tailGap))
	if len(inside) > 0 && sess.Live() && to.Sub(last) >= idleGap {
		add(phaseIdle, last.Add(tailGap), to, "")
	}

	for _, e := range events {
		if e.Time.Before(from) || e.Time.After(to) || len(lane.Markers) >= maxMarkers {
			continue
		}
		switch {
		case e.Kind == agents.EventBD:
			lane.Markers = append(lane.Markers, TimelineMarker{Time: formatTime(e.Time), Kind: "bd", Label: label(e)})
		case e.Kind == agents.EventErr:
			lane.Markers = append(lane.Markers, TimelineMarker{Time: formatTime(e.Time), Kind: "fail", Label: label(e)})
		case e.Kind == agents.EventCtx && strings.Contains(strings.ToLower(e.Text), "compact"):
			lane.Markers = append(lane.Markers, TimelineMarker{Time: formatTime(e.Time), Kind: "ctx", Label: label(e)})
		}
	}
	return lane
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	hours := 2
	if v := r.URL.Query().Get("hours"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 48 {
			writeError(w, http.StatusBadRequest, "hours must be a whole number from 1 to 48")
			return
		}
		hours = n
	}
	snap, src := s.feed.snapshot(r.Context())
	to := s.now()
	from := to.Add(-time.Duration(hours) * time.Hour)
	commits := s.projectCommits(r.Context(), 200)

	out := TimelineResponse{From: formatTime(from), To: formatTime(to), Hours: hours, Lanes: []TimelineLane{}}
	for _, sess := range snap.Sessions {
		if !sess.Live() && sess.LastActivity.Before(from) {
			continue
		}
		events, err := src.Events(r.Context(), sess.ID)
		if err != nil {
			continue
		}
		lane := buildLane(sess, events, from, to)
		if len(lane.Segments) == 0 && !sess.Live() {
			continue
		}
		if strings.EqualFold(sess.Project, s.opts.Store.Info().Name) {
			for _, c := range commits {
				if c.Time.Before(from) || c.Time.After(to) || len(lane.Markers) >= maxMarkers {
					continue
				}
				lane.Markers = append(lane.Markers, TimelineMarker{Time: formatTime(c.Time), Kind: "commit", Label: c.Subject})
			}
		}
		out.Lanes = append(out.Lanes, lane)
	}
	writeJSON(w, r, out)
}

// projectCommits reads the open project's recent commits, or none when the
// project has no checkout or git fails: a marker is decoration.
func (s *Server) projectCommits(ctx context.Context, limit int) []gitlog.Commit {
	dir := s.opts.Store.CheckoutDir()
	if dir == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	root, err := gitlog.Root(ctx, dir)
	if err != nil {
		return nil
	}
	commits, err := gitlog.Read(ctx, root, gitlog.Options{Max: limit})
	if err != nil {
		return nil
	}
	return commits
}

const (
	defaultCommits = 50
	maxCommits     = 400
)

func (s *Server) handleCommits(w http.ResponseWriter, r *http.Request) {
	limit := defaultCommits
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxCommits {
			writeError(w, http.StatusBadRequest, "limit must be a whole number from 1 to "+strconv.Itoa(maxCommits))
			return
		}
		limit = n
	}
	dir := s.opts.Store.CheckoutDir()
	if dir == "" {
		writeError(w, http.StatusConflict, "this project has no checkout to read commits from")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	root, err := gitlog.Root(ctx, dir)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	commits, err := gitlog.Read(ctx, root, gitlog.Options{Max: limit})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := CommitsResponse{Repo: root, Commits: make([]CommitInfo, len(commits))}
	for i, c := range commits {
		out.Commits[i] = CommitInfo{Hash: c.Hash, Short: c.Short, Time: formatTime(c.Time), Author: c.Author, Subject: c.Subject,
			Refs: nonNil(c.Refs), Issues: nonNil(c.Issues)}
	}
	writeJSON(w, r, out)
}
