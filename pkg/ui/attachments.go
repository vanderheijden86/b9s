package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanderheijden86/beadwork/internal/attach"
	"github.com/vanderheijden86/beadwork/internal/attachref"
	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
)

// AttachmentPickerModel lists one issue's attachments for the R key
// (bd-t8j5.9) to open. It follows StatusPickerModel's shape: a plain struct,
// no bubbles list, rendered as a centered bordered box.
type AttachmentPickerModel struct {
	attachments   []attachref.Attachment
	selectedIndex int
	width, height int
	theme         Theme
}

// NewAttachmentPickerModel creates a picker over atts, which must be
// non-empty; the caller (openAttachmentPicker) is responsible for not
// opening the picker when an issue has no attachments.
func NewAttachmentPickerModel(atts []attachref.Attachment, theme Theme) AttachmentPickerModel {
	return AttachmentPickerModel{attachments: atts, theme: theme}
}

// SetSize updates the picker dimensions.
func (m *AttachmentPickerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// MoveUp moves the selection up.
func (m *AttachmentPickerModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves the selection down.
func (m *AttachmentPickerModel) MoveDown() {
	if m.selectedIndex < len(m.attachments)-1 {
		m.selectedIndex++
	}
}

// Selected returns the highlighted attachment, or false if the list is
// empty (which NewAttachmentPickerModel's caller never passes).
func (m *AttachmentPickerModel) Selected() (attachref.Attachment, bool) {
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.attachments) {
		return attachref.Attachment{}, false
	}
	return m.attachments[m.selectedIndex], true
}

// Count returns how many attachments the picker lists.
func (m *AttachmentPickerModel) Count() int {
	return len(m.attachments)
}

// View renders the attachment picker overlay.
func (m *AttachmentPickerModel) View() string {
	if m.width == 0 {
		m.width = 60
	}
	if m.height == 0 {
		m.height = 20
	}
	t := m.theme

	boxWidth := 50
	if m.width < 60 {
		boxWidth = m.width - 10
	}
	if boxWidth < 25 {
		boxWidth = 25
	}

	var lines []string
	titleStyle := t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).MarginBottom(1)
	lines = append(lines, titleStyle.Render("Attachments"))
	lines = append(lines, "")

	for i, a := range m.attachments {
		isSelected := i == m.selectedIndex
		itemStyle := t.Renderer.NewStyle()
		if isSelected {
			itemStyle = itemStyle.Foreground(t.Primary).Bold(true)
		} else {
			itemStyle = itemStyle.Foreground(t.Base.GetForeground())
		}
		prefix := "  "
		if isSelected {
			prefix = "> "
		}
		name := sanitizeTerminalLine(a.Name)
		meta := fmt.Sprintf("%s, %s", sanitizeTerminalLine(a.Type), formatBytes(a.Size))
		lines = append(lines, itemStyle.Render(prefix+name)+
			t.Renderer.NewStyle().Foreground(t.Muted).Render(" ("+meta+")"))
	}

	lines = append(lines, "")
	footerStyle := t.Renderer.NewStyle().Foreground(t.Secondary).Italic(true)
	lines = append(lines, footerStyle.Render("j/k: navigate | enter: open | esc: cancel"))

	content := strings.Join(lines, "\n")
	boxStyle := t.Renderer.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 2).
		Width(boxWidth)
	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// openAttachmentPicker opens the attachment picker over the selected
// issue's attachments, parsed from its comments alone (attachref.Collect
// needs no blob store configuration), so a project with no attachments:
// section still shows the list; only Enter needs the store.
func (m *Model) openAttachmentPicker() {
	issue := m.getSelectedIssue()
	if issue == nil {
		return
	}
	atts := attachref.Collect(issue.Comments)
	if len(atts) == 0 {
		m.statusMsg = "no attachments on " + issue.ID
		m.statusIsError = false
		return
	}
	m.attachmentPicker = NewAttachmentPickerModel(atts, m.theme)
	if m.width > 0 && m.height > 1 {
		m.attachmentPicker.SetSize(m.width, m.height-1)
	}
	m.attachIssueID = issue.ID
	m.showAttachmentPicker = true
}

