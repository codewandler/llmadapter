# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions below are backfilled from the repository's implementation milestones. Tags
match these entries as the project starts publishing releases.

## [Unreleased]

### Added

- Added endpoint request-body size limits with 413 responses for oversized gateway requests.
- Added gateway happy-path tests for Anthropic Messages and OpenAI Responses endpoints.

### Fixed

- Added HTTP gateway server timeouts, context-aware provider stream error sends, OpenAI Responses WebSocket session locking, default TLS 1.2 enforcement, and stricter transport error handling.

## [1.0.0-rc.9] - 2026-04-27

### Added

- Added strict `agentic_coding` conformance validation so approved compatibility rows must include live required-feature evidence plus explicit continuation and transport evidence.

### Changed

- `llmadapter conformance` now reports approved versus valid approved rows and exits non-zero when an approved `agentic_coding` row violates the required evidence contract.

## [1.0.0-rc.8] - 2026-04-27

### Added

- Added `llmadapter infer --interaction`, `--session`, and `--branch` for continuation/session diagnostics without making llmadapter own conversation state.
- Added typed Codex interaction/session/branch extension helpers for future WebSocket continuation work.
- Added a reusable WebSocket byte-stream transport and Codex session-mode WebSocket request path with pre-stream HTTP/SSE fallback.
- Added branch-safe Codex internal continuation state so WebSocket `previous_response_id` is only attached for same-session, same-branch, append-only continuations.
- Added `unified.ProviderExecutionEvent` so providers can report actual transport/internal continuation; mux route events and `infer` diagnostics now reflect Codex runtime WebSocket versus HTTP/SSE decisions.
- Added live `TEST_INTEGRATION` smoke coverage for Codex session-mode WebSocket continuation and WebSocket prompt-cache accounting, including runtime metadata assertions for WebSocket transport and internal `previous_response_id` reuse.
- Added deterministic Codex provider coverage for WebSocket enabled, disabled, missing stable session ID, fallback, retry-after-fallback, and mid-stream invalidation behavior.
- Added `responses.WithWebSocketMode(...)` with default, auto, enabled, and disabled modes for direct OpenAI Responses clients, plus deterministic coverage for default HTTP/SSE, WebSocket enabled, auto-with-cache-key, and auto fallback.
- Added a shared OpenAI Responses default WebSocket transport with compression enabled and IPv4 forced; Codex now uses the same default constructor.
- Extracted shared OpenAI Responses WebSocket session reuse/open-or-write mechanics for native OpenAI Responses and Codex.
- Added live OpenAI Responses WebSocket smoke coverage gated by `TEST_INTEGRATION=1` and OpenAI credentials.

### Fixed

- Hardened Codex WebSocket session invalidation after stale writes or incomplete streams so the next turn replays instead of using a response ID tied to a lost connection.
- Replaced length-only Codex WebSocket lineage checks with canonical per-input prefix hashes so rewritten branches cannot reuse the wrong internal response ID.
- Kept Codex WebSocket fallback scoped to the failed turn/session instead of permanently disabling WebSocket after a transient pre-stream failure.
- Applied context deadlines to WebSocket writes when the request context has a deadline.

### Migration Notes

- Agentsdk-style consumers should continue treating `codex_responses` as public replay/stateless. WebSocket transport and internal `previous_response_id` reuse are llmadapter implementation details; consumers may pass stable Codex session/branch/cache hints for optimization, but projection decisions should be based on `consumer_continuation`.
- OpenAI Responses has an official WebSocket mode. Direct `openai_responses` clients can opt in with `responses.WithWebSocketMode(...)`; provider descriptors, JSON config, auto mux, OpenRouter Responses, and workload compatibility evidence still default to HTTP/SSE.

## [1.0.0-rc.7] - 2026-04-27

### Added

- Added `llmadapter conformance` and a `conformance` package to report provider descriptors, endpoint evidence, warnings, and live use-case approval rows from the compatibility artifact.
- Added explicit continuation and transport metadata across provider descriptors, route events, config inspection, model resolution, compatibility artifacts, conformance output, and CLI diagnostics.

### Fixed

- Stopped forwarding unsupported `previous_response_id` continuation hints to the Codex HTTP/SSE backend and emit a canonical warning instead.
- Moved the canonical Responses provider wire implementation to `providers/openai/responses`; OpenRouter Responses now layers OpenRouter-only body extensions on the OpenAI Responses base, and Codex Responses depends on OpenAI Responses rather than OpenRouter Responses.

## [1.0.0-rc.6] - 2026-04-26

### Added

- Added compatibility artifact helpers and `llmadapter compatibility-record` to regenerate the use-case matrix from the JSON evidence artifact.
- Added optional live e2e artifact writing through `LLMADAPTER_COMPAT_ARTIFACT`.

## [1.0.0-rc.5] - 2026-04-26

### Added

- Added modeldb runtime-view based use-case selection APIs for consumers that need approved provider/model/API choices.
- Added compatibility evidence loading and `llmadapter resolve --approved-only` for strict workload-approved model resolution.

