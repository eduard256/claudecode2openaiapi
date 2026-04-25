package sse

import (
	"encoding/json"
	"net/http"
)

// Writer pushes data: <json>\n\n frames and flushes after each.
type Writer struct {
	w http.ResponseWriter
	f http.Flusher
}

func NewWriter(w http.ResponseWriter) *Writer {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	f, _ := w.(http.Flusher)
	return &Writer{w: w, f: f}
}

func (s *Writer) Write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err = s.w.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err = s.w.Write(b); err != nil {
		return err
	}
	if _, err = s.w.Write([]byte("\n\n")); err != nil {
		return err
	}
	if s.f != nil {
		s.f.Flush()
	}
	return nil
}

func (s *Writer) Done() {
	_, _ = s.w.Write([]byte("data: [DONE]\n\n"))
	if s.f != nil {
		s.f.Flush()
	}
}
