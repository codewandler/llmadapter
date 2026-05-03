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
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

const TagModelDBServiceID = "modeldb.service_id"

type MuxClientOptions struct {
	SourceAPI                  adapt.ApiKind
	Fallback                   *bool
	ProviderTransport          transport.ByteStreamTransport
	ProviderWebSocketTransport transport.ByteStreamTransport
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

// WithProviderTransport overrides the byte-stream transport used by constructed
// provider clients. It is intended for diagnostics and tests; normal callers
// should let provider clients choose their endpoint-family default transport.
func WithProviderTransport(t transport.ByteStreamTransport) MuxClientOption {
	return func(o *MuxClientOptions) {
		o.ProviderTransport = t
	}
}

// WithProviderWebSocketTransport overrides the WebSocket byte-stream transport
// used by provider clients that support a WebSocket mode. It is intended for
// diagnostics and tests.
func WithProviderWebSocketTransport(t transport.ByteStreamTransport) MuxClientOption {
	return func(o *MuxClientOptions) {
		o.ProviderWebSocketTransport = t
	}
}

func NewMuxClient(cfg Config, opts ...MuxClientOption) (unified.Client, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	options := MuxClientOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	r, err := buildRouter(cfg, options)
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
	return buildRouter(cfg, MuxClientOptions{})
}

func buildRouter(cfg Config, options MuxClientOptions) (router.Router, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		endpoint, err := buildProviderEndpoint(provider, options)
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
			route, err = ResolveRouteModelDBModel(route, endpoint, catalog, cfg.ModelDB)
			if err != nil {
				return nil, err
			}
			endpoint = EndpointWithModelDBMetadata(endpoint, route, catalog)
			endpoint = EndpointWithPricing(endpoint, route, catalog)
		}
		routes = append(routes, router.StaticRoute{
			SourceAPI:          route.SourceAPI,
			Model:              route.Model,
			NativeModel:        route.NativeModel,
			DynamicModels:      route.DynamicModels,
			Weight:             route.Weight,
			Endpoint:           endpoint,
			CapabilityResolver: dynamicModelCapabilityResolver(endpoint, route, catalog, modelDBEnabled),
			ModelResolver:      dynamicModelResolver(endpoint, route, catalog, cfg.ModelDB, modelDBEnabled),
			ModelMetadata:      routeModelMetadata(endpoint, route, catalog),
		})
	}
	return router.NewStaticRouter(routes...), nil
}

func BuildProviderEndpoint(provider ProviderConfig) (router.ProviderEndpoint, error) {
	return buildProviderEndpoint(provider, MuxClientOptions{})
}

