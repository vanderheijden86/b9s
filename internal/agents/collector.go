package agents

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Options configures a Collector. Zero values pick the defaults below.
type Options struct {
	Home       string        // home directory; default os.UserHomeDir
	ClaudeDir  string        // default ~/.claude/projects
	CodexDir   string        // default ~/.codex/sessions
	MaxAge     time.Duration // ignore transcripts not written for this long; default 48h
	LiveWindow time.Duration // recent activity that counts as live without a pane; default 30m
	IdleAfter  time.Duration // a live session quiet this long is idle; default 5m
	Now        func() time.Time
	// Panes lists tmux panes; nil means ListPanes. Tests pass a fake.
	Panes func(ctx context.Context) []Pane
	// Pods, when set, adds Omnigent lane pods through kubectl.
	Pods *PodSource
}

func (o *Options) defaults() {
	if o.Home == "" {
		o.Home, _ = os.UserHomeDir()
	}
	if o.ClaudeDir == "" {
		o.ClaudeDir = filepath.Join(o.Home, ".claude", "projects")
	}
	if o.CodexDir == "" {
		o.CodexDir = filepath.Join(o.Home, ".codex", "sessions")
	}
	if o.MaxAge == 0 {
		o.MaxAge = 48 * time.Hour
	}
	if o.LiveWindow == 0 {
		o.LiveWindow = 30 * time.Minute
	}
	if o.IdleAfter == 0 {
		o.IdleAfter = 5 * time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Panes == nil {
		o.Panes = ListPanes
	}
}

// Snapshot is the result of one scan.
type Snapshot struct {
	Sessions []Session `json:"sessions"`
	Panes    []Pane    `json:"panes"`
	PodError string    `json:"podError,omitempty"`
	At       time.Time `json:"at"`
}

// Collector scans transcripts and keeps their parse state between scans.
// It is safe for concurrent use.
type Collector struct {
	opts  Options
	mu    sync.Mutex
	files map[string]*fileState
	last  Snapshot
	pods  map[string]Session
}

type fileState struct {
	offset int64
	tr     *transcript
}

// NewCollector returns a collector with opts' defaults filled in.
func NewCollector(opts Options) *Collector {
	opts.defaults()
	return &Collector{opts: opts, files: map[string]*fileState{}, pods: map[string]Session{}}
}

// Home is the home directory the collector resolves instruction files in.
func (c *Collector) Home() string { return c.opts.Home }

// Scan reads what each recent transcript appended since the last scan and
// returns every session, live ones first, most recent first.
func (c *Collector) Scan(ctx context.Context) Snapshot {
	now := c.opts.Now()
	paths := c.transcripts(now)
	panes := c.opts.Panes(ctx)
	var pods []Session
	var podErr error
	if c.opts.Pods != nil {
		pods, podErr = ListPods(ctx, *c.opts.Pods)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.readAll(paths)
	var list []*Session
	for path, st := range c.files {
		if _, ok := paths[path]; !ok {
			continue
		}
		if st.tr.sess.ID == "" || st.tr.sess.Cwd == "" {
			continue
		}
		s := st.tr.sess
		s.TokensPerMin = st.tr.tokensPerMin(now)
		s.Where = Where{}
		list = append(list, &s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].LastActivity.After(list[j].LastActivity) })
	MatchPanes(list, panes)
	out := make([]Session, 0, len(list)+len(pods))
	for _, s := range list {
		s.State = c.state(*s, c.files[s.Transcript].tr, now)
		out = append(out, *s)
	}
	c.pods = map[string]Session{}
	for _, p := range pods {
		c.pods[p.ID] = p
		out = append(out, p)
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := out[i].Live(), out[j].Live()
		if li != lj {
			return li
		}
		return out[i].LastActivity.After(out[j].LastActivity)
	})
	snap := Snapshot{Sessions: out, Panes: panes, At: now}
	if podErr != nil {
		snap.PodError = podErr.Error()
	}
	c.last = snap
	return snap
}

// Last is the most recent snapshot without rescanning.
func (c *Collector) Last() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

func (c *Collector) state(s Session, tr *transcript, now time.Time) State {
	quiet := now.Sub(s.LastActivity)
	live := s.Where.Kind != WhereNone || quiet < c.opts.LiveWindow
	switch {
	case !live:
		return StateDone
	case tr.apiError:
		return StateFailed
	case tr.lastStop == "end_turn" || tr.lastStop == "task_complete" || tr.lastStop == "stop_sequence":
		return StateWaiting
	case quiet > c.opts.IdleAfter:
		return StateIdle
	}
	return StateRunning
}

// transcripts lists the transcript files written within MaxAge.
func (c *Collector) transcripts(now time.Time) map[string]Tool {
	out := map[string]Tool{}
	add := func(pattern string, tool Tool) {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || now.Sub(info.ModTime()) > c.opts.MaxAge {
				continue
			}
			out[m] = tool
		}
	}
	add(filepath.Join(c.opts.ClaudeDir, "*", "*.jsonl"), ToolClaude)
	// Codex nests sessions by date: YYYY/MM/DD/rollout-*.jsonl. Only the
	// days inside MaxAge are globbed.
	for d := now.Add(-c.opts.MaxAge); !d.After(now.Add(24 * time.Hour)); d = d.Add(24 * time.Hour) {
		add(filepath.Join(c.opts.CodexDir, d.Format("2006"), d.Format("01"), d.Format("02"), "*.jsonl"), ToolCodex)
	}
	return out
}

