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
