# llmadapter

`llmadapter` is a stateless Go library and gateway for adapting LLM provider APIs into one canonical request/event stream.

Use it when you want one routing layer for Anthropic, OpenAI, OpenRouter, MiniMax, Claude Code-compatible access, Codex/ChatGPT-compatible access, and related API shapes without pretending every provider is identical.

## What It Is

`llmadapter` provides three related surfaces:

- A Go `unified.Client` abstraction for application and agent runtimes.
- An HTTP compatibility gateway exposing `/v1/chat/completions`, `/v1/responses`, and `/v1/messages`.
- A CLI for provider discovery, model resolution, inference, gateway serving, and smoke testing.

The core path is:

```text
downstream API request
  -> endpoint codec
  -> unified.Request / unified.Event stream
  -> provider endpoint
  -> upstream provider API
```

Provider-specific behavior is preserved through explicit capabilities, warnings, usage/cost events, reasoning signatures, citations, raw metadata, and namespaced extensions.

## Who It Is For

- Agent/runtime authors who want one Go client over multiple LLM providers.
- Tooling authors who need model routing, provider inspection, usage/cost accounting, and smoke tests.
- Gateway operators who want OpenAI/Anthropic-compatible endpoints backed by configurable upstream providers.
- Provider implementers who want to add one API kind without building `M x N` adapters.
- Higher-level systems such as `miniagent` or `agentsdk` that need stateless provider access while owning conversation/session policy themselves.

## What It Is Not

- It is not a stateful conversation database.
- It is not a full clone of every upstream provider field.
- It is not a hidden fallback layer that silently substitutes unrelated models.
- It is not where long-term memory, replay policy, context management, or session cache strategy should live.

Stateful conversation/session logic belongs above `unified.Client`, for example in `agentsdk`.

## Main Use Cases

- **Go library mux client:** build a `unified.Client` from env/local credentials or JSON config and let it route requests.
- **HTTP gateway:** expose OpenAI/Anthropic-shaped endpoints while routing upstream by provider, model, API kind, capabilities, and health.
- **CLI inference:** run `llmadapter infer` to inspect route resolution, stream reasoning/text, and print usage.
- **Continuation diagnostics:** route events and CLI output expose the public continuation contract consumers must follow, plus diagnostic provider transport/internal-continuation details for observability.
- **Provider diagnostics:** run `providers`, `routes`, `models`, `resolve`, and `smoke` to understand exactly what will happen.
- **Transport troubleshooting:** use `infer --debug` or the `diagnostics` package to inspect redacted HTTP/SSE and WebSocket request, response, and stream frames.
- **Provider extension:** add a provider endpoint with explicit API kind/family and shared smoke coverage.

## Supported Provider Endpoints

Current v1 release-candidate provider endpoint types:

- `anthropic`
- `claude`
- `openai_chat`
- `openai_responses`
- `codex_responses`
- `openrouter_chat`
- `openrouter_responses`
- `openrouter_messages`
- `minimax_chat`
- `minimax_messages`

Supported downstream gateway endpoints:

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/messages`

See [docs/PROVIDER_MATRIX.md](docs/PROVIDER_MATRIX.md) for feature coverage, credential triggers, smoke-test status, and known provider-specific limits.
See [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) for WebSocket close, context-window, and transport-mode debugging.

## Quick Start

List provider endpoint types:

```sh
go run ./cmd/llmadapter providers
```

Inspect auto-detected credentials:

```sh
go run ./cmd/llmadapter providers --auto
```

Run one prompt through the auto-detected mux client:

```sh
go run ./cmd/llmadapter infer -m anthropic/claude-haiku-4-5-20251001 "reply with one short sentence"
```

Test session-style continuation hints without giving llmadapter ownership of the conversation:

```sh
go run ./cmd/llmadapter infer -m codex/gpt-5.4 --session demo --branch main "continue"
```

For Codex, session mode can use provider-internal WebSocket transport when available, while callers still send a full replay-style request. If your consumer owns a branchable conversation tree, pass stable Codex session and branch hints so llmadapter can keep internal WebSocket continuation state branch-safe.

Explain how a model will route:

```sh
go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001
```

Inspect provider endpoint conformance and approved compatibility evidence:

```sh
go run ./cmd/llmadapter conformance
```

Select only provider/model/API paths approved for agentic coding:

```sh
go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001 --use-case agentic_coding --approved-only
```

Run a config-driven gateway:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json
```

Inspect the same config without requiring provider credentials:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
```

Run the gateway in Docker:

```sh
docker build -t llmadapter:local .
docker run --rm -p 8080:8080 -w /app \
  -v "$PWD/examples:/app/examples:ro" \
  -e ANTHROPIC_API_KEY -e OPENAI_API_KEY -e OPENROUTER_API_KEY \
  llmadapter:local serve --config examples/llmadapter.example.json
