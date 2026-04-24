package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	anthropicendpoint "github.com/codewandler/llmadapter/endpoints/anthropicmessages"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	responsesendpoint "github.com/codewandler/llmadapter/endpoints/openairesponses"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimax "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

func main() {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if err := validateConfig(cfg); err != nil {
		log.Fatal(err)
	}
	r, err := buildRouter(cfg)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", gateway.Handler{
		Endpoint: chat.Codec{},
		Router:   r,
	})
	mux.Handle("/v1/messages", gateway.Handler{
		Endpoint: anthropicendpoint.Codec{},
		Router:   r,
	})
	mux.Handle("/v1/responses", gateway.Handler{
		Endpoint: responsesendpoint.Codec{},
		Router:   r,
	})

	log.Printf("llmadapter gateway listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func buildRouter(cfg config) (router.Router, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		endpoint, err := buildProviderEndpoint(provider)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	routes := make([]router.StaticRoute, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		endpoint, ok, ambiguous := findProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
		if ambiguous {
			return nil, fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
		}
		if !ok {
			return nil, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
		}
		routes = append(routes, router.StaticRoute{
			SourceAPI:   route.SourceAPI,
			Model:       route.Model,
			NativeModel: route.NativeModel,
			Weight:      route.Weight,
			Endpoint:    endpoint,
		})
	}
	return router.NewStaticRouter(routes...), nil
}

func buildProviderEndpoint(provider providerConfig) (router.ProviderEndpoint, error) {
	client, err := buildProvider(provider)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	apiKind, family, capabilities, err := providerEndpointMetadata(provider.Type)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	return router.ProviderEndpoint{
		ProviderName: provider.Name,
		APIKind:      apiKind,
		Family:       family,
		Client:       client,
		Capabilities: capabilities,
		Priority:     provider.Priority,
	}, nil
}

func providerEndpointMetadata(providerType string) (adapt.ApiKind, adapt.ApiFamily, router.CapabilitySet, error) {
	switch providerType {
	case "anthropic":
		return adapt.ApiAnthropicMessages, adapt.FamilyAnthropicMessages, router.CapabilitySet{Streaming: true, Tools: true, Vision: true}, nil
	case "openai_chat":
		return adapt.ApiOpenAIChatCompletions, adapt.FamilyOpenAIChatCompletions, router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true}, nil
	case "openrouter_chat":
		return adapt.ApiOpenRouterChatCompletions, adapt.FamilyOpenAIChatCompletions, router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true}, nil
	case "openrouter_responses":
		return adapt.ApiOpenRouterResponses, adapt.FamilyOpenAIResponses, router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true}, nil
	case "openrouter_messages":
		return adapt.ApiOpenRouterAnthropicMessages, adapt.FamilyAnthropicMessages, router.CapabilitySet{Streaming: true, Tools: true, Vision: true}, nil
	case "minimax_chat":
		return adapt.ApiMiniMaxChatCompletions, adapt.FamilyOpenAIChatCompletions, router.CapabilitySet{Streaming: true}, nil
	case "minimax_messages":
		return adapt.ApiMiniMaxAnthropicMessages, adapt.FamilyAnthropicMessages, router.CapabilitySet{Streaming: true, Tools: true}, nil
	default:
		return "", "", router.CapabilitySet{}, fmt.Errorf("unsupported provider type %q", providerType)
	}
}

func findProviderEndpoint(endpoints []router.ProviderEndpoint, providerName string, apiKind adapt.ApiKind) (router.ProviderEndpoint, bool, bool) {
	var match router.ProviderEndpoint
	matches := 0
	for _, endpoint := range endpoints {
		if endpoint.ProviderName != providerName {
			continue
		}
		if apiKind != "" && endpoint.APIKind != apiKind {
			continue
		}
		match = endpoint
		matches++
	}
	if matches == 1 {
		return match, true, false
	}
	return router.ProviderEndpoint{}, false, matches > 1
}

func buildProvider(provider providerConfig) (unified.Client, error) {
	switch provider.Type {
	case "anthropic":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []anthropic.Option{
			anthropic.WithAPIKey(apiKey),
			anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
				// The Anthropic client path is stream-first; endpoint codecs can still collect into non-stream JSON.
				req.Unified.Stream = true
				return nil
			})),
		}
		if provider.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(provider.BaseURL))
		}
		return anthropic.NewClient(opts...)
	case "openai_chat":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("OPENAI_API_KEY", "OPENAI_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openai.Option{openai.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(provider.BaseURL))
		}
		return openai.NewClient(opts...)
	case "openrouter_chat":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("OPENROUTER_API_KEY", "OPENROUTER_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openrouter.Option{openrouter.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openrouter.WithBaseURL(provider.BaseURL))
		}
		return openrouter.NewClient(opts...)
	case "openrouter_responses":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("OPENROUTER_API_KEY", "OPENROUTER_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openrouterresponses.Option{openrouterresponses.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openrouterresponses.WithBaseURL(provider.BaseURL))
		}
		return openrouterresponses.NewClient(opts...)
	case "openrouter_messages":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("OPENROUTER_API_KEY", "OPENROUTER_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openroutermessages.Option{openroutermessages.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openroutermessages.WithBaseURL(provider.BaseURL))
		}
		return openroutermessages.NewClient(opts...)
	case "minimax_chat":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("MINIMAX_API_KEY", "MINIMAX_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []minimax.Option{minimax.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, minimax.WithBaseURL(provider.BaseURL))
		}
		return minimax.NewClient(opts...)
	case "minimax_messages":
		apiKey := provider.APIKey
		if apiKey == "" && provider.APIKeyEnv != "" {
			apiKey = os.Getenv(provider.APIKeyEnv)
		}
		if apiKey == "" {
			apiKey = firstEnv("MINIMAX_API_KEY", "MINIMAX_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []minimaxmessages.Option{minimaxmessages.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, minimaxmessages.WithBaseURL(provider.BaseURL))
		}
		return minimaxmessages.NewClient(opts...)
	default:
		return nil, fmt.Errorf("unsupported provider type %q", provider.Type)
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
