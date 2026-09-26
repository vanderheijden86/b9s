package web

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/identity"
	"github.com/vanderheijden86/beadwork/pkg/model"
	"github.com/vanderheijden86/beadwork/pkg/ui"
	"github.com/vanderheijden86/beadwork/pkg/watcher"
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
func (s *Store) Issue(id string) (Issue, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.issues {
		if s.issues[i].ID == id {
			return convertIssue(&s.issues[i], statusByID(s.issues)), true
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
	default:
		h.Watching = "watches the file"
	}
	return h
}

func sourceLabel(source datasource.DataSource) string {
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
	case datasource.SourceTypeDolt:
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
