package compatibility

import (
	"sort"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
)

type UseCase string

const (
	UseCaseAgenticCoding UseCase = "agentic_coding"
	UseCaseSummarization UseCase = "summarization"
)

type Feature string

const (
	FeatureStreamingText    Feature = "streaming_text"
	FeatureTools            Feature = "tools"
	FeatureToolContinuation Feature = "tool_continuation"
	FeatureStructuredOutput Feature = "structured_output"
	FeatureReasoning        Feature = "reasoning"
	FeaturePromptCaching    Feature = "prompt_caching"
	FeatureCacheAccounting  Feature = "cache_accounting"
	FeatureUsage            Feature = "usage"
	FeaturePricing          Feature = "pricing"
	FeatureGateway          Feature = "gateway"
)

type RequirementLevel string

const (
	RequirementRequired    RequirementLevel = "required"
	RequirementPreferred   RequirementLevel = "preferred"
	RequirementOptional    RequirementLevel = "optional"
	RequirementNotRequired RequirementLevel = "not_required"
)

type EvidenceLevel string

const (
	EvidenceLive               EvidenceLevel = "live"
	EvidenceFixture            EvidenceLevel = "fixture"
	EvidenceMapped             EvidenceLevel = "mapped"
	EvidenceModelDB            EvidenceLevel = "modeldb"
	EvidenceProviderDescriptor EvidenceLevel = "provider_descriptor"
	EvidenceConfigOverride     EvidenceLevel = "config_override"
	EvidenceManual             EvidenceLevel = "manual"
	EvidenceUntested           EvidenceLevel = "untested"
	EvidenceUnsupported        EvidenceLevel = "unsupported"
)

type Status string

const (
	StatusApproved    Status = "approved"
	StatusDegraded    Status = "degraded"
	StatusFailed      Status = "failed"
	StatusUntested    Status = "untested"
	StatusUnavailable Status = "unavailable"
)

type Profile struct {
	UseCase      UseCase                      `json:"use_case"`
	Description  string                       `json:"description,omitempty"`
	Requirements map[Feature]RequirementLevel `json:"requirements"`
}

type FeatureEvidence struct {
	Feature   Feature       `json:"feature"`
	Supported bool          `json:"supported"`
	Evidence  EvidenceLevel `json:"evidence"`
	Detail    string        `json:"detail,omitempty"`
}

type Candidate struct {
	Input            string               `json:"input"`
	MatchedAs        string               `json:"matched_as,omitempty"`
	SourceAPI        adapt.ApiKind        `json:"source_api,omitempty"`
	PublicModel      string               `json:"public_model,omitempty"`
	NativeModel      string               `json:"native_model,omitempty"`
	Provider         string               `json:"provider"`
	ProviderType     string               `json:"provider_type"`
	ProviderAPI      adapt.ApiKind        `json:"provider_api"`
	Family           adapt.ApiFamily      `json:"family"`
	Weight           int                  `json:"weight,omitempty"`
	Priority         int                  `json:"priority,omitempty"`
	ModelDBService   string               `json:"modeldb_service_id,omitempty"`
	Capabilities     router.CapabilitySet `json:"capabilities"`
	CapabilitySource string               `json:"capability_source,omitempty"`
	Features         []FeatureEvidence    `json:"features,omitempty"`
}

type Evaluation struct {
	Candidate           Candidate       `json:"candidate"`
	UseCase             UseCase         `json:"use_case"`
	Status              Status          `json:"status"`
	Features            []FeatureResult `json:"features"`
	MissingRequired     []Feature       `json:"missing_required,omitempty"`
	UntestedRequired    []Feature       `json:"untested_required,omitempty"`
	DegradedPreferred   []Feature       `json:"degraded_preferred,omitempty"`
	UnsupportedOptional []Feature       `json:"unsupported_optional,omitempty"`
}

type FeatureResult struct {
	Feature     Feature          `json:"feature"`
	Requirement RequirementLevel `json:"requirement"`
	Supported   bool             `json:"supported"`
	Evidence    EvidenceLevel    `json:"evidence"`
	Detail      string           `json:"detail,omitempty"`
}

func (e Evaluation) Approved() bool {
	return e.Status == StatusApproved
}

func sortedFeatures(requirements map[Feature]RequirementLevel) []Feature {
	features := make([]Feature, 0, len(requirements))
	for feature := range requirements {
		features = append(features, feature)
	}
	sort.Slice(features, func(i, j int) bool { return features[i] < features[j] })
	return features
}
