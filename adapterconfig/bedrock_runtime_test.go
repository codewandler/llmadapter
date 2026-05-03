package adapterconfig

import (
	"context"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

func TestResolveRouteModelDBModelUsesBedrockRuntimeAccess(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	catalog := bedrockRuntimeCatalogForTest(t)
	endpoint := router.ProviderEndpoint{
		Family: adapt.FamilyBedrockConverse,
		Tags:   map[string]string{TagModelDBServiceID: "bedrock"},
	}
	route := RouteConfig{
		Provider:     "bedrock",
		ModelDBModel: "anthropic.claude-sonnet-4-6",
	}

	got, err := ResolveRouteModelDBModel(route, endpoint, catalog, ModelDBConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelDBWireModelID != "anthropic.claude-sonnet-4-6" {
		t.Fatalf("modeldb wire model = %q", got.ModelDBWireModelID)
	}
	if got.NativeModel != "us.anthropic.claude-sonnet-4-6" {
		t.Fatalf("native model = %q", got.NativeModel)
	}
}

func TestDynamicModelResolverUsesBedrockRuntimeAccess(t *testing.T) {
	t.Setenv("AWS_REGION", "us-east-1")
	catalog := bedrockRuntimeCatalogForTest(t)
	endpoint := router.ProviderEndpoint{
		Family:       adapt.FamilyBedrockConverse,
		Capabilities: router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true},
		Tags:         map[string]string{TagModelDBServiceID: "bedrock"},
	}
	resolver := dynamicModelResolver(endpoint, RouteConfig{DynamicModels: true}, catalog, ModelDBConfig{}, true)
	if resolver == nil {
		t.Fatal("expected resolver")
	}

	got, ok := resolver(context.Background(), adapt.Request{Unified: unified.Request{Model: "anthropic.claude-sonnet-4-6"}}, endpoint)
	if !ok {
		t.Fatal("expected model resolution")
	}
	if got.NativeModel != "us.anthropic.claude-sonnet-4-6" {
		t.Fatalf("native model = %q", got.NativeModel)
	}
	if got.ModelMetadata == nil || got.ModelMetadata.WireModelID != "anthropic.claude-sonnet-4-6" {
		t.Fatalf("metadata = %+v", got.ModelMetadata)
	}
}

func bedrockRuntimeCatalogForTest(t *testing.T) modeldb.Catalog {
	t.Helper()
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "anthropic", Family: "claude", Series: "sonnet", Version: "4.6"}
	if err := modeldb.MergeCatalogFragment(&catalog, &modeldb.Fragment{
		Services: []modeldb.Service{{ID: "bedrock"}},
		Models: []modeldb.ModelRecord{{
			Key:          key,
			Name:         "Claude Sonnet 4.6",
			Capabilities: modeldb.Capabilities{Streaming: true, ToolUse: true, Reasoning: &modeldb.ReasoningCapability{Available: true}},
		}},
		Offerings: []modeldb.Offering{{
			ServiceID:   "bedrock",
			WireModelID: "anthropic.claude-sonnet-4-6",
			ModelKey:    key,
			Exposures: []modeldb.OfferingExposure{{
				APIType:             modeldb.APITypeBedrockConverse,
				ExposedCapabilities: &modeldb.Capabilities{Streaming: true, ToolUse: true, Reasoning: &modeldb.ReasoningCapability{Available: true}},
			}},
		}},
		Runtimes: []modeldb.Runtime{{ID: "bedrock-us", ServiceID: "bedrock"}},
		RuntimeAccess: []modeldb.RuntimeAccess{{
			RuntimeID:      "bedrock-us",
			Offering:       modeldb.OfferingRef{ServiceID: "bedrock", WireModelID: "anthropic.claude-sonnet-4-6"},
			Routable:       true,
			ResolvedWireID: "us.anthropic.claude-sonnet-4-6",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	return catalog
}
