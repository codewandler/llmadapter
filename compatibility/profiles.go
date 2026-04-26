package compatibility

import "fmt"

func BuiltinProfile(useCase UseCase) (Profile, bool) {
	switch useCase {
	case UseCaseAgenticCoding:
		return AgenticCodingProfile(), true
	case UseCaseSummarization:
		return SummarizationProfile(), true
	default:
		return Profile{}, false
	}
}

func MustBuiltinProfile(useCase UseCase) Profile {
	profile, ok := BuiltinProfile(useCase)
	if !ok {
		panic(fmt.Sprintf("unknown compatibility use case %q", useCase))
	}
	return profile
}

func AgenticCodingProfile() Profile {
	return Profile{
		UseCase:     UseCaseAgenticCoding,
		Description: "Coding-agent runtime with tool use, continuation, reasoning, caching, structured output, and usage accounting.",
		Requirements: map[Feature]RequirementLevel{
			FeatureStreamingText:    RequirementRequired,
			FeatureTools:            RequirementRequired,
			FeatureToolContinuation: RequirementRequired,
			FeatureStructuredOutput: RequirementRequired,
			FeatureReasoning:        RequirementRequired,
			FeaturePromptCaching:    RequirementRequired,
			FeatureUsage:            RequirementRequired,
			FeatureCacheAccounting:  RequirementRequired,
			FeaturePricing:          RequirementPreferred,
			FeatureGateway:          RequirementOptional,
		},
	}
}

func SummarizationProfile() Profile {
	return Profile{
		UseCase:     UseCaseSummarization,
		Description: "Simple text generation or summarization where tools, reasoning, and prompt-cache behavior are not required.",
		Requirements: map[Feature]RequirementLevel{
			FeatureStreamingText:    RequirementRequired,
			FeatureUsage:            RequirementRequired,
			FeatureTools:            RequirementOptional,
			FeatureToolContinuation: RequirementOptional,
			FeatureStructuredOutput: RequirementOptional,
			FeatureReasoning:        RequirementOptional,
			FeaturePromptCaching:    RequirementOptional,
			FeatureCacheAccounting:  RequirementOptional,
			FeaturePricing:          RequirementOptional,
			FeatureGateway:          RequirementOptional,
		},
	}
}
