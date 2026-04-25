# claudecode2openaiapi

OpenAI-compatible HTTP API on top of the local `claude` CLI.

Lets any tool that speaks OpenAI Chat Completions (Cline, OpenCode, OpenWebUI, LangChain, curl) talk to a Claude Pro/Max subscription. The CLI handles auth and billing through your subscription. This server only translates the protocol.

No MCP. No built-in Claude Code tools. The model is fully isolated — no `CLAUDE.md` discovery, no memory, no skills, no `.credentials.json` outside the symlink. About 100 tokens of overhead per request.

## What works

- `POST /v1/chat/completions` — streaming and non-streaming
- Tool calling — `tools`, `tool_choice`, `tool_calls`, `role:tool`
- Parallel tool calls
- Vision — `image_url` with `data:` or `https://`
- Multi-turn history (the CLI replays it; we forward only the final turn)
- `GET /v1/models`
- `GET /health`
- Bearer auth with locally-managed `sk-cc2oa-*` tokens
- Anthropic prompt cache works automatically — no flags, no markers

## Requirements

- Go 1.26+
- `claude` CLI installed and logged in once: `claude /login`
- Claude Pro / Max / Team / Enterprise subscription

The CLI does the auth. This server never touches your credentials directly — it just symlinks `~/.claude/.credentials.json` into an isolated fake `$HOME` for each spawn.

## Install

```sh
go install github.com/eduard256/claudecode2openaiapi@latest
```

## Run

```sh
sudo claudecode2openaiapi tokens add my-laptop
# sk-cc2oa-...

sudo claudecode2openaiapi
```

Use it:

```sh
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-cc2oa-..." \
  -H "Content-Type: application/json" \
  -d '{"model":"sonnet","messages":[{"role":"user","content":"hello"}]}'
```

## Tokens

```
claudecode2openaiapi tokens add <name>     # generate, print to stdout
claudecode2openaiapi tokens list           # name, token, created_at
claudecode2openaiapi tokens rm <name>
claudecode2openaiapi tokens show <name>    # print token in full
```

Stored at `/var/lib/claudecode2openaiapi/tokens.json`, 0600.

## Models

`sonnet`, `opus`, `haiku` aliases pass through to the CLI. Full ids like `claude-sonnet-4-6` work too. `gpt-4o` and `gpt-4o-mini` are aliased to `sonnet` and `haiku` so OpenAI clients with hardcoded model names just work.

## Tool calling

The model is taught a text protocol via the system prompt:

```
<tool_calls>
<tool_call>{"name":"NAME","arguments":{...}}</tool_call>
</tool_calls>
```

When the parser sees `</tool_calls>` in the stream it kills the subprocess immediately and emits OpenAI-shaped `tool_calls` to the client. No native MCP, no `tool_use` blocks. Multiple `<tool_call>` inside the same wrapper run as parallel tool calls.

When the client sends `role:"tool"` results back, history is replayed as text (`<tool_calls>...` for the assistant turn, `<tool_result>...</tool_result>` for the result). The CLI processes the input as N sub-turns, but only the last one is forwarded to the client.

**Important:**
1. Sonnet and Opus follow the protocol reliably. Haiku struggles with multi-step tool chains.
2. The model sometimes writes a short preamble before `<tool_calls>` ("Let me check..."). It's streamed to the client as content, then `tool_calls` follows. OpenAI allows this.

## Environment

- `CC2OA_LISTEN` — listen address. Default `:8080`.
- `CC2OA_DATA_DIR` — data directory. Default `/var/lib/claudecode2openaiapi`.

## Limitations

- Multi-turn requests are slower because the CLI replays the conversation turn-by-turn (one Anthropic call per user message).
- `n > 1`, `logprobs`, `logit_bias`, `presence_penalty`, `frequency_penalty` are ignored.
- `seed` is accepted and ignored. Anthropic doesn't expose it through the CLI.
- Audio modality not supported.

## License

MIT
