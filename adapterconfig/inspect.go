package adapterconfig

import (
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

type ConfigInspection struct {
	Addr           string               `json:"addr,omitempty"`
	HealthCooldown string               `json:"health_cooldown,omitempty"`
	MaxAttempts    int                  `json:"max_attempts,omitempty"`
	Providers      []ProviderInspection `json:"providers,omitempty"`
	Routes         []RouteInspection    `json:"routes,omitempty"`
}

type ProviderInspection struct {
	Name             string               `json:"name"`
	Type             string               `json:"type"`
	APIKind          adapt.ApiKind        `json:"api_kind,omitempty"`
	Family           adapt.ApiFamily      `json:"family,omitempty"`
	Model            string               `json:"model,omitempty"`
	ModelDBServiceID string               `json:"modeldb_service_id,omitempty"`
	APIKeyEnv        string               `json:"api_key_env,omitempty"`
	InlineAPIKey     bool                 `json:"inline_api_key,omitempty"`
	BaseURL          string               `json:"base_url,omitempty"`
	Priority         int                  `json:"priority,omitempty"`
	Capabilities     CapabilityInspection `json:"capabilities,omitempty"`
	CapabilitySource string               `json:"capability_source,omitempty"`
	Tags             map[string]string    `json:"tags,omitempty"`
	Error            string               `json:"error,omitempty"`
}

type RouteInspection struct {
	SourceAPI          adapt.ApiKind        `json:"source_api"`
	Model              string               `json:"model,omitempty"`
	Provider           string               `json:"provider"`
	ProviderAPI        adapt.ApiKind        `json:"provider_api,omitempty"`
	DynamicModels      bool                 `json:"dynamic_models,omitempty"`
	ModelDBModel       string               `json:"modeldb_model,omitempty"`
	NativeModel        string               `json:"native_model,omitempty"`
	ModelDBWireModelID string               `json:"modeldb_wire_model_id,omitempty"`
	Weight             int                  `json:"weight,omitempty"`
	TargetAPI          adapt.ApiKind        `json:"target_api,omitempty"`
	TargetFamily       adapt.ApiFamily      `json:"target_family,omitempty"`
	Priority           int                  `json:"priority,omitempty"`
	Capabilities       CapabilityInspection `json:"capabilities,omitempty"`
	CapabilitySource   string               `json:"capability_source,omitempty"`
	ModelDB            ModelDBInspection    `json:"modeldb,omitempty"`
}

type CapabilityInspection struct {
	Streaming         bool `json:"streaming"`
	Tools             bool `json:"tools"`
	ParallelTools     bool `json:"parallel_tools"`
	Vision            bool `json:"vision"`
	AudioInput        bool `json:"audio_input"`
	AudioOutput       bool `json:"audio_output"`
	JSONMode          bool `json:"json_mode"`
	JSONSchema        bool `json:"json_schema"`
	Reasoning         bool `json:"reasoning"`
	ReasoningDeltas   bool `json:"reasoning_deltas"`
	Citations         bool `json:"citations"`
	BuiltInWebSearch  bool `json:"built_in_web_search"`
	BuiltInFileSearch bool `json:"built_in_file_search"`
	CodeExecution     bool `json:"code_execution"`
	ComputerUse       bool `json:"computer_use"`
	ServerSideState   bool `json:"server_side_state"`
	PromptCaching     bool `json:"prompt_caching"`
	MaxInputTokens    int  `json:"max_input_tokens,omitempty"`
	MaxOutputTokens   int  `json:"max_output_tokens,omitempty"`
}

type ModelDBInspection struct {
	Enabled          bool            `json:"enabled"`
	ServiceID        string          `json:"service_id,omitempty"`
	WireModelID      string          `json:"wire_model_id,omitempty"`
	APIType          modeldb.APIType `json:"api_type,omitempty"`
	OfferingFound    bool            `json:"offering_found,omitempty"`
	ExposureFound    bool            `json:"exposure_found,omitempty"`
	PricingAvailable bool            `json:"pricing_available,omitempty"`
}

func InspectConfig(cfg Config) (ConfigInspection, error) {
	catalog, modelDBEnabled, err := modelDBCatalog(cfg)
	if err != nil {
		return ConfigInspection{}, err
	}
	return InspectConfigWithCatalog(cfg, catalog, modelDBEnabled)
}

func InspectConfigWithCatalog(cfg Config, catalog modeldb.Catalog, modelDBEnabled bool) (ConfigInspection, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	out := ConfigInspection{
		Addr:           cfg.Addr,
		HealthCooldown: cfg.HealthCooldown,
		MaxAttempts:    cfg.MaxAttempts,
		Providers:      make([]ProviderInspection, 0, len(cfg.Providers)),
		Routes:         make([]RouteInspection, 0, len(cfg.Routes)),
	}

	for _, provider := range cfg.Providers {
		endpoint, err := ProviderEndpointConfig(provider)
		info := ProviderInspection{
			Name:         provider.Name,
			Type:         provider.Type,
			Model:        provider.Model,
			APIKeyEnv:    provider.APIKeyEnv,
			InlineAPIKey: provider.APIKey != "",
			BaseURL:      provider.BaseURL,
			Priority:     provider.Priority,
		}
		if err != nil {
			info.Error = err.Error()
			out.Providers = append(out.Providers, info)
			continue
		}
		endpoints = append(endpoints, endpoint)
		info.APIKind = endpoint.APIKind
		info.Family = endpoint.Family
		info.ModelDBServiceID = providerModelDBServiceID(provider)
		info.Capabilities = inspectCapabilities(endpoint.Capabilities)
		info.CapabilitySource = providerCapabilitySource(provider)
		info.Tags = endpoint.Tags
		out.Providers = append(out.Providers, info)
	}

	for _, route := range cfg.Routes {
		endpoint, ok, ambiguous := FindProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
		if ambiguous {
			return ConfigInspection{}, fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
		}
		if !ok {
			return ConfigInspection{}, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
		}
		if modelDBEnabled {
			var err error
			route, err = ResolveRouteModelDBModel(route, endpoint, catalog, cfg.ModelDB)
			if err != nil {
				return ConfigInspection{}, err
			}
			endpoint = EndpointWithModelDBMetadata(endpoint, route, catalog)
		}
		out.Routes = append(out.Routes, RouteInspection{
			SourceAPI:          route.SourceAPI,
			Model:              route.Model,
			Provider:           route.Provider,
			ProviderAPI:        route.ProviderAPI,
			DynamicModels:      route.DynamicModels,
			ModelDBModel:       route.ModelDBModel,
			NativeModel:        route.NativeModel,
			ModelDBWireModelID: route.ModelDBWireModelID,
			Weight:             route.Weight,
			TargetAPI:          endpoint.APIKind,
			TargetFamily:       endpoint.Family,
			Priority:           endpoint.Priority,
			Capabilities:       inspectCapabilities(endpoint.Capabilities),
			CapabilitySource:   routeCapabilitySource(providerForInspection(cfg, route.Provider), endpoint, route, catalog, modelDBEnabled),
			ModelDB:            inspectModelDB(catalog, modelDBEnabled, endpoint, route),
		})
	}

	return out, nil
}

func providerForInspection(cfg Config, name string) ProviderConfig {
	for _, provider := range cfg.Providers {
		if provider.Name == name {
			return provider
		}
	}
	return ProviderConfig{}
}

func providerCapabilitySource(provider ProviderConfig) string {
	if provider.Capabilities != nil {
		return "config_override"
	}
	return "provider_descriptor"
}

func routeCapabilitySource(provider ProviderConfig, endpoint router.ProviderEndpoint, route RouteConfig, catalog modeldb.Catalog, modelDBEnabled bool) string {
	if modelDBEnabled && modelDBExposureFound(catalog, endpoint, route) {
		return "modeldb_exposure"
	}
	return providerCapabilitySource(provider)
}

func inspectModelDB(catalog modeldb.Catalog, enabled bool, endpoint router.ProviderEndpoint, route RouteConfig) ModelDBInspection {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	info := ModelDBInspection{
		Enabled:     enabled && serviceID != "" && wireModelID != "",
		ServiceID:   serviceID,
		WireModelID: wireModelID,
	}
	if !info.Enabled {
		return info
	}
	apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
	if ok {
		info.APIType = apiType
	}
	offering, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}]
	if !ok {
		return info
	}
	info.OfferingFound = true
	info.PricingAvailable = offering.Pricing != nil
	if apiType != "" {
		info.ExposureFound = offering.Exposure(apiType) != nil
	}
	return info
}

