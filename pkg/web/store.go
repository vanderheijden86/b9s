package web

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/debug"
	"github.com/vanderheijden86/b9s/pkg/identity"
	"github.com/vanderheijden86/b9s/pkg/model"
	"github.com/vanderheijden86/b9s/pkg/ui"
	"github.com/vanderheijden86/b9s/pkg/watcher"
)

// Store holds the open project and tells subscribers when its data changes.
// It is the web server's counterpart of the TUI model's project state: the
// same readers load it and the same watchers refresh it.
type Store struct {
	pollInterval time.Duration

	mu          sync.RWMutex
	info        ProjectInfo
	target      datasource.OpenTarget
	source      datasource.DataSource
	multi       *datasource.MultiDoltReader
	fallback    string
	loadErr     string
	loadedAt    time.Time
	issues      []model.Issue
	memoryGraph MemoryGraphResponse
	memory      memoryState
	version     uint64
	registry    *identity.Registry
	actorDir    string
	stopWatcher func()

	subMu sync.Mutex
	subs  map[chan Event]struct{}

	reloadCh chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewStore returns an empty store. Open or OpenAll loads a project into it.
func NewStore(pollInterval time.Duration) *Store {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	s := &Store{
		pollInterval: pollInterval,
		subs:         make(map[chan Event]struct{}),
		reloadCh:     make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
	go s.reloadLoop()
	return s
}

// Open loads target and, only when it loads, replaces the project on screen
// (ADR 0012). A failure leaves the current project in place.
func (s *Store) Open(target datasource.OpenTarget, key string) *datasource.OpenFailure {
	opened, failure := datasource.OpenProject(target)
	if failure != nil {
		return failure
	}
	registry := loadRegistry(opened.Source)
	fallback := ""
	if opened.DoltFailure != nil {
		fallback = opened.DoltFailure.Message()
	}
	_, hasCheckout := ui.NewCheckout(target.Dir)

	s.mu.Lock()
	s.stopWatchingLocked()
	s.closeMultiLocked()
	s.info = ProjectInfo{Name: target.Name, Key: key, ReadOnly: !hasCheckout}
	s.target = target
	s.source = opened.Source
	s.fallback = fallback
	s.loadErr = ""
	s.loadedAt = time.Now()
	s.issues = opened.Issues
	s.resetMemoryLocked(opened.Source)
	s.registry = registry
	s.actorDir = target.Dir
	s.version++
	version := s.version
	s.stopWatcher = s.watch(opened.Source)
	s.mu.Unlock()

	s.publish(Event{Type: "project", Version: version})
	return nil
}

// OpenAll loads the read-only view of every listed database.
func (s *Store) OpenAll(dbs []datasource.DoltDBInfo) error {
	if len(dbs) == 0 {
		return errors.New("no project uses a Dolt server, so there is nothing to combine")
	}
	reader, err := datasource.NewMultiDoltReader(dbs)
	if err != nil {
		return err
	}
	issues, err := reader.LoadAllIssues()
	if err != nil {
		reader.Close()
		return err
	}

	s.mu.Lock()
	s.stopWatchingLocked()
	s.closeMultiLocked()
	s.info = ProjectInfo{Name: "all projects", Key: AllProjectsKey, ReadOnly: true, All: true}
	s.target = datasource.OpenTarget{Name: "all projects"}
	s.source = datasource.DataSource{Type: datasource.SourceTypeDolt, Path: fmt.Sprintf("%d databases", len(dbs))}
	s.multi = reader
	s.fallback = ""
	s.loadErr = ""
	s.loadedAt = time.Now()
	s.issues = issues
	s.memory = memoryState{generation: s.memory.generation + 1}
	s.memoryGraph = MemoryGraphResponse{Reason: "the all-projects view has no Memory graph", Nodes: []MemoryGraphNode{}, Edges: []MemoryGraphEdge{}}
	s.registry = nil
	s.actorDir = ""
	s.version++
	version := s.version
	w := datasource.NewMultiDoltWatcher(reader, s.pollInterval)
	if err := w.Start(); err == nil {
		go func() {
			for w.WaitForChange() {
				s.requestReload()
			}
		}()
	}
	s.stopWatcher = w.Stop
	s.mu.Unlock()

	s.publish(Event{Type: "project", Version: version})
	return nil
}

// AllProjectsKey is the project key of the combined read-only view (TUI 0).
const AllProjectsKey = "*"

// Reload reads the open project again and publishes a change when it loads.
// A failed read keeps the issues on screen and reports the error in Health.
func (s *Store) Reload() {
	s.mu.RLock()
	source, multi := s.source, s.multi
	s.mu.RUnlock()

	var issues []model.Issue
	var err error
	switch {
	case multi != nil:
		issues, err = multi.LoadAllIssues()
	case source.Type != "":
		issues, err = datasource.LoadFromSource(source)
	default:
		return
	}

	s.mu.Lock()
	if s.source != source || s.multi != multi {
		// Another project opened while this one was loading.
		s.mu.Unlock()
		return
	}
	if err != nil {
		s.loadErr = err.Error()
		s.mu.Unlock()
		debug.Log("web: reload failed: %v", err)
		s.publish(Event{Type: "health"})
		return
	}
	s.loadErr = ""
	s.loadedAt = time.Now()
	s.issues = issues
	s.refreshMemoryLocked()
	s.version++
	version := s.version
	s.mu.Unlock()
	s.publish(Event{Type: "changed", Version: version})
}

// Version is the number of loads so far. Every load increments it.
func (s *Store) Version() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.version
}

// Info describes the project on screen.
func (s *Store) Info() ProjectInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

// CheckoutDir is where bd runs for the project on screen, or "" when it is
// read-only.
func (s *Store) CheckoutDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.info.ReadOnly {
		return ""
	}
	return s.target.Dir
}

