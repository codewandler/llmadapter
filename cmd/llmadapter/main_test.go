package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/codewandler/modeldb"
)

func TestProvidersJSONCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"providers", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"type": "openai_responses"`) {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}

func TestProvidersStatusCommandWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"providers", "--status", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"NAME", "openai", "openai_responses", "inline_api_key"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"api_key"`) || strings.Contains(got, " test ") {
		t.Fatalf("provider status leaked api key:\n%s", got)
	}
}

func TestProvidersAutoCommandReportsSkippedWithoutCredentials(t *testing.T) {
	clearProviderStatusEnv(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"providers", "--auto"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "skipped") || !strings.Contains(got, "missing env credentials") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestRoutesCommandWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"routes", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"SOURCE_API", "public-fast", "openai", "gpt-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func clearProviderStatusEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_CONFIG_DIR",
		"CODEX_ACCESS_TOKEN",
		"CODEX_CODE_OAUTH_TOKEN",
		"CODEX_AUTH_PATH",
		"MINIMAX_API_KEY",
		"MINIMAX_KEY",
		"OPENAI_API_KEY",
		"OPENAI_KEY",
		"OPENROUTER_API_KEY",
		"OPENROUTER_KEY",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_AUTH_PATH", filepath.Join(t.TempDir(), "missing-auth.json"))
}

func TestModelsCommandWithConfigAndQuery(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"models", "--config", path, "--query", "public"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "public-fast") || !strings.Contains(got, "gpt-test") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestModelsCommandCatalogWithConfig(t *testing.T) {
	path := writeTestCatalogConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"models", "--catalog", "--config", path, "--service", "openai", "--api-type", "openai-responses", "--query", "gpt"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"MODEL", "openai/gpt/test", "openai", "openai-responses", "gpt-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "public-fast", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Matched as:   public_model", "Provider API: openai.responses", "Native model: gpt-test", "Capabilities:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandWithModelDBAliasConfig(t *testing.T) {
	path := writeTestAliasConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "haiku", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Matched as:   public_model", "Public model: haiku", "Native model: gpt-haiku-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandJSONWithConfig(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "gpt-test", "--config", path, "--source-api", "openai.responses", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{`"matched_as": "native_model"`, `"provider_type": "openai_responses"`, `"family": "openai.responses"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandShowsRankedCandidates(t *testing.T) {
	path := writeTestResolveCandidateConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "haiku", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Matches: 3 candidates",
		"[01] provider=claude type=claude_messages source=anthropic.messages api=anthropic.messages",
		"Provider type: claude_messages",
		"[02] provider=anthropic type=anthropic source=anthropic.messages api=anthropic.messages",
		"Provider type: anthropic",
		"[03] provider=openrouter type=openrouter_responses source=openrouter.responses api=openai.responses",
		"Provider type: openrouter_responses",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	claudeIdx := strings.Index(got, "Provider type: claude_messages")
	anthropicIdx := strings.Index(got, "Provider type: anthropic")
	if claudeIdx == -1 || anthropicIdx == -1 {
		t.Fatalf("missing expected provider ranking in output:\n%s", got)
	}
	if claudeIdx >= anthropicIdx {
		t.Fatalf("candidate ranking order is wrong:\n%s", got)
	}
}

func TestResolveCommandWithDynamicRoute(t *testing.T) {
	path := writeTestDynamicConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "gpt-new", "--config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Matched as:   dynamic_model", "Native model: gpt-new", "Provider API: openai.responses"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestServeInspectConfigCommand(t *testing.T) {
	path := writeTestConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"serve", "--config", path, "--addr", ":9090", "--inspect-config"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{`"addr": ":9090"`, `"type": "openai_responses"`, `"source_api": "openai.responses"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"api_key"`) || strings.Contains(got, `"test",`) {
		t.Fatalf("inspect output leaked inline api key:\n%s", got)
	}
}

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llmadapter.json")
	data := []byte(`{
		"providers":[{"name":"openai","type":"openai_responses","api_key":"test","model":"gpt-test"}],
		"routes":[{"source_api":"openai.responses","model":"public-fast","provider":"openai","native_model":"gpt-test","weight":100}]
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestCatalogConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "test"}
	catalog.Services["openai"] = modeldb.Service{ID: "openai", Name: "OpenAI"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Name: "GPT Test", Aliases: []string{"gpt-test"}}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-test"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-test",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
		}},
	}
	if err := modeldb.SaveJSON(catalogPath, catalog); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "llmadapter.json")
	data := []byte(`{"modeldb":{"catalog_path":` + strconv.Quote(catalogPath) + `}}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func writeTestAliasConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "haiku"}
	catalog.Services["openai"] = modeldb.Service{ID: "openai", Name: "OpenAI"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Name: "GPT Haiku Test"}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-haiku-test"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-haiku-test",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
		}},
	}
	if err := modeldb.SaveJSON(catalogPath, catalog); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "llmadapter.json")
	data := []byte(`{
		"modeldb":{
			"catalog_path":` + strconv.Quote(catalogPath) + `,
			"aliases":[{"name":"haiku","service_id":"openai","wire_model_id":"gpt-haiku-test"}]
		},
		"providers":[{"name":"openai","type":"openai_responses","api_key":"test","modeldb_service_id":"openai","model":"provider-default"}],
		"routes":[{"source_api":"openai.responses","model":"haiku","provider":"openai","modeldb_model":"haiku","weight":100}]
	}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func writeTestDynamicConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llmadapter.json")
	data := []byte(`{
		"providers":[{"name":"openai","type":"openai_responses","api_key":"test","model":"gpt-test"}],
		"routes":[{"source_api":"openai.responses","provider":"openai","dynamic_models":true,"weight":1}]
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestResolveCandidateConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "llmadapter.json")
	data := []byte(`{
		"providers":[
			{"name":"openai","type":"openai_responses","api_key":"test","model":"provider-default"},
			{"name":"anthropic","type":"anthropic","api_key":"test","modeldb_service_id":"anthropic","model":"anthropic/claude-haiku-4-5-20251001"},
			{"name":"claude","type":"claude_messages"},
			{"name":"openrouter","type":"openrouter_responses","api_key":"test","modeldb_service_id":"openrouter","model":"openai/gpt-haiku-test"}
		],
		"routes":[
			{"source_api":"anthropic.messages","model":"haiku","provider":"anthropic","weight":100},
			{"source_api":"anthropic.messages","model":"haiku","provider":"claude","weight":100},
			{"source_api":"openai.responses","model":"haiku","provider":"openrouter","weight":100}
		]
	}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
