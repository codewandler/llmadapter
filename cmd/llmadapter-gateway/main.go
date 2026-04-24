package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	anthropicendpoint "github.com/codewandler/llmadapter/endpoints/anthropicmessages"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	responsesendpoint "github.com/codewandler/llmadapter/endpoints/openairesponses"
	"github.com/codewandler/llmadapter/gateway"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/pipeline"
	pricingpkg "github.com/codewandler/llmadapter/pricing"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimax "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

const tagModelDBServiceID = "modeldb.service_id"

func main() {
	inspectConfigFlag := flag.Bool("inspect-config", false, "print resolved gateway config metadata as JSON and exit")
	flag.Parse()

	cfg, err := loadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	if err := validateConfig(cfg); err != nil {
		log.Fatal(err)
	}
	if *inspectConfigFlag {
		inspection, err := inspectConfig(cfg)
		if err != nil {
			log.Fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inspection); err != nil {
			log.Fatal(err)
		}
		return
	}
	r, err := buildRouter(cfg)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	cooldown, err := healthCooldown(cfg)
	if err != nil {
		log.Fatal(err)
	}
	health := gateway.NewHealthTracker(cooldown)
	mux.Handle("/v1/chat/completions", gateway.Handler{
		Endpoint: chat.Codec{},
		Router:   r,
		Health:   health,
	})
	mux.Handle("/v1/messages", gateway.Handler{
		Endpoint: anthropicendpoint.Codec{},
		Router:   r,
		Health:   health,
	})
	mux.Handle("/v1/responses", gateway.Handler{
		Endpoint: responsesendpoint.Codec{},
		Router:   r,
		Health:   health,
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
	catalog, modelDBEnabled, err := modelDBCatalog(cfg)
	if err != nil {
		return nil, err
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
		if modelDBEnabled {
			var err error
			route, err = resolveRouteModelDBModel(route, endpoint, catalog, cfg.ModelDB)
			if err != nil {
				return nil, err
			}
			endpoint = endpointWithModelDBMetadata(endpoint, route, catalog)
			endpoint = endpointWithPricing(endpoint, route, catalog)
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
	endpoint, err := providerEndpointConfig(provider)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	endpoint.Client = client
	return endpoint, nil
}

func providerEndpointConfig(provider providerConfig) (router.ProviderEndpoint, error) {
	apiKind, family, capabilities, err := providerEndpointMetadata(provider.Type)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	capabilities = applyCapabilityOverrides(capabilities, provider.Capabilities)
	return router.ProviderEndpoint{
		ProviderName: provider.Name,
		APIKind:      apiKind,
		Family:       family,
		Capabilities: capabilities,
		Priority:     provider.Priority,
		Tags:         providerEndpointTags(provider),
	}, nil
}

func providerEndpointTags(provider providerConfig) map[string]string {
	serviceID := providerModelDBServiceID(provider)
	if serviceID == "" {
		return nil
	}
	return map[string]string{tagModelDBServiceID: serviceID}
}

func providerModelDBServiceID(provider providerConfig) string {
	if provider.ModelDBServiceID != "" {
		return provider.ModelDBServiceID
	}
	if provider.Type == "claude_messages" {
		return "anthropic"
	}
	return ""
}

func modelDBCatalog(cfg config) (modeldb.Catalog, bool, error) {
	if !configUsesModelDB(cfg) {
		return modeldb.Catalog{}, false, nil
	}
	catalog, err := loadModelDBCatalog(cfg.ModelDB)
	if err != nil {
		return modeldb.Catalog{}, false, err
	}
	return catalog, true, nil
}

func configUsesModelDB(cfg config) bool {
	if cfg.ModelDB.CatalogPath != "" || len(cfg.ModelDB.OverlayPaths) != 0 || len(cfg.ModelDB.Aliases) != 0 {
		return true
	}
	for _, route := range cfg.Routes {
		if route.ModelDBModel != "" {
			return true
		}
		if pricingWireModel(route) != "" {
			for _, provider := range cfg.Providers {
				if providerMatchesRoute(provider, route) && providerModelDBServiceID(provider) != "" {
					return true
				}
			}
		}
	}
	return false
}

func configUsesPricing(cfg config) bool {
	return configUsesModelDB(cfg)
}

func loadModelDBCatalog(cfg modelDBConfig) (modeldb.Catalog, error) {
	var (
		catalog modeldb.Catalog
		err     error
	)
	if cfg.CatalogPath != "" {
		catalog, err = modeldb.LoadJSON(cfg.CatalogPath)
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load modeldb catalog %q: %w", cfg.CatalogPath, err)
		}
	} else {
		catalog, err = modeldb.LoadBuiltIn()
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load built-in modeldb catalog: %w", err)
		}
	}
	for _, path := range cfg.OverlayPaths {
		overlay, err := modeldb.LoadJSON(path)
		if err != nil {
			return modeldb.Catalog{}, fmt.Errorf("load modeldb overlay %q: %w", path, err)
		}
		if err := mergeCatalog(&catalog, overlay); err != nil {
			return modeldb.Catalog{}, fmt.Errorf("merge modeldb overlay %q: %w", path, err)
		}
	}
	return catalog, nil
}

func mergeCatalog(dst *modeldb.Catalog, src modeldb.Catalog) error {
	return modeldb.MergeCatalogFragment(dst, &modeldb.Fragment{
		Services:  catalogServices(src),
		Models:    catalogModels(src),
		Offerings: catalogOfferings(src),
	})
}

func providerMatchesRoute(provider providerConfig, route routeConfig) bool {
	if provider.Name != route.Provider {
		return false
	}
	if route.ProviderAPI == "" {
		return true
	}
	apiKind, _, _, err := providerEndpointMetadata(provider.Type)
	return err == nil && apiKind == route.ProviderAPI
}

func endpointWithPricing(endpoint router.ProviderEndpoint, route routeConfig, catalog modeldb.Catalog) router.ProviderEndpoint {
	serviceID := endpoint.Tags[tagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	if serviceID == "" || wireModelID == "" || endpoint.Client == nil {
		return endpoint
	}
	endpoint.Client = &eventProcessorClient{
		inner: endpoint.Client,
		processors: []pipeline.Processor[unified.Event]{
			pricingpkg.NewProcessor(catalog, serviceID, wireModelID),
		},
	}
	return endpoint
}

func endpointWithModelDBMetadata(endpoint router.ProviderEndpoint, route routeConfig, catalog modeldb.Catalog) router.ProviderEndpoint {
	serviceID := endpoint.Tags[tagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	if serviceID == "" || wireModelID == "" {
		return endpoint
	}
	capabilities, ok := modelmeta.EnrichCapabilities(endpoint.Capabilities, catalog, serviceID, wireModelID, endpoint.Family)
	if ok {
		endpoint.Capabilities = capabilities
	}
	return endpoint
}

func pricingWireModel(route routeConfig) string {
	if route.ModelDBWireModelID != "" {
		return route.ModelDBWireModelID
	}
	return route.NativeModel
}

func providerEndpointMetadata(providerType string) (adapt.ApiKind, adapt.ApiFamily, router.CapabilitySet, error) {
	switch providerType {
	case "anthropic":
		return adapt.ApiAnthropicMessages, adapt.FamilyAnthropicMessages, router.CapabilitySet{Streaming: true, Tools: true, Vision: true}, nil
	case "claude_messages":
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
		apiKey := providerAPIKey(provider)
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
	case "claude_messages":
		apiKey := providerAPIKey(provider, "CLAUDE_ACCESS_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN")
		opts := []anthropic.Option{
			anthropic.WithClaudeHeaders(),
			anthropic.WithClaudeCodePreflight(),
			anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
				req.Unified.Stream = true
				return nil
			})),
		}
		if apiKey != "" {
			opts = append(opts, anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken(apiKey))))
		} else {
			opts = append(opts, anthropic.WithLocalClaudeOAuth())
		}
		if provider.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(provider.BaseURL))
		}
		return anthropic.NewClient(opts...)
	case "openai_chat":
		apiKey := providerAPIKey(provider, "OPENAI_API_KEY", "OPENAI_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openai.Option{openai.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(provider.BaseURL))
		}
		return openai.NewClient(opts...)
	case "openrouter_chat":
		apiKey := providerAPIKey(provider, "OPENROUTER_API_KEY", "OPENROUTER_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openrouter.Option{openrouter.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openrouter.WithBaseURL(provider.BaseURL))
		}
		return openrouter.NewClient(opts...)
	case "openrouter_responses":
		apiKey := providerAPIKey(provider, "OPENROUTER_API_KEY", "OPENROUTER_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openrouterresponses.Option{openrouterresponses.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openrouterresponses.WithBaseURL(provider.BaseURL))
		}
		return openrouterresponses.NewClient(opts...)
	case "openrouter_messages":
		apiKey := providerAPIKey(provider, "OPENROUTER_API_KEY", "OPENROUTER_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []openroutermessages.Option{openroutermessages.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, openroutermessages.WithBaseURL(provider.BaseURL))
		}
		return openroutermessages.NewClient(opts...)
	case "minimax_chat":
		apiKey := providerAPIKey(provider, "MINIMAX_API_KEY", "MINIMAX_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("provider %q requires api_key", provider.Name)
		}
		opts := []minimax.Option{minimax.WithAPIKey(apiKey)}
		if provider.BaseURL != "" {
			opts = append(opts, minimax.WithBaseURL(provider.BaseURL))
		}
		return minimax.NewClient(opts...)
	case "minimax_messages":
		apiKey := providerAPIKey(provider, "MINIMAX_API_KEY", "MINIMAX_KEY")
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

func providerAPIKey(provider providerConfig, fallbackEnv ...string) string {
	if provider.APIKey != "" {
		return provider.APIKey
	}
	if provider.APIKeyEnv != "" {
		return os.Getenv(provider.APIKeyEnv)
	}
	return firstEnv(fallbackEnv...)
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