// Target is what the project on screen was opened from.
func (s *Store) Target() datasource.OpenTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.target
}

// Issues returns the loaded issues and the version they belong to.
func (s *Store) Issues() ([]model.Issue, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.issues, s.version
}

// memoryState tracks the Memory graph of a Memory workspace, which is read on
// request rather than with the Issues (ADR 0051).
type memoryState struct {
	// beadsDir is the workspace's .beads directory, "" for any other project.
	beadsDir string
	// generation changes with every project opened, so a read that finishes
	// after a switch is dropped.
	generation uint64
	reading    bool
	// stale marks a reload that arrived during a read, which then reads again.
	stale       bool
	done, total int
	// failure is the last read's error, reported to one request so the next
	// one retries.
	failure string
}

// memoryReadTimeout bounds one read of the whole graph.
const memoryReadTimeout = 5 * time.Minute

// resetMemoryLocked forgets the graph of the previous project. A Memory
// workspace keeps no graph until it is requested; any other project gets the
// reason it has none.
func (s *Store) resetMemoryLocked(source datasource.DataSource) {
	s.memory = memoryState{generation: s.memory.generation + 1}
	s.memoryGraph = MemoryGraphResponse{Nodes: []MemoryGraphNode{}, Edges: []MemoryGraphEdge{}}
	if source.Type != datasource.SourceTypeDoltEmbedded {
		s.memoryGraph.Reason = "this project is not a Memory graph workspace"
		return
	}
	beadsDir := datasource.EmbeddedBeadsDir(source)
	if !datasource.MemoryWorkspaceFor(beadsDir) {
		s.memoryGraph.Reason = memoryUnavailableReason(filepath.Dir(beadsDir))
		return
	}
	s.memory.beadsDir = beadsDir
}

// refreshMemoryLocked reads a graph that was read before again after a
// reload. A graph nobody requested stays unread.
func (s *Store) refreshMemoryLocked() {
	switch {
	case s.memory.reading:
		s.memory.stale = true
	case s.memoryGraph.Available:
		s.startMemoryReadLocked()
	}
}

// requestMemoryLocked starts the first read of a workspace's graph. A failed
// read waits for the next request.
func (s *Store) requestMemoryLocked() {
	if s.memory.beadsDir == "" || s.memoryGraph.Available || s.memory.reading || s.memory.failure != "" {
		return
	}
	s.startMemoryReadLocked()
}

func (s *Store) startMemoryReadLocked() {
	s.memory.reading, s.memory.stale = true, false
	s.memory.done, s.memory.total = 0, 0
	beadsDir, generation := s.memory.beadsDir, s.memory.generation
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), memoryReadTimeout)
		defer cancel()
		go func() {
			select {
			case <-s.done:
				cancel()
			case <-ctx.Done():
			}
		}()
		current := func() bool { return s.memory.generation == generation }
		graph, err := func() (datasource.MemoryGraph, error) {
			client, err := datasource.OpenGraphPreview(filepath.Dir(beadsDir))
			if err != nil {
				return datasource.MemoryGraph{}, err
			}
			return client.Graph(ctx, func(done, total int) {
				s.mu.Lock()
				if current() {
					s.memory.done, s.memory.total = done, total
				}
				s.mu.Unlock()
			})
		}()

		s.mu.Lock()
		if !current() {
			s.mu.Unlock()
			return
		}
		s.memory.reading = false
		if err != nil {
			debug.Log("web: Memory graph read failed: %v", err)
			if !s.memoryGraph.Available {
				s.memory.failure = "the Memory graph could not be read: " + err.Error()
			}
			if s.memory.stale {
				s.startMemoryReadLocked()
			}
			s.mu.Unlock()
			return
		}
		s.memoryGraph = memoryGraphResponse(graph)
		if s.memory.stale {
			s.startMemoryReadLocked()
		}
		// The new version makes the browser fetch the graph and the open
		// detail again.
		s.version++
		version := s.version
		s.mu.Unlock()
		s.publish(Event{Type: "changed", Version: version})
	}()
}

