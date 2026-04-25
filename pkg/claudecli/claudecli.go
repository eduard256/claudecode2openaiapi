// Package claudecli wraps the `claude` CLI subprocess. It handles all the
// isolation flags we determined experimentally and translates the noisy
// stream-json output into a clean Event channel.
//
// Spawn returns a Process whose Events channel emits TextDelta values as the
// model writes, plus a final Usage and a Done sentinel. Caller can call
// Kill() at any time to terminate generation early.
package claudecli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/eduard256/claudecode2openaiapi/pkg/isolation"
)

// Options for one spawn.
type Options struct {
	Model        string // "sonnet" | "opus" | "haiku" | full id
	SystemPrompt string
	StdinLines   []string // already-formatted stream-json lines (without trailing \n)
}

type Process struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	Events chan Event
}

// Spawn starts claude with our standard isolation. Reads stdin lines from
// opts.StdinLines, joins with "\n", closes stdin so claude knows the input is
// complete, and streams events.
func Spawn(opts Options) (*Process, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, "claude",
		"--tools", "",
		"--print",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--no-session-persistence",
		"--strict-mcp-config",
		"--mcp-config", `{"mcpServers":{}}`,
		"--setting-sources", "",
		"--disable-slash-commands",
		"--verbose",
		"--model", opts.Model,
		"--system-prompt", opts.SystemPrompt,
	)
	cmd.Dir = isolation.WorkDir
	cmd.Env = append(os.Environ(),
		"HOME="+isolation.FakeHome,
		"MAX_THINKING_TOKENS=0",
		"CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING=1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Stderr = io.Discard

	if err = cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	go func() {
		defer stdin.Close()
		for _, line := range opts.StdinLines {
			_, _ = io.WriteString(stdin, line+"\n")
		}
	}()

	p := &Process{
		cmd:    cmd,
		cancel: cancel,
		Events: make(chan Event, 64),
	}
	go p.read(stdout)
	return p, nil
}

func (p *Process) Kill() {
	p.cancel()
}

func (p *Process) read(stdout io.ReadCloser) {
	defer close(p.Events)
	defer func() { _ = p.cmd.Wait() }()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}

		var t string
		_ = json.Unmarshal(env["type"], &t)

		switch t {
		case "stream_event":
			p.handleStreamEvent(env["event"])
		case "result":
			if u, ok := parseUsage(env["usage"]); ok {
				p.send(Event{Usage: u})
			}
			if isErr := parseBool(env["is_error"]); isErr {
				var msg string
				_ = json.Unmarshal(env["result"], &msg)
				p.send(Event{Err: errors.New("claudecli: " + msg)})
			}
		}
	}
	p.send(Event{Done: true})
}

func (p *Process) handleStreamEvent(raw json.RawMessage) {
	var ev struct {
		Type    string          `json:"type"`
		Delta   json.RawMessage `json:"delta"`
		Message json.RawMessage `json:"message"`
		Usage   json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		return
	}

	switch ev.Type {
	case "content_block_delta":
		var d struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(ev.Delta, &d); err != nil {
			return
		}
		if d.Type == "text_delta" && d.Text != "" {
			p.send(Event{TextDelta: d.Text})
		}

	case "message_start":
		// Usage on message_start has accurate input/cache numbers.
		var msg struct {
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(ev.Message, &msg); err != nil {
			return
		}
		if u, ok := parseUsage(msg.Usage); ok {
			p.send(Event{Usage: u})
		}

	case "message_delta":
		// Each message_delta carries an updated output_tokens count. The
		// last one we see before kill has the most accurate output count.
		if u, ok := parseUsage(ev.Usage); ok {
			p.send(Event{Usage: u})
		}
	}
}

func (p *Process) send(e Event) {
	// Block until consumer reads. If channel was already closed by read()'s
	// defer, the second goroutine that tries to send will panic — but that
	// can't happen here because send() is only called from read() itself.
	p.Events <- e
}

// parseUsage extracts the four numbers we care about from the result usage object.
func parseUsage(raw json.RawMessage) (*Usage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var u struct {
		Input  int `json:"input_tokens"`
		Output int `json:"output_tokens"`
		Create int `json:"cache_creation_input_tokens"`
		Read   int `json:"cache_read_input_tokens"`
	}
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, false
	}
	return &Usage{
		InputTokens:         u.Input,
		OutputTokens:        u.Output,
		CacheCreationTokens: u.Create,
		CacheReadTokens:     u.Read,
	}, true
}

func parseBool(raw json.RawMessage) bool {
	var b bool
	_ = json.Unmarshal(raw, &b)
	return b
}
