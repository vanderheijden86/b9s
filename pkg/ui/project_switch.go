package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/watcher"
)

// projectOpenFunc loads a project to switch to; datasource.OpenProject outside tests.
type projectOpenFunc func(datasource.OpenTarget) (datasource.OpenedProject, *datasource.OpenFailure)

// SwitchState is the project switch lifecycle. While Opening, the current
// project stays on screen and browsable; it is replaced only once the new
// project has loaded.
type SwitchState uint8

const (
	SwitchIdle SwitchState = iota
	SwitchOpening
)

// projectOpenDeadline bounds Opening. The Dolt connection's own 5s connect
// timeout sits inside it.
const projectOpenDeadline = 10 * time.Second

type projectSwitch struct {
	state      SwitchState
	generation uint64 // results from any other generation are stale
	project    config.Project
	target     datasource.OpenTarget
}

type projectOpenedMsg struct {
	generation uint64
	project    config.Project
	opened     datasource.OpenedProject
}

type projectOpenFailedMsg struct {
	generation uint64
	project    config.Project
	failure    *datasource.OpenFailure
}

type projectOpenDeadlineMsg struct {
	generation uint64
}

// beginProjectSwitch enters Opening. Every request to switch comes through
// here, a number key, the :project table or a retry, so each one starts the
// load and arms the deadline.
func (m Model) beginProjectSwitch(project config.Project) (Model, tea.Cmd) {
	m.projectSwitch.generation++
	generation := m.projectSwitch.generation
	target := m.openTargetFor(project)
	m.projectSwitch.state = SwitchOpening
	m.projectSwitch.project = project
	m.projectSwitch.target = target
	m.issueWriter.SetOpening(project.Name)
	m.statusMsg = fmt.Sprintf("Opening %s… (esc cancels)", project.Name)
	m.statusIsError = false

	open := m.openProject
	return m, tea.Batch(
		func() tea.Msg {
			opened, failure := open(target)
			if failure != nil {
				return projectOpenFailedMsg{generation: generation, project: project, failure: failure}
			}
			return projectOpenedMsg{generation: generation, project: project, opened: opened}
		},
		tea.Tick(projectOpenDeadline, func(time.Time) tea.Msg {
			return projectOpenDeadlineMsg{generation: generation}
		}),
	)
}

// openTargetFor opens a project from its checkout when it has one, and
// otherwise from its database as the startup project's Dolt user, the only
// credential b9s holds for it.
func (m Model) openTargetFor(project config.Project) datasource.OpenTarget {
	path := project.ResolvedPath()
	if _, ok := NewCheckout(path); ok {
		return datasource.OpenTarget{Name: project.Name, Dir: path}
	}
	if project.Database != "" {
		source := databaseSource(project, m.startupDoltUser)
		return datasource.OpenTarget{Name: project.Name, Dolt: &source}
	}
	return datasource.OpenTarget{Name: project.Name, Dir: path}
}

func (m Model) isCurrentSwitch(generation uint64) bool {
	return m.projectSwitch.state == SwitchOpening && generation == m.projectSwitch.generation
}

func (m *Model) settleSwitch() {
	m.projectSwitch.state = SwitchIdle
	m.issueWriter.SetOpening("")
}

func (m Model) handleProjectOpened(msg projectOpenedMsg) (Model, tea.Cmd) {
	if !m.isCurrentSwitch(msg.generation) {
		return m, nil
	}
	m.settleSwitch()
	m, cmd := m.applyProjectSwitch(msg.project)
	m.rememberRecentProject(msg.project)
	return m, cmd
}

func (m Model) handleProjectOpenFailed(msg projectOpenFailedMsg) Model {
	if !m.isCurrentSwitch(msg.generation) {
		return m
	}
	m.settleSwitch()
	m.showOpenFailurePopup(msg.project, msg.failure, false)
	return m
}

func (m Model) handleProjectOpenDeadline(msg projectOpenDeadlineMsg) Model {
	if !m.isCurrentSwitch(msg.generation) {
		return m
	}
	project := m.projectSwitch.project
	failure := &datasource.OpenFailure{Reason: datasource.OpenTimedOut, Project: project.Name, Dir: m.projectSwitch.target.Dir}
	if source := m.projectSwitch.target.Dolt; source != nil {
		failure.Server, failure.Database, failure.User = source.Path, source.Database, source.User
	}
	m.settleSwitch()
	m.showOpenFailurePopup(project, failure, false)
	return m
}

