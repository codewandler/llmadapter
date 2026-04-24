package router

import (
	"context"
	"strings"
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

func TestStaticRouterSkipsCapabilityMismatch(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Endpoint: ProviderEndpoint{
				ProviderName: "minimax",
				APIKind:      adapt.ApiMiniMaxChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Endpoint: ProviderEndpoint{
				ProviderName: "openai",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true, Tools: true},
			},
		},
	)
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified: unified.Request{
			Model:  "model",
			Stream: true,
			Tools:  []unified.Tool{{Kind: unified.ToolKindFunction, Name: "lookup"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.ProviderName != "openai" {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestStaticRouterRanksByRouteWeight(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: ProviderEndpoint{
				ProviderName: "low",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    20,
			Endpoint: ProviderEndpoint{
				ProviderName: "high",
				APIKind:      adapt.ApiOpenRouterChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
	)
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.ProviderName != "high" {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestStaticRouterRanksByEndpointPriorityWhenWeightsTie(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Endpoint: ProviderEndpoint{
				ProviderName: "first",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
				Priority:     10,
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Endpoint: ProviderEndpoint{
				ProviderName: "second",
				APIKind:      adapt.ApiOpenRouterChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
				Priority:     20,
			},
		},
	)
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.ProviderName != "second" {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestStaticRouterRoutesReturnsRankedCandidates(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: ProviderEndpoint{
				ProviderName: "first",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    20,
			Endpoint: ProviderEndpoint{
				ProviderName: "second",
				APIKind:      adapt.ApiOpenRouterChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
	)
	routes, err := r.Routes(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified:   unified.Request{Model: "model"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].ProviderName != "second" || routes[1].ProviderName != "first" {
		t.Fatalf("routes = %+v", routes)
	}
}

func TestStaticRouterAutoSourceConsidersAllRoutesAndRanksBySource(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIResponses,
			Model:     "haiku",
			Weight:    100,
			Endpoint: ProviderEndpoint{
				ProviderName: "claude",
				APIKind:      adapt.ApiAnthropicMessages,
				Family:       adapt.FamilyAnthropicMessages,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiAnthropicMessages,
			Model:     "haiku",
			Weight:    100,
			Endpoint: ProviderEndpoint{
				ProviderName: "claude",
				APIKind:      adapt.ApiAnthropicMessages,
				Family:       adapt.FamilyAnthropicMessages,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
	)
	route, err := r.Route(context.Background(), adapt.Request{
		Unified: unified.Request{Model: "haiku", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.SourceAPI != adapt.ApiAnthropicMessages || route.TargetAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestStaticRouterFallsBackFromHigherWeightCapabilityMismatch(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    100,
			Endpoint: ProviderEndpoint{
				ProviderName: "text",
				APIKind:      adapt.ApiMiniMaxChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true},
			},
		},
		StaticRoute{
			SourceAPI: adapt.ApiOpenAIChatCompletions,
			Weight:    10,
			Endpoint: ProviderEndpoint{
				ProviderName: "tools",
				APIKind:      adapt.ApiOpenAIChatCompletions,
				Family:       adapt.FamilyOpenAIChatCompletions,
				Client:       client,
				Capabilities: CapabilitySet{Streaming: true, Tools: true},
			},
		},
	)
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified: unified.Request{
			Model: "model",
			Tools: []unified.Tool{{Kind: unified.ToolKindFunction, Name: "lookup"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.ProviderName != "tools" {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestStaticRouterRejectsCapabilityMismatch(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Endpoint: ProviderEndpoint{
			ProviderName: "minimax",
			APIKind:      adapt.ApiMiniMaxChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
			Capabilities: CapabilitySet{Streaming: true},
		},
	})
	_, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified: unified.Request{
			Model: "model",
			Tools: []unified.Tool{{Kind: unified.ToolKindFunction, Name: "lookup"}},
		},
	})
	if err == nil {
		t.Fatalf("expected capability error")
	}
	if !strings.Contains(err.Error(), "tools required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStaticRouterRejectsJSONModeMismatch(t *testing.T) {
	client := noopClient{}
	r := NewStaticRouter(StaticRoute{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Endpoint: ProviderEndpoint{
			ProviderName: "text",
			APIKind:      adapt.ApiOpenAIChatCompletions,
			Family:       adapt.FamilyOpenAIChatCompletions,
			Client:       client,
			Capabilities: CapabilitySet{Streaming: true, Tools: true},
		},
	})
	_, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIChatCompletions,
		Unified: unified.Request{
			Model:          "model",
			ResponseFormat: &unified.ResponseFormat{Kind: unified.ResponseFormatJSON},
		},
	})
	if err == nil {
		t.Fatalf("expected capability error")
	}
	if !strings.Contains(err.Error(), "json mode required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
