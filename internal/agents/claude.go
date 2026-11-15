package agents

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// claudeLine is the subset of a Claude Code transcript line the parser reads.
type claudeLine struct {
	Type              string          `json:"type"`
	Subtype           string          `json:"subtype"`
	Timestamp         string          `json:"timestamp"`
	Cwd               string          `json:"cwd"`
	SessionID         string          `json:"sessionId"`
	GitBranch         string          `json:"gitBranch"`
	CustomTitle       string          `json:"customTitle"`
	AgentName         string          `json:"agentName"`
	IsMeta            bool            `json:"isMeta"`
	IsCompactSummary  bool            `json:"isCompactSummary"`
	IsSidechain       bool            `json:"isSidechain"`
	IsAPIErrorMessage bool            `json:"isApiErrorMessage"`
	Content           string          `json:"content"`
	Message           *claudeMessage  `json:"message"`
	Attachment        json.RawMessage `json:"attachment"`
	CompactMetadata   *struct {
		PostTokens int `json:"postTokens"`
	} `json:"compactMetadata"`
}

type claudeMessage struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Content    json.RawMessage `json:"content"`
	Usage      *struct {
		InputTokens              int `json:"input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		OutputTokens             int `json:"output_tokens"`
	} `json:"usage"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

type claudeAttachment struct {
	Type     string `json:"type"`
	HookName string `json:"hookName"`
	Stdout   string `json:"stdout"`
	Content  any    `json:"content"`
}

func (t *transcript) feedClaude(line []byte) {
	var l claudeLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	if l.IsSidechain {
		return
	}
	at := parseTime(l.Timestamp)
	if l.SessionID != "" && t.sess.ID == "" {
		t.sess.ID = l.SessionID
	}
	t.setCwd(l.Cwd)
	t.setBranch(l.GitBranch)
	switch l.Type {
	case "custom-title":
		if l.CustomTitle != "" {
			t.sess.Name = l.CustomTitle
		}
		return
	case "agent-name":
		if l.AgentName != "" {
			t.sess.Name = l.AgentName
		}
		return
	}
	t.touch(at)
	switch l.Type {
	case "attachment":
		t.claudeAttachment(at, l.Attachment)
	case "system":
		switch l.Subtype {
		case "compact_boundary":
			post := 0
			if l.CompactMetadata != nil {
				post = l.CompactMetadata.PostTokens
			}
			t.compact(at, post)
		case "api_error":
			t.apiError = true
			t.sess.Errors++
			t.emit(Event{Time: at, Kind: EventErr, Text: "API error: " + l.Content})
		}
	case "user":
		if l.Message == nil || l.IsCompactSummary {
			return
		}
		t.claudeUser(at, l.IsMeta, l.Message.Content)
	case "assistant":
		if l.Message == nil {
			return
		}
		t.claudeAssistant(at, l)
	}
}

func (t *transcript) claudeAttachment(at time.Time, raw json.RawMessage) {
	var a claudeAttachment
	if json.Unmarshal(raw, &a) != nil {
		return
	}
	text := a.Stdout
	if text == "" {
		if s, ok := a.Content.(string); ok {
			text = s
		}
	}
	if text == "" {
		return
	}
	switch a.Type {
	case "hook_success", "hook_additional_context":
	default:
		return
	}
	name := a.HookName
	if name == "" {
		name = a.Type
	}
	mems := parseMemories(text)
	memBytes := 0
	for _, m := range mems {
		memBytes += m.bytes
		t.addSource("memory:"+m.key, Source{Layer: LayerMemories, Name: m.key, Scope: "project", Tokens: EstimateTokens(m.bytes), LoadedAt: at})
	}
	if rest := len(text) - memBytes; rest > 0 {
		t.addSource("hook:"+name, Source{Layer: LayerInstructions, Name: "hook " + name, Scope: "hook", Tokens: EstimateTokens(rest), LoadedAt: at})
	}
	msg := fmt.Sprintf("loaded %s output", name)
	if len(mems) > 0 {
		msg = fmt.Sprintf("loaded %d memories from %s", len(mems), name)
	}
	t.emit(Event{Time: at, Kind: EventCtx, Text: msg, Detail: text})
}

func (t *transcript) claudeUser(at time.Time, meta bool, raw json.RawMessage) {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if meta || isHarnessText(s) {
			t.conversed += len(s)
			return
		}
		t.prompt(at, s)
		return
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if meta || isHarnessText(b.Text) {
				t.conversed += len(b.Text)
				continue
			}
			t.prompt(at, b.Text)
		case "tool_result":
			t.claudeToolResult(at, b)
		}
	}
}

