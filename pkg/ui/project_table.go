package ui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/b9s/internal/agents"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
)

// ProjectTableState is the :project table's loading lifecycle.
type ProjectTableState uint8

const (
	ProjectTableLoading ProjectTableState = iota + 1
	ProjectTableLoaded
	ProjectTableFailed
)

// errNoStartupServer means the startup project has no Dolt server whose
// databases could be listed.
var errNoStartupServer = errors.New("the startup project does not use a Dolt server")

// projectTableLoadedMsg carries the databases the startup user can read, or
// why they could not be listed.
type projectTableLoadedMsg struct {
	databases []string
	err       error
}

// ProjectTableModel is the full-screen :project table. It lists the Beads
// databases on the startup project's server that the startup user can read.
type ProjectTableModel struct {
	state     ProjectTableState
	host      string
	databases []string
	slots     map[string]int // database -> header number key, for recent projects
	reason    string
	cursor    int
	width     int
	height    int
	theme     Theme
	// counts and agents arrive after the database list: issue counts per
	// database, and live agent sessions per database.
	counts     map[string]projectCounts
	agents     map[string]int
	countsRead bool
}

// projectTableCountsMsg carries the per-database columns of the table.
type projectTableCountsMsg struct {
	counts map[string]projectCounts
	agents map[string]int
}

// NewProjectTable returns a table that has not started loading.
func NewProjectTable(theme Theme) ProjectTableModel {
	return ProjectTableModel{theme: theme}
}

// State is the table's loading state.
func (t ProjectTableModel) State() ProjectTableState { return t.state }

// SetSize updates the table dimensions.
func (t *ProjectTableModel) SetSize(width, height int) {
	t.width = width
	t.height = height
}

func (t *ProjectTableModel) startLoading(host string) {
	t.state = ProjectTableLoading
	t.host = host
	t.databases = nil
	t.slots = nil
	t.reason = ""
	t.cursor = 0
}

func (t *ProjectTableModel) finishLoading(databases []string, err error, slots map[string]int) {
	if err != nil {
		t.state = ProjectTableFailed
		if errors.Is(err, errNoStartupServer) {
			t.reason = err.Error()
		} else {
			t.reason = datasource.ClassifyConnError(err).String()
		}
		return
	}
	t.state = ProjectTableLoaded
	t.databases = databases
	t.slots = slots
	t.cursor = 0
}

func (t *ProjectTableModel) moveUp() {
	if t.cursor > 0 {
		t.cursor--
	}
}

func (t *ProjectTableModel) moveDown() {
	if t.cursor < len(t.databases)-1 {
		t.cursor++
	}
}

func (t ProjectTableModel) selected() (string, bool) {
	if t.state != ProjectTableLoaded || t.cursor < 0 || t.cursor >= len(t.databases) {
		return "", false
	}
	return t.databases[t.cursor], true
}

