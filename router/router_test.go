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
		Client:      client,
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
	if _, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "missing"},
	}); err == nil {
		t.Fatalf("expected missing route error")
	}
}
