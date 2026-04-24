package responses

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

func TestClientStreamWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.content_part.added","response_id":"resp_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`),
		[]byte(`data: {"type":"response.content_part.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"hello"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "openai/test",
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
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(fake.Seen) != 1 || fake.Seen[0].URL != "https://openrouter.ai/api/v1/responses" {
		t.Fatalf("unexpected request: %+v", fake.Seen)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
}

func TestEncodeRequest(t *testing.T) {
	maxTokens := 8
	wire := encodeRequest(unified.Request{
		Model:           "openai/test",
		MaxOutputTokens: &maxTokens,
		Instructions: []unified.Instruction{{
			Content: []unified.ContentPart{unified.TextPart{Text: "be brief"}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
	})
	if wire.Model != "openai/test" || wire.Instructions != "be brief" || wire.MaxOutputTokens == nil || *wire.MaxOutputTokens != 8 {
		t.Fatalf("unexpected request: %+v", wire)
	}
	if len(wire.Input) != 1 || wire.Input[0].Role != "user" || wire.Input[0].Content[0].Type != "input_text" {
		t.Fatalf("unexpected input: %+v", wire.Input)
	}
}