func memoryGraphResponse(graph datasource.MemoryGraph) MemoryGraphResponse {
	result := MemoryGraphResponse{Available: true, Nodes: []MemoryGraphNode{}, Edges: []MemoryGraphEdge{}}
	for _, node := range graph.Nodes {
		body := node.Body
		if node.Kind == "issue" {
			body = node.Issue.Description
		}
		result.Nodes = append(result.Nodes, MemoryGraphNode{ID: node.ID, Kind: node.Kind, Title: node.Title, Status: node.Status, Type: node.Type, Version: node.Version, Body: body})
	}
	for _, edge := range graph.Edges {
		result.Edges = append(result.Edges, MemoryGraphEdge{ID: edge.ID, Type: edge.Type, Source: edge.Source, Target: edge.Target, Kind: edge.Kind, Note: edge.Note})
	}
	return result
}

// memoryUnavailableReason names why a project has no captured graph. The
// detection is the same one that decided to skip the graph read, and costs a
// metadata read and a cached probe. A ready project without a graph had its
// read fail, which Health already reports.
func memoryUnavailableReason(projectDir string) string {
	capability := datasource.DetectMemory(context.Background(), projectDir)
	if reason, ok := datasource.MemoryUnavailableReason(capability.Err); ok {
		return reason
	}
	return "the Memory graph could not be read"
}

// MemoryGraph returns the graph of the open project. The first request for a
// workspace's graph starts its read and reports the progress until it is read.
func (s *Store) MemoryGraph() MemoryGraphResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestMemoryLocked()
	result := s.memoryGraph
	result.Version = s.version
	if s.memory.failure != "" {
		result.Reason = s.memory.failure
		s.memory.failure = ""
	}
	result.Loading = !result.Available && s.memory.reading
	result.Done, result.Total = s.memory.done, s.memory.total
	return result
}

// Snapshot converts the open project into its JSON form.
func (s *Store) Snapshot(bdFound bool) Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	registry := s.registry
	actor := ""
	if !s.info.All {
		actor = registry.Resolve(ui.ResolveActor(s.actorDir)).Name
	}
	return Snapshot{
		Version: s.version,
		Project: s.info,
		Issues:  leanIssues(s.issues),
		Actor:   actor,
		People:  people(registry, s.issues),
		Health:  s.healthLocked(bdFound),
	}
}

// Issue returns one issue with every field, or false when it does not exist.
// The detail lists the issue's Memory Links, so it asks for the graph.
func (s *Store) Issue(id string) (Issue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.issues {
		if s.issues[i].ID == id {
			s.requestMemoryLocked()
			issue := convertIssue(&s.issues[i], statusByID(s.issues))
			issue.MemoryAvailable = s.memoryGraph.Available
			issue.MemoryLoading = !s.memoryGraph.Available && s.memory.reading
			issue.MemoryLinks = []IssueMemoryLink{}
			for _, edge := range s.memoryGraph.Edges {
				other := ""
				if edge.Source == id {
					other = edge.Target
				} else if edge.Target == id {
					other = edge.Source
				}
				if other == "" {
					continue
				}
				for _, node := range s.memoryGraph.Nodes {
					if node.ID == other && node.Kind == "memory" {
						issue.MemoryLinks = append(issue.MemoryLinks, IssueMemoryLink{Link: edge, Memory: node, Outgoing: edge.Source == id})
					}
				}
			}
			return issue, true
		}
	}
	return Issue{}, false
}

// Health reports the data source of the open project.
func (s *Store) Health(bdFound bool) Health {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthLocked(bdFound)
}

