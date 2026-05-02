# Repository Review: llmadapter

Review date: 2026-05-02.

Scope: full repository pass after the modeldb metadata cleanup, alias removal, MiniMax Chat continuation fix, docs/example refresh, and `v1.0.0-rc.20`.

## Findings

### Medium: Explicit provider configs do not consistently infer modeldb service identity

Auto config assigns modeldb service IDs for every known provider type when modeldb is enabled (`anthropic`, `openai_*`, `openrouter_*`, `minimax_*`, `codex_responses`) in `adapterconfig/auto.go:418`.

Explicit JSON config only auto-tags `claude` and `codex_responses`; every other provider type must specify `modeldb_service_id` manually in `adapterconfig/runtime.go:330`. This is visible in the example config, where the explicit Anthropic provider needs `"modeldb_service_id": "anthropic"` to get modeldb-backed capabilities/pricing in `examples/llmadapter.example.json:23`.

Risk: operators can define an explicit provider that looks modeldb-backed but silently runs with descriptor defaults and no pricing/model metadata unless they know to add the service ID. That inconsistency is now more important because modeldb is the source of truth for model-specific parameter behavior.

Recommendation: either infer service IDs for explicit provider types the same way auto config does, or make the absence explicit in validation/inspection warnings when routes request modeldb-backed behavior but the provider has no service ID.

### Medium: OpenAI Responses WebSocket payload mutation silently falls back on JSON errors

`providers/openai/responses/websocket.go:85` mutates the internally generated Responses request body into a WebSocket `response.create` payload. If JSON decode or encode fails, it returns the original body instead of an error. `webSocketSessionID` and `requestPreviousResponseID` similarly ignore decode failures in `providers/openai/responses/websocket.go:114`.

Risk: today the body is generated internally, so failure is unlikely. If a future body mutator or extension path produces invalid JSON, WebSocket mode may send a payload missing the required `"type":"response.create"` wrapper and fail with an opaque upstream error. This is avoidable because malformed internal request JSON is a local invariant violation.

Recommendation: make WebSocket body construction return `([]byte, error)` and fail before opening/writing the WebSocket when the body cannot be decoded or re-encoded. Treat session ID extraction errors as non-fatal only when a stable `Request.CacheKey` is present.

### Low: Tool-loop replay correctness is documented and tested, but not easy for consumers

The MiniMax Chat live failure showed that some OpenAI-compatible reasoning models require replaying the complete assistant tool-call message, including assistant content/reasoning and `ToolCalls`, before appending tool results. The shared smoke test now does this in `tests/e2e/smoke_test.go:260`, and docs call it out in `docs/LIBRARY_USAGE.md:228`.

Risk: library consumers still have to hand-assemble the replay message from a collected `unified.Response`. It is easy to preserve `ToolCalls` but accidentally drop `Content`, reproducing the same provider-specific failure outside the test harness.

Recommendation: consider adding a small public helper such as `unified.AssistantMessageFromResponse(resp)` or `unified.ToolContinuationMessages(user, resp, results)` so consumers get the safe replay shape by default without changing llmadapter's stateless conversation semantics.

### Low: Anthropic-compatible wrapper providers are protected from Anthropic-native metadata, but service-specific metadata is still shallow

OpenRouter Messages and MiniMax Messages intentionally call `WithoutBuiltInModelMetadata()` in `providers/openrouter/messages/options.go:34` and `providers/minimax/messages/options.go:34`, and tests now guard that Anthropic-native metadata such as Claude-specific `output_config` is not attached to those wrapper services.

Risk: this prevents incorrect Claude/Anthropic behavior from leaking into OpenRouter/MiniMax, but it also means those wrappers do not yet have equivalent service-specific model metadata processors for their own Anthropic-compatible catalogs. Any future OpenRouter/MiniMax model-specific parameter mapping must come from mux/config route metadata, not direct wrapper construction.

Recommendation: keep the opt-out, but add provider-specific direct metadata attachment only when modeldb has service-native OpenRouter/MiniMax Anthropic-compatible metadata that can be resolved without confusing Anthropic model IDs with third-party wire IDs.

## Non-Findings Checked

- Gateway server timeout concerns are already addressed: the server uses explicit read header, read, write, and idle timeouts.
- Gateway request body size concerns are already addressed: endpoint codecs use bounded request-body reads and return 413 for oversized bodies.
- Provider stream send goroutine leaks from context cancellation are already addressed: send paths select on `ctx.Done()`.
- OpenAI/Codex Responses shared WebSocket session access is protected by a mutex.
- SSE/WebSocket transport helper error handling for seek and CRLF edge cases is stricter than in earlier review notes.

## Verification

Passed:

```sh
env GOCACHE=/tmp/go-cache go test ./providers/minimax/messages ./providers/openrouter/messages ./tests/e2e ./cmd/llmadapter ./adapterconfig
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json example-fast
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001 --source-api anthropic.messages
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream|TestSmokeToolUse|TestSmokeToolResultContinuation|TestGatewaySmoke' -count=1 -v
```

## Current Assessment

No release-blocking findings were found in this pass. The main cleanup candidate before final v1 is explicit provider modeldb-service inference or warnings, because the current behavior is easy to misconfigure and conflicts with the goal of modeldb as the source of truth.
