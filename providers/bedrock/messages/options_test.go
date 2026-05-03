package messages

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestClientUsesMantleMessagesURLAuthAndAPIKind(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"anthropic.claude-opus-4-7","content":[],"usage":{"input_tokens":4,"output_tokens":0}}}` + "\n\n"),
		[]byte(`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n"),
		[]byte(`event: content_block_delta` + "\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}` + "\n\n"),
		[]byte(`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":0}` + "\n\n"),
		[]byte(`event: content_block_start` + "\n" + `data: {"type":"content_block_start","index":1,"content_block":{"type":"server_tool_use","id":"srv_1","name":"web_search"}}` + "\n\n"),
		[]byte(`event: content_block_stop` + "\n" + `data: {"type":"content_block_stop","index":1}` + "\n\n"),
		[]byte(`event: message_delta` + "\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}` + "\n\n"),
		[]byte(`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n"),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://bedrock-mantle.eu-central-1.api.aws/anthropic/v1/messages"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 16
	events, err := client.Request(context.Background(), unified.Request{
		Model:           DefaultModel,
		MaxOutputTokens: &maxTokens,
		Messages:        []unified.Message{{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "hello"}}}},
		Stream:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if responseText(resp) != "ok" {
		t.Fatalf("response text = %q", responseText(resp))
	}
	if len(fake.Seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.Seen))
	}
	if got, want := fake.Seen[0].URL, "https://bedrock-mantle.eu-central-1.api.aws/anthropic/v1/messages"; got != want {
		t.Fatalf("request URL = %q, want %q", got, want)
	}
	if got := fake.Seen[0].Header.Get("Authorization"); got != "Bearer key" {
		t.Fatalf("authorization header = %q", got)
	}
	if got := fake.Seen[0].Header.Get("x-api-key"); got != "" {
		t.Fatalf("x-api-key header = %q, want empty", got)
	}
	var body map[string]any
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if got := body["model"]; got != DefaultModel {
		t.Fatalf("model = %#v, want %q; body=%#v", got, DefaultModel, body)
	}
	if len(resp.Raw) == 0 || resp.Raw[0].APIKind != defaultAPIKind {
		t.Fatalf("raw API kind = %+v, want %q", resp.Raw, defaultAPIKind)
	}
}

func TestClientUsesGeneratedTokenProvider(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"anthropic.claude-opus-4-7","content":[]}}` + "\n\n"),
		[]byte(`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n"),
	}}
	client, err := NewClient(
		WithTokenProvider(tokenProviderFunc(func(context.Context) (string, error) { return "generated-token", nil })),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 16
	events, err := client.Request(context.Background(), unified.Request{Model: DefaultModel, MaxOutputTokens: &maxTokens, Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if got := fake.Seen[0].Header.Get("Authorization"); got != "Bearer generated-token" {
		t.Fatalf("authorization header = %q", got)
	}
	body, err := io.ReadAll(fake.Seen[0].Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), DefaultModel) {
		t.Fatalf("body = %s", body)
	}
}

func TestClientMapsEnabledThinkingToBedrockAdaptiveThinking(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`event: message_start` + "\n" + `data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"anthropic.claude-opus-4-7","content":[]}}` + "\n\n"),
		[]byte(`event: message_stop` + "\n" + `data: {"type":"message_stop"}` + "\n\n"),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 2048
	budget := 1024
	events, err := client.Request(context.Background(), unified.Request{
		Model:           DefaultModel,
		MaxOutputTokens: &maxTokens,
		Reasoning:       &unified.ReasoningConfig{MaxTokens: &budget, Expose: true},
		Messages:        []unified.Message{{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "think"}}}},
		Stream:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Thinking struct {
			Type         string `json:"type"`
			BudgetTokens int    `json:"budget_tokens,omitempty"`
		} `json:"thinking"`
		OutputConfig struct {
			Effort string `json:"effort"`
		} `json:"output_config"`
	}
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Thinking.Type != "adaptive" || body.Thinking.BudgetTokens != 0 || body.OutputConfig.Effort == "" {
		t.Fatalf("unexpected thinking/output_config: %+v", body)
	}
}

type tokenProviderFunc func(context.Context) (string, error)

func (f tokenProviderFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

func responseText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		if text, ok := part.(unified.TextPart); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}
