package main

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

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"addr":":9090",
		"health_cooldown":"5s",
		"providers":[{"name":"anthropic","type":"anthropic","api_key_env":"ANTHROPIC_API_KEY","model":"native","modeldb_service_id":"anthropic","priority":7,"capabilities":{"vision":false}}],
		"routes":[{"source_api":"openai.chat_completions","model":"public","provider":"anthropic","modeldb_wire_model_id":"native-pricing","weight":11}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || len(cfg.Providers) != 1 || len(cfg.Routes) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Routes[0].SourceAPI != adapt.ApiOpenAIChatCompletions || cfg.Routes[0].NativeModel != "native" {
		t.Fatalf("unexpected route defaults: %+v", cfg.Routes[0])
	}
	if cfg.Providers[0].Priority != 7 || cfg.Routes[0].Weight != 11 {
		t.Fatalf("unexpected routing weights: %+v %+v", cfg.Providers[0], cfg.Routes[0])
	}
	if cfg.HealthCooldown != "5s" || cfg.Providers[0].Capabilities == nil || cfg.Providers[0].Capabilities.Vision == nil || *cfg.Providers[0].Capabilities.Vision {
		t.Fatalf("unexpected stabilization config: %+v", cfg)
	}
	if cfg.Providers[0].ModelDBServiceID != "anthropic" || cfg.Routes[0].ModelDBWireModelID != "native-pricing" {
		t.Fatalf("unexpected modeldb metadata: %+v %+v", cfg.Providers[0], cfg.Routes[0])
	}
}

func TestLoadConfigModelDBPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"modeldb":{"catalog_path":"catalog.json","overlay_paths":["local.json"],"aliases":[{"name":"fast","service_id":"anthropic","wire_model_id":"claude-fast"}]},
		"providers":[{"name":"anthropic","type":"anthropic"}],
		"routes":[{"provider":"anthropic","modeldb_model":"fast"}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelDB.CatalogPath != "catalog.json" || len(cfg.ModelDB.OverlayPaths) != 1 || cfg.ModelDB.OverlayPaths[0] != "local.json" {
		t.Fatalf("unexpected modeldb config: %+v", cfg.ModelDB)
	}
	if len(cfg.ModelDB.Aliases) != 1 || cfg.ModelDB.Aliases[0].Name != "fast" || cfg.Routes[0].ModelDBModel != "fast" {
		t.Fatalf("unexpected modeldb alias config: %+v %+v", cfg.ModelDB, cfg.Routes[0])
	}
}

func TestDefaultConfigIncludesGatewayRoutes(t *testing.T) {
	t.Setenv("LLMADAPTER_ADDR", "")
	t.Setenv("ANTHROPIC_API_KEY", "key")
	cfg := defaultConfigFromEnv()
	if len(cfg.Routes) != 3 {
		t.Fatalf("routes = %+v", cfg.Routes)
	}
	if cfg.Routes[0].SourceAPI != adapt.ApiOpenAIChatCompletions || cfg.Routes[1].SourceAPI != adapt.ApiAnthropicMessages || cfg.Routes[2].SourceAPI != adapt.ApiOpenAIResponses {
		t.Fatalf("unexpected default routes: %+v", cfg.Routes)
	}
}

func TestValidateConfig(t *testing.T) {
	err := validateConfig(config{
		Providers: []providerConfig{{Name: "anthropic", Type: "anthropic"}},
		Routes:    []routeConfig{{Provider: "missing"}},
	})
	if err == nil {
		t.Fatalf("expected unknown provider error")
	}

	err = validateConfig(config{
		HealthCooldown: "soon",
		Providers:      []providerConfig{{Name: "anthropic", Type: "anthropic"}},
		Routes:         []routeConfig{{Provider: "anthropic"}},
	})
	if err == nil {
		t.Fatalf("expected invalid health cooldown error")
	}
}