// cancelProjectSwitch leaves Opening. The load keeps running, but its result
// no longer matches an Opening switch and is dropped.
func (m Model) cancelProjectSwitch() Model {
	name := m.projectSwitch.project.Name
	m.settleSwitch()
	m.statusMsg = fmt.Sprintf("Stopped opening %s", name)
	m.statusIsError = false
	return m
}

// applyProjectSwitch replaces the visible project with one that has opened.
// It is the only place that swaps the active project's data and watchers.
func (m Model) applyProjectSwitch(project config.Project) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.allProjectsMode {
		m.leaveAllProjectsMode()
	}
	// Switch to a different project (bd-q5z, bd-ey3, bd-87w)
	m.activeProjectName = project.Name
	m.activeProjectPath = project.ResolvedPath()
	checkout, hasCheckout := NewCheckout(project.ResolvedPath())
	m.issueWriter.SetCheckout(checkout)
	m.board.SetActiveProjectName(project.Name)
	m.updateListDelegate()
	beadsDir, _ := projectBeadsDir(project.ResolvedPath())
	sources, discErr := m.projectSources(project)
	hasDoltSource := false
	for _, source := range sources {
		if source.Type == datasource.SourceTypeDolt {
			hasDoltSource = true
			break
		}
	}

	newPath := ""
	if hasCheckout {
		// Dolt server projects are fully described by metadata.json and need no
		// local JSONL file. Keep the conventional path as an inert fallback value;
		// Dolt reloads use activeProjectPath instead.
		var err error
		newPath, err = loader.FindJSONLPath(beadsDir)
		if err != nil && !hasDoltSource {
			m.statusMsg = fmt.Sprintf("No beads found in %s", project.Name)
			m.statusIsError = true
			return m, nil
		}
		if err != nil {
			newPath = filepath.Join(beadsDir, "issues.jsonl")
		}
	} else if !hasDoltSource {
		m.statusMsg = fmt.Sprintf("No beads found in %s", project.Name)
		m.statusIsError = true
		return m, nil
	}
	// Stop background worker and old watchers (bd-87w)
	if m.backgroundWorker != nil {
		m.backgroundWorker.Stop()
		m.backgroundWorker = nil
	}
	if m.watcher != nil {
		m.watcher.Stop()
		m.watcher = nil
	}
	if m.doltWatcher != nil {
		m.doltWatcher.Stop()
		m.doltWatcher = nil
	}
	m.beadsPath = newPath

	// Re-discover datasource for the new project
	m.sourceType = datasource.SourceTypeJSONLLocal
	m.doltSource = datasource.DataSource{}
	m.doltFailure = nil
	m.sourceInfo = fmt.Sprintf("jsonl %s", filepath.Base(newPath))
	if discErr == nil {
		for _, s := range sources {
			if s.Type == datasource.SourceTypeDolt {
				db := s.Database
				if db == "" {
					db = "beads"
				}
				doltLabel := fmt.Sprintf("dolt://%s/%s", s.Path, db)
				dw, dwErr := datasource.NewDoltWatcher(s, m.doltPollInterval)
				if dwErr != nil {
					m.doltFailure = &DoltFailure{Server: s.Path, Database: db, User: s.User, Error: dwErr.Error()}
				} else if err := dw.Start(); err != nil {
					dw.Stop()
					m.doltFailure = &DoltFailure{Server: s.Path, Database: db, User: s.User, Error: err.Error()}
				} else {
					m.doltWatcher = dw
					m.doltSource = s
					m.sourceType = datasource.SourceTypeDolt
					m.sourceInfo = doltLabel + " ✓"
				}
				break
			} else if s.Type == datasource.SourceTypeSQLite {
				m.sourceInfo = fmt.Sprintf("sqlite %s", filepath.Base(s.Path))
			}
		}
	}
	debug.Log("project-switch: %s sourceType=%s sourceInfo=%s", project.Name, m.sourceType, m.sourceInfo)
	debug.Log("project-switch: preserving filters: currentFilter=%q labelFilter=%q assigneeFilter=%q pickerMode=%d", m.currentFilter, m.labelFilter, m.assigneeFilter, m.pickerMode)

	// Clear old project data to prevent stale rendering (bd-lll, bd-134a)
	m.issues = nil
	m.issueMap = nil
	m.snapshot = nil
	m.isLoading = true
	m.countOpen, m.countReady, m.countBlocked, m.countClosed = 0, 0, 0, 0
	// Clear the list immediately so stale items are gone (bd-134a)
	m.list.SetItems(nil)
	m.board.SetIssues(nil)
	// Preserve label/assignee filters across project switches (bd-v2lx)
	// Only clear picker entries (will be rebuilt from new project's issues)
	m.currentFilter = "all"
	m.labelEntries = nil
	m.labelScrollOffset = 0
	m.assigneeEntries = nil
	m.assigneeScrollOffset = 0
	// Keep tree filters in sync with preserved label/assignee filters
	m.tree.ApplyFilter("all")
	m.tree.ClearSearch()
	m.tree.Build(nil)
	// Start new background worker or watcher for the new project
	bw, bwErr := NewBackgroundWorker(WorkerConfig{BeadsPath: newPath})
	if bwErr == nil && m.sourceType != datasource.SourceTypeDolt {
		m.backgroundWorker = bw
		cmds = append(cmds, StartBackgroundWorkerCmd(bw))
		cmds = append(cmds, WaitForBackgroundWorkerMsgCmd(bw))
	} else if m.doltWatcher != nil {
		// Dolt project: use DoltWatcher for live reload
		cmds = append(cmds, DoltWatchCmd(m.doltWatcher))
		cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
	} else {
		// Fallback: file watcher
		w, watchErr := watcher.NewWatcher(newPath)
		if watchErr == nil {
			m.watcher = w
			cmds = append(cmds, WatchFileCmd(w))
		}
		cmds = append(cmds, func() tea.Msg { return FileChangedMsg{} })
	}
	m.statusMsg = fmt.Sprintf("Switched to %s", project.Name)
	m.statusIsError = false
	// Rebuild picker entries to reflect new active project (bd-ey3)
	entries := m.buildProjectEntries()
	m.projectPicker = NewProjectPicker(entries, m.theme)
	m.projectPicker.SetSourceInfo(m.sourceInfo)
	m.projectPicker.SetSize(m.width, m.height)
	return m, tea.Batch(cmds...)
}

