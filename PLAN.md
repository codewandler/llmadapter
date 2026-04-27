# llmadapter Implementation Plan

Implementation history and release-candidate roadmap for stabilizing the implemented `DESIGN.md` foundation.

Primary goal: keep the adapter buildable and incrementally useful while hardening provider routing, endpoint compatibility, gateway behavior, live smoke coverage, and documentation.

---

## Current Status

Status date: 2026-04-27.

`v1.0.0-rc.10` has been cut. The current remaining v1 work is release-candidate validation and final v1.0.0 promotion if no regressions or documentation inaccuracies are found.

Highest-priority post-rc work:

```text
Use-case compatibility approval for agentic coding.

The provider endpoint matrix answers what endpoint implementations support.
The new use-case compatibility layer must answer which provider/model/API-kind combinations are approved for workloads such as agentic coding.

Concrete execution plan: docs/USE_CASE_COMPATIBILITY_PLAN.md
```

Completed:

```text
Phase 1: unified, adapt, and pipeline core packages
Phase 2: transport package with SSE, NDJSON, HTTP, fake, retry, and rate-limit wrappers
Phase 3: Anthropic Messages programmatic client path
Phase 4 first slice: /v1/chat/completions endpoint codec and minimal gateway handler
Phase 4 gateway e2e slice: runnable Anthropic-backed gateway command and live gateway smoke tests
Phase 6 first slice: static router with endpoint/model matching and native model rewrite
Gateway config slice: optional JSON config for providers and static routes
Provider support slice: OpenAI Chat Completions upstream provider
Gateway provider matrix slice: OpenAI-backed route covered by live gateway smoke tests
Tool-use provider slice: OpenAI streamed tool calls and shared live tool-use smoke tests
Tool loop e2e slice: shared live tool-result continuation smoke tests
OpenRouter provider slice: native Chat Completions provider wrapper and shared smoke matrix entry
Provider endpoint routing slice: routes carry target API kind, API family, provider name, and capabilities
OpenRouter multi-endpoint slice: native Responses text streaming and Anthropic-compatible Messages support
OpenRouter Responses tool slice: native Responses function-call streaming and tool-result continuation support
Documentation slice: minimal README, AGENTS, and provider-extension agent skill
MiniMax provider slice: OpenAI-compatible Chat Completions wrapper, gateway registration, and shared text smoke matrix entry
MiniMax Messages slice: Anthropic-compatible Messages wrapper, gateway registration, and shared text/tool smoke matrix entries
Conformance cleanup slice: OpenAI Chat gateway reasoning_details encoding and structured provider HTTP/mid-stream error tests
Endpoint slice: downstream Anthropic-compatible /v1/messages gateway codec and live Anthropic-family gateway smokes
Endpoint slice: downstream OpenAI-compatible /v1/responses gateway codec and live OpenRouter Responses gateway smokes
Routing hardening slice: static router now checks request capabilities before selecting a provider endpoint
Mapping warnings slice: Anthropic-family provider best-effort request mapping emits canonical warning events for dropped unsupported fields
Mapping warnings slice: OpenAI Chat, OpenRouter Chat, and OpenRouter Responses provider mappings emit canonical warning events for dropped non-text content and unsupported tool kinds
Endpoint decode warnings slice: OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages endpoint codecs retain warnings on adapt.Request for unsupported inbound fields dropped during canonical decode
OpenRouter extension passthrough slice: endpoint codecs preserve OpenRouter routing/provider/plugin/debug/trace/session controls in unified.Request.Extensions and OpenRouter Chat, Responses, and Messages providers encode those controls upstream
Weighted routing slice: static router ranks compatible candidates by route weight and endpoint priority while falling back past capability mismatches
Gateway fallback slice: gateway handlers retry lower-ranked route candidates when the selected provider fails before response bytes are written
OpenAI Chat structured-output slice: OpenAI Chat-compatible endpoint decode and provider encode preserve response_format JSON mode and JSON schema requests
OpenAI Responses structured-output slice: OpenAI Responses endpoint decode and OpenRouter Responses provider encode preserve text.format JSON mode and JSON schema requests
Endpoint image decode slice: OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs preserve supported image inputs as canonical ImagePart values
Provider image passthrough slice: OpenAI Chat-compatible providers, OpenRouter Responses, and Anthropic-compatible providers encode supported canonical image inputs upstream; gateway metadata advertises vision on those endpoint families
Tool argument hardening slice: OpenAI Chat and OpenAI Responses endpoint codecs replace malformed tool-call argument JSON with an empty object and retain decode warnings
Gateway health slice: gateway command shares an in-memory health tracker that temporarily deprioritizes failed provider endpoints during a cooldown window
Stabilization slice: gateway health cooldown is configurable, health deprioritization is keyed by provider/API/model, and provider capability metadata can be overridden per configured provider/model
Stabilization slice: OpenRouter raw extension preservation is centralized in unified helpers used by endpoint and provider codecs
Stabilization slice: Anthropic Messages endpoint decoding preserves mixed user text/tool-result content by splitting it into canonical user and tool messages
Structured usage slice: canonical usage now carries token and cost item breakdowns, with flat API counters derived only at endpoint wire boundaries
Pricing slice: `pricing` package adds a modeldb-backed event processor that enriches UsageEvent cost items for an explicit service/model offering
Gateway pricing slice: configured fixed-model routes can wrap provider clients with modeldb-backed usage cost enrichment via provider `modeldb_service_id` plus route `native_model` or `modeldb_wire_model_id`; dynamic model routes can price each request from the selected provider-native model ID
Model metadata slice: `modelmeta` maps modeldb offering exposures into route capability narrowing and model token limits for configured fixed-model gateway routes
Dynamic model metadata slice: `dynamic_models` routes resolve requested models through the loaded modeldb catalog plus overlays, rewrite to the selected offering wire model, narrow request capabilities from modeldb exposure metadata, and reject catalog-missing models instead of falling through to provider defaults
Operator inspection slice: `llmadapter-gateway -inspect-config` prints resolved providers, routes, capabilities, limits, modeldb metadata, and pricing availability without constructing provider clients
Provider registry slice: shared `providerregistry` package lists endpoint types and can construct direct provider clients for CLI/library use
Mux client slice: `muxclient` provides a stateless unified.Client over router/provider endpoints with native model rewrite, pre-stream fallback, and auto-source routing when no source API is preset
Route attempt slice: gateway and muxclient share route candidate lookup, native model rewrite, and provider/API error formatting through `internal/routeattempt` while gateway keeps HTTP response-start fallback policy local
Adapter config slice: `adapterconfig` exposes JSON config loading/defaulting/validation plus config-driven muxclient construction with modeldb alias resolution, capability metadata, and pricing processors
CLI slice: `cmd/llmadapter` is Cobra-based and can list provider endpoint types, inspect redacted provider credential status, inspect configured/auto-detected routes and models, explain route resolution, run the gateway server, and run minimal direct, manual mux-routed, config-driven mux, or auto-detected mux text smoke requests
CLI inference slice: `llmadapter infer <message>` sends a prompt through the shared config/auto mux path, prints resolved route/model metadata first, streams reasoning/text deltas, and prints usage/cost data when providers report it
Container slice: Dockerfile builds a standalone `llmadapter` image that runs `llmadapter serve`
Auto mux slice: `adapterconfig.AutoMuxClient` can construct a stateless mux client from detected env credentials and local Claude Code OAuth credentials, with default modeldb service tags when enabled and optional source API presetting
Auto mux modeldb intent slice: when `UseModelDB` is enabled, auto intents and aliases choose a provider whose catalog service can resolve the requested model alias; unresolved intents do not fall back to provider defaults. Service-qualified names such as `openai/gpt-5.5` and `codex/gpt-5.4` constrain resolution to that service before mapping to the offering wire model.
Model alias slice: `adapterconfig.DefaultModelDBAliases()` centralizes built-in provider-local aliases for Claude-family `haiku`, `sonnet`, and `opus`; auto mux callers can inject or override aliases through `AutoOptions.ModelDBAliases`
Modeldb catalog config slice: gateway config supports `modeldb.catalog_path` as an explicit catalog base and `modeldb.overlay_paths` for local operator overlays
Modeldb alias resolution slice: route `modeldb_model` resolves catalog aliases/names or local `modeldb.aliases` into explicit fixed native/modeldb wire model IDs
Claude compatibility slice: `claude` registers a Claude Code-compatible Anthropic Messages endpoint with OAuth/bearer auth, Claude CLI headers/query behavior, request preflight metadata, Anthropic modeldb service identity, and Anthropic extended-thinking request mapping
Codex compatibility slice: `codex_responses` registers a Codex/ChatGPT OAuth-backed Responses endpoint with API kind `codex.responses`, OpenAI Responses family routing, Codex-specific URL/header/body handling, local auth detection, and Codex modeldb service identity
Prompt cache slice: canonical `CachePolicy`, `CacheKey`, and `CacheTTL` primitives are available on unified requests; `TextPart.CacheControl` hints encode through Anthropic-family system/content blocks, including OpenRouter Messages and MiniMax Messages; Responses-style prompt cache keys are available for OpenAI/OpenRouter Responses and Codex Responses, with Codex mapping the key into session/window headers; live prompt-cache smoke verifies provider-reported cache write/read usage for Anthropic-family entries and Codex where credentials are available
OpenAI Responses provider slice: native OpenAI Responses provider endpoint is registered and live-verified for text, tools, gateway routing, and previous_response_id continuation
Conformance hardening slice: shared HTTP transport normalizes provider JSON error bodies plus Retry-After into `unified.APIError`; provider clients preserve that error through request setup, and modeldb-backed dynamic route resolution is regression-tested across router and CLI diagnostic/infer paths
Extension validation slice: typed extension readers validate mature OpenAI Responses, OpenRouter, Anthropic, and Codex extension groups; provider encoders emit `invalid_extension_dropped` warnings for invalid controls, and Codex rejects invalid session/window headers before transport
Extension semantic conformance slice: OpenAI Responses cache retention, OpenRouter model/session/plugin/provider controls, Anthropic beta header values, and Codex turn metadata now have focused semantic validation with provider encode/reject tests
Reasoning conformance slice: shared provider fixtures verify reasoning stream projection for Anthropic Messages, MiniMax Messages, OpenRouter Messages, OpenAI Responses, OpenRouter Responses, and Codex Responses; `unified.Collect` now preserves citation events and raw provider events alongside content, usage, warnings, and finish metadata
Error conformance slice: shared provider fixtures verify mid-stream API errors across Anthropic-family Messages, OpenAI Chat-compatible, OpenAI Responses-compatible, OpenRouter, MiniMax, and Codex paths; shared HTTP transport normalizes numeric error codes and MiniMax `base_resp` error bodies; gateway and mux tests pin that fallback only applies before response streaming starts
Multimodal conformance slice: shared provider fixtures verify supported image URL/base64 encoding and unsupported audio/file/video warning behavior across OpenAI Chat, OpenRouter Responses, and Anthropic Messages; Anthropic codec tests pin strict unsupported multimodal errors and best-effort warning/drop behavior
Architecture cleanup slice: `cmd/llmadapter-gateway` is now a thin compatibility binary over shared `adapterconfig` inspection/config validation and `gatewayserver` serving; duplicated command-local config, provider construction, modeldb, pricing, and inspection code was removed
Reasoning signature slice: canonical reasoning content/events now preserve provider signatures; Anthropic-family providers and endpoints decode/encode thinking signatures for Anthropic, Claude Code-compatible access, OpenRouter Messages, and MiniMax Messages continuations
Codex parity hardening slice: Codex Responses is included in the shared text, reasoning, tool, tool-continuation, prompt-cache, and `/v1/responses` gateway smoke matrices; prompt-cache smoke uses the Codex session/window cache key path and a larger stable prefix while allowing the warmup request to omit write counters
Codex extension validation slice: `unified.CodexExtensions` provides typed namespaced controls for Codex session/window/turn metadata headers, and the Codex transport validates and applies them without changing default cache-key behavior
Conformance fixture slice: focused tests cover Responses mid-stream provider errors and dynamic model resolver rejection for unavailable catalog models
Anthropic wire cleanup slice: shared Anthropic Messages wire structs live in neutral `anthropicwire`, while the provider package keeps type aliases for compatibility; downstream `/v1/messages` no longer imports upstream provider implementation types
Extension helper slice: typed `unified` helpers now cover OpenAI Responses continuation/cache controls, OpenRouter raw routing/provider/plugin/debug controls, Anthropic beta controls, and Codex session/window/turn controls
Parallel tool conformance slice: OpenAI Chat and Responses-family stream decoders have deterministic parallel tool-call fixtures; live e2e has opt-in parallel tool smoke coverage for providers advertising the capability
Registry cleanup slice: provider descriptors now carry static client factories, removing the growing central `NewClient` provider-type switch while preserving deterministic construction
Error conformance slice: live e2e coverage includes invalid API-key and invalid-model normalization checks for configured providers, asserting useful `unified.APIError` details
Provider error conformance slice: shared fixtures preserve provider raw payloads on mid-stream API errors, cover Responses response-object failures, and extend non-2xx parsing for top-level message, detail, and string-error provider body variants
Citation conformance slice: Responses-family annotation events and Anthropic-family text-block citations emit canonical CitationEvent values, preserving URL/title/text/ranges/document IDs and unknown citation metadata
Endpoint codec conformance slice: OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs preserve HTTP/raw decode metadata; OpenAI Responses and Anthropic Messages endpoints project canonical citations back into compatible response annotation/citation fields
Unsupported media/tool conformance slice: endpoint and provider fixtures pin current best-effort policy for unsupported audio/video/file/document content and built-in tools: supported images are preserved, unsupported fields warn/drop, and provider wire payloads do not leak unsupported media or built-in tool declarations
V1 phase 1 docs/status truth slice: README, PLAN, ARCHITECTURE, and CHANGELOG now describe the current implementation accurately, split v1 blockers from non-blockers and post-v1 expansion, and require changelog updates before every tag/release
V1 phase 2 conformance fixture slice: endpoint decode edge cases, additional reasoning event shapes, citation metadata variants, message-only stream errors, raw/unmapped events, and unsupported media/tool policy are covered by deterministic offline fixtures
V1 phase 3 routing/fallback policy slice: shared route-attempt policy classifies non-retryable request validation failures, gateway config supports `max_attempts`, muxclient exposes `WithMaxAttempts`, and gateway/mux tests pin retry-limit exhaustion plus response-start boundaries
V1 phase 4 capability/model policy slice: config inspection and model resolution expose capability provenance as provider descriptor defaults, config overrides, or modeldb exposure metadata; dynamic modeldb routes still reject catalog-missing models without provider-default substitution
V1 phase 5 CLI/config/examples slice: README usage examples cover auto mux, config mux, direct infer, gateway serve, Docker, model resolution, and provider identity inspection; `examples/llmadapter.example.json` plus a modeldb overlay are load-tested
V1 phase 6 provider matrix slice: `docs/PROVIDER_MATRIX.md` documents the v1 provider endpoints, feature coverage, credential triggers, live smoke commands, skip behavior, and latest full matrix result
V1 phase 7 public API freeze slice: primary public packages have package docs, `docs/API_SURFACE.md` records stable consumer/extension/internal boundaries, and no exported renames are required before v1.0.0 promotion
V1 phase 8 release-candidate slice: CHANGELOG documents the v1 stable surface, stale blocker wording is removed, non-secret CLI examples plus Docker build are verified, and v1.0.0-rc.1 is ready to cut
Use-case compatibility foundation slice: `compatibility` defines workload profiles and feature evidence, `adapterconfig` converts existing model-resolution candidates into compatibility candidates and exposes compatibility filtering helpers, `llmadapter compatibility` and `resolve --use-case` expose offline use-case inspection, and docs now split endpoint evidence from workload suitability
Use-case compatibility live slice: `tests/e2e/TestUseCaseAgenticCoding` verifies promoted OpenAI, Codex, OpenRouter, Claude, Anthropic, MiniMax, and Moonshot Kimi through OpenRouter rows for streaming text, tools, tool continuation, structured output, reasoning, prompt caching, and usage; `docs/compatibility/agentic_coding.json` records the latest live result
Agentic cache-accounting slice: `agentic_coding` now requires provider-reported cache read/write token accounting, OpenRouter Responses decodes both Responses-style and Chat/Completions-style cache usage details, and the latest live matrix approves every promoted row with cache-accounting evidence
Runtime-view selection slice: configured provider instances are projected into modeldb runtime views, compatibility evidence artifacts can be loaded by consumers, `adapterconfig.SelectModelForUseCase` and `AutoResult.SelectModelForUseCase` fail closed unless a provider/API/native model row is approved, and `llmadapter resolve --use-case ... --approved-only` exposes the same strict selection path
Compatibility artifact automation slice: `compatibility` can load/save/render evidence artifacts, live agentic-coding e2e can write the JSON artifact when `LLMADAPTER_COMPAT_ARTIFACT` is set, and `llmadapter compatibility-record` regenerates the use-case matrix generated section from the artifact
Modeldb Codex gpt-5.5 slice: bumped modeldb to v0.13.2 so auto mux modeldb routing can resolve the Codex service-qualified `codex/gpt-5.5` offering through `codex_responses`
Provider conformance report slice: `conformance` joins provider registry descriptors, endpoint feature evidence, warnings, and live use-case compatibility artifact rows; `llmadapter conformance --json` exposes the report for automation and operator review
Responses ownership cleanup slice: `providers/openai/responses` owns the canonical Responses provider wire implementation; `openrouter_responses` wraps it with OpenRouter-only body extensions and `codex_responses` wraps it with Codex auth/header/body behavior instead of depending on OpenRouter internals
Codex continuation guard slice: `codex_responses` no longer forwards unsupported `previous_response_id` fields to the Codex HTTP/SSE backend; callers receive a warning and should use replay rather than native Responses continuation for this provider endpoint
Continuation metadata slice: provider descriptors, routes, mux `RouteEvent`s, config inspection, model resolution, compatibility artifacts, conformance output, and CLI diagnostics expose `consumer_continuation`, `internal_continuation`, and `transport`; consumers use only `consumer_continuation` for projection decisions, while `internal_continuation` and `transport` remain diagnostics; OpenAI Responses advertises public `previous_response_id`, while Codex/OpenRouter-compatible endpoints remain public replay until live-verified
Codex interaction diagnostics slice: `unified.CodexExtensions` includes interaction/session/branch hints and `llmadapter infer` exposes `--interaction`, `--session`, and `--branch`; default infer remains one-shot, while `--session` implies session mode unless explicitly overridden
Codex WebSocket transport slice: shared transport includes a WebSocket byte-stream implementation; `codex_responses` can send session-mode requests as Codex `response.create` WebSocket messages, wraps received JSON events into the existing Responses decoder path, forces IPv4 for the Codex WebSocket dial to avoid IPv6 stalls seen in practice, and falls back to HTTP/SSE before streaming starts
Codex branch-safe continuation slice: `codex_responses` tracks in-memory continuation state per session/branch, computes canonical per-input prefix fingerprints from the mutated Codex wire body, reuses the same explicit-session WebSocket connection for affinity, invalidates continuation state after failed/stale WebSocket sessions, and only attaches internal WebSocket `previous_response_id` for same-model, same-instructions, append-only continuations after a prior successful completion
Provider execution metadata slice: providers can emit `unified.ProviderExecutionEvent` with actual transport/internal-continuation decisions; Codex emits this for WebSocket and HTTP/SSE paths, Responses decoding preserves it, and `muxclient` folds it into the initial `RouteEvent` for consumers
Codex WebSocket live smoke slice: `tests/e2e` includes `TEST_INTEGRATION`-gated Codex session continuation and prompt-cache smokes that assert WebSocket transport, second-turn internal `previous_response_id` reuse, and provider-reported cache read accounting while WebSocket is active
Responses WebSocket option slice: `providers/openai/responses` exposes three-state `WithWebSocketMode(...)` support for official OpenAI Responses WebSocket mode, with deterministic enabled/default/auto/fallback tests, a shared compression-on, IPv4-forced default WebSocket transport, and shared session reuse/open-or-write mechanics used by native OpenAI Responses and Codex; `openrouter_responses` stays on HTTP/SSE unless it explicitly opts in later
OpenAI Responses WebSocket smoke slice: `tests/e2e/TestSmokeOpenAIResponsesWebSocket` verifies direct OpenAI Responses WebSocket mode with live OpenAI credentials, stable cache/session key, streamed text, usage, and runtime `transport=websocket` metadata
Codex WebSocket conformance slice: deterministic Codex provider tests cover session-mode WebSocket enabled, WebSocket disabled, missing stable session ID, pre-stream HTTP/SSE fallback, retry-after-fallback, mid-stream failure invalidation, and branch-safe internal continuation; Codex uses the same WebSocket mode vocabulary while preserving Codex-specific auth/session behavior
```

