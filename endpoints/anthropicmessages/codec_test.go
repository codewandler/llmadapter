package anthropicmessages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	"github.com/codewandler/llmadapter/unified"
)

func TestDecodeHTTP(t *testing.T) {
	body := `{
		"model":"claude-test",
		"system":"be brief",
		"max_tokens":32,
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],
		"tools":[{"name":"lookup","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"tool","name":"lookup"},
		"stream":true
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.SourceAPI != adapt.ApiAnthropicMessages || req.Unified.Model != "claude-test" || req.Unified.MaxOutputTokens == nil || *req.Unified.MaxOutputTokens != 32 || !req.Unified.Stream {
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

func TestDecodeHTTPToolResult(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu","content":"ok"}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 1 || req.Unified.Messages[0].Role != unified.RoleTool || len(req.Unified.Messages[0].ToolResults) != 1 {
		t.Fatalf("unexpected messages: %+v", req.Unified.Messages)
	}
	if req.Unified.Messages[0].ToolResults[0].ToolCallID != "toolu" {
		t.Fatalf("unexpected tool result: %+v", req.Unified.Messages[0].ToolResults[0])
	}
}

func TestDecodeHTTPWarnings(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"hello"},
			{"type":"image","source":{"type":"url","url":"x"}},
			{"type":"tool_result","tool_use_id":"toolu","content":[{"type":"text","text":"ok"},{"type":"image"}]}
		]}],
		"tool_choice":{"type":"unknown"},
		"provider":{"order":["openai"]},
		"trace":{"enabled":true},
		"session_id":"sess_1"
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertWarning(t, req.Warnings, "messages.0.content.1.type")
	assertWarning(t, req.Warnings, "messages.0.content.2.content.1")
	assertWarning(t, req.Warnings, "tool_choice.type")
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterProvider, `{"order":["openai"]}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterTrace, `{"enabled":true}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterSessionID, `"sess_1"`)
}

func TestWriteEventsNonStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.UsageEvent{InputTokens: 1, OutputTokens: 2}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(false), events)
	if err != nil {
		t.Fatal(err)
	}
	var resp anthropic.MessageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Type != "message" || resp.Role != "assistant" || len(resp.Content) != 1 || resp.Content[0].Text != "hello" || resp.StopReason != "end_turn" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Usage == nil || resp.Usage.InputTokens != 1 || resp.Usage.OutputTokens != 2 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}

func TestWriteEventsStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
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
	if !strings.Contains(body, "event: message_start") || !strings.Contains(body, `"text":"hello"`) || !strings.Contains(body, "event: message_stop") {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestWriteEventsStreamingToolCall(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ToolCallStartEvent{Index: 0, ID: "toolu", Name: "lookup"}
	events <- unified.ToolCallArgsDeltaEvent{Index: 0, ID: "toolu", Delta: `{"q":"x"}`}
	events <- unified.ToolCallDoneEvent{Index: 0, ID: "toolu", Name: "lookup"}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonToolCall}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"tool_use"`) || !strings.Contains(body, `"partial_json":"{\"q\":\"x\"}"`) || !strings.Contains(body, `"stop_reason":"tool_use"`) {
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
