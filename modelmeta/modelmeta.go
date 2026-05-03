package modelmeta

import (
	"context"
	"sync"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
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
	case adapt.FamilyBedrockConverse:
		return modeldb.APITypeBedrockConverse, true
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
	var modelCaps *modeldb.Capabilities
	if model, ok := catalog.Models[offering.ModelKey]; ok {
		modelCaps = &model.Capabilities
	}
	if exposure.ExposedCapabilities != nil {
		caps := effectiveExposureCapabilities(*exposure.ExposedCapabilities, modelCaps)
		out = applyExposureCapabilities(out, caps, *exposure)
	}
	if model, ok := catalog.Models[offering.ModelKey]; ok {
		out = applyLimits(out, model.Limits)
	}
	if offering.LimitsOverride != nil {
		out = applyLimits(out, *offering.LimitsOverride)
	}
	return out, true
}

func effectiveExposureCapabilities(exposureCaps modeldb.Capabilities, modelCaps *modeldb.Capabilities) modeldb.Capabilities {
	if modelCaps == nil || !isCacheOnlyExposure(exposureCaps) {
		return exposureCaps
	}
	merged := *modelCaps
	merged.Caching = exposureCaps.Caching
	return merged
}

func isCacheOnlyExposure(caps modeldb.Capabilities) bool {
	return caps.Caching != nil &&
		caps.Reasoning == nil &&
		!caps.ToolUse &&
		!caps.ParallelToolCalls &&
		!caps.StructuredOutput &&
		!caps.StructuredOutputs &&
		!caps.Vision &&
		!caps.Streaming &&
		!caps.Temperature &&
		!caps.Logprobs &&
		!caps.Seed &&
		!caps.WebSearch
}

func ResolvedMetadata(catalog modeldb.Catalog, serviceID, wireModelID string, family adapt.ApiFamily) (unified.ResolvedModelMetadata, bool) {
	apiType, ok := APITypeForFamily(family)
	if !ok {
		return unified.ResolvedModelMetadata{}, false
	}
	return ResolvedMetadataForAPIType(catalog, serviceID, wireModelID, apiType)
}

func ResolvedMetadataForAPIType(catalog modeldb.Catalog, serviceID, wireModelID string, apiType modeldb.APIType) (unified.ResolvedModelMetadata, bool) {
	offering, ok := catalog.Offerings[modeldb.OfferingRef{ServiceID: serviceID, WireModelID: wireModelID}]
	if !ok {
		return unified.ResolvedModelMetadata{}, false
	}
	exposure := offering.Exposure(apiType)
	if exposure == nil {
		return unified.ResolvedModelMetadata{}, false
	}
	out := unified.ResolvedModelMetadata{
		ServiceID:              serviceID,
		WireModelID:            wireModelID,
		APIType:                string(apiType),
		ParameterValues:        copyParameterValues(exposure.ParameterValues),
		ParameterMappings:      copyParameterMappings(exposure.ParameterMappings),
		ParameterValueMappings: copyParameterValueMappings(exposure.ParameterValueMappings),
	}
	if exposure.ExposedCapabilities != nil && exposure.ExposedCapabilities.Reasoning != nil {
		reasoning := exposure.ExposedCapabilities.Reasoning
		out.DefaultDisplayMode = reasoning.DefaultDisplay
		for _, mode := range reasoning.Modes {
			out.ReasoningModes = append(out.ReasoningModes, string(mode))
		}
		for _, effort := range reasoning.Efforts {
			out.ReasoningEfforts = append(out.ReasoningEfforts, string(effort))
		}
	} else if model, ok := catalog.Models[offering.ModelKey]; ok && model.Capabilities.Reasoning != nil {
		reasoning := model.Capabilities.Reasoning
		out.DefaultDisplayMode = reasoning.DefaultDisplay
		for _, mode := range reasoning.Modes {
			out.ReasoningModes = append(out.ReasoningModes, string(mode))
		}
		for _, effort := range reasoning.Efforts {
			out.ReasoningEfforts = append(out.ReasoningEfforts, string(effort))
		}
	}
	return out, true
}

func RequestMetadataProcessor(catalog modeldb.Catalog, serviceID string, family adapt.ApiFamily) adapt.RequestProcessor {
	return requestProcessorFunc(func(_ context.Context, req *adapt.Request) error {
		return AttachResolvedMetadata(&req.Unified, catalog, serviceID, family)
	})
}

func BuiltInRequestMetadataProcessor(serviceID string, family adapt.ApiFamily) adapt.RequestProcessor {
	var (
		once    sync.Once
		catalog modeldb.Catalog
		err     error
	)
	return requestProcessorFunc(func(_ context.Context, req *adapt.Request) error {
		once.Do(func() {
			catalog, err = modeldb.LoadBuiltIn()
		})
		if err != nil {
			return err
		}
		return AttachResolvedMetadata(&req.Unified, catalog, serviceID, family)
	})
}

func AttachResolvedMetadata(req *unified.Request, catalog modeldb.Catalog, serviceID string, family adapt.ApiFamily) error {
	if req == nil || req.Model == "" {
		return nil
	}
	if _, ok, err := unified.ResolvedModelMetadataFrom(req.Extensions); ok || err != nil {
		return err
	}
	meta, ok := ResolvedMetadata(catalog, serviceID, req.Model, family)
	if !ok {
		return nil
	}
	return unified.SetResolvedModelMetadata(&req.Extensions, meta)
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
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

func copyParameterValues(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}

func copyParameterMappings(mappings []modeldb.ParameterMapping) map[string]string {
	if len(mappings) == 0 {
		return nil
	}
	out := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping.Normalized == "" || mapping.WireName == "" {
			continue
		}
		out[string(mapping.Normalized)] = mapping.WireName
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func copyParameterValueMappings(mappings []modeldb.ParameterValueMapping) map[string]map[string]string {
	if len(mappings) == 0 {
		return nil
	}
	out := make(map[string]map[string]string)
	for _, mapping := range mappings {
		if mapping.Parameter == "" || mapping.Canonical == "" || mapping.WireValue == "" {
			continue
		}
		parameter := string(mapping.Parameter)
		if out[parameter] == nil {
			out[parameter] = make(map[string]string)
		}
		out[parameter][mapping.Canonical] = mapping.WireValue
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
