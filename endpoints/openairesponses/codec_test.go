package openairesponses

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

func TestDecodeHTTP(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"instructions":"be brief",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"max_output_tokens":32,
		"stream":true,
		"tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}],
		"tool_choice":{"type":"function","name":"lookup"}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.SourceAPI != adapt.ApiOpenAIResponses || req.Unified.Model != "gpt-test" || req.Unified.MaxOutputTokens == nil || *req.Unified.MaxOutputTokens != 32 || !req.Unified.Stream {
		t.Fatalf("unexpected request: %+v", req.Unified)
	}
	if len(req.Unified.Instructions) != 1 || len(req.Unified.Messages) != 1 {
		t.Fatalf("unexpected messages/instructions: %+v %+v", req.Unified.Messages, req.Unified.Instructions)
	}
	if len(req.Unified.Tools) != 1 || req.Unified.Tools[0].Name != "lookup" {
		t.Fatalf("unexpected tools: %+v", req.Unified.Tools)
	}
	if req.Unified.ToolChoice == nil || req.Unified.ToolChoice.Name != "lookup" {
		t.Fatalf("unexpected tool choice: %+v", req.Unified.ToolChoice)
	}
}

func TestDecodeHTTPFunctionCallOutput(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 1 || req.Unified.Messages[0].Role != unified.RoleTool || len(req.Unified.Messages[0].ToolResults) != 1 {
		t.Fatalf("unexpected messages: %+v", req.Unified.Messages)
	}
}

func TestDecodeHTTPWarnings(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_image","image_url":"x"}]},
			{"type":"web_search_call"}
		],
		"tools":[{"type":"web_search","name":"ignored"}],
		"tool_choice":{"type":"unknown"},
		"provider":{"allow_fallbacks":true},
		"debug":true,
		"session_id":"sess_1"
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertWarning(t, req.Warnings, "input.0.content.1.type")
	assertWarning(t, req.Warnings, "input.1.type")
	assertWarning(t, req.Warnings, "tools.0.type")
	assertWarning(t, req.Warnings, "tool_choice")
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterProvider, `{"allow_fallbacks":true}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterDebug, `true`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterSessionID, `"sess_1"`)
}

func TestWriteEventsNonStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.UsageEvent{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(false), events)
	if err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Object != "response" || resp.Status != "completed" || len(resp.Output) != 1 || resp.Output[0].Content[0].Text != "hello" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestWriteEventsStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"response.created"`) || !strings.Contains(body, `"type":"response.output_text.delta"`) || !strings.Contains(body, `"delta":"hello"`) || !strings.Contains(body, `"type":"response.done"`) {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestWriteEventsStreamingToolCall(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
	events <- unified.ToolCallStartEvent{Index: 0, ID: "call_1", Name: "lookup"}
	events <- unified.ToolCallArgsDeltaEvent{Index: 0, ID: "call_1", Delta: `{"q":"x"}`}
	events <- unified.ToolCallDoneEvent{Index: 0, ID: "call_1", Name: "lookup"}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonToolCall}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"function_call"`) || !strings.Contains(body, `"arguments":"{\"q\":\"x\"}"`) || !strings.Contains(body, `"type":"response.done"`) {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func decodedReq(stream bool) adapt.Request {
	return adapt.Request{Unified: unified.Request{Stream: stream}}
}

func assertWarning(t *testing.T, warnings []adapt.Warning, field string) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == "unsupported_field_dropped" && warning.Field == field {
			return
		}
	}
	t.Fatalf("missing warning for %s: %+v", field, warnings)
}

func assertRawExtension(t *testing.T, extensions unified.Extensions, key, want string) {
	t.Helper()
	raw, ok, err := unified.GetExtension[json.RawMessage](extensions, key)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("missing extension %s", key)
	}
	if string(raw) != want {
		t.Fatalf("extension %s = %s, want %s", key, raw, want)
	}
}
