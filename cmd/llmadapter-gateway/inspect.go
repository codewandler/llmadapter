package main

import (
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

type configInspection struct {
	Addr           string               `json:"addr,omitempty"`
	HealthCooldown string               `json:"health_cooldown,omitempty"`
	Providers      []providerInspection `json:"providers,omitempty"`
	Routes         []routeInspection    `json:"routes,omitempty"`
}

type providerInspection struct {
	Name             string               `json:"name"`
	Type             string               `json:"type"`
	APIKind          adapt.ApiKind        `json:"api_kind"`
	Family           adapt.ApiFamily      `json:"family"`
	Model            string               `json:"model,omitempty"`
	ModelDBServiceID string               `json:"modeldb_service_id,omitempty"`
	Priority         int                  `json:"priority,omitempty"`
	Capabilities     capabilityInspection `json:"capabilities"`
	Credential       credentialInspection `json:"credential"`
	Tags             map[string]string    `json:"tags,omitempty"`
}

type routeInspection struct {
	SourceAPI    adapt.ApiKind        `json:"source_api"`
	PublicModel  string               `json:"public_model,omitempty"`
	Provider     string               `json:"provider"`
	ProviderAPI  adapt.ApiKind        `json:"provider_api,omitempty"`
	TargetAPI    adapt.ApiKind        `json:"target_api"`
	TargetFamily adapt.ApiFamily      `json:"target_family"`
	NativeModel  string               `json:"native_model,omitempty"`
	Weight       int                  `json:"weight,omitempty"`
	Priority     int                  `json:"priority,omitempty"`
	Capabilities capabilityInspection `json:"capabilities"`
	ModelDB      modelDBInspection    `json:"modeldb"`
}

type credentialInspection struct {
	InlineAPIKey bool   `json:"inline_api_key"`
	APIKeyEnv    string `json:"api_key_env,omitempty"`
}

type modelDBInspection struct {
	Enabled          bool            `json:"enabled"`
	ServiceID        string          `json:"service_id,omitempty"`
	WireModelID      string          `json:"wire_model_id,omitempty"`
	APIType          modeldb.APIType `json:"api_type,omitempty"`
	OfferingFound    bool            `json:"offering_found,omitempty"`
	ExposureFound    bool            `json:"exposure_found,omitempty"`
	PricingAvailable bool            `json:"pricing_available,omitempty"`
}

type capabilityInspection struct {
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

func inspectConfig(cfg config) (configInspection, error) {
	catalog, modelDBEnabled, err := modelDBCatalog(cfg)
	if err != nil {
		return configInspection{}, err
	}
	return inspectConfigWithCatalog(cfg, catalog, modelDBEnabled)
}

func inspectConfigWithCatalog(cfg config, catalog modeldb.Catalog, modelDBEnabled bool) (configInspection, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	out := configInspection{
		Addr:           cfg.Addr,
		HealthCooldown: cfg.HealthCooldown,
		Providers:      make([]providerInspection, 0, len(cfg.Providers)),
		Routes:         make([]routeInspection, 0, len(cfg.Routes)),
	}

	for _, provider := range cfg.Providers {
		endpoint, err := providerEndpointConfig(provider)
		if err != nil {
			return configInspection{}, err
		}
		endpoints = append(endpoints, endpoint)
		out.Providers = append(out.Providers, providerInspection{
			Name:             provider.Name,
			Type:             provider.Type,
			APIKind:          endpoint.APIKind,
			Family:           endpoint.Family,
			Model:            provider.Model,
			ModelDBServiceID: provider.ModelDBServiceID,
			Priority:         provider.Priority,
			Capabilities:     inspectCapabilities(endpoint.Capabilities),
			Credential: credentialInspection{
				InlineAPIKey: provider.APIKey != "",
				APIKeyEnv:    provider.APIKeyEnv,
			},
			Tags: endpoint.Tags,
		})
	}

	for _, route := range cfg.Routes {
		endpoint, ok, ambiguous := findProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
		if ambiguous {
			return configInspection{}, fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
		}
		if !ok {
			return configInspection{}, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
		}
		if modelDBEnabled {
			endpoint = endpointWithModelDBMetadata(endpoint, route, catalog)
		}
		out.Routes = append(out.Routes, routeInspection{
			SourceAPI:    route.SourceAPI,
			PublicModel:  route.Model,
			Provider:     route.Provider,
			ProviderAPI:  route.ProviderAPI,
			TargetAPI:    endpoint.APIKind,
			TargetFamily: endpoint.Family,
			NativeModel:  route.NativeModel,
			Weight:       route.Weight,
			Priority:     endpoint.Priority,
			Capabilities: inspectCapabilities(endpoint.Capabilities),
			ModelDB:      inspectModelDB(catalog, modelDBEnabled, endpoint, route),
		})
	}

	return out, nil
}

func inspectModelDB(catalog modeldb.Catalog, enabled bool, endpoint router.ProviderEndpoint, route routeConfig) modelDBInspection {
	serviceID := endpoint.Tags[tagModelDBServiceID]
	wireModelID := pricingWireModel(route)
	info := modelDBInspection{
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

func inspectCapabilities(caps router.CapabilitySet) capabilityInspection {
	return capabilityInspection{
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
