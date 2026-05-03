# Provider Matrix

This matrix is the v1 supported provider endpoint surface. It describes what llmadapter routes and smoke-tests today; it is not a promise that every upstream provider-specific field is implemented.

This is endpoint evidence, not workload approval. For agentic-coding suitability, see `docs/USE_CASE_MATRIX.md`.

For a machine-readable join of provider descriptors, endpoint evidence, warnings, and approved use-case rows, run:

```sh
go run ./cmd/llmadapter conformance --json
```

Legend:

- `live`: covered by `TEST_INTEGRATION=1` smoke tests when credentials are available.
- `fixture`: covered by deterministic offline conformance fixtures.
- `mapped`: encoded/decoded by provider or endpoint mapping, but not asserted by a live provider accounting check.
- `n/a`: not part of that provider endpoint family.

## Endpoints

| Provider endpoint | API kind | Family | Credentials | Default smoke model |
| --- | --- | --- | --- | --- |
| `anthropic` | `anthropic.messages` | `anthropic.messages` | `ANTHROPIC_API_KEY` | `claude-haiku-4-5-20251001` |
| `claude` | `anthropic.messages` | `anthropic.messages` | `~/.claude/.credentials.json` or `CLAUDE_CONFIG_DIR` | `claude-haiku-4-5-20251001` |
| `openai_chat` | `openai.chat_completions` | `openai.chat_completions` | `OPENAI_API_KEY` or `OPENAI_KEY` | `gpt-4.1-mini` |
| `openai_responses` | `openai.responses` | `openai.responses` | `OPENAI_API_KEY` or `OPENAI_KEY` | `gpt-4.1-mini` |
| `codex_responses` | `codex.responses` | `openai.responses` | `CODEX_ACCESS_TOKEN`, `CODEX_CODE_OAUTH_TOKEN`, or `~/.codex/auth.json` | provider default |
| `bedrock_responses` | `bedrock.responses` | `openai.responses` | `BEDROCK_API_KEY` or `AWS_BEARER_TOKEN_BEDROCK` | `openai.gpt-oss-120b` |
| `bedrock_messages` | `bedrock.anthropic_messages` | `anthropic.messages` | `BEDROCK_API_KEY` or `AWS_BEARER_TOKEN_BEDROCK` | `anthropic.claude-opus-4-7` |
| `bedrock_converse` | `bedrock.converse` | `bedrock.converse` | AWS SDK default credential chain | `anthropic.claude-sonnet-4-6` |
| `openrouter_chat` | `openrouter.chat_completions` | `openai.chat_completions` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `openai/gpt-4.1-mini` |
| `openrouter_responses` | `openrouter.responses` | `openai.responses` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `openai/gpt-4.1-mini` |
| `openrouter_messages` | `openrouter.anthropic_messages` | `anthropic.messages` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `anthropic/claude-sonnet-4` |
| `minimax_chat` | `minimax.chat_completions` | `openai.chat_completions` | `MINIMAX_API_KEY` or `MINIMAX_KEY` | `MiniMax-M2.7` |
| `minimax_messages` | `minimax.anthropic_messages` | `anthropic.messages` | `MINIMAX_API_KEY` or `MINIMAX_KEY` | `MiniMax-M2.7` |

## Continuation And Transport

| Provider endpoint | Consumer continuation | Internal continuation | Transport |
| --- | --- | --- | --- |
| `anthropic` | `replay` | `replay` | `http_sse` |
| `claude` | `replay` | `replay` | `http_sse` |
| `openai_chat` | `replay` | `replay` | `http_sse` |
| `openai_responses` | `previous_response_id` | `previous_response_id` | `http_sse` |
| `codex_responses` | `replay` | `replay` | `http_sse` |
| `bedrock_responses` | `previous_response_id` | `previous_response_id` | `http_sse` |
| `bedrock_messages` | `replay` | `replay` | `http_sse` |
| `bedrock_converse` | `replay` | `replay` | `http_sse` |
| `openrouter_chat` | `replay` | `replay` | `http_sse` |
| `openrouter_responses` | `replay` | `replay` | `http_sse` |
| `openrouter_messages` | `replay` | `replay` | `http_sse` |
| `minimax_chat` | `replay` | `replay` | `http_sse` |
| `minimax_messages` | `replay` | `replay` | `http_sse` |

