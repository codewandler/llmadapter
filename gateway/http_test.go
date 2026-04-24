package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type staticClient struct {
	req    unified.Request
	events []unified.Event
	err    error
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