## [1.0.0-rc.4] - 2026-04-26

### Added

- Added OpenRouter Kimi K2.6 to the live agentic-coding compatibility matrix.
- Added per-candidate and total timing evidence to the agentic-coding compatibility artifact and result table.

### Changed

- Made cache accounting a required feature for the `agentic_coding` compatibility profile.
- Updated the live agentic-coding matrix so every approved row must report cache read/write token evidence.

### Fixed

- Decoded OpenRouter Responses cache usage from Chat/Completions-style `prompt_tokens_details` fields as well as Responses-style `input_tokens_details` fields.

## [1.0.0-rc.3] - 2026-04-26

### Added

- Added a `compatibility` package for workload profiles, feature evidence, and candidate evaluation.
- Added `adapterconfig` compatibility helpers so library consumers can evaluate or filter existing model-resolution candidates by use case.
- Added `llmadapter compatibility` plus `llmadapter resolve --use-case` to inspect agentic-coding and summarization suitability from the same adapterconfig/modeldb resolution path.
- Added `docs/USE_CASE_MATRIX.md` and an implementation plan for live use-case compatibility certification.
- Added live `TestUseCaseAgenticCoding` e2e coverage and recorded the first agentic-coding compatibility evidence artifact.

## [1.0.0-rc.2] - 2026-04-26

### Added

- Reworked adoption documentation with a product-focused README and new guides for getting started, CLI usage, configuration, library usage, and provider development.
- Added `examples/README.md` to explain the shipped example config and overlay.

### Changed

- Reframed DESIGN, PLAN, API surface, provider matrix, AGENTS, and provider-extension skill wording for the v1 release-candidate state.

## [1.0.0-rc.1] - 2026-04-25

### Added

- Cut the first v1 release candidate for llmadapter's stable stateless adapter/gateway/mux surface.
- Added final release-candidate documentation tying together the v1 provider matrix, public API surface, architecture review, examples, and verification status.
- Tightened the live tool-continuation smoke prompt so providers are forced into the initial tool call through `ToolChoice` without carrying an instruction that discourages the final post-tool answer.

### Stable Surface

- Canonical `unified.Request` / `unified.Event` model with stream-first provider clients.
- HTTP gateway endpoints for OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages compatibility surfaces.
- In-process mux client and auto-detected mux construction through the same `adapterconfig` and modeldb-backed resolution path as the gateway and CLI.
- Provider endpoint support for Anthropic, Claude Code-compatible access, OpenAI Chat, OpenAI Responses, Codex Responses, OpenRouter Chat/Responses/Messages, and MiniMax Chat/Messages.
- Explicit usage, pricing, prompt-cache intent, reasoning signatures, citations, provider errors, raw provider metadata, and namespaced extension primitives.

### Verification

- Mandatory local test, vet, and build gates pass.
- Full available `TEST_INTEGRATION=1` e2e provider smoke matrix passes with local credentials available.
- Docker image build passes.

## [0.48.31] - 2026-04-25

### Added

- Added package documentation for the primary public API packages.
- Added `docs/API_SURFACE.md` documenting stable consumer packages, extension packages, internal packages, and pre-v1 stability rules.

### Changed

- Recorded the public API/package-boundary freeze in `PLAN.md`; no pre-v1 exported renames are required from the current surface.

## [0.48.30] - 2026-04-25

### Added

- Added `docs/PROVIDER_MATRIX.md` documenting the v1 provider endpoints, credential triggers, feature coverage, live smoke commands, skip behavior, and latest full matrix result.
- Recorded the full live e2e matrix command in `PLAN.md` and linked the provider matrix from README.

### Changed

- MiniMax Messages cache controls remain mapped, but provider-reported prompt-cache accounting is no longer advertised as v1 live-smoke verified because the provider response did not report cache write/read counters in the full matrix run.
- Narrowed the v1 provider-matrix gap to the remaining public API/package-boundary freeze.

## [0.48.29] - 2026-04-25

### Added

- Added a load-tested `examples/llmadapter.example.json` config plus modeldb overlay covering provider endpoints, dynamic model routing, aliases, capability overrides, pricing metadata, and route attempt limits.
- Added README examples for auto mux inference, config-driven mux inference, model resolution, gateway serving, Docker startup, and provider identity inspection.
- Added regression coverage that the public example config loads, validates, and produces inspectable modeldb-backed route metadata.

### Changed

- Narrowed the v1 CLI/config/examples gap by making the documented outside-in usage paths executable from repository examples.

## [0.48.28] - 2026-04-25

### Added

- Added capability provenance fields to config inspection and model resolution diagnostics.
- `llmadapter resolve` now reports whether capabilities come from provider descriptor defaults, config overrides, or modeldb exposure metadata.
- Added regression tests for capability source reporting across provider defaults, explicit config overrides, modeldb-backed routes, and CLI output.

### Changed

- Narrowed the v1 capability/model policy gap by making default-versus-catalog-confirmed capability decisions inspectable.

## [0.48.27] - 2026-04-25

### Added

