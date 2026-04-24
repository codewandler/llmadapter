---
name: llmadapter-provider-extension
description: Use when extending this repository with a new provider, provider endpoint, API kind, endpoint codec, or shared e2e provider smoke coverage. Applies to adding OpenAI-compatible, Anthropic-compatible, OpenRouter-style multi-endpoint, or new API-family provider support in llmadapter.
---

# llmadapter Provider Extension

Use this workflow for provider/API-kind work in this repo.

## First Read

Read these files before editing:

- `README.md` for current implemented surface and env vars.
- `AGENTS.md` for repo rules and required checks.
- `PLAN.md` current status and known gaps.
- `DESIGN.md` sections for API kind, API family, provider endpoint, routing, and capabilities.

Inspect similar code before creating new code:

- OpenAI-compatible provider: `providers/openai/chatcompletions/`
- Anthropic-compatible provider: `providers/anthropic/messages/`
- OpenRouter multi-endpoint wrappers: `providers/openrouter/`
- Gateway registration: `cmd/llmadapter-gateway/main.go`
- Shared live smokes: `tests/e2e/smoke_test.go` and `tests/e2e/gateway_smoke_test.go`

## Provider Endpoint Rule

Model each callable upstream surface as a provider endpoint:

```text
Provider = who we talk to.
API kind = exact wire protocol used.
API family = compatibility shape.
Provider endpoint = provider + API kind + family + client + capabilities.
```

Do not model a multi-surface provider as one API kind. Add one endpoint per wire protocol.

## Implementation Checklist

1. Add or confirm `adapt.ApiKind` and `adapt.ApiFamily`.
2. Add provider package under `providers/<provider>/<api-shape>/`.
3. Prefer wrapping an existing compatibility provider only when the upstream wire protocol is actually compatible.
4. Implement focused request encoding and stream decoding tests with fake transport.
5. Register provider type and endpoint metadata in `cmd/llmadapter-gateway/main.go`.
6. Add config validation tests in `cmd/llmadapter-gateway/config_test.go`.
7. Add shared e2e matrix entries gated by provider-specific env vars.
8. Update `PLAN.md`; update `README.md` for new env vars or provider types.

## Provider Test Matrix

Minimum for a text provider:

- Offline fake transport text stream test.
- Live `TestSmokeTextStream` entry.
- Gateway smoke if it can consume canonical OpenAI Chat-shaped input.

Minimum for a tool-capable provider:

- Offline encode/decode tests for tool definitions, tool calls, and tool results.
- Live `TestSmokeToolUse`.
- Live `TestSmokeToolResultContinuation`.
- Capability metadata must set `Tools: true`.

## Validation Commands

Run before commit:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Run live tests when credentials are available:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream|TestSmokeToolUse|TestSmokeToolResultContinuation|TestGatewaySmoke' -count=1 -v
```

## Common Shortcuts To Avoid

- Do not silently drop unsupported fields; emit warnings or use namespaced extensions when possible.
- Do not claim a capability in `CapabilitySet` until there is codec support and e2e coverage.
- Do not add provider-specific fields directly to `unified.Request`; use namespaced extensions.
- Do not rely only on live e2e tests; add fake transport unit coverage for stream event shapes.
