package adapterconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

type fakeClient struct {
	events []unified.Event
}

func TestLoadAndValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"addr":":9090",
		"health_cooldown":"5s",
		"max_attempts":2,
		"providers":[{"name":"openai","type":"openai_responses","api_key":"key","model":"gpt-test"}],
		"routes":[{"source_api":"openai.responses","model":"public","provider":"openai"}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Routes[0].NativeModel != "gpt-test" {
		t.Fatalf("native model default = %q", cfg.Routes[0].NativeModel)
	}
	if cfg.MaxAttempts != 2 {
		t.Fatalf("max attempts = %d, want 2", cfg.MaxAttempts)
	}
}

func TestValidateRejectsNegativeMaxAttempts(t *testing.T) {
	cfg := Config{
		MaxAttempts: -1,
		Providers:   []ProviderConfig{{Name: "openai", Type: "openai_responses", APIKey: "key"}},
		Routes:      []RouteConfig{{SourceAPI: adapt.ApiOpenAIResponses, Provider: "openai"}},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("expected max_attempts validation error")
	}
}

func TestProviderEndpointConfigCodexMetadata(t *testing.T) {
	endpoint, err := ProviderEndpointConfig(ProviderConfig{Name: "codex", Type: "codex_responses"})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.APIKind != adapt.ApiCodexResponses || endpoint.Family != adapt.FamilyOpenAIResponses {
		t.Fatalf("unexpected endpoint: %+v", endpoint)
	}
	if endpoint.Tags[TagModelDBServiceID] != "codex" {
		t.Fatalf("unexpected tags: %+v", endpoint.Tags)
	}
}

func TestProviderEndpointConfigClaudeMetadata(t *testing.T) {
	endpoint, err := ProviderEndpointConfig(ProviderConfig{Name: "claude", Type: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.APIKind != adapt.ApiAnthropicMessages || endpoint.Family != adapt.FamilyAnthropicMessages {
		t.Fatalf("unexpected endpoint: %+v", endpoint)
	}
	if endpoint.Tags[TagModelDBServiceID] != "anthropic" {
		t.Fatalf("unexpected tags: %+v", endpoint.Tags)
	}
}

func TestBuildRouterResolvesModelDBAlias(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(path, testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"gpt-test"})); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ModelDB: ModelDBConfig{
			CatalogPath: path,
			Aliases: []ModelDBAliasConfig{{
				Name:        "fast",
				ServiceID:   "openrouter",
				WireModelID: "openai/gpt-test",
			}},
		},
		Providers: []ProviderConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			APIKey:           "key",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []RouteConfig{{
			SourceAPI:    adapt.ApiOpenAIResponses,
			Model:        "public",
			Provider:     "openrouter",
			ProviderAPI:  adapt.ApiOpenRouterResponses,
			ModelDBModel: "fast",
		}},
	}
	ApplyDefaults(&cfg)
	r, err := BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "public", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != "openai/gpt-test" {
		t.Fatalf("native model = %q", route.NativeModel)
	}
}

func TestBuildRouterDynamicModelPassthrough(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:   "openai",
			Type:   "openai_responses",
			APIKey: "key",
			Model:  "configured-default",
		}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Provider:      "openai",
			ProviderAPI:   adapt.ApiOpenAIResponses,
			DynamicModels: true,
		}},
	}
	ApplyDefaults(&cfg)
	if cfg.Routes[0].NativeModel != "" {
		t.Fatalf("dynamic route native model default = %q", cfg.Routes[0].NativeModel)
	}
	r, err := BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "gpt-new", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != "gpt-new" || route.PublicModel != "gpt-new" {
		t.Fatalf("unexpected dynamic route: %+v", route)
	}
}

