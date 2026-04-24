package chatcompletions

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient()
	if err == nil {
		t.Fatalf("expected missing API key error")
	}
}

func TestNewClientUsesOpenRouterBaseURL(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"openrouter-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"openrouter-test","choices":[{"index":0,"delta":{"content":"ok"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"openrouter-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "openai/gpt-test",
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
	if responseText(resp) != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(fake.Seen) != 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	if fake.Seen[0].URL != "https://openrouter.ai/api/v1/chat/completions" {
		t.Fatalf("url = %q", fake.Seen[0].URL)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
}

func TestNewClientEncodesOpenRouterExtensions(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"openrouter-test","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "openai/gpt-test", Stream: true}
	if err := req.Extensions.Set(unified.ExtOpenRouterProvider, map[string]any{"order": []string{"anthropic"}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterSessionID, "sess_1"); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
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
	if string(wire["provider"]) != `{"order":["anthropic"]}` {
		t.Fatalf("provider = %s body=%s", wire["provider"], body)
	}
	if string(wire["session_id"]) != `"sess_1"` {
		t.Fatalf("session_id = %s body=%s", wire["session_id"], body)
	}
}

func responseText(resp unified.Response) string {
	if len(resp.Content) == 0 {
		return ""
	}
	text, ok := resp.Content[0].(unified.TextPart)
	if !ok {
		return ""
	}
	return text.Text
}
