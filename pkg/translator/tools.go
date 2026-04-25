package translator

import (
	"github.com/eduard256/claudecode2openaiapi/pkg/openai"
	"github.com/eduard256/claudecode2openaiapi/pkg/toolproto"
)

// toolDefs flattens openai.Tool[] to toolproto.ToolDef[]. We only support
// type=function tools — anything else is silently dropped.
func toolDefs(tools []openai.Tool) []toolproto.ToolDef {
	out := make([]toolproto.ToolDef, 0, len(tools))
	for _, t := range tools {
		if t.Type != "function" {
			continue
		}
		out = append(out, toolproto.ToolDef{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	return out
}
