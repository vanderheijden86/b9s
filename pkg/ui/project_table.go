package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
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

	boxWidth := 56
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
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  %-40s %s", "NAME", "RECENT")))
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
			lines = append(lines, style.Render(fmt.Sprintf("%s%-40s %s", prefix, database, recent)))
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

func (m Model) handleProjectTableLoaded(msg projectTableLoadedMsg) Model {
	if !m.showProjectTable {
		return m
	}
	m.projectTable.finishLoading(msg.databases, msg.err, m.projectTableSlots())
	return m
}

// projectTableSlots maps each database already in the header to its number key.
func (m Model) projectTableSlots() map[string]int {
	slots := make(map[string]int)
	for i, p := range m.allProjects {
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
	m.allProjects = headerProjects(m.appConfig.RecentProjects, m.activeProjectName, m.activeProjectPath)
	m.projectPicker = NewProjectPicker(m.buildProjectEntries(), m.theme)
	m.projectPicker.SetSourceInfo(m.sourceInfo)
	m.projectPicker.SetSize(m.width, m.height)
}
