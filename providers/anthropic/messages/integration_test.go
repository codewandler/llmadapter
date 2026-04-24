package messages

import (
	"context"
	"io"
	"net/http"
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
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("usage total = %d, want 5", resp.Usage.TotalTokens)
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
