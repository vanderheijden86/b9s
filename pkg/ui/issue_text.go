package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/debug"
	"github.com/vanderheijden86/b9s/pkg/model"
)

// A lean Dolt snapshot leaves closed issues Abridged: their description,
// notes and comments are unknown until loaded one issue at a time
// (docs/adr/0060-load-a-lean-snapshot-and-text-on-demand.md).

// issueTextTimeout bounds one text request. It is far shorter than a whole
// reload, because it reads one row and its comments.
const issueTextTimeout = 30 * time.Second

// issueTextCacheLimit bounds the texts kept across reloads. Each one is an
// issue somebody opened, so the limit is seldom reached; past it the cache
// starts over rather than tracking recency.
const issueTextCacheLimit = 512

// cachedIssueText is the text of an issue as of its updated_at. A reload that
// brings a later updated_at makes it outdated.
type cachedIssueText struct {
	updatedAt time.Time
	text      datasource.IssueText
}

// issueTextLoadedMsg carries one fetched text back to the event loop.
type issueTextLoadedMsg struct {
	generation uint64
	id         string
	updatedAt  time.Time
	text       datasource.IssueText
	err        error
}

func loadIssueText(req doltReloadRequest, id string) (datasource.IssueText, error) {
	if req.fromSource {
		return datasource.LoadIssueText(req.source, id)
	}
	return datasource.LoadIssueTextFromDir(req.beadsDir, id)
}

// wholeIssue returns issue with its text, or nil after starting to load it
// and saying so in the status line. Every action that shows, copies or
// writes an issue's text goes through it: an Abridged issue's empty
// description means "not loaded", and an edit form saving it would erase it.
func (m *Model) wholeIssue(issue *model.Issue) *model.Issue {
	if issue == nil || !issue.Abridged {
		return issue
	}
	if whole, ok := m.issueMap[issue.ID]; ok && !whole.Abridged {
		return whole
	}
	if err := m.issueTextErr[issue.ID]; err != nil {
		m.statusMsg = fmt.Sprintf("Could not load the text of %s: %v", issue.ID, err)
		m.statusIsError = true
		return nil
	}
	m.requestIssueText(*issue)
	m.statusMsg = fmt.Sprintf("Loading the text of %s; try again in a moment", issue.ID)
	m.statusIsError = false
	return nil
}

// requestIssueText starts loading issue's text unless a request for it runs.
func (m *Model) requestIssueText(issue model.Issue) {
	if m.issueTextLoading[issue.ID] || m.issueTextErr[issue.ID] != nil {
		return
	}
	if m.issueTextLoading == nil {
		m.issueTextLoading = make(map[string]bool)
	}
	m.issueTextLoading[issue.ID] = true
	load := m.textLoader
	if load == nil {
		load = loadIssueText
	}
	req := m.doltRequest()
	generation := m.projectGeneration
	id, updatedAt := issue.ID, issue.UpdatedAt
	debug.Log("issueText: request %s generation=%d", id, generation)
	cmd := func() tea.Msg {
		type result struct {
			text datasource.IssueText
			err  error
		}
		done := make(chan result, 1)
		go func() {
			text, err := load(req, id)
			done <- result{text, err}
		}()
		timer := time.NewTimer(issueTextTimeout)
		defer timer.Stop()
		select {
		case r := <-done:
			return issueTextLoadedMsg{generation: generation, id: id, updatedAt: updatedAt, text: r.text, err: r.err}
		case <-timer.C:
			return issueTextLoadedMsg{generation: generation, id: id, updatedAt: updatedAt,
				err: fmt.Errorf("timed out after %s", issueTextTimeout)}
		}
	}
	m.pendingCmd = tea.Batch(m.pendingCmd, cmd)
}

// finishIssueText applies a fetched text. A reply from an earlier project is
// dropped: the switch cleared the requests it belonged to.
func (m Model) finishIssueText(msg issueTextLoadedMsg) Model {
	if msg.generation != m.projectGeneration {
		debug.Log("issueText: drop stale reply for %s", msg.id)
		return m
	}
	delete(m.issueTextLoading, msg.id)
	if msg.err != nil {
		debug.Log("issueText: %s failed: %v", msg.id, msg.err)
		if m.issueTextErr == nil {
			m.issueTextErr = make(map[string]error)
		}
		m.issueTextErr[msg.id] = msg.err
		m.updateViewportContent()
		return m
	}
	if m.issueTexts == nil || len(m.issueTexts) >= issueTextCacheLimit {
		m.issueTexts = make(map[string]cachedIssueText)
	}
	m.issueTexts[msg.id] = cachedIssueText{updatedAt: msg.updatedAt, text: msg.text}
	if issue, ok := m.issueMap[msg.id]; ok && issue.Abridged && issue.UpdatedAt.Equal(msg.updatedAt) {
		msg.text.ApplyTo(issue)
	}
	m.updateViewportContent()
	return m
}

// restoreIssueTexts gives a fresh snapshot the texts fetched for issues that
// have not changed since, and forgets the rest. Failed requests are forgotten
// too, so the next look at such an issue tries again.
func (m *Model) restoreIssueTexts() {
	m.issueTextErr = nil
	for id, cached := range m.issueTexts {
		issue, ok := m.issueMap[id]
		if !ok || !issue.Abridged || !issue.UpdatedAt.Equal(cached.updatedAt) {
			delete(m.issueTexts, id)
			continue
		}
		cached.text.ApplyTo(issue)
	}
}

// resetIssueTexts forgets every text and request of the previous project.
func (m *Model) resetIssueTexts() {
	m.quickView = nil
	m.issueTexts = nil
	m.issueTextLoading = nil
	m.issueTextErr = nil
}

// placeholderText shows, in an Abridged issue's description, why its text is
// not there yet.
func (m *Model) placeholderText(issue model.Issue) model.Issue {
	if err := m.issueTextErr[issue.ID]; err != nil {
		issue.Description = fmt.Sprintf("_Could not load the description and comments: %v_", err)
	} else {
		issue.Description = "_Loading the description and comments…_"
	}
	return issue
}
