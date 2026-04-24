package muxclient

import (
	"context"
	"errors"
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
