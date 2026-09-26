package web

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanderheijden86/beadwork/pkg/ui"
)

// issueIDPattern admits Beads IDs (prefix-hash, dotted children) and nothing
// that bd could read as a flag.
var issueIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// Statuses the status sheet may set. closed goes through bd close, and
// tombstone is bd delete's business.
var settableStatuses = map[string]bool{
	"open": true, "in_progress": true, "blocked": true, "deferred": true,
	"pinned": true, "hooked": true, "review": true, "closed": true,
}

// updateFields and createFields name the bd flags the browser may set. Each
// value is passed as one --name=value argument, so a value cannot add flags.
var updateFields = map[string]bool{
	"title": true, "description": true, "design": true, "acceptance": true, "notes": true,
	"status": true, "priority": true, "type": true, "assignee": true, "parent": true,
	"add-label": true, "remove-label": true,
}

var createFields = map[string]bool{
	"title": true, "description": true, "design": true, "acceptance": true, "notes": true,
	"priority": true, "type": true, "assignee": true, "parent": true, "labels": true,
}

// handleWrite runs one write through bd, then reloads so the response and
// the next snapshot already show it. Writes run one at a time.
func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req WriteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cmd, err := s.planWrite(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if req.Key != "" {
		if cached, ok := s.idem.get(req.Key); ok {
			writeJSONStatus(w, writeStatus(cached), cached)
			return
		}
	}

	result := s.run(req, cmd)
	if result.OK {
		s.opts.Store.Reload()
	}
	result.Version = s.opts.Store.Version()
	if req.Key != "" {
		s.idem.put(req.Key, result)
	}
	writeJSONStatus(w, writeStatus(result), result)
}

func writeStatus(result WriteResult) int {
	if result.OK {
		return http.StatusOK
	}
	return http.StatusUnprocessableEntity
}

// writePlan is a validated write, ready to run in the current checkout.
type writePlan func(*ui.IssueWriter) tea.Cmd

func (s *Server) planWrite(req WriteRequest) (writePlan, error) {
	for _, id := range req.IDs {
		if !issueIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid issue id %q", id)
		}
	}
	ids := append([]string(nil), req.IDs...)
	needIDs := func(one bool) error {
		if len(ids) == 0 {
			return errors.New("no issue ids given")
		}
		if one && len(ids) != 1 {
			return fmt.Errorf("%s takes exactly one issue", req.Op)
		}
		return nil
	}

	switch req.Op {
	case OpStatus:
		if err := needIDs(false); err != nil {
			return nil, err
		}
		if !settableStatuses[req.Status] {
			return nil, fmt.Errorf("unknown status %q", req.Status)
		}
		if req.Status == "closed" {
			return closePlan(ids, req.Reason), nil
		}
		if len(ids) == 1 {
			return func(w *ui.IssueWriter) tea.Cmd { return w.SetStatus(ids[0], req.Status) }, nil
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.SetStatuses(ids, req.Status) }, nil

	case OpClose:
		if err := needIDs(false); err != nil {
			return nil, err
		}
		return closePlan(ids, req.Reason), nil

	case OpDelete:
		if err := needIDs(false); err != nil {
			return nil, err
		}
		if len(ids) == 1 {
			return func(w *ui.IssueWriter) tea.Cmd { return w.DeleteIssue(ids[0]) }, nil
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.DeleteIssues(ids) }, nil

	case OpUpdate:
		if err := needIDs(true); err != nil {
			return nil, err
		}
		fields, err := checkFields(req.Fields, updateFields)
		if err != nil {
			return nil, err
		}
		if len(fields) == 0 {
			return nil, errors.New("nothing to change")
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.UpdateIssue(ids[0], fields) }, nil

	case OpCreate:
		if len(ids) != 0 {
			return nil, errors.New("create takes no issue ids")
		}
		fields, err := checkFields(req.Fields, createFields)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(fields["title"]) == "" {
			return nil, errors.New("a new issue needs a title")
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.CreateIssue(fields) }, nil

	case OpComment:
		if err := needIDs(true); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.Text) == "" {
			return nil, errors.New("a comment needs text")
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.AddComment(ids[0], req.Text) }, nil

	case OpDefer:
		if err := needIDs(true); err != nil {
			return nil, err
		}
		if req.Until != "" && strings.HasPrefix(req.Until, "-") {
			return nil, fmt.Errorf("invalid defer date %q", req.Until)
		}
		return func(w *ui.IssueWriter) tea.Cmd { return w.DeferIssue(ids[0], req.Until) }, nil
	}
	return nil, fmt.Errorf("unknown write %q", req.Op)
}

func closePlan(ids []string, reason string) writePlan {
	if len(ids) == 1 {
		return func(w *ui.IssueWriter) tea.Cmd { return w.CloseIssue(ids[0], reason) }
	}
	return func(w *ui.IssueWriter) tea.Cmd { return w.CloseIssues(ids) }
}

func checkFields(fields map[string]string, allowed map[string]bool) (map[string]string, error) {
	out := make(map[string]string, len(fields))
	for name, value := range fields {
		if !allowed[name] {
			return nil, fmt.Errorf("field %q cannot be set here", name)
		}
		if name == "priority" {
			n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(value), "P"))
			if err != nil || n < 0 || n > 4 {
				return nil, fmt.Errorf("priority must be 0-4, got %q", value)
			}
			value = strconv.Itoa(n)
		}
		if name == "status" && !settableStatuses[value] {
			return nil, fmt.Errorf("unknown status %q", value)
		}
		if name == "parent" && value != "" && !issueIDPattern.MatchString(value) {
			return nil, fmt.Errorf("invalid parent id %q", value)
		}
		out[name] = value
	}
	return out, nil
}

// run executes plan in the checkout of the project on screen. The writer
// refuses a read-only project itself, with the message the TUI shows.
func (s *Server) run(req WriteRequest, plan writePlan) WriteResult {
	s.opts.Writer.SetCheckout(checkoutFor(s.opts.Store.CheckoutDir()))
	msg := plan(s.opts.Writer)()
	res, ok := msg.(ui.BdResultMsg)
	if !ok {
		return WriteResult{Error: fmt.Sprintf("unexpected writer result %T", msg)}
	}
	out := WriteResult{OK: res.Success, IDs: req.IDs, Output: res.Output}
	if out.IDs == nil {
		out.IDs = []string{}
	}
	if req.Op == OpCreate && res.Success {
		out.Created = res.IssueID
	}
	if res.Error != nil {
		out.Error = res.Error.Error()
	}
	return out
}

func checkoutFor(dir string) ui.Checkout {
	checkout, _ := ui.NewCheckout(dir)
	return checkout
}

// idempotency remembers recent write results by client key, so a request
// the phone retries after a dropped response does not run bd twice.
type idempotency struct {
	mu      sync.Mutex
	max     int
	ttl     time.Duration
	entries map[string]idemEntry
}

type idemEntry struct {
	result WriteResult
	at     time.Time
}

func newIdempotency(max int, ttl time.Duration) *idempotency {
	return &idempotency{max: max, ttl: ttl, entries: make(map[string]idemEntry)}
}

func (c *idempotency) get(key string) (WriteResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.at) > c.ttl {
		return WriteResult{}, false
	}
	return e.result, true
}

func (c *idempotency) put(key string, result WriteResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.max {
		oldestKey, oldest := "", time.Now()
		for k, e := range c.entries {
			if e.at.Before(oldest) {
				oldestKey, oldest = k, e.at
			}
		}
		delete(c.entries, oldestKey)
	}
	c.entries[key] = idemEntry{result: result, at: time.Now()}
}
