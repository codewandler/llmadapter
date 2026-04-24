# Changelog

All notable project changes are tracked here.

This project does not have tagged releases yet. Entries below group the commit history into milestone-level changes.

## Unreleased

### Architecture And Planning

- Added the initial architecture design for the canonical request/event model, provider routing, stream pipeline, transport abstraction, provider endpoints, capabilities, and testing strategy. (`eb25315`, `3a70b88`)
- Added and refined the implementation plan for phases 1-3, then expanded it as the project moved into gateway, routing, provider, and e2e work. (`f10978c`)
- Added minimal repo onboarding docs and a provider-extension agent skill. (`2ffafd9`)

### Core Adapter Slice

- Implemented the first working vertical slice across `unified`, `adapt`, `pipeline`, `transport`, and Anthropic Messages. (`f2012a1`)
- Added reusable SSE/NDJSON/HTTP/fake transport primitives, retry/rate-limit wrappers, provider event decoding, and `unified.Collect`.
- Added the first live e2e smoke harness gated by `TEST_INTEGRATION`. (`8b57bdd`)

### Gateway And Routing

- Added the OpenAI-compatible `/v1/chat/completions` endpoint codec and minimal gateway handler. (`d2e5d80`)
- Added live gateway smoke coverage for streaming and non-streaming requests. (`9b9c0b0`)
- Added static gateway routing with source API/model matching and native model rewrite. (`25182e5`)
- Added minimal JSON gateway config with providers, routes, `api_key` / `api_key_env`, `provider_api`, and model override support. (`a6aa676`, `3b47ba2`)
- Modeled routes as provider endpoints with provider name, API kind, API family, client, and capability metadata. (`3b47ba2`)

### Provider Support

- Added OpenAI Chat Completions provider support, including `OPENAI_API_KEY` / `OPENAI_KEY` lookup and gateway route coverage. (`fff8ac8`, `9d88990`)
- Added OpenAI Chat streamed tool-call decoding and live shared tool-use coverage. (`45bb30c`)
- Added live tool-result continuation smoke coverage across supported tool-capable providers. (`9961e5d`)
- Added OpenRouter Chat Completions provider support as a distinct OpenRouter API kind in the OpenAI Chat family. (`f3099b8`)
- Added OpenRouter Responses and OpenRouter Anthropic-compatible Messages providers. (`74678ee`)
- Added OpenRouter Responses function-call streaming and tool-result continuation support. (`e482cbd`)

### Test Coverage

- Added shared live e2e smoke tests for text streaming, tool calls, tool-result continuation, and gateway routing.
- Expanded the live matrix across Anthropic Messages, OpenAI Chat Completions, OpenRouter Chat Completions, OpenRouter Responses, and OpenRouter Anthropic-compatible Messages.
- Added fake transport unit coverage for provider request construction and stream decoding paths.

### Tooling

- Added `.gitignore` entries for local workspace files. (`1c1bbeb`)
