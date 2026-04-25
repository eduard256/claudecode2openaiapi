package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/eduard256/claudecode2openaiapi/api"
	"github.com/eduard256/claudecode2openaiapi/pkg/openai"
	"github.com/eduard256/claudecode2openaiapi/pkg/tokens"
)

var store *tokens.Store

func Init(s *tokens.Store) {
	store = s
	api.Use(middleware)
}

// Allowlist of paths that don't require auth.
var public = map[string]bool{
	"/health": true,
}

func middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if public[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		const pfx = "Bearer "
		if !strings.HasPrefix(auth, pfx) {
			writeError(w, 401, "Missing Authorization: Bearer header", "authentication_error", "missing_credentials")
			return
		}
		token := strings.TrimSpace(auth[len(pfx):])
		if _, ok := store.Find(token); !ok {
			writeError(w, 401, "Invalid API key", "authentication_error", "invalid_api_key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, msg, typ, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openai.Error{Error: openai.ErrorBody{
		Message: msg, Type: typ, Code: code,
	}})
}
