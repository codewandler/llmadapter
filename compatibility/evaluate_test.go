package compatibility

import (
	"testing"

	"github.com/codewandler/llmadapter/router"
)

func TestEvaluateApprovedWhenRequiredAndPreferredSupported(t *testing.T) {
	candidate := Candidate{
		Provider:         "claude",
		ProviderType:     "claude",
		ModelDBService:   "anthropic",
		CapabilitySource: "modeldb_exposure",
		Features: append(CandidateFeatures(router.CapabilitySet{
			Streaming:     true,
			Tools:         true,
			Reasoning:     true,
			PromptCaching: true,
		}, "modeldb_exposure", "anthropic"), FeatureEvidence{
			Feature:   FeatureCacheAccounting,
			Supported: true,
			Evidence:  EvidenceLive,
		}),
	}

	evaluation := Evaluate(candidate, AgenticCodingProfile())
	if evaluation.Status != StatusApproved {
		t.Fatalf("status = %s, want %s: %+v", evaluation.Status, StatusApproved, evaluation)
	}
}

func TestEvaluateFailsWhenRequiredUnsupported(t *testing.T) {
	candidate := Candidate{Features: CandidateFeatures(router.CapabilitySet{
		Streaming:     true,
		Tools:         true,
		PromptCaching: true,
	}, "provider_descriptor", "")}

	evaluation := Evaluate(candidate, AgenticCodingProfile())
	if evaluation.Status != StatusFailed {
		t.Fatalf("status = %s, want %s", evaluation.Status, StatusFailed)
	}
	if !containsFeature(evaluation.MissingRequired, FeatureReasoning) {
		t.Fatalf("missing required = %v, want reasoning", evaluation.MissingRequired)
	}
}

func TestEvaluateDegradesWhenPreferredUntested(t *testing.T) {
	candidate := Candidate{
		ModelDBService: "anthropic",
		Features: CandidateFeatures(router.CapabilitySet{
			Streaming:     true,
			Tools:         true,
			Reasoning:     true,
			PromptCaching: true,
		}, "provider_descriptor", "anthropic"),
	}

	evaluation := Evaluate(candidate, AgenticCodingProfile())
	if evaluation.Status != StatusDegraded {
		t.Fatalf("status = %s, want %s", evaluation.Status, StatusDegraded)
	}
	if !containsFeature(evaluation.DegradedPreferred, FeatureCacheAccounting) {
		t.Fatalf("degraded preferred = %v, want cache_accounting", evaluation.DegradedPreferred)
	}
}

func TestEvaluateUntestedRequired(t *testing.T) {
	evaluation := Evaluate(Candidate{}, Profile{
		UseCase: UseCaseAgenticCoding,
		Requirements: map[Feature]RequirementLevel{
			FeatureStreamingText: RequirementRequired,
		},
	})
	if evaluation.Status != StatusUntested {
		t.Fatalf("status = %s, want %s", evaluation.Status, StatusUntested)
	}
}

func containsFeature(features []Feature, want Feature) bool {
	for _, feature := range features {
		if feature == want {
			return true
		}
	}
	return false
}