func modelDBExposureFound(catalog modeldb.Catalog, endpoint router.ProviderEndpoint, route RouteConfig) bool {
	serviceID := endpoint.Tags[TagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	if serviceID == "" || wireModelID == "" {
		return false
	}
	apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
	if !ok {
		return false
	}
	offering, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}]
	if !ok {
		return false
	}
	return offering.Exposure(apiType) != nil
}

func inspectCapabilities(caps router.CapabilitySet) CapabilityInspection {
	return CapabilityInspection{
		Streaming:         caps.Streaming,
		Tools:             caps.Tools,
		ParallelTools:     caps.ParallelTools,
		Vision:            caps.Vision,
		AudioInput:        caps.AudioInput,
		AudioOutput:       caps.AudioOutput,
		JSONMode:          caps.JSONMode,
		JSONSchema:        caps.JSONSchema,
		Reasoning:         caps.Reasoning,
		ReasoningDeltas:   caps.ReasoningDeltas,
		Citations:         caps.Citations,
		BuiltInWebSearch:  caps.BuiltInWebSearch,
		BuiltInFileSearch: caps.BuiltInFileSearch,
		CodeExecution:     caps.CodeExecution,
		ComputerUse:       caps.ComputerUse,
		ServerSideState:   caps.ServerSideState,
		PromptCaching:     caps.PromptCaching,
		MaxInputTokens:    caps.MaxInputTokens,
		MaxOutputTokens:   caps.MaxOutputTokens,
	}
}
