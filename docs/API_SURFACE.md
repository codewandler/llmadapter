# Public API Surface

This document records the v1 package-boundary decision for llmadapter. It is intended to prevent accidental public API expansion after the release-candidate baseline.

## Stable Consumer Packages

These packages are the primary v1 surface for library consumers:

- `unified`: canonical request, response, event, content, tool, usage, cache, extension, and `unified.Client` types.
- `adapterconfig`: config loading/defaulting/validation, auto credential detection, modeldb-backed resolution, modeldb runtime-view use-case selection, router construction, and mux client construction.
- `muxclient`: stateless in-process client over the router/provider endpoint path.
- `router`: provider endpoint metadata, route definitions, capability checks, model resolution hooks, and deterministic candidate ranking.
- `providerregistry`: provider endpoint descriptors and descriptor-backed client construction.
- `gatewayserver`: shared HTTP server wiring used by `llmadapter serve` and `cmd/llmadapter-gateway`.
- `compatibility`: workload profile definitions and candidate evaluation for use cases such as agentic coding.
- `diagnostics`: redacted provider transport diagnostics for library consumers that need HTTP/SSE or WebSocket request, response, stream, event, and transport-mode logging.

## Extension Packages

These packages are public because advanced users and provider implementers need them:

- `adapt`: API kind/family identity, mapping warnings, request envelopes, and codec interfaces.
- `transport`: byte-stream transport, HTTP transport, SSE/NDJSON readers, retry/rate-limit wrappers, fake transports, and decompression-aware HTTP clients.
- `pricing`: modeldb-backed usage cost enrichment.
- `modelmeta`: modeldb exposure to capability/limit metadata mapping.
- `anthropicwire`: neutral Anthropic Messages wire structs shared by downstream endpoint codecs and upstream provider mappings.
- `endpoints/*`: downstream compatibility codecs for OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages.
- `providers/*`: concrete provider clients and provider-specific options for direct use or registry construction.

## Internal Packages

These packages are intentionally not public API:

- `internal/routeattempt`: shared gateway/mux route-attempt mechanics.
- `internal/citations`: shared citation conversion helpers.

Keep new cross-package helpers internal unless there is a clear external implementation need.

## Stability Rules

- Do not add new exported types or functions when an unexported helper or option is enough.
- Keep provider-specific controls inside namespaced `unified.Request.Extensions` until their semantics are stable enough for canonical fields.
- Keep stateful conversation/session logic outside this repository; llmadapter remains a stateless adapter/gateway/mux layer.
- Prefer adding provider endpoints through `providerregistry.Descriptor` instead of adding central switch statements.
- Treat `cmd/*`, `tests/e2e`, and `.agents/*` as repository tooling, not Go library API.

## Continuation Boundary

`unified.RouteEvent.ConsumerContinuation` is the public projection contract for consumers. `ProviderExecutionEvent.InternalContinuation` and `Transport` are diagnostics for what happened inside a provider endpoint during a turn.

Codex WebSocket continuation does not change the public API surface: consumers still send full replay-style requests to `codex_responses`, while the provider may internally use WebSocket and `previous_response_id` after same-session/same-branch lineage checks pass. Do not add consumer branching logic based on provider name, API family, `Transport`, or `InternalContinuation`.

`unified.AssistantMessageFromResponse` is a stateless helper for replay-style tool continuation. It copies the collected assistant content/reasoning and tool calls into a `unified.Message`; it does not make llmadapter own conversation state.

`unified.Message.Phase`, `unified.Response.Phase`, `unified.MessageStartEvent.Phase`, and `unified.MessageDoneEvent.Phase` preserve provider-supplied assistant message phase metadata such as OpenAI Responses `commentary` and `final_answer`. Empty phase means unknown/legacy behavior. Codecs only encode phase where the target wire API supports assistant message phases.

## Quota Telemetry

Providers can emit `unified.QuotaUsageEvent` when an upstream reports subscription or quota-window usage. The event is observational metadata for library consumers; it does not affect routing, retry, or request projection. `unified.Collect` preserves quota snapshots in `Response.Quotas`.

Codex maps `x-codex-primary-used-percent`, `x-codex-secondary-used-percent`, and related window/reset headers into primary and secondary quota windows. Claude-compatible access maps live `anthropic-ratelimit-unified-5h-*` and `anthropic-ratelimit-unified-7d-*` headers into the same primary and secondary session windows. Anthropic API-key access maps documented `anthropic-ratelimit-*` headers into request/token quota windows with limit, remaining, reset, and derived used-percent fields. Providers with similar subscription models should map their native telemetry into the same event rather than exposing provider-specific headers directly; provider-specific labels and statuses remain in `ProviderRaw`.

## V1 Review Result

No exported renames are required from the current surface before v1.0.0 promotion. The potentially confusing pieces have explicit boundaries:

- Provider identity is `ProviderName`/configured provider instance plus provider `Type`; OpenRouter, MiniMax, Claude, and Codex endpoint variants are modeled as provider endpoint types with concrete API kinds/families.
- Model resolution lives in `adapterconfig` and modeldb overlays; CLI, gateway, mux, and auto construction use that path.
- Workload compatibility consumes adapterconfig candidates and live evidence artifacts; strict selection uses modeldb runtime views and does not add another model resolver.
- Gateway/mux fallback mechanics share `internal/routeattempt`, while HTTP response-start behavior remains in `gateway`.
- Prompt-cache primitives are request-level intent plus explicit block controls; conversation-level cache policy belongs above llmadapter.
- WebSocket transport is part of the public `transport` extension package. OpenAI Responses exposes `WithWebSocketMode(...)` as a direct-client option while keeping the API kind/family as Responses. Provider-specific WebSocket session reuse remains an implementation detail when the public caller contract stays unchanged; a true bidirectional realtime protocol should be modeled as its own API kind/family.
