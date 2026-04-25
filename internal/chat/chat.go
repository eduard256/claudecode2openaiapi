package chat

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/eduard256/claudecode2openaiapi/api"
	"github.com/eduard256/claudecode2openaiapi/pkg/claudecli"
	"github.com/eduard256/claudecode2openaiapi/pkg/openai"
	"github.com/eduard256/claudecode2openaiapi/pkg/sse"
	"github.com/eduard256/claudecode2openaiapi/pkg/toolproto"
	"github.com/eduard256/claudecode2openaiapi/pkg/translator"
)

func Init() {
	api.HandleFunc("/v1/chat/completions", handle)
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, 405, "method not allowed", "invalid_request_error", "")
		return
	}

	var req openai.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid json: "+err.Error(), "invalid_request_error", "")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, 400, "messages is required", "invalid_request_error", "messages")
		return
	}
	if req.Model == "" {
		req.Model = "sonnet"
	}

	tr, err := translator.OpenAIToClaude(&req)
	if err != nil {
		writeError(w, 400, err.Error(), "invalid_request_error", "")
		return
	}

	proc, err := claudecli.Spawn(claudecli.Options{
		Model:        modelAlias(req.Model),
		SystemPrompt: tr.SystemPrompt,
		StdinLines:   tr.StdinLines,
	})
	if err != nil {
		writeError(w, 502, "claude spawn failed: "+err.Error(), "server_error", "")
		return
	}

	if req.Stream {
		streamResponse(w, r, &req, proc)
	} else {
		bufferResponse(w, &req, proc)
	}
}

// modelAlias maps OpenAI-style "gpt-*" to Claude alias if anyone tries.
func modelAlias(m string) string {
	switch m {
	case "gpt-4o", "gpt-4-turbo", "gpt-4":
		return "sonnet"
	case "gpt-4o-mini", "gpt-3.5-turbo":
		return "haiku"
	}
	return m
}

// streamResponse drives the SSE flow.
func streamResponse(w http.ResponseWriter, r *http.Request, req *openai.ChatRequest, proc *claudecli.Process) {
	id := openai.CompletionID()
	created := time.Now().Unix()
	wr := sse.NewWriter(w)

	parser := toolproto.NewStream()
	var (
		usage        claudecli.Usage // accumulated max-of-fields across events
		sentRole     bool
		gotToolBlock bool
		errSeen      error
		clientGone   = r.Context().Done()
	)

	emit := func(c openai.ChunkDelta, finish *string) {
		_ = wr.Write(openai.StreamChunk{
			ID: id, Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []openai.ChunkChoice{{Index: 0, Delta: c, FinishReason: finish}},
		})
	}

	// First chunk announces the assistant role (per OpenAI spec).
	emit(openai.ChunkDelta{Role: "assistant"}, nil)
	sentRole = true
	_ = sentRole

loop:
	for {
		select {
		case <-clientGone:
			proc.Kill()
			return
		case e, ok := <-proc.Events:
			if !ok {
				break loop
			}
			if e.Err != nil {
				errSeen = e.Err
				continue
			}
			if e.Usage != nil {
				mergeUsage(&usage, e.Usage)
				continue
			}
			if e.Done {
				continue
			}
			if e.TextDelta == "" {
				continue
			}

			ev := parser.Feed(e.TextDelta)
			if ev.WrapperOpened {
				gotToolBlock = true
			}
			if !gotToolBlock && ev.TextDelta != "" {
				emit(openai.ChunkDelta{Content: ev.TextDelta}, nil)
			}
			if ev.WrapperClosed {
				proc.Kill()
				break loop
			}
		}
	}

	finish := "stop"
	if errSeen != nil {
		finish = "stop"
	}
	// Flush any text the parser was holding back as a possible wrapper prefix.
	if !gotToolBlock {
		if tail := parser.Flush(); tail != "" {
			emit(openai.ChunkDelta{Content: tail}, nil)
		}
	}
	if gotToolBlock {
		finish = "tool_calls"
		// Emit tool_calls as one or more delta chunks. We pick the simplest
		// shape: a single chunk per tool call carrying both id+name+arguments.
		calls := toolproto.Extract(parser.Buffered())
		for i, c := range calls {
			argJSON, _ := json.Marshal(c.Arguments)
			emit(openai.ChunkDelta{
				ToolCalls: []openai.ChunkToolCall{{
					Index: i,
					ID:    openai.ToolCallID(),
					Type:  "function",
					Function: &openai.ChunkFunctionCall{
						Name:      c.Name,
						Arguments: string(argJSON),
					},
				}},
			}, nil)
		}
	}

	emit(openai.ChunkDelta{}, &finish)

	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		_ = wr.Write(openai.StreamChunk{
			ID: id, Object: "chat.completion.chunk", Created: created, Model: req.Model,
			Choices: []openai.ChunkChoice{},
			Usage: &openai.Usage{
				PromptTokens:     usage.InputTokens,
				CompletionTokens: usage.OutputTokens,
				TotalTokens:      usage.InputTokens + usage.OutputTokens,
			},
		})
	}

	wr.Done()
}