```

See [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md) for a guided first run.

## Go Library Example

Auto-detect env/local credentials and send a request through the mux client:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/unified"
)

func main() {
	ctx := context.Background()

	result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
		UseModelDB: true,
		Intents: []adapterconfig.AutoIntent{
			{Name: "anthropic/claude-haiku-4-5-20251001"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	maxTokens := 256
	events, err := result.Client.Request(ctx, unified.Request{
		Model:           "anthropic/claude-haiku-4-5-20251001",
		MaxOutputTokens: &maxTokens,
		Stream:          true,
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "Say hello in one sentence."},
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	resp, err := unified.Collect(ctx, events)
	if err != nil {
		log.Fatal(err)
	}
	var text strings.Builder
	for _, part := range resp.Content {
		if part, ok := part.(unified.TextPart); ok {
			text.WriteString(part.Text)
		}
	}
	fmt.Println(text.String())
}
```

See [docs/LIBRARY_USAGE.md](docs/LIBRARY_USAGE.md) for config-driven mux, direct provider clients, usage, pricing, caching, and extensions.

## Credentials

Live provider calls use standard env vars or local OAuth files:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY` or `OPENAI_KEY`
- `OPENROUTER_API_KEY` or `OPENROUTER_KEY`
- `MINIMAX_API_KEY` or `MINIMAX_KEY`
- local Claude Code OAuth credentials in `~/.claude/.credentials.json`; set `CLAUDE_CONFIG_DIR` to override the directory.
- `CODEX_ACCESS_TOKEN`, `CODEX_CODE_OAUTH_TOKEN`, or local Codex OAuth credentials in `~/.codex/auth.json`; set `CODEX_AUTH_PATH` to override the file.

Model override env vars:

- `ANTHROPIC_MODEL`
- `CLAUDE_MODEL`
- `CODEX_MODEL`
- `OPENAI_MODEL`
- `OPENAI_RESPONSES_MODEL`
- `OPENROUTER_MODEL`
- `OPENROUTER_RESPONSES_MODEL`
- `OPENROUTER_MESSAGES_MODEL`
- `MINIMAX_MODEL`
- `MINIMAX_MESSAGES_MODEL`

The CLI redacts credential status. Use `llmadapter providers --auto` or `llmadapter providers --status --config <path>` to inspect what is enabled.

## Core Concepts

- **Provider:** who llmadapter talks to, for example OpenRouter or Anthropic.
- **Provider instance:** a configured provider name such as `anthropic`, `claude`, or `openrouter`.
- **Provider type:** implementation type such as `openrouter_responses`.
- **API kind:** exact wire protocol, for example `openrouter.responses`.
- **API family:** compatibility shape, for example `openai.responses`.
- **Provider endpoint:** provider instance plus API kind/family, client, capabilities, priority, and tags.
- **Route:** source API/model selection to a provider endpoint and native model.
- **Modeldb:** model/catalog metadata used for aliases, model existence, capabilities, token limits, and pricing.

Routing targets provider endpoints, not just providers. This is why OpenRouter and MiniMax can expose multiple API kinds without being collapsed into one ambiguous provider mode.

## Documentation Map

- [docs/GETTING_STARTED.md](docs/GETTING_STARTED.md): first run, credentials, CLI, gateway, Docker.
- [docs/CLI.md](docs/CLI.md): command reference and common workflows.
- [docs/CONFIGURATION.md](docs/CONFIGURATION.md): JSON config, routes, modeldb, aliases, dynamic models, capabilities, pricing.
- [docs/LIBRARY_USAGE.md](docs/LIBRARY_USAGE.md): Go client patterns.
- [docs/PROVIDER_MATRIX.md](docs/PROVIDER_MATRIX.md): provider endpoint support and smoke coverage.
- [docs/USE_CASE_MATRIX.md](docs/USE_CASE_MATRIX.md): workload compatibility status for agentic coding and summarization.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): package architecture and request flow.
- [docs/API_SURFACE.md](docs/API_SURFACE.md): public package boundary.
- [docs/PROVIDER_DEVELOPMENT.md](docs/PROVIDER_DEVELOPMENT.md): adding provider endpoints/API kinds.
- [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md): common transport, provider, and continuation issues.
- [CHANGELOG.md](CHANGELOG.md): release history.

## Verification

Local checks:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Live provider smoke matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```

Live tests skip when `TEST_INTEGRATION` is unset or credentials are missing.

## Stability And Limits

The `v1.0.0-rc` track is intended to stabilize the stateless adapter/gateway/mux surface before the first v1 release.

Stable behavior:

- CLI, gateway, mux client, and auto mux use the same `adapterconfig` and modeldb-backed resolution path.
- Capability decisions are inspectable through `resolve` and `serve --inspect-config`.
- Gateway/mux fallback is deterministic and only retries before response bytes have started.
- Unsupported or lossy fields are rejected, warned, or preserved in namespaced extensions; they are not silently treated as supported.

Known limits:

- Provider paths are compatibility surfaces, not exhaustive provider API clones.
- Some provider cache controls are mapped but not live-verified for provider-reported cache accounting.
- Broad audio/video/file/document/built-in tool support remains post-v1 expansion.
- Additional provider families such as Ollama, Bedrock, Vertex, Azure OpenAI, Gemini, Mistral, or Cohere remain future work.
