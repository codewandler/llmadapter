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
	ContextWindow   int           `json:"context_window,omitempty"`
}

func (r AutoResult) RouteSummary(sourceAPI adapt.ApiKind, model string) (AutoRouteSummary, bool) {
	if model == "" {
		model = defaultSummaryModel(r.Config.Routes, sourceAPI)
	}
	resolution, err := ResolveModel(r.Config, model, sourceAPI)
	if err != nil {
		return AutoRouteSummary{}, false
	}
	summary := AutoRouteSummary{
		SourceAPI:     resolution.SourceAPI,
		Model:         resolution.PublicModel,
		Provider:      resolution.Provider,
		ProviderAPI:   resolution.ProviderAPI,
		NativeModel:   resolution.NativeModel,
		ContextWindow: resolution.Limits.ContextWindow,
	}
	if summary.Model == "" {
		summary.Model = model
	}
	for _, provider := range r.Enabled {
		if provider.Name == summary.Provider {
			summary.EnabledProvider = provider.Type
			summary.EnabledReason = provider.Reason
			break
		}
	}
	return summary, true
}

func defaultSummaryModel(routes []RouteConfig, sourceAPI adapt.ApiKind) string {
	for _, route := range routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		if route.Model != "" {
			return route.Model
		}
	}
	return ""
}