// View renders the table as a centred box.
func (t ProjectTableModel) View() string {
	width, height := t.width, t.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 20
	}
	th := t.theme

	boxWidth := 96
	if width-10 < boxWidth {
		boxWidth = width - 10
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	titleStyle := th.Renderer.NewStyle().Foreground(th.Primary).Bold(true)
	dimStyle := th.Renderer.NewStyle().Foreground(th.Secondary)
	rowStyle := th.Renderer.NewStyle().Foreground(th.Base.GetForeground())
	cursorStyle := rowStyle.Foreground(th.Primary).Bold(true)

	title := "Projects"
	if t.host != "" {
		title = fmt.Sprintf("Projects (%s)", t.host)
	}
	lines := []string{titleStyle.Render(title), ""}

	switch t.state {
	case ProjectTableLoading:
		lines = append(lines, dimStyle.Render("Loading databases…"))
	case ProjectTableFailed:
		lines = append(lines, dimStyle.Render("Cannot list databases: "+t.reason))
	default:
		if len(t.databases) == 0 {
			lines = append(lines, dimStyle.Render("No readable Beads databases."))
		}
		nameW := boxWidth - 6 - 52
		if nameW < 12 {
			nameW = 12
		}
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  %-*s %5s %5s %9s %-11s %6s %s", nameW, "NAME", "OPEN", "WIP", "CLOSED/7d", "LAST CHANGE", "AGENTS", "RECENT")))
		now := time.Now()
		for i, database := range t.databases {
			recent := ""
			if slot := t.slots[database]; slot > 0 {
				recent = fmt.Sprintf("<%d>", slot)
			}
			prefix := "  "
			style := rowStyle
			if i == t.cursor {
				prefix = "▸ "
				style = cursorStyle
			}
			open, wip, closed, last := "…", "…", "…", "…"
			if t.countsRead {
				open, wip, closed, last = "-", "-", "-", "-"
				if c, ok := t.counts[database]; ok {
					open, wip, closed = strconv.Itoa(c.open), strconv.Itoa(c.inProgress), strconv.Itoa(c.closed7d)
					if !c.lastChange.IsZero() {
						last = fmtAge(now.Sub(c.lastChange)) + " ago"
					}
				}
			}
			agentCol := ""
			if n := t.agents[database]; n > 0 {
				agentCol = fmt.Sprintf("● %d", n)
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s%-*s %5s %5s %9s %-11s %6s %s", prefix, nameW, truncateRunesHelper(database, nameW, "…"), open, wip, closed, last, agentCol, recent)))
		}
		if t.countsRead {
			lines = append(lines, "", dimStyle.Render(t.facts()))
		}
	}

	lines = append(lines, "", dimStyle.Render("j/k: navigate • enter: open • esc: back"))

	box := th.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary).
		Padding(1, 2).
		Width(boxWidth).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// facts is the footer line of cross-project numbers.
func (t ProjectTableModel) facts() string {
	live, withAgents, closed := 0, 0, 0
	busiest, most := "", 0
	for _, database := range t.databases {
		if n := t.agents[database]; n > 0 {
			live += n
			withAgents++
		}
		c := t.counts[database]
		closed += c.closed7d
		if c.closed7d > most {
			busiest, most = database, c.closed7d
		}
	}
	out := fmt.Sprintf("%d live agents in %d projects · %d issues closed in 7 days", live, withAgents, closed)
	if busiest != "" {
		out += fmt.Sprintf(" · busiest: %s (%d)", busiest, most)
	}
	return out
}

// loadProjectTableCountsCmd reads every listed database's issue counts and
// counts the live agent sessions working in each project. A session belongs
// to a database when its project is the database, the header project's name
// or the base name of that project's checkout.
func loadProjectTableCountsCmd(projects map[string]config.Project, startupUser string, collector *agents.Collector) tea.Cmd {
	return func() tea.Msg {
		list := make([]config.Project, 0, len(projects))
		for _, p := range projects {
			list = append(list, p)
		}
		byKey, _ := probeProjects(list, startupUser)
		msg := projectTableCountsMsg{counts: map[string]projectCounts{}, agents: map[string]int{}}
		names := map[string]string{}
		for database, p := range projects {
			if c, ok := byKey[projectKey(p)]; ok {
				msg.counts[database] = c
			}
			names[strings.ToLower(database)] = database
			if p.Name != "" {
				names[strings.ToLower(p.Name)] = database
			}
			if p.Path != "" {
				names[strings.ToLower(agents.ProjectFromCwd(p.Path))] = database
			}
		}
		if collector != nil {
			snap := collector.Last()
			if snap.At.IsZero() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				snap = collector.Scan(ctx)
				cancel()
			}
			for _, s := range snap.Sessions {
				if !s.Live() {
					continue
				}
				if database, ok := names[strings.ToLower(s.Project)]; ok {
					msg.agents[database]++
				}
			}
		}
		return msg
	}
}

// loadProjectTableCmd lists the databases on host readable by user.
func loadProjectTableCmd(host, user string) tea.Cmd {
	return func() tea.Msg {
		if host == "" {
			return projectTableLoadedMsg{err: errNoStartupServer}
		}
		databases, err := datasource.ListProjectDatabases(datasource.DataSource{
			Type: datasource.SourceTypeDolt,
			Path: host,
			User: user,
		})
		return projectTableLoadedMsg{databases: databases, err: err}
	}
}

func (m Model) openProjectTable() (Model, tea.Cmd) {
	m.projectTable = NewProjectTable(m.theme)
	m.projectTable.SetSize(m.width, m.height-1)
	m.projectTable.startLoading(m.startupDoltHost)
	m.showProjectTable = true
	return m, loadProjectTableCmd(m.startupDoltHost, m.startupDoltUser)
}

func (m Model) handleProjectTableLoaded(msg projectTableLoadedMsg) (Model, tea.Cmd) {
	if !m.showProjectTable {
		return m, nil
	}
	m.projectTable.finishLoading(msg.databases, msg.err, m.projectTableSlots())
	if msg.err != nil {
		return m, nil
	}
	if m.fleetCollector == nil {
		m.fleetCollector = agents.NewCollector(m.fleetCollectorOptions())
	}
	projects := make(map[string]config.Project, len(msg.databases))
	for _, database := range msg.databases {
		projects[database] = m.projectForDatabase(database)
	}
	return m, loadProjectTableCountsCmd(projects, m.startupDoltUser, m.fleetCollector)
}

