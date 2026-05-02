# Repository Review: llmadapter

Review date: 2026-05-02.

Scope: full repository pass after the modeldb metadata cleanup, alias removal, MiniMax Chat continuation fix, docs/example refresh, and the follow-up review-finding fixes.

## Findings

No release-blocking findings remain in this pass.

## Fixed Findings

- Explicit JSON provider configs now infer modeldb service identity for known provider endpoint types, matching auto config behavior while preserving `modeldb_service_id` as an override.
- OpenAI Responses WebSocket request-body mutation now fails locally on invalid JSON or re-encoding failures instead of falling back to HTTP/SSE or sending malformed WebSocket payloads.
- `unified.AssistantMessageFromResponse` now gives consumers a safe stateless tool-loop replay helper that preserves assistant content/reasoning plus tool calls.
- MiniMax Messages direct clients now attach MiniMax-native built-in model metadata while still rejecting Anthropic-native metadata for Claude IDs.
- OpenRouter Messages direct clients now attach OpenRouter-native Anthropic Messages modeldb metadata for `anthropic/...` offerings without enabling Anthropic-native Claude metadata for wrapper services.
- Anthropic-family JSON-schema response formats now encode through `output_config.format` only when resolved modeldb metadata confirms that mapping.
- Anthropic-family top-level `cache_control` now encodes only when resolved modeldb metadata confirms `top_level_cache_control -> cache_control`.

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
env GOCACHE=/tmp/go-cache go test ./providers/anthropic/messages ./providers/openrouter/messages ./tests/e2e
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json example-fast
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001 --source-api anthropic.messages
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeOpenRouterMessagesStructuredOutput' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream|TestSmokeToolUse|TestSmokeToolResultContinuation|TestGatewaySmoke' -count=1 -v
```

## Current Assessment

No release-blocking findings remain in this pass. The required full live matrix passed after the MiniMax rows were rerun and stabilized.
