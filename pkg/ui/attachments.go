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
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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
	if m.allProjectsMode {
		// attachmentOpenCmd refuses this too (review item 6), but refusing
		// here as well means the picker never opens only to fail on every
		// Enter: the merged view has no single project to resolve the blob
		// store against (review item D).
		m.statusMsg = "attachments: not available in all-projects mode; switch to one project (0 or :project) first"
		m.statusIsError = false
		return
	}
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

// attachTempState holds the private per-run attachment temp directory in a
// place every copy of Model shares. Model is a value type: Update returns a
// new copy on each message, and cmd/b9s/main.go's deferred Stop runs on the
// original value handed to tea.NewProgram, which never sees a field set on
// a later copy. Model.NewModel allocates one attachTempState and every
// WithXxx builder and Update carry the same pointer forward by copying the
// struct, so ensureAttachTempDir and Stop always reach the same dir field
// regardless of which copy of Model calls them.
type attachTempState struct {
	mu  sync.Mutex
	dir string
}

// take clears dir and returns its previous value, so a concurrent or
// repeated Stop call removes the directory at most once.
func (s *attachTempState) take() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := s.dir
	s.dir = ""
	return dir
}

// ensureAttachTempDir returns the private per-run directory attachment
// downloads land in, creating it on first use. Every subsequent open reuses
// the same parent directory (review item 4); attachmentOpenDownload still
// gives each individual download its own subdirectory so two attachments
// sharing a file name never collide. Model.Stop removes the whole tree once,
// at program exit, not after each open, since the system opener may still be
// reading the file when Update moves on.
func (m *Model) ensureAttachTempDir() (string, error) {
	m.attachTemp.mu.Lock()
	defer m.attachTemp.mu.Unlock()
	if m.attachTemp.dir != "" {
		return m.attachTemp.dir, nil
	}
	dir, err := os.MkdirTemp(os.TempDir(), "b9s-attach-*")
	if err != nil {
		return "", fmt.Errorf("creating attachments temp dir: %w", err)
	}
	m.attachTemp.dir = dir
	return dir, nil
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
	// dispatchProjectPath is the active project when attachmentOpenCmd was
	// dispatched. Update drops a result whose dispatchProjectPath no longer
	// matches the active project: a :project switch that lands between
	// dispatch and here means this result names the wrong project's blob
	// store and the wrong project's footer message (review item 7).
	dispatchProjectPath string

	attachmentName string
	openerCmd      []string
	openedPath     string
	statusMsg      string
	// hyperlink is an OSC 8 escape sequence for the B9S_WEB=1 path,
	// carried separately from statusMsg so the footer can write it raw
	// after sanitizing statusMsg (review item 1: sanitizeTerminalLine
	// would otherwise strip the escape codes as control characters).
	hyperlink string
	isError   bool
}

// openAllowedContentClass returns the class mimeType belongs to for the
// opener allowlist ("image", "pdf", "text", "audio" or "video"), or "" for
// anything else. image/svg+xml is refused even though it sniffs under
// image/, since SVG can embed script; http.DetectContentType never actually
// returns it (SVG has no magic-byte signature), but the exclusion also
// covers the extension side of openAllowedForContent below.
func openAllowedContentClass(mimeType string) string {
	base, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		base = mimeType
	}
	switch {
	case base == "image/svg+xml":
		return ""
	case strings.HasPrefix(base, "image/"):
		return "image"
	case base == "application/pdf":
		return "pdf"
	case base == "text/plain":
		return "text"
	case strings.HasPrefix(base, "audio/"):
		return "audio"
	case strings.HasPrefix(base, "video/"):
		return "video"
	default:
		return ""
	}
}

