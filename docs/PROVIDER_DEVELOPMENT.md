# Provider Development

This guide is for contributors adding a provider, provider endpoint, API kind, endpoint codec, or shared smoke coverage.

## Design Rule

Model each callable upstream surface as a provider endpoint:

```text
Provider = who we talk to.
API kind = exact wire protocol used.
API family = compatibility shape.
Provider endpoint = provider + API kind + API family + client + capabilities.
```

Do not model a multi-surface provider as one API kind. OpenRouter and MiniMax are examples of one provider exposing multiple endpoint types.

## Files To Read First

- [README.md](../README.md): product surface and supported providers.
- [AGENTS.md](../AGENTS.md): repo rules and required checks.
- [ARCHITECTURE.md](ARCHITECTURE.md): package boundaries and request flow.
- [API_SURFACE.md](API_SURFACE.md): public/internal package boundary.
- [PROVIDER_MATRIX.md](PROVIDER_MATRIX.md): current support and smoke coverage.
- [DESIGN.md](../DESIGN.md): long-form target design.

Inspect similar implementations:

- OpenAI Chat-compatible: `providers/openai/chatcompletions/`
- OpenAI Responses-compatible: `providers/openai/responses/`
- Anthropic Messages-compatible: `providers/anthropic/messages/`
- OpenRouter multi-endpoint wrappers: `providers/openrouter/`
- MiniMax wrappers: `providers/minimax/`
- Provider descriptors: `providerregistry/`
- Config, modeldb, and mux construction: `adapterconfig/`
- Gateway wiring: `gatewayserver/`
- Endpoint codecs: `endpoints/`
- Shared live smokes: `tests/e2e/`

## Implementation Checklist

1. Add or confirm `adapt.ApiKind` and `adapt.ApiFamily`.
2. Add a provider package under `providers/<provider>/<api-shape>/`.
3. Reuse an existing compatibility provider only when the upstream wire protocol is actually compatible.
4. Implement request encoding tests with fake transport.
5. Implement stream/event decoding tests with fake transport.
6. Register a `providerregistry.Descriptor` with API kind, family, default capabilities, model env var, credential env vars, smoke model, and client factory.
7. Add `adapterconfig` tests for provider endpoint metadata, defaults, modeldb service identity, and auto detection when relevant.
8. Ensure `gatewayserver` can expose the provider through existing endpoint codecs when the API family is supported.
9. Add shared e2e smoke entries gated by provider-specific env vars or local auth detection.
10. Update [PROVIDER_MATRIX.md](PROVIDER_MATRIX.md), [CONFIGURATION.md](CONFIGURATION.md), and [README.md](../README.md) if public provider support changes.

## Capability Rules

Default capabilities are endpoint-family/provider defaults, not proof that every model supports a feature.

Only advertise a capability when:

- provider mapping supports it,
- endpoint codec behavior is defined,
- tests cover it,
- live smoke coverage exists when practical.

Use modeldb exposure metadata or config overrides to narrow capabilities for specific models.

## Testing Expectations

Minimum for a text provider:

- Fake transport unit test for request construction.
- Fake transport unit test for streamed text decoding.
- Live `TestSmokeTextStream` entry.
- Gateway smoke if the provider can be reached through an implemented endpoint family.

Minimum for a tool-capable provider:

- Tool definition encoding coverage.
- Tool-call stream decoding coverage.
- Tool-result continuation coverage.
- Live `TestSmokeToolUse`.
- Live `TestSmokeToolResultContinuation`.
- `CapabilitySet.Tools = true`.

Minimum for reasoning:

- Reasoning request mapping coverage.
- Reasoning stream/usage fixture.
- Live `TestSmokeReasoningStream` when the provider exposes verifiable reasoning evidence.

Minimum for prompt caching:

- Request/block cache-control mapping coverage.
- Usage mapping for cache read/write counters when the provider reports them.
- Live `TestSmokePromptCache` only when provider-reported accounting is reliable enough to assert.

## Validation Commands

Run before commit:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

Run relevant live slices when credentials are available:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolUse -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolResultContinuation -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestGatewaySmoke -count=1 -v
```

## Common Mistakes

- Do not silently drop unsupported fields without warnings.
- Do not add provider-specific fields directly to `unified.Request`; use namespaced extensions.
- Do not collapse multiple API kinds into one provider type.
- Do not enable broad capabilities because one model supports them.
- Do not rely only on live e2e tests; add deterministic fake transport coverage.
- Do not add hidden conversation/session state to router, gateway, or provider clients.
