package adapterconfig

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
)

func TestCompatibilityCandidatesUseModelResolution(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:             "anthropic",
			Type:             "anthropic",
			APIKey:           "test",
			ModelDBServiceID: "anthropic",
		}},
		Routes: []RouteConfig{{
			SourceAPI: adapt.ApiAnthropicMessages,
			Model:     "haiku",
			Provider:  "anthropic",
			Weight:    100,
		}},
	}
	ApplyDefaults(&cfg)

	candidates, err := CompatibilityCandidates(cfg, "haiku", adapt.ApiAnthropicMessages)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %d, want 1", len(candidates))
	}
	candidate := candidates[0]
	if candidate.Provider != "anthropic" || candidate.ProviderType != "anthropic" || candidate.ProviderAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected candidate: %+v", candidate)
	}
	evaluation := compatibility.Evaluate(candidate, compatibility.AgenticCodingProfile())
	if evaluation.Status != compatibility.StatusUntested {
		t.Fatalf("status = %s, want untested: %+v", evaluation.Status, evaluation)
	}
	if !containsFeature(evaluation.UntestedRequired, compatibility.FeatureCacheAccounting) {
		t.Fatalf("untested required = %v, want cache_accounting", evaluation.UntestedRequired)
	}
}

func TestCompatibleCandidatesExcludeUntestedRequired(t *testing.T) {
	cfg := Config{
		Providers: []ProviderConfig{{
			Name:             "anthropic",
			Type:             "anthropic",
			APIKey:           "test",
			ModelDBServiceID: "anthropic",
		}},
		Routes: []RouteConfig{{
			SourceAPI: adapt.ApiAnthropicMessages,
			Model:     "haiku",
			Provider:  "anthropic",
			Weight:    100,
		}},
	}
	ApplyDefaults(&cfg)

	approvedOnly, err := CompatibleCandidates(cfg, "haiku", adapt.ApiAnthropicMessages, compatibility.UseCaseAgenticCoding, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(approvedOnly) != 0 {
		t.Fatalf("approvedOnly = %d, want 0 because cache accounting is still untested", len(approvedOnly))
	}

	withDegraded, err := CompatibleCandidates(cfg, "haiku", adapt.ApiAnthropicMessages, compatibility.UseCaseAgenticCoding, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withDegraded) != 0 {
		t.Fatalf("withDegraded = %d, want 0 because required cache accounting is untested", len(withDegraded))
	}
}

func containsFeature(features []compatibility.Feature, want compatibility.Feature) bool {
	for _, feature := range features {
		if feature == want {
			return true
		}
	}
	return false
}
