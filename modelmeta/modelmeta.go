package modelmeta

import (
	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/modeldb"
)

// APITypeForFamily maps a llmadapter compatibility family to the matching
// modeldb exposure type. The exact provider API kind remains owned by the
// configured ProviderEndpoint.
func APITypeForFamily(family adapt.ApiFamily) (modeldb.APIType, bool) {
	switch family {
	case adapt.FamilyAnthropicMessages:
		return modeldb.APITypeAnthropicMessages, true
	case adapt.FamilyOpenAIChatCompletions:
		return modeldb.APITypeOpenAIChat, true
	case adapt.FamilyOpenAIResponses:
		return modeldb.APITypeOpenAIResponses, true
	default:
		return "", false
	}
}

// EnrichCapabilities narrows endpoint-family capabilities with modeldb
// offering/exposure metadata and adds known model limits. It never enables
// invocation capabilities that the endpoint family did not already advertise.
func EnrichCapabilities(base router.CapabilitySet, catalog modeldb.Catalog, serviceID, wireModelID string, family adapt.ApiFamily) (router.CapabilitySet, bool) {
	apiType, ok := APITypeForFamily(family)
	if !ok {
		return base, false
	}
	return EnrichCapabilitiesForAPIType(base, catalog, serviceID, wireModelID, apiType)
}

func EnrichCapabilitiesForAPIType(base router.CapabilitySet, catalog modeldb.Catalog, serviceID, wireModelID string, apiType modeldb.APIType) (router.CapabilitySet, bool) {
	offering, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}]
	if !ok {
		return base, false
	}
	exposure := offering.Exposure(apiType)
	if exposure == nil {
		return base, false
	}

	out := base
	if exposure.ExposedCapabilities != nil {
		out = applyExposureCapabilities(out, *exposure.ExposedCapabilities, *exposure)
	}
	if model, ok := catalog.Models[offering.ModelKey]; ok {
		out = applyLimits(out, model.Limits)
	}
	if offering.LimitsOverride != nil {
		out = applyLimits(out, *offering.LimitsOverride)
	}
	return out, true
}

func applyExposureCapabilities(base router.CapabilitySet, caps modeldb.Capabilities, exposure modeldb.OfferingExposure) router.CapabilitySet {
	out := base
	out.Streaming = base.Streaming && caps.Streaming
	out.Tools = base.Tools && (caps.ToolUse || exposure.SupportsParameter(modeldb.ParamTools))
	out.ParallelTools = base.ParallelTools && (caps.ParallelToolCalls || exposure.SupportsParameter(modeldb.ParamParallelTools))
	out.Vision = base.Vision && caps.Vision
	out.JSONMode = base.JSONMode && supportsStructuredOutput(caps, exposure)
	out.JSONSchema = base.JSONSchema && supportsStructuredOutput(caps, exposure)
	out.Reasoning = base.Reasoning && supportsReasoning(caps, exposure)
	out.ReasoningDeltas = base.ReasoningDeltas && supportsReasoning(caps, exposure)
	out.BuiltInWebSearch = base.BuiltInWebSearch && (caps.WebSearch || exposure.SupportsParameter(modeldb.ParamWebSearch))
	out.PromptCaching = caps.Caching != nil && caps.Caching.Available
	return out
}

func supportsStructuredOutput(caps modeldb.Capabilities, exposure modeldb.OfferingExposure) bool {
	return caps.StructuredOutput || caps.StructuredOutputs || exposure.SupportsParameter(modeldb.ParamResponseFormat)
}

func supportsReasoning(caps modeldb.Capabilities, exposure modeldb.OfferingExposure) bool {
	return caps.SupportsReasoning() ||
		exposure.SupportsParameter(modeldb.ParamReasoningEffort) ||
		exposure.SupportsParameter(modeldb.ParamThinking)
}

func applyLimits(caps router.CapabilitySet, limits modeldb.Limits) router.CapabilitySet {
	if limits.ContextWindow != 0 {
		caps.MaxInputTokens = limits.ContextWindow
	}
	if limits.MaxOutput != 0 {
		caps.MaxOutputTokens = limits.MaxOutput
	}
	return caps
}
