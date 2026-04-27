package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/providerregistry"
)

func TestBuildReportsEveryProviderDescriptor(t *testing.T) {
	path := writeTestArtifact(t)
	report, err := Build(Options{CompatibilityArtifactPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Providers) != len(providerregistry.List()) {
		t.Fatalf("providers = %d, want %d", len(report.Providers), len(providerregistry.List()))
	}

	byType := map[string]ProviderReport{}
	for _, provider := range report.Providers {
		byType[provider.Type] = provider
		if provider.Coverage.Text == "" {
			t.Fatalf("provider %q has no feature coverage", provider.Type)
		}
	}
	anthropic := byType["anthropic"]
	if anthropic.APIKind != "anthropic.messages" || !anthropic.Capabilities.Tools {
		t.Fatalf("unexpected anthropic report: %+v", anthropic)
	}
	if anthropic.AgenticCoding.ApprovedCount != 1 {
		t.Fatalf("approved count = %d, want 1", anthropic.AgenticCoding.ApprovedCount)
	}
	if anthropic.Coverage.PromptCacheAccounting != "live" {
		t.Fatalf("cache coverage = %q, want live", anthropic.Coverage.PromptCacheAccounting)
	}
}

func TestBuildWarnsWhenAgenticArtifactHasNoApprovedRows(t *testing.T) {
	path := writeEmptyTestArtifact(t)
	report, err := Build(Options{CompatibilityArtifactPath: path})
	if err != nil {
		t.Fatal(err)
	}
	var anthropic ProviderReport
	for _, provider := range report.Providers {
		if provider.Type == "anthropic" {
			anthropic = provider
			break
		}
	}
	if len(anthropic.Warnings) == 0 {
		t.Fatalf("expected warning for missing approved rows: %+v", anthropic)
	}
}

func TestBuildFlagsApprovedAgenticContractViolations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentic_coding.json")
	data := `{
		"use_case": "agentic_coding",
		"result_date": "2026-04-26",
		"rows": [
			{
				"candidate": "openai_gpt",
				"public_model": "gpt",
				"native_model": "gpt",
				"provider": "openai_responses",
				"provider_api": "openai.responses",
				"family": "openai.responses",
				"status": "approved",
				"text": "live"
			}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Build(Options{CompatibilityArtifactPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if !report.HasFailures() {
		t.Fatalf("expected strict conformance failure: %+v", report)
	}
	var openai ProviderReport
	for _, provider := range report.Providers {
		if provider.Type == "openai_responses" {
			openai = provider
			break
		}
	}
	if openai.AgenticCoding.ValidApprovedCount != 0 {
		t.Fatalf("valid approved count = %d, want 0", openai.AgenticCoding.ValidApprovedCount)
	}
	if openai.AgenticCoding.ContractStatus != "failed" || len(openai.AgenticCoding.ContractViolations) != 1 {
		t.Fatalf("unexpected contract result: %+v", openai.AgenticCoding)
	}
}

func writeTestArtifact(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agentic_coding.json")
	data := `{
		"use_case": "agentic_coding",
		"result_date": "2026-04-26",
		"rows": [
			{
				"candidate": "anthropic_haiku",
				"public_model": "haiku",
				"native_model": "claude-haiku-test",
				"provider": "anthropic",
				"provider_api": "anthropic.messages",
				"family": "anthropic.messages",
				"status": "approved",
				"required_status": "passed",
				"text": "live",
				"tools": "live",
				"tool_continuation": "live",
				"structured_output": "live",
				"reasoning": "live",
				"prompt_caching": "live",
				"usage": "live",
				"cache_accounting": "live",
				"consumer_continuation": "replay",
				"internal_continuation": "replay",
				"transport": "http_sse",
				"duration_seconds": 1.25
			}
		]
	}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEmptyTestArtifact(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agentic_coding.json")
	data := `{"use_case":"agentic_coding","result_date":"2026-04-26","rows":[]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
