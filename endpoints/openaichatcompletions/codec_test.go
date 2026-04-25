package openaichatcompletions

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
		"model":"test-model",
		"messages":[
			{"role":"system","content":"be brief"},
			{"role":"user","content":"hello"}
		],
		"max_tokens":32,
		"stream":true,
		"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}}],
		"tool_choice":{"type":"function","function":{"name":"lookup"}}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.Unified.Model != "test-model" || req.Unified.MaxOutputTokens == nil || *req.Unified.MaxOutputTokens != 32 || !req.Unified.Stream {
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
	body := `{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?trace=1", strings.NewReader(body))
	httpReq.Header.Set("X-Test", "yes")
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.SourceAPI != adapt.ApiOpenAIChatCompletions || req.HTTP == nil || req.HTTP.Path != "/v1/chat/completions" || req.HTTP.Query.Get("trace") != "1" || req.HTTP.Headers.Get("X-Test") != "yes" {
		t.Fatalf("unexpected request metadata: %+v", req)
	}
	if string(req.RawBody) != body {
		t.Fatalf("raw body = %s, want %s", req.RawBody, body)
	}
	if _, ok := req.Raw.(Request); !ok {
		t.Fatalf("raw request = %T, want Request", req.Raw)
	}
}

func TestDecodeHTTPWarnings(t *testing.T) {
	body := `{
		"model":"test-model",
		"messages":[{"role":"user","content":[{"type":"text","text":"hello"},{"type":"input_audio","input_audio":{"data":"x"}},{"type":"input_file","file_id":"file_1"}]}],
		"stop":["ok",42],
		"tools":[{"type":"web_search","function":{"name":"ignored"}}],
		"tool_choice":{"type":"unknown"},
		"provider":{"order":["anthropic"]},
		"plugins":[{"id":"web"}],
		"session_id":"sess_1"
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertWarning(t, req.Warnings, "messages.0.content.1.type")
	assertWarning(t, req.Warnings, "messages.0.content.2.type")
	assertWarning(t, req.Warnings, "stop.1")
	assertWarning(t, req.Warnings, "tools.0.type")
	assertWarning(t, req.Warnings, "tool_choice")
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterProvider, `{"order":["anthropic"]}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterPlugins, `[{"id":"web"}]`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterSessionID, `"sess_1"`)
}

func TestDecodeHTTPImageContent(t *testing.T) {
	body := `{
		"model":"test-model",
		"messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"https://example.com/image.png","detail":"low"}}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Unified.Messages) != 1 || len(req.Unified.Messages[0].Content) != 2 {
		t.Fatalf("content = %+v", req.Unified.Messages)
	}
	image, ok := req.Unified.Messages[0].Content[1].(unified.ImagePart)
	if !ok || image.Source.Kind != unified.BlobSourceURL || image.Source.URL != "https://example.com/image.png" || image.Source.Meta["detail"] != "low" {
		t.Fatalf("image = %+v", req.Unified.Messages[0].Content[1])
	}
}

func TestDecodeHTTPInvalidToolCallArgumentsWarning(t *testing.T) {
	body := `{
		"model":"test-model",
		"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{"}}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if string(req.Unified.Messages[0].ToolCalls[0].Arguments) != `{}` {
		t.Fatalf("arguments = %s", req.Unified.Messages[0].ToolCalls[0].Arguments)
	}
	assertWarning(t, req.Warnings, "messages.0.tool_calls.0.function.arguments")
}

func TestDecodeHTTPResponseFormat(t *testing.T) {
	body := `{
		"model":"test-model",
		"messages":[{"role":"user","content":"hello"}],
		"response_format":{
			"type":"json_schema",
			"json_schema":{"name":"answer","schema":{"type":"object"},"strict":true}
		}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	format := req.Unified.ResponseFormat
	if format == nil || format.Kind != unified.ResponseFormatJSONSchema || format.Name != "answer" || !format.Strict || string(format.Schema) != `{"type":"object"}` {
		t.Fatalf("response format = %+v", format)
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
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Object != "chat.completion" || resp.Choices[0].Message.Content != "hello" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestWriteEventsNonStreamingSeparatesReasoning(t *testing.T) {
	events := make(chan unified.Event, 12)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindReasoning}
	events <- unified.ReasoningDeltaEvent{Index: 0, Text: "think"}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindReasoning}
	events <- unified.ContentBlockStartEvent{Index: 1, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 1, Text: "answer"}
	events <- unified.ContentBlockDoneEvent{Index: 1, Kind: unified.ContentKindText}
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
	msg := resp.Choices[0].Message
	if msg.Content != "answer" {
		t.Fatalf("content = %q", msg.Content)
	}
	if len(msg.ReasoningDetails) != 1 || msg.ReasoningDetails[0].Text != "think" {
		t.Fatalf("reasoning = %+v", msg.ReasoningDetails)
	}
}

func TestWriteEventsStreaming(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"object":"chat.completion.chunk"`) || !strings.Contains(body, `"content":"hello"`) || !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("unexpected stream body: %s", body)
	}
}

func TestWriteEventsStreamingSeparatesReasoning(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "msg", Model: "model"}
	events <- unified.ReasoningDeltaEvent{Index: 0, Text: "think"}
	events <- unified.TextDeltaEvent{Index: 1, Text: "answer"}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"reasoning_details":[{"type":"text","text":"think"}]`) || !strings.Contains(body, `"content":"answer"`) {
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
