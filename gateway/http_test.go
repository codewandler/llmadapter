package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	anthropicendpoint "github.com/codewandler/llmadapter/endpoints/anthropicmessages"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	responsesendpoint "github.com/codewandler/llmadapter/endpoints/openairesponses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type staticClient struct {
	req    unified.Request
	events []unified.Event
	err    error
	calls  int
}

func TestHandlerWritesEndpointErrorBeforeResponseStarts(t *testing.T) {
	client := &staticClient{events: []unified.Event{
		unified.ErrorEvent{Err: errors.New("provider failed")},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Endpoint: router.ProviderEndpoint{
			ProviderName: "test",
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "provider failed") {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func (c *staticClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	c.req = req
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	ch := make(chan unified.Event, len(c.events))
	for _, ev := range c.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestHandlerFallsBackWhenClientRequestFailsBeforeResponseStarts(t *testing.T) {
	primary := &staticClient{err: errors.New("primary unavailable")}
	fallback := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "fallback-model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "fallback"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(
		router.StaticRoute{
			SourceAPI:   adapt.ApiOpenAIChatCompletions,
			NativeModel: "primary-model",
			Weight:      100,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "primary",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       primary,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
		router.StaticRoute{
			SourceAPI:   adapt.ApiOpenAIChatCompletions,
			NativeModel: "fallback-model",
			Weight:      10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       fallback,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
	)}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("calls primary=%d fallback=%d", primary.calls, fallback.calls)
	}
	if primary.req.Model != "primary-model" || fallback.req.Model != "fallback-model" {
		t.Fatalf("models primary=%q fallback=%q", primary.req.Model, fallback.req.Model)
	}
	if !strings.Contains(w.Body.String(), `"content":"fallback"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerFallsBackWhenEndpointReturnsErrorBeforeResponseStarts(t *testing.T) {
	primary := &staticClient{events: []unified.Event{unified.ErrorEvent{Err: errors.New("primary stream failed")}}}
	fallback := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "fallback-model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "fallback"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    100,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "primary",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       primary,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       fallback,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
	)}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("calls primary=%d fallback=%d", primary.calls, fallback.calls)
	}
	if !strings.Contains(w.Body.String(), `"content":"fallback"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerRespectsMaxAttempts(t *testing.T) {
	primary := &staticClient{err: errors.New("primary down")}
	second := &staticClient{err: errors.New("second down")}
	third := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "third-model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "third"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{
		Endpoint:    chat.Codec{},
		MaxAttempts: 2,
		Router: router.NewStaticRouter(
			staticGatewayRoute("primary", 100, primary),
			staticGatewayRoute("second", 50, second),
			staticGatewayRoute("third", 10, third),
		),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if primary.calls != 1 || second.calls != 1 || third.calls != 0 {
		t.Fatalf("unexpected calls: primary=%d second=%d third=%d", primary.calls, second.calls, third.calls)
	}
	if !strings.Contains(w.Body.String(), "primary down") || !strings.Contains(w.Body.String(), "second down") || strings.Contains(w.Body.String(), "third") {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerDoesNotFallbackOnNonRetryableError(t *testing.T) {
	primary := &staticClient{err: &adapt.UnsupportedFieldError{APIKind: adapt.ApiAnthropicMessages, Field: "audio", Reason: "not supported"}}
	fallback := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "fallback-model"},
		unified.TextDeltaEvent{Index: 0, Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(
		staticGatewayRoute("primary", 100, primary),
		staticGatewayRoute("fallback", 10, fallback),
	)}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if primary.calls != 1 || fallback.calls != 0 {
		t.Fatalf("unexpected calls: primary=%d fallback=%d", primary.calls, fallback.calls)
	}
	if !strings.Contains(w.Body.String(), "does not support field audio") {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerDoesNotFallBackAfterResponseStarts(t *testing.T) {
	primary := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "primary-model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "partial"},
		unified.ErrorEvent{Err: errors.New("primary stream failed")},
	}}
	fallback := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "fallback-model"},
		unified.TextDeltaEvent{Index: 0, Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    100,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "primary",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       primary,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       fallback,
				Capabilities: router.CapabilitySet{Streaming: true},
			},
		},
	)}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}],
		"stream":true
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if fallback.calls != 0 {
		t.Fatalf("fallback should not be called after response starts")
	}
	if !strings.Contains(w.Body.String(), `"content":"partial"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func staticGatewayRoute(provider string, weight int, client unified.Client) router.StaticRoute {
	return router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Weight:    weight,
		Endpoint: router.ProviderEndpoint{
			ProviderName: provider,
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
			Capabilities: router.CapabilitySet{Streaming: true},
		},
	}
}

func TestHandlerDeprioritizesRecentlyFailedRoute(t *testing.T) {
	primary := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "primary-model"},
		unified.TextDeltaEvent{Index: 0, Text: "primary"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	fallback := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "fallback-model"},
		unified.TextDeltaEvent{Index: 0, Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	health := NewHealthTracker(time.Minute)
	primaryRoute := router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Weight:    100,
		Endpoint: router.ProviderEndpoint{
			ProviderName: "primary",
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       primary,
		},
	}
	health.MarkFailure(router.Route{ProviderName: "primary", TargetAPI: adapt.ApiOpenAIChatCompletions, PublicModel: "model"})
	handler := Handler{Endpoint: chat.Codec{}, Health: health, Router: router.NewStaticRouter(
		primaryRoute,
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       fallback,
			},
		},
	)}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if primary.calls != 0 || fallback.calls != 1 {
		t.Fatalf("calls primary=%d fallback=%d", primary.calls, fallback.calls)
	}
	if !strings.Contains(w.Body.String(), `"content":"fallback"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHealthTrackerKeysFailuresByNativeModel(t *testing.T) {
	health := NewHealthTracker(time.Minute)
	failed := router.Route{ProviderName: "provider", TargetAPI: adapt.ApiOpenAIChatCompletions, NativeModel: "model-a"}
	health.MarkFailure(failed)

	routes := []router.Route{
		{ProviderName: "provider", TargetAPI: adapt.ApiOpenAIChatCompletions, NativeModel: "model-b"},
		{ProviderName: "provider", TargetAPI: adapt.ApiOpenAIChatCompletions, NativeModel: "model-a"},
	}
	ordered := orderByHealth(routes, health)
	if ordered[0].NativeModel != "model-b" || ordered[1].NativeModel != "model-a" {
		t.Fatalf("unexpected health order: %+v", ordered)
	}
}

func TestHandlerChatCompletions(t *testing.T) {
	client := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "pong"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: chat.Codec{}, Router: router.NewStaticRouter(router.StaticRoute{
		SourceAPI:   adapt.ApiOpenAIChatCompletions,
		NativeModel: "native-model",
		Endpoint: router.ProviderEndpoint{
			ProviderName: "test",
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"model",
		"messages":[{"role":"user","content":"ping"}],
		"max_tokens":16
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if client.req.Model != "native-model" || len(client.req.Messages) != 1 {
		t.Fatalf("unexpected client request: %+v", client.req)
	}
	if !strings.Contains(w.Body.String(), `"content":"pong"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerAnthropicMessages(t *testing.T) {
	client := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "msg", Model: "model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "pong"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: anthropicendpoint.Codec{}, Router: router.NewStaticRouter(router.StaticRoute{
		SourceAPI:   adapt.ApiAnthropicMessages,
		NativeModel: "native-claude",
		Endpoint: router.ProviderEndpoint{
			ProviderName: "test",
			APIKind:      adapt.ApiAnthropicMessages,
			Family:       adapt.FamilyAnthropicMessages,
			Client:       client,
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{
		"model":"claude",
		"max_tokens":16,
		"messages":[{"role":"user","content":[{"type":"text","text":"ping"}]}]
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if client.req.Model != "native-claude" || len(client.req.Messages) != 1 || client.req.MaxOutputTokens == nil || *client.req.MaxOutputTokens != 16 {
		t.Fatalf("unexpected client request: %+v", client.req)
	}
	if !strings.Contains(w.Body.String(), `"text":"pong"`) || !strings.Contains(w.Body.String(), `"role":"assistant"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}

func TestHandlerOpenAIResponses(t *testing.T) {
	client := &staticClient{events: []unified.Event{
		unified.MessageStartEvent{ID: "resp", Model: "model"},
		unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText},
		unified.TextDeltaEvent{Index: 0, Text: "pong"},
		unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	handler := Handler{Endpoint: responsesendpoint.Codec{}, Router: router.NewStaticRouter(router.StaticRoute{
		SourceAPI:   adapt.ApiOpenAIResponses,
		NativeModel: "native-gpt",
		Endpoint: router.ProviderEndpoint{
			ProviderName: "test",
			APIKind:      adapt.ApiOpenAIResponses,
			Family:       adapt.FamilyOpenAIResponses,
			Client:       client,
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"gpt",
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"ping"}]}],
		"max_output_tokens":16
	}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if client.req.Model != "native-gpt" || len(client.req.Messages) != 1 || client.req.MaxOutputTokens == nil || *client.req.MaxOutputTokens != 16 {
		t.Fatalf("unexpected client request: %+v", client.req)
	}
	if !strings.Contains(w.Body.String(), `"text":"pong"`) || !strings.Contains(w.Body.String(), `"object":"response"`) {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}
}