Verified:

```text
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go build ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke' -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/openai_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/openai_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestAnthropicMessagesGatewaySmoke' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestResponsesGatewaySmoke' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokePromptCache/(anthropic|claude)' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmoke(TextStream|ToolUse|ToolResultContinuation|ResponsesContinuation)/openai_responses|TestResponsesGatewaySmoke(NonStreaming|Streaming)/openai_responses' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
env GOCACHE=/tmp/go-cache go test ./...
```

Implemented package surface:

```text
unified/
adapt/
pipeline/
transport/
pricing/
modelmeta/
providers/anthropic/messages/
providers/openai/chatcompletions/
providers/openrouter/chatcompletions/
providers/openrouter/messages/
providers/openrouter/responses/
providers/minimax/chatcompletions/
providers/minimax/messages/
tests/e2e/
endpoints/openaichatcompletions/
endpoints/openairesponses/
endpoints/anthropicmessages/
gateway/
cmd/llmadapter-gateway/
router/
.agents/skills/llmadapter-provider-extension/
```

Gateway command config:

```text
LLMADAPTER_CONFIG optionally points to a JSON config file
LLMADAPTER_ADDR sets the listen address when no config file is used
ANTHROPIC_API_KEY provides default Anthropic credentials when no config file is used
LLMADAPTER_UPSTREAM_MODEL sets the default native model override when no config file is used
provider config supports api_key or api_key_env
provider config supports base_url, model, priority, and capability overrides
provider config supports modeldb_service_id for pricing/catalog service identity
provider type claude defaults modeldb_service_id to anthropic
provider type codex_responses defaults modeldb_service_id to codex
modeldb config supports catalog_path, overlay_paths, and aliases
route config supports source_api, model, provider, provider_api, modeldb_model, native_model, modeldb_wire_model_id, dynamic_models, and weight
health_cooldown configures the in-memory provider endpoint/model failure deprioritization window
llmadapter-gateway -inspect-config prints resolved config metadata as JSON and does not require provider API keys
```

