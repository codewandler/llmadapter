package adapterconfig

import "github.com/codewandler/llmadapter/adapt"

type AutoRouteSummary struct {
	SourceAPI       adapt.ApiKind `json:"source_api,omitempty"`
	Model           string        `json:"model,omitempty"`
	Provider        string        `json:"provider,omitempty"`
	ProviderAPI     adapt.ApiKind `json:"provider_api,omitempty"`
	NativeModel     string        `json:"native_model,omitempty"`
	EnabledProvider string        `json:"enabled_provider,omitempty"`
	EnabledReason   string        `json:"enabled_reason,omitempty"`
}

func (r AutoResult) RouteSummary(sourceAPI adapt.ApiKind, model string) (AutoRouteSummary, bool) {
	bestRank := autoRouteSummaryRank{Source: 100, Weight: -1, ProviderPriority: -1, Index: len(r.Config.Routes)}
	var best AutoRouteSummary
	var found bool
	for _, route := range r.Config.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		if model != "" && !route.DynamicModels && route.Model != "" && route.Model != model {
			continue
		}
		summary := AutoRouteSummary{
			SourceAPI:   route.SourceAPI,
			Model:       route.Model,
			Provider:    route.Provider,
			ProviderAPI: route.ProviderAPI,
			NativeModel: route.NativeModel,
		}
		if summary.Model == "" {
			summary.Model = model
		}
		if summary.NativeModel == "" {
			summary.NativeModel = summary.Model
		}
		for _, provider := range r.Enabled {
			if provider.Name == route.Provider {
				summary.EnabledProvider = provider.Type
				summary.EnabledReason = provider.Reason
				break
			}
		}
		rank := r.routeSummaryRank(route)
		if !found || rank.less(bestRank) {
			best = summary
			bestRank = rank
			found = true
		}
	}
	return best, found
}

type autoRouteSummaryRank struct {
	Source           int
	Weight           int
	ProviderPriority int
	Index            int
}

func (r autoRouteSummaryRank) less(other autoRouteSummaryRank) bool {
	if r.Source != other.Source {
		return r.Source < other.Source
	}
	if r.Weight != other.Weight {
		return r.Weight > other.Weight
	}
	if r.ProviderPriority != other.ProviderPriority {
		return r.ProviderPriority > other.ProviderPriority
	}
	return r.Index < other.Index
}

func (r AutoResult) routeSummaryRank(route RouteConfig) autoRouteSummaryRank {
	return autoRouteSummaryRank{
		Source:           autoRouteSummarySourceRank(route.SourceAPI),
		Weight:           route.Weight,
		ProviderPriority: r.providerPriority(route.Provider),
		Index:            routeIndex(r.Config.Routes, route),
	}
}

func (r AutoResult) providerPriority(providerName string) int {
	for _, provider := range r.Config.Providers {
		if provider.Name == providerName {
			return provider.Priority
		}
	}
	return 0
}

func autoRouteSummarySourceRank(api adapt.ApiKind) int {
	switch api {
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

func routeIndex(routes []RouteConfig, route RouteConfig) int {
	for i, candidate := range routes {
		if candidate == route {
			return i
		}
	}
	return len(routes)
}
