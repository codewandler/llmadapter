# Repository Review: llmadapter

Review date: 2026-05-02.

Scope: full repository pass after the modeldb metadata cleanup, alias removal, MiniMax Chat continuation fix, docs/example refresh, and the follow-up review-finding fixes.

## Findings

### Low: Anthropic-compatible wrapper providers are protected from Anthropic-native metadata, but service-specific metadata is still shallow

OpenRouter Messages and MiniMax Messages intentionally call `WithoutBuiltInModelMetadata()` in `providers/openrouter/messages/options.go:34` and `providers/minimax/messages/options.go:34`, and tests guard that Anthropic-native metadata such as Claude-specific `output_config` is not attached to those wrapper services.

Risk: this prevents incorrect Claude/Anthropic behavior from leaking into OpenRouter/MiniMax, but it also means those wrappers do not yet have equivalent service-specific model metadata processors for their own Anthropic-compatible catalogs. Any future OpenRouter/MiniMax model-specific parameter mapping must come from mux/config route metadata, not direct wrapper construction.

Recommendation: keep the opt-out, but add provider-specific direct metadata attachment only when modeldb has service-native OpenRouter/MiniMax Anthropic-compatible metadata that can be resolved without confusing Anthropic model IDs with third-party wire IDs.

## Fixed Findings

- Explicit JSON provider configs now infer modeldb service identity for known provider endpoint types, matching auto config behavior while preserving `modeldb_service_id` as an override.
- OpenAI Responses WebSocket request-body mutation now fails locally on invalid JSON or re-encoding failures instead of falling back to HTTP/SSE or sending malformed WebSocket payloads.
- `unified.AssistantMessageFromResponse` now gives consumers a safe stateless tool-loop replay helper that preserves assistant content/reasoning plus tool calls.

## Non-Findings Checked

- Gateway server timeout concerns are already addressed: the server uses explicit read header, read, write, and idle timeouts.
- Gateway request body size concerns are already addressed: endpoint codecs use bounded request-body reads and return 413 for oversized bodies.
- Provider stream send goroutine leaks from context cancellation are already addressed: send paths select on `ctx.Done()`.
- OpenAI/Codex Responses shared WebSocket session access is protected by a mutex.
- SSE/WebSocket transport helper error handling for seek and CRLF edge cases is strict.

## Verification

Passed:

```sh
env GOCACHE=/tmp/go-cache go test ./providers/minimax/messages ./providers/openrouter/messages ./tests/e2e ./cmd/llmadapter ./adapterconfig
env GOCACHE=/tmp/go-cache go test ./adapterconfig ./providers/openai/responses ./unified ./tests/e2e
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

No release-blocking findings remain in this pass. The remaining issue is a future enhancement around service-specific direct metadata for Anthropic-compatible third-party wrappers.
