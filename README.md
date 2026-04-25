# claudecode2openaiapi

OpenAI-compatible HTTP API backed by the local `claude` CLI subprocess.
Self-hosted, stateless, MIT-licensed.

Lets any OpenAI-compatible client (Cline, OpenCode, OpenWebUI, LangChain,
curl, etc.) talk to a Claude Max / Pro subscription **without** paying per
token — Claude Code CLI handles the auth and billing through the
subscription, this server just translates the protocol.

## Features

- `POST /v1/chat/completions` — full OpenAI-compatible endpoint
  - streaming and non-streaming
  - tool calling (`tools`, `tool_choice`, `tool_calls`, `role:tool`)
  - parallel tool calls
  - vision (`image_url` with `data:` or `https://`)
- `GET /v1/models` — model catalog
- `GET /health` — unauthenticated health check
- Bearer auth with locally-managed tokens (`sk-cc2oa-...`)
- Anthropic prompt cache works automatically — no extra config

## Requirements

- Go 1.26+
- `claude` CLI installed and logged in (`claude /login` once)
- Claude Pro / Max / Team / Enterprise subscription

## Install

```sh
go install github.com/eduard256/claudecode2openaiapi@latest
```

## Quick start

```sh
# 1. Generate an API key
sudo claudecode2openaiapi tokens add my-laptop
# -> sk-cc2oa-...

# 2. Run the server
sudo claudecode2openaiapi

# 3. Use it
curl https://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-cc2oa-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"sonnet","messages":[{"role":"user","content":"hello"}]}'
```

## Token management

```
claudecode2openaiapi tokens add <name>   # generate and print new token
claudecode2openaiapi tokens list         # list tokens
claudecode2openaiapi tokens rm <name>    # remove
claudecode2openaiapi tokens show <name>  # print token in full
```

Tokens are stored at `/var/lib/claudecode2openaiapi/tokens.json`.

## Environment variables

- `CC2OA_LISTEN` — listen address (default `:8080`)
- `CC2OA_DATA_DIR` — data directory (default `/var/lib/claudecode2openaiapi`)

## License

MIT