Consumers should choose their public projection strategy from `Consumer continuation`, not from provider name, API family, `Internal continuation`, or `Transport`. For example, Codex Responses is OpenAI Responses-family but requires replay at the public boundary; WebSocket and internal `previous_response_id` reuse are provider optimizations, not caller contracts.

## Feature Coverage

| Provider endpoint | Text | Tools | Tool continuation | Parallel tools | Reasoning | Prompt cache accounting | Structured output | Vision | Usage | Pricing | Gateway |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `anthropic` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |
| `claude` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |
| `openai_chat` | live | live | live | live | n/a | n/a | fixture | fixture | live | modeldb | live |
| `openai_responses` | live | live | live | live | live | live | fixture | fixture | live | modeldb | live |
| `codex_responses` | live | live | live | live | live | live | fixture | fixture | live | modeldb | live |
| `bedrock_responses` | live | live | live | n/a | live | n/a | fixture | n/a | live | modeldb | live |
| `bedrock_messages` | live | live | live | n/a | n/a | n/a | n/a | fixture | live | modeldb | live |
| `bedrock_converse` | live | live | live | n/a | live | live | n/a | n/a | live | modeldb | live |
| `openrouter_chat` | live | live | live | live | n/a | n/a | fixture | fixture | live | modeldb | live |
| `openrouter_responses` | live | live | live | live | live | live | fixture | fixture | live | modeldb | live |
| `openrouter_messages` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |
| `minimax_chat` | live | live | live | n/a | n/a | n/a | fixture | fixture | live | modeldb | live |
| `minimax_messages` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |

Prompt cache accounting means the live smoke test checks provider-reported cache write/read token counters. `mapped` means llmadapter maps the cache controls onto the provider wire shape, but the v1 smoke matrix does not assert provider-reported cache accounting for that endpoint.

Codex Responses uses public replay semantics. One-shot requests use the Codex HTTP/SSE backend. Session-mode requests with an explicit session ID may prefer the Codex WebSocket transport, keep the WebSocket open for backend affinity, and use internal `previous_response_id` after lineage checks pass. Lineage requires exact canonical input-prefix matching, not just input length. HTTP/SSE fallback can happen before user-visible output starts; after output starts, a lost WebSocket fails the current turn and invalidates internal continuation state so the next request replays. `previous_response_id` is still not a public caller contract for `codex_responses`.

Codex prompt-cache accounting is verified in two ways: the shared prompt-cache smoke checks provider-reported cache counters, and `TestSmokeCodexWebSocketPromptCache` specifically requires WebSocket transport plus cache-read accounting for a repeated cached request.

OpenAI platform Responses has an official WebSocket mode. Direct `openai_responses` clients can opt into it with `responses.WithWebSocketMode(...)`; this matrix still marks `openai_responses` as HTTP/SSE because provider descriptors, JSON config, auto mux, and the live workload matrix default to HTTP/SSE unless an explicit direct-client option is used. `openai_chat` remains HTTP/SSE. The OpenAI Realtime API is a separate WebSocket/WebRTC surface and is not represented in this matrix yet.

The default OpenAI Responses WebSocket transport enables compression and forces IPv4. OpenRouter Responses does not inherit that mode unless it opts in explicitly.

OpenAI Responses owns the base provider wire implementation for the Responses family. OpenRouter Responses wraps that base with OpenRouter-specific request extensions; Codex Responses wraps it with Codex auth/session/header behavior and Codex-specific unsupported-field handling.

Bedrock Responses uses Amazon Bedrock Mantle's OpenAI-compatible `/v1/responses` endpoint. The default base URL is derived from `AWS_REGION` or `AWS_DEFAULT_REGION`, falling back to `us-east-1`; config `base_url` may also be set explicitly, with or without a trailing `/v1`. Authentication can use explicit Bedrock bearer keys or AWS SDK-loaded profile/region credentials to generate short-term Bedrock bearer tokens. Native Bedrock Converse and Bedrock Chat Completions are separate endpoint shapes and are not represented by `bedrock_responses`; Anthropic-compatible Bedrock access is represented by `bedrock_messages`.

