package compatibility

import (
	"strings"
	"testing"
)

func TestNewArtifactIncludesProfileRequirements(t *testing.T) {
	artifact, err := NewArtifact(UseCaseAgenticCoding, "go test", []Row{{Candidate: "x"}}, 1.25, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFeature(artifact.RequiredFeatures, FeatureCacheAccounting) {
		t.Fatalf("required features = %v, want cache_accounting", artifact.RequiredFeatures)
	}
	if !hasFeature(artifact.PreferredFeatures, FeaturePricing) {
		t.Fatalf("preferred features = %v, want pricing", artifact.PreferredFeatures)
	}
}

func TestRenderAndReplaceGeneratedSection(t *testing.T) {
	artifact := Artifact{
		Command:              "go test ./tests/e2e",
		TotalDurationSeconds: 2.5,
		Rows: []Row{{
			Candidate:       "openai_gpt",
			Provider:        "openai_responses",
			NativeModel:     "gpt",
			RequiredStatus:  "passed",
			CacheAccounting: "live",
			Status:          StatusApproved,
			DurationSeconds: 1.2,
		}},
	}
	generated := RenderArtifactMarkdown(artifact)
	if !strings.Contains(generated, "| `openai_gpt` | `openai_responses` | `gpt` | pass | live | approved | 1.20s |") {
		t.Fatalf("unexpected generated markdown:\n%s", generated)
	}
	doc := "before\n" + MatrixGeneratedStart + "\nold\n" + MatrixGeneratedEnd + "\nafter\n"
	updated, err := ReplaceGeneratedSection(doc, generated)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "old") || !strings.Contains(updated, "openai_gpt") {
		t.Fatalf("unexpected updated markdown:\n%s", updated)
	}
}

func hasFeature(features []Feature, want Feature) bool {
	for _, feature := range features {
		if feature == want {
			return true
		}
	}
	return false
}