- Added shared route-attempt retryability classification for unsupported canonical fields and 400/422 provider API errors.
- Added optional gateway `max_attempts` config and `muxclient.WithMaxAttempts` for deterministic route fallback limits.
- Added gateway, muxclient, and routeattempt tests for retry limit exhaustion and non-retryable request validation failures.

### Changed

- Gateway and mux fallback now share explicit max-attempt and non-retryable-failure policy while preserving the gateway-only response-start boundary.

## [0.48.26] - 2026-04-25

### Added

- Added endpoint decode edge-case fixtures for OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages compatibility surfaces.
- Added provider reasoning conformance fixtures for Anthropic-family initial thinking blocks and Responses-family `response.reasoning_text.delta` streams.
- Added citation metadata variant fixtures across Responses-family and Anthropic-family providers.
- Added message-only stream error fixtures for Chat and Responses-family providers.

### Changed

- Narrowed the v1 conformance gap to future provider variants rather than currently known fixture classes.

## [0.48.25] - 2026-04-25

### Changed

- Added the v1 completion roadmap to `PLAN.md`, including mandatory phase-by-phase tasks, verification, changelog, release, and v1 promotion criteria.
- Refreshed README and architecture docs to separate v1 blockers, v1 non-blockers, and post-v1 expansion work.
- Updated the implementation status to reflect completed provider registry, Anthropic wire cleanup, extension validation, Codex parity, and conformance hardening slices.

## [0.48.24] - 2026-04-25

### Changed

- Tightened semantic validation for OpenAI Responses cache retention, OpenRouter model/route/provider/plugin/session controls, Anthropic beta header values, and Codex turn metadata.
- Added provider encode/reject coverage so invalid semantic values are dropped with `invalid_extension_dropped` warnings where safe, while Codex rejects unsafe header/metadata controls before transport.
- Updated README, architecture notes, and PLAN status for current extension validation coverage.

## [0.48.23] - 2026-04-25

### Added

- Added endpoint and provider fixtures that pin unsupported audio/video/file/document content and built-in tool policy.

### Changed

- Confirmed supported images are preserved while unsupported media and built-in tools warn/drop or reject instead of leaking into provider wire payloads.

## [0.48.22] - 2026-04-25

### Added

- Added endpoint codec conformance fixtures for HTTP/raw decode metadata preservation.
- Added projection coverage for canonical citations back into OpenAI Responses annotations and Anthropic Messages citation fields.

## [0.48.21] - 2026-04-25

### Added

- Added citation conformance fixtures for Responses-family annotation events and Anthropic-family text-block citations.
- Canonical citation events preserve URL/title/text/ranges/document IDs and unknown citation metadata where providers expose them.

## [0.48.20] - 2026-04-25

### Added

- Added provider error conformance fixtures for mid-stream API errors, Responses response-object failures, and non-2xx error body variants.

### Changed

- Provider raw payloads are preserved on normalized provider errors where available.

## [0.48.19] - 2026-04-25

### Changed

- Refreshed stable implementation status after the conformance hardening rounds.

## [0.48.18] - 2026-04-25

### Added

- Added raw provider usage payload preservation on canonical usage events across OpenAI Chat, Responses-family, Codex Responses, and Anthropic-family Messages paths.
- Added canonical `RawEvent` preservation for selected unmapped provider stream events.

## [0.48.17] - 2026-04-25

### Added

- Added multimodal conformance fixtures for supported image URL/base64 encoding and unsupported audio/file/video warning behavior.

## [0.48.16] - 2026-04-25

### Added

- Added stream error conformance fixtures across Anthropic-family Messages, OpenAI Chat-compatible, OpenAI Responses-compatible, OpenRouter, MiniMax, and Codex paths.

### Changed

- Shared HTTP transport now normalizes numeric provider error codes and MiniMax `base_resp` error bodies.

## [0.48.15] - 2026-04-25

### Added

- Added shared reasoning stream projection fixtures for Anthropic Messages, MiniMax Messages, OpenRouter Messages, OpenAI Responses, OpenRouter Responses, and Codex Responses.
- `unified.Collect` preserves citation events and raw provider events alongside content, usage, warnings, and finish metadata.

## [0.48.14] - 2026-04-25

### Added

- Added typed extension validation for mature OpenAI Responses, OpenRouter, Anthropic, and Codex extension groups.
- Provider encoders emit `invalid_extension_dropped` warnings for invalid extension controls.

## [0.48.13] - 2026-04-25

### Added

- Added focused conformance tests for Responses mid-stream provider errors.
- Added dynamic model resolver rejection coverage for unavailable catalog models.

## [0.48.12] - 2026-04-25

### Changed

- Shared gateway and muxclient route candidate lookup, native model rewrite, and provider/API error formatting through `internal/routeattempt`.

## [0.48.11] - 2026-04-25

### Added

- Added provider conformance hardening around error normalization and modeldb-backed dynamic route behavior.

## [0.48.10] - 2026-04-25

### Added

- Added `unified.CodexExtensions` for Codex session/window/turn metadata headers.
- Codex transport validates and applies typed Codex extensions without changing default cache-key behavior.

