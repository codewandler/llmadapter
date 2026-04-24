package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
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

	log.Printf("llmadapter gateway listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func buildRouter(cfg config) (router.Router, error) {
	clients := make(map[string]unified.Client)
	for _, provider := range cfg.Providers {
		client, err := buildProvider(provider)
		if err != nil {
			return nil, err
		}
		clients[provider.Name] = client
	}
	routes := make([]router.StaticRoute, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		client, ok := clients[route.Provider]
		if !ok {
			return nil, fmt.Errorf("route references unknown provider %q", route.Provider)
		}
		routes = append(routes, router.StaticRoute{
			SourceAPI:   route.SourceAPI,
			Model:       route.Model,
			NativeModel: route.NativeModel,
			Client:      client,
		})
	}
	return router.NewStaticRouter(routes...), nil
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
