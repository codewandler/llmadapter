package conformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/llmadapter/providerregistry"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

// Options configures conformance report generation.
type Options struct {
	CompatibilityArtifactPath string
}

// Report joins static provider descriptors with endpoint and workload evidence.
type Report struct {
	Providers             []ProviderReport `json:"providers"`
	CompatibilityArtifact string           `json:"compatibility_artifact,omitempty"`
}

// HasFailures reports whether compatibility evidence violates a strict
// conformance contract. Missing optional evidence is surfaced as warnings only.
func (r Report) HasFailures() bool {
	for _, provider := range r.Providers {
		if len(provider.AgenticCoding.ContractViolations) > 0 {
			return true
		}
	}
	return false
}

// ProviderReport describes one registered provider endpoint type.
type ProviderReport struct {
	Type              string                `json:"type"`
	APIKind           adapt.ApiKind         `json:"api_kind"`
	Family            adapt.ApiFamily       `json:"family"`
	DefaultAPIKeyEnvs []string              `json:"default_api_key_envs,omitempty"`
	DefaultModelEnv   string                `json:"default_model_env,omitempty"`
	DefaultModel      string                `json:"default_model,omitempty"`
	Continuation      ContinuationReport    `json:"continuation,omitempty"`
	Transport         unified.TransportKind `json:"transport,omitempty"`
	Capabilities      CapabilityReport      `json:"capabilities"`
	Coverage          FeatureCoverage       `json:"coverage"`
	AgenticCoding     UseCaseCoverage       `json:"agentic_coding,omitempty"`
	Warnings          []string              `json:"warnings,omitempty"`
}

// ContinuationReport describes the public replay contract and internal provider strategy.
type ContinuationReport struct {
	Consumer unified.ContinuationMode `json:"consumer,omitempty"`
	Internal unified.ContinuationMode `json:"internal,omitempty"`
}

// CapabilityReport is the JSON-stable subset of router.CapabilitySet.
type CapabilityReport struct {
	Streaming       bool `json:"streaming"`
	Tools           bool `json:"tools"`
	ParallelTools   bool `json:"parallel_tools"`
	Vision          bool `json:"vision"`
	JSONMode        bool `json:"json_mode"`
	JSONSchema      bool `json:"json_schema"`
	Reasoning       bool `json:"reasoning"`
	ReasoningDeltas bool `json:"reasoning_deltas"`
	PromptCaching   bool `json:"prompt_caching"`
	MaxInputTokens  int  `json:"max_input_tokens,omitempty"`
	MaxOutputTokens int  `json:"max_output_tokens,omitempty"`
}

// FeatureCoverage records the current endpoint evidence level for major features.
type FeatureCoverage struct {
	Text                  string `json:"text,omitempty"`
	Tools                 string `json:"tools,omitempty"`
	ToolContinuation      string `json:"tool_continuation,omitempty"`
	ParallelTools         string `json:"parallel_tools,omitempty"`
	Reasoning             string `json:"reasoning,omitempty"`
	PromptCacheAccounting string `json:"prompt_cache_accounting,omitempty"`
	StructuredOutput      string `json:"structured_output,omitempty"`
	Vision                string `json:"vision,omitempty"`
	Usage                 string `json:"usage,omitempty"`
	Pricing               string `json:"pricing,omitempty"`
	Gateway               string `json:"gateway,omitempty"`
}

// UseCaseCoverage records rows from a workload-specific compatibility artifact.
type UseCaseCoverage struct {
	ArtifactPath       string                     `json:"artifact_path,omitempty"`
	UseCase            compatibility.UseCase      `json:"use_case,omitempty"`
	ResultDate         string                     `json:"result_date,omitempty"`
	ApprovedCount      int                        `json:"approved_count"`
	ValidApprovedCount int                        `json:"valid_approved_count,omitempty"`
	ContractStatus     string                     `json:"contract_status,omitempty"`
	ContractViolations []UseCaseContractViolation `json:"contract_violations,omitempty"`
	Rows               []UseCaseRow               `json:"rows,omitempty"`
}

// UseCaseRow is the compatibility artifact row subset shown per provider.
type UseCaseRow struct {
	Candidate        string                `json:"candidate,omitempty"`
	PublicModel      string                `json:"public_model,omitempty"`
	NativeModel      string                `json:"native_model,omitempty"`
	ProviderAPI      adapt.ApiKind         `json:"provider_api,omitempty"`
	Family           adapt.ApiFamily       `json:"family,omitempty"`
	Status           compatibility.Status  `json:"status"`
	DurationSeconds  float64               `json:"duration_seconds,omitempty"`
	Text             string                `json:"text,omitempty"`
	Tools            string                `json:"tools,omitempty"`
	ToolContinuation string                `json:"tool_continuation,omitempty"`
	StructuredOutput string                `json:"structured_output,omitempty"`
	Reasoning        string                `json:"reasoning,omitempty"`
	PromptCaching    string                `json:"prompt_caching,omitempty"`
	Usage            string                `json:"usage,omitempty"`
	CacheAccounting  string                `json:"cache_accounting,omitempty"`
	Continuation     ContinuationReport    `json:"continuation,omitempty"`
	Transport        unified.TransportKind `json:"transport,omitempty"`
}

