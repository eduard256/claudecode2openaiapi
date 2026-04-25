package claudecli

// Event is one item emitted by Spawn() while the subprocess runs.
//
// Only one of TextDelta / Usage / Err / Done is meaningful per Event.
type Event struct {
	TextDelta string
	Usage     *Usage
	Err       error
	Done      bool
}

type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}
