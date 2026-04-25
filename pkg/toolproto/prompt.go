package toolproto

import (
	"encoding/json"
	"strings"
)

// ToolDef matches the OpenAI function-tool shape, simplified.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// BuildSystem returns the prefix of a system prompt that teaches the model our
// XML protocol and lists the tools it can call. Caller appends the user's
// own system prompt after this.
func BuildSystem(tools []ToolDef) string {
	if len(tools) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("# TOOL CALLING PROTOCOL\n\n")
	b.WriteString("When you decide to call one or more tools, output ONE wrapper:\n\n")
	b.WriteString("<tool_calls>\n")
	b.WriteString("<tool_call>{\"name\":\"NAME\",\"arguments\":{...}}</tool_call>\n")
	b.WriteString("<tool_call>{\"name\":\"NAME\",\"arguments\":{...}}</tool_call>\n")
	b.WriteString("</tool_calls>\n\n")
	b.WriteString("Multiple <tool_call> blocks inside the wrapper run in parallel.\n")
	b.WriteString("After </tool_calls> STOP IMMEDIATELY. Do not write text before or after the wrapper when calling tools.\n")
	b.WriteString("If you do not need any tool, reply with normal text (no wrapper).\n\n")
	b.WriteString("# HOW RESULTS COME BACK\n\n")
	b.WriteString("After your <tool_calls>, the host runs each tool and replies with native\n")
	b.WriteString("tool_result content blocks containing the actual outputs. Treat them as\n")
	b.WriteString("ground truth. Decide whether to call MORE tools (new wrapper) or write a\n")
	b.WriteString("final answer in plain text. NEVER call a tool with arguments identical to\n")
	b.WriteString("a call already in the conversation.\n\n")
	b.WriteString("# AVAILABLE TOOLS\n\n")
	for _, t := range tools {
		b.WriteString("## ")
		b.WriteString(t.Name)
		b.WriteString("\n")
		if t.Description != "" {
			b.WriteString(t.Description)
			b.WriteString("\n")
		}
		if t.Parameters != nil {
			schema, _ := json.MarshalIndent(t.Parameters, "", "  ")
			b.WriteString("\nArguments (JSON Schema):\n```json\n")
			b.Write(schema)
			b.WriteString("\n```\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