func TestBuildRouterDynamicModelCapabilitiesUseModelDBExposure(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(catalogPath, testDynamicCapabilityCatalog("openrouter")); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ModelDB: ModelDBConfig{CatalogPath: catalogPath},
		Providers: []ProviderConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			APIKey:           "key",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Provider:      "openrouter",
			ProviderAPI:   adapt.ApiOpenRouterResponses,
			DynamicModels: true,
		}},
	}
	ApplyDefaults(&cfg)
	r, err := BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified: unified.Request{
			Model:  "no-tools",
			Stream: true,
			Tools:  []unified.Tool{{Name: "lookup"}},
		},
	})
	if err == nil {
		t.Fatal("expected no-tools model to be rejected for tool request")
	}

	_, err = r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified: unified.Request{
			Model:  "missing-model",
			Stream: true,
		},
	})
	if err == nil {
		t.Fatal("expected catalog-missing dynamic model to be rejected")
	}

	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified: unified.Request{
			Model:  "with-tools",
			Stream: true,
			Tools:  []unified.Tool{{Name: "lookup"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != "with-tools" || !route.Capabilities.Tools {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestDynamicModelResolutionMatchesRouterModelDBResolution(t *testing.T) {
	catalogPath := filepath.Join(t.TempDir(), "catalog.json")
	if err := modeldb.SaveJSON(catalogPath, testDynamicAliasCatalog("openai")); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ModelDB: ModelDBConfig{CatalogPath: catalogPath},
		Providers: []ProviderConfig{{
			Name:             "openai",
			Type:             "openai_responses",
			APIKey:           "key",
			ModelDBServiceID: "openai",
		}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Provider:      "openai",
			ProviderAPI:   adapt.ApiOpenAIResponses,
			DynamicModels: true,
		}},
	}
	ApplyDefaults(&cfg)

	resolution, err := ResolveModel(cfg, "fast", adapt.ApiOpenAIResponses)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.MatchedAs != "dynamic_model" || resolution.NativeModel != "gpt-fast-wire" || resolution.ModelDBService != "openai" {
		t.Fatalf("unexpected diagnostic resolution: %+v", resolution)
	}

	r, err := BuildRouter(cfg)
	if err != nil {
		t.Fatal(err)
	}
	route, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "fast", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != resolution.NativeModel || route.TargetAPI != resolution.ProviderAPI {
		t.Fatalf("router/diagnostic mismatch: route=%+v resolution=%+v", route, resolution)
	}
}

func TestValidateRejectsDynamicRouteWithFixedModel(t *testing.T) {
	err := Validate(Config{
		Providers: []ProviderConfig{{Name: "openai", Type: "openai_responses"}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Model:         "public",
			Provider:      "openai",
			DynamicModels: true,
		}},
	})
	if err == nil {
		t.Fatal("expected dynamic route validation error")
	}
}

func TestEndpointWithPricingEnrichesUsageEvents(t *testing.T) {
	catalog := modeldb.NewCatalog()
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-test"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-test",
		Pricing:     &modeldb.Pricing{Input: 3, Output: 15},
	}
	endpoint := EndpointWithPricing(router.ProviderEndpoint{
		ProviderName: "openai",
		Client: fakeClient{events: []unified.Event{
			unified.NewUsageEvent(unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: 1000},
				{Kind: unified.TokenKindOutput, Count: 2000},
			}, nil),
		}},
		Tags: map[string]string{TagModelDBServiceID: "openai"},
	}, RouteConfig{NativeModel: "gpt-test"}, catalog)

	events, err := endpoint.Client.Request(context.Background(), unified.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var usage unified.UsageEvent
	for ev := range events {
		if ev, ok := ev.(unified.UsageEvent); ok {
			usage = ev
		}
	}
	if got, want := usage.Costs.ByKind(unified.CostKindInput), 1000*3.0/1_000_000; got != want {
		t.Fatalf("input cost = %g, want %g", got, want)
	}
	if got, want := usage.Costs.ByKind(unified.CostKindOutput), 2000*15.0/1_000_000; got != want {
		t.Fatalf("output cost = %g, want %g", got, want)
	}
}

func TestEndpointWithPricingUsesRequestModelForDynamicRoutes(t *testing.T) {
	catalog := modeldb.NewCatalog()
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-dynamic"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-dynamic",
		Pricing:     &modeldb.Pricing{Input: 2, Output: 10},
	}
	endpoint := EndpointWithPricing(router.ProviderEndpoint{
		ProviderName: "openai",
		Client: fakeClient{events: []unified.Event{
			unified.NewUsageEvent(unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: 1000},
				{Kind: unified.TokenKindOutput, Count: 2000},
			}, nil),
		}},
		Tags: map[string]string{TagModelDBServiceID: "openai"},
	}, RouteConfig{DynamicModels: true}, catalog)

	events, err := endpoint.Client.Request(context.Background(), unified.Request{Model: "gpt-dynamic"})
	if err != nil {
		t.Fatal(err)
	}
	var usage unified.UsageEvent
	for ev := range events {
		if ev, ok := ev.(unified.UsageEvent); ok {
			usage = ev
		}
	}
	if got, want := usage.Costs.ByKind(unified.CostKindInput), 1000*2.0/1_000_000; got != want {
		t.Fatalf("input cost = %g, want %g", got, want)
	}
	if got, want := usage.Costs.ByKind(unified.CostKindOutput), 2000*10.0/1_000_000; got != want {
		t.Fatalf("output cost = %g, want %g", got, want)
	}
}

