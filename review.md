# Repository Review: llmadapter

Review date: 2026-05-02.

Scope: full repository pass after the modeldb metadata cleanup, alias removal, MiniMax Chat continuation fix, docs/example refresh, and the follow-up review-finding fixes.

## Findings

### Low: OpenRouter Anthropic-compatible direct metadata is blocked by modeldb exposure shape

OpenRouter Messages intentionally calls `WithoutBuiltInModelMetadata()` in `providers/openrouter/messages/options.go:34`, and tests guard that Anthropic-native metadata such as Claude-specific `output_config` is not attached to the wrapper service.

Risk: this prevents incorrect Claude/Anthropic behavior from leaking into OpenRouter, but OpenRouter direct Anthropic-compatible clients still do not get service-specific model metadata. modeldb v0.14.0 exposes OpenRouter Claude offerings for OpenAI-style API types, not `anthropic-messages`, so attaching that metadata to the Anthropic-compatible wrapper would conflate provider API surfaces.

Recommendation: keep the opt-out until modeldb records OpenRouter `anthropic-messages` exposures for the relevant offerings, then add an OpenRouter-native direct metadata processor.

## Fixed Findings

- Explicit JSON provider configs now infer modeldb service identity for known provider endpoint types, matching auto config behavior while preserving `modeldb_service_id` as an override.
- OpenAI Responses WebSocket request-body mutation now fails locally on invalid JSON or re-encoding failures instead of falling back to HTTP/SSE or sending malformed WebSocket payloads.
- `unified.AssistantMessageFromResponse` now gives consumers a safe stateless tool-loop replay helper that preserves assistant content/reasoning plus tool calls.
- MiniMax Messages direct clients now attach MiniMax-native built-in model metadata while still rejecting Anthropic-native metadata for Claude IDs.

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
env GOCACHE=/tmp/go-cache go test ./providers/minimax/messages ./providers/openrouter/messages ./modelmeta
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

No release-blocking findings remain in this pass. The remaining issue is a future modeldb-backed enhancement for OpenRouter Anthropic-compatible direct metadata.
