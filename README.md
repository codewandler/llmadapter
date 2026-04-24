# llmadapter

`llmadapter` is a Go adapter layer for routing LLM requests across provider API shapes through a canonical `unified.Request` / `unified.Event` stream.

Current implemented surface:

- Core packages: `unified`, `adapt`, `pipeline`, `transport`, `router`, `gateway`.
- Utility packages: `pricing` for modeldb-backed usage cost enrichment and `modelmeta` for modeldb-backed endpoint metadata mapping.
- Endpoint codecs: OpenAI-compatible `/v1/chat/completions`, OpenAI-compatible `/v1/responses`, Anthropic-compatible `/v1/messages`.
- Providers: Anthropic Messages, Claude Code-compatible Anthropic Messages, OpenAI Chat Completions, OpenAI Responses, Codex Responses, OpenRouter Chat Completions, OpenRouter Responses, OpenRouter Anthropic-compatible Messages, MiniMax Chat Completions, MiniMax Anthropic-compatible Messages.
- Live e2e smoke tests for text streaming, tool calls, tool-result continuation, prompt caching, and gateway routing.

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

List provider endpoint types and run a minimal direct smoke through the CLI:

```sh
go run ./cmd/llmadapter providers
go run ./cmd/llmadapter routes
go run ./cmd/llmadapter models
go run ./cmd/llmadapter models --catalog --service openai --query gpt
go run ./cmd/llmadapter resolve gpt-4.1-mini
go run ./cmd/llmadapter serve --inspect-config
go run ./cmd/llmadapter smoke -type openai_responses
go run ./cmd/llmadapter smoke -mode mux -type openai_responses
go run ./cmd/llmadapter smoke -mode mux -config ./llmadapter.json -model public-fast
go run ./cmd/llmadapter smoke -mode auto
```

