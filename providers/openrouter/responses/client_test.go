package responses

import (
	"context"
	"encoding/json"
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
}

func TestEncodeRequest(t *testing.T) {
	maxTokens := 8
	wire, _ := encodeRequest(unified.Request{
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

func TestEncodeRequestTools(t *testing.T) {
	wire, _ := encodeRequest(unified.Request{
		Model: "openai/test",
		Messages: []unified.Message{
			{
				Role: unified.RoleAssistant,
				ToolCalls: []unified.ToolCall{{
					ID:        "call_1",
					Name:      "lookup",
					Arguments: json.RawMessage(`{"q":"x"}`),
				}},
			},
			{
				Role: unified.RoleTool,
				ToolResults: []unified.ToolResult{{
					ToolCallID: "call_1",
					Content:    []unified.ContentPart{unified.TextPart{Text: "result"}},
				}},
			},
		},
		Tools: []unified.Tool{{
			Kind:        unified.ToolKindFunction,
			Name:        "lookup",
			Description: "lookup values",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup"},
	})
	if len(wire.Input) != 2 {
		t.Fatalf("input = %+v", wire.Input)
	}
	if wire.Input[0].Type != "function_call" || wire.Input[0].ID != "" || wire.Input[0].CallID != "call_1" || wire.Input[0].Arguments != `{"q":"x"}` {
		t.Fatalf("unexpected function call input: %+v", wire.Input[0])
	}
	if wire.Input[1].Type != "function_call_output" || wire.Input[1].ID != "" || wire.Input[1].CallID != "call_1" || wire.Input[1].Output != "result" {
		t.Fatalf("unexpected function output input: %+v", wire.Input[1])
	}
	if len(wire.Tools) != 1 || wire.Tools[0].Name != "lookup" {
		t.Fatalf("tools = %+v", wire.Tools)
	}
	choice, ok := wire.ToolChoice.(map[string]string)
	if !ok || choice["type"] != "function" || choice["name"] != "lookup" {
		t.Fatalf("tool choice = %#v", wire.ToolChoice)
	}
}

func TestEncodeRequestPreservesOpenAIResponsesFunctionCallItemID(t *testing.T) {
	wire, _ := encodeRequest(unified.Request{
		Model: "openai/test",
		Messages: []unified.Message{{
			Role: unified.RoleAssistant,
			ToolCalls: []unified.ToolCall{{
				ID:        "fc_1",
				Name:      "lookup",
				Arguments: json.RawMessage(`{"q":"x"}`),
			}},
		}},
	})
	if len(wire.Input) != 1 || wire.Input[0].ID != "fc_1" || wire.Input[0].CallID != "fc_1" {
		t.Fatalf("unexpected function call input: %+v", wire.Input)
	}
}

func TestEncodeRequestWarnings(t *testing.T) {
	wire, warnings := encodeRequest(unified.Request{
		Model: "openai/test",
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "hello"},
				unified.ReasoningPart{Text: "think"},
			},
		}},
		Tools: []unified.Tool{{Kind: "custom", Name: "ignored"}},
	})
	if len(wire.Input) != 1 || len(wire.Input[0].Content) != 1 || len(wire.Tools) != 0 {
		t.Fatalf("unexpected wire request: %+v", wire)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestEncodeOpenRouterExtensions(t *testing.T) {
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
	wire, _ := encodeRequest(req)
	if string(wire.OpenRouterProvider) != `{"allow_fallbacks":true}` {
		t.Fatalf("provider = %s", wire.OpenRouterProvider)
	}
	if string(wire.OpenRouterDebug) != `true` {
		t.Fatalf("debug = %s", wire.OpenRouterDebug)
	}
	if string(wire.OpenRouterSessionID) != `"sess_1"` {
		t.Fatalf("session_id = %s", wire.OpenRouterSessionID)
	}
}

func TestEncodeOpenAIResponsesExtensions(t *testing.T) {
	req := unified.Request{Model: "openai/test"}
	if err := req.Extensions.Set(unified.ExtOpenAIPreviousResponseID, "resp_prev"); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenAIStore, false); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenAIPromptCacheKey, "cache_key_1"); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.Set(unified.ExtOpenAIPromptCacheRetention, "24h"); err != nil {
		t.Fatal(err)
	}
	wire, warnings := encodeRequest(req)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if wire.PreviousResponseID != "resp_prev" {
		t.Fatalf("previous_response_id = %q", wire.PreviousResponseID)
	}
	if wire.Store == nil || *wire.Store {
		t.Fatalf("store = %+v, want false pointer", wire.Store)
	}
	if wire.PromptCacheKey != "cache_key_1" {
		t.Fatalf("prompt_cache_key = %q", wire.PromptCacheKey)
	}
	if wire.PromptCacheRetention != "24h" {
		t.Fatalf("prompt_cache_retention = %q", wire.PromptCacheRetention)
	}
}

func TestEncodeOpenAIResponsesExtensionsInvalid(t *testing.T) {
	req := unified.Request{Model: "openai/test"}
	if err := req.Extensions.SetRaw(unified.ExtOpenAIPreviousResponseID, json.RawMessage(`123`)); err != nil {
		t.Fatal(err)
	}
	if err := req.Extensions.SetRaw(unified.ExtOpenAIStore, json.RawMessage(`"true"`)); err != nil {
		t.Fatal(err)
	}
	wire, warnings := encodeRequest(req)
	if wire.PreviousResponseID != "" || wire.Store != nil {
		t.Fatalf("invalid extensions should be dropped: %+v", wire)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestEncodeRequestWithoutExtensionsLeavesContinuationFieldsEmpty(t *testing.T) {
	wire, warnings := encodeRequest(unified.Request{Model: "openai/test"})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if wire.PreviousResponseID != "" || wire.Store != nil || wire.PromptCacheKey != "" || wire.PromptCacheRetention != "" {
		t.Fatalf("unexpected continuation fields: %+v", wire)
	}
}

func TestEncodeResponseFormat(t *testing.T) {
	wire, _ := encodeRequest(unified.Request{
		Model: "openai/test",
		ResponseFormat: &unified.ResponseFormat{
			Kind:   unified.ResponseFormatJSONSchema,
			Name:   "answer",
			Schema: json.RawMessage(`{"type":"object"}`),
			Strict: true,
		},
	})
	format, ok := wire.Text.Format.(map[string]any)
	if !ok || format["type"] != "json_schema" || format["name"] != "answer" || format["strict"] != true || string(format["schema"].(json.RawMessage)) != `{"type":"object"}` {
		t.Fatalf("text format = %#v", wire.Text.Format)
	}
}

func TestEncodeImageContent(t *testing.T) {
	wire, warnings := encodeRequest(unified.Request{
		Model: "openai/test",
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "describe"},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/image.png"}},
			},
		}},
	})
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(wire.Input) != 1 || len(wire.Input[0].Content) != 2 || wire.Input[0].Content[1].Type != "input_image" || wire.Input[0].Content[1].ImageURL != "https://example.com/image.png" {
		t.Fatalf("input = %+v", wire.Input)
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