func (t *transcript) claudeToolResult(at time.Time, b claudeBlock) {
	text := blockText(b.Content)
	call := t.calls[b.ToolUseID]
	delete(t.calls, b.ToolUseID)
	if call.name == "Read" && call.path != "" && !b.IsError {
		t.addSource("file:"+call.path, Source{Layer: LayerFiles, Name: shortPath(call.path), Path: call.path, Scope: "session", Tokens: EstimateTokens(len(text)), LoadedAt: at})
	} else {
		t.toolOut += len(text)
	}
	if b.IsError {
		t.sess.Errors++
		t.emit(Event{Time: at, Kind: EventErr, Tool: call.name, Text: text, Detail: text})
		return
	}
	t.emit(Event{Time: at, Kind: EventResult, Tool: call.name, Text: text, Detail: text})
}

func (t *transcript) claudeAssistant(at time.Time, l claudeLine) {
	m := l.Message
	if m.Model != "" && m.Model != "<synthetic>" {
		t.sess.Model = m.Model
	}
	if l.IsAPIErrorMessage {
		t.apiError = true
		t.sess.Errors++
		t.emit(Event{Time: at, Kind: EventErr, Text: blockText(m.Content)})
		return
	}
	t.apiError = false
	if m.StopReason != "" {
		t.lastStop = m.StopReason
	}
	if u := m.Usage; u != nil {
		window := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
		if window > 0 {
			t.sess.ContextTokens = window
			if window > t.sess.ContextLimit {
				t.sess.ContextLimit = 1000000
			}
		}
		t.sample(at, u.OutputTokens)
	}
	var blocks []claudeBlock
	if json.Unmarshal(m.Content, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			t.conversed += len(b.Text)
			t.emit(Event{Time: at, Kind: EventAI, Text: b.Text, Detail: b.Text})
		case "tool_use":
			t.claudeToolUse(at, b)
		}
	}
}

func (t *transcript) claudeToolUse(at time.Time, b claudeBlock) {
	t.sess.Tools++
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	str := func(k string) string { s, _ := in[k].(string); return s }
	call := toolCall{name: b.Name}
	summary := ""
	kind := EventTool
	switch b.Name {
	case "Bash":
		summary = str("command")
		if bdCmdRe.MatchString(" " + summary) {
			kind = EventBD
			t.bdCommand(summary)
		}
	case "Read", "Edit", "Write", "NotebookEdit":
		call.path = str("file_path")
		summary = shortPath(call.path)
	case "Skill":
		skill := str("skill")
		summary = skill
		t.addSource("skill:"+skill, Source{Layer: LayerSkills, Name: skill, Scope: "session", Tokens: 0, LoadedAt: at})
	case "Grep", "Glob":
		summary = str("pattern")
	case "Agent", "Task":
		summary = str("description")
	default:
		summary = compactJSON(b.Input)
	}
	t.calls[b.ID] = call
	t.conversed += len(b.Input)
	t.emit(Event{Time: at, Kind: kind, Tool: b.Name, Text: summary, Detail: compactJSON(b.Input)})
}

type memory struct {
	key   string
	bytes int
}

// parseMemories reads the "## Persistent Memories" block that bd prime
// writes: one "### key" heading per memory, ending at the next heading of
// level one or two.
func parseMemories(text string) []memory {
	i := strings.Index(text, "## Persistent Memories")
	if i < 0 {
		return nil
	}
	var out []memory
	lines := strings.Split(text[i:], "\n")
	for _, ln := range lines[1:] {
		if strings.HasPrefix(ln, "# ") || strings.HasPrefix(ln, "## ") {
			break
		}
		if strings.HasPrefix(ln, "### ") {
			out = append(out, memory{key: strings.TrimSpace(ln[4:]), bytes: len(ln)})
			continue
		}
		if len(out) > 0 {
			out[len(out)-1].bytes += len(ln) + 1
		}
	}
	return out
}

// isHarnessText recognises text the harness injects as a user turn (command
// echoes, reminders, task notifications) rather than a person's prompt.
func isHarnessText(s string) bool {
	s = strings.TrimSpace(s)
	for _, p := range []string{"<command-", "<local-command", "<system-reminder>", "<task-notification>", "Caveat:", "[Request interrupted"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func blockText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []claudeBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var b strings.Builder
		for _, x := range blocks {
			if x.Text != "" {
				b.WriteString(x.Text)
				b.WriteString("\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	}
	return string(raw)
}

func compactJSON(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if len(s) > 2000 {
		s = s[:2000] + "…"
	}
	return s
}

func shortPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) > 3 {
		return "…/" + strings.Join(parts[len(parts)-3:], "/")
	}
	return p
}