Anthropic path coverage:

```text
unified.Request -> Anthropic MessageRequest
HTTP byte-stream request construction
default HTTP transport supports gzip, deflate, br, and zstd response decompression
optional bearer/OAuth auth instead of x-api-key for Claude Code-compatible access
Claude Code-compatible headers, beta=true query behavior, local OAuth token refresh, and request preflight metadata
Anthropic system/content text blocks encode canonical cache_control hints
raw SSE event block parsing
Anthropic wire event decoding
Anthropic wire event -> unified.Event mapping
text streaming
tool-use streaming with argument deltas
usage and finish-reason mapping
structured token categories for input.new, input.cache_read, input.cache_write, and output
unified.Collect(...)
fake transport integration tests
live Anthropic smoke test through unified.Client
live prompt-cache smoke checks provider-reported cache write/read tokens for Anthropic and claude
```

Gateway path coverage:

```text
OpenAI Chat Completions HTTP request -> unified.Request
unified.Event -> OpenAI Chat Completions non-streaming JSON response
unified.Event -> OpenAI Chat Completions SSE chunks
Anthropic Messages HTTP request -> unified.Request
unified.Event -> Anthropic Messages non-streaming JSON response
unified.Event -> Anthropic Messages SSE chunks
OpenAI Responses HTTP request -> unified.Request
unified.Event -> OpenAI Responses non-streaming JSON response
unified.Event -> OpenAI Responses SSE chunks
minimal gateway handler with configured unified.Client
endpoint-shaped errors before response start
runnable Anthropic-backed /v1/chat/completions gateway command
live gateway smoke tests for streaming and non-streaming requests
gateway route selection through router.StaticRouter
native model rewrite before provider client invocation
route results preserve provider endpoint metadata: target API kind, compatibility family, provider name, and capabilities
same OpenAI Chat endpoint smoke-tested against Anthropic and OpenAI upstreams
same Anthropic Messages endpoint smoke-tested against Anthropic, OpenRouter Messages, and MiniMax Messages upstreams
same OpenAI Responses endpoint smoke-tested against OpenRouter Responses upstream
shared unified.Client tool-use smoke tests pass against Anthropic and OpenAI
shared unified.Client tool-result continuation smoke tests pass against Anthropic and OpenAI
shared unified.Client text smoke tests pass across Anthropic, OpenAI Chat, OpenRouter Chat, OpenRouter Responses, and OpenRouter Messages
OpenRouter Chat, Responses, and Messages pass shared tool-use and tool-result continuation smokes
OpenRouter Responses routes through the OpenAI Chat gateway smoke path via canonical text conversion
MiniMax Chat uses the OpenAI-compatible stream path and is registered in text and gateway smoke matrices
MiniMax Messages uses the Anthropic-compatible stream path and is registered in text, tool, continuation, and gateway smoke matrices
Anthropic, OpenAI Chat, and OpenRouter Responses provider decoders emit structured usage token categories where upstream details are available
fixed-model gateway routes can enrich provider usage events with modeldb pricing when configured with explicit service/model metadata
dynamic model gateway routes resolve models through modeldb, narrow capabilities, rewrite to the selected offering wire model, and enrich provider usage events with modeldb metadata/pricing when the catalog has a matching offering
fixed-model gateway routes can narrow endpoint capabilities and attach token limits from modeldb OfferingExposure metadata
gateway config inspection exposes the resolved metadata path for debugging route/API/modeldb configuration
gateway modeldb loading can use built-in catalog data, an explicit JSON catalog path, and local JSON overlays
route modeldb_model resolution can turn catalog/local aliases into explicit native models before metadata/pricing enrichment
claude can be routed as an Anthropic Messages-compatible endpoint while preserving Anthropic modeldb pricing/capability metadata
```

Live e2e defaults:

```text
TEST_INTEGRATION=1 enables live e2e tests
ANTHROPIC_API_KEY provides Anthropic credentials
ANTHROPIC_MODEL overrides the default Anthropic smoke-test model
default Anthropic smoke-test model: claude-haiku-4-5-20251001
OPENAI_API_KEY or OPENAI_KEY provides OpenAI credentials
OPENAI_MODEL overrides the default OpenAI smoke-test model
default OpenAI smoke-test model: gpt-4.1-mini
OPENROUTER_API_KEY or OPENROUTER_KEY provides OpenRouter credentials
OPENROUTER_MODEL overrides the default OpenRouter smoke-test model
default OpenRouter smoke-test model: openai/gpt-4.1-mini
OPENROUTER_RESPONSES_MODEL overrides the default OpenRouter Responses smoke-test model
default OpenRouter Responses smoke-test model: openai/gpt-4.1-mini
OPENROUTER_MESSAGES_MODEL overrides the default OpenRouter Messages smoke-test model
default OpenRouter Messages smoke-test model: anthropic/claude-sonnet-4
MINIMAX_API_KEY or MINIMAX_KEY provides MiniMax credentials
MINIMAX_MODEL overrides the default MiniMax smoke-test model
default MiniMax smoke-test model: MiniMax-M2.7
MINIMAX_MESSAGES_MODEL overrides the default MiniMax Messages smoke-test model
default MiniMax Messages smoke-test model: MiniMax-M2.7
Local Claude Code OAuth credentials in `~/.claude/.credentials.json` (or `CLAUDE_CONFIG_DIR`) provide claude credentials when that provider type is configured
CLAUDE_MODEL overrides the default claude smoke-test model
CODEX_ACCESS_TOKEN, CODEX_CODE_OAUTH_TOKEN, or local Codex OAuth credentials provide codex_responses credentials when that provider type is configured
CODEX_AUTH_PATH overrides the Codex local auth file path, otherwise ~/.codex/auth.json is used
CODEX_MODEL overrides the default codex_responses smoke-test model
```

Known stable limitations and post-v1 follow-up:

```text
Stable limitation: OpenAI Chat, OpenAI Responses, Anthropic Messages, OpenRouter, MiniMax, Claude-compatible access, and Codex provider paths are stream-first compatibility surfaces, not full clones of every upstream provider field.
Stable limitation: OpenRouter extension passthrough is centralized through typed raw helpers with shape and focused semantic validation for mature routing/provider/plugin/session controls; broader provider-specific controls should stay namespaced until their semantics are stable.
Stable limitation: prompt caching request hints are implemented for Anthropic-family block cache_control and OpenAI Responses prompt_cache_key/prompt_cache_retention extensions; higher-level cache policy belongs above llmadapter. Codex uses the session/window cache key path and the live smoke checks follow-up cache-read accounting.
Known follow-up: decide whether JSON config, auto mux, and workload compatibility evidence should expose OpenAI Responses WebSocket mode beyond direct provider clients; keep OpenAI Realtime as a separate future API kind/family.
Post-v1: broad audio/video/file/document/built-in-tool provider support, plugin-style external provider loading, probabilistic load balancing, broad provider-specific extension semantics, and additional provider families such as Ollama/Bedrock/Vertex/Azure/Gemini.
```

Current stable state:

```text
llmadapter is a stateless, stream-first adapter with a shared canonical request/event model, provider endpoint routing, HTTP gateway endpoints, a Cobra CLI, and an in-process mux client.
Model resolution is centralized through adapterconfig/modeldb catalog loading plus alias overlays; CLI resolve/infer, auto mux, gateway, and muxclient use the same catalog-backed route/native-model decision path when modeldb is enabled.
Capability provenance is inspectable as provider descriptor defaults, explicit config overrides, or modeldb exposure metadata.
Supported provider endpoint families cover Anthropic Messages-compatible, OpenAI Chat-compatible, and OpenAI Responses-compatible surfaces, including Anthropic, Claude Code-compatible access, OpenAI, OpenRouter, MiniMax, and Codex endpoint variants.
Usage/cost accounting is canonical and structured; provider raw usage payloads are retained when available, and modeldb-backed pricing is absent-safe.
Prompt caching primitives are explicit request hints only; session-level cache policy and stateful conversation projection remain agentsdk responsibilities.
Conformance coverage now includes text, function tools, parallel tool-call decoding, tool continuation, reasoning signatures, citation variants, endpoint raw decode metadata, endpoint citation projection, prompt-cache accounting for Anthropic/Claude/Codex, cache-control mapping for additional compatible surfaces, error normalization, provider raw error payloads, dynamic model rejection, multimodal image support, unsupported media/built-in-tool warning/drop behavior, typed extension validation, raw provider usage, and selected raw provider events.
```

