package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/internal/attach"
	"github.com/vanderheijden86/beadwork/internal/bdrun"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/pkg/debug"
)

// attachAddTimeout bounds internal/attach.Add: it never blocks Update, but a
// stuck upload or bd invocation still needs a deadline so a stalled network
// call cannot hang the run indefinitely.
const attachAddTimeout = 2 * time.Minute

// AttachAddModal is the huh form the "I" key opens over the selected issue
// (bd-t8j5.10): one multi-line field for the paths to attach. It follows
// EditModal's shape (a *huh.Form field, ctrl+s/esc caught ahead of the form
// so huh never sees them) rather than reusing EditModal itself, since the
// form has one field instead of nine and no dirty-tracking against original
// values.
type AttachAddModal struct {
	form    *huh.Form
	theme   Theme
	issueID string

	// paths MUST be *string (not string), so huh's value binding survives
	// Bubbletea's value-receiver copy semantics; see EditModal's title field
	// for the same reasoning.
	paths *string

	width, height int

	// initCmd stores the tea.Cmd from form.Init() called during construction,
	// returned by Init() for callers that can propagate cmds.
	initCmd tea.Cmd

	submitRequested bool
	cancelRequested bool
}

// NewAttachAddModal creates the attach-files form for issueID.
func NewAttachAddModal(issueID string, theme Theme) AttachAddModal {
	paths := ""
	m := AttachAddModal{theme: theme, issueID: issueID, paths: &paths}
	m.form = buildAttachAddForm(&m)
	m.initCmd = m.form.Init()
	return m
}

func buildAttachAddForm(m *AttachAddModal) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title("Attach files to " + m.issueID).
				Description("One path per line, or several separated by spaces " +
					`(quote a path with a space, e.g. "my file.txt"). ` +
					"A leading ~ expands to the home directory.").
				Value(m.paths).
				Lines(6),
		),
	).WithTheme(huh.ThemeDracula()).
		WithKeyMap(editFormKeyMap()).
		WithShowHelp(true).
		WithShowErrors(true)
}

// Init returns the stored init command from form construction.
func (m AttachAddModal) Init() tea.Cmd {
	return m.initCmd
}

// Update handles input for the attach-add modal. ctrl+s submits and esc
// cancels ahead of the huh.Form (editFormKeyMap leaves huh's own Submit
// unbound), the same discipline EditModal.Update uses.
func (m AttachAddModal) Update(msg tea.Msg) (AttachAddModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			m.submitRequested = true
			return m, nil
		case "esc":
			m.cancelRequested = true
			return m, nil
		}
	}

	form, cmd := m.form.Update(msg)
	m.form = form.(*huh.Form)
	if m.form.State == huh.StateCompleted {
		m.submitRequested = true
	}
	return m, cmd
}

// View renders the attach-add modal.
func (m AttachAddModal) View() string {
	r := m.theme.Renderer

	headerStyle := r.NewStyle().Bold(true).Foreground(m.theme.Primary)

	boxWidth := m.width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}

	var content strings.Builder
	content.WriteString(headerStyle.Render("Attach Files"))
	content.WriteString("\n\n")
	content.WriteString(m.form.View())

	boxStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(boxWidth)

	box := boxStyle.Render(content.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// SetSize sets the modal dimensions.
func (m *AttachAddModal) SetSize(width, height int) {
	m.width = width
	m.height = height

	boxWidth := width - 10
	if boxWidth < 60 {
		boxWidth = 60
	}
	if boxWidth > 80 {
		boxWidth = 80
	}
	m.form = m.form.WithWidth(boxWidth - 6)
}

// IsSubmitRequested returns true once ctrl+s was pressed or the form otherwise completed.
func (m AttachAddModal) IsSubmitRequested() bool {
	return m.submitRequested
}

// IsCancelRequested returns true once esc was pressed.
func (m AttachAddModal) IsCancelRequested() bool {
	return m.cancelRequested
}

// RawPaths returns the form's current path text, unparsed.
func (m AttachAddModal) RawPaths() string {
	if m.paths == nil {
		return ""
	}
	return *m.paths
}

// parseAttachPaths splits raw into individual file paths. Each line may hold
// one path, or several separated by spaces; a path containing a space
// survives single- or double-quoting. Blank lines are skipped, and a leading
// ~ or ~/ expands to the user's home directory.
func parseAttachPaths(raw string) ([]string, error) {
	var paths []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields, err := splitQuotedFields(line)
		if err != nil {
			return nil, err
		}
		for _, f := range fields {
			if f == "" {
				continue
			}
			paths = append(paths, expandHomePath(f))
		}
	}
	return paths, nil
}