func TestConfigUsesModelDBForDynamicPricingRoutes(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []RouteConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Provider:      "openrouter",
			ProviderAPI:   adapt.ApiOpenRouterResponses,
			DynamicModels: true,
		}},
	}
	if !ConfigUsesModelDB(cfg) {
		t.Fatal("expected dynamic pricing route to enable modeldb")
	}
}

func TestEndpointWithModelDBMetadataNarrowsCapabilities(t *testing.T) {
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "test"}
	catalog := modeldb.NewCatalog()
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Limits: modeldb.Limits{ContextWindow: 128000, MaxOutput: 4096}}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-test"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-test",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &modeldb.Capabilities{
				Streaming:        true,
				ToolUse:          false,
				StructuredOutput: true,
				Vision:           true,
			},
		}},
	}
	endpoint := EndpointWithModelDBMetadata(router.ProviderEndpoint{
		ProviderName: "openai",
		Family:       adapt.FamilyOpenAIResponses,
		Capabilities: router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		Tags:         map[string]string{TagModelDBServiceID: "openai"},
	}, RouteConfig{NativeModel: "gpt-test"}, catalog)
	if endpoint.Capabilities.Tools {
		t.Fatalf("expected tools disabled: %+v", endpoint.Capabilities)
	}
	if endpoint.Capabilities.MaxInputTokens != 128000 || endpoint.Capabilities.MaxOutputTokens != 4096 {
		t.Fatalf("expected limits: %+v", endpoint.Capabilities)
	}
}

func TestInspectConfigResolvesRoutesWithoutProviderCredentials(t *testing.T) {
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "test"}
	catalog := modeldb.NewCatalog()
	catalog.Models[key] = modeldb.ModelRecord{
		Key:    key,
		Limits: modeldb.Limits{ContextWindow: 128000, MaxOutput: 4096},
	}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openrouter", WireModelID: "openai/gpt-test"}] = modeldb.Offering{
		ServiceID:   "openrouter",
		WireModelID: "openai/gpt-test",
		ModelKey:    key,
		Pricing:     &modeldb.Pricing{Input: 1, Output: 2},
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &modeldb.Capabilities{
				Streaming:        true,
				ToolUse:          true,
				StructuredOutput: true,
			},
		}},
	}
	cfg := Config{
		Addr:        ":9090",
		MaxAttempts: 2,
		Providers: []ProviderConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			Model:            "openai/gpt-test",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Provider:    "openrouter",
			ProviderAPI: adapt.ApiOpenRouterResponses,
		}},
	}
	ApplyDefaults(&cfg)

	inspection, err := InspectConfigWithCatalog(cfg, catalog, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Providers) != 1 || inspection.Providers[0].InlineAPIKey {
		t.Fatalf("unexpected provider inspection: %+v", inspection.Providers)
	}
	if inspection.MaxAttempts != 2 {
		t.Fatalf("max attempts = %d, want 2", inspection.MaxAttempts)
	}
	if inspection.Providers[0].CapabilitySource != "provider_descriptor" {
		t.Fatalf("unexpected provider capability source: %+v", inspection.Providers[0])
	}
	if len(inspection.Routes) != 1 {
		t.Fatalf("routes = %+v", inspection.Routes)
	}
	route := inspection.Routes[0]
	if route.TargetAPI != adapt.ApiOpenRouterResponses || route.TargetFamily != adapt.FamilyOpenAIResponses {
		t.Fatalf("unexpected target metadata: %+v", route)
	}
	if !route.Capabilities.Streaming || !route.Capabilities.Tools || !route.Capabilities.JSONSchema {
		t.Fatalf("unexpected capabilities: %+v", route.Capabilities)
	}
	if route.CapabilitySource != "modeldb_exposure" {
		t.Fatalf("unexpected route capability source: %+v", route)
	}
	if route.Capabilities.MaxInputTokens != 128000 || route.Capabilities.MaxOutputTokens != 4096 {
		t.Fatalf("unexpected limits: %+v", route.Capabilities)
	}
	if !route.ModelDB.Enabled || !route.ModelDB.OfferingFound || !route.ModelDB.ExposureFound || !route.ModelDB.PricingAvailable {
		t.Fatalf("unexpected modeldb inspection: %+v", route.ModelDB)
	}
}