Implementation assessment:

```text
Foundation is solid for a stateless adapter: canonical request/event model, stream-first provider clients, deterministic weighted routing, pre-response gateway fallback, fake transport unit tests, shared CLI/gateway config construction, in-process mux routing, modeldb-backed resolution/pricing, and live outside-in e2e tests are all working.
Main intentional shortcuts are stream-first provider paths, family-level default capabilities, explicit static provider descriptors, and partial protocol coverage for provider-specific advanced fields.
Current live tests are good smoke coverage, not full conformance coverage.
Important remaining test gaps: additional endpoint-codec edge cases, additional citation variants as providers expose new annotation shapes, additional provider-specific error variants as new providers expose them, actual audio/video/file/document/built-in-tool provider support if/when added, and broader semantic validation for provider-specific extension groups that are still intentionally raw.
Compared with ../agentapis and ../llmproviders, llmadapter is stronger as a stateless gateway/adapter foundation and now covers provider auto-detection for env/local credentials. Stateful conversation ownership has moved to agentsdk.
```

Next planned phase:

```text
The remaining work is the v1 completion roadmap below. The immediate next implementation phase after v1.0.0-rc.1 is V1 phase 9: promote v1.0.0 after any release-candidate regressions or documentation inaccuracies are fixed.
```

V1 completion roadmap:

```text
Goal: reach a stable v1.0.0 baseline that can be promoted as the public stateless adapter/gateway/mux foundation.

Execution rule:
- Work phase by phase in order.
- Each phase must leave the tree buildable, documented, committed, tagged, pushed, and released.
- Use patch releases while hardening, for example v0.48.25, v0.48.26, and so on.
- Cut v1.0.0 only after every mandatory phase below is complete and verified.
- Do not add new broad provider families during this track unless they are needed to close an existing v1 gap.

Mandatory verification for every implementation phase:
- env GOCACHE=/tmp/go-cache go test ./...
- env GOCACHE=/tmp/go-cache go vet ./...
- env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...

Mandatory release checklist for every phase:
- update PLAN.md status and current stable state when behavior changes
- update README.md or docs/ARCHITECTURE.md when public behavior or architecture changes
- update CHANGELOG.md with the exact version entry before tagging
- commit with a focused message
- create an annotated git tag
- push main and the tag
- create a GitHub release with concise release notes and verification commands
```

V1 phase 1: status/docs truth pass.

```text
Purpose:
Make the repository self-describing before more code changes.

Tasks:
1. Update PLAN.md status date, current stable state, known gaps, and next planned phase so they match the current implementation.
2. Update docs/ARCHITECTURE.md known shortcomings so solved items are removed or rewritten as remaining policy choices.
3. Update README.md known limitations so users can tell which behavior is stable, smoke-tested, best-effort, or intentionally out of scope.
4. Add or update a short "v1 stability target" section that says llmadapter is stateless and conversation state belongs above it.
5. Audit TODO/FIXME/deferred wording in README.md, PLAN.md, docs/ARCHITECTURE.md, and AGENTS.md.
6. Do not change runtime behavior in this phase unless a doc claim is plainly contradicted by a tiny bug.

Done criteria:
- docs have no stale claims about already-completed provider registry, Anthropic wire cleanup, extension validation, Codex parity, or conformance slices
- remaining gaps are phrased as explicit v1 blockers, v1 non-blockers, or post-v1 expansion items
- release a docs/status patch
```

V1 phase 2: conformance fixture closure.

```text
Purpose:
Turn the remaining "smoke coverage, not conformance" risk into deterministic offline fixtures.

Tasks:
1. Add endpoint codec fixtures for malformed and edge-case OpenAI Chat, OpenAI Responses, and Anthropic Messages inputs.
2. Add provider stream fixtures for additional reasoning variants exposed by supported providers, including signed/unsigned Anthropic-family blocks and Responses-family reasoning summaries.
3. Add citation fixtures for supported Responses-family and Anthropic-family citation shapes that are not already covered.
4. Add provider error fixtures for additional non-2xx and mid-stream body shapes that supported providers can emit.
5. Add raw/unmapped event preservation fixtures for at least one extra Responses-family event and one extra Anthropic-family event.
6. Add regression tests that unsupported audio/video/file/document/built-in-tool inputs never leak into provider wire payloads.
7. Keep live tests as smoke tests; do not make conformance depend on paid network calls.

Done criteria:
- all listed fixture groups have focused unit tests
- conformance gaps in PLAN.md/docs are narrowed to genuinely unknown future provider variants
- release a conformance patch
```

V1 phase 3: routing and fallback policy finalization.

```text
Purpose:
Make gateway and mux failure behavior explicit and stable.

Tasks:
1. Add configurable retry attempt limits for route fallback where they do not already exist.
2. Add optional backoff configuration only if it can be done without complicating the default path.
3. Classify retryable setup/pre-stream failures versus non-retryable request validation failures.
4. Keep HTTP response-start behavior in gateway and shared route mechanics in internal/routeattempt.
5. Ensure muxclient and gateway report route attempts, selected route, native model, and final provider/API error consistently.
6. Add tests for retry limit exhaustion, non-retryable validation failures, and consistent mux/gateway error formatting.
7. Update CLI resolve/infer diagnostics if route failure output is ambiguous.

Done criteria:
- fallback behavior is deterministic and documented
- gateway and mux differ only where HTTP response-start semantics require it
- release a routing-policy patch
```

V1 phase 4: capability and model policy finalization.

```text
Purpose:
Remove ambiguity around what can be routed and why.

Tasks:
1. Audit all provider descriptors for default capabilities and mark which are endpoint-family defaults versus live-verified model capabilities.
2. Ensure modeldb-backed dynamic routes reject unavailable catalog models and do not silently relabel unknown models as provider defaults.
3. Ensure fixed routes with explicit native_model remain deterministic and inspectable.
4. Add CLI inspect/resolve warnings when a route relies on provider-family default capabilities instead of catalog-confirmed metadata.
5. Add tests for catalog hit, catalog miss, service-qualified model, alias overlay, fixed route, and dynamic route behavior.
6. Ensure pricing remains absent-safe when modeldb has no pricing for a selected offering.

Done criteria:
- model resolution has one documented path through modeldb plus overlays when modeldb is enabled
- capability decisions are explainable through CLI/config inspection
- no fallback model substitution surprises remain
- release a model-policy patch
```

V1 phase 5: public CLI, config, and examples finalization.

```text
Purpose:
Make the project usable without reading the code.

Tasks:
1. Review `llmadapter providers`, `llmadapter models`, `llmadapter resolve`, `llmadapter infer`, `llmadapter smoke`, and `llmadapter serve` for consistent flag names and output labels.
2. Add README examples for auto mux, config mux, direct inference, gateway serve, model resolution, and Docker gateway startup.
3. Add a minimal documented config example that includes provider endpoints, dynamic model routes, modeldb catalog/overlays, aliases, capabilities, and pricing.
4. Confirm `cmd/llmadapter-gateway` is documented as a compatibility binary over the shared server path.
5. Add CLI regression tests where output format is user-facing enough to matter, especially resolve/infer/provider identity fields.
6. Ensure no command prints secrets and redaction behavior is documented.

Done criteria:
- a new consumer can configure and run CLI/gateway/mux paths from README examples
- CLI diagnostics use provider instance/type/API/family/model terminology consistently
- release a CLI/docs patch
```

V1 phase 6: provider smoke matrix finalization.

```text
Purpose:
Define the supported provider matrix for v1 and verify it outside-in.

Tasks:
1. Enumerate v1-supported provider endpoints: anthropic messages, claude messages, openai chat, openai responses, openrouter chat, openrouter responses, openrouter messages, minimax chat, minimax messages, and codex responses.
2. For each endpoint, document supported features: text, tools, tool continuation, reasoning, prompt caching, structured output, vision, raw events, usage, pricing, and gateway routing.
3. Ensure e2e smoke tests skip cleanly when credentials or local auth files are missing.
4. Run the full available live smoke matrix with current local credentials.
5. Update README/PLAN with the verified matrix and any skipped credentials.
6. Do not block v1 on Ollama, DockerMR, Bedrock, Vertex, or other new providers unless explicitly reprioritized.

Done criteria:
- the v1 provider matrix is documented and matches test coverage
- live smoke results are recorded in PLAN.md with exact commands
- release a provider-matrix patch
```

V1 phase 7: public API and package-boundary freeze.

```text
Purpose:
Avoid cutting v1 with avoidable exported API churn.

Tasks:
1. Review exported identifiers in unified, adapt, router, muxclient, adapterconfig, providerregistry, pricing, modelmeta, transport, and gatewayserver.
2. Rename or deprecate confusing exported names before v1 rather than after v1.
3. Confirm internal packages contain implementation details that should not be public.
4. Confirm provider packages expose only stable constructors/options needed by registry and advanced users.
5. Add doc comments for exported types/functions that are part of the public surface and currently undocumented.
6. Run `go vet` and package documentation checks; fix obvious lint/doc issues without adding noisy comments.

Done criteria:
- public API names align with the architecture terminology: provider instance, provider type, API kind, API family, endpoint, route, modeldb service/offering
- no obvious package-boundary leaks remain
- release an API-freeze patch
```

V1 phase 8: final hardening and release-candidate cut.

```text
Purpose:
Produce a release candidate that should be v1 unless a real bug appears.

Tasks:
1. Run the mandatory local verification suite from a clean tree.
2. Run all non-secret CLI examples from README.
3. Run the available live smoke matrix with TEST_INTEGRATION=1 and record skipped providers.
4. Review CHANGELOG.md and backfill a concise v1 section summarizing the stable surface.
5. Review README.md, AGENTS.md, docs/ARCHITECTURE.md, DESIGN.md, and PLAN.md for contradictions.
6. Verify Docker image build still works if Docker is available.
7. Cut a release candidate tag, for example v1.0.0-rc.1.

Done criteria:
- no known mandatory v1 blocker remains
- release candidate notes include supported provider matrix, verification commands, known limitations, and migration notes from v0.x if needed
```

