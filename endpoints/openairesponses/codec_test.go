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

func TestDecodeHTTPPreservesRequestMetadata(t *testing.T) {
	body := `{"model":"gpt-test","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses?trace=1", strings.NewReader(body))
	httpReq.Header.Set("X-Test", "yes")
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if req.SourceAPI != adapt.ApiOpenAIResponses || req.HTTP == nil || req.HTTP.Path != "/v1/responses" || req.HTTP.Query.Get("trace") != "1" || req.HTTP.Headers.Get("X-Test") != "yes" {
		t.Fatalf("unexpected request metadata: %+v", req)
	}
	if string(req.RawBody) != body {
		t.Fatalf("raw body = %s, want %s", req.RawBody, body)
	}
	if _, ok := req.Raw.(Request); !ok {
		t.Fatalf("raw request = %T, want Request", req.Raw)
	}
}

func TestDecodeHTTPEdgeCaseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "invalid json", body: `{`, code: "invalid_json"},
		{name: "missing model", body: `{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`, code: "missing_model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(tt.body))
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

func TestDecodeHTTPEdgeCaseWarnings(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_image"}]},
			{"type":"unknown_future_item"}
		],
		"text":{"format":{"type":"future_format"}}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertWarning(t, req.Warnings, "input.0.content.0")
	assertWarning(t, req.Warnings, "input.1.type")
	assertWarning(t, req.Warnings, "text.format.type")
}

func TestDecodeHTTPWarnings(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"},{"type":"input_audio"},{"type":"input_file","file_id":"file_1"},{"type":"input_video","video_url":"https://example.com/video.mp4"}]},
			{"type":"web_search_call"}
		],
		"tools":[{"type":"web_search","name":"ignored"},{"type":"code_interpreter","name":"ignored"}],
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
	assertWarning(t, req.Warnings, "input.0.content.2.type")
	assertWarning(t, req.Warnings, "input.0.content.3.type")
	assertWarning(t, req.Warnings, "input.1.type")
	assertWarning(t, req.Warnings, "tools.0.type")
	assertWarning(t, req.Warnings, "tools.1.type")
	assertWarning(t, req.Warnings, "tool_choice")
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterProvider, `{"allow_fallbacks":true}`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterDebug, `true`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenRouterSessionID, `"sess_1"`)
}

func TestDecodeHTTPOpenAIResponsesExtensions(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"previous_response_id":"resp_prev",
		"store":false,
		"prompt_cache_key":"cache_key_1",
		"prompt_cache_retention":"24h"
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenAIPreviousResponseID, `"resp_prev"`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenAIStore, `false`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenAIPromptCacheKey, `"cache_key_1"`)
	assertRawExtension(t, req.Unified.Extensions, unified.ExtOpenAIPromptCacheRetention, `"24h"`)
}

func TestDecodeHTTPImageContent(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"describe"},{"type":"input_image","image_url":"https://example.com/image.png"}]}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
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

func TestDecodeHTTPInvalidToolCallArgumentsWarning(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{"}]
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req, err := (Codec{}).DecodeHTTP(context.Background(), httpReq)
	if err != nil {
		t.Fatal(err)
	}
	if string(req.Unified.Messages[0].ToolCalls[0].Arguments) != `{}` {
		t.Fatalf("arguments = %s", req.Unified.Messages[0].ToolCalls[0].Arguments)
	}
	assertWarning(t, req.Warnings, "input.0.arguments")
}

func TestDecodeHTTPResponseFormat(t *testing.T) {
	body := `{
		"model":"gpt-test",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}],
		"text":{"format":{"type":"json_schema","name":"answer","schema":{"type":"object"},"strict":true}}
	}`
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
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
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
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
	if resp.Object != "response" || resp.Status != "completed" || len(resp.Output) != 1 || resp.Output[0].Content[0].Text != "hello" || resp.Usage.TotalTokens != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestWriteEventsNonStreamingCitation(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
	events <- unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.CitationEvent{Index: 0, Citation: unified.Citation{Type: "url_citation", URL: "https://example.test", Title: "Example", Text: "quote", StartOffset: 1, EndOffset: 5, Meta: map[string]any{"source": "web"}}}
	events <- unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText}
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
	annotations := resp.Output[0].Content[0].Annotations
	if len(annotations) != 1 {
		t.Fatalf("annotations = %+v", annotations)
	}
	annotation, ok := annotations[0].(map[string]any)
	if !ok || annotation["url"] != "https://example.test" || annotation["source"] != "web" {
		t.Fatalf("annotation = %#v", annotations[0])
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

func TestWriteEventsStreamingCitation(t *testing.T) {
	events := make(chan unified.Event, 8)
	events <- unified.MessageStartEvent{ID: "resp", Model: "model"}
	events <- unified.TextDeltaEvent{Index: 0, Text: "hello"}
	events <- unified.CitationEvent{Index: 0, Citation: unified.Citation{Type: "url_citation", URL: "https://example.test", Title: "Example", Text: "quote", StartOffset: 1, EndOffset: 5, Meta: map[string]any{"source": "web"}}}
	events <- unified.CompletedEvent{FinishReason: unified.FinishReasonStop}
	close(events)

	w := httptest.NewRecorder()
	err := (Codec{}).WriteEvents(context.Background(), w, decodedReq(true), events)
	if err != nil {
		t.Fatal(err)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"type":"response.output_text.annotation.added"`) || !strings.Contains(body, `"url":"https://example.test"`) || !strings.Contains(body, `"source":"web"`) {
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