// projectTableSlots maps each database shown in the header to its number key.
func (m Model) projectTableSlots() map[string]int {
	slots := make(map[string]int)
	for i, p := range m.visibleProjects() {
		if i >= config.MaxRecentProjects {
			break
		}
		if p.Database != "" && p.Host == m.startupDoltHost {
			slots[p.Database] = i + 1
		}
	}
	return slots
}

// projectForDatabase is the header project for database when there is one, so
// a recent project keeps its checkout; otherwise a project read from the database.
func (m Model) projectForDatabase(database string) config.Project {
	for _, p := range m.allProjects {
		if p.Database == database && p.Host == m.startupDoltHost {
			return p
		}
	}
	return config.Project{Name: database, Database: database, Host: m.startupDoltHost}
}

func (m Model) handleProjectTableKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.projectTable.moveDown()
	case "k", "up":
		m.projectTable.moveUp()
	case "esc", "q":
		m.showProjectTable = false
	case "enter":
		database, ok := m.projectTable.selected()
		if !ok {
			return m, nil
		}
		m.showProjectTable = false
		project := m.projectForDatabase(database)
		return m, func() tea.Msg { return SwitchProjectMsg{Project: project} }
	}
	return m, nil
}

// rememberRecentProject records that project opened: it joins the recent list
// if new, its opened_at is updated, the list is saved, and the header rows are
// rebuilt so a new project has a number key.
func (m *Model) rememberRecentProject(project config.Project) {
	recent := config.RecentProject{Name: project.Name, Database: project.Database, Host: project.Host, Path: project.Path}
	m.appConfig.TouchRecent(recent)
	if !m.appConfig.MarkOpened(recent, time.Now()) {
		return
	}
	if err := config.SaveRecentTo(config.ConfigPath(), m.appConfig.RecentProjects, m.appConfig.LockRecent); err != nil {
		m.statusMsg = fmt.Sprintf("Could not save recent projects: %v", err)
		m.statusIsError = true
	}
	m.allProjects = headerProjects(m.appConfig.RecentProjects, m.activeRecent())
	m.projectPicker = NewProjectPicker(m.buildProjectEntries(), m.theme)
	m.projectPicker.SetSourceInfo(m.sourceInfo)
	m.projectPicker.SetSize(m.width, m.height)
}

func (m Model) forgetRecentProject(number string) Model {
	m.statusIsError = true
	slot, err := strconv.Atoi(number)
	if err != nil || len(number) != 1 || slot < 1 || slot > config.MaxRecentProjects {
		m.statusMsg = "Usage: forget <header number 1-9>"
		return m
	}
	if m.appConfig.LockRecent {
		m.statusMsg = "Recent projects are locked (lock_recent: true)"
		return m
	}
	projects := m.visibleProjects()
	if slot > len(projects) {
		m.statusMsg = fmt.Sprintf("No recent project on header key %d", slot)
		return m
	}
	project := projects[slot-1]
	recent := config.RecentProject{Name: project.Name, Database: project.Database, Host: project.Host, Path: project.Path}
	active := m.activeRecent()
	if recent.SameProject(active) || (project.Name == m.activeProjectName && project.ResolvedPath() == m.activeProjectPath) {
		m.statusMsg = "Switch to another project before forgetting the active project"
		return m
	}
	next := m.appConfig
	if !next.ForgetRecent(recent) {
		m.statusMsg = "Project is not in the recent list"
		return m
	}
	if err := config.SaveRecentTo(config.ConfigPath(), next.RecentProjects, next.LockRecent, recent); err != nil {
		m.statusMsg = fmt.Sprintf("Could not save recent projects: %v", err)
		return m
	}
	m.appConfig = next
	m.allProjects = headerProjects(next.RecentProjects, active)
	m.projectPicker = NewProjectPicker(m.buildProjectEntries(), m.theme)
	m.projectPicker.SetSourceInfo(m.sourceInfo)
	m.projectPicker.SetSize(m.width, m.height)
	m.projectPicker.SetAllProjectsMode(m.allProjectsMode)
	m.statusMsg = fmt.Sprintf("Forgot recent project %s", project.Name)
	m.statusIsError = false
	return m
}