func (s *Store) healthLocked(bdFound bool) Health {
	h := Health{
		OK:       s.loadErr == "" && s.fallback == "",
		Kind:     string(s.source.Type),
		Source:   sourceLabel(s.source),
		Issues:   len(s.issues),
		Fallback: s.fallback,
		Error:    s.loadErr,
		BdFound:  bdFound,
	}
	if !s.loadedAt.IsZero() {
		h.LoadedAt = s.loadedAt.UTC().Format(time.RFC3339)
	}
	switch {
	case s.stopWatcher == nil:
		h.Watching = "not watching"
	case s.source.Type == datasource.SourceTypeDolt:
		h.Watching = fmt.Sprintf("polls the database hash every %s", s.pollInterval)
	case s.source.Type == datasource.SourceTypeDoltEmbedded:
		h.Watching = fmt.Sprintf("polls the store files every %s", s.pollInterval)
	default:
		h.Watching = "watches the file"
	}
	return h
}

func sourceLabel(source datasource.DataSource) string {
	if source.Type == datasource.SourceTypeDoltEmbedded {
		return source.EmbeddedLabel()
	}
	if source.Type == datasource.SourceTypeDolt && source.Database != "" {
		return fmt.Sprintf("dolt://%s/%s", source.Path, source.Database)
	}
	return source.Path
}

// Subscribe returns a channel of events and a function that ends the
// subscription. A slow subscriber misses events rather than blocking a
// reload; every event makes the browser refetch, so one is enough.
func (s *Store) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 4)
	s.subMu.Lock()
	s.subs[ch] = struct{}{}
	s.subMu.Unlock()
	return ch, func() {
		s.subMu.Lock()
		delete(s.subs, ch)
		s.subMu.Unlock()
	}
}

func (s *Store) publish(e Event) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Close stops the watcher and the reload loop.
func (s *Store) Close() {
	s.stopOnce.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.stopWatchingLocked()
		s.closeMultiLocked()
		s.mu.Unlock()
	})
}

func (s *Store) requestReload() {
	select {
	case s.reloadCh <- struct{}{}:
	default:
	}
}

// reloadLoop runs reloads one at a time. Change notices that arrive during a
// reload collapse into one more reload.
func (s *Store) reloadLoop() {
	for {
		select {
		case <-s.done:
			return
		case <-s.reloadCh:
			s.Reload()
		}
	}
}

func (s *Store) stopWatchingLocked() {
	if s.stopWatcher != nil {
		s.stopWatcher()
		s.stopWatcher = nil
	}
}

func (s *Store) closeMultiLocked() {
	if s.multi != nil {
		s.multi.Close()
		s.multi = nil
	}
}

// watch starts the watcher the TUI uses for source and returns its stop
// function, or nil when the source cannot be watched.
func (s *Store) watch(source datasource.DataSource) func() {
	switch source.Type {
	case datasource.SourceTypeDolt, datasource.SourceTypeDoltEmbedded:
		// An embedded store changes files on every read, so it is polled by
		// fingerprint rather than handed to the file watcher (ADR 0025).
		w, err := datasource.NewDoltWatcher(source, s.pollInterval)
		if err != nil {
			debug.Log("web: dolt watcher: %v", err)
			return nil
		}
		w.SetOnChange(s.requestReload)
		if err := w.Start(); err != nil {
			w.Stop()
			return nil
		}
		return w.Stop
	default:
		if source.Path == "" {
			return nil
		}
		w, err := watcher.NewWatcher(source.Path, watcher.WithOnChange(s.requestReload))
		if err != nil {
			debug.Log("web: file watcher: %v", err)
			return nil
		}
		if err := w.Start(); err != nil {
			return nil
		}
		return w.Stop
	}
}

func loadRegistry(source datasource.DataSource) *identity.Registry {
	if source.Type != datasource.SourceTypeDolt {
		return nil
	}
	cfg, err := datasource.LoadIdentityConfig(source)
	if err != nil {
		debug.Log("web: identities: %v", err)
		return nil
	}
	registry, err := identity.Parse(cfg.Identities, cfg.ClaimPools)
	if err != nil {
		debug.Log("web: identities: %v", err)
	}
	return registry
}

// people lists the configured identities, then every other name that owns
// or created an issue, so a picker offers everyone the tree shows.
func people(registry *identity.Registry, issues []model.Issue) []Person {
	var out []Person
	known := make(map[string]bool)
	for _, id := range registry.Identities() {
		out = append(out, Person{Name: id.Name, Kind: string(id.Kind), Aliases: nonNil(id.Aliases)})
		known[id.Name] = true
	}
	var raw []string
	for _, issue := range issues {
		for _, name := range []string{issue.Assignee, issue.CreatedBy} {
			if name == "" {
				continue
			}
			display := registry.DisplayName(name)
			if !known[display] {
				known[display] = true
				raw = append(raw, display)
			}
		}
	}
	sort.Strings(raw)
	for _, name := range raw {
		out = append(out, Person{Name: name, Aliases: []string{}})
	}
	if out == nil {
		out = []Person{}
	}
	return out
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}
