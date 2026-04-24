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
	if sourceAPI == "" {
		sourceAPI = adapt.ApiOpenAIResponses
	}
	for _, route := range r.Config.Routes {
		if route.SourceAPI != sourceAPI {
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
		return summary, true
	}
	return AutoRouteSummary{}, false
}
