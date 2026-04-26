package muxclient

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type fakeClient struct {
	req    unified.Request
	events []unified.Event
	err    error
	calls  int
}

func (c *fakeClient) Request(_ context.Context, req unified.Request) (<-chan unified.Event, error) {
	c.req = req
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	out := make(chan unified.Event, len(c.events))
	for _, event := range c.events {
		out <- event
	}
	close(out)
	return out, nil
}

func TestClientRoutesAndRewritesNativeModel(t *testing.T) {
	provider := &fakeClient{events: []unified.Event{
		unified.ProviderExecutionEvent{InternalContinuation: unified.ContinuationPreviousResponseID, Transport: unified.TransportHTTPSSE},
		unified.TextDeltaEvent{Text: "ok"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(router.StaticRoute{
		SourceAPI:   adapt.ApiOpenAIResponses,
		Model:       "public",
		NativeModel: "native",
		Endpoint: router.ProviderEndpoint{
			ProviderName: "openai",
			APIKind:      adapt.ApiOpenAIResponses,
			Family:       adapt.FamilyOpenAIResponses,
			Client:       provider,
			Capabilities: router.CapabilitySet{Streaming: true},
		},
	}))

	events, err := client.Request(context.Background(), unified.Request{Model: "public", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	routeEvent, ok := first.(unified.RouteEvent)
	if !ok {
		t.Fatalf("first event = %T, want unified.RouteEvent", first)
	}
	if routeEvent.ProviderName != "openai" || routeEvent.PublicModel != "public" || routeEvent.NativeModel != "native" {
		t.Fatalf("unexpected route event: %+v", routeEvent)
	}
	if routeEvent.InternalContinuation != unified.ContinuationPreviousResponseID || routeEvent.Transport != unified.TransportHTTPSSE {
		t.Fatalf("runtime metadata not applied to route event: %+v", routeEvent)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content[0].(unified.TextPart).Text != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if provider.calls != 1 || provider.req.Model != "native" {
		t.Fatalf("unexpected provider request: calls=%d req=%+v", provider.calls, provider.req)
	}
}

func TestClientFallsBackWhenRequestFails(t *testing.T) {
	primary := &fakeClient{err: errors.New("down")}
	fallback := &fakeClient{events: []unified.Event{
		unified.TextDeltaEvent{Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(
		router.StaticRoute{
			SourceAPI:   adapt.ApiOpenAIResponses,
			NativeModel: "primary",
			Weight:      100,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "primary",
				APIKind:      adapt.ApiOpenAIResponses,
				Family:       adapt.FamilyOpenAIResponses,
				Client:       primary,
			},
		},
		router.StaticRoute{
			SourceAPI:   adapt.ApiOpenAIResponses,
			NativeModel: "fallback",
			Weight:      10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIResponses,
				Family:       adapt.FamilyOpenAIResponses,
				Client:       fallback,
			},
		},
	))

	events, err := client.Request(context.Background(), unified.Request{Model: "public"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content[0].(unified.TextPart).Text != "fallback" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if primary.calls != 1 || fallback.calls != 1 || fallback.req.Model != "fallback" {
		t.Fatalf("unexpected provider calls: primary=%d fallback=%d req=%+v", primary.calls, fallback.calls, fallback.req)
	}
}

func TestClientFallbackDisabledReturnsRouteAttemptError(t *testing.T) {
	primary := &fakeClient{err: errors.New("down")}
	client := New(router.NewStaticRouter(router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIResponses,
		Endpoint: router.ProviderEndpoint{
			ProviderName: "primary",
			APIKind:      adapt.ApiOpenAIResponses,
			Family:       adapt.FamilyOpenAIResponses,
			Client:       primary,
		},
	}), WithFallback(false))

	_, err := client.Request(context.Background(), unified.Request{Model: "public"})
	if err == nil {
		t.Fatalf("expected request error")
	}
	if !strings.Contains(err.Error(), "provider primary/openai.responses failed: down") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientRespectsMaxAttempts(t *testing.T) {
	primary := &fakeClient{err: errors.New("primary down")}
	second := &fakeClient{err: errors.New("second down")}
	third := &fakeClient{events: []unified.Event{
		unified.TextDeltaEvent{Text: "third"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(
		staticMuxRoute("primary", 100, primary),
		staticMuxRoute("second", 50, second),
		staticMuxRoute("third", 10, third),
	), WithMaxAttempts(2))

	_, err := client.Request(context.Background(), unified.Request{Model: "public"})
	if err == nil {
		t.Fatalf("expected max-attempt-limited failure")
	}
	if primary.calls != 1 || second.calls != 1 || third.calls != 0 {
		t.Fatalf("unexpected calls: primary=%d second=%d third=%d", primary.calls, second.calls, third.calls)
	}
	if !strings.Contains(err.Error(), "primary down") || !strings.Contains(err.Error(), "second down") || strings.Contains(err.Error(), "third") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDoesNotFallbackOnNonRetryableError(t *testing.T) {
	primary := &fakeClient{err: &adapt.UnsupportedFieldError{APIKind: adapt.ApiAnthropicMessages, Field: "audio", Reason: "not supported"}}
	fallback := &fakeClient{events: []unified.Event{
		unified.TextDeltaEvent{Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(
		staticMuxRoute("primary", 100, primary),
		staticMuxRoute("fallback", 10, fallback),
	))

	_, err := client.Request(context.Background(), unified.Request{Model: "public"})
	if err == nil {
		t.Fatalf("expected non-retryable failure")
	}
	if primary.calls != 1 || fallback.calls != 0 {
		t.Fatalf("unexpected calls: primary=%d fallback=%d", primary.calls, fallback.calls)
	}
	if !strings.Contains(err.Error(), "does not support field audio") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientDoesNotFallBackAfterStreamStarts(t *testing.T) {
	primary := &fakeClient{events: []unified.Event{
		unified.TextDeltaEvent{Text: "partial"},
		unified.ErrorEvent{Err: errors.New("stream failed")},
	}}
	fallback := &fakeClient{events: []unified.Event{
		unified.TextDeltaEvent{Text: "fallback"},
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIResponses,
			Weight:    100,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "primary",
				APIKind:      adapt.ApiOpenAIResponses,
				Family:       adapt.FamilyOpenAIResponses,
				Client:       primary,
			},
		},
		router.StaticRoute{
			SourceAPI: adapt.ApiOpenAIResponses,
			Weight:    10,
			Endpoint: router.ProviderEndpoint{
				ProviderName: "fallback",
				APIKind:      adapt.ApiOpenAIResponses,
				Family:       adapt.FamilyOpenAIResponses,
				Client:       fallback,
			},
		},
	))

	events, err := client.Request(context.Background(), unified.Request{Model: "public"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = unified.Collect(context.Background(), events)
	if err == nil || !strings.Contains(err.Error(), "stream failed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if primary.calls != 1 || fallback.calls != 0 {
		t.Fatalf("unexpected calls: primary=%d fallback=%d", primary.calls, fallback.calls)
	}
}

func staticMuxRoute(provider string, weight int, client unified.Client) router.StaticRoute {
	return router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIResponses,
		Weight:    weight,
		Endpoint: router.ProviderEndpoint{
			ProviderName: provider,
			APIKind:      adapt.ApiOpenAIResponses,
			Family:       adapt.FamilyOpenAIResponses,
			Client:       client,
		},
	}
}

func TestClientNoRoute(t *testing.T) {
	client := New(router.NewStaticRouter())
	_, err := client.Request(context.Background(), unified.Request{Model: "missing"})
	if err == nil {
		t.Fatalf("expected no route error")
	}
}

func TestClientRespectsSourceAPI(t *testing.T) {
	provider := &fakeClient{}
	client := New(router.NewStaticRouter(router.StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Endpoint: router.ProviderEndpoint{
			ProviderName: "openai",
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       provider,
		},
	}), WithSourceAPI(adapt.ApiOpenAIResponses))

	_, err := client.Request(context.Background(), unified.Request{Model: "model"})
	if err == nil {
		t.Fatalf("expected source api route mismatch")
	}
	if provider.calls != 0 {
		t.Fatalf("provider should not be called")
	}
}

func TestClientDefaultSourceIsAuto(t *testing.T) {
	provider := &fakeClient{events: []unified.Event{
		unified.CompletedEvent{FinishReason: unified.FinishReasonStop},
	}}
	client := New(router.NewStaticRouter(router.StaticRoute{
		SourceAPI:   adapt.ApiAnthropicMessages,
		Model:       "haiku",
		NativeModel: "claude-haiku",
		Endpoint: router.ProviderEndpoint{
			ProviderName: "claude",
			APIKind:      adapt.ApiAnthropicMessages,
			Family:       adapt.FamilyAnthropicMessages,
			Client:       provider,
		},
	}))

	events, err := client.Request(context.Background(), unified.Request{Model: "haiku"})
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	routeEvent, ok := first.(unified.RouteEvent)
	if !ok {
		t.Fatalf("first event = %T, want unified.RouteEvent", first)
	}
	if routeEvent.SourceAPI != string(adapt.ApiAnthropicMessages) || provider.req.Model != "claude-haiku" {
		t.Fatalf("unexpected route/provider request: route=%+v req=%+v", routeEvent, provider.req)
	}
}
