package openai

import (
	"crypto/rand"
	"encoding/hex"
)

// CompletionID -> "chatcmpl-<24hex>"
func CompletionID() string {
	return "chatcmpl-" + randHex(12)
}

// ToolCallID -> "call_<24hex>"
func ToolCallID() string {
	return "call_" + randHex(12)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
