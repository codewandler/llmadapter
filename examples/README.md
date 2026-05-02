# Examples

This directory contains load-tested config examples for `llmadapter`.

## Files

- `llmadapter.example.json`: gateway/mux config with multiple provider instances, an explicit operator-defined alias, dynamic routes, capability overrides, modeldb metadata, pricing, and fallback limits.
- `modeldb.overlay.example.json`: small modeldb overlay used by the example config.

## Inspect

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
go run ./cmd/llmadapter routes --config examples/llmadapter.example.json
go run ./cmd/llmadapter models --config examples/llmadapter.example.json
go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json example-fast
```

Inspection does not require provider credentials.

## Run

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json
```

The example references these credentials when live provider calls are made:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY`
- `OPENROUTER_API_KEY`
- `MINIMAX_API_KEY`
- local Claude Code OAuth credentials for the `claude` provider instance
- local Codex/ChatGPT OAuth credentials for the `codex_responses` provider instance

## Adapt

Copy `llmadapter.example.json` to your own config and change:

- provider instances and env vars,
- route weights and provider priorities,
- modeldb aliases owned by your application or operator config,
- modeldb overlay offerings,
- capability overrides for specific provider/model combinations.
