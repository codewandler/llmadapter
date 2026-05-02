# Troubleshooting

This document covers runtime diagnostics for provider transport and continuation failures.

## WebSocket Close Or Context-Window Failure

Symptoms:

- `websocket closed`, `keep alive ping timeout`, or a provider-specific close error appears mid-session.
- A later turn fails with a context-window error even though the caller expected provider-internal continuation.
- `llmadapter infer` initially reports `transport: http_sse` in route metadata but later provider execution metadata may update it to `websocket`.

Important distinction:

- `consumer_continuation` is the public caller contract. If it is `replay`, callers must be prepared to replay the conversation.
- `internal_continuation` and `transport` are diagnostics for provider optimizations inside llmadapter.
- Codex remains public replay even when llmadapter uses a provider-internal WebSocket and internal `previous_response_id`.

Capture a current Codex session trace:

```sh
go run ./cmd/llmadapter infer \
  --debug request,response,stream \
  -m codex/gpt-5.4 \
  --session debug-session-1 \
  --branch main \
  --max-tokens 64 \
  "reply with one short sentence"
```

Capture a Claude/Anthropic HTTP/SSE trace:

```sh
go run ./cmd/llmadapter infer \
  --debug request,response,stream \
  -m anthropic/claude-haiku-4-5-20251001 \
  --source-api anthropic.messages \
  --max-tokens 64 \
  "reply with one short sentence"
```

Useful scopes:

- `request`: outbound URL, transport mode, redacted headers, body, and initial WebSocket frame.
- `response`: inbound HTTP/SSE response or WebSocket handshake headers and non-2xx error bodies.
- `stream`: raw provider stream frames after llmadapter transport framing.
- `events`: unified events emitted by llmadapter.

What to inspect:

- Confirm whether `[debug #N] transport:` is `http_sse` or `websocket`.
- Confirm whether `[debug mode] provider transport=websocket` appears before the final route section.
- Check for `!!! stream error`, `websocket closed`, context-window errors, or provider error frames.
- For Codex, check whether the final route section reports `transport: websocket` or fell back to `transport: http_sse`.
- If WebSocket closes before the response starts, llmadapter should fall back to HTTP/SSE for that turn.
- If WebSocket closes after streamed output starts, llmadapter cannot transparently retry without duplicating output; the caller should treat the turn as failed and replay from its own conversation state.

Avoid storing raw traces in permanent logs unless required. Debug output redacts known sensitive headers and JSON keys, but prompts, response IDs, and provider metadata can still be operationally sensitive.

## Library-Level Diagnostics

Consumers can wire the same redacted diagnostics used by `llmadapter infer`:

```go
scopes, err := diagnostics.ParseScopes([]string{"request,response,stream"})
if err != nil {
	return err
}

client, err := adapterconfig.NewMuxClient(
	cfg,
	adapterconfig.WithSourceAPI(adapt.ApiOpenAIResponses),
	adapterconfig.WithProviderTransport(diagnostics.NewHTTPTransport(os.Stderr, scopes)),
	adapterconfig.WithProviderWebSocketTransport(diagnostics.NewWebSocketTransport(os.Stderr, scopes)),
)
```

This wraps only the provider transports created by `adapterconfig.NewMuxClient`. It does not change the caller's public continuation contract.
