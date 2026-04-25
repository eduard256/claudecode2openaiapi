package health

import (
	"encoding/json"
	"net/http"

	"github.com/eduard256/claudecode2openaiapi/api"
)

func Init() {
	api.HandleFunc("/health", handle)
}

func handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
}
