package anthropicmessages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	anthropic "github.com/codewandler/llmadapter/anthropicwire"
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

func TestDecodeHTTPPreservesRequestMetadata(t *testing.T) {
	body := `{"model":"claude-test","max_tokens":32,"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages?trace=1", strings.NewReader(body))
	httpReq.Header.Set("X-Test", "yes")
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.SourceAPI != adapt.ApiAnthropicMessages || req.HTTP == nil || req.HTTP.Path != "/v1/messages" || req.HTTP.Query.Get("trace") != "1" || req.HTTP.Headers.Get("X-Test") != "yes" {
		t.Fatalf("unexpected request metadata: %+v", req)
	}
	if string(req.RawBody) != body {
		t.Fatalf("raw body = %s, want %s", req.RawBody, body)
	}
	if _, ok := req.Raw.(anthropic.MessageRequest); !ok {
		t.Fatalf("raw request = %T, want MessageRequest", req.Raw)
	}
}

func TestDecodeHTTPEdgeCaseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "invalid json", body: `{`, code: "invalid_json"},
		{name: "missing model", body: `{"max_tokens":32,"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`, code: "missing_model"},
		{name: "missing max tokens", body: `{"model":"claude-test","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`, code: "missing_max_tokens"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(tt.body))
			_, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
			if err == nil {
				t.Fatal("expected decode error")
			}
			got, ok := err.(httpError)
			if !ok || got.code != tt.code {
				t.Fatalf("error = %#v, want code %s", err, tt.code)
			}
		})
	}
}

func TestDecodeHTTPSystemCacheControl(t *testing.T) {
	body := `{
		"model":"claude-test",
		"system":[{"type":"text","text":"stable prefix","cache_control":{"type":"ephemeral","ttl":"5m"}}],
		"max_tokens":32,
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Instructions) != 1 || len(req.Unified.Instructions[0].Content) != 1 {
		t.Fatalf("unexpected instructions: %+v", req.Unified.Instructions)
	}
	text, ok := req.Unified.Instructions[0].Content[0].(unified.TextPart)
	if !ok || text.Text != "stable prefix" || text.CacheControl == nil || text.CacheControl.Type != unified.CacheControlEphemeral || text.CacheControl.TTL != "5m" {
		t.Fatalf("unexpected cached system part: %+v", req.Unified.Instructions[0].Content[0])
	}
}

func TestDecodeHTTPReasoningSignature(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"think","signature":"sig"}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 1 || len(req.Unified.Messages[0].Content) != 1 {
		t.Fatalf("messages = %+v", req.Unified.Messages)
	}
	part, ok := req.Unified.Messages[0].Content[0].(unified.ReasoningPart)
	if !ok || part.Text != "think" || part.Signature != "sig" {
		t.Fatalf("reasoning = %+v", req.Unified.Messages[0].Content[0])
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

func TestDecodeHTTPMixedTextAndToolResult(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"also note this"},
			{"type":"tool_result","tool_use_id":"toolu","content":"ok"}
		]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 2 {
		t.Fatalf("unexpected messages: %+v", req.Unified.Messages)
	}
	if req.Unified.Messages[0].Role != unified.RoleUser || len(req.Unified.Messages[0].Content) != 1 {
		t.Fatalf("unexpected user message: %+v", req.Unified.Messages[0])
	}
	if text, ok := req.Unified.Messages[0].Content[0].(unified.TextPart); !ok || text.Text != "also note this" {
		t.Fatalf("unexpected user content: %+v", req.Unified.Messages[0].Content)
	}
	if req.Unified.Messages[1].Role != unified.RoleTool || len(req.Unified.Messages[1].ToolResults) != 1 {
		t.Fatalf("unexpected tool message: %+v", req.Unified.Messages[1])
	}
}

func TestDecodeHTTPWarnings(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"hello"},
			{"type":"document","source":{"type":"url","url":"x"}},
			{"type":"server_tool_use","id":"srv_1","name":"web_search"},
			{"type":"tool_result","tool_use_id":"toolu","content":[{"type":"text","text":"ok"},{"type":"image"},{"type":"document"}]}
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
	assertWarning(t, req.Warnings, "messages.0.content.2.type")
	assertWarning(t, req.Warnings, "messages.0.content.3.content.1")
	assertWarning(t, req.Warnings, "messages.0.content.3.content.2")
	assertWarning(t, req.Warnings, "tool_choice.type")
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterProvider, `{"order":["openai"]}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterTrace, `{"enabled":true}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterSessionID, `"sess_1"`)
}

func TestDecodeHTTPEdgeCaseWarnings(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[
			{"type":"image","source":{"type":"file","file_id":"file_1"}},
			{"type":"tool_result","tool_use_id":"toolu","content":{"type":"text","text":"object shape"}}
		]}],
		"tool_choice":{"type":"future_choice"}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertWarning(t, req.Warnings, "messages.0.content.0.source.type")
	assertWarning(t, req.Warnings, "messages.0.content.1.content")
	assertWarning(t, req.Warnings, "tool_choice.type")
}

func TestDecodeHTTPImageContent(t *testing.T) {
	body := `{
		"model":"claude-test",
		"max_tokens":32,
		"messages":[{"role":"user","content":[
			{"type":"text","text":"describe"},
			{"type":"image","source":{"type":"url","url":"https://example.com/image.png"}}
		]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 1 || len(req.Unified.Messages[0].Content) != 2 {
		t.Fatalf("content = %+v", req.Unified.Messages)
	}
	image, ok := req.Unified.Messages[0].Content[1].(unified.ImagePart)
	if !ok || image.Source.Kind != unified.BlobSourceURL || image.Source.URL != "https://example.com/image.png" {
		t.Fatalf("image = %+v", req.Unified.Messages[0].Content[1])
	}
}

func TestWriteEventsNonStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.NewUsageEvent(unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: 1},
		{Kind: unified.TokenKindOutput, Count: 2},
	}, nil)
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

func TestWriteEventsNonStreamingCitation(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.CitationEvent{Index: 0, Citation: unified.Citation{Type: "char_location", Title: "Manual", Text: "quote", DocumentID: "doc_1", StartOffset: 1, EndOffset: 5, Meta: map[string]any{"source": "pdf"}}}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
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
	if len(resp.Content) != 1 || len(resp.Content[0].Citations) != 1 {
		t.Fatalf("content = %+v", resp.Content)
	}
	citation, ok := resp.Content[0].Citations[0].(map[string]any)
	if !ok || citation["document_id"] != "doc_1" || citation["source"] != "pdf" {
		t.Fatalf("citation = %#v", resp.Content[0].Citations[0])
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

func TestWriteEventsReasoningSignature(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindReasoning}
	events <- unified.ReasoningDeltaEvent{Index: 0, Text: "think"}
	events <- unified.ReasoningDeltaEvent{Index: 0, Signature: "sig"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindReasoning}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"thinking_delta","thinking":"think"`) || !strings.Contains(body, `"type":"signature_delta","signature":"sig"`) {
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