// readAll reads what was appended to each transcript, several files at a
// time. A first scan parses hundreds of megabytes of JSON, and one core per
// file brings that wait down from seconds to the largest file's share. Every
// worker owns one fileState, and c.mu, held by the caller, keeps readers out
// until all of them finish.
func (c *Collector) readAll(paths map[string]Tool) {
	work := make(chan string, len(paths))
	for path, tool := range paths {
		if c.files[path] == nil {
			c.files[path] = &fileState{tr: newTranscript(tool, path)}
		}
		work <- path
	}
	close(work)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(paths)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range work {
				c.readAppended(path, c.files[path])
			}
		}()
	}
	wg.Wait()
}

// readAppended feeds the complete lines written after st.offset. A partial
// last line stays unread until the writer finishes it.
func (c *Collector) readAppended(path string, st *fileState) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return
	}
	if info.Size() < st.offset {
		st.offset = 0
		st.tr = newTranscript(st.tr.tool, path)
	}
	if info.Size() == st.offset {
		return
	}
	if _, err := f.Seek(st.offset, io.SeekStart); err != nil {
		return
	}
	r := bufio.NewReaderSize(f, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 && line[len(line)-1] == '\n' {
			st.offset += int64(len(line))
			st.tr.feed(line[:len(line)-1])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

// Session returns one session from the last scan.
func (c *Collector) Session(id string) (Session, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.last.Sessions {
		if s.ID == id {
			return s, true
		}
	}
	return Session{}, false
}

func (c *Collector) transcriptOf(id string) *transcript {
	for _, st := range c.files {
		if st.tr.sess.ID == id {
			return st.tr
		}
	}
	return nil
}

// Events returns a session's log. A lane pod's log comes from kubectl.
func (c *Collector) Events(ctx context.Context, id string) ([]Event, error) {
	c.mu.Lock()
	if p, ok := c.pods[id]; ok && c.opts.Pods != nil {
		src := *c.opts.Pods
		c.mu.Unlock()
		return PodLogs(ctx, src, p.Where.Ref, 500)
	}
	defer c.mu.Unlock()
	tr := c.transcriptOf(id)
	if tr == nil {
		return nil, nil
	}
	return append([]Event(nil), tr.events...), nil
}

// ReadEvents returns the events the last scan read for a local session
// without waiting. It reports false for a pod session, whose log comes over
// the network, and while a scan holds the collector, so a UI goroutine can
// draw from memory and fall back to Events in a command.
func (c *Collector) ReadEvents(id string) ([]Event, bool) {
	if !c.mu.TryLock() {
		return nil, false
	}
	defer c.mu.Unlock()
	if _, pod := c.pods[id]; pod {
		return nil, false
	}
	tr := c.transcriptOf(id)
	if tr == nil {
		return nil, false
	}
	return append([]Event(nil), tr.events...), true
}

// Context builds the context report of one session.
func (c *Collector) Context(id string) (ContextReport, bool) {
	c.mu.Lock()
	tr := c.transcriptOf(id)
	var sess Session
	var recorded []Source
	var compacted time.Time
	found := false
	for _, s := range c.last.Sessions {
		if s.ID == id {
			sess, found = s, true
		}
	}
	if tr != nil {
		recorded = tr.sourceList()
		compacted = tr.compactedAt
		if !found {
			sess, found = tr.sess, true
		}
	}
	c.mu.Unlock()
	if !found {
		return ContextReport{}, false
	}
	return BuildContext(sess, recorded, c.opts.Home, compacted), true
}

// Activity is a histogram over equal time buckets, oldest first.
type Activity struct {
	Start  time.Time     `json:"start"`
	Bucket time.Duration `json:"bucket"`
	Active []int         `json:"active"` // sessions with at least one event
	Tokens []int         `json:"tokens"` // output tokens
	Errors []int         `json:"errors"` // error events
}

// Activity buckets every scanned transcript's events and token samples into
// n buckets of width bucket that end at now.
func (c *Collector) Activity(now time.Time, n int, bucket time.Duration) Activity {
	start := now.Add(-time.Duration(n) * bucket)
	a := Activity{Start: start, Bucket: bucket, Active: make([]int, n), Tokens: make([]int, n), Errors: make([]int, n)}
	index := func(at time.Time) int {
		if at.Before(start) || at.After(now) {
			return -1
		}
		i := int(at.Sub(start) / bucket)
		if i >= n {
			i = n - 1
		}
		return i
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, st := range c.files {
		seen := make([]bool, n)
		for _, e := range st.tr.events {
			i := index(e.Time)
			if i < 0 {
				continue
			}
			if !seen[i] {
				seen[i] = true
				a.Active[i]++
			}
			if e.Kind == EventErr {
				a.Errors[i]++
			}
		}
		for _, s := range st.tr.samples {
			if i := index(s.at); i >= 0 {
				a.Tokens[i] += s.tokens
			}
		}
	}
	return a
}