// splitQuotedFields splits line on whitespace, treating a single- or
// double-quoted run as one field, so a path containing a space can be typed
// as "my file.txt" or 'my file.txt'.
func splitQuotedFields(line string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inField := false
	var quote rune
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inField = true
		case r == ' ' || r == '\t':
			if inField {
				fields = append(fields, cur.String())
				cur.Reset()
				inField = false
			}
		default:
			cur.WriteRune(r)
			inField = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", line)
	}
	if inField {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

// expandHomePath expands a leading ~ or ~/ to the user's home directory. A
// home directory lookup failure leaves p unchanged rather than erroring:
// the path is then handed to os.Open as-is, which reports its own error.
func expandHomePath(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// openAttachAddModal opens the attach-files form over the selected issue.
// Unlike openAttachmentPicker, it does not check attachments configuration
// up front: like R's picker (which lists reference comments with no blob
// store involved), the not-configured refusal is deferred to submission,
// where attachAddCmd resolves the blob store the same way attachmentOpenCmd
// does.
func (m *Model) openAttachAddModal() {
	if m.allProjectsMode {
		m.statusMsg = "attachments: not available in all-projects mode; switch to one project (0 or :project) first"
		m.statusIsError = false
		return
	}
	issue := m.getSelectedIssue()
	if issue == nil {
		return
	}
	m.attachAddModal = NewAttachAddModal(issue.ID, m.theme)
	if m.width > 0 && m.height > 1 {
		m.attachAddModal.SetSize(m.width, m.height-1)
	}
	m.showAttachAddModal = true
}

// attachAddResultMsg carries the outcome of attachAddCmd back into Update.
type attachAddResultMsg struct {
	handle      *blobstore.Handle
	projectPath string
	// dispatchProjectPath is the active project when attachAddCmd was
	// dispatched. Update drops a result whose dispatchProjectPath no longer
	// matches the active project, the same staleness guard
	// attachmentOpenResultMsg uses: a :project switch that lands between
	// dispatch and here means this result names the wrong project's blob
	// store and the wrong project's footer message.
	dispatchProjectPath string

	issueID   string
	statusMsg string
	isError   bool
	// reload is true when at least one file attached (or was already
	// stored), so Update triggers the same FileChangedMsg reload other bd
	// writes trigger; a run that attached nothing has no comment to reload for.
	reload bool
}

// bdRunnerForAttach returns a BdRunner bound to the same bd binary and
// checkout IssueWriter uses, or an error naming why writes are refused: the
// same three checks IssueWriter.runBdCmd applies (issue_writer.go), reused
// here because internal/attach.Add needs a BdRunner rather than a tea.Cmd.
func bdRunnerForAttach(w *IssueWriter) (attach.BdRunner, error) {
	if w == nil || !w.available {
		return nil, fmt.Errorf("bd CLI not found in PATH; install beads to edit issues")
	}
	if w.opening != "" {
		return nil, fmt.Errorf("edits wait until %s has opened", w.opening)
	}
	dir := w.checkout.Dir()
	if dir == "" {
		return nil, fmt.Errorf("read-only: no local checkout for this project")
	}
	bdPath := w.bdPath
	return attach.RunnerFunc(func(args ...string) (string, error) {
		return bdrun.Run(bdPath, dir, args...)
	}), nil
}

// attachAddCmd runs internal/attach.Add for paths against issueID, reusing
// the lazily cached blob store handle attachmentOpenCmd also uses and a bd
// runner built from IssueWriter's resolved binary and checkout. It never
// blocks Update: every network or subprocess call runs inside the returned
// closure, bounded by attachAddTimeout.
func (m Model) attachAddCmd(issueID string, paths []string) tea.Cmd {
	handle := m.attachHandle
	handleProject := m.attachHandleProject
	projectPath := m.activeProjectPath
	projectName := m.activeProjectName
	sourceType := m.sourceType
	doltSource := m.doltSource
	allProjectsMode := m.allProjectsMode
	writer := m.issueWriter

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), attachAddTimeout)
		defer cancel()
		result := attachAddResultMsg{issueID: issueID, dispatchProjectPath: projectPath}

		if allProjectsMode {
			// Belt-and-suspenders backstop, mirroring attachmentOpenCmd:
			// openAttachAddModal already refuses this at open time.
			result.statusMsg = "attachments: not available in all-projects mode; switch to one project (0 or :project) first"
			result.isError = true
			return result
		}

		bdRunner, err := bdRunnerForAttach(writer)
		if err != nil {
			result.statusMsg = fmt.Sprintf("attach: %v", err)
			result.isError = true
			return result
		}

		if handle == nil || handleProject != projectPath {
			h, err := openProjectBlobStore(ctx, projectPath, projectName, sourceType, doltSource)
			if err != nil {
				result.statusMsg = attachmentOpenErrorMsg(err)
				result.isError = true
				return result
			}
			handle = h
			result.handle = h
			result.projectPath = projectPath
		}

		results := attach.Add(ctx, handle, bdRunner, issueID, paths)
		debug.Log("attach-add: %s: %d result(s)", issueID, len(results))
		msgText, isErr, reload := summarizeAttachAddResults(results)
		result.statusMsg = msgText
		result.isError = isErr
		result.reload = reload
		return result
	}
}

// summarizeAttachAddResults folds internal/attach.Add's per-file results
// into one footer line: counts of attached / already stored / failed, with
// the first failure's error text appended, sanitized the same way any other
// value from outside b9s's own trusted input reaches the footer.
func summarizeAttachAddResults(results []attach.AddResult) (msg string, isError bool, reload bool) {
	var attached, skipped, failed int
	var firstErr string
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
			if firstErr == "" {
				firstErr = sanitizeTerminalLine(r.Err.Error())
			}
		case r.Skipped:
			skipped++
		default:
			attached++
		}
	}

	var parts []string
	if attached > 0 {
		parts = append(parts, fmt.Sprintf("%d attached", attached))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d already stored", skipped))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	msg = strings.Join(parts, ", ")
	if msg == "" {
		msg = "no files attached"
	}
	if failed > 0 {
		msg = fmt.Sprintf("%s: %s", msg, firstErr)
	}
	return msg, failed > 0, attached+skipped > 0
}
