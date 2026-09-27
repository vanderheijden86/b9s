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

// modalRefusedBySwitch reports whether a writing overlay (edit, create,
// attach, the status picker, or a close/delete confirmation) must stay
// closed because a project switch is in flight, setting the footer message
// a refused key shows. A projectOpenedMsg landing while the overlay was
// still open would let its eventual write dispatch against the newly
// switched IssueWriter while still carrying the old project's issue ID
// (bd-grtt, bd-l66t); since projects share the bd- id prefix, that would
// silently write to (or delete) the wrong project's issue.
func (m *Model) modalRefusedBySwitch() bool {
	if m.projectSwitch.state != SwitchOpening {
		return false
	}
	m.statusMsg = fmt.Sprintf("Opening %s… try again once it opens", m.projectSwitch.project.Name)
	m.statusIsError = false
	return true
}

// discardOpenModals closes any writing overlay left open by the project
// being replaced and reports whether one was open. This is the second half
// of the bd-grtt/bd-l66t fix: modalRefusedBySwitch stops an overlay from
// opening once a switch is in flight, but one opened before the switch began
// is untouched by that guard, so applyProjectSwitch calls this to close it
// the moment the new project actually replaces the old one. The status
// picker and confirmation are closed here too, in addition to being covered
// by writeAllowedForGeneration on their own write paths, so the footer never
// shows a stale picker/confirmation for a project no longer on screen.
func (m *Model) discardOpenModals() bool {
	discarded := m.showEditModal || m.showAttachAddModal || m.showStatusPicker || m.issueConfirm.action != issueConfirmNone
	if !discarded {
		return false
	}
	if m.focused == focusEditModal {
		// List-mode "e" (model.go) is the only path that moves focus here;
		// cancelling the edit modal restores the same value, so a discard
		// must do it too or list mode is stuck unable to receive keys.
		m.focused = focusList
	}
	m.showEditModal = false
	m.showAttachAddModal = false
	m.showStatusPicker = false
	m.statusTargets = nil
	m.issueConfirm = issueConfirmation{}
	return true
}

// writeAllowedForGeneration reports whether generation still matches the
// active project, refusing the write and setting the footer message
// otherwise. Every write path fed by an overlay that captured an issue ID
// against a specific project (edit modal, attach form, status picker,
// close/delete confirmation) must call this with the generation the overlay
// stamped at open time before dispatching. Without it, an overlay opened
// before a project switch began and still open when the switch lands would
// dispatch its write against the newly switched IssueWriter while carrying
// the old project's issue ID; since projects share the bd- id prefix, that
// silently writes to (or deletes) the wrong project's issue (bd-l66t).
// discardOpenModals already closes the overlay itself the moment a switch
// completes, so this is the second line of defense for a write already in
// flight when that happens, and the only defense for an overlay type a
// future change forgets to add to discardOpenModals.
func (m *Model) writeAllowedForGeneration(generation uint64) bool {
	if generation == m.projectGeneration {
		return true
	}
	m.statusMsg = "project changed; action discarded"
	m.statusIsError = false
	return false
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
	discardedModal := m.discardOpenModals()
	if m.allProjectsMode {
		m.leaveAllProjectsMode()
	}
	// Switch to a different project (bd-q5z, bd-ey3, bd-87w)
	m.activeProjectName = project.Name
	m.activeProjectPath = project.ResolvedPath()
	// Every overlay open against the previous project stamped the generation
	// this bump invalidates; discardOpenModals above already closed the ones
	// it knows about, and writeAllowedForGeneration refuses any write from
	// one still in flight (bd-l66t).
	m.projectGeneration++
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
	if discardedModal {
		m.statusMsg = fmt.Sprintf("Switched to %s (edit discarded: project switched)", project.Name)
	} else {
		m.statusMsg = fmt.Sprintf("Switched to %s", project.Name)
	}
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
