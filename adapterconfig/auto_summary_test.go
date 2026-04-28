package adapterconfig

import (
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/modeldb"
)

func TestAutoResultRouteSummaryFromConfig(t *testing.T) {
	result := AutoResult{
		Config: Config{
			Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses"}},
			Routes: []RouteConfig{{
				SourceAPI:   adapt.ApiOpenAIResponses,
				Model:       "default",
				Provider:    "openai_responses",
				ProviderAPI: adapt.ApiOpenAIResponses,
				NativeModel: "gpt-test",
			}},
		},
		Enabled: []AutoProvider{{Name: "openai_responses", Type: "openai_responses", Reason: "env:OPENAI_API_KEY"}},
	}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "default")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.Provider != "openai_responses" || summary.NativeModel != "gpt-test" || summary.EnabledReason != "env:OPENAI_API_KEY" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryDefaultsSourceAPI(t *testing.T) {
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "default",
			Provider:    "openai_responses",
			NativeModel: "default",
		}},
	}}

	summary, ok := result.RouteSummary("", "")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.SourceAPI != adapt.ApiOpenAIResponses || summary.NativeModel != "default" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryAutoSourcePrefersAnthropicMessages(t *testing.T) {
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "claude", Type: "claude"}},
		Routes: []RouteConfig{
			{
				SourceAPI:   adapt.ApiOpenAIResponses,
				Model:       "haiku",
				Provider:    "claude",
				ProviderAPI: adapt.ApiAnthropicMessages,
				NativeModel: "claude-haiku",
				Weight:      100,
			},
			{
				SourceAPI:   adapt.ApiAnthropicMessages,
				Model:       "haiku",
				Provider:    "claude",
				ProviderAPI: adapt.ApiAnthropicMessages,
				NativeModel: "claude-haiku",
				Weight:      100,
			},
		},
	}}

	summary, ok := result.RouteSummary("", "haiku")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.SourceAPI != adapt.ApiAnthropicMessages || summary.ProviderAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryDynamicModel(t *testing.T) {
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses"}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Provider:      "openai_responses",
			ProviderAPI:   adapt.ApiOpenAIResponses,
			DynamicModels: true,
		}},
	}}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "gpt-new")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.Model != "gpt-new" || summary.NativeModel != "gpt-new" {
		t.Fatalf("unexpected dynamic summary: %+v", summary)
	}
}

func testCatalogWithLimits(serviceID, wireModelID string, contextWindow, maxOutput int, limitsOverride *modeldb.Limits) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "test", Family: "model", Version: "v1"}
	catalog.Services[serviceID] = modeldb.Service{ID: serviceID, Name: "Test Service"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Limits: modeldb.Limits{ContextWindow: contextWindow, MaxOutput: maxOutput}}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}] = modeldb.Offering{
		ServiceID:      serviceID,
		WireModelID:    wireModelID,
		ModelKey:       key,
		LimitsOverride: limitsOverride,
	}
	return catalog
}

func TestRouteSummaryIncludesContextWindow(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(catalogPath, testCatalogWithLimits("openai", "gpt-test", 128_000, 4096, nil)); err != nil {
		t.Fatal(err)
	}
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses", ModelDBServiceID: "openai"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "test-model",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}},
		ModelDB: ModelDBConfig{CatalogPath: catalogPath},
	}}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "test-model")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.ContextWindow != 128_000 {
		t.Fatalf("expected context_window=128000, got %d", summary.ContextWindow)
	}
}

func TestRouteSummaryWithoutModelDBReturnsZeroContextWindow(t *testing.T) {
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "test-model",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}},
	}}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "test-model")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.ContextWindow != 0 {
		t.Fatalf("expected context_window=0, got %d", summary.ContextWindow)
	}
}

func TestRouteSummaryUsesOfferingLimitsOverride(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(catalogPath, testCatalogWithLimits("openai", "gpt-test", 128_000, 4096, &modeldb.Limits{ContextWindow: 64_000})); err != nil {
		t.Fatal(err)
	}
	result := AutoResult{Config: Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses", ModelDBServiceID: "openai"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "test-model",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}},
		ModelDB: ModelDBConfig{CatalogPath: catalogPath},
	}}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "test-model")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.ContextWindow != 64_000 {
		t.Fatalf("expected context_window=64000 (offering override), got %d", summary.ContextWindow)
	}
}
func TestResolveModelCandidateCarriesLimits(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(catalogPath, testCatalogWithLimits("openai", "gpt-test", 200_000, 8192, nil)); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses", ModelDBServiceID: "openai"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "test-model",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}},
		ModelDB: ModelDBConfig{CatalogPath: catalogPath},
	}
	candidates, err := ResolveModelCandidates(cfg, "test-model", adapt.ApiOpenAIResponses)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if candidates[0].Limits.ContextWindow != 200_000 {
		t.Fatalf("expected context_window=200000, got %d", candidates[0].Limits.ContextWindow)
	}
	if candidates[0].Limits.MaxOutput != 8192 {
		t.Fatalf("expected max_output=8192, got %d", candidates[0].Limits.MaxOutput)
	}
}

func TestResolveModelCandidateWithoutModelDBReturnsZeroLimits(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{Name: "openai_responses", Type: "openai_responses"}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "test-model",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}},
	}
	candidates, err := ResolveModelCandidates(cfg, "test-model", adapt.ApiOpenAIResponses)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if candidates[0].Limits.ContextWindow != 0 {
		t.Fatalf("expected context_window=0, got %d", candidates[0].Limits.ContextWindow)
	}
}
