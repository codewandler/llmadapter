# llmadapter

`llmadapter` is a Go adapter layer for routing LLM requests across provider API shapes through a canonical `unified.Request` / `unified.Event` stream.

Current implemented surface:

- Core packages: `unified`, `adapt`, `pipeline`, `transport`, `router`, `gateway`.
- Endpoint codecs: OpenAI-compatible `/v1/chat/completions`, OpenAI-compatible `/v1/responses`, Anthropic-compatible `/v1/messages`.
- Providers: Anthropic Messages, OpenAI Chat Completions, OpenRouter Chat Completions, OpenRouter Responses, OpenRouter Anthropic-compatible Messages, MiniMax Chat Completions, MiniMax Anthropic-compatible Messages.
- Live e2e smoke tests for text streaming, tool calls, tool-result continuation, and gateway routing.

## Quick Start

Run local verification:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Run live e2e tests:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -v
```

Live tests skip when credentials are missing. Supported credential env vars:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY` or `OPENAI_KEY`
- `OPENROUTER_API_KEY` or `OPENROUTER_KEY`
- `MINIMAX_API_KEY` or `MINIMAX_KEY`

Provider model override env vars:

- `ANTHROPIC_MODEL`
- `OPENAI_MODEL`
- `OPENROUTER_MODEL`
- `OPENROUTER_RESPONSES_MODEL`
- `OPENROUTER_MESSAGES_MODEL`
- `MINIMAX_MODEL`
- `MINIMAX_MESSAGES_MODEL`

## Gateway

The gateway command is `cmd/llmadapter-gateway`.

Configuration:

- `LLMADAPTER_CONFIG` points to a JSON config file.
- `LLMADAPTER_ADDR` sets the listen address when no config file is used.
- Routes select a provider endpoint with `provider` and optional `provider_api`.
- Routes can set `weight`; providers can set `priority`. Compatible routes are ranked by weight first, then endpoint priority, with declaration order as the final tie-breaker.
- The gateway exposes `/v1/chat/completions`, `/v1/responses`, and `/v1/messages`.

Example provider endpoint types:

- `anthropic`
- `openai_chat`
- `openrouter_chat`
- `openrouter_responses`
- `openrouter_messages`
- `minimax_chat`
- `minimax_messages`

## Design Notes

Provider routing is intentionally endpoint-based:

```text
Provider = who we talk to.
API kind = exact upstream wire protocol.
API family = compatibility family.
Provider endpoint = provider + API kind + family + client + capabilities.
```

Routes skip provider endpoints that cannot satisfy required request capabilities such as streaming, tools, JSON mode, JSON schema, reasoning, vision, or audio input, then rank the remaining candidates by configured weight and endpoint priority. If the selected provider fails before response bytes are written, the gateway retries lower-ranked route candidates.

OpenAI Chat-compatible `response_format` requests and OpenAI Responses `text.format` requests are mapped into canonical JSON mode / JSON schema settings. Those settings are encoded back for OpenAI Chat, OpenRouter Chat, and OpenRouter Responses providers.

OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs decode supported image inputs into canonical `unified.ImagePart` values. Vision routing is still capability-gated by provider metadata.

Best-effort endpoint codecs retain decode/lossiness warnings on `adapt.Request`, and provider mappings emit canonical warning events for common unsupported dropped fields. Collected responses expose provider-side warnings under `Response.Warnings`.

OpenRouter-specific request controls are carried through `unified.Request.Extensions` using namespaced keys such as `openrouter.provider`, `openrouter.plugins`, `openrouter.debug`, `openrouter.trace`, and `openrouter.session_id`. The OpenRouter Chat, Responses, and Messages providers encode those extensions back into upstream request bodies.

See `DESIGN.md` for the target architecture and `PLAN.md` for current status, known gaps, and next implementation phases.