V1 phase 9: v1.0.0 promotion.

```text
Purpose:
Mark the project stable.

Tasks:
1. Fix only release-candidate regressions or documentation inaccuracies.
2. Re-run mandatory verification and available live smoke matrix.
3. Ensure CHANGELOG.md has a proper v1.0.0 entry.
4. Ensure README.md starts with the stable public use cases and not historical implementation notes.
5. Tag v1.0.0, push, and create GitHub release notes.

Done criteria:
- v1.0.0 release exists on GitHub
- repository docs describe the current stable architecture and usage
- future work is separated into post-v1 backlog, not mixed with v1 blockers
```

V1 done definition:

```text
Consider llmadapter v1-done when all mandatory phases above are complete and:
- local tests, vet, and build are green
- available live smoke tests are green or explicitly skipped for missing credentials
- CLI, gateway, mux client, and auto mux use the same adapterconfig/modeldb-backed resolution path
- supported providers have documented feature coverage and deterministic unsupported-feature behavior
- routing/capability/model decisions are inspectable and do not silently substitute unrelated models
- usage, pricing, caching, reasoning signatures, citations, provider errors, and raw provider metadata have fixture coverage where supported
- README.md, PLAN.md, docs/ARCHITECTURE.md, AGENTS.md, CHANGELOG.md, and release notes agree on the stable surface
- stateful conversations remain out of llmadapter and are documented as agentsdk territory
```

Post-v1 backlog:

```text
- Ollama/local provider support
- Bedrock, Vertex, Azure OpenAI, Gemini, or other additional provider families
- broader multimodal provider support beyond current image passthrough and unsupported-media warning policy
- plugin-style external provider loading
- richer retry/backoff/load-balancing policies if production operators need them
- additional provider-specific extensions as their semantics become stable
- deeper live conformance against provider-specific edge cases as APIs evolve
```

Prototype parity notes from ../agentapis and ../llmproviders:

```text
agentapis has a richer canonical stream shape than llmadapter today:
- TokenItems with input.new, input.cache_read, input.cache_write, output, and output.reasoning categories
- CostItems derived from usage through an injected CostCalculator
- request identity, cache hints, request extras, and protocol-specific cache/prompt controls
- stateful conversation.Session with committed canonical history, replay versus previous_response_id strategies, MessageProjector hooks, prompt cache keys, and commit callbacks (owned by agentsdk)
- conversation-level tests for tool loops, multi-turn memory, usage/cost enrichment, and cache policy behavior (owned by agentsdk)

llmproviders has a higher-level provider service layer than llmadapter today:
- modeldb-backed service/offering/model lookup
- provider registry detection and construction from local credentials/API keys
- alias and intent resolution such as fast/default/powerful and sonnet/opus/haiku
- cost-aware provider wrappers using modeldb pricing
- Claude OAuth as an Anthropic service instance named "claude"
- broader provider targets, including OpenAI Codex, Ollama, DockerMR/local runtime probes, OpenRouter model metadata, and a shared integration matrix

Do not copy these layers directly into llmadapter core. Port the durable concepts behind small seams:
- modeldb catalog adapter for endpoint metadata and pricing
- event processor for cost enrichment
- provider auth options for Claude OAuth compatibility
- stateless unified.Client primitives consumed by agentsdk conversation/runtime layers
- adapter-native CLI commands for serving, model/route inspection, resolving, and smoke testing
- in-process mux client for library users that want gateway-like routing without running an HTTP server
- explicit dynamic model pass-through routes for consumers that need full provider/modeldb access beyond deterministic aliases
- shared e2e matrix scenarios that exercise the public outside-in surface
```

CLI plan:

```text
Goal: provide a repo-native CLI comparable to ../llmproviders/llmcli while preserving llmadapter's architecture.

Target binary:
- cmd/llmadapter or cmd/llmadapter-cli; keep cmd/llmadapter-gateway working until the new CLI can subsume it

Initial commands:
1. serve
   - start the compatibility gateway (implemented in `llmadapter serve`)
   - accept the same LLMADAPTER_CONFIG config path and listen address overrides (implemented)
   - expose /v1/chat/completions, /v1/responses, and /v1/messages (implemented)
2. models
   - list configured or auto-detected provider endpoints and native/public models (implemented)
   - fixed-route modeldb service/offering/exposure metadata is visible through the gateway inspection path; fold this into the future unified CLI
3. resolve
   - explain how a public model/API request maps to provider endpoint, API kind, family, native model, capabilities, and route weight/priority (implemented)
4. providers
   - list provider endpoint types and API metadata, plus redacted auto/configured credential status without printing secrets (implemented)
5. smoke
   - run a minimal outside-in request against one provider endpoint for text streaming (implemented for direct provider endpoints, single-route mux mode, config-driven mux mode, and auto-detected mux mode)
6. catalog
   - wrap modeldb catalog inspection once modeldb is an explicit dependency (implemented through `llmadapter models --catalog`)
7. infer
   - send a prompt through the shared config/auto mux path (implemented)
   - support familiar flags from llmproviders/llmcli: --model/-m, --system/-s, --max-tokens, --temperature, --thinking, and --effort (implemented)
   - print resolved model/route information before streaming and usage/cost data after streaming (implemented)

Non-goals for the first CLI slice:
- provider auto-detection from local credentials (implemented for the shared auto mux path)
- interactive agent REPL
- hidden provider registry separate from gateway config
- full replacement for go test e2e matrix

Design rule: CLI commands are operator ergonomics over configured provider endpoints. They must use the same route/config/modeldb metadata path as the gateway so CLI behavior does not drift from server behavior.
```

Mux client plan:

```text
Goal: provide a library-level client comparable to agentapis' mux client while preserving llmadapter's stateless architecture.

Shape:
- package muxclient (initial package implemented)
- exposes unified.Client (implemented)
- accepts config/modeldb/providerregistry inputs (implemented through adapterconfig)
- constructs configured provider endpoints (implemented through adapterconfig)
- resolves public model aliases through modeldb when configured (implemented through adapterconfig)
- chooses an API kind/provider endpoint from request needs and endpoint capabilities (implemented through router.Router input)
- uses router.StaticRouter or a compatible router interface internally (implemented)
- optionally applies modeldb-backed pricing processors just like the gateway (implemented through adapterconfig)

Non-goals:
- conversation/session state
- durable history
- hidden provider credentials outside explicit config/env discovery
- provider-native continuation state

Why it belongs here:
- it is stateless request routing over provider endpoints
- agentsdk can use it as an upstream unified.Client
- it lets tools/tests run gateway-equivalent routing without an HTTP server
```

modeldb integration plan:

```text
Research finding: ../modeldb is a standalone catalog package with ModelRecord, Offering, OfferingExposure, Service, Runtime, Pricing, Capabilities, APIType, ServiceView, RuntimeView, selectors, preference overlays, and built-in catalog loading.
Important modeldb rule: runtime invocation should target an OfferingExposure, not just a logical model or offering.

Initial integration goals:
1. Add an internal or public catalog bridge package that imports github.com/codewandler/modeldb.
2. Map modeldb.APIType to llmadapter adapt.ApiFamily/adapt.ApiKind candidates:
   - anthropic-messages -> FamilyAnthropicMessages
   - openai-chat -> FamilyOpenAIChatCompletions
   - openai-responses -> FamilyOpenAIResponses
   - openai-messages remains future/experimental until llmadapter has an exact endpoint family
3. Map modeldb OfferingExposure.ExposedCapabilities into router.CapabilitySet:
   - streaming -> Streaming
   - tool_use -> Tools
   - parallel_tool_calls -> ParallelTools
   - structured_output/structured_outputs -> JSONSchema/JSONMode depending on supported parameters
   - vision -> Vision
   - reasoning availability and modes -> Reasoning/ReasoningDeltas when the endpoint codec supports them
   - caching availability -> PromptCaching, not automatic request cache behavior
   - limits -> MaxInputTokens/MaxOutputTokens
   (implemented for fixed-model configured routes; modeldb narrows endpoint-family capabilities and does not enable unimplemented endpoint features)
4. Use ServiceID + WireModelID + APIType to enrich configured provider endpoints and routes. (implemented for fixed-model gateway routes)
5. Keep credentials and clients explicit in gateway config; the catalog may select metadata and native model IDs, but it must not secretly instantiate providers.
6. Add modeldb preference/alias support only after the explicit metadata path works:
   - resolve public aliases like sonnet/haiku through ServiceView
   - support intent aliases such as fast/default/powerful as user config, not built-in magic
   - allow operator preference overlays to rank service/model offerings before route weights
7. Add catalog-driven validation tests with a small fixture catalog before using the built-in catalog in gateway tests.

Non-goals for the first modeldb slice:
- dynamic provider detection
- automatic model downloads/local runtime acquisition
- replacing explicit gateway routes
- route mutation based on live pricing or provider health
```

Claude-as-Anthropic-subset plan:

