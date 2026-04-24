package chatcompletions

import (
	"context"
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

func TestNewClientUsesMiniMaxBaseURL(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"MiniMax-M2.7","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"MiniMax-M2.7","choices":[{"index":0,"delta":{"content":"ok"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"MiniMax-M2.7","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
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
	if responseText(resp) != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(fake.Seen) != 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	if fake.Seen[0].URL != "https://api.minimax.io/v1/chat/completions" {
		t.Fatalf("url = %q", fake.Seen[0].URL)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
}

func TestWithBaseURLAcceptsDocumentedV1BaseURL(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"MiniMax-M2.7","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://api.minimax.io/v1"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "MiniMax-M2.7",
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
	if fake.Seen[0].URL != "https://api.minimax.io/v1/chat/completions" {
		t.Fatalf("url = %q", fake.Seen[0].URL)
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