// WithStartupFailure shows why the folder b9s was started in did not open,
// after b9s fell back to the project that last opened successfully.
func (m Model) WithStartupFailure(f *datasource.OpenFailure) Model {
	if f == nil {
		return m
	}
	project := config.Project{Name: f.Project, Path: f.Dir, Database: f.Database, Host: f.Server}
	m.showOpenFailurePopup(project, f, true)
	return m
}

func (m *Model) showOpenFailurePopup(project config.Project, f *datasource.OpenFailure, fellBack bool) {
	m.openFailure = f
	m.openFailureProject = project
	m.openFailureFellBack = fellBack
	m.showOpenFailure = true
	m.statusMsg = fmt.Sprintf("Could not open %s", project.Name)
	m.statusIsError = true
}

func (m *Model) closeOpenFailure() {
	m.showOpenFailure = false
	m.openFailure = nil
}

// handleOpenFailureKeys owns every key while the failure popup is open.
func (m Model) handleOpenFailureKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		project := m.openFailureProject
		m.closeOpenFailure()
		return m.beginProjectSwitch(project)
	case "esc", "enter", "q":
		m.closeOpenFailure()
	}
	return m, nil
}

// renderOpenFailurePopup explains a failed open in the words of its reason,
// with next steps and which project is on screen instead.
func (m Model) renderOpenFailurePopup() string {
	f := m.openFailure
	if f == nil {
		return ""
	}
	th := m.theme
	boxWidth := m.width - 8
	if boxWidth > 96 {
		boxWidth = 96
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	titleStyle := th.Renderer.NewStyle().Foreground(th.Primary).Bold(true)
	labelStyle := th.Renderer.NewStyle().Bold(true)
	dimStyle := th.Renderer.NewStyle().Foreground(th.Secondary)

	name := m.openFailureProject.Name
	if name == "" {
		name = f.Project
	}
	lines := []string{titleStyle.Render("Cannot open " + name), "", f.Message()}
	if f.Err != nil {
		lines = append(lines, "", dimStyle.Render("Detail  "+f.Err.Error()))
	}
	lines = append(lines, "", labelStyle.Render("Try"))
	for _, step := range f.Try() {
		lines = append(lines, "  "+step)
	}
	lines = append(lines, "")
	if m.openFailureFellBack {
		lines = append(lines, fmt.Sprintf("Showing %s, the last project that opened successfully.", m.activeProjectName))
	} else {
		lines = append(lines, fmt.Sprintf("Still showing %s.", m.activeProjectName))
	}
	lines = append(lines, "", dimStyle.Render("r retry • esc close"))

	box := th.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.Primary).
		Padding(1, 2).
		Width(boxWidth).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height-1, lipgloss.Center, lipgloss.Center, box)
}