```text
Goal: support provider name "claude" as an Anthropic Messages-compatible provider endpoint implemented by the Anthropic Messages client with different auth, headers, and request transforms.

Provider endpoint shape:
- ProviderName: "claude"
- APIKind: anthropic.messages unless an exact claude OAuth API kind becomes necessary later
- Family: anthropic.messages
- modeldb ServiceID: anthropic, because the model catalog/offering identity is still Anthropic Claude models
- capabilities: derived from Anthropic Messages exposure plus Claude OAuth compatibility tests

Implementation pieces:
1. Add an Anthropic auth abstraction to providers/anthropic/messages:
   - API key auth writes x-api-key (implemented)
   - OAuth auth writes Authorization: Bearer <access token> (implemented)
   - TokenProvider and TokenStore interfaces for refreshable OAuth tokens (implemented)
2. Add local Claude credential support:
   - respect CLAUDE_CONFIG_DIR when set (implemented)
   - otherwise read ~/.claude/.credentials.json (implemented)
   - preserve unknown JSON fields on token refresh (implemented)
   - refresh through Anthropic/Claude OAuth token endpoint when expired (implemented)
3. Add Claude compatibility headers and query behavior:
   - Anthropic-Version remains required
   - Anthropic-Beta must include Claude/OAuth/interleaved-thinking/context-management/prompt-caching/effort betas
   - add Claude CLI-style User-Agent, X-App, X-Stainless-* headers, direct browser access header, Accept-Encoding, and beta=true query parameter
   (implemented)
4. Add Claude request transforms:
   - prepend Claude billing/system preflight system blocks (implemented)
   - set metadata user_id derived from ~/.claude.json device/account/session data when available (implemented)
   - optionally add cache_control to the last system block with default TTL (implemented)
   - coerce thinking temperature to an Anthropic-valid value when extended thinking is enabled (implemented)
5. Add gateway config provider type "claude". (implemented)
6. Add e2e tests gated by TEST_INTEGRATION and local Claude credentials:
   - text stream (implemented)
   - thinking stream for Anthropic, Claude Code-compatible access, MiniMax Messages, and OpenRouter Messages (implemented)
   - prompt cache write/read behavior (implemented)
   - tool use/tool result continuation if Claude OAuth account supports it (implemented as regular tool smoke entries)

Safety rule: Claude OAuth compatibility stays an Anthropic provider option/sub-provider, not a new canonical API family. Only split a new API kind if the wire request/stream format diverges from Anthropic Messages.
```

Codex compatibility:

```text
Shape:
- provider endpoint type: codex_responses
- provider/service identity: codex
- exact API kind: codex.responses
- API family: openai.responses

Implemented behavior:
- auth supports CODEX_ACCESS_TOKEN, CODEX_CODE_OAUTH_TOKEN, or local ~/.codex/auth.json via CODEX_AUTH_PATH
- request handling preserves the Responses compatibility surface while applying Codex-specific backend URL, auth headers, installation/session headers, default instructions, store=false, and unsupported body field removal
- modeldb service identity is codex so catalog metadata/pricing stays separate from normal OpenAI platform offerings

Open hardening:
- broader provider-specific extension groups should get typed helpers once they have stable semantics
```

Caching, usage, pricing, and agentsdk conversation integration plan:

```text
Design constraint: llmadapter core and gateway remain stateless per request. Stateful conversations and cache policy are owned by agentsdk or another wrapper above unified.Client; llmadapter owns explicit request/event primitives and optional cost attribution processors.

Usage/cost foundation:
1. Replace flat unified.UsageEvent counters with structured token categories: (implemented)
   - input.new
   - input.cache_read
   - input.cache_write
   - output
   - output.reasoning
2. Derive flattened endpoint wire counters from structured items for API compatibility. (implemented)
3. Add optional cost items: (implemented)
   - input
   - input.cache_read
   - input.cache_write
   - output
   - reasoning
   - future image/audio/video/request/web_search categories
4. Add a cost calculator/event processor that looks up modeldb Offering.Pricing by service/provider endpoint and native model ID. (implemented for explicit service/model offering refs, fixed-model gateway routes, and dynamic routes using the selected request model)
5. Keep pricing absent-safe: if catalog pricing is missing, emit usage without costs.

Prompt caching:
1. Add canonical request/message cache hints only where they map to real provider controls. (implemented for TextPart cache_control)
2. Encode Anthropic-style per-message/block cache_control through Anthropic-family codecs. (implemented)
3. Encode OpenAI Responses prompt_cache_key and prompt_cache_retention through Responses-family codecs. (implemented for endpoint decode and OpenRouter Responses provider encode)
4. Keep OpenAI implicit caching observational only unless a request-side parameter exists.
5. Test cache accounting using provider-reported cache_read/cache_write tokens, not local token estimation.

Conversations:
1. Keep conversation/session implementation in agentsdk, not llmadapter.
2. llmadapter exposes explicit request primitives for agentsdk: previous_response_id, store, prompt_cache_key, prompt_cache_retention, provider-specific session hints, response IDs, and usage/cost events.
3. Replay strategy, commit-safe history, MessageProjector hooks, and session cache policy remain agentsdk responsibilities.
4. llmadapter e2e validates provider semantics that agentsdk depends on, especially native OpenAI Responses previous_response_id behavior.
```

MiniMax research notes:

```text
Official docs describe Text Generation through Anthropic SDK (recommended) and OpenAI SDK.
Anthropic-compatible base URL: https://api.minimax.io/anthropic
Anthropic-compatible endpoint: /anthropic/v1/messages
OpenAI-compatible base URL: https://api.minimax.io/v1
OpenAI-compatible endpoint: /v1/chat/completions
For llmadapter's existing OpenAI-compatible client wrapper, configure base URL as https://api.minimax.io because the client appends /v1/chat/completions.
Supported text models include MiniMax-M2.7, MiniMax-M2.7-highspeed, MiniMax-M2.5, MiniMax-M2.5-highspeed, MiniMax-M2.1, MiniMax-M2.1-highspeed, and MiniMax-M2.
Anthropic-compatible text supports streaming, system, max_tokens, temperature, top_p, tools, tool_choice, metadata, and thinking.
Anthropic-compatible messages support text, tool_use, tool_result, and thinking; image/document inputs are not supported yet.
MiniMax also offers speech, video, image, music, and file APIs; these are out of scope for the current text-first llmadapter provider path.
Sources:
https://platform.minimax.io/docs/api-reference/api-overview
https://platform.minimax.io/docs/api-reference/text-anthropic-api
https://platform.minimax.io/docs/api-reference/text-openai-api
https://platform.minimax.io/docs/api-reference/text-chat-anthropic
https://platform.minimax.io/docs/api-reference/text-chat
```

MiniMax implementation plan:

```text
1. Add adapt API kinds if/when implementation starts:
   - minimax.anthropic_messages -> family anthropic.messages (implemented)
   - minimax.chat_completions -> family openai.chat_completions (implemented)
2. Add providers/minimax/messages as an Anthropic-compatible wrapper over providers/anthropic/messages. (implemented)
   - Base URL: https://api.minimax.io/anthropic
   - Credential env: MINIMAX_API_KEY or MINIMAX_KEY
   - Default model: MiniMax-M2.7
   - Capabilities: streaming, tools, reasoning/thinking; no vision/document input initially
3. Add shared live e2e entries:
   - TestSmokeTextStream/minimax_messages
   - TestSmokeToolUse/minimax_messages
   - TestSmokeToolResultContinuation/minimax_messages (implemented and live-verified)
4. Add providers/minimax/chatcompletions as an OpenAI-compatible wrapper over providers/openai/chatcompletions. (implemented)
   - Base URL: https://api.minimax.io
   - Default model: MiniMax-M2.7
   - Validate text/tool streaming behavior before setting Tools: true
5. Register gateway provider types:
   - minimax_messages (implemented)
   - minimax_chat (implemented)
6. Update README and PLAN with MiniMax env vars and verification commands. (implemented for minimax_chat)
```

---

## Target outcome

At the end of phases 1-3, this must work:

```text
unified.Request
  -> Anthropic Messages request codec
  -> HTTP byte-stream transport
  -> SSE frame decoding
  -> Anthropic wire events
  -> unified.Event stream
  -> unified.Collect(...)
```

The public surface at that point is intentionally small:

```text
unified.Request
unified.Event
unified.Client
providers/anthropic/messages.NewClient(...)
```

Out of scope for these phases:

```text
HTTP gateway endpoints
router / model registry / fallback routing
multi-provider support
OpenAI Responses / Chat compatibility handlers
coreprovider shared package
gateway package
router package
websocket transport
provider-to-provider relays
JSON-schema emulation across providers
```

---

## Working rules

Every step must:

```text
compile without forward references
include focused unit tests
leave the repo in a buildable state
prefer stdlib only
```

Implementation order matters more than package purity in the first pass. If a shared abstraction is only used by Anthropic in phases 1-3, keep it local unless the abstraction is already clearly stable.

---

## Locked decisions for phases 1-3

These decisions remove ambiguity from the original plan and should be treated as fixed unless implementation proves them wrong.

### 1. Public error signaling

Inside the pipeline, use `pipeline.Item[T]` to propagate errors.

At the public `unified.Client` boundary, convert pipeline errors into a final `unified.ErrorEvent` and close the channel.

### 2. SSE transport contract

`transport.ByteStream` remains byte-oriented.

For `FrameFormatSSE`, `transport.HTTPByteStreamTransport` returns one complete raw SSE event block per `Recv`, not just the `data:` payload. This avoids losing the upstream `event:` field, which Anthropic requires to dispatch event types correctly.

`transport/sse.go` therefore owns both:

```text
splitting a response stream into SSE event blocks
parsing one raw SSE block into transport.SSEFrame
```

### 3. Retry and rate limiting live in `transport`

Phase 2 includes:

```text
RetryMode
RetryTransport
RateLimiter interface
RateLimitedTransport wrapper
```

Anthropic phase 3 only needs `WithTransport(...)`. Exposing dedicated rate-limit/retry options can wait until phase 4+.

### 4. No `coreprovider` package yet

`DESIGN.md` describes a future shared provider config package. Do not introduce it in phases 1-3.

For the first implementation, Anthropic owns its local:

```text
Config
Option
HeaderFunc
default base URL / version
processor wiring
```

If the pattern survives phase 4, extract it later.

### 5. First-pass lossiness policy

For Anthropic in phase 3:

```text
strict mode: return UnsupportedFieldError
best_effort mode: append warnings and skip unsupported fields
```

Do not emulate unsupported semantics yet. In particular:

```text
ResponseFormat JSON schema: warn/error only
Seed: warn/error only
provider-specific reasoning controls: extension or warn/error only
```

### 6. Vertical-slice priority

Phase 3 should be implemented in this order:

```text
text-only streaming
usage + completion mapping
tool-use streaming
public constructor
integration coverage
```

Do not start with the full feature matrix.

---

## Phase 1: Core IR and stream pipeline

Goal: define the canonical types and processor pipeline needed by a single provider path. No transport, no HTTP gateway, no router.

### Step 1.1: `unified` content, messages, tools, response config

Create:

```text
unified/content.go
unified/message.go
unified/tool.go
unified/response_format.go
```

Types:

