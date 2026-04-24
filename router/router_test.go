package router

import (
	"context"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type noopClient struct{}

func (noopClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	ch := make(chan unified.Event)
	close(ch)
	return ch, nil
}

func TestStaticRouter(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(StaticRoute{
		SourceAPI:   adapt.ApiOpenAIChatCompletions,
		Model:       "public-model",
		NativeModel: "native-model",
		Endpoint: ProviderEndpoint{
			ProviderName: "openrouter",
			APIKind:      adapt.ApiOpenRouterChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
			Capabilities: CapabilitySet{Streaming: true, Tools: true},
		},
	})
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "public-model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.Client == nil || route.NativeModel != "native-model" {
		t.Fatalf("unexpected route: %+v", route)
	}
	if route.TargetAPI != adapt.ApiOpenRouterChatCompletions || route.TargetFamily != adapt.FamilyOpenAIChatCompletions || route.ProviderName != "openrouter" {
		t.Fatalf("unexpected endpoint metadata: %+v", route)
	}
	if !route.Capabilities.Streaming || !route.Capabilities.Tools {
		t.Fatalf("unexpected route: %+v", route)
	}
	if _, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "missing"},
	}); err == nil {
		t.Fatalf("expected missing route error")
	}
}
