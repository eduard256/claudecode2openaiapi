// claudecode2openaiapi -- OpenAI-compatible HTTP API backed by the local
// `claude` CLI subprocess. Self-hosted, stateless, no MCP, no built-in tools.
//
//   claudecode2openaiapi              — start the server (default :8080)
//   claudecode2openaiapi tokens add <name>
//   claudecode2openaiapi tokens list
//   claudecode2openaiapi tokens rm <name>
//   claudecode2openaiapi tokens show <name>
package main

import (
	"log/slog"
	"os"

	"github.com/eduard256/claudecode2openaiapi/internal/auth"
	"github.com/eduard256/claudecode2openaiapi/internal/chat"
	"github.com/eduard256/claudecode2openaiapi/internal/health"
	"github.com/eduard256/claudecode2openaiapi/internal/models"
	"github.com/eduard256/claudecode2openaiapi/internal/server"
	"github.com/eduard256/claudecode2openaiapi/internal/tokenscli"
	"github.com/eduard256/claudecode2openaiapi/pkg/isolation"
	"github.com/eduard256/claudecode2openaiapi/pkg/tokens"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "tokens" {
		tokenscli.Run(os.Args[2:])
		return
	}

	if err := isolation.Setup(); err != nil {
		slog.Error("isolation setup failed", "err", err)
		os.Exit(1)
	}

	store := tokens.NewStore(isolation.TokensFile)
	if err := store.Load(); err != nil {
		slog.Error("tokens load failed", "err", err)
		os.Exit(1)
	}

	auth.Init(store)
	health.Init()
	models.Init()
	chat.Init()
	server.Init()

	select {} // block forever
}
