# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions below are backfilled from the repository's implementation milestones. Tags
match these entries as the project starts publishing releases.

## [Unreleased]

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

[Unreleased]: https://github.com/codewandler/llmadapter/compare/v0.23.0...HEAD
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