// UseCaseContractViolation explains why an approved workload row is not
// acceptable as strict conformance evidence.
type UseCaseContractViolation struct {
	Candidate   string   `json:"candidate,omitempty"`
	PublicModel string   `json:"public_model,omitempty"`
	NativeModel string   `json:"native_model,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Reasons     []string `json:"reasons"`
}

// Build creates a provider conformance report from registry descriptors and optional compatibility evidence.
func Build(opts Options) (Report, error) {
	var artifact compatibility.Artifact
	if opts.CompatibilityArtifactPath != "" {
		loaded, err := compatibility.LoadArtifact(opts.CompatibilityArtifactPath)
		if err != nil {
			return Report{}, err
		}
		artifact = loaded
	}

	reports := make([]ProviderReport, 0, len(providerregistry.List()))
	for _, descriptor := range providerregistry.List() {
		coverage := coverageForProvider(descriptor.Type)
		useCase := useCaseCoverage(opts.CompatibilityArtifactPath, artifact, descriptor)
		report := ProviderReport{
			Type:              descriptor.Type,
			APIKind:           descriptor.APIKind,
			Family:            descriptor.Family,
			DefaultAPIKeyEnvs: append([]string(nil), descriptor.DefaultAPIKeyEnvs...),
			DefaultModelEnv:   descriptor.DefaultModelEnv,
			DefaultModel:      descriptor.DefaultModel,
			Continuation: ContinuationReport{
				Consumer: descriptor.ConsumerContinuation,
				Internal: descriptor.InternalContinuation,
			},
			Transport:     descriptor.Transport,
			Capabilities:  capabilityReport(descriptor.Capabilities),
			Coverage:      coverage,
			AgenticCoding: useCase,
		}
		report.Warnings = warningsForProvider(descriptor, coverage, useCase)
		reports = append(reports, report)
	}

	return Report{
		Providers:             reports,
		CompatibilityArtifact: opts.CompatibilityArtifactPath,
	}, nil
}

func capabilityReport(capabilities router.CapabilitySet) CapabilityReport {
	return CapabilityReport{
		Streaming:       capabilities.Streaming,
		Tools:           capabilities.Tools,
		ParallelTools:   capabilities.ParallelTools,
		Vision:          capabilities.Vision,
		JSONMode:        capabilities.JSONMode,
		JSONSchema:      capabilities.JSONSchema,
		Reasoning:       capabilities.Reasoning,
		ReasoningDeltas: capabilities.ReasoningDeltas,
		PromptCaching:   capabilities.PromptCaching,
		MaxInputTokens:  capabilities.MaxInputTokens,
		MaxOutputTokens: capabilities.MaxOutputTokens,
	}
}

func useCaseCoverage(path string, artifact compatibility.Artifact, descriptor providerregistry.Descriptor) UseCaseCoverage {
	out := UseCaseCoverage{
		ArtifactPath: path,
		UseCase:      artifact.UseCase,
		ResultDate:   artifact.ResultDate,
	}
	for _, row := range artifact.Rows {
		if row.Provider != descriptor.Type {
			continue
		}
		consumerContinuation := row.ConsumerContinuation
		if consumerContinuation == "" {
			consumerContinuation = descriptor.ConsumerContinuation
		}
		internalContinuation := row.InternalContinuation
		if internalContinuation == "" {
			internalContinuation = descriptor.InternalContinuation
		}
		transport := row.Transport
		if transport == "" {
			transport = descriptor.Transport
		}
		if row.Status == compatibility.StatusApproved {
			out.ApprovedCount++
			if artifact.UseCase != compatibility.UseCaseAgenticCoding {
				out.ValidApprovedCount++
			} else {
				reasons := agenticCodingContractViolations(row)
				if len(reasons) == 0 {
					out.ValidApprovedCount++
				} else {
					out.ContractViolations = append(out.ContractViolations, UseCaseContractViolation{
						Candidate:   row.Candidate,
						PublicModel: row.PublicModel,
						NativeModel: row.NativeModel,
						Provider:    row.Provider,
						Reasons:     reasons,
					})
				}
			}
		}
		out.Rows = append(out.Rows, UseCaseRow{
			Candidate:        row.Candidate,
			PublicModel:      row.PublicModel,
			NativeModel:      row.NativeModel,
			ProviderAPI:      row.ProviderAPI,
			Family:           row.Family,
			Status:           row.Status,
			DurationSeconds:  row.DurationSeconds,
			Text:             row.Text,
			Tools:            row.Tools,
			ToolContinuation: row.ToolContinuation,
			StructuredOutput: row.StructuredOutput,
			Reasoning:        row.Reasoning,
			PromptCaching:    row.PromptCaching,
			Usage:            row.Usage,
			CacheAccounting:  row.CacheAccounting,
			Continuation: ContinuationReport{
				Consumer: consumerContinuation,
				Internal: internalContinuation,
			},
			Transport: transport,
		})
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		if out.Rows[i].PublicModel == out.Rows[j].PublicModel {
			return out.Rows[i].Candidate < out.Rows[j].Candidate
		}
		return out.Rows[i].PublicModel < out.Rows[j].PublicModel
	})
	if artifact.UseCase == compatibility.UseCaseAgenticCoding && path != "" {
		switch {
		case len(out.ContractViolations) > 0:
			out.ContractStatus = "failed"
		case out.ApprovedCount == 0:
			out.ContractStatus = "no_approved"
		default:
			out.ContractStatus = "passed"
		}
	}
	return out
}

func warningsForProvider(descriptor providerregistry.Descriptor, coverage FeatureCoverage, useCase UseCaseCoverage) []string {
	var warnings []string
	if descriptor.Capabilities.PromptCaching && coverage.PromptCacheAccounting != "live" {
		warnings = append(warnings, fmt.Sprintf("prompt caching is advertised but cache accounting evidence is %q", coverage.PromptCacheAccounting))
	}
	if descriptor.Capabilities.Reasoning && coverage.Reasoning != "live" {
		warnings = append(warnings, fmt.Sprintf("reasoning is advertised but live reasoning evidence is %q", coverage.Reasoning))
	}
	if supportsAgenticPrimitives(descriptor.Capabilities) && useCase.ArtifactPath != "" && useCase.ApprovedCount == 0 {
		warnings = append(warnings, "no approved agentic-coding rows in compatibility artifact")
	}
	for _, violation := range useCase.ContractViolations {
		warnings = append(warnings, fmt.Sprintf("approved agentic-coding row %q violates conformance contract: %s", violation.Candidate, strings.Join(violation.Reasons, "; ")))
	}
	return warnings
}

func agenticCodingContractViolations(row compatibility.Row) []string {
	var reasons []string
	if row.RequiredStatus != "passed" {
		reasons = append(reasons, fmt.Sprintf("required_status=%q, want passed", row.RequiredStatus))
	}
	requiredLive := map[string]string{
		"text":              row.Text,
		"tools":             row.Tools,
		"tool_continuation": row.ToolContinuation,
		"structured_output": row.StructuredOutput,
		"reasoning":         row.Reasoning,
		"prompt_caching":    row.PromptCaching,
		"usage":             row.Usage,
		"cache_accounting":  row.CacheAccounting,
	}
	for _, name := range []string{"text", "tools", "tool_continuation", "structured_output", "reasoning", "prompt_caching", "usage", "cache_accounting"} {
		if requiredLive[name] != string(compatibility.EvidenceLive) {
			reasons = append(reasons, fmt.Sprintf("%s=%q, want live", name, requiredLive[name]))
		}
	}
	if row.ConsumerContinuation == "" {
		reasons = append(reasons, "consumer_continuation is empty")
	}
	if row.InternalContinuation == "" {
		reasons = append(reasons, "internal_continuation is empty")
	}
	if row.Transport == "" {
		reasons = append(reasons, "transport is empty")
	}
	return reasons
}

func supportsAgenticPrimitives(capabilities router.CapabilitySet) bool {
	return capabilities.Streaming && capabilities.Tools && capabilities.Reasoning && capabilities.PromptCaching
}

func coverageForProvider(providerType string) FeatureCoverage {
	coverage, ok := featureCoverageByProvider[providerType]
	if !ok {
		return FeatureCoverage{}
	}
	return coverage
}

var featureCoverageByProvider = map[string]FeatureCoverage{
	"anthropic": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "n/a", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"claude": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "n/a", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"openai_chat": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "live", Reasoning: "n/a", PromptCacheAccounting: "n/a", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"openai_responses": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "live", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"codex_responses": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "live", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"bedrock_responses": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "n/a", StructuredOutput: "fixture", Vision: "n/a", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"bedrock_messages": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "n/a", PromptCacheAccounting: "n/a", StructuredOutput: "n/a", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"bedrock_converse": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "n/a", StructuredOutput: "n/a", Vision: "n/a", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"openrouter_chat": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "live", Reasoning: "n/a", PromptCacheAccounting: "n/a", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"openrouter_responses": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "live", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"openrouter_messages": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "n/a", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"minimax_chat": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "n/a", PromptCacheAccounting: "n/a", StructuredOutput: "fixture", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
	"minimax_messages": {
		Text: "live", Tools: "live", ToolContinuation: "live", ParallelTools: "n/a", Reasoning: "live", PromptCacheAccounting: "live", StructuredOutput: "n/a", Vision: "fixture", Usage: "live", Pricing: "modeldb", Gateway: "live",
	},
}
