package models

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/eduard256/claudecode2openaiapi/api"
)

func Init() {
	api.HandleFunc("/v1/models", handle)
}

type model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

var catalog = []string{
	"sonnet",
	"opus",
	"haiku",
	"claude-sonnet-4-6",
	"claude-opus-4-7",
	"claude-haiku-4-5",
}

func handle(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	out := struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}{
		Object: "list",
		Data:   make([]model, 0, len(catalog)),
	}
	for _, id := range catalog {
		out.Data = append(out.Data, model{
			ID:      id,
			Object:  "model",
			Created: now,
			OwnedBy: "anthropic",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