// mergeUsage takes the max of each field. Different stream events report
// different subsets (message_start has input, message_delta has output,
// final result has both) — taking max gives us the union.
func mergeUsage(dst *claudecli.Usage, src *claudecli.Usage) {
	if src.InputTokens > dst.InputTokens {
		dst.InputTokens = src.InputTokens
	}
	if src.OutputTokens > dst.OutputTokens {
		dst.OutputTokens = src.OutputTokens
	}
	if src.CacheCreationTokens > dst.CacheCreationTokens {
		dst.CacheCreationTokens = src.CacheCreationTokens
	}
	if src.CacheReadTokens > dst.CacheReadTokens {
		dst.CacheReadTokens = src.CacheReadTokens
	}
}

// bufferResponse collects everything into a single non-streaming response.
func bufferResponse(w http.ResponseWriter, req *openai.ChatRequest, proc *claudecli.Process) {
	parser := toolproto.NewStream()
	var (
		usage      claudecli.Usage
		visibleTxt strings.Builder
		gotTool    bool
		errSeen    error
	)

	for e := range proc.Events {
		if e.Err != nil {
			errSeen = e.Err
			continue
		}
		if e.Usage != nil {
			mergeUsage(&usage, e.Usage)
			continue
		}
		if e.Done || e.TextDelta == "" {
			continue
		}
		ev := parser.Feed(e.TextDelta)
		if ev.WrapperOpened {
			gotTool = true
		}
		if !gotTool && ev.TextDelta != "" {
			visibleTxt.WriteString(ev.TextDelta)
		}
		if ev.WrapperClosed {
			proc.Kill()
			break
		}
	}
	// Flush parser tail for non-streaming buffered response too.
	if !gotTool {
		visibleTxt.WriteString(parser.Flush())
	}

	if errSeen != nil {
		writeError(w, 502, errSeen.Error(), "server_error", "")
		return
	}

	resp := openai.ChatResponse{
		ID:      openai.CompletionID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
	}
	choice := openai.Choice{
		Index: 0,
		Message: openai.ResponseMessage{
			Role:    "assistant",
			Content: visibleTxt.String(),
		},
		FinishReason: "stop",
	}
	if gotTool {
		calls := toolproto.Extract(parser.Buffered())
		for _, c := range calls {
			argJSON, _ := json.Marshal(c.Arguments)
			choice.Message.ToolCalls = append(choice.Message.ToolCalls, openai.ToolCall{
				ID:   openai.ToolCallID(),
				Type: "function",
				Function: openai.FunctionCall{
					Name:      c.Name,
					Arguments: string(argJSON),
				},
			})
		}
		choice.FinishReason = "tool_calls"
	}
	resp.Choices = []openai.Choice{choice}
	resp.Usage = openai.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.InputTokens + usage.OutputTokens,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, msg, typ, param string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openai.Error{Error: openai.ErrorBody{
		Message: msg, Type: typ, Param: param,
	}})
}
