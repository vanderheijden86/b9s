package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/identity"
)

// identitiesLoadedMsg carries the identity registry of the project database
// named by source. The source travels with it so a load that finishes after a
// project switch is dropped instead of relabelling the new project's people.
type identitiesLoadedMsg struct {
	source   datasource.DataSource
	config   datasource.IdentityConfig
	registry *identity.Registry
	err      error
}

// Identities returns the alias registry of the open project, or nil when the
// project has none. A nil registry resolves every name to itself.
func (m Model) Identities() *identity.Registry {
	return m.identities
}

// identitySource names the database that holds the identity configuration.
// JSONL projects have no config table, so they show raw names (ADR 0014).
func (m Model) identitySource() (datasource.DataSource, bool) {
	if m.sourceType != datasource.SourceTypeDolt || m.doltSource.Type != datasource.SourceTypeDolt {
		return datasource.DataSource{}, false
	}
	return m.doltSource, true
}

// loadIdentitiesCmd reads the identity configuration off the update loop.
func (m Model) loadIdentitiesCmd() tea.Cmd {
	source, ok := m.identitySource()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		cfg, err := datasource.LoadIdentityConfig(source)
		if err != nil {
			return identitiesLoadedMsg{source: source, err: err}
		}
		registry, parseErr := identity.Parse(cfg.Identities, cfg.ClaimPools)
		return identitiesLoadedMsg{source: source, config: cfg, registry: registry, err: parseErr}
	}
}

func (m Model) applyIdentities(msg identitiesLoadedMsg) Model {
	current, ok := m.identitySource()
	if !ok || current.Path != msg.source.Path || current.Database != msg.source.Database {
		return m
	}
	if msg.err != nil {
		debug.Log("identities: %v", msg.err)
	}
	m.identityErr = msg.err
	if msg.registry == nil {
		// A failed read keeps the registry already shown.
		return m
	}
	m.setIdentities(msg.registry)
	m.identityConfig = msg.config
	m.rebuildPickerEntries()
	m.applyFilter()
	m.tree.SetAssigneeFilter(m.assigneeFilter)
	return m
}

// setIdentities gives the model and the tree one registry, so the list and the
// tree filter by assignee identically.
func (m *Model) setIdentities(reg *identity.Registry) {
	m.identities = reg
	m.tree.identities = reg
}

// assigneeMatches reports whether an issue passes the assignee filter. The
// filter holds a display name; the issue may store any alias of it.
func (m *Model) assigneeMatches(assignee string) bool {
	return m.assigneeFilter == "" || m.identities.Matches(m.assigneeFilter, assignee)
}

// fillIdentityHealth adds the identity section of the health popup.
func (m Model) fillIdentityHealth(h *DatabaseHealth) {
	if login, _, _ := strings.Cut(m.identityConfig.SQLUser, "@"); login != "" {
		h.SQLLogin = login
	}
	who := m.currentIdentity()
	h.Actor, h.ActorKind = who.Name, who.Kind
	h.IdentityProblems = nil
	if m.identityErr != nil {
		h.IdentityProblems = append(h.IdentityProblems, m.identityErr.Error())
	}
	h.IdentityProblems = append(h.IdentityProblems, m.identities.Conflicts()...)
}
