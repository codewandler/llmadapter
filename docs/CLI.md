# CLI

The `llmadapter` CLI is the fastest way to inspect providers, debug model routing, run direct inference, start the gateway, and smoke-test provider endpoints.

Run commands through source:

```sh
go run ./cmd/llmadapter <command>
```

Or build a binary:

```sh
go build -o llmadapter ./cmd/llmadapter
./llmadapter <command>
```

## Command Overview

| Command | Purpose |
| --- | --- |
| `providers` | List provider endpoint types or credential status. |
| `routes` | List configured or auto-detected routes. |
| `models` | List route models or modeldb catalog models. |
| `resolve` | Explain how a model routes to a provider endpoint. |
| `compatibility` | Evaluate route candidates for a workload use case. |
| `compatibility-record` | Refresh generated compatibility docs from an artifact. |
| `conformance` | Report provider descriptors plus compatibility evidence. |
| `infer` | Send a prompt through the mux client and stream output. |
| `proxy` | Inspect provider HTTP headers and streamed messages. |
| `serve` | Run the HTTP compatibility gateway. |
| `smoke` | Run minimal direct, mux, config, or auto provider smoke calls. |

## providers

List registered provider endpoint types:

```sh
go run ./cmd/llmadapter providers
```

Show auto-detected provider status:

```sh
go run ./cmd/llmadapter providers --auto
```

Show configured provider status:

```sh
go run ./cmd/llmadapter providers --status --config examples/llmadapter.example.json
```

Use JSON for automation:

```sh
go run ./cmd/llmadapter providers --json
```

Credential values are not printed.

## routes

List auto-detected routes:

```sh
go run ./cmd/llmadapter routes
```

List routes from config:

```sh
go run ./cmd/llmadapter routes --config examples/llmadapter.example.json
```

Filter by source API:

```sh
go run ./cmd/llmadapter routes --source-api openai.responses
```

Route output includes the configured provider endpoint plus `CONSUMER_CONTINUATION`, `INTERNAL_CONTINUATION`, and `TRANSPORT`. Consumers should use only `CONSUMER_CONTINUATION` to decide whether they must replay history or may send native continuation IDs. `INTERNAL_CONTINUATION` and `TRANSPORT` are diagnostics.

## models

List configured route models:

```sh
go run ./cmd/llmadapter models --config examples/llmadapter.example.json
```

Query the modeldb catalog:

```sh
go run ./cmd/llmadapter models --catalog --service openai --query gpt
go run ./cmd/llmadapter models --catalog --service anthropic --query claude
```

Expand catalog offerings:

```sh
go run ./cmd/llmadapter models --catalog --offerings --service openrouter --query claude
```

## resolve

Explain the selected route:

```sh
go run ./cmd/llmadapter resolve haiku
```

Resolve from a config:

```sh
go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json fast
```

Pin the incoming API shape:

```sh
go run ./cmd/llmadapter resolve --source-api anthropic.messages haiku
go run ./cmd/llmadapter resolve --source-api openai.responses openai/gpt-5.5
```

Important output fields:

- `Provider`: configured provider instance.
- `Provider type`: implementation type.
- `Provider API`: exact upstream API kind.
- `Family`: compatibility family.
- `ModelDB svc`: modeldb service identity for metadata/pricing.
- `Capability source`: `provider_descriptor`, `config_override`, or `modeldb_exposure`.
- `Continuation`: public consumer strategy plus diagnostic provider-internal strategy and current transport class.

Use JSON for automation:

```sh
go run ./cmd/llmadapter resolve haiku --json
```

Annotate route candidates for a workload:

```sh
go run ./cmd/llmadapter resolve haiku --use-case agentic_coding
```

Return only candidates approved by live compatibility evidence:

```sh
go run ./cmd/llmadapter resolve haiku --use-case agentic_coding --approved-only
```

Use an explicit evidence artifact:

```sh
go run ./cmd/llmadapter resolve haiku --use-case agentic_coding --approved-only --compatibility-evidence docs/compatibility/agentic_coding.json
```

## compatibility

Evaluate whether configured or auto-detected route candidates satisfy a workload profile:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model haiku
```

Use a config:

```sh
go run ./cmd/llmadapter compatibility --config examples/llmadapter.example.json --model fast
```

Use JSON for downstream tools:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model haiku --json
```

Initial use cases:

- `agentic_coding`: requires streaming text, tools, tool continuation, structured output, reasoning, prompt caching, usage, and cache accounting; prefers pricing.
- `summarization`: requires streaming text and usage; tools, reasoning, prompt caching, cache accounting, pricing, and gateway support are optional.

Compatibility output is offline inspection. It uses provider descriptors, config/modeldb capability provenance, and existing route resolution. `resolve --approved-only` joins that route resolution with modeldb runtime views and the live workload-specific evidence artifact.

## compatibility-record

Refresh generated documentation from a compatibility artifact:

```sh
go run ./cmd/llmadapter compatibility-record --use-case agentic_coding
```