func TestInspectConfigReportsCapabilityOverrideSource(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:   "openai",
			Type:   "openai_responses",
			APIKey: "key",
			Capabilities: &CapabilityConfig{
				Streaming: boolPtr(true),
				Tools:     boolPtr(false),
			},
		}},
		Routes: []RouteConfig{{
			SourceAPI: adapt.ApiOpenAIResponses,
			Provider:  "openai",
		}},
	}
	ApplyDefaults(&cfg)

	inspection, err := InspectConfigWithCatalog(cfg, modeldb.Catalog{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Providers) != 1 || len(inspection.Routes) != 1 {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	if inspection.Providers[0].CapabilitySource != "config_override" {
		t.Fatalf("unexpected provider capability source: %+v", inspection.Providers[0])
	}
	if inspection.Routes[0].CapabilitySource != "config_override" {
		t.Fatalf("unexpected route capability source: %+v", inspection.Routes[0])
	}
}

func TestInspectConfigReportsResolvedModelDBModel(t *testing.T) {
	catalog := testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"fast-model"})
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []RouteConfig{{
			SourceAPI:    adapt.ApiOpenAIResponses,
			Provider:     "openrouter",
			ProviderAPI:  adapt.ApiOpenRouterResponses,
			ModelDBModel: "fast-model",
		}},
	}
	ApplyDefaults(&cfg)

	inspection, err := InspectConfigWithCatalog(cfg, catalog, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Routes) != 1 {
		t.Fatalf("routes = %+v", inspection.Routes)
	}
	route := inspection.Routes[0]
	if route.ModelDBModel != "fast-model" || route.NativeModel != "openai/gpt-test" {
		t.Fatalf("unexpected resolved route inspection: %+v", route)
	}
	if route.ModelDB.WireModelID != "openai/gpt-test" || !route.ModelDB.ExposureFound {
		t.Fatalf("unexpected modeldb inspection: %+v", route.ModelDB)
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func (c fakeClient) Request(context.Context, unified.Request) (<-chan unified.Event, error) {
	out := make(chan unified.Event, len(c.events))
	for _, ev := range c.events {
		out <- ev
	}
	close(out)
	return out, nil
}

func testResolvableModelDBCatalog(serviceID, wireModelID string, aliases []string) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "test", Family: "model"}
	catalog.Services[serviceID] = modeldb.Service{ID: serviceID, Name: "Test Service"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Aliases: aliases}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}] = modeldb.Offering{
		ServiceID:   serviceID,
		WireModelID: wireModelID,
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType:             modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &modeldb.Capabilities{Streaming: true},
		}},
	}
	return catalog
}

func testDynamicCapabilityCatalog(serviceID string) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	catalog.Services[serviceID] = modeldb.Service{ID: serviceID, Name: "Test Service"}
	add := func(wireModelID string, tools bool) {
		key := modeldb.ModelKey{Creator: "test", Family: wireModelID}
		catalog.Models[key] = modeldb.ModelRecord{Key: key}
		catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}] = modeldb.Offering{
			ServiceID:   serviceID,
			WireModelID: wireModelID,
			ModelKey:    key,
			Exposures: []modeldb.OfferingExposure{{
				APIType: modeldb.APITypeOpenAIResponses,
				ExposedCapabilities: &modeldb.Capabilities{
					Streaming:        true,
					ToolUse:          tools,
					StructuredOutput: true,
				},
			}},
		}
	}
	add("no-tools", false)
	add("with-tools", true)
	return catalog
}

func testDynamicAliasCatalog(serviceID string) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "fast"}
	catalog.Services[serviceID] = modeldb.Service{ID: serviceID, Name: "Test Service"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Name: "GPT Fast", Aliases: []string{"fast"}}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: "gpt-fast-wire"}] = modeldb.Offering{
		ServiceID:   serviceID,
		WireModelID: "gpt-fast-wire",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &modeldb.Capabilities{
				Streaming:        true,
				ToolUse:          true,
				StructuredOutput: true,
			},
		}},
	}
	return catalog
}