## [0.48.9] - 2026-04-25

### Added

- Canonical reasoning content/events now preserve provider signatures for Anthropic-family thinking continuations.
- Anthropic, Claude-compatible, OpenRouter Messages, and MiniMax Messages decode/encode thinking signatures.

## [0.48.8] - 2026-04-25

### Added

- Responses-family providers now stream canonical reasoning summaries when upstream events expose them.

## [0.48.7] - 2026-04-25

### Changed

- Centralized modeldb-backed model resolution across CLI resolve/infer, auto mux, gateway, and muxclient paths.
- Dynamic model and alias resolution now use the catalog plus overlays as the single model existence source when modeldb is enabled.

## [0.48.6] - 2026-04-25

### Fixed

- Improved `llmadapter infer` defaults and routing behavior, including defaulting inference to the `haiku` intent.

## [0.48.5] - 2026-04-25

### Added

- Added canonical cache primitives on `unified.Request`: `CachePolicy`, `CacheKey`, and `CacheTTL`.
- Added `TextPart.CacheControl` request hints for Anthropic-family block caching.

### Changed

- Cleaned up auto-routing/model resolution behavior around cache-capable provider paths.

## [0.48.4] - 2026-04-24

### Changed

- Normalized Claude-compatible provider naming by using `claude` as the canonical provider type (replacing `claude_messages`) across descriptors, auto-routing, gateway/runtime pathing, and CLI/provider resolution.
- Removed the separate instance naming layer so provider identity is consistently derived from provider type while preserving local Claude credential-based auto-enable behavior.
- Updated resolve/output test snapshots, default docs, and e2e smoke/gateway provider labels to match the new naming model.

## [0.48.3] - 2026-04-24

### Added

- `llmadapter resolve` now shows ranked candidates by default.

## [0.48.2] - 2026-04-24

### Changed

- Documented the Anthropic client construction refactor.

## [0.48.1] - 2026-04-24

### Changed

- Internal refactor: `providerregistry` now builds both `anthropic` and `claude` clients through a shared Anthropic client construction path while preserving Claude-compatible options as a provider variant.

## [0.48.0] - 2026-04-24

### Added

- Added central modeldb aliases for Claude-family `haiku`, `sonnet`, and `opus` provider-local routing, with `sonnet` and `opus` pinned to Claude 4.6.
- `AutoOptions.ModelDBAliases` can inject or override auto mux model aliases from callers.
- MiniMax Chat now advertises canonical tool support after live tool-use and tool-result continuation validation.

## [0.47.0] - 2026-04-24

### Added

- Added `TEST_INTEGRATION`-gated reasoning stream smoke coverage for Anthropic, Claude Code-compatible Messages, MiniMax Messages, and OpenRouter Messages.
- MiniMax Messages now advertises canonical reasoning capability.

## [0.46.0] - 2026-04-24

### Added

- Anthropic-compatible providers now encode canonical reasoning requests as Anthropic extended thinking with request-side temperature/top-k compatibility handling.
- Anthropic, Claude Code-compatible, and OpenRouter Messages endpoints now advertise reasoning capability.
- Auto mux model intents with `UseModelDB` now prefer a provider whose catalog can resolve the requested intent, so aliases like `opus` can route to Claude-compatible endpoints instead of a default OpenAI model.

## [0.45.0] - 2026-04-24

### Added

- Dynamic model routes now enrich usage costs per request when the selected provider-native model has modeldb pricing.
- Added unit coverage for dynamic-route modeldb activation and request-scoped pricing in both shared adapter config and the compatibility gateway path.

## [0.44.0] - 2026-04-24

### Fixed

- Auto mux configuration now keeps dynamic model passthrough routes when explicit intent routes are configured.

## [0.43.0] - 2026-04-24

### Added

- Added explicit `dynamic_models` route support for pass-through access to arbitrary provider-native model IDs.
- Added auto-config dynamic route generation behind `adapterconfig.AutoOptions.DynamicModels`.

## [0.42.0] - 2026-04-24

### Added

- Added `AutoResult.RouteSummary` for consumer-facing summaries of auto-detected mux route selection, including provider, provider API, public model, native model, and enabled-provider reason.

## [0.41.0] - 2026-04-24

### Added

- Added `codex_responses` provider support for Codex/ChatGPT OAuth-backed Responses requests through API kind `codex.responses`.
- Added Codex env/local credential auto-detection, Codex modeldb service identity, and shared live smoke matrix entries.

## [0.40.0] - 2026-04-24

### Added

- Added `llmadapter models --catalog` for modeldb-backed catalog inspection with service/API/parameter/model filters and optional per-offering output.

## [0.39.0] - 2026-04-24

### Added

- Added `llmadapter providers --auto` for redacted auto-detected provider credential status.
- Added `llmadapter providers --status --config <path>` for redacted configured provider credential status.

## [0.38.0] - 2026-04-24

### Changed

- `cmd/llmadapter-gateway` now runs through the shared `adapterconfig` and `gatewayserver` path used by `llmadapter serve`, while preserving the compatibility binary.

