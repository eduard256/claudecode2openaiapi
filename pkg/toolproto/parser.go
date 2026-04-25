package toolproto

import (
	"encoding/json"
	"regexp"
	"strings"
)

const (
	OpenWrapper  = "<tool_calls>"
	CloseWrapper = "</tool_calls>"
	OpenCall     = "<tool_call>"
	CloseCall    = "</tool_call>"
)

// State of the streaming parser.
type State byte

const (
	StateText State = iota
	StateBetween
	StateInsideCall
)

// Call is one parsed tool invocation.
type Call struct {
	Name      string
	Arguments map[string]any
}

var callRe = regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)

// Extract pulls out all <tool_call>...</tool_call> blocks from text.
// Used after the subprocess has been killed at </tool_calls>.
func Extract(text string) []Call {
	matches := callRe.FindAllStringSubmatch(text, -1)
	out := make([]Call, 0, len(matches))
	for _, m := range matches {
		var raw struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(m[1]), &raw); err != nil {
			continue
		}
		out = append(out, Call{Name: raw.Name, Arguments: raw.Arguments})
	}
	return out
}

// CleanAssistant returns just the <tool_calls>...</tool_calls> wrapper, with
// any preamble or post-junk stripped. If no wrapper found, returns text as-is.
func CleanAssistant(text string) string {
	a := strings.Index(text, OpenWrapper)
	b := strings.Index(text, CloseWrapper)
	if a < 0 || b < 0 {
		return text
	}
	return text[a : b+len(CloseWrapper)]
}

// Stream is an incremental text-stream parser. Feed it chunks, it tells you
// when wrapper opens and closes so you can decide what to forward to the
// client (text deltas) and when to kill the subprocess (close wrapper).
type Stream struct {
	state State
	buf   strings.Builder
}

// NewStream returns a fresh parser in StateText.
func NewStream() *Stream { return &Stream{} }

// Event describes what the parser saw on the latest Feed.
type Event struct {
	// TextDelta is non-empty when in plain-text mode and chunk should be forwarded.
	TextDelta string
	// WrapperOpened is true when </tool_calls> wrapper just started; stop forwarding text.
	WrapperOpened bool
	// WrapperClosed is true when </tool_calls> just appeared; caller should kill subprocess.
	WrapperClosed bool
}

// Feed appends one chunk and returns what happened. The state machine works at
// rune level so multi-byte tag boundaries split across chunks are detected.
func (p *Stream) Feed(chunk string) Event {
	var ev Event
	for _, r := range chunk {
		ch := string(r)
		p.buf.WriteString(ch)
		s := p.buf.String()

		switch p.state {
		case StateText:
			if strings.HasSuffix(s, OpenWrapper) {
				p.state = StateBetween
				ev.WrapperOpened = true
				continue
			}
			ev.TextDelta += ch

		case StateBetween:
			if strings.HasSuffix(s, OpenCall) {
				p.state = StateInsideCall
				continue
			}
			if strings.HasSuffix(s, CloseWrapper) {
				ev.WrapperClosed = true
				return ev
			}

		case StateInsideCall:
			if strings.HasSuffix(s, CloseCall) {
				p.state = StateBetween
			}
		}
	}
	return ev
}

// Buffered returns everything seen so far (used by Extract after kill).
func (p *Stream) Buffered() string { return p.buf.String() }
