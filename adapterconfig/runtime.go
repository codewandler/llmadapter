package adapterconfig

import (
	"context"
	"fmt"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/muxclient"
	"github.com/codewandler/llmadapter/pipeline"
	"github.com/codewandler/llmadapter/pricing"
	"github.com/codewandler/llmadapter/providerregistry"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

const TagModelDBServiceID = "modeldb.service_id"

type MuxClientOptions struct {
	SourceAPI adapt.ApiKind
	Fallback  *bool
}

type MuxClientOption func(*MuxClientOptions)

func WithSourceAPI(api adapt.ApiKind) MuxClientOption {
	return func(o *MuxClientOptions) {
		if api != "" {
			o.SourceAPI = api
		}
	}
}

func WithFallback(enabled bool) MuxClientOption {
	return func(o *MuxClientOptions) {
		o.Fallback = &enabled
	}
}

func NewMuxClient(cfg Config, opts ...MuxClientOption) (unified.Client, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	options := MuxClientOptions{SourceAPI: adapt.ApiOpenAIResponses}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	r, err := BuildRouter(cfg)
	if err != nil {
		return nil, err
	}
	muxOpts := []muxclient.Option{muxclient.WithSourceAPI(options.SourceAPI)}
	if options.Fallback != nil {
		muxOpts = append(muxOpts, muxclient.WithFallback(*options.Fallback))
	}
	return muxclient.New(r, muxOpts...), nil
}

func BuildRouter(cfg Config) (router.Router, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		endpoint, err := BuildProviderEndpoint(provider)
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
		endpoint, ok, ambiguous := FindProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
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
			endpoint = EndpointWithModelDBMetadata(endpoint, route, catalog)
			endpoint = EndpointWithPricing(endpoint, route, catalog)
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

func BuildProviderEndpoint(provider ProviderConfig) (router.ProviderEndpoint, error) {
	client, err := buildProviderClient(provider)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	endpoint, err := ProviderEndpointConfig(provider)
	if err != nil {
		return router.ProviderEndpoint{}, err
	}
	endpoint.Client = client
	return endpoint, nil
}

func ProviderEndpointConfig(provider ProviderConfig) (router.ProviderEndpoint, error) {
	descriptor, ok := descriptorForProvider(provider)
	if !ok {
		return router.ProviderEndpoint{}, fmt.Errorf("unsupported provider type %q", provider.Type)
	}
	capabilities := ApplyCapabilityOverrides(descriptor.Capabilities, provider.Capabilities)
	return router.ProviderEndpoint{
		ProviderName: provider.Name,
		APIKind:      descriptor.APIKind,
		Family:       descriptor.Family,
		Capabilities: capabilities,
		Priority:     provider.Priority,
		Tags:         providerEndpointTags(provider),
	}, nil
}

func FindProviderEndpoint(endpoints []router.ProviderEndpoint, providerName string, apiKind adapt.ApiKind) (router.ProviderEndpoint, bool, bool) {
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

func EndpointWithPricing(endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog) router.ProviderEndpoint {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	if serviceID == "" || wireModelID == "" || endpoint.Client == nil {
		return endpoint
	}
	endpoint.Client = &eventProcessorClient{
		inner: endpoint.Client,
		processors: []pipeline.Processor[unified.Event]{
			pricing.NewProcessor(catalog, serviceID, wireModelID),
		},
	}
	return endpoint
}

func EndpointWithModelDBMetadata(endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog) router.ProviderEndpoint {
	serviceID := endpoint.Tags[TagModelDBServiceID]
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

func descriptorForProvider(provider ProviderConfig) (providerregistry.Descriptor, bool) {
	return providerregistry.Lookup(provider.Type)
}

func buildProviderClient(provider ProviderConfig) (unified.Client, error) {
	apiKey := providerAPIKey(provider)
	return providerregistry.NewClient(providerregistry.ClientConfig{
		Type:    provider.Type,
		APIKey:  apiKey,
		BaseURL: provider.BaseURL,
	})
}

func providerAPIKey(provider ProviderConfig) string {
	if provider.APIKey != "" {
		return provider.APIKey
	}
	if provider.APIKeyEnv != "" {
		return os.Getenv(provider.APIKeyEnv)
	}
	descriptor, ok := descriptorForProvider(provider)
	if !ok {
		return ""
	}
	for _, key := range descriptor.DefaultAPIKeyEnvs {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func providerEndpointTags(provider ProviderConfig) map[string]string {
	serviceID := providerModelDBServiceID(provider)
	if serviceID == "" {
		return nil
	}
	return map[string]string{TagModelDBServiceID: serviceID}
}

func providerModelDBServiceID(provider ProviderConfig) string {
	if provider.ModelDBServiceID != "" {
		return provider.ModelDBServiceID
	}
	if provider.Type == "claude_messages" {
		return "anthropic"
	}
	return ""
}

func pricingWireModel(route RouteConfig) string {
	if route.ModelDBWireModelID != "" {
		return route.ModelDBWireModelID
	}
	return route.NativeModel
}

type eventProcessorClient struct {
	inner      unified.Client
	processors []pipeline.Processor[unified.Event]
}

func (c *eventProcessorClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	events, err := c.inner.Request(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan unified.Event)
	go func() {
		defer close(out)
		chain := pipeline.NewChain(c.processors...)
		emit := func(values []unified.Event) bool {
			for _, ev := range values {
				select {
				case <-ctx.Done():
					return false
				case out <- ev:
				}
			}
			return true
		}
		fail := func(err error) {
			if err == nil {
				return
			}
			select {
			case <-ctx.Done():
			case out <- unified.ErrorEvent{Err: err}:
			}
		}
		for {
			select {
			case <-ctx.Done():
				fail(ctx.Err())
				return
			case ev, ok := <-events:
				if !ok {
					flushed, err := chain.Close(ctx)
					if err != nil {
						fail(err)
						return
					}
					emit(flushed)
					return
				}
				processed, err := chain.Push(ctx, ev)
				if err != nil {
					fail(err)
					return
				}
				if !emit(processed) {
					return
				}
			}
		}
	}()
	return out, nil
}