## [0.37.0] - 2026-04-24

### Added

- Added canonical `unified.RouteEvent` metadata for selected mux routes.
- `muxclient` now emits a `RouteEvent` before provider response events so library consumers such as miniagent can observe source API, target API/family, provider, public model, and native model.

## [0.36.0] - 2026-04-24

### Added

- Added `llmadapter serve` to run the compatibility gateway from the Cobra CLI using the shared `adapterconfig` router construction path.
- Added a Dockerfile for building a container image that runs `llmadapter serve`.

### Changed

- Pinned `github.com/codewandler/modeldb` to `v0.13.0` and removed the local replace so standalone Docker builds work from a normal repository checkout.
- Anthropic-family provider registry clients now force stream-first upstream requests, matching the gateway's SSE-based provider decoding path.

## [0.35.0] - 2026-04-24

### Added

- Added `llmadapter resolve <model>` to explain configured or auto-detected route selection, provider endpoint metadata, native model mapping, modeldb service identity, and capabilities.

## [0.34.0] - 2026-04-24

### Added

- Migrated `cmd/llmadapter` to Cobra for a CLI shape closer to `../llmproviders/llmcli`.
- Added `llmadapter routes` and `llmadapter models` inspection commands for configured or auto-detected mux routes.

## [0.33.0] - 2026-04-24

### Added

- Added `adapterconfig.AutoMuxClient` for constructing a stateless mux client from detected environment credentials and local Claude Code OAuth credentials.
- Added `llmadapter smoke -mode auto` to exercise the auto-detected mux path from the CLI.

## [0.32.0] - 2026-04-24

### Added

- Added `adapterconfig` for public llmadapter JSON config loading/defaulting/validation and config-driven mux client construction.
- `adapterconfig.NewMuxClient` now builds provider endpoints, resolves modeldb aliases, applies modeldb capability metadata, and wraps pricing processors.
- `llmadapter smoke -mode mux` now accepts `-config` for config-driven in-process routing.

## [0.31.0] - 2026-04-24

### Added

- Added a stateless `muxclient` package implementing `unified.Client` over router/provider endpoints.
- Added mux-routed mode to the `llmadapter smoke` command.
- Documented the planned modeldb/config expansion for the mux client layer.

## [0.30.0] - 2026-04-24

### Added

- Added a shared `providerregistry` package for provider endpoint descriptors and direct client construction.
- Added an initial `llmadapter` CLI with `providers` and direct-provider `smoke` commands.

## [0.29.0] - 2026-04-24

### Added

- Added a native `openai_responses` provider endpoint backed by the OpenAI Responses API.

### Changed

- Clarified that stateful conversation/session ownership belongs above llmadapter, for example in `agentsdk`; llmadapter owns only stateless request/event/provider primitives.

## [0.28.0] - 2026-04-24

### Added

- Added canonical `TextPart.CacheControl` hints and Anthropic-family block-level `cache_control` request encoding.
- Added live prompt-cache smoke coverage for Anthropic and Claude Code-compatible access, checking provider-reported cache write/read token usage.
- Added OpenAI Responses-compatible continuation and prompt-cache extension keys, with `/v1/responses` endpoint decode and OpenRouter Responses provider encode support.
- Added an e2e smoke harness for Responses `previous_response_id` continuation; no current provider is flagged until a backend has live verified continuation semantics.
- Claude Code request preflight now uses Anthropic system blocks and applies cache control to the last system text block.

## [0.27.0] - 2026-04-24

### Added

- Added `claude` as a Claude Code-compatible Anthropic Messages provider endpoint with bearer/local OAuth auth, Claude CLI headers, `beta=true` request shaping, and request preflight metadata.
- Claude Code-compatible providers now default `modeldb_service_id` to `anthropic` so fixed-route pricing and capability metadata continue to use Anthropic Claude offerings.
- Added `claude` to the shared live text/tool/gateway smoke matrices, gated by local Claude Code OAuth credentials.
- Added a default decompressing HTTP transport that advertises and decodes `gzip`, `deflate`, `br`, and `zstd` response compression.

## [0.26.0] - 2026-04-24

### Added

- Gateway config now supports `modeldb.catalog_path` and `modeldb.overlay_paths` for operator-supplied catalog bases and overlays.
- Gateway routes can now set `modeldb_model` to resolve catalog aliases or local `modeldb.aliases` into explicit native/modeldb wire model IDs.

## [0.25.0] - 2026-04-24

### Added

- Documented the next highest-priority planning track for `modeldb` catalog integration, Claude OAuth compatibility, structured usage/pricing, prompt caching, optional conversation/session support, and a repo-native CLI.
- Replaced duplicated canonical usage counters with structured token and cost items; endpoint codecs now derive flat API-specific usage counters at the wire boundary.
- Added a `pricing` package with a modeldb-backed event processor for enriching canonical usage events with cost items.
- Gateway config can now wire modeldb-backed usage cost enrichment for fixed-model routes using provider `modeldb_service_id` and route `modeldb_wire_model_id`.
- Added a `modelmeta` package and gateway wiring to narrow fixed-route capabilities and attach token limits from modeldb offering exposures.
- Added `llmadapter-gateway -inspect-config` for credential-free JSON inspection of resolved providers, routes, capabilities, limits, modeldb metadata, and pricing availability.
- Anthropic, OpenAI Chat, and OpenRouter Responses provider decoders now emit structured token categories where upstream usage details are available.