Bedrock Mantle's `/v1/models` list is broader than the `/v1/responses` surface. On 2026-05-03 in `us-east-1`, the listed models that live-probed successfully through `/v1/responses` were `openai.gpt-oss-120b` and `openai.gpt-oss-20b`; all other listed Mantle IDs returned Bedrock validation errors stating that the model does not support `/v1/responses`.

Bedrock Messages uses Mantle's Anthropic-compatible route at `/anthropic/v1/messages`. On 2026-05-03 in `us-east-1`, `anthropic.claude-opus-4-7` and `anthropic.claude-haiku-4-5` live-probed successfully through this endpoint; `anthropic.claude-opus-4-6` returned a provider not-found error for that Mantle model ID. A separate `/anthropic/v1/models` listing is not exposed, so the broad Mantle `/v1/models` endpoint remains the discovery source even though it mixes API surfaces. The shared smoke matrix uses Haiku 4.5 for tool-continuation because Opus 4.7 intentionally refuses the synthetic prompt-injection-shaped tool result used by the generic continuation smoke. Bedrock Messages accepts adaptive-thinking-shaped requests, but the live stream did not expose reasoning deltas or reasoning token accounting, so this endpoint does not advertise reasoning yet.

Bedrock Converse uses the native AWS SDK Bedrock Runtime `ConverseStream` API and AWS SDK credentials rather than Mantle bearer tokens. It targets replay-based continuation, text, tools, prompt cache points, and Claude extended-thinking request fields. Modeldb-backed fixed and dynamic routes prefer `RuntimeAccess.ResolvedWireID` for region-specific inference profiles and fall back to provider-local prefixing only when runtime metadata is unavailable. On 2026-05-03 in `us-east-1`, the shared text, tool-use, tool-result continuation, reasoning, OpenAI Chat gateway, and Anthropic Messages gateway smokes passed with the default Sonnet 4.6 model; the agentic-coding compatibility run also passed cache-accounting checks for Haiku 4.5, Sonnet 4.6, Opus 4.6, and Opus 4.7 through Bedrock Converse.

## Live Smoke Commands

Full available matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```

Focused slices:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolUse -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolResultContinuation -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeReasoningStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokePromptCache -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeCodexWebSocketContinuation -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeCodexWebSocketPromptCache -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeOpenAIResponsesWebSocket -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestGatewaySmoke -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestAnthropicMessagesGatewaySmoke -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestResponsesGatewaySmoke -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestBedrockResponsesModelsList -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 BEDROCK_RESPONSES_PROBE_MODELS=openai.gpt-oss-120b,openai.gpt-oss-20b go test ./tests/e2e -run TestBedrockResponsesProbeModels -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 BEDROCK_MESSAGES_PROBE_MODELS=anthropic.claude-opus-4-7,anthropic.claude-haiku-4-5 go test ./tests/e2e -run TestBedrockMessagesProbeModels -count=1 -v
```

The e2e package skips cleanly when `TEST_INTEGRATION` is unset. Individual provider subtests skip when their credential env vars or local OAuth files are unavailable, or when a feature is not advertised by that endpoint in the smoke matrix.

## Latest V1 Track Result

On 2026-05-02, the full live command above was run with local credentials available for all v1 provider endpoints. Text, tools, tool continuation, and gateway routing passed for Anthropic, Claude-compatible access, OpenAI Chat, OpenAI Responses, Codex Responses, OpenRouter Chat, OpenRouter Responses, OpenRouter Messages, MiniMax Chat, and MiniMax Messages.

On 2026-04-27, the focused Codex WebSocket continuation and WebSocket prompt-cache smokes passed with `TEST_INTEGRATION=1`, including runtime `transport=websocket` evidence and provider-reported cache-read token accounting on the repeated cached request.

On 2026-04-27, `llmadapter conformance` was tightened so approved `agentic_coding` rows must also carry live evidence for every required workload feature and explicit continuation/transport evidence. The bundled `docs/compatibility/agentic_coding.json` passes that stricter contract.