Live tests skip when credentials are missing. Supported credential env vars:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY` or `OPENAI_KEY`
- `OPENROUTER_API_KEY` or `OPENROUTER_KEY`
- `MINIMAX_API_KEY` or `MINIMAX_KEY`
- `CLAUDE_ACCESS_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`, or local Claude Code OAuth credentials in `~/.claude/.credentials.json`; set `CLAUDE_CONFIG_DIR` to override the local Claude config directory.
- `CODEX_ACCESS_TOKEN`, `CODEX_CODE_OAUTH_TOKEN`, or local Codex OAuth credentials in `~/.codex/auth.json`; set `CODEX_AUTH_PATH` to override the local auth file.

Provider model override env vars:

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

## CLI And Gateway

The main CLI command is `cmd/llmadapter`.

Initial commands:

- `llmadapter providers` lists registered provider endpoint types, API kinds, families, model env vars, and default smoke models.
- `llmadapter providers --auto` shows redacted auto-detected provider credential status; `--status --config <path>` does the same for a config file.
- `llmadapter routes` lists configured or auto-detected source API to provider endpoint routes; pass `--config` to inspect a JSON config instead of auto-detected credentials.
- `llmadapter models` lists public/native model mappings from configured or auto-detected routes; pass `--query` to filter. Add `--catalog` to inspect the modeldb catalog using built-in metadata or `modeldb.catalog_path`/`overlay_paths` from `--config`.
- `llmadapter resolve <model>` explains which source API route, provider endpoint, API family, native model, modeldb service, and capabilities will be used.
- `llmadapter serve` runs the compatibility gateway on `/v1/chat/completions`, `/v1/responses`, and `/v1/messages`; pass `--config`, `--addr`, or `--inspect-config`.
- `llmadapter smoke` runs a minimal direct text request through a configured provider endpoint type; `-mode mux` runs the same request through the stateless mux client route path, `-config` builds that route path from a llmadapter JSON config, and `-mode auto` builds a mux client from detected environment/local Claude credentials.

`cmd/llmadapter-gateway` remains as a compatibility binary and now runs through the same shared gateway server path as `llmadapter serve`.

The gateway command is `cmd/llmadapter-gateway`.

Configuration:

- `LLMADAPTER_CONFIG` points to a JSON config file.
- `LLMADAPTER_ADDR` sets the listen address when no config file is used.
- `llmadapter-gateway -inspect-config` prints resolved provider/route metadata as JSON and exits before constructing provider clients or requiring API keys.
- Routes select a provider endpoint with `provider` and optional `provider_api`.
- Routes can set `weight`; providers can set `priority`. Compatible routes are ranked by weight first, then endpoint priority, with declaration order as the final tie-breaker.
- Routes can set `dynamic_models: true` with no fixed `model` or `native_model` to pass arbitrary requested model IDs through to the selected provider endpoint. Use lower weight than fixed routes when mixing deterministic aliases with full provider/catalog access.
- `health_cooldown` is an optional Go duration string such as `30s`; recently failed provider endpoint/model pairs are deprioritized for that window.
- `modeldb.catalog_path` optionally replaces the built-in modeldb catalog with a JSON catalog file.
- `modeldb.overlay_paths` optionally merges one or more JSON catalog files after the base catalog.
- `modeldb.aliases` can bind local intent names to explicit service/wire-model pairs; route `modeldb_model` resolves a catalog alias/name into a fixed native model for that route.
- Provider `capabilities` can override default endpoint metadata for a configured model, for example to disable `vision` or `json_schema` on a model that does not support it.
- Provider `modeldb_service_id` plus a fixed route `native_model` or `modeldb_wire_model_id` enables modeldb-backed usage cost enrichment and endpoint capability/limit metadata for that route. Dynamic routes with `dynamic_models: true` can also enrich usage costs per request when the requested provider-native model ID exists in the catalog.
- `claude_messages` defaults `modeldb_service_id` to `anthropic` because it invokes Anthropic Claude models through Claude Code-compatible auth.
- The gateway exposes `/v1/chat/completions`, `/v1/responses`, and `/v1/messages`.

Docker:

```sh
docker build -t llmadapter:local .
docker run --rm -p 8080:8080 -e ANTHROPIC_API_KEY llmadapter:local
docker run --rm -p 8080:8080 -v "$PWD/llmadapter.json:/etc/llmadapter/config.json:ro" -e LLMADAPTER_CONFIG=/etc/llmadapter/config.json llmadapter:local
```

Example provider endpoint types:

- `anthropic`
- `claude_messages`
- `openai_chat`
- `openai_responses`
- `codex_responses`
- `openrouter_chat`
- `openrouter_responses`
- `openrouter_messages`
- `minimax_chat`
- `minimax_messages`

Example config:

```json
{
  "addr": ":8080",
  "health_cooldown": "30s",
  "modeldb": {
    "overlay_paths": ["./local-modeldb.json"],
    "aliases": [
      {
        "name": "fast",
        "service_id": "openrouter",
        "wire_model_id": "openai/gpt-4.1-mini"
      }
    ]
  },
  "providers": [
    {
      "name": "openrouter",
      "type": "openrouter_chat",
      "api_key_env": "OPENROUTER_API_KEY",
      "model": "openai/gpt-4.1-mini",
      "modeldb_service_id": "openrouter",
      "priority": 10,
      "capabilities": {
        "streaming": true,
        "tools": true,
        "vision": false,
        "json_mode": true,
        "json_schema": true
      }
    },
    {
      "name": "anthropic",
      "type": "anthropic",
      "api_key_env": "ANTHROPIC_API_KEY",
      "model": "claude-haiku-4-5-20251001"
    },
    {
      "name": "claude",
      "type": "claude_messages",
      "model": "claude-haiku-4-5-20251001"
    }
  ],
  "routes": [
    {
      "source_api": "openai.chat_completions",
      "model": "public-fast",
      "provider": "openrouter",
      "provider_api": "openrouter.chat_completions",
      "modeldb_model": "fast",
      "weight": 100
    },
    {
      "source_api": "openai.chat_completions",
      "model": "public-fast",
      "provider": "anthropic",
      "native_model": "claude-haiku-4-5-20251001",
      "weight": 10
    },
    {
      "source_api": "openai.responses",
      "provider": "openrouter",
      "provider_api": "openrouter.responses",
      "dynamic_models": true,
      "weight": 1
    }
  ]
}
```

## Design Notes

Provider routing is intentionally endpoint-based:

```text
Provider = who we talk to.
API kind = exact upstream wire protocol.
API family = compatibility family.
Provider endpoint = provider + API kind + family + client + capabilities.
```

Routes skip provider endpoints that cannot satisfy required request capabilities such as streaming, tools, JSON mode, JSON schema, reasoning, vision, or audio input, then rank the remaining candidates by configured weight and endpoint priority. If the selected provider fails before response bytes are written, the gateway retries lower-ranked route candidates.

The gateway command shares an in-memory health tracker across endpoints and temporarily deprioritizes provider endpoints that fail before response completion.

OpenAI Chat-compatible `response_format` requests and OpenAI Responses `text.format` requests are mapped into canonical JSON mode / JSON schema settings. Those settings are encoded back for OpenAI Chat, OpenRouter Chat, and OpenRouter Responses providers.

OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs decode supported image inputs into canonical `unified.ImagePart` values. OpenAI Chat-compatible providers, OpenRouter Responses, and Anthropic-compatible providers can encode supported image inputs upstream; gateway vision routing remains capability-gated by provider metadata.

Best-effort endpoint codecs retain decode/lossiness warnings on `adapt.Request`, and provider mappings emit canonical warning events for common unsupported dropped fields. Collected responses expose provider-side warnings under `Response.Warnings`.

Malformed tool-call argument JSON from OpenAI Chat and OpenAI Responses inputs is replaced with `{}` and recorded as a decode warning.

OpenRouter-specific request controls are carried through `unified.Request.Extensions` using namespaced keys such as `openrouter.provider`, `openrouter.plugins`, `openrouter.debug`, `openrouter.trace`, and `openrouter.session_id`. The OpenRouter Chat, Responses, and Messages providers encode those extensions back into upstream request bodies.

OpenAI Responses-compatible continuation and cache-key controls are also carried through extensions. `openai.responses.previous_response_id`, `openai.responses.store`, `openai.responses.prompt_cache_key`, and `openai.responses.prompt_cache_retention` are decoded by the `/v1/responses` endpoint and encoded by OpenAI/OpenRouter Responses providers without adding gateway/session state.

Conversation/session state belongs above llmadapter, for example in `agentsdk`. llmadapter only exposes stateless request/event/provider primitives needed by those layers.

The in-process mux client is a stateless library layer over provider endpoints and router selection. `adapterconfig.NewMuxClient` can build it from llmadapter JSON config, including modeldb-backed model alias resolution, capability metadata, and pricing processors, without requiring an HTTP gateway process.

`adapterconfig.AutoMuxClient` can build the same stateless mux client from detected credentials. It checks registered provider endpoint env vars such as `OPENAI_API_KEY`/`OPENAI_KEY`, Anthropic/OpenRouter/MiniMax keys, Claude bearer token env vars, and local Claude Code OAuth credentials when enabled. With `UseModelDB`, detected providers are tagged with their default modeldb service IDs so fixed-route capability metadata and fixed or dynamic-route pricing enrichment can work without hand-written provider config. Auto intents also use modeldb aliases to choose a provider that can resolve the requested model name, for example routing `opus` to a Claude-compatible endpoint when available.

Usage events use structured token/cost items as the canonical accounting surface. Endpoint codecs derive flat API-specific usage counters from token categories such as `input.new`, `input.cache_read`, `input.cache_write`, `output`, and `output.reasoning` where upstream usage details are available.

Canonical `unified.TextPart` values can carry `CacheControl` hints. Anthropic-family providers encode those hints as block-level `cache_control`, and the shared prompt-cache smoke verifies provider-reported cache write/read accounting for Anthropic and Claude Code-compatible access.

The `modelmeta` package maps modeldb offering exposures into route capabilities and limits. The gateway uses metadata only for configured fixed-model routes; modeldb can narrow advertised capabilities and add token limits, but it never creates hidden providers, clients, or routes.

The `pricing` package can enrich `unified.UsageEvent` values with `CostItems` using `modeldb` offering pricing for an explicit service/model pair. Pricing enrichment is an optional event processor and is not hardcoded into provider codecs. The gateway wires this processor for configured fixed routes with explicit `modeldb_service_id` and for dynamic model routes using the request's selected native model.

Codex is modeled as provider endpoint type `codex_responses` with API kind `codex.responses` and family `openai.responses`. It uses Codex/ChatGPT OAuth credentials and `https://chatgpt.com/backend-api/codex/responses`, not the normal OpenAI platform API URL. Its default modeldb service ID is `codex`.