func buildProviderEndpoint(provider ProviderConfig, options MuxClientOptions) (router.ProviderEndpoint, error) {
	client, err := buildProviderClient(provider, options)
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
		ProviderName:         provider.Name,
		APIKind:              descriptor.APIKind,
		Family:               descriptor.Family,
		Capabilities:         capabilities,
		ConsumerContinuation: descriptor.ConsumerContinuation,
		InternalContinuation: descriptor.InternalContinuation,
		Transport:            descriptor.Transport,
		Priority:             provider.Priority,
		Tags:                 providerEndpointTags(provider),
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
	if serviceID == "" || endpoint.Client == nil {
		return endpoint
	}
	if route.DynamicModels {
		endpoint.Client = &requestScopedPricingClient{
			inner:     endpoint.Client,
			catalog:   catalog,
			serviceID: serviceID,
		}
		return endpoint
	}
	if wireModelID == "" {
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

func routeModelMetadata(endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog) *unified.ResolvedModelMetadata {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	if serviceID == "" || wireModelID == "" {
		return nil
	}
	meta, ok := modelmeta.ResolvedMetadata(catalog, serviceID, wireModelID, endpoint.Family)
	if !ok {
		return nil
	}
	return &meta
}

func dynamicModelCapabilityResolver(endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog, modelDBEnabled bool) router.CapabilityResolver {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	if !modelDBEnabled || !route.DynamicModels || serviceID == "" {
		return nil
	}
	return func(_ context.Context, req adapt.Request, endpoint router.ProviderEndpoint) router.CapabilitySet {
		if req.Unified.Model == "" {
			return endpoint.Capabilities
		}
		capabilities, ok := modelmeta.EnrichCapabilities(endpoint.Capabilities, catalog, serviceID, req.Unified.Model, endpoint.Family)
		if !ok {
			return endpoint.Capabilities
		}
		return capabilities
	}
}

func dynamicModelResolver(endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog, cfg ModelDBConfig, modelDBEnabled bool) router.ModelResolver {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	if !modelDBEnabled || !route.DynamicModels || serviceID == "" {
		return nil
	}
	apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
	if !ok {
		return func(context.Context, adapt.Request, router.ProviderEndpoint) (router.ModelResolution, bool) {
			return router.ModelResolution{}, false
		}
	}
	return func(_ context.Context, req adapt.Request, endpoint router.ProviderEndpoint) (router.ModelResolution, bool) {
		if req.Unified.Model == "" {
			return router.ModelResolution{}, false
		}
		item, ok := resolveModelDBItem(catalog, cfg, serviceID, apiType, req.Unified.Model)
		if !ok {
			return router.ModelResolution{}, false
		}
		resolution := router.ModelResolution{NativeModel: item.Offering.WireModelID}
		if capabilities, ok := modelmeta.EnrichCapabilities(endpoint.Capabilities, catalog, serviceID, item.Offering.WireModelID, endpoint.Family); ok {
			resolution.Capabilities = &capabilities
		}
		if meta, ok := modelmeta.ResolvedMetadata(catalog, serviceID, item.Offering.WireModelID, endpoint.Family); ok {
			resolution.ModelMetadata = &meta
		}
		return resolution, true
	}
}

func descriptorForProvider(provider ProviderConfig) (providerregistry.Descriptor, bool) {
	return providerregistry.Lookup(provider.Type)
}

func buildProviderClient(provider ProviderConfig, options MuxClientOptions) (unified.Client, error) {
	apiKey := providerAPIKey(provider)
	return providerregistry.NewClient(providerregistry.ClientConfig{
		Type:               provider.Type,
		APIKey:             apiKey,
		BaseURL:            provider.BaseURL,
		Transport:          options.ProviderTransport,
		WebSocketTransport: options.ProviderWebSocketTransport,
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
	return modelDBServiceIDForProviderType(provider.Type)
}

func modelDBServiceIDForProviderType(providerType string) string {
	switch providerType {
	case "anthropic", "claude":
		return "anthropic"
	case "openai_chat", "openai_responses":
		return "openai"
	case "codex_responses":
		return "codex"
	case "bedrock_responses", "bedrock_messages":
		return "bedrock"
	case "openrouter_chat", "openrouter_responses", "openrouter_messages":
		return "openrouter"
	case "minimax_chat", "minimax_messages":
		return "minimax"
	default:
		return ""
	}
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
	return processEventStream(ctx, events, c.processors...), nil
}

type requestScopedPricingClient struct {
	inner     unified.Client
	catalog   modeldb.Catalog
	serviceID string
}

func (c *requestScopedPricingClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	events, err := c.inner.Request(ctx, req)
	if err != nil {
		return nil, err
	}
	if req.Model == "" {
		return events, nil
	}
	return processEventStream(ctx, events, pricing.NewProcessor(c.catalog, c.serviceID, req.Model)), nil
}

func processEventStream(ctx context.Context, events <-chan unified.Event, processors ...pipeline.Processor[unified.Event]) <-chan unified.Event {
	out := make(chan unified.Event)
	go func() {
		defer close(out)
		chain := pipeline.NewChain(processors...)
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
	return out
}
