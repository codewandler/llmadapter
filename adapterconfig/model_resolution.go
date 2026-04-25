package adapterconfig

import (
	"fmt"
	"sort"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

type ModelResolutionCandidate struct {
	Input            string
	MatchedAs        string
	SourceAPI        adapt.ApiKind
	PublicModel      string
	NativeModel      string
	Provider         string
	ProviderType     string
	ProviderAPI      adapt.ApiKind
	Family           adapt.ApiFamily
	Weight           int
	Priority         int
	ModelDBService   string
	Capabilities     router.CapabilitySet
	CapabilitySource string
}

func ResolveModelCandidates(cfg Config, model string, sourceAPI adapt.ApiKind) ([]ModelResolutionCandidate, error) {
	endpoints, err := providerEndpointConfigs(cfg.Providers)
	if err != nil {
		return nil, err
	}
	catalog, modelDBEnabled, err := modelDBCatalog(cfg)
	if err != nil {
		return nil, err
	}
	var out []ModelResolutionCandidate
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		matchedAs := modelRouteMatch(route, model)
		if matchedAs == "" {
			continue
		}
		provider, endpoint, err := providerEndpointConfigForRoute(cfg, endpoints, route)
		if err != nil {
			return nil, err
		}
		route, endpoint, nativeModel, ok := resolveRouteModelForCandidate(route, endpoint, catalog, cfg.ModelDB, modelDBEnabled, model)
		if !ok {
			continue
		}
		if route.ProviderAPI == "" {
			route.ProviderAPI = endpoint.APIKind
		}
		out = append(out, ModelResolutionCandidate{
			Input:            model,
			MatchedAs:        matchedAs,
			SourceAPI:        route.SourceAPI,
			PublicModel:      route.Model,
			NativeModel:      nativeModel,
			Provider:         route.Provider,
			ProviderType:     provider.Type,
			ProviderAPI:      route.ProviderAPI,
			Family:           endpoint.Family,
			Weight:           route.Weight,
			Priority:         provider.Priority,
			ModelDBService:   endpoint.Tags[TagModelDBServiceID],
			Capabilities:     endpoint.Capabilities,
			CapabilitySource: routeCapabilitySource(provider, endpoint, route, catalog, modelDBEnabled),
		})
	}
	if len(out) == 0 {
		if sourceAPI != "" {
			return nil, fmt.Errorf("no route found for model %q and source_api %s", model, sourceAPI)
		}
		return nil, fmt.Errorf("no route found for model %q", model)
	}
	out = suppressDynamicFallbacks(out)
	sort.SliceStable(out, func(i, j int) bool {
		if pi := modelResolutionPriority(out[i]); pi != modelResolutionPriority(out[j]) {
			return pi < modelResolutionPriority(out[j])
		}
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		if out[i].ProviderType != out[j].ProviderType {
			return out[i].ProviderType < out[j].ProviderType
		}
		return out[i].Provider < out[j].Provider
	})
	return out, nil
}

func suppressDynamicFallbacks(candidates []ModelResolutionCandidate) []ModelResolutionCandidate {
	hasExact := false
	for _, candidate := range candidates {
		if candidate.MatchedAs != "dynamic_model" {
			hasExact = true
			break
		}
	}
	if !hasExact {
		return candidates
	}
	out := candidates[:0]
	for _, candidate := range candidates {
		if candidate.MatchedAs != "dynamic_model" {
			out = append(out, candidate)
		}
	}
	return out
}

func ResolveModel(cfg Config, model string, sourceAPI adapt.ApiKind) (ModelResolutionCandidate, error) {
	candidates, err := ResolveModelCandidates(cfg, model, sourceAPI)
	if err != nil {
		return ModelResolutionCandidate{}, err
	}
	return candidates[0], nil
}

func providerEndpointConfigs(providers []ProviderConfig) ([]router.ProviderEndpoint, error) {
	endpoints := make([]router.ProviderEndpoint, 0, len(providers))
	for _, provider := range providers {
		endpoint, err := ProviderEndpointConfig(provider)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

func providerEndpointConfigForRoute(cfg Config, endpoints []router.ProviderEndpoint, route RouteConfig) (ProviderConfig, router.ProviderEndpoint, error) {
	var provider ProviderConfig
	for _, candidate := range cfg.Providers {
		if candidate.Name == route.Provider {
			provider = candidate
			break
		}
	}
	if provider.Name == "" {
		return ProviderConfig{}, router.ProviderEndpoint{}, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
	}
	endpoint, ok, ambiguous := FindProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
	if ambiguous {
		return ProviderConfig{}, router.ProviderEndpoint{}, fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
	}
	if !ok {
		return ProviderConfig{}, router.ProviderEndpoint{}, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
	}
	return provider, endpoint, nil
}

func modelRouteMatch(route RouteConfig, model string) string {
	switch {
	case route.Model == model:
		return "public_model"
	case route.NativeModel == model:
		return "native_model"
	case route.DynamicModels:
		return "dynamic_model"
	default:
		return ""
	}
}

func resolveRouteModelForCandidate(route RouteConfig, endpoint router.ProviderEndpoint, catalog modeldb.Catalog, cfg ModelDBConfig, modelDBEnabled bool, model string) (RouteConfig, router.ProviderEndpoint, string, bool) {
	if modelDBEnabled {
		if route.ModelDBModel != "" {
			var err error
			route, err = ResolveRouteModelDBModel(route, endpoint, catalog, cfg)
			if err != nil {
				return RouteConfig{}, router.ProviderEndpoint{}, "", false
			}
			endpoint = EndpointWithModelDBMetadata(endpoint, route, catalog)
		}
		if route.DynamicModels {
			serviceID := endpoint.Tags[TagModelDBServiceID]
			apiType, ok := modelmeta.APITypeForFamily(endpoint.Family)
			if !ok || serviceID == "" {
				return RouteConfig{}, router.ProviderEndpoint{}, "", false
			}
			item, ok := resolveModelDBItem(catalog, cfg, serviceID, apiType, model)
			if !ok {
				return RouteConfig{}, router.ProviderEndpoint{}, "", false
			}
			route.NativeModel = item.Offering.WireModelID
			endpoint = EndpointWithModelDBMetadata(endpoint, route, catalog)
		}
	}
	nativeModel := route.NativeModel
	if nativeModel == "" && route.DynamicModels {
		nativeModel = model
	}
	return route, endpoint, nativeModel, true
}

func modelResolutionPriority(resolution ModelResolutionCandidate) int {
	return modelResolutionMatchPriority(resolution.MatchedAs)*10000 + modelResolutionSourcePriority(resolution.SourceAPI)*1000 + modelResolutionProviderTypePriority(resolution.ProviderType)
}

func modelResolutionMatchPriority(matchedAs string) int {
	switch matchedAs {
	case "public_model":
		return 0
	case "native_model":
		return 1
	case "dynamic_model":
		return 2
	default:
		return 10
	}
}

func modelResolutionSourcePriority(sourceAPI adapt.ApiKind) int {
	switch sourceAPI {
	case adapt.ApiAnthropicMessages:
		return 0
	case adapt.ApiOpenAIResponses:
		return 1
	case adapt.ApiOpenAIChatCompletions:
		return 2
	default:
		return 10
	}
}

func modelResolutionProviderTypePriority(providerType string) int {
	switch providerType {
	case "claude":
		return 0
	case "anthropic":
		return 1
	case "openrouter_messages", "openrouter_chat", "openrouter_responses":
		return 4
	default:
		return 5
	}
}