The `claude_messages` provider type is an Anthropic Messages-compatible endpoint variant for Claude Code-style access. It uses bearer/OAuth auth instead of `x-api-key`, adds Claude CLI compatibility headers and `beta=true`, reads local Claude OAuth credentials when no bearer token env var is configured, applies Claude Code request preflight system blocks with cache control, and maps canonical reasoning requests to Anthropic extended thinking.

The default HTTP byte-stream transport advertises and decodes `gzip`, `deflate`, `br`, and `zstd` response compression. Custom HTTP clients can preserve that behavior by starting from `transport.CloneDefaultHTTPClient()`.

See `DESIGN.md` for the target architecture and `PLAN.md` for current status, known gaps, and next implementation phases.

## Known Limitations

- Capability defaults are endpoint-family guesses. Use provider `capabilities` overrides for model-specific support before routing production traffic.
- Gateway fallback only retries before response bytes are written. Mid-stream provider failures are marked unhealthy but cannot be converted into a fresh endpoint-shaped response.
- OpenRouter extension passthrough preserves raw JSON controls but does not yet validate provider-specific extension schemas.
- Native OpenAI Responses has live smoke coverage for `previous_response_id` continuation. OpenRouter Responses encodes the same fields, but live context preservation is not advertised because the current backend smoke did not preserve prior-turn context.
- Modeldb-backed metadata only narrows configured fixed-model routes. Pricing works for fixed routes and dynamic routes when the selected native model has catalog pricing; dynamic per-request capability lookup is still planned.
- Prompt cache request hints currently map to Anthropic-family block-level cache controls and OpenAI Responses-compatible cache-key extensions; higher-level session cache policy belongs above llmadapter.
- Provider and endpoint codecs cover smoke-tested text, tools, structured output, and basic image inputs; they are not full conformance implementations for every provider field.