// attachmentOpenRecord is the test-visible record of the most recent
// attachment-open attempt, including the opener command that ran or, under
// B9S_TEST_MODE, would have run, so a test can assert on it without a real
// opener or browser (AGENTS.md: tests never open a browser or an editor).
type attachmentOpenRecord struct {
	AttachmentName string
	OpenerCmd      []string
	Path           string
	StatusMsg      string
	IsError        bool
}

// attachmentOpenResultMsg carries the outcome of attachmentOpenCmd back into
// Update. handle is non-nil only when this run opened a fresh blob store
// handle, so Update caches it; a run that reused the cached handle leaves it
// nil rather than re-sending an unchanged pointer.
type attachmentOpenResultMsg struct {
	handle      *blobstore.Handle
	projectPath string

	attachmentName string
	openerCmd      []string
	openedPath     string
	statusMsg      string
	isError        bool
}

// refusedOpenContentTypes are sniffed base MIME types attachmentOpenCmd
// refuses to hand to the system opener, because the comment's type= field is
// an untrusted claim and the opener would otherwise run whatever the OS
// associates with the sniffed type. image/svg+xml never appears from
// http.DetectContentType (it has no signature), but is listed for defense in
// depth in case a future sniffer version adds one.
var refusedOpenContentTypes = map[string]bool{
	"text/html":             true,
	"application/xhtml+xml": true,
	"image/svg+xml":         true,
	"text/xml":              true,
	"application/xml":       true,
}

func isRefusedOpenContentType(sniffed string) bool {
	base, _, err := mime.ParseMediaType(sniffed)
	if err != nil {
		base = sniffed
	}
	return refusedOpenContentTypes[base]
}

// systemOpenerCommand names the OS's file opener and its argument, run with
// the path as a single argv element and no shell, so a crafted file name
// can never be interpreted as a second command.
func systemOpenerCommand(path string) (string, []string) {
	if runtime.GOOS == "darwin" {
		return "open", []string{path}
	}
	return "xdg-open", []string{path}
}

// osc8Hyperlink renders url as an OSC 8 terminal hyperlink labeled label,
// for the B9S_WEB=1 path: a presigned URL is printed rather than opened,
// since a web session has no local file system for the download to land in.
func osc8Hyperlink(url, label string) string {
	return "\x1b]8;;" + url + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

const notConfiguredHint = `attachments are not configured for this project; add an attachments: section (README.md, section "Attachments")`

// attachmentOpenCmd downloads and opens att, or (B9S_WEB=1) prints a
// presigned link, returning its result as a tea.Msg rather than blocking
// Update, whose receiver is by value (pkg/ui architecture: every network or
// subprocess call runs inside the returned closure).
func (m Model) attachmentOpenCmd(att attachref.Attachment) tea.Cmd {
	handle := m.attachHandle
	handleProject := m.attachHandleProject
	projectPath := m.activeProjectPath
	projectName := m.activeProjectName
	sourceType := m.sourceType
	doltSource := m.doltSource
	webMode := os.Getenv("B9S_WEB") != ""
	testMode := os.Getenv("B9S_TEST_MODE") != ""

	return func() tea.Msg {
		ctx := context.Background()
		result := attachmentOpenResultMsg{attachmentName: att.Name}

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

		if webMode {
			return attachmentOpenWeb(ctx, handle, att, result)
		}
		return attachmentOpenDownload(ctx, handle, att, testMode, result)
	}
}

// openProjectBlobStore loads config and opens the blob store for one
// project. It is the TUI counterpart of cmd/b9s/attach.go's newAttachEnv,
// built from fields the Model already holds instead of re-resolving the
// project from the current working directory.
func openProjectBlobStore(ctx context.Context, projectPath, projectName string, sourceType datasource.SourceType, doltSource datasource.DataSource) (*blobstore.Handle, error) {
	appCfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading b9s config: %w", err)
	}
	if appCfg.Attachments == nil {
		return nil, blobstore.ErrNotConfigured
	}
	beadsDir, ok := projectBeadsDir(projectPath)
	if !ok {
		return nil, fmt.Errorf("attachments: no .beads directory for %s", projectPath)
	}
	var src datasource.DataSource
	if sourceType == datasource.SourceTypeDolt {
		src = doltSource
	} else {
		src = datasource.DataSource{Type: sourceType}
	}
	srcInfo, err := blobstore.SourceInfoFromDataSource(src, beadsDir, projectName)
	if err != nil {
		return nil, err
	}
	return blobstore.Open(ctx, appCfg.Attachments, srcInfo)
}