## [0.24.0] - 2026-04-24

### Added

- Gateway config now supports `health_cooldown` and provider-level capability overrides for model-specific routing corrections.

### Changed

- Gateway health deprioritization is keyed by provider/API/model instead of only provider/API.
- Gateway provider API-key resolution now uses a shared helper across provider types.
- OpenRouter raw extension preservation now uses shared `unified` helpers across endpoint and provider codecs.
- Anthropic Messages endpoint decoding now preserves mixed user text/tool-result content instead of dropping the non-tool-result content.

## [0.23.0] - 2026-04-24

### Added

- **Gateway health tracking** - Gateway handlers now mark failed provider endpoints unhealthy for a cooldown window and temporarily deprioritize them while keeping them as last-resort candidates.

## [0.22.0] - 2026-04-24

### Added

- **Tool argument decode hardening** - OpenAI Chat and OpenAI Responses endpoint codecs now replace malformed tool-call argument JSON with `{}` and retain decode warnings instead of carrying invalid JSON forward.

## [0.21.0] - 2026-04-24

### Added

- **Provider image input passthrough** - OpenAI Chat-compatible providers and OpenRouter Responses now encode canonical image inputs upstream, and gateway metadata advertises vision capability for image-capable endpoint families.

## [0.20.0] - 2026-04-24

### Added

- **Endpoint image input decode** - OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs now decode supported image URL/base64/file references into canonical `unified.ImagePart` values instead of dropping them.

## [0.19.0] - 2026-04-24

### Added

- **OpenAI Responses structured output mapping** - OpenAI Responses endpoint decoding and OpenRouter Responses provider encoding now preserve `text.format` JSON mode and JSON schema requests, with matching gateway capability metadata.

## [0.18.0] - 2026-04-24

### Added

- **OpenAI Chat structured output mapping** - OpenAI Chat-compatible endpoint decoding and provider encoding now preserve `response_format` JSON mode and JSON schema requests, and gateway metadata advertises those capabilities for OpenAI Chat and OpenRouter Chat endpoints.

## [0.17.0] - 2026-04-24

### Added

- **Gateway provider fallback** - Gateway handlers now try ordered route candidates and fall back to lower-ranked providers when the selected provider fails before response bytes are written.

## [0.16.0] - 2026-04-24

### Added

- **Endpoint decode warnings** - OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages endpoint codecs now store best-effort decode warnings on `adapt.Request` when unsupported inbound fields are dropped.
- **OpenRouter extension passthrough** - Endpoint codecs preserve OpenRouter routing/provider/plugin/debug/trace/session controls in `unified.Request.Extensions`, and OpenRouter Chat, Responses, and Messages providers encode those controls back to upstream requests.
- **Weighted route ranking** - Static routing now ranks compatible routes by route `weight`, then provider endpoint `priority`, while still falling back past capability mismatches.

## [0.15.0] - 2026-04-24

### Added

- **OpenAI-family mapping warnings** - OpenAI Chat, OpenRouter Chat, and OpenRouter Responses provider mappings now emit canonical warnings when non-text content or unsupported tool kinds are dropped.

## [0.14.0] - 2026-04-24

### Added

- **Best-effort mapping warnings** - Anthropic-family provider request mapping now emits canonical warnings when unsupported fields are dropped in best-effort mode.

## [0.13.0] - 2026-04-24

### Added

- **Capability-aware routing** - Static routing now skips endpoints that cannot satisfy required request capabilities such as streaming, tools, JSON mode, JSON schema, reasoning, vision, or audio input.

## [0.12.0] - 2026-04-24

### Added

- **OpenAI Responses endpoint** - Added a downstream `/v1/responses` gateway codec for text and function-tool requests.
- **OpenAI Responses gateway route** - Wired `/v1/responses` into the gateway command and default route set.
- **OpenAI Responses gateway smokes** - Added live gateway smoke coverage against OpenRouter Responses.

## [0.11.0] - 2026-04-24

### Added

- **Anthropic Messages endpoint** - Added a downstream `/v1/messages` gateway codec for Anthropic-compatible requests and responses.
- **Anthropic Messages gateway route** - Wired `/v1/messages` into the gateway command and default Anthropic route set.
- **Anthropic Messages gateway smokes** - Added live gateway smoke coverage for Anthropic, OpenRouter Messages, and MiniMax Messages upstreams.

## [0.10.1] - 2026-04-24

### Added

- **Provider error tests** - Added focused OpenAI-compatible and Anthropic-compatible mid-stream error tests.

### Changed

- **Gateway reasoning encoding** - OpenAI Chat Completions gateway responses now expose canonical reasoning as `reasoning_details` instead of mixing it into assistant `content`.
- **Provider HTTP errors** - Non-2xx HTTP transport errors now parse common OpenAI-style and Anthropic-style JSON bodies into structured `unified.APIError` fields.

