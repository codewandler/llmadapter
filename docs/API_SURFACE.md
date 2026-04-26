# Public API Surface

This document records the v1 package-boundary decision for llmadapter. It is intended to prevent accidental public API expansion after the release-candidate baseline.

## Stable Consumer Packages

These packages are the primary v1 surface for library consumers:

- `unified`: canonical request, response, event, content, tool, usage, cache, extension, and `unified.Client` types.
- `adapterconfig`: config loading/defaulting/validation, auto credential detection, modeldb-backed resolution, router construction, and mux client construction.
- `muxclient`: stateless in-process client over the router/provider endpoint path.
- `router`: provider endpoint metadata, route definitions, capability checks, model resolution hooks, and deterministic candidate ranking.
- `providerregistry`: provider endpoint descriptors and descriptor-backed client construction.
- `gatewayserver`: shared HTTP server wiring used by `llmadapter serve` and `cmd/llmadapter-gateway`.
- `compatibility`: workload profile definitions and candidate evaluation for use cases such as agentic coding.

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

## V1 Review Result

No exported renames are required from the current surface before v1.0.0 promotion. The potentially confusing pieces have explicit boundaries:

- Provider identity is `ProviderName`/configured provider instance plus provider `Type`; OpenRouter, MiniMax, Claude, and Codex endpoint variants are modeled as provider endpoint types with concrete API kinds/families.
- Model resolution lives in `adapterconfig` and modeldb overlays; CLI, gateway, mux, and auto construction use that path.
- Workload compatibility consumes adapterconfig candidates; it does not add another model resolver.
- Gateway/mux fallback mechanics share `internal/routeattempt`, while HTTP response-start behavior remains in `gateway`.
- Prompt-cache primitives are request-level intent plus explicit block controls; conversation-level cache policy belongs above llmadapter.
