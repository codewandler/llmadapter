package responses

import (
	"context"
	"encoding/json"
	"errors"
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

func TestDecodeMidStreamError(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"error","error":{"type":"server_error","code":"upstream_failed","message":"upstream failed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "openai/test", Stream: true})
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

func TestClientStreamWithFakeTransport(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.content_part.added","response_id":"resp_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`),
		[]byte(`data: {"type":"response.content_part.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"hello"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed","usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15,"input_tokens_details":{"cached_tokens":3},"output_tokens_details":{"reasoning_tokens":2}}}}`),
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
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello" || resp.Usage.TotalTokens() != 15 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ID != "resp_1" {
		t.Fatalf("response id = %q, want resp_1", resp.ID)
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
	if len(fake.Seen) != 1 || fake.Seen[0].URL != "https://openrouter.ai/api/v1/responses" {
		t.Fatalf("unexpected request: %+v", fake.Seen)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
	if got := fake.Seen[0].Header.Get("OpenAI-Beta"); got != "" {
		t.Fatalf("OpenRouter Responses should not opt into OpenAI WebSocket beta, got %q", got)
	}
}

func TestClientStreamMapsOpenRouterPromptCacheWriteUsage(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed","usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_tokens_details":{"cached_tokens":3,"cache_write_tokens":2},"completion_tokens_details":{"reasoning_tokens":1}}}}`),
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
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputNew), 5; got != want {
		t.Fatalf("input.new = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputCacheRead), 3; got != want {
		t.Fatalf("input.cache_read = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputCacheWrite), 2; got != want {
		t.Fatalf("input.cache_write = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindOutput), 4; got != want {
		t.Fatalf("output = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindOutputReasoning), 1; got != want {
		t.Fatalf("output.reasoning = %d, want %d", got, want)
	}
}

func TestClientEncodesOpenRouterExtensions(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "openai/test"}
	if err := req.Extensions.Set(unified.ExtOpenRouterProvider, map[string]any{"allow_fallbacks": true}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterDebug, true); err != nil {
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

	var body map[string]json.RawMessage
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if string(body["provider"]) != `{"allow_fallbacks":true}` {
		t.Fatalf("provider = %s", body["provider"])
	}
	if string(body["debug"]) != `true` {
		t.Fatalf("debug = %s", body["debug"])
	}
	if string(body["session_id"]) != `"sess_1"` {
		t.Fatalf("session_id = %s", body["session_id"])
	}
}

func TestClientWarnsForInvalidOpenRouterExtensions(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "openai/test"}
	if err := req.Extensions.Set(unified.ExtOpenRouterProvider, map[string]any{"order": []string{"anthropic", ""}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterPlugins, []map[string]any{{"id": "web"}, {"id": " "}}); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenRouterSessionID, " "); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Warnings) != 3 || resp.Warnings[0].Code != "invalid_extension_dropped" {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}
}

func TestDecodeReasoningSummaryDelta(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test"}}`),
		[]byte(`data: {"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"thinking"}`),
		[]byte(`data: {"type":"response.output_text.delta","output_index":1,"delta":"answer"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "openai/test", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	var foundReasoning bool
	for _, part := range resp.Content {
		if reasoning, ok := part.(unified.ReasoningPart); ok && reasoning.Text == "thinking" {
			foundReasoning = true
		}
	}
	if !foundReasoning {
		t.Fatalf("reasoning content = %+v", resp.Content)
	}
}

func TestClientEmitsEncodeWarnings(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "openai/test",
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
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_item.added","response_id":"resp_1","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"lookup","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.function_call_arguments.done","response_id":"resp_1","output_index":0,"arguments":"{\"q\":\"x\"}"}`),
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
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai/test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_item.added","response_id":"resp_1","output_index":0,"item":{"type":"function_call","id":"fc_city","call_id":"call_city","name":"lookup_city","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_item.added","response_id":"resp_1","output_index":1,"item":{"type":"function_call","id":"fc_country","call_id":"call_country","name":"lookup_country","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.function_call_arguments.done","response_id":"resp_1","output_index":0,"arguments":"{\"city\":\"Berlin\"}"}`),
		[]byte(`data: {"type":"response.function_call_arguments.done","response_id":"resp_1","output_index":1,"arguments":"{\"country\":\"Germany\"}"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai/test","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "openai/test",
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