func attachmentOpenErrorMsg(err error) string {
	if errors.Is(err, blobstore.ErrNotConfigured) {
		return notConfiguredHint
	}
	return fmt.Sprintf("attachments: %v", err)
}

// attachmentOpenWeb handles B9S_WEB=1: a presigned link is printed rather
// than downloaded, since a web session has no local file system for the
// opener to reach.
func attachmentOpenWeb(ctx context.Context, handle *blobstore.Handle, att attachref.Attachment, result attachmentOpenResultMsg) attachmentOpenResultMsg {
	if _, ok := handle.Store.(*blobstore.S3); !ok {
		result.statusMsg = `attachments use the local backend; presigned links need the s3 backend (README.md, section "Attachments")`
		result.isError = true
		return result
	}
	url, err := attach.URL(ctx, handle, att)
	if err != nil {
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}
	result.statusMsg = osc8Hyperlink(url, att.Name)
	return result
}

// attachmentOpenDownload downloads att into a private per-run temp
// directory, sniffs the content actually downloaded (the comment's type=
// field is an untrusted claim), and runs the system opener unless the
// sniffed type is script-capable or B9S_TEST_MODE is set, in which case the
// would-be command is recorded instead of run.
func attachmentOpenDownload(ctx context.Context, handle *blobstore.Handle, att attachref.Attachment, testMode bool, result attachmentOpenResultMsg) attachmentOpenResultMsg {
	dir, err := os.MkdirTemp(os.TempDir(), "b9s-attach-*")
	if err != nil {
		result.statusMsg = fmt.Sprintf("attachments: creating temp dir: %v", err)
		result.isError = true
		return result
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		os.RemoveAll(dir)
		result.statusMsg = fmt.Sprintf("attachments: securing temp dir: %v", err)
		result.isError = true
		return result
	}

	path, err := attach.Download(ctx, handle, att, dir, attach.DownloadModeRestricted)
	if err != nil {
		os.RemoveAll(dir)
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}
	result.openedPath = path

	sniffed, err := sniffContentType(path)
	if err != nil {
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}

	if isRefusedOpenContentType(sniffed) {
		result.statusMsg = fmt.Sprintf("%s downloaded to %s (opening refused: sniffed as %s)", att.Name, path, sniffed)
		return result
	}

	opener, args := systemOpenerCommand(path)
	result.openerCmd = append([]string{opener}, args...)
	if testMode {
		result.statusMsg = fmt.Sprintf("%s downloaded to %s (test mode: opener not run)", att.Name, path)
		return result
	}
	if err := exec.Command(opener, args...).Start(); err != nil {
		result.statusMsg = fmt.Sprintf("attachments: opening %s: %v", path, err)
		result.isError = true
		return result
	}
	result.statusMsg = fmt.Sprintf("opening %s", att.Name)
	return result
}

// sniffContentType reads the first 512 bytes of path, the same window
// http.DetectContentType is documented to use, without loading the whole
// (possibly large) attachment into memory.
func sniffContentType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}