The command reads `docs/compatibility/agentic_coding.json` by default and rewrites the generated section in `docs/USE_CASE_MATRIX.md`. Use `--artifact`, `--matrix`, or `--command` to override those inputs.

## conformance

Inspect provider endpoint descriptors together with endpoint evidence and live use-case approval rows:

```sh
go run ./cmd/llmadapter conformance
```

Use JSON for automation:

```sh
go run ./cmd/llmadapter conformance --json
```

The default report joins the provider registry with `docs/compatibility/agentic_coding.json`. Use `--compatibility-artifact` to point at another recorded artifact.

For `agentic_coding`, every approved row is validated as a strict workload contract. The row must have `required_status=passed`, live evidence for text, tools, tool continuation, structured output, reasoning, prompt caching, usage, and cache accounting, plus explicit consumer continuation, internal continuation, and transport evidence. The text report shows `AGENTIC_APPROVED`, `AGENTIC_VALID`, and `AGENTIC_CONTRACT`; the command exits non-zero if an approved row violates that contract.

## infer

Run one prompt:

```sh
go run ./cmd/llmadapter infer "what is 2+2?"
```

Choose a model:

```sh
go run ./cmd/llmadapter infer -m haiku "summarize this project"
go run ./cmd/llmadapter infer -m openai/gpt-5.5 "write a haiku"
```

Use reasoning controls:

```sh
go run ./cmd/llmadapter infer -m sonnet --thinking on --effort high "explain channels"
```

Disable cache policy for a request:

```sh
go run ./cmd/llmadapter infer -m haiku --no-cache "short answer only"
```

Use continuation diagnostics:

```sh
go run ./cmd/llmadapter infer -m codex/gpt-5.4 --session demo --branch main "continue the session"
go run ./cmd/llmadapter infer -m codex/gpt-5.4 --interaction one_shot "single request"
```

Use a config:

```sh
go run ./cmd/llmadapter infer --config examples/llmadapter.example.json -m fast "what is 2+2?"
```

`infer` prints resolved model/route information before streaming output, including continuation mode and transport. By default it uses `--interaction one_shot`; setting `--session` without `--interaction` switches to `session` and also sets a stable cache/session key. For `codex_responses`, session mode with a stable session/cache key can prefer the provider-internal WebSocket transport and fall back to HTTP/SSE before streaming starts. Use `--branch` when testing branch-specific continuation behavior. The final route section reports actual provider execution metadata when the provider emits it. `--no-cache` disables cache policy but still preserves explicit session diagnostics.

## proxy

Run a local reverse proxy and inspect redacted request/response headers plus JSON or SSE/NDJSON body content:

```sh
go run ./cmd/llmadapter proxy --bind 127.0.0.1:8089 --upstream https://api.anthropic.com
```

Analyze Claude CLI traffic by starting the proxy on a random local port, setting Claude/Anthropic base-url environment variables for the child process, and forwarding all arguments after `--` to `claude`:

```sh
go run ./cmd/llmadapter proxy --analyze claude -- --print "reply ok"
```

Use `--command` when the Claude executable has a different name or path:

```sh
go run ./cmd/llmadapter proxy --analyze claude --command /path/to/claude -- --print "reply ok"
```

The proxy writes diagnostics to stderr and preserves the child process stdio. Sensitive headers and JSON fields such as authorization, API keys, cookies, session IDs, and account IDs are redacted. To keep stream logs readable, the proxy removes outbound `Accept-Encoding` before forwarding so Go's HTTP transport can receive and forward decompressed response bodies.

## serve

Run the gateway from auto-detected env/local credentials:

```sh
go run ./cmd/llmadapter serve
```

Run the gateway from config:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json
```

Set address:

```sh
go run ./cmd/llmadapter serve --addr :9090
```

Inspect config and exit:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
```

The compatibility binary is still available:

```sh
go run ./cmd/llmadapter-gateway -inspect-config
```

`cmd/llmadapter-gateway` is a compatibility entry point over the same `gatewayserver` path as `llmadapter serve`.

## smoke

Run a direct provider endpoint smoke:

```sh
go run ./cmd/llmadapter smoke -type openai_responses
```

Run through mux routing:

```sh
go run ./cmd/llmadapter smoke -mode mux -type openai_responses
```

Run through a config:

```sh
go run ./cmd/llmadapter smoke -mode mux -config examples/llmadapter.example.json -model fast
```

Run through auto detection:

```sh
go run ./cmd/llmadapter smoke -mode auto
```

## Environment Variables

Provider credentials:

- `ANTHROPIC_API_KEY`
- `OPENAI_API_KEY` or `OPENAI_KEY`
- `OPENROUTER_API_KEY` or `OPENROUTER_KEY`
- `MINIMAX_API_KEY` or `MINIMAX_KEY`
- `CODEX_ACCESS_TOKEN` or `CODEX_CODE_OAUTH_TOKEN`

Local credential paths:

- `CLAUDE_CONFIG_DIR`
- `CODEX_AUTH_PATH`

Gateway:

- `LLMADAPTER_CONFIG`
- `LLMADAPTER_ADDR`

Model overrides:

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
