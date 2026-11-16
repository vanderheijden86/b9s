package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/agents"
)

// fleetCollectorOptions builds the collector options from the user config.
// Tests replace the collector through SetFleetCollector.
func (m Model) fleetCollectorOptions() agents.Options {
	var opts agents.Options
	if pods := m.appConfig.Fleet.Pods; pods.Enabled {
		opts.Pods = &agents.PodSource{Context: pods.Context, Namespace: pods.Namespace, Selector: pods.Selector}
	}
	return opts
}

// SetFleetCollector replaces the collector the fleet views read.
func (m *Model) SetFleetCollector(c *agents.Collector) { m.fleetCollector = c }

// FleetVisible reports whether a fleet view fills the screen.
func (m Model) FleetVisible() bool { return m.fleetVisible && m.fleet != nil }

// openFleet shows a fleet view. The browser and its scan state survive a
// close, so reopening is instant. Each open starts a new tick chain under a
// new generation, and the chain of the previous open is dropped.
func (m Model) openFleet(view fleetView) (Model, tea.Cmd) {
	if m.fleetCollector == nil {
		m.fleetCollector = agents.NewCollector(m.fleetCollectorOptions())
	}
	deps := m.fleetDeps()
	var b FleetBrowser
	if m.fleet != nil {
		b = *m.fleet
		b.deps = deps.withDefaults()
		b.width, b.height = m.width, m.height
	} else {
		b = NewFleetBrowser(deps, m.theme, m.width, m.height, fleetAgents)
	}
	b, root, _ := b.switchRoot(view)
	m.fleetGeneration++
	cmd := b.Init()
	m.fleet, m.fleetVisible = &b, true
	return m, wrapFleetCmd(tea.Batch(cmd, root), m.fleetGeneration)
}

func (m *Model) fleetDeps() FleetDeps {
	repos := map[string]string{}
	for _, p := range m.allProjects {
		if p.Path != "" {
			repos[p.Name] = p.Path
		}
	}
	if m.activeProjectName != "" && m.activeProjectPath != "" {
		repos[m.activeProjectName] = m.activeProjectPath
	}
	issueMap := m.issueMap
	issues := m.issues
	write := m.writeClipboard
	return FleetDeps{
		Collector: m.fleetCollector,
		RepoDir:   m.activeProjectPath,
		Repos:     repos,
		Issue: func(id string) (string, string, bool) {
			is, ok := issueMap[id]
			if !ok || is == nil {
				return "", "", false
			}
			return is.Title, string(is.Status), true
		},
		Closed: func() []time.Time {
			var out []time.Time
			for i := range issues {
				if t := issues[i].ClosedAt; t != nil {
					out = append(out, *t)
				}
			}
			return out
		},
		Clipboard: write,
	}
}

// updateFleet routes a message to the fleet browser. It reports false for
// messages the rest of the Model handles.
func (m Model) updateFleet(msg tea.Msg) (Model, tea.Cmd, bool) {
	if envelope, ok := msg.(embeddedFleetMsg); ok {
		if m.fleet == nil || envelope.generation != m.fleetGeneration {
			return m, nil, true
		}
		// A hidden fleet stops polling: the tick chain ends here, and the
		// next open starts a new one.
		if _, tick := envelope.msg.(fleetTickMsg); tick && !m.fleetVisible {
			return m, nil, true
		}
		b, cmd, _ := m.fleet.Update(envelope.msg)
		m.fleet = &b
		return m, wrapFleetCmd(cmd, m.fleetGeneration), true
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok && m.fleet != nil {
		b, _, _ := m.fleet.Update(size)
		m.fleet = &b
		return m, nil, false
	}
	if !m.fleetVisible || m.fleet == nil {
		return m, nil, false
	}
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
	default:
		return m, nil, false
	}
	if key, ok := msg.(tea.KeyMsg); ok && fleetIssueKeys[key.String()] && m.fleet.prompt == fleetPromptNone {
		m.fleetVisible = false
		m.updateViewportContent()
		return m, nil, false
	}
	b, cmd, out := m.fleet.Update(msg)
	m.fleet = &b
	cmd = wrapFleetCmd(cmd, m.fleetGeneration)
	switch {
	case out.close:
		m.fleetVisible = false
		m.updateViewportContent()
		return m, cmd, true
	case out.openIssue != "":
		m.fleetVisible = false
		next, show := m.handleShowBranch(out.openIssue)
		return next, tea.Batch(cmd, show), true
	case out.command != "":
		m.fleetVisible = false
		command, err := ResolveCommand(out.command)
		if err != nil {
			m.statusMsg, m.statusIsError = err.Error(), true
			return m, cmd, true
		}
		next, run := m.executeCommand(command)
		return next, tea.Batch(cmd, run), true
	}
	return m, cmd, true
}