// openAllowedForContent reports whether attachmentOpenDownload may hand the
// downloaded file to the system opener. Both the sniffed bytes (an
// untrusted comment's type= field is not trusted for this) and name's
// extension must resolve to the same allowed class, so a disguised
// extension (a .terminal or .desktop file whose bytes happen to sniff as
// plain text) never reaches the opener on the strength of the sniff alone,
// and a mislabeled extension on genuinely dangerous bytes never reaches it
// on the strength of the name alone.
func openAllowedForContent(sniffedType, name string) bool {
	sniffClass := openAllowedContentClass(sniffedType)
	if sniffClass == "" {
		return false
	}
	extClass := openAllowedContentClass(mime.TypeByExtension(filepath.Ext(name)))
	if extClass == "" {
		return false
	}
	return extClass == sniffClass
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
// subprocess call runs inside the returned closure). dir is the per-run
// temp directory (Model.ensureAttachTempDir), created by the caller because
// creating it needs a pointer receiver this value-receiver Cmd builder does
// not have.
func (m Model) attachmentOpenCmd(att attachref.Attachment, dir string) tea.Cmd {
	handle := m.attachHandle
	handleProject := m.attachHandleProject
	projectPath := m.activeProjectPath
	projectName := m.activeProjectName
	sourceType := m.sourceType
	doltSource := m.doltSource
	allProjectsMode := m.allProjectsMode
	webMode := os.Getenv("B9S_WEB") != ""
	testMode := os.Getenv("B9S_TEST_MODE") != ""

	return func() tea.Msg {
		ctx := context.Background()
		result := attachmentOpenResultMsg{attachmentName: att.Name, dispatchProjectPath: projectPath}

		if allProjectsMode {
			// All-projects mode merges issues from every Dolt database in
			// the workspace, and the blob key's database segment must be
			// the issue's own source database, not whichever project is
			// merely "active" for the picker. Deriving that cheaply needs
			// the multi-reader to tag each issue with its source database,
			// which it does not do today, so this is refused outright
			// (review item 6) rather than risk resolving the wrong
			// project's blob store.
			result.statusMsg = "attachments: not available in all-projects mode; switch to one project (0 or :project) first"
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

		if webMode {
			return attachmentOpenWeb(ctx, handle, att, result)
		}
		return attachmentOpenDownload(ctx, handle, att, dir, testMode, result)
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
	if containsControlByte(url) {
		// The database segment of the blob key comes from repo-controlled
		// metadata.json (b9s's own project catalog, not attacker input from
		// this download), but a presigned URL is about to be written raw to
		// the terminal past sanitizeTerminalLine, so it still gets its own
		// check rather than trusting the S3 SDK's escaping (review item B).
		result.statusMsg = fmt.Sprintf("attachments: refusing to print a presigned URL with a control byte for %s", att.Name)
		result.isError = true
		return result
	}
	// The link itself, not the plain statusMsg label, carries the OSC 8
	// escape codes: renderFooter sanitizes statusMsg and writes hyperlink
	// raw afterward (review item 1). The visible label is "open <name>"
	// rather than the full URL (review item C): att.Name is already
	// validated as a bare file name with no path separators or control
	// characters (internal/attachref's Parse), so it is safe to show as-is.
	result.statusMsg = att.Name + ":"
	result.hyperlink = osc8Hyperlink(url, "open "+att.Name)
	return result
}

// containsControlByte reports whether s has any byte below 0x20 or equal to
// 0x7f, the range that lets a value break out of an OSC 8 escape sequence or
// otherwise act on the terminal instead of rendering as plain text.
func containsControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// attachmentOpenDownload downloads att into its own subdirectory of the
// private per-run temp directory (dir), renames the verified download to
// its declared name so the system opener's own type detection (e.g.
// macOS LaunchServices) sees a real extension instead of the download's
// randomly named temp file, sniffs the content actually downloaded (the
// comment's type= field is an untrusted claim), and runs the system opener
// unless the sniffed type and the name's extension do not agree on an
// allowed class, or B9S_TEST_MODE is set, in which case the would-be
// command is recorded instead of run. The subdirectory (not dir directly)
// is what makes two attachments sharing a file name safe to open in the
// same run, since dir is reused across opens (review item 4) while att.Name
// is only unique within one attachment.
func attachmentOpenDownload(ctx context.Context, handle *blobstore.Handle, att attachref.Attachment, dir string, testMode bool, result attachmentOpenResultMsg) attachmentOpenResultMsg {
	subDir, err := os.MkdirTemp(dir, "dl-*")
	if err != nil {
		result.statusMsg = fmt.Sprintf("attachments: creating temp dir: %v", err)
		result.isError = true
		return result
	}

	path, err := attach.Download(ctx, handle, att, subDir, attach.DownloadModeRestricted)
	if err != nil {
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}

	finalPath, err := attach.SafeJoin(subDir, att.Name)
	if err != nil {
		os.Remove(path)
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}
	if err := os.Rename(path, finalPath); err != nil {
		os.Remove(path)
		result.statusMsg = fmt.Sprintf("attachments: renaming download: %v", err)
		result.isError = true
		return result
	}
	path = finalPath
	result.openedPath = path

	sniffed, err := sniffContentType(path)
	if err != nil {
		os.Remove(path)
		result.statusMsg = fmt.Sprintf("attachments: %v", err)
		result.isError = true
		return result
	}

	if !openAllowedForContent(sniffed, att.Name) {
		// The file itself is not removed on refusal, unlike the sniff and
		// rename error paths above: it stays reachable at path until b9s
		// exits (Model.Stop removes the whole per-run temp dir), so the
		// message says so rather than leaving the user to guess (review
		// item E).
		result.statusMsg = fmt.Sprintf("%s downloaded to %s (opening refused: sniffed as %s; file stays until b9s exits)", att.Name, path, sniffed)
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
