# llmadapter

`llmadapter` is a stateless adapter layer for LLM providers.

It gives Go applications, CLIs, and gateways one canonical request/event stream across OpenAI, Anthropic, OpenRouter, MiniMax, Claude Code-compatible access, Codex/ChatGPT-compatible access, and related API shapes without pretending those providers are identical.

## Why It Exists

LLM providers expose similar concepts through different wire protocols, naming schemes, streaming formats, tool-call shapes, cache controls, quota signals, and continuation models.

`llmadapter` keeps those differences explicit while giving consumers a common integration surface:

- one Go `unified.Client` abstraction,
- one configurable routing/mux layer,
- OpenAI/Anthropic-compatible gateway endpoints,
- provider/model/capability inspection,
- usage, cost, quota, reasoning, citation, and warning events,
- stream-first transports with conservative retry/fallback behavior.

It is built for agent runtimes and infrastructure that need predictable provider access while owning their own conversation state, tool execution, memory, compaction, and session policy.

## What You Can Do

- Build a Go client that routes across provider endpoints.
- Run an HTTP gateway exposing `/v1/chat/completions`, `/v1/responses`, and `/v1/messages`.
- Inspect routes, modeldb-backed capabilities, pricing, and provider availability.
- Run CLI inference through real providers with redacted debug output.
- Add a new provider endpoint without building `M x N` adapters.

## Supported Provider Surfaces

Current v1 release-candidate endpoint types:

| Endpoint type | Upstream shape |
| --- | --- |
| `anthropic` | Anthropic Messages |
| `claude` | Claude Code-compatible Anthropic Messages |
| `openai_chat` | OpenAI Chat Completions |
| `openai_responses` | OpenAI Responses |
| `codex_responses` | Codex/ChatGPT-compatible Responses |
| `openrouter_chat` | OpenRouter Chat Completions |
| `openrouter_responses` | OpenRouter Responses |
| `openrouter_messages` | OpenRouter Anthropic Messages |
| `minimax_chat` | MiniMax OpenAI-compatible Chat |
| `minimax_messages` | MiniMax Anthropic-compatible Messages |

See [docs/PROVIDER_MATRIX.md](docs/PROVIDER_MATRIX.md) for feature coverage and live smoke-test status.

## Quick Start

List provider endpoint types:

```sh
go run ./cmd/llmadapter providers
```

Inspect auto-detected credentials without printing secrets:

```sh
go run ./cmd/llmadapter providers --auto
```

Run one prompt through the auto-detected mux client:

```sh
go run ./cmd/llmadapter infer -m anthropic/claude-haiku-4-5-20251001 "reply with one short sentence"
```

Inspect a config without making provider calls:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
```

Start the gateway:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json
```

Call the OpenAI Responses-compatible endpoint:

```sh
curl -sS http://localhost:8080/v1/responses \
  -H 'content-type: application/json' \
  -d '{"model":"example-fast","input":"Reply with exactly: llmadapter ok"}'
```

## Documentation

Start with [docs/README.md](docs/README.md).

The repository includes a GitHub Pages workflow that can publish the `docs/` folder as a documentation site.

Most users want:

- [Getting Started](docs/GETTING_STARTED.md)
- [CLI](docs/CLI.md)
- [Configuration](docs/CONFIGURATION.md)
- [Library Usage](docs/LIBRARY_USAGE.md)
- [Provider Matrix](docs/PROVIDER_MATRIX.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)

## Credentials

Live provider calls use standard env vars or local OAuth files:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY` or `OPENAI_KEY`
- `OPENROUTER_API_KEY` or `OPENROUTER_KEY`
- `MINIMAX_API_KEY` or `MINIMAX_KEY`
- local Claude Code credentials in `~/.claude/.credentials.json`
- local Codex/ChatGPT credentials in `~/.codex/auth.json`

## Scope

`llmadapter` is intentionally not:

- a stateful conversation database,
- a long-term memory layer,
- a tool execution runtime,
- a provider-specific API clone,
- a hidden model substitution layer.

Stateful conversation/session policy belongs above `unified.Client`, for example in an agent runtime.

## Status

The `v1.0.0-rc` track is stabilizing the stateless adapter/gateway/mux surface before the first v1 release.

Core behavior is covered by local tests and credential-gated live smoke tests across the supported provider matrix. Provider-specific compatibility details are documented in [docs/PROVIDER_MATRIX.md](docs/PROVIDER_MATRIX.md).

## Development Checks

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Live matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```

## Release History

See [CHANGELOG.md](CHANGELOG.md).
