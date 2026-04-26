package compatibility

import (
	"github.com/codewandler/llmadapter/router"
)

func CandidateFeatures(caps router.CapabilitySet, capabilitySource, modelDBService string) []FeatureEvidence {
	evidence := evidenceFromCapabilitySource(capabilitySource)
	return []FeatureEvidence{
		{
			Feature:   FeatureStreamingText,
			Supported: caps.Streaming,
			Evidence:  evidenceForSupport(caps.Streaming, evidence),
			Detail:    "provider endpoint streaming capability",
		},
		{
			Feature:   FeatureTools,
			Supported: caps.Tools,
			Evidence:  evidenceForSupport(caps.Tools, evidence),
			Detail:    "provider endpoint tool-use capability",
		},
		{
			Feature:   FeatureToolContinuation,
			Supported: caps.Tools,
			Evidence:  evidenceForSupport(caps.Tools, evidence),
			Detail:    "tool-result continuation uses the same endpoint tool capability",
		},
		{
			Feature:   FeatureStructuredOutput,
			Supported: caps.JSONSchema || caps.JSONMode || caps.Tools,
			Evidence:  evidenceForSupport(caps.JSONSchema || caps.JSONMode || caps.Tools, evidence),
			Detail:    "JSON mode/schema or tool schema can carry structured data",
		},
		{
			Feature:   FeatureReasoning,
			Supported: caps.Reasoning,
			Evidence:  evidenceForSupport(caps.Reasoning, evidence),
			Detail:    "provider endpoint reasoning/thinking capability",
		},
		{
			Feature:   FeaturePromptCaching,
			Supported: caps.PromptCaching,
			Evidence:  evidenceForSupport(caps.PromptCaching, evidence),
			Detail:    "request-side prompt cache controls can be encoded",
		},
		{
			Feature:   FeatureUsage,
			Supported: true,
			Evidence:  EvidenceMapped,
			Detail:    "canonical usage events are mapped when providers report usage",
		},
		{
			Feature:   FeatureCacheAccounting,
			Supported: false,
			Evidence:  EvidenceUntested,
			Detail:    "requires live provider cache write/read counter evidence",
		},
		{
			Feature:   FeaturePricing,
			Supported: modelDBService != "",
			Evidence:  evidenceForSupport(modelDBService != "", EvidenceModelDB),
			Detail:    "pricing requires modeldb service/offering metadata",
		},
		{
			Feature:   FeatureGateway,
			Supported: true,
			Evidence:  EvidenceFixture,
			Detail:    "gateway path exists for supported API families",
		},
	}
}

func evidenceFromCapabilitySource(source string) EvidenceLevel {
	switch source {
	case "modeldb_exposure":
		return EvidenceModelDB
	case "config_override":
		return EvidenceConfigOverride
	case "provider_descriptor", "":
		return EvidenceProviderDescriptor
	default:
		return EvidenceManual
	}
}

func evidenceForSupport(supported bool, evidence EvidenceLevel) EvidenceLevel {
	if supported {
		return evidence
	}
	return EvidenceUnsupported
}
