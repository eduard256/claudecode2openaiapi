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
//
// In StateText we hold back a tail of characters that COULD be the start of
// "<tool_calls>" — so the client never sees a partial tag leak through.
type Stream struct {
	state State
	buf   strings.Builder // everything ever fed (used by Extract after kill)
	hold  strings.Builder // pending text that might be a wrapper prefix
}

// NewStream returns a fresh parser in StateText.
func NewStream() *Stream { return &Stream{} }

// Event describes what the parser saw on the latest Feed.
type Event struct {
	// TextDelta is non-empty when in plain-text mode and chunk should be forwarded.
	TextDelta string
	// WrapperOpened is true when <tool_calls> wrapper just started; stop forwarding text.
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

		switch p.state {
		case StateText:
			p.hold.WriteString(ch)
			h := p.hold.String()

			// Full match -> enter wrapper mode, drop the held prefix.
			if strings.HasSuffix(h, OpenWrapper) {
				p.state = StateBetween
				p.hold.Reset()
				ev.WrapperOpened = true
				continue
			}
			// If the tail of the hold buffer is a possible prefix of
			// "<tool_calls>", keep holding. Otherwise flush all but the
			// longest possible-prefix tail as text to the client.
			cut := len(h) - longestPrefixMatch(h, OpenWrapper)
			if cut > 0 {
				ev.TextDelta += h[:cut]
				rest := h[cut:]
				p.hold.Reset()
				p.hold.WriteString(rest)
			}

		case StateBetween:
			s := p.buf.String()
			if strings.HasSuffix(s, OpenCall) {
				p.state = StateInsideCall
				continue
			}
			if strings.HasSuffix(s, CloseWrapper) {
				ev.WrapperClosed = true
				return ev
			}

		case StateInsideCall:
			s := p.buf.String()
			if strings.HasSuffix(s, CloseCall) {
				p.state = StateBetween
			}
		}
	}
	return ev
}

// longestPrefixMatch returns the length of the longest suffix of s that is
// also a prefix of pattern. Used to decide how much of the hold buffer must
// stay buffered because it might still grow into the full tag.
func longestPrefixMatch(s, pattern string) int {
	max := len(pattern) - 1
	if max > len(s) {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasSuffix(s, pattern[:n]) {
			return n
		}
	}
	return 0
}

// Flush drains any text held in the prefix-buffer. Call this when the
// upstream stream ends in StateText — otherwise the trailing characters
// would never reach the client.
func (p *Stream) Flush() string {
	if p.state != StateText {
		return ""
	}
	out := p.hold.String()
	p.hold.Reset()
	return out
}

// Buffered returns everything seen so far (used by Extract after kill).
func (p *Stream) Buffered() string { return p.buf.String() }