## [0.10.0] - 2026-04-24

### Added

- **MiniMax Anthropic-compatible Messages provider** - Added a MiniMax Messages provider wrapper with default `https://api.minimax.io/anthropic` routing.
- **MiniMax Messages gateway registration** - Added `minimax_messages` as a gateway provider endpoint type in the Anthropic Messages family.
- **MiniMax Messages smoke coverage** - Added shared text, tool-use, tool-result continuation, and gateway e2e smoke entries gated by `MINIMAX_API_KEY` or `MINIMAX_KEY`.

### Changed

- **MiniMax plan status** - Updated provider planning to keep downstream endpoint codecs and broader MiniMax conformance as explicit follow-up slices.
- **MiniMax e2e budget** - Added a provider-specific token budget for MiniMax Messages because it streams reasoning before final text.

## [0.9.0] - 2026-04-24

### Added

- **MiniMax Chat Completions provider** - Added a MiniMax OpenAI-compatible Chat Completions provider wrapper with default `https://api.minimax.io` routing.
- **MiniMax gateway registration** - Added `minimax_chat` as a gateway provider endpoint type in the OpenAI Chat Completions family.
- **MiniMax Chat smoke coverage** - Added shared text and gateway e2e smoke entries gated by `MINIMAX_API_KEY` or `MINIMAX_KEY`.

### Changed

- **MiniMax plan status** - Updated provider planning to keep MiniMax tools and Anthropic-compatible Messages as explicit follow-up slices.

## [0.8.0] - 2026-04-24

### Added

- **Agent onboarding** - Added minimal `README.md` and `AGENTS.md` files for repository orientation.
- **Provider extension skill** - Added `.agents/skills/llmadapter-provider-extension/SKILL.md` to guide future provider, API kind, endpoint codec, and e2e smoke extensions.
- **MiniMax planning** - Added MiniMax as the next planned provider integration, including notes for its OpenAI-compatible chat and Anthropic-compatible messages surfaces.

### Changed

- **Project plan status** - Updated `PLAN.md` with current implementation progress, technical debt, test gaps, and next provider-support work.
- **Changelog policy** - Introduced this versioned changelog as the source of milestone-level release history.

## [0.7.0] - 2026-04-24

### Added

- **OpenRouter Responses tool calls** - Added streaming function-call decoding for the OpenRouter Responses endpoint.
- **OpenRouter tool-result continuation** - Added request encoding for continuing OpenRouter Responses conversations with tool results.

### Changed

- **Capability metadata** - Marked OpenRouter Responses as tool-call capable so shared smoke tests and routing decisions can target it correctly.

## [0.6.0] - 2026-04-24

### Added

- **OpenRouter Chat Completions provider** - Added OpenRouter's `/api/v1/chat/completions` support as an OpenAI Chat Completions-family endpoint.
- **OpenRouter Responses provider** - Added OpenRouter's `/api/v1/responses` support as an OpenAI Responses-family endpoint.
- **OpenRouter Messages provider** - Added OpenRouter's Anthropic-compatible `/api/v1/messages` support as an Anthropic Messages-family endpoint.
- **Provider endpoint model** - Added explicit provider endpoint registration with provider name, API kind, API family, client, and capabilities.

### Changed

- **Routing model** - Shifted routing from provider-only targets to provider endpoint targets, allowing one provider to expose multiple protocol shapes.
- **OpenRouter architecture** - Modeled OpenRouter as one upstream provider with multiple API kinds instead of collapsing it into a single provider-specific API kind.

## [0.5.0] - 2026-04-24

### Added

- **OpenAI Chat Completions provider** - Added OpenAI `/v1/chat/completions` request construction and stream decoding.
- **OpenAI API key lookup** - Added support for `OPENAI_API_KEY` and `OPENAI_KEY` environment variable sources.
- **OpenAI gateway smoke coverage** - Added live gateway route coverage for OpenAI-backed requests.
- **OpenAI tool-call streaming** - Added streamed OpenAI tool-call delta decoding into canonical tool-call events.
- **Tool-result continuation smoke coverage** - Added live e2e coverage for sending tool results back through supported tool-capable providers.

### Changed

- **Shared smoke matrix** - Expanded provider smoke tests so the same text streaming, tool-call, and continuation checks can run across multiple providers.

## [0.4.0] - 2026-04-24

### Added

- **Chat Completions gateway** - Added an OpenAI-compatible `/v1/chat/completions` gateway endpoint codec and minimal HTTP handler.
- **Gateway smoke tests** - Added live streaming and non-streaming smoke coverage for gateway requests.
- **Static router** - Added source API and model matching with native upstream model rewrite support.
- **Gateway config** - Added minimal JSON config loading for providers, routes, API keys, API key environment variables, provider APIs, and model overrides.

### Changed

- **Gateway execution path** - Connected inbound endpoint codecs, static routing, provider clients, canonical events, and response encoding into a working outside-in flow.