func TestValidateConfigRequiresProviderAPIForAmbiguousProvider(t *testing.T) {
	err := validateConfig(config{
		Providers: []providerConfig{
			{Name: "openrouter", Type: "openrouter_chat"},
			{Name: "openrouter", Type: "anthropic"},
		},
		Routes: []routeConfig{{Provider: "openrouter"}},
	})
	if err == nil {
		t.Fatalf("expected ambiguous provider endpoint error")
	}

	err = validateConfig(config{
		Providers: []providerConfig{
			{Name: "openrouter", Type: "openrouter_chat"},
			{Name: "openrouter", Type: "anthropic"},
		},
		Routes: []routeConfig{{Provider: "openrouter", ProviderAPI: adapt.ApiOpenRouterChatCompletions}},
	})
	if err != nil {
		t.Fatalf("unexpected provider endpoint validation error: %v", err)
	}
}

func TestValidateConfigRejectsDynamicRouteWithFixedModel(t *testing.T) {
	err := validateConfig(config{
		Providers: []providerConfig{{Name: "openai", Type: "openai_responses"}},
		Routes: []routeConfig{{
			SourceAPI:     adapt.ApiOpenAIResponses,
			Model:         "public",
			Provider:      "openai",
			DynamicModels: true,
		}},
	})
	if err == nil {
		t.Fatalf("expected dynamic route validation error")
	}
}

func TestBuildProviderOpenAIRequiresKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_KEY", "")
	_, err := buildProvider(providerConfig{Name: "openai", Type: "openai_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "openai", Type: "openai_responses"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestBuildProviderOpenRouterRequiresKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_KEY", "")
	_, err := buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_responses"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_messages"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestBuildProviderMiniMaxRequiresKey(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("MINIMAX_KEY", "")
	_, err := buildProvider(providerConfig{Name: "minimax", Type: "minimax_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "minimax", Type: "minimax_messages"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestBuildProviderClaudeMessagesAuthSources(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if err := os.WriteFile(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := buildProvider(providerConfig{Name: "claude", Type: "claude"})
	if err != nil {
		t.Fatalf("unexpected claude provider error: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}

	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	_, err = buildProvider(providerConfig{Name: "claude", Type: "claude"})
	if err == nil {
		t.Fatalf("expected missing local Claude credentials error")
	}
}

func TestBuildProviderCodexAuthSources(t *testing.T) {
	t.Setenv("CODEX_ACCESS_TOKEN", "token")
	client, err := buildProvider(providerConfig{Name: "codex", Type: "codex_responses"})
	if err != nil {
		t.Fatalf("unexpected codex provider error: %v", err)
	}
	if client == nil {
		t.Fatalf("expected client")
	}

	t.Setenv("CODEX_ACCESS_TOKEN", "")
	t.Setenv("CODEX_CODE_OAUTH_TOKEN", "")
	t.Setenv("CODEX_AUTH_PATH", filepath.Join(t.TempDir(), "missing-auth.json"))
	_, err = buildProvider(providerConfig{Name: "codex", Type: "codex_responses"})
	if err == nil {
		t.Fatalf("expected missing local Codex credentials error")
	}
}

func TestProviderEndpointMetadata(t *testing.T) {
	tests := []struct {
		providerType string
		apiKind      adapt.ApiKind
		family       adapt.ApiFamily
		tools        bool
	}{
		{"claude", adapt.ApiAnthropicMessages, adapt.FamilyAnthropicMessages, true},
		{"openai_responses", adapt.ApiOpenAIResponses, adapt.FamilyOpenAIResponses, true},
		{"codex_responses", adapt.ApiCodexResponses, adapt.FamilyOpenAIResponses, true},
		{"openrouter_chat", adapt.ApiOpenRouterChatCompletions, adapt.FamilyOpenAIChatCompletions, true},
		{"openrouter_responses", adapt.ApiOpenRouterResponses, adapt.FamilyOpenAIResponses, true},
		{"openrouter_messages", adapt.ApiOpenRouterAnthropicMessages, adapt.FamilyAnthropicMessages, true},
		{"minimax_chat", adapt.ApiMiniMaxChatCompletions, adapt.FamilyOpenAIChatCompletions, true},
		{"minimax_messages", adapt.ApiMiniMaxAnthropicMessages, adapt.FamilyAnthropicMessages, true},
	}
	for _, tt := range tests {
		apiKind, family, capabilities, err := providerEndpointMetadata(tt.providerType)
		if err != nil {
			t.Fatal(err)
		}
		if apiKind != tt.apiKind || family != tt.family {
			t.Fatalf("unexpected endpoint metadata: %q %q", apiKind, family)
		}
		if !capabilities.Streaming || capabilities.Tools != tt.tools {
			t.Fatalf("unexpected capabilities: %+v", capabilities)
		}
		if (tt.providerType == "openai_chat" || tt.providerType == "openai_responses" || tt.providerType == "openrouter_chat" || tt.providerType == "openrouter_responses") && (!capabilities.JSONMode || !capabilities.JSONSchema) {
			t.Fatalf("expected JSON capabilities: %+v", capabilities)
		}
		if (tt.providerType == "openai_chat" || tt.providerType == "openai_responses" || tt.providerType == "openrouter_chat" || tt.providerType == "openrouter_responses" || tt.providerType == "openrouter_messages") && !capabilities.Vision {
			t.Fatalf("expected vision capability: %+v", capabilities)
		}
	}
}

func TestApplyCapabilityOverrides(t *testing.T) {
	vision := false
	jsonSchema := false
	reasoning := true
	caps := applyCapabilityOverrides(router.CapabilitySet{
		Streaming:  true,
		Tools:      true,
		Vision:     true,
		JSONMode:   true,
		JSONSchema: true,
	}, &capabilityConfig{
		Vision:     &vision,
		JSONSchema: &jsonSchema,
		Reasoning:  &reasoning,
	})
	if !caps.Streaming || !caps.Tools || !caps.JSONMode {
		t.Fatalf("unexpected unchanged capabilities: %+v", caps)
	}
	if caps.Vision || caps.JSONSchema || !caps.Reasoning {
		t.Fatalf("unexpected overridden capabilities: %+v", caps)
	}
}

func TestClaudeMessagesDefaultsModelDBServiceIDToAnthropic(t *testing.T) {
	provider := providerConfig{Name: "claude", Type: "claude"}
	tags := providerEndpointTags(provider)
	if tags[tagModelDBServiceID] != "anthropic" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
	cfg := config{
		Providers: []providerConfig{provider},
		Routes: []routeConfig{{
			Provider:    "claude",
			NativeModel: "claude-test",
		}},
	}
	if !configUsesModelDB(cfg) {
		t.Fatalf("expected Claude provider to enable modeldb pricing by default")
	}
}

func TestCodexResponsesDefaultsModelDBServiceIDToCodex(t *testing.T) {
	provider := providerConfig{Name: "codex", Type: "codex_responses"}
	tags := providerEndpointTags(provider)
	if tags[tagModelDBServiceID] != "codex" {
		t.Fatalf("unexpected tags: %+v", tags)
	}
}

func TestConfigUsesPricingForConfiguredModelDBOffering(t *testing.T) {
	cfg := config{
		Providers: []providerConfig{
			{Name: "openrouter", Type: "openrouter_chat"},
			{Name: "openrouter", Type: "openrouter_responses", Model: "openai/gpt-4.1-mini", ModelDBServiceID: "openrouter"},
		},
		Routes: []routeConfig{{
			Provider:    "openrouter",
			ProviderAPI: adapt.ApiOpenRouterResponses,
		}},
	}
	applyConfigDefaults(&cfg)
	if !configUsesPricing(cfg) {
		t.Fatalf("expected pricing to be enabled")
	}
}

func TestConfigUsesPricingForDynamicModelDBRoutes(t *testing.T) {
	cfg := config{
		Providers: []providerConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []routeConfig{{
			Provider:      "openrouter",
			ProviderAPI:   adapt.ApiOpenRouterResponses,
			DynamicModels: true,
		}},
	}
	if !configUsesPricing(cfg) {
		t.Fatalf("expected dynamic pricing route to enable modeldb")
	}
}

func TestLoadModelDBCatalogFromConfiguredPathAndOverlays(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := modeldb.SaveJSON(basePath, testModelDBCatalog("base-model", nil)); err != nil {
		t.Fatal(err)
	}
	if err := modeldb.SaveJSON(overlayPath, testModelDBCatalog("overlay-model", &modeldb.Pricing{Input: 1, Output: 2})); err != nil {
		t.Fatal(err)
	}

	catalog, err := loadModelDBCatalog(modelDBConfig{
		CatalogPath:  basePath,
		OverlayPaths: []string{overlayPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: "testsvc", WireModelID: "base-model"}]; !ok {
		t.Fatalf("expected base offering")
	}
	offering, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: "testsvc", WireModelID: "overlay-model"}]
	if !ok {
		t.Fatalf("expected overlay offering")
	}
	if offering.Pricing == nil || offering.Pricing.Input != 1 {
		t.Fatalf("expected overlay pricing: %+v", offering.Pricing)
	}
}

func TestResolveRouteModelDBModelUsesConfiguredAlias(t *testing.T) {
	catalog := testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"gpt-test"})
	route, err := resolveRouteModelDBModel(routeConfig{
		Provider:     "openrouter",
		ProviderAPI:  adapt.ApiOpenRouterResponses,
		ModelDBModel: "fast",
	}, router.ProviderEndpoint{
		ProviderName: "openrouter",
		Family:       adapt.FamilyOpenAIResponses,
		Tags:         map[string]string{tagModelDBServiceID: "openrouter"},
	}, catalog, modelDBConfig{Aliases: []modelDBAliasConfig{{
		Name:        "fast",
		ServiceID:   "openrouter",
		WireModelID: "openai/gpt-test",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != "openai/gpt-test" || route.ModelDBWireModelID != "openai/gpt-test" {
		t.Fatalf("unexpected resolved route: %+v", route)
	}
}

func TestResolveRouteModelDBModelUsesCatalogModelAlias(t *testing.T) {
	catalog := testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"fast-model"})
	route, err := resolveRouteModelDBModel(routeConfig{
		Provider:     "openrouter",
		ProviderAPI:  adapt.ApiOpenRouterResponses,
		ModelDBModel: "fast-model",
	}, router.ProviderEndpoint{
		ProviderName: "openrouter",
		Family:       adapt.FamilyOpenAIResponses,
		Tags:         map[string]string{tagModelDBServiceID: "openrouter"},
	}, catalog, modelDBConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if route.NativeModel != "openai/gpt-test" || route.ModelDBWireModelID != "openai/gpt-test" {
		t.Fatalf("unexpected resolved route: %+v", route)
	}
}

func TestResolveRouteModelDBModelRejectsConflictingNativeModel(t *testing.T) {
	catalog := testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"fast-model"})
	_, err := resolveRouteModelDBModel(routeConfig{
		Provider:     "openrouter",
		ProviderAPI:  adapt.ApiOpenRouterResponses,
		ModelDBModel: "fast-model",
		NativeModel:  "other-model",
	}, router.ProviderEndpoint{
		ProviderName: "openrouter",
		Family:       adapt.FamilyOpenAIResponses,
		Tags:         map[string]string{tagModelDBServiceID: "openrouter"},
	}, catalog, modelDBConfig{})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestEndpointWithPricingEnrichesUsageEvents(t *testing.T) {
	catalog := modeldb.NewCatalog()
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "anthropic", WireModelID: "claude-test"}] = modeldb.Offering{
		ServiceID:   "anthropic",
		WireModelID: "claude-test",
		Pricing:     &modeldb.Pricing{Input: 3, Output: 15},
	}

	endpoint := endpointWithPricing(router.ProviderEndpoint{
		ProviderName: "anthropic",
		Client: fakeClient{events: []unified.Event{
			unified.NewUsageEvent(unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: 1000},
				{Kind: unified.TokenKindOutput, Count: 2000},
			}, nil),
		}},
		Tags: map[string]string{tagModelDBServiceID: "anthropic"},
	}, routeConfig{NativeModel: "claude-test"}, catalog)

	events, err := endpoint.Client.Request(context.Background(), unified.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var usage unified.UsageEvent
	for ev := range events {
		if errEv, ok := ev.(unified.ErrorEvent); ok {
			t.Fatalf("unexpected error event: %v", errEv.Err)
		}
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
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "anthropic", WireModelID: "claude-dynamic"}] = modeldb.Offering{
		ServiceID:   "anthropic",
		WireModelID: "claude-dynamic",
		Pricing:     &modeldb.Pricing{Input: 2, Output: 10},
	}

	endpoint := endpointWithPricing(router.ProviderEndpoint{
		ProviderName: "anthropic",
		Client: fakeClient{events: []unified.Event{
			unified.NewUsageEvent(unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: 1000},
				{Kind: unified.TokenKindOutput, Count: 2000},
			}, nil),
		}},
		Tags: map[string]string{tagModelDBServiceID: "anthropic"},
	}, routeConfig{DynamicModels: true}, catalog)

	events, err := endpoint.Client.Request(context.Background(), unified.Request{Model: "claude-dynamic"})
	if err != nil {
		t.Fatal(err)
	}
	var usage unified.UsageEvent
	for ev := range events {
		if errEv, ok := ev.(unified.ErrorEvent); ok {
			t.Fatalf("unexpected error event: %v", errEv.Err)
		}
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

func TestEndpointWithModelDBMetadataEnrichesRouteCapabilities(t *testing.T) {
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

	endpoint := endpointWithModelDBMetadata(router.ProviderEndpoint{
		ProviderName: "openrouter",
		Family:       adapt.FamilyOpenAIResponses,
		Capabilities: router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		Tags:         map[string]string{tagModelDBServiceID: "openrouter"},
	}, routeConfig{ModelDBWireModelID: "openai/gpt-test"}, catalog)

	if endpoint.Capabilities.Tools {
		t.Fatalf("expected modeldb metadata to disable tools: %+v", endpoint.Capabilities)
	}
	if !endpoint.Capabilities.Streaming || !endpoint.Capabilities.Vision || !endpoint.Capabilities.JSONMode || !endpoint.Capabilities.JSONSchema {
		t.Fatalf("expected supported capabilities to remain enabled: %+v", endpoint.Capabilities)
	}
	if endpoint.Capabilities.MaxInputTokens != 128000 || endpoint.Capabilities.MaxOutputTokens != 4096 {
		t.Fatalf("unexpected limits: %+v", endpoint.Capabilities)
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
	cfg := config{
		Addr: ":9090",
		Providers: []providerConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			Model:            "openai/gpt-test",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []routeConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Provider:    "openrouter",
			ProviderAPI: adapt.ApiOpenRouterResponses,
		}},
	}
	applyConfigDefaults(&cfg)

	inspection, err := inspectConfigWithCatalog(cfg, catalog, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inspection.Providers) != 1 || inspection.Providers[0].Credential.InlineAPIKey {
		t.Fatalf("unexpected provider inspection: %+v", inspection.Providers)
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
	if route.Capabilities.MaxInputTokens != 128000 || route.Capabilities.MaxOutputTokens != 4096 {
		t.Fatalf("unexpected limits: %+v", route.Capabilities)
	}
	if !route.ModelDB.Enabled || !route.ModelDB.OfferingFound || !route.ModelDB.ExposureFound || !route.ModelDB.PricingAvailable {
		t.Fatalf("unexpected modeldb inspection: %+v", route.ModelDB)
	}
}

func TestInspectConfigReportsResolvedModelDBModel(t *testing.T) {
	catalog := testResolvableModelDBCatalog("openrouter", "openai/gpt-test", []string{"fast-model"})
	cfg := config{
		Providers: []providerConfig{{
			Name:             "openrouter",
			Type:             "openrouter_responses",
			ModelDBServiceID: "openrouter",
		}},
		Routes: []routeConfig{{
			SourceAPI:    adapt.ApiOpenAIResponses,
			Provider:     "openrouter",
			ProviderAPI:  adapt.ApiOpenRouterResponses,
			ModelDBModel: "fast-model",
		}},
	}
	applyConfigDefaults(&cfg)

	inspection, err := inspectConfigWithCatalog(cfg, catalog, true)
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

type fakeClient struct {
	events []unified.Event
}

func testModelDBCatalog(wireModelID string, pricing *modeldb.Pricing) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "test", Family: wireModelID}
	catalog.Services["testsvc"] = modeldb.Service{ID: "testsvc", Name: "Test Service"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "testsvc", WireModelID: wireModelID}] = modeldb.Offering{
		ServiceID:   "testsvc",
		WireModelID: wireModelID,
		ModelKey:    key,
		Pricing:     pricing,
		PricingStatus: func() string {
			if pricing != nil {
				return "known"
			}
			return "unknown"
		}(),
	}
	return catalog
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

func (c fakeClient) Request(context.Context, unified.Request) (<-chan unified.Event, error) {
	out := make(chan unified.Event, len(c.events))
	for _, ev := range c.events {
		out <- ev
	}
	close(out)
	return out, nil
}
