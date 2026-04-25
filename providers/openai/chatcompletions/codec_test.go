package chatcompletions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
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

func TestEncodeResponseFormat(t *testing.T) {
	wire, _, err := encodeRequest(unified.Request{
		Model: "gpt-test",
		ResponseFormat: &unified.ResponseFormat{
			Kind:   unified.ResponseFormatJSONSchema,
			Name:   "answer",
			Schema: json.RawMessage(`{"type":"object"}`),
			Strict: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	format, ok := wire.ResponseFormat.(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("response format = %#v", wire.ResponseFormat)
	}
	schema, ok := format["json_schema"].(map[string]any)
	if !ok || schema["name"] != "answer" || schema["strict"] != true || string(schema["schema"].(json.RawMessage)) != `{"type":"object"}` {
		t.Fatalf("json schema = %#v", format["json_schema"])
	}
}

func TestEncodeImageContent(t *testing.T) {
	wire, warnings, err := encodeRequest(unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "describe"},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/image.png"}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	parts, ok := wire.Messages[0].Content.([]contentPartWire)
	if !ok || len(parts) != 2 || parts[1].Type != "image_url" || parts[1].ImageURL.URL != "https://example.com/image.png" {
		t.Fatalf("content = %#v", wire.Messages[0].Content)
	}
}

func TestEncodeOpenRouterExtensions(t *testing.T) {
	req := unified.Request{Model: "openrouter/test"}
	if err := req.Extensions.Set(unified.ExtOpenRouterProvider, map[string]any{"order": []string{"anthropic"}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterPlugins, []map[string]any{{"id": "web"}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterSessionID, "sess_1"); err != nil {
		t.Fatal(err)
	}
	wire, _, err := encodeRequestForAPI(req, adapt.ApiOpenRouterChatCompletions)
	if err != nil {
		t.Fatal(err)
	}
	if string(wire.OpenRouterProvider) != `{"order":["anthropic"]}` {
		t.Fatalf("provider = %s", wire.OpenRouterProvider)
	}
	if string(wire.OpenRouterPlugins) != `[{"id":"web"}]` {
		t.Fatalf("plugins = %s", wire.OpenRouterPlugins)
	}
	if string(wire.OpenRouterSessionID) != `"sess_1"` {
		t.Fatalf("session_id = %s", wire.OpenRouterSessionID)
	}
}

func TestEncodeOpenRouterExtensionsInvalid(t *testing.T) {
	req := unified.Request{Model: "openrouter/test"}
	if err := req.Extensions.Set(unified.ExtOpenRouterProvider, map[string]any{"order": []string{"anthropic", ""}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterPlugins, []map[string]any{{"id": "web"}, {"id": " "}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterSessionID, " "); err != nil {
		t.Fatal(err)
	}
	wire, warnings, err := encodeRequestForAPI(req, adapt.ApiOpenRouterChatCompletions)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.OpenRouterProvider) != 0 || len(wire.OpenRouterPlugins) != 0 || len(wire.OpenRouterSessionID) != 0 {
		t.Fatalf("invalid extensions should be dropped: %+v", wire)
	}
	if len(warnings) != 3 || warnings[0].code != "invalid_extension_dropped" {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestClientStreamWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`: OPENROUTER PROCESSING`),
		[]byte(`data:`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":2}}}`),
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
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello" || resp.Usage.TotalTokens() != 15 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputNew), 7; got != want {
		t.Fatalf("input.new = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputCacheRead), 3; got != want {
		t.Fatalf("input.cache_read = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindOutput), 3; got != want {
		t.Fatalf("output = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindOutputReasoning), 2; got != want {
		t.Fatalf("output.reasoning = %d, want %d", got, want)
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

func TestClientStreamParallelToolCallsWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_city","type":"function","function":{"name":"lookup_city","arguments":""}},{"index":1,"id":"call_country","type":"function","function":{"name":"lookup_country","arguments":""}}]}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Berlin\"}"}},{"index":1,"function":{"arguments":"{\"country\":\"Germany\"}"}}]}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "gpt-test",
		Tools: []unified.Tool{
			{Kind: unified.ToolKindFunction, Name: "lookup_city"},
			{Kind: unified.ToolKindFunction, Name: "lookup_country"},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.FinishReason != unified.FinishReasonToolCall || len(resp.ToolCalls) != 2 {
		t.Fatalf("response = %+v", resp)
	}
	if resp.ToolCalls[0].ID != "call_city" || resp.ToolCalls[0].Name != "lookup_city" || string(resp.ToolCalls[0].Arguments) != `{"city":"Berlin"}` {
		t.Fatalf("first tool call = %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].ID != "call_country" || resp.ToolCalls[1].Name != "lookup_country" || string(resp.ToolCalls[1].Arguments) != `{"country":"Germany"}` {
		t.Fatalf("second tool call = %+v", resp.ToolCalls[1])
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
