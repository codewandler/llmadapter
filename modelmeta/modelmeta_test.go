package modelmeta

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

func TestAPITypeForFamily(t *testing.T) {
	tests := []struct {
		family adapt.ApiFamily
		want   modeldb.APIType
	}{
		{adapt.FamilyAnthropicMessages, modeldb.APITypeAnthropicMessages},
		{adapt.FamilyOpenAIChatCompletions, modeldb.APITypeOpenAIChat},
		{adapt.FamilyOpenAIResponses, modeldb.APITypeOpenAIResponses},
	}
	for _, tt := range tests {
		got, ok := APITypeForFamily(tt.family)
		if !ok || got != tt.want {
			t.Fatalf("APITypeForFamily(%q) = %q %v, want %q true", tt.family, got, ok, tt.want)
		}
	}
}

func TestEnrichCapabilitiesNarrowsEndpointCapabilitiesAndAppliesLimits(t *testing.T) {
	catalog := fixtureCatalog(modeldb.Capabilities{
		Streaming:        true,
		ToolUse:          true,
		StructuredOutput: true,
		Vision:           false,
		Caching:          &modeldb.CachingCapability{Available: true},
	})
	base := router.CapabilitySet{
		Streaming:       true,
		Tools:           true,
		Vision:          true,
		JSONMode:        true,
		JSONSchema:      true,
		PromptCaching:   false,
		MaxInputTokens:  10,
		MaxOutputTokens: 20,
	}

	got, ok := EnrichCapabilities(base, catalog, "openai", "gpt-test", adapt.FamilyOpenAIResponses)
	if !ok {
		t.Fatalf("expected metadata match")
	}
	if !got.Streaming || !got.Tools || !got.JSONMode || !got.JSONSchema {
		t.Fatalf("expected supported capabilities to remain enabled: %+v", got)
	}
	if got.Vision {
		t.Fatalf("expected modeldb exposure to disable unsupported vision: %+v", got)
	}
	if !got.PromptCaching {
		t.Fatalf("expected prompt caching metadata: %+v", got)
	}
	if got.MaxInputTokens != 200000 || got.MaxOutputTokens != 8192 {
		t.Fatalf("unexpected limits: %+v", got)
	}
}

func TestEnrichCapabilitiesDoesNotEnableUnsupportedEndpointFamilyCapabilities(t *testing.T) {
	catalog := fixtureCatalog(modeldb.Capabilities{
		Streaming:        true,
		ToolUse:          true,
		StructuredOutput: true,
		Vision:           true,
	})
	base := router.CapabilitySet{Streaming: true}

	got, ok := EnrichCapabilities(base, catalog, "openai", "gpt-test", adapt.FamilyOpenAIResponses)
	if !ok {
		t.Fatalf("expected metadata match")
	}
	if got.Tools || got.Vision || got.JSONMode || got.JSONSchema {
		t.Fatalf("modeldb must not enable endpoint-family unsupported capabilities: %+v", got)
	}
}

func TestEnrichCapabilitiesMissesUnknownExposure(t *testing.T) {
	got, ok := EnrichCapabilities(router.CapabilitySet{Streaming: true}, fixtureCatalog(modeldb.Capabilities{}), "openai", "gpt-test", adapt.FamilyAnthropicMessages)
	if ok {
		t.Fatalf("expected no exposure match")
	}
	if !got.Streaming {
		t.Fatalf("expected unchanged capabilities: %+v", got)
	}
}

func fixtureCatalog(caps modeldb.Capabilities) modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "test"}
	catalog.Models[key] = modeldb.ModelRecord{
		Key:    key,
		Limits: modeldb.Limits{ContextWindow: 128000, MaxOutput: 8192},
	}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-test"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-test",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType:             modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &caps,
		}},
		LimitsOverride: &modeldb.Limits{ContextWindow: 200000},
	}
	return catalog
}
