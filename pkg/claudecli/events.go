package claudecli

// Event is one item emitted by Spawn() while the subprocess runs.
//
// Only one of TextDelta / Usage / Err / TurnEnd / Done is meaningful per Event.
type Event struct {
	TextDelta string
	Usage     *Usage
	Err       error
	// TurnEnd is set on each "result" event from the CLI. Claude CLI processes
	// a multi-turn stream-json input by replaying the conversation: it spawns
	// one Anthropic API call per user message in the input. So a 3-message
	// history (user/assistant/user) produces TWO turn ends — only the LAST
	// turn's deltas are the real assistant reply we care about. Consumers
	// should reset their parser state on TurnEnd and use only the deltas
	// emitted after the most recent TurnEnd.
	TurnEnd bool
	Done    bool
}

type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}