```text
ContentKind
ContentPart
TextPart
ImagePart
AudioPart
VideoPart
FilePart
ReasoningPart
RefusalPart
BlobSourceKind
BlobSource

Role
Message
InstructionKind
Instruction

ToolKind
Tool
ToolChoiceMode
ToolChoice
ToolCall
ToolResult

ResponseFormatKind
ResponseFormat
ReasoningEffort
ReasoningConfig
SafetyConfig
```

Tests:

```text
content parts report the correct kind
public zero values marshal sensibly where relevant
```

Notes:

```text
keep `ContentPart` and `Event` sealed with unexported marker methods
use pointers for optional scalar request fields later
do not add provider-specific fields here
```

### Step 1.2: `unified.Extensions`

Create `unified/extensions.go`.

API:

```text
type Extensions struct
(*Extensions).Set(key string, value any) error
(Extensions).Has(key string) bool
(Extensions).Keys() []string
GetExtension[T](Extensions, key string) (T, bool, error)
```

Extension keys to add now:

```text
ExtOpenAIPreviousResponseID
ExtOpenAIStore
ExtOpenAIPromptCacheKey
ExtOpenAIPromptCacheRetention
ExtAnthropicBetas
ExtGeminiSafetySettings
ExtOpenRouterProviderPrefs
ExtOllamaOptions
```

Tests:

```text
roundtrip for scalar and struct values
missing key returns (zero, false, nil)
type mismatch returns error
Keys returns sorted keys
nil value marshals as JSON null
```

### Step 1.3: `unified.Request`, `APIError`, `Client`

Create:

```text
unified/request.go
unified/errors.go
unified/client.go
```

Types:

```text
Request
APIError
Client
```

Tests:

```text
APIError satisfies error
APIError.Error() is stable and useful
errors.As works through wrapping
```

### Step 1.4: `unified.Event` model

Create `unified/event.go`.

Event set:

```text
MessageStartEvent
MessageDoneEvent
ContentBlockStartEvent
ContentBlockDoneEvent
TextDeltaEvent
ReasoningDeltaEvent
RefusalDeltaEvent
ToolCallStartEvent
ToolCallArgsDeltaEvent
ToolCallDoneEvent
CitationEvent
UsageEvent
CompletedEvent
WarningEvent
RawEvent
ErrorEvent
```

Support types:

```text
Citation
FinishReason
```

Tests:

```text
every event type satisfies the Event interface
```

Notes:

```text
keep `RawEvent` minimal: API kind, type, JSON bytes, optional decoded value
do not add endpoint-specific chunk types
```

### Step 1.5: `unified.Response` and `Collect`

Create:

```text
unified/usage.go
unified/response.go
```

API:

```text
Response
Collect(ctx, <-chan Event) (Response, error)
```

Tests:

```text
empty stream returns zero response
text-only stream assembles a single text block
tool-call stream assembles tool calls
usage is captured from UsageEvent
CompletedEvent sets finish reason
WarningEvents accumulate
RawEvents accumulate
ErrorEvent returns error
cancelled context returns ctx.Err()
mixed text + tool-call stream assembles correctly
```

### Step 1.6: `adapt` request envelope and codec interfaces

Create:

```text
adapt/types.go
adapt/request.go
adapt/codec.go
```

Types:

```text
ApiKind
ApiFamily
MappingMode
Warning
UnsupportedFieldError
HTTPRequestInfo
Request

EventDecoder[In, Out]
EventEncoder[In, Out]
ProviderCodec[Req, Evt]
RequestProcessor
ProviderRequestProcessor[Req]
NativeClient[Req, Evt]
```

Tests:

```text
UnsupportedFieldError satisfies error
```

Notes:

```text
define `NativeClient` here because phase 3 needs it
do not add endpoint codec interfaces yet; those are phase 4 concerns
```

### Step 1.7: generic pipeline primitives

Create:

```text
pipeline/processor.go
pipeline/chain.go
pipeline/transform.go
```

API:

```text
Processor[E]
Chain[E]
NewChain[E](...)
Item[T]
Transform[In, Out](...)
```

Behavior:

```text
Chain.Push runs left-to-right
Chain.Close cascades close-generated events through downstream processors
Transform drains input, forwards decoder output, flushes decoder on close
Transform emits Item.Err on decoder error or ctx cancellation
```

Tests:

```text
empty chain passthrough
filtering processor
expanding processor
two processors compose
Close cascade works
Push error stops chain
Close error propagates

Transform forwards all decoded values
decoder.Close runs when input closes
Push errors become Item.Err
Close errors become Item.Err
ctx cancellation stops transform
slow consumer does not lose values
```

### Step 1.8: built-in unified event processors

Create:

```text
pipeline/coalesce.go
pipeline/filter.go
pipeline/inject.go
```

Processors:

```text
TextCoalescer
ReasoningFilter
CompletionInjector
```

Tests:

```text
TextCoalescer buffers and flushes by threshold
TextCoalescer flushes before non-text events
TextCoalescer flushes on Close
ReasoningFilter drops reasoning when disabled
CompletionInjector injects one completion if none was seen
CompletionInjector does not double-inject
```

### Phase 1 checkpoint

Verify:

```text
go build ./unified/... ./adapt/... ./pipeline/...
go test ./unified/... ./adapt/... ./pipeline/...
go vet ./...
```

Done criteria:

```text
canonical request/event model is stable enough to support one provider
pipeline errors are representable without leaking implementation details
no package requires transport or provider code to compile
```

---

## Phase 2: Transport foundation

Goal: provide a reusable transport layer for streaming provider clients, with SSE support aligned to Anthropic's event model.

### Step 2.1: transport core types

Create `transport/transport.go`.

Types:

```text
Request
ByteStreamTransport
ByteStream
FrameFormat
FrameDecoder[Evt]
RateLimiter
```

Notes:

```text
`Request.Body` should be `io.Reader`
`Request.Extensions` can be `map[string]any`
`RateLimiter` should be a tiny interface: `Wait(context.Context) error`
```

### Step 2.2: SSE framing and parsing

Create `transport/sse.go`.

Types / functions:

```text
SSEFrame
SSEReader
NewSSEReader(io.Reader) *SSEReader
(*SSEReader).Next() ([]byte, error)         // raw SSE event block
ParseSSEFrame([]byte) (SSEFrame, error)     // parse one block
```

Required behavior:

```text
split on blank-line event boundaries
support `event:`, `data:`, `id:`, `retry:`
join multi-line data with `\n`
ignore comment lines
accept `\n`, `\r\n`, and `\r`
handle malformed `field` lines as empty-value fields
return io.EOF cleanly at stream end
```

Tests:

```text
single event
multiple events
multi-line data
event type present
id and retry fields
comment lines skipped
mixed line endings
empty data field
malformed field line
consecutive blank lines
EOF behavior
```

### Step 2.3: NDJSON framing

Create `transport/ndjson.go`.

Types:

```text
NDJSONReader
NewNDJSONReader(io.Reader) *NDJSONReader
(*NDJSONReader).Next() ([]byte, error)
```

Behavior:

```text
return one non-empty line at a time
skip empty lines
support configurable max line size, default 1MB
return io.EOF at end
```

Tests:

```text
single line
multiple lines
empty lines skipped
trailing whitespace
large line under limit
line over limit errors
EOF behavior
```

### Step 2.4: HTTP byte-stream transport

Create `transport/http.go`.

Types:

```text
HTTPTransportConfig
HTTPByteStreamTransport
NewHTTPByteStreamTransport(...)
```

Behavior:

```text
Open sends an HTTP request through http.Client
non-2xx returns *unified.APIError with status and body preview
SSE mode returns one raw SSE event block per Recv
NDJSON mode returns one line per Recv
Raw mode returns full body once, then io.EOF
Close closes the underlying response body
Recv respects ctx cancellation
```

Tests using `httptest.Server`:

```text
SSE stream yields multiple raw event blocks
NDJSON stream yields multiple lines
Raw mode returns one full body
non-2xx maps to APIError
request method, URL, headers, and body are correct
Close closes the body
ctx cancellation interrupts Recv
```

### Step 2.5: fake transport for unit and integration tests

Create `transport/fake.go`.

Types:

```text
FakeByteStreamTransport
fakeByteStream
```

Behavior:

```text
capture seen requests
allow Open error
allow frame-by-frame error injection
return io.EOF when frames are exhausted
```

Tests:

```text
frames returned in order
error at configured frame
Open error returned
seen requests captured
empty stream returns EOF
```

### Step 2.6: rate-limit wrapper

Create `transport/ratelimit.go`.

Types:

```text
RateLimitedTransport
NewRateLimitedTransport(inner ByteStreamTransport, limiter RateLimiter) *RateLimitedTransport
```

Behavior:

```text
call limiter.Wait(ctx) before inner.Open
do nothing when limiter is nil
propagate limiter errors directly
```

Tests:

```text
Wait called before Open
limiter error stops Open
nil limiter delegates directly
```

### Step 2.7: retry wrapper

Create `transport/retry.go`.

Types:

```text
RetryMode
RetryConfig
RetryTransport
NewRetryTransport(inner ByteStreamTransport, cfg RetryConfig) *RetryTransport
```

Behavior:

```text
Mode=never delegates directly
Mode=before_stream retries Open failures and retryable pre-stream API errors
default retryable statuses: 429, 500, 502, 503, 504
never retry after the first successful Recv
respect ctx cancellation during backoff
```

Tests:

```text
never mode performs no retries
Open error retries and later succeeds
retryable APIError retries
non-retryable APIError does not retry
MaxRetries is enforced
ctx cancellation during backoff returns ctx.Err()
```

### Phase 2 checkpoint

Verify:

```text
go build ./transport/...
go test ./transport/...
go vet ./...
```

Done criteria:

```text
Anthropic native client can be built entirely on transport abstractions
SSE event type information is preserved end-to-end
retry and rate-limit behavior are testable without provider code
```

---

## Phase 3: One complete provider path - Anthropic Messages

Goal: ship one real provider path from `unified.Request` to `unified.Event`.

Implementation order inside phase 3 is strict:

```text
3A text-only request/response path
3B usage + completion + tool-use support
3C public constructor and end-to-end tests
```

### Step 3.1: Anthropic wire types

Create:

```text
providers/anthropic/messages/wire.go
providers/anthropic/messages/wire_test.go
```

Request-side types:

```text
MessageRequest
InputMessage
ContentBlock
ToolDefinition
ToolChoiceWire
Metadata
```

