package messages

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestNewClientUsesMiniMaxMessagesEndpoint(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"MiniMax-M2.7","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "MiniMax-M2.7",
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
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(fake.Seen) != 1 || fake.Seen[0].URL != "https://api.minimax.io/anthropic/v1/messages" {
		t.Fatalf("unexpected request: %+v", fake.Seen)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
	if fake.Seen[0].Header.Get("x-api-key") != "key" {
		t.Fatalf("missing x-api-key header: %+v", fake.Seen[0].Header)
	}
}

func TestWithBaseURLAcceptsDocumentedMessagesURL(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"MiniMax-M2.7","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://api.minimax.io/anthropic/v1/messages"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "MiniMax-M2.7",
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
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if fake.Seen[0].URL != "https://api.minimax.io/anthropic/v1/messages" {
		t.Fatalf("url = %q", fake.Seen[0].URL)
	}
}

func TestNewClientDoesNotAttachAnthropicNativeModelMetadata(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"claude-opus-4-7","content":[]}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 4096
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-opus-4-7",
		MaxOutputTokens: &maxTokens,
		Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortMax},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(fake.Seen[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["output_config"]; ok {
		t.Fatalf("minimax wrapper should not attach Anthropic-native xhigh metadata: body=%s", body)
	}
}

func TestNewClientUsesMiniMaxNativeModelMetadata(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","type":"message","role":"assistant","model":"MiniMax-M2.7","content":[]}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 4096
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "MiniMax-M2.7",
		MaxOutputTokens: &maxTokens,
		Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(fake.Seen[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["thinking"]; !ok {
		t.Fatalf("minimax reasoning request should include thinking config: body=%s", body)
	}
	if _, ok := wire["output_config"]; ok {
		t.Fatalf("minimax metadata should not invent unsupported effort values: body=%s", body)
	}
}
