package adapterconfig

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/modeldb"
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

func TestSelectModelForUseCaseUsesRuntimeViewAndEvidence(t *testing.T) {
	cfg := useCaseSelectionTestConfig(t)
	evidence := CompatibilityEvidence{
		UseCase: compatibility.UseCaseAgenticCoding,
		Rows: []CompatibilityRowEvidence{{
			PublicModel:      "haiku",
			NativeModel:      "claude-haiku-test",
			Provider:         "claude",
			ProviderAPI:      adapt.ApiAnthropicMessages,
			Status:           compatibility.StatusApproved,
			Text:             "live",
			Tools:            "live",
			ToolContinuation: "live",
			StructuredOutput: "live",
			Reasoning:        "live",
			PromptCaching:    "live",
			Usage:            "live",
			CacheAccounting:  "live",
		}},
	}

	selection, err := SelectModelForUseCase(cfg, "haiku", "", UseCaseSelectionOptions{
		UseCase:  compatibility.UseCaseAgenticCoding,
		Evidence: evidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if selection.Resolution.Provider != "claude" || selection.Resolution.NativeModel != "claude-haiku-test" {
		t.Fatalf("unexpected selection: %+v", selection)
	}
	if selection.RuntimeID != "llmadapter-anthropic-messages-claude" {
		t.Fatalf("runtime id = %q", selection.RuntimeID)
	}
	if selection.Item.Offering.WireModelID != "claude-haiku-test" {
		t.Fatalf("unexpected modeldb item: %+v", selection.Item)
	}
}

func TestSelectModelForUseCaseFailsClosedWithoutApprovedEvidence(t *testing.T) {
	cfg := useCaseSelectionTestConfig(t)
	evidence := CompatibilityEvidence{
		UseCase: compatibility.UseCaseAgenticCoding,
		Rows: []CompatibilityRowEvidence{{
			PublicModel: "haiku",
			NativeModel: "claude-haiku-test",
			Provider:    "anthropic",
			ProviderAPI: adapt.ApiAnthropicMessages,
			Status:      compatibility.StatusUntested,
		}},
	}

	if _, err := SelectModelForUseCase(cfg, "haiku", "", UseCaseSelectionOptions{
		UseCase:  compatibility.UseCaseAgenticCoding,
		Evidence: evidence,
	}); err == nil {
		t.Fatal("expected strict selection to reject missing approved evidence")
	}
}

func useCaseSelectionTestConfig(t *testing.T) Config {
	t.Helper()
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "anthropic", Family: "claude", Series: "haiku", Version: "test"}
	catalog.Services["anthropic"] = modeldb.Service{ID: "anthropic", Name: "Anthropic"}
	catalog.Models[key] = modeldb.ModelRecord{
		Key:     key,
		Name:    "Claude Haiku Test",
		Aliases: []string{"haiku"},
	}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "anthropic", WireModelID: "claude-haiku-test"}] = modeldb.Offering{
		ServiceID:   "anthropic",
		WireModelID: "claude-haiku-test",
		ModelKey:    key,
		Aliases:     []string{"haiku"},
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeAnthropicMessages,
			ExposedCapabilities: &modeldb.Capabilities{
				Streaming:        true,
				ToolUse:          true,
				StructuredOutput: true,
				Reasoning:        &modeldb.ReasoningCapability{Available: true},
				Caching:          &modeldb.CachingCapability{Available: true},
			},
		}},
	}
	path := t.TempDir() + "/catalog.json"
	if err := modeldb.SaveJSON(path, catalog); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ModelDB: ModelDBConfig{CatalogPath: path},
		Providers: []ProviderConfig{
			{Name: "claude", Type: "claude", ModelDBServiceID: "anthropic"},
			{Name: "anthropic", Type: "anthropic", APIKey: "test", ModelDBServiceID: "anthropic"},
		},
		Routes: []RouteConfig{
			{SourceAPI: adapt.ApiAnthropicMessages, Model: "haiku", Provider: "claude", ProviderAPI: adapt.ApiAnthropicMessages, ModelDBModel: "haiku", Weight: 100},
			{SourceAPI: adapt.ApiAnthropicMessages, Model: "haiku", Provider: "anthropic", ProviderAPI: adapt.ApiAnthropicMessages, ModelDBModel: "haiku", Weight: 100},
		},
	}
	ApplyDefaults(&cfg)
	return cfg
}
