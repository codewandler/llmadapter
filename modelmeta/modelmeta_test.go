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

func TestResolvedMetadataProjectsExposureDetails(t *testing.T) {
	catalog := fixtureCatalog(modeldb.Capabilities{
		Reasoning: &modeldb.ReasoningCapability{
			Available:      true,
			Modes:          []modeldb.ReasoningMode{modeldb.ReasoningModeAdaptive},
			Efforts:        []modeldb.ReasoningEffortLevel{modeldb.ReasoningEffortHigh},
			DefaultDisplay: "summarized",
		},
	})
	ref := modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-test"}
	offering := catalog.Offerings[ref]
	offering.Exposures[0].SupportedParameters = []modeldb.NormalizedParameter{modeldb.ParamReasoningEffort}
	offering.Exposures[0].ParameterValues = map[string][]string{string(modeldb.ParamReasoningEffort): {string(modeldb.ReasoningEffortHigh)}}
	offering.Exposures[0].ParameterMappings = []modeldb.ParameterMapping{{Normalized: modeldb.ParamReasoningEffort, WireName: "reasoning.effort"}}
	offering.Exposures[0].ParameterValueMappings = []modeldb.ParameterValueMapping{{
		Parameter: modeldb.ParamReasoningEffort,
		Canonical: string(modeldb.ReasoningEffortMax),
		WireValue: string(modeldb.ReasoningEffortXHigh),
	}}
	catalog.Offerings[ref] = offering

	got, ok := ResolvedMetadata(catalog, "openai", "gpt-test", adapt.FamilyOpenAIResponses)
	if !ok {
		t.Fatalf("expected metadata")
	}
	if got.ServiceID != "openai" || got.WireModelID != "gpt-test" || got.APIType != string(modeldb.APITypeOpenAIResponses) {
		t.Fatalf("unexpected identity: %+v", got)
	}
	if len(got.ReasoningModes) != 1 || got.ReasoningModes[0] != "adaptive" {
		t.Fatalf("modes = %+v", got.ReasoningModes)
	}
	if len(got.ReasoningEfforts) != 1 || got.ReasoningEfforts[0] != "high" {
		t.Fatalf("efforts = %+v", got.ReasoningEfforts)
	}
	if got.ParameterMappings[string(modeldb.ParamReasoningEffort)] != "reasoning.effort" {
		t.Fatalf("mappings = %+v", got.ParameterMappings)
	}
	if got.ParameterValues[string(modeldb.ParamReasoningEffort)][0] != "high" {
		t.Fatalf("values = %+v", got.ParameterValues)
	}
	if got.ParameterValueMappings[string(modeldb.ParamReasoningEffort)][string(modeldb.ReasoningEffortMax)] != string(modeldb.ReasoningEffortXHigh) {
		t.Fatalf("value mappings = %+v", got.ParameterValueMappings)
	}
	if got.DefaultDisplayMode != "summarized" {
		t.Fatalf("default display = %q", got.DefaultDisplayMode)
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
