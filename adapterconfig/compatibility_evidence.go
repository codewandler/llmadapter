package adapterconfig

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/llmadapter/unified"
)

type CompatibilityEvidence struct {
	UseCase compatibility.UseCase      `json:"use_case"`
	Rows    []CompatibilityRowEvidence `json:"rows"`
}

type CompatibilityRowEvidence struct {
	Candidate            string                   `json:"candidate,omitempty"`
	PublicModel          string                   `json:"public_model,omitempty"`
	NativeModel          string                   `json:"native_model,omitempty"`
	Provider             string                   `json:"provider"`
	ProviderAPI          adapt.ApiKind            `json:"provider_api"`
	Family               adapt.ApiFamily          `json:"family,omitempty"`
	Status               compatibility.Status     `json:"status"`
	Text                 string                   `json:"text,omitempty"`
	Tools                string                   `json:"tools,omitempty"`
	ToolContinuation     string                   `json:"tool_continuation,omitempty"`
	StructuredOutput     string                   `json:"structured_output,omitempty"`
	Reasoning            string                   `json:"reasoning,omitempty"`
	PromptCaching        string                   `json:"prompt_caching,omitempty"`
	Usage                string                   `json:"usage,omitempty"`
	CacheAccounting      string                   `json:"cache_accounting,omitempty"`
	ConsumerContinuation unified.ContinuationMode `json:"consumer_continuation,omitempty"`
	InternalContinuation unified.ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            unified.TransportKind    `json:"transport,omitempty"`
}

func LoadCompatibilityEvidence(path string) (CompatibilityEvidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CompatibilityEvidence{}, fmt.Errorf("load compatibility evidence %q: %w", path, err)
	}
	var evidence CompatibilityEvidence
	if err := json.Unmarshal(data, &evidence); err != nil {
		return CompatibilityEvidence{}, fmt.Errorf("decode compatibility evidence %q: %w", path, err)
	}
	return evidence, nil
}

func DefaultCompatibilityEvidencePath(useCase compatibility.UseCase) string {
	switch useCase {
	case "", compatibility.UseCaseAgenticCoding:
		return "docs/compatibility/agentic_coding.json"
	default:
		return "docs/compatibility/" + string(useCase) + ".json"
	}
}
