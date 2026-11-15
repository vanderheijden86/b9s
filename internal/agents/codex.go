package agents

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayload struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Cwd       string          `json:"cwd"`
	Model     string          `json:"model"`
	Role      string          `json:"role"`
	Name      string          `json:"name"`
	CallID    string          `json:"call_id"`
	Arguments string          `json:"arguments"`
	Input     string          `json:"input"`
	Output    json.RawMessage `json:"output"`
	Content   json.RawMessage `json:"content"`
	Message   string          `json:"message"`
	Git       *struct {
		Branch string `json:"branch"`
	} `json:"git"`
	Info *struct {
		LastTokenUsage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"last_token_usage"`
		ModelContextWindow int `json:"model_context_window"`
	} `json:"info"`
}

var (
	codexCmdRe      = regexp.MustCompile(`"?cmd"?\s*:\s*"((?:[^"\\]|\\.)*)"`)
	codexReadRe     = regexp.MustCompile(`^(?:cat|head|tail|sed -n \S+|nl -ba)\s+(/\S+)$`)
	codexAgentsMdRe = regexp.MustCompile(`^# AGENTS\.md instructions for (\S+)`)
)

func (t *transcript) feedCodex(line []byte) {
	var l codexLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	var p codexPayload
	_ = json.Unmarshal(l.Payload, &p)
	at := parseTime(l.Timestamp)
	t.touch(at)
	switch l.Type {
	case "session_meta":
		if t.sess.ID == "" {
			t.sess.ID = p.ID
		}
		t.setCwd(p.Cwd)
		if p.Git != nil {
			t.setBranch(p.Git.Branch)
		}
	case "turn_context":
		t.setCwd(p.Cwd)
		if p.Model != "" {
			t.sess.Model = p.Model
		}
	case "compacted":
		t.compact(at, EstimateTokens(len(p.Message)))
	case "event_msg":
		t.codexEvent(at, p)
	case "response_item":
		t.codexItem(at, p)
	}
}

func (t *transcript) codexEvent(at time.Time, p codexPayload) {
	switch p.Type {
	case "token_count":
		if p.Info == nil {
			return
		}
		if p.Info.ModelContextWindow > 0 {
			t.sess.ContextLimit = p.Info.ModelContextWindow
		}
		if u := p.Info.LastTokenUsage; u != nil {
			if u.InputTokens > 0 {
				t.sess.ContextTokens = u.InputTokens
			}
			t.sample(at, u.OutputTokens)
		}
	case "task_started", "task_complete":
		t.lastStop = p.Type
		t.apiError = false
	case "turn_aborted":
		t.lastStop = "task_complete"
		t.emit(Event{Time: at, Kind: EventErr, Text: "turn aborted"})
	case "error", "stream_error":
		t.apiError = true
		t.sess.Errors++
		t.emit(Event{Time: at, Kind: EventErr, Text: p.Message})
	}
}

func (t *transcript) codexItem(at time.Time, p codexPayload) {
	switch p.Type {
	case "message":
		text := codexText(p.Content)
		switch p.Role {
		case "developer", "system":
			t.addSource("developer", Source{Layer: LayerInstructions, Name: "developer message", Scope: "global", Tokens: EstimateTokens(len(text)), LoadedAt: at})
		case "user":
			if m := codexAgentsMdRe.FindStringSubmatch(text); m != nil {
				t.addSource("agents:"+m[1], Source{Layer: LayerInstructions, Name: "AGENTS.md (" + m[1] + ")", Path: m[1], Scope: "project", Tokens: EstimateTokens(len(text)), LoadedAt: at})
				t.emit(Event{Time: at, Kind: EventCtx, Text: "loaded AGENTS.md for " + m[1], Detail: text})
				return
			}
			if strings.HasPrefix(strings.TrimSpace(text), "<environment_context>") {
				t.conversed += len(text)
				return
			}
			t.prompt(at, text)
		case "assistant":
			if strings.TrimSpace(text) == "" {
				return
			}
			t.conversed += len(text)
			t.emit(Event{Time: at, Kind: EventAI, Text: text, Detail: text})
		}
	case "custom_tool_call", "function_call", "local_shell_call":
		t.sess.Tools++
		input := p.Input
		if input == "" {
			input = p.Arguments
		}
		t.conversed += len(input)
		t.calls[p.CallID] = toolCall{name: p.Name}
		cmds := codexCommands(input)
		if len(cmds) == 0 {
			t.emit(Event{Time: at, Kind: EventTool, Tool: p.Name, Text: p.Name + " " + input, Detail: input})
			return
		}
		for _, c := range cmds {
			kind := EventTool
			if bdCmdRe.MatchString(" " + c) {
				kind = EventBD
				t.bdCommand(c)
			}
			if m := codexReadRe.FindStringSubmatch(c); m != nil {
				t.calls[p.CallID] = toolCall{name: p.Name, path: m[1]}
			}
			t.emit(Event{Time: at, Kind: kind, Tool: p.Name, Text: c, Detail: input})
		}
	case "custom_tool_call_output", "function_call_output", "local_shell_call_output":
		text := codexText(p.Output)
		call := t.calls[p.CallID]
		delete(t.calls, p.CallID)
		if call.path != "" {
			t.addSource("file:"+call.path, Source{Layer: LayerFiles, Name: shortPath(call.path), Path: call.path, Scope: "session", Tokens: EstimateTokens(len(text)), LoadedAt: at})
		} else {
			t.toolOut += len(text)
		}
		kind := EventResult
		if strings.Contains(text, "Process exited with code") && !strings.Contains(text, "code 0") {
			kind = EventErr
			t.sess.Errors++
		}
		t.emit(Event{Time: at, Kind: kind, Tool: call.name, Text: text, Detail: text})
	}
}

// codexCommands pulls the shell commands out of an exec tool call. Codex
// writes the call as a script of tools.exec_command({cmd:"..."}) calls, or as
// a JSON argument object with a "cmd" field.
func codexCommands(input string) []string {
	var out []string
	for _, m := range codexCmdRe.FindAllStringSubmatch(input, -1) {
		var s string
		if json.Unmarshal([]byte(`"`+m[1]+`"`), &s) != nil {
			s = m[1]
		}
		out = append(out, s)
	}
	return out
}

func codexText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Text != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	var obj struct {
		Output string `json:"output"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Output != "" {
		return obj.Output
	}
	return string(raw)
}
