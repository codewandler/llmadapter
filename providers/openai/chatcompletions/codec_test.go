package chatcompletions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestEncodeRequest(t *testing.T) {
	maxTokens := 32
	wire, err := encodeRequest(unified.Request{
		Model:           "gpt-test",
		MaxOutputTokens: &maxTokens,
		Instructions: []unified.Instruction{{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: "be brief"}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Tools: []unified.Tool{{Kind: unified.ToolKindFunction, Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wire.Model != "gpt-test" || wire.MaxTokens == nil || *wire.MaxTokens != 32 {
		t.Fatalf("unexpected request: %+v", wire)
	}
	if len(wire.Messages) != 2 || wire.Messages[0].Role != "system" || wire.Messages[1].Content != "hello" {
		t.Fatalf("unexpected messages: %+v", wire.Messages)
	}
	if len(wire.Tools) != 1 || wire.Tools[0].Function.Name != "lookup" {
		t.Fatalf("unexpected tools: %+v", wire.Tools)
	}
}

func TestClientStreamWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "gpt-test",
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
}