Response/event-side types:

```text
Event
MessageStartEvent
MessageResponse
ContentBlockStartEvent
ContentBlockDeltaEvent
ContentBlockStopEvent
MessageDeltaEvent
MessageDeltaBody
MessageStopEvent
PingEvent
ErrorEventWire
APIErrorBody
UsageWire
```

Tests:

```text
request JSON roundtrip: text
request JSON roundtrip: system instructions
request JSON roundtrip: tools
request JSON roundtrip: images
response/event JSON roundtrip
omitempty behavior
```

### Step 3.2: Anthropic request codec (`unified` -> wire)

Create:

```text
providers/anthropic/messages/codec.go
providers/anthropic/messages/codec_test.go
testdata/anthropic/request_*.json
```

Codec responsibilities:

```text
implement adapt.ProviderCodec[MessageRequest, Event]
encode unified.Request into MessageRequest
create a new wire-event decoder later via NewEventDecoder()
```

Mapping rules for first pass:

```text
Model -> Model
MaxOutputTokens -> MaxTokens (required)
Messages -> Messages
Instructions -> System
Temperature / TopP / TopK / Stop -> direct mappings
Tools -> Tools (function tools only)
ToolChoice -> ToolChoice
Stream -> Stream
```

Lossiness rules:

```text
strict: error on missing MaxOutputTokens, unsupported tool kinds, Seed, ResponseFormat, unsupported content
best_effort: append warnings and drop unsupported fields
```

Implementation order:

```text
first: text-only user/assistant messages
then: system instructions
then: tools + tool results
then: image blocks
```

Tests:

```text
basic text request fixture
multi-turn fixture
system instruction fixture
tool definition fixture
tool result fixture
image fixture
missing MaxOutputTokens errors
strict vs best_effort behavior
```

### Step 3.3: Anthropic SSE frame decoder (raw SSE block -> wire event)

Create:

```text
providers/anthropic/messages/sse.go
providers/anthropic/messages/sse_test.go
```

API:

```text
SSEFrameDecoder implementing transport.FrameDecoder[Event]
```

Behavior:

```text
ParseSSEFrame on each raw block
dispatch using SSEFrame.Event
decode JSON from SSEFrame.Data
support message_start, content_block_start, content_block_delta,
content_block_stop, message_delta, message_stop, ping, error
```

Policy:

```text
unknown SSE event type -> error in phase 3
empty ping payload is allowed
malformed JSON returns error
```

Tests:

```text
each supported event type parses correctly
tool_use and input_json_delta cases parse correctly
unknown event type errors
bad JSON errors
```

### Step 3.4: Anthropic event decoder (wire event -> `unified.Event`)

Create:

```text
providers/anthropic/messages/decoder.go
providers/anthropic/messages/decoder_test.go
testdata/anthropic/events_*.ndjson
```

API:

```text
EventDecoder implementing adapt.EventDecoder[Event, unified.Event]
```

State to track:

```text
open content blocks by index
tool-call IDs and names
buffered tool-call argument fragments
message metadata needed for final completion mapping
```

Required mappings:

```text
message_start -> MessageStartEvent
content_block_start(text) -> ContentBlockStartEvent
content_block_delta(text_delta) -> TextDeltaEvent
content_block_stop(text) -> ContentBlockDoneEvent
message_delta -> CompletedEvent and optional UsageEvent
message_stop -> MessageDoneEvent
ping -> dropped
error -> unified.ErrorEvent wrapping unified.APIError
```

Additional mappings for 3B:

```text
tool_use block start -> ContentBlockStartEvent + ToolCallStartEvent
input_json_delta -> ToolCallArgsDeltaEvent
tool block stop -> ToolCallDoneEvent + ContentBlockDoneEvent
thinking deltas -> ReasoningDeltaEvent
stop_reason mapping:
  end_turn -> stop
  max_tokens -> length
  tool_use -> tool_call
  stop_sequence -> stop
```

Close behavior:

```text
error if a tool-call buffer is incomplete
otherwise flush nothing
```

Tests:

```text
plain text stream fixture
multi-block text fixture
tool-use fixture
parallel tool-call fixture
reasoning fixture
usage mapping
stop-reason mapping
ping dropped
error event mapped
incomplete tool buffer errors on Close
```

### Step 3.5: Anthropic native client

Create:

```text
providers/anthropic/messages/client.go
providers/anthropic/messages/client_test.go
```

API:

```text
NativeClient implementing adapt.NativeClient[MessageRequest, Event]
```

Responsibilities:

```text
marshal MessageRequest to JSON
construct POST {baseURL}/v1/messages
set content-type, x-api-key, anthropic-version
apply static headers and HeaderFuncs
open the transport
read raw SSE blocks from ByteStream
decode blocks via SSEFrameDecoder
emit wire events on a channel
close stream resources on exit
```

Tests using `transport.FakeByteStreamTransport`:

```text
correct URL and method
required headers set
custom headers and HeaderFuncs applied
request body is valid JSON
decoded wire events emitted in order
transport errors propagate
ctx cancellation stops the stream loop
```

### Step 3.6: adapted Anthropic client (`unified.Client`)

Create:

```text
providers/anthropic/messages/adapted.go
providers/anthropic/messages/adapted_test.go
```

Responsibilities:

```text
wrap unified.Request in adapt.Request
run request processors
encode with codec
run provider request processors
call NativeClient
run provider-event processor chain
decode to unified.Event
run unified-event processor chain
convert internal errors to a final unified.ErrorEvent
```

Tests:

```text
request processors can mutate adapt.Request
provider request processors can mutate MessageRequest
provider event processors compose before decoding
unified event processors compose after decoding
internal errors are surfaced as ErrorEvent
```

### Step 3.7: Anthropic public constructor

Create:

```text
providers/anthropic/messages/options.go
providers/anthropic/messages/options_test.go
```

Local provider API only:

```text
type HeaderFunc func(context.Context, *http.Request) error
type Option interface { applyAnthropic(*Config) error }
type Config struct { ... }
```

Options to implement now:

```text
WithAPIKey(string)
WithBaseURL(string)
WithVersion(string)
WithBeta(string)
WithHeader(key, value string)
WithHeaderFunc(HeaderFunc)
WithTransport(transport.ByteStreamTransport)
WithRequestProcessor(adapt.RequestProcessor)
WithUnifiedEventProcessor(pipeline.Processor[unified.Event])
WithProviderRequestProcessor(adapt.ProviderRequestProcessor[MessageRequest])
WithProviderEventProcessor(pipeline.Processor[Event])
```

Constructor:

```text
NewClient(opts ...Option) (unified.Client, error)
```

Behavior:

```text
require API key
apply default base URL and Anthropic version
default transport to HTTPByteStreamTransport in SSE mode
wire all processors into the adapted client
```

Tests:

```text
API key required
base URL override works
version override works
beta header appended
custom transport replaces default
processors are wired into the right stage
```

### Step 3.8: end-to-end integration coverage

Create `providers/anthropic/messages/integration_test.go`.

Coverage:

```text
text response stream -> unified events -> Collect response
tool-use stream -> unified tool-call events -> Collect response
provider error stream -> ErrorEvent -> Collect returns error
TextCoalescer integration
CompletionInjector integration
```

Use `transport.FakeByteStreamTransport` with raw SSE blocks as inputs.

### Phase 3 checkpoint

Verify:

```text
go build ./...
go test ./...
go vet ./...
```

Done criteria:

```text
a caller can construct an Anthropic client and stream unified events
text-only and tool-use responses are covered
the phase 3 path does not depend on gateway or router code
all failures surface as normal Go errors or unified.ErrorEvent
```

---

## Immediate implementation order

This is the recommended coding order across the first three phases.

### Milestone A: core compile spine

Build in this order:

```text
1.1 unified core types
1.2 extensions
1.3 request + APIError + client
1.4 events
1.5 response + Collect
1.6 adapt types
1.7 pipeline primitives
1.8 built-in processors
```

Exit condition:

```text
`go test ./unified/... ./adapt/... ./pipeline/...` passes
```

### Milestone B: transport spine

Build in this order:

```text
2.1 transport types
2.2 SSE framing/parsing
2.3 NDJSON
2.4 HTTP transport
2.5 fake transport
2.6 rate-limit wrapper
2.7 retry wrapper
```

Exit condition:

```text
raw SSE event blocks can be read and parsed without provider code
```

### Milestone C: Anthropic text-only path

Build in this order:

```text
3.1 wire types
3.2 request codec for text/system only
3.3 SSE frame decoder
3.4 event decoder for text + usage + completion only
3.5 native client
3.6 adapted client
3.7 NewClient
3.8 integration test for text streaming
```

Exit condition:

```text
simple text prompt roundtrip works end to end
```

### Milestone D: Anthropic tool-use completion

Extend:

```text
3.2 tool definitions + tool results
3.4 tool-use event mapping + argument buffering + reasoning deltas
3.8 tool-use integration coverage
```

Exit condition:

```text
tool calls survive the roundtrip into unified.Response
```

---

## Dependency summary

```text
Phase 1
  1.1 core unified types
  1.2 extensions                <- 1.1
  1.3 request/error/client      <- 1.1, 1.2
  1.4 events                    <- 1.1
  1.5 response/collect          <- 1.1, 1.4
  1.6 adapt types/interfaces    <- 1.3, 1.4
  1.7 pipeline primitives       <- 1.6
  1.8 built-in processors       <- 1.4, 1.7

Phase 2
  2.1 transport types
  2.2 SSE framing/parsing       <- 2.1
  2.3 NDJSON                    <- 2.1
  2.4 HTTP transport            <- 2.1, 2.2, 2.3, 1.3
  2.5 fake transport            <- 2.1
  2.6 rate-limit wrapper        <- 2.1
  2.7 retry wrapper             <- 2.1, 1.3

Phase 3
  3.1 Anthropic wire types
  3.2 request codec             <- 3.1, 1.3, 1.6
  3.3 SSE frame decoder         <- 3.1, 2.2
  3.4 event decoder             <- 3.1, 1.4, 1.6
  3.5 native client             <- 3.1, 3.3, 2.4
  3.6 adapted client            <- 3.2, 3.4, 3.5, 1.7, 1.8
  3.7 constructor               <- 3.5, 3.6
  3.8 integration               <- 3.7, 2.5, 1.5
```
