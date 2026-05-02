package messages

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/codewandler/llmadapter/pipeline"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestIntegrationTextStream(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","role":"assistant","model":"claude-test"}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":2,"output_tokens":3}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://anthropic.test"),
		WithTransport(fake),
		WithHeader("X-Test", "1"),
		WithUnifiedEventProcessor(&pipeline.TextCoalescer{MaxBytes: 64}),
	)
	if err != nil {
		t.Fatal(err)
	}

	maxTokens := 64
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello world" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Usage.TotalTokens() != 5 {
		t.Fatalf("usage total = %d, want 5", resp.Usage.TotalTokens())
	}
	if len(fake.Seen) != 1 || fake.Seen[0].URL != "https://anthropic.test/v1/messages" || fake.Seen[0].Header.Get("X-Test") != "1" {
		t.Fatalf("unexpected transport request: %+v", fake.Seen)
	}
	body, err := io.ReadAll(fake.Seen[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || fake.Seen[0].Header.Get("x-api-key") != "key" || fake.Seen[0].Method != http.MethodPost {
		t.Fatalf("request was not populated correctly")
	}
}

func TestClientDefaultTransportRetriesTransient503(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, `{"error":{"message":"temporary"}}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg\",\"role\":\"assistant\",\"model\":\"claude-test\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n"))
		_, _ = w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(WithAPIKey("key"), WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "ok" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestIntegrationMapsAnthropicRateLimitHeadersToQuota(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{
		Header: http.Header{
			"Anthropic-Ratelimit-Requests-Limit":         []string{"100"},
			"Anthropic-Ratelimit-Requests-Remaining":     []string{"75"},
			"Anthropic-Ratelimit-Requests-Reset":         []string{"2026-05-02T12:00:00Z"},
			"Anthropic-Ratelimit-Input-Tokens-Limit":     []string{"10000"},
			"Anthropic-Ratelimit-Input-Tokens-Remaining": []string{"2500"},
			"Anthropic-Ratelimit-Input-Tokens-Reset":     []string{"2026-05-02T12:01:00Z"},
		},
		Frames: [][]byte{
			[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","role":"assistant","model":"claude-test"}}`),
			[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
			[]byte(`event: message_stop
data: {"type":"message_stop"}`),
		},
	}
	client, err := NewClient(WithAPIKey("key"), WithBaseURL("https://anthropic.test"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 64
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want one quota event", resp.Quotas)
	}
	quota := resp.Quotas[0]
	if quota.Provider != "anthropic" || len(quota.Limits) != 1 {
		t.Fatalf("quota metadata = %+v", quota)
	}
	windows := quota.Limits[0].Windows
	if len(windows) != 2 {
		t.Fatalf("windows = %+v, want 2", windows)
	}
	if windows[0].Name != "requests" || windows[0].UsedPercent != 25 {
		t.Fatalf("requests window = %+v", windows[0])
	}
	if windows[0].Limit == nil || *windows[0].Limit != 100 || windows[0].Remaining == nil || *windows[0].Remaining != 75 {
		t.Fatalf("requests limit/remaining = %+v", windows[0])
	}
	if windows[0].ResetsAtUnix == nil || *windows[0].ResetsAtUnix != 1777723200 {
		t.Fatalf("requests reset = %+v", windows[0].ResetsAtUnix)
	}
	if windows[1].Name != "input_tokens" || windows[1].UsedPercent != 75 {
		t.Fatalf("input tokens window = %+v", windows[1])
	}
}

func TestIntegrationEmitsBestEffortWarnings(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","role":"assistant","model":"claude-test"}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 64
	seed := int64(1)
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Seed:            &seed,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Warnings) != 1 {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}
	if resp.Warnings[0].Code != "unsupported_field_dropped" || resp.Warnings[0].Meta["field"] != "seed" {
		t.Fatalf("unexpected warning: %+v", resp.Warnings[0])
	}
}

func TestIntegrationToolUseStream(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","role":"assistant","model":"claude-test"}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu","name":"lookup","input":{}}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"q\""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":":\"x\"}"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":2,"output_tokens":1}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 64
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "use tool"}},
		}},
		Tools:  []unified.Tool{{Kind: unified.ToolKindFunction, Name: "lookup"}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.FinishReason != unified.FinishReasonToolCall || len(resp.ToolCalls) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if got := string(resp.ToolCalls[0].Arguments); got != `{"q":"x"}` {
		t.Fatalf("tool args = %q", got)
	}
}
