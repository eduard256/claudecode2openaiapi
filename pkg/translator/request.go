package translator

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/eduard256/claudecode2openaiapi/pkg/openai"
	"github.com/eduard256/claudecode2openaiapi/pkg/toolproto"
)

// Result is what we feed into claudecli.Spawn.
type Result struct {
	SystemPrompt string
	StdinLines   []string
	// UserTurns is the count of "user" messages in StdinLines. Claude CLI
	// will process them sequentially, producing one Anthropic API call per
	// user turn — only the LAST turn's deltas are the real reply.
	UserTurns int
}

// OpenAIToClaude is the main converter. Given an OpenAI ChatRequest, it
// produces a system prompt (tool protocol + tools + user system) and an
// ordered list of stream-json lines representing the conversation history.
//
// Conversion rules:
//   - All role:system messages -> joined into the user-system part of the prompt
//   - role:user with text -> stream-json user message with text content
//   - role:user with image_url parts -> native Anthropic image blocks
//   - role:assistant with text -> stream-json assistant message
//   - role:assistant with tool_calls -> NATIVE tool_use content blocks
//   - role:tool -> NATIVE tool_result content block (merged with adjacent tool messages)
func OpenAIToClaude(req *openai.ChatRequest) (*Result, error) {
	var (
		userSystem strings.Builder
		lines      []string
	)

	// Pending tool_results buffer — OpenAI sends one tool message per result,
	// but Anthropic wants them grouped in a single user turn.
	flushTools := func(buf *[]map[string]any) {
		if len(*buf) == 0 {
			return
		}
		lines = append(lines, marshalMsg("user", *buf))
		*buf = nil
	}
	var pendingTools []map[string]any

	for _, m := range req.Messages {
		switch m.Role {
		case "system", "developer":
			flushTools(&pendingTools)
			s, err := messageText(m.Content)
			if err != nil {
				return nil, err
			}
			if s != "" {
				if userSystem.Len() > 0 {
					userSystem.WriteString("\n\n")
				}
				userSystem.WriteString(s)
			}

		case "user":
			flushTools(&pendingTools)
			content, err := userContent(m.Content)
			if err != nil {
				return nil, err
			}
			lines = append(lines, marshalMsg("user", content))

		case "assistant":
			flushTools(&pendingTools)
			content := assistantContent(m)
			if len(content) == 0 {
				continue
			}
			lines = append(lines, marshalMsg("assistant", content))

		case "tool":
			block, err := toolResultBlock(m)
			if err != nil {
				return nil, err
			}
			pendingTools = append(pendingTools, block)
		}
	}
	flushTools(&pendingTools)

	if len(lines) == 0 {
		return nil, errors.New("translator: no user messages in request")
	}

	system := toolproto.BuildSystem(toolDefs(req.Tools))
	if us := strings.TrimSpace(userSystem.String()); us != "" {
		if system != "" {
			system += "\n# USER INSTRUCTIONS\n" + us + "\n"
		} else {
			system = us
		}
	}

	return &Result{SystemPrompt: system, StdinLines: lines, UserTurns: countUserTurns(lines)}, nil
}

// countUserTurns parses each emitted stream-json line and counts those whose
// outer "type" is "user". Needed so the chat handler knows how many CLI
// sub-turns to skip before forwarding deltas to the client.
func countUserTurns(lines []string) int {
	n := 0
	for _, l := range lines {
		var env struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(l), &env); err != nil {
			continue
		}
		if env.Type == "user" {
			n++
		}
	}
	return n
}

// messageText returns the plain string of a content field that is either
// "string" or [{type:text}, ...].
func messageText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	var parts []openai.ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(p.Text)
		}
	}
	return b.String(), nil
}

// userContent converts an OpenAI user message content (string or parts) into
// Anthropic native content blocks.
func userContent(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return []map[string]any{{"type": "text", "text": ""}}, nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return []map[string]any{{"type": "text", "text": s}}, nil
	}
	var parts []openai.ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case "text":
			out = append(out, map[string]any{"type": "text", "text": p.Text})
		case "image_url":
			if p.ImageURL == nil {
				continue
			}
			block, err := imageBlock(p.ImageURL.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, block)
		}
	}
	if len(out) == 0 {
		out = append(out, map[string]any{"type": "text", "text": ""})
	}
	return out, nil
}

// assistantContent serializes an assistant turn into a single text block
// that mirrors the XML protocol the model itself produces. We DO NOT use
// native Anthropic tool_use blocks here — the model gets confused when its
// own free-form output (XML) is later replayed as native blocks with ids it
// never picked. Mirroring the XML in history is consistent.
func assistantContent(m openai.Message) []map[string]any {
	var b strings.Builder

	if s, _ := messageText(m.Content); s != "" {
		b.WriteString(s)
	}
	if len(m.ToolCalls) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("<tool_calls>\n")
		for _, tc := range m.ToolCalls {
			args := tc.Function.Arguments
			if args == "" {
				args = "{}"
			}
			b.WriteString(`<tool_call>{"name":"`)
			b.WriteString(tc.Function.Name)
			b.WriteString(`","arguments":`)
			b.WriteString(args)
			b.WriteString("}</tool_call>\n")
		}
		b.WriteString("</tool_calls>")
	}
	return []map[string]any{{"type": "text", "text": b.String()}}
}

// toolResultBlock formats a tool result as a plain-text <tool_result> XML
// element matching the protocol described in the system prompt.
func toolResultBlock(m openai.Message) (map[string]any, error) {
	content, err := messageText(m.Content)
	if err != nil {
		return nil, err
	}
	body := "<tool_result>" + content + "</tool_result>"
	return map[string]any{"type": "text", "text": body}, nil
}

func marshalMsg(role string, content []map[string]any) string {
	msg := map[string]any{
		"type": role, // "user" | "assistant"
		"message": map[string]any{
			"role":    role,
			"content": content,
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}
