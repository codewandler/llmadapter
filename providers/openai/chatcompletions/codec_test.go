package chatcompletions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestEncodeRequest(t *testing.T) {
	maxTokens := 32
	wire, _, err := encodeRequest(unified.Request{
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

func TestEncodeToolResults(t *testing.T) {
	wire, _, err := encodeRequest(unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role: unified.RoleTool,
			ToolResults: []unified.ToolResult{
				{ToolCallID: "call_1", Name: "lookup", Content: []unified.ContentPart{unified.TextPart{Text: "one"}}},
				{ToolCallID: "call_2", Name: "lookup", Content: []unified.ContentPart{unified.TextPart{Text: "two"}}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages) != 2 {
		t.Fatalf("messages = %+v", wire.Messages)
	}
	if wire.Messages[0].Role != "tool" || wire.Messages[0].ToolCallID != "call_1" || wire.Messages[0].Content != "one" {
		t.Fatalf("unexpected first tool result: %+v", wire.Messages[0])
	}
	if wire.Messages[1].ToolCallID != "call_2" || wire.Messages[1].Content != "two" {
		t.Fatalf("unexpected second tool result: %+v", wire.Messages[1])
	}
}

func TestEncodeRequestWarnings(t *testing.T) {
	wire, warnings, err := encodeRequest(unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "hello"},
				unified.ReasoningPart{Text: "think"},
			},
		}},
		Tools: []unified.Tool{{Kind: "custom", Name: "ignored"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages) != 1 || wire.Messages[0].Content != "hello" || len(wire.Tools) != 0 {
		t.Fatalf("unexpected wire request: %+v", wire)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestClientStreamWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`: OPENROUTER PROCESSING`),
		[]byte(`data:`),
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

func TestClientEmitsEncodeWarnings(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.ReasoningPart{Text: "think"}},
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
	if len(resp.Warnings) != 1 || resp.Warnings[0].Code != "unsupported_field_dropped" {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}
}

func TestClientStreamToolCallWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\""}}]}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"x\"}"}}]}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`),
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
	if resp.FinishReason != unified.FinishReasonToolCall {
		t.Fatalf("finish reason = %q", resp.FinishReason)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("tool calls = %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "lookup" || string(resp.ToolCalls[0].Arguments) != `{"q":"x"}` {
		t.Fatalf("unexpected tool call: %+v", resp.ToolCalls[0])
	}
}

func TestClientStreamErrorWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"error":{"type":"server_error","code":"upstream_failed","message":"upstream failed"}}`),
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
	_, err = unified.Collect(context.Background(), events)
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.Type != "server_error" || apiErr.Code != "upstream_failed" || apiErr.Message != "upstream failed" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}