## [0.3.0] - 2026-04-24

### Added

- **Core adapter vertical slice** - Implemented the first working path across `unified`, `adapt`, `pipeline`, `transport`, and Anthropic Messages.
- **Canonical event collection** - Added `unified.Collect` for turning streamed events into a final canonical response.
- **Transport primitives** - Added SSE, NDJSON, HTTP, fake transport, retry, and rate-limit primitives.
- **Anthropic Messages provider** - Added request encoding and stream decoding for Anthropic Messages.
- **Live e2e smoke harness** - Added `TEST_INTEGRATION`-gated live smoke tests that skip when required API keys are unavailable.

### Changed

- **Test entrypoint** - Established `tests/e2e` as the outside-in integration test location.

## [0.2.0] - 2026-04-24

### Added

- **Implementation plan** - Added `PLAN.md` with concrete phases for the initial adapter implementation, gateway work, provider support, and integration testing.

## [0.1.0] - 2026-04-24

### Added

- **Initial architecture** - Added `DESIGN.md` covering the canonical request/event model, adapters, provider clients, stream pipeline, routing, capabilities, transports, and testing strategy.

### Changed

- **Design review amendments** - Refined the architecture with provider endpoint modeling, canonical lossiness expectations, extension handling, and routing considerations.

[Unreleased]: https://github.com/codewandler/llmadapter/compare/v0.48.0...HEAD
[0.48.0]: https://github.com/codewandler/llmadapter/compare/v0.47.0...v0.48.0
[0.47.0]: https://github.com/codewandler/llmadapter/compare/v0.46.0...v0.47.0
[0.46.0]: https://github.com/codewandler/llmadapter/compare/v0.45.0...v0.46.0
[0.45.0]: https://github.com/codewandler/llmadapter/compare/v0.44.0...v0.45.0
[0.44.0]: https://github.com/codewandler/llmadapter/compare/v0.43.0...v0.44.0
[0.43.0]: https://github.com/codewandler/llmadapter/compare/v0.42.0...v0.43.0
[0.42.0]: https://github.com/codewandler/llmadapter/compare/v0.41.0...v0.42.0
[0.41.0]: https://github.com/codewandler/llmadapter/compare/v0.40.0...v0.41.0
[0.40.0]: https://github.com/codewandler/llmadapter/compare/v0.39.0...v0.40.0
[0.39.0]: https://github.com/codewandler/llmadapter/compare/v0.38.0...v0.39.0
[0.38.0]: https://github.com/codewandler/llmadapter/compare/v0.37.0...v0.38.0
[0.37.0]: https://github.com/codewandler/llmadapter/compare/v0.36.0...v0.37.0
[0.36.0]: https://github.com/codewandler/llmadapter/compare/v0.35.0...v0.36.0
[0.35.0]: https://github.com/codewandler/llmadapter/compare/v0.34.0...v0.35.0
[0.34.0]: https://github.com/codewandler/llmadapter/compare/v0.33.0...v0.34.0
[0.33.0]: https://github.com/codewandler/llmadapter/compare/v0.32.0...v0.33.0
[0.32.0]: https://github.com/codewandler/llmadapter/compare/v0.31.0...v0.32.0
[0.31.0]: https://github.com/codewandler/llmadapter/compare/v0.30.0...v0.31.0
[0.30.0]: https://github.com/codewandler/llmadapter/compare/v0.29.0...v0.30.0
[0.29.0]: https://github.com/codewandler/llmadapter/compare/v0.28.0...v0.29.0
[0.28.0]: https://github.com/codewandler/llmadapter/compare/v0.27.0...v0.28.0
[0.27.0]: https://github.com/codewandler/llmadapter/compare/v0.26.0...v0.27.0
[0.26.0]: https://github.com/codewandler/llmadapter/compare/v0.25.0...v0.26.0
[0.25.0]: https://github.com/codewandler/llmadapter/compare/v0.24.0...v0.25.0
[0.24.0]: https://github.com/codewandler/llmadapter/compare/v0.23.0...v0.24.0
[0.23.0]: https://github.com/codewandler/llmadapter/compare/v0.22.0...v0.23.0
[0.22.0]: https://github.com/codewandler/llmadapter/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/codewandler/llmadapter/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/codewandler/llmadapter/compare/v0.19.0...v0.20.0
[0.19.0]: https://github.com/codewandler/llmadapter/compare/v0.18.0...v0.19.0
[0.18.0]: https://github.com/codewandler/llmadapter/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/codewandler/llmadapter/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/codewandler/llmadapter/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/codewandler/llmadapter/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/codewandler/llmadapter/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/codewandler/llmadapter/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/codewandler/llmadapter/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/codewandler/llmadapter/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/codewandler/llmadapter/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/codewandler/llmadapter/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/codewandler/llmadapter/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/codewandler/llmadapter/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/codewandler/llmadapter/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/codewandler/llmadapter/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/codewandler/llmadapter/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/codewandler/llmadapter/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/codewandler/llmadapter/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/codewandler/llmadapter/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/codewandler/llmadapter/releases/tag/v0.1.0
