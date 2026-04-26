package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/llmadapter/unified"
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
	for _, want := range []string{"Matched as:   public_model", "Provider API: openai.responses", "Native model: gpt-test", "Capability source: provider_descriptor", "Capabilities:"} {
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
		"[01] provider=claude type=claude source=anthropic.messages api=anthropic.messages",
		"Provider type: claude",
		"[02] provider=anthropic type=anthropic source=anthropic.messages api=anthropic.messages",
		"Provider type: anthropic",
		"[03] provider=openrouter type=openrouter_responses source=openai.responses api=openrouter.responses",
		"Provider type: openrouter_responses",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	claudeIdx := strings.Index(got, "Provider type: claude")
	anthropicIdx := strings.Index(got, "Provider type: anthropic")
	if claudeIdx == -1 || anthropicIdx == -1 {
		t.Fatalf("missing expected provider ranking in output:\n%s", got)
	}
	if claudeIdx >= anthropicIdx {
		t.Fatalf("candidate ranking order is wrong:\n%s", got)
	}
}

func TestResolveCommandAnnotatesUseCase(t *testing.T) {
	path := writeTestResolveCandidateConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "haiku", "--config", path, "--use-case", "agentic_coding"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Use case:     agentic_coding",
		"Status:       untested",
		"Provider type: claude",
		"Untested required: cache_accounting",
		"structured_output: requirement=required supported=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandApprovedOnlyUsesCompatibilityEvidence(t *testing.T) {
	configPath, evidencePath := writeTestApprovedUseCaseConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "haiku", "--config", configPath, "--use-case", "agentic_coding", "--approved-only", "--compatibility-evidence", evidencePath})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Runtime ID:   llmadapter-anthropic-messages-claude",
		"Status:       approved",
		"Provider:     claude",
		"Native model: claude-haiku-test",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Provider:     anthropic") {
		t.Fatalf("unexpected unapproved candidate in output:\n%s", got)
	}
}

func TestCompatibilityCommandWithConfig(t *testing.T) {
	path := writeTestResolveCandidateConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"compatibility", "--config", path, "--model", "haiku", "--use-case", "agentic_coding"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Matches: 3 candidates",
		"status=untested provider=claude type=claude",
		"provider=openrouter type=openrouter_responses",
		"prompt_caching: requirement=required supported=true",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestCompatibilityRecordCommandUpdatesMatrix(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "agentic_coding.json")
	matrixPath := filepath.Join(dir, "USE_CASE_MATRIX.md")
	artifact := `{
		"use_case":"agentic_coding",
		"result_date":"2026-04-26",
		"command":"old command",
		"total_duration_seconds":1.25,
		"required_features":["streaming_text","cache_accounting"],
		"preferred_features":["pricing"],
		"rows":[
			{"candidate":"demo","provider":"openai_responses","native_model":"gpt-demo","provider_api":"openai.responses","required_status":"passed","status":"approved","cache_accounting":"live","duration_seconds":1.2}
		]
	}`
	if err := os.WriteFile(artifactPath, []byte(artifact), 0o600); err != nil {
		t.Fatal(err)
	}
	matrix := "before\n" + compatibility.MatrixGeneratedStart + "\nold table\n" + compatibility.MatrixGeneratedEnd + "\nafter\n"
	if err := os.WriteFile(matrixPath, []byte(matrix), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"compatibility-record", "--artifact", artifactPath, "--matrix", matrixPath, "--command", "new command"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	for _, want := range []string{"new command", "`demo`", "`openai_responses`", "approved"} {
		if !strings.Contains(got, want) {
			t.Fatalf("matrix missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "old table") {
		t.Fatalf("matrix still contains old generated content:\n%s", got)
	}
}

func TestCompatibilityCommandJSON(t *testing.T) {
	path := writeTestResolveCandidateConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"compatibility", "--config", path, "--model", "haiku", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{`"use_case": "agentic_coding"`, `"status": "untested"`, `"provider_type": "claude"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveInferModelUsesBestCandidateWhenSourceAPIOmitted(t *testing.T) {
	cfg := readTestConfig(t, writeTestResolveCandidateConfig(t))
	resolution, err := resolveInferModel(cfg, "haiku", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Provider != "claude" || resolution.ProviderAPI != adapt.ApiAnthropicMessages || resolution.SourceAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected infer resolution: %+v", resolution)
	}
}

func TestResolveInferModelPrefersExactRouteOverDynamicFallback(t *testing.T) {
	cfg := adapterconfig.Config{
		Providers: []adapterconfig.ProviderConfig{
			{Name: "anthropic", Type: "anthropic", APIKey: "test"},
			{Name: "openai", Type: "openai_responses", APIKey: "test"},
		},
		Routes: []adapterconfig.RouteConfig{
			{SourceAPI: adapt.ApiAnthropicMessages, Provider: "anthropic", DynamicModels: true, Weight: 1},
			{SourceAPI: adapt.ApiOpenAIResponses, Model: "gpt-4.1-mini", Provider: "openai", Weight: 100},
		},
	}
	adapterconfig.ApplyDefaults(&cfg)
	resolution, err := resolveInferModel(cfg, "gpt-4.1-mini", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Provider != "openai" || resolution.SourceAPI != adapt.ApiOpenAIResponses || resolution.MatchedAs != "public_model" {
		t.Fatalf("unexpected infer resolution: %+v", resolution)
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

func TestResolveCommandWithModelDBDynamicRoute(t *testing.T) {
	path := writeTestModelDBDynamicConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "fast", "--config", path, "--source-api", "openai.responses"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"Matched as:   dynamic_model", "Native model: gpt-fast-wire", "ModelDB svc:  openai", "Capability source: modeldb_exposure"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestResolveCommandWithModelDBDynamicRouteRejectsMissingModel(t *testing.T) {
	path := writeTestModelDBDynamicConfig(t)
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"resolve", "missing-model", "--config", path, "--source-api", "openai.responses"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing dynamic model to be rejected")
	}
	if !strings.Contains(err.Error(), `no route found for model "missing-model"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveInferModelWithModelDBDynamicRoute(t *testing.T) {
	cfg := readTestConfig(t, writeTestModelDBDynamicConfig(t))
	resolution, err := resolveInferModel(cfg, "fast", adapt.ApiOpenAIResponses)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.MatchedAs != "dynamic_model" || resolution.NativeModel != "gpt-fast-wire" || resolution.Provider != "openai" {
		t.Fatalf("unexpected infer resolution: %+v", resolution)
	}
}

func TestInferCommandModelDefaultIsHaiku(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := newRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"infer", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "-m, --model string") || !strings.Contains(got, "(default \"haiku\")") {
		t.Fatalf("infer help does not show haiku model default:\n%s", got)
	}
}

func TestInferAutoIntentsUseRequestedModel(t *testing.T) {
	intents := inferAutoIntents("openai/gpt-5.5", adapt.ApiOpenAIResponses)
	if len(intents) != 1 {
		t.Fatalf("expected one intent, got %+v", intents)
	}
	if intents[0].Name != "openai/gpt-5.5" || intents[0].SourceAPI != adapt.ApiOpenAIResponses {
		t.Fatalf("unexpected intents: %+v", intents)
	}
	if got := inferAutoIntents("", ""); got != nil {
		t.Fatalf("expected nil intents, got %+v", got)
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
	for _, want := range []string{`"addr": ":9090"`, `"type": "openai_responses"`, `"source_api": "openai.responses"`, `"capability_source": "provider_descriptor"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, `"api_key"`) || strings.Contains(got, `"test",`) {
		t.Fatalf("inspect output leaked inline api key:\n%s", got)
	}
}

func TestRunInferRequestStreamsReasoningTextAndUsage(t *testing.T) {
	client := fakeInferClient{events: []unified.Event{
		unified.RouteEvent{ProviderName: "openai", NativeModel: "gpt-test"},
		unified.ReasoningDeltaEvent{Text: "thinking"},
		unified.TextDeltaEvent{Text: "answer"},
		unified.NewUsageEvent(unified.TokenItems{
			{Kind: unified.TokenKindInputNew, Count: 4},
			{Kind: unified.TokenKindOutputReasoning, Count: 2},
			{Kind: unified.TokenKindOutput, Count: 3},
		}, unified.CostItems{{Kind: unified.CostKindOutput, Amount: 0.0012}}),
	}}
	var out bytes.Buffer
	err := runInferRequest(context.Background(), &out, &client, "gpt-test", "hello", inferParams{maxTokens: 16, thinking: "on", timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{ansiDim + "thinking" + ansiReset + "answer", "── usage ──", "input.new: 4", "output.reasoning: 2", "output: 3", "cost: $0.001200"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
	if client.request.Model != "gpt-test" || client.request.MaxOutputTokens == nil || *client.request.MaxOutputTokens != 16 {
		t.Fatalf("unexpected request: %+v", client.request)
	}
	if client.request.CachePolicy != unified.CachePolicyOn {
		t.Fatalf("expected cache policy on by default, got %q", client.request.CachePolicy)
	}
	if client.request.Reasoning == nil || !client.request.Reasoning.Expose {
		t.Fatalf("expected reasoning request: %+v", client.request.Reasoning)
	}
}

func TestInferRequestNoCacheDisablesCachePolicy(t *testing.T) {
	req, err := inferRequest("gpt-test", "hello", inferParams{maxTokens: 16, noCache: true})
	if err != nil {
		t.Fatal(err)
	}
	if req.CachePolicy != unified.CachePolicyOff {
		t.Fatalf("expected cache policy off, got %q", req.CachePolicy)
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

func readTestConfig(t *testing.T, path string) adapterconfig.Config {
	t.Helper()
	cfg, err := adapterconfig.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

type fakeInferClient struct {
	request unified.Request
	events  []unified.Event
}

func (c *fakeInferClient) Request(_ context.Context, req unified.Request) (<-chan unified.Event, error) {
	c.request = req
	out := make(chan unified.Event, len(c.events))
	for _, ev := range c.events {
		out <- ev
	}
	close(out)
	return out, nil
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

func writeTestModelDBDynamicConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	catalog := modeldb.NewCatalog()
	key := modeldb.ModelKey{Creator: "openai", Family: "gpt", Version: "fast"}
	catalog.Services["openai"] = modeldb.Service{ID: "openai", Name: "OpenAI"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Name: "GPT Fast", Aliases: []string{"fast"}}
	catalog.Offerings[modeldb.OfferingRef{ServiceID: "openai", WireModelID: "gpt-fast-wire"}] = modeldb.Offering{
		ServiceID:   "openai",
		WireModelID: "gpt-fast-wire",
		ModelKey:    key,
		Exposures: []modeldb.OfferingExposure{{
			APIType: modeldb.APITypeOpenAIResponses,
			ExposedCapabilities: &modeldb.Capabilities{
				Streaming:        true,
				ToolUse:          true,
				StructuredOutput: true,
			},
		}},
	}
	if err := modeldb.SaveJSON(catalogPath, catalog); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "llmadapter.json")
	data := []byte(`{
		"modeldb":{"catalog_path":` + strconv.Quote(catalogPath) + `},
		"providers":[{"name":"openai","type":"openai_responses","api_key":"test","modeldb_service_id":"openai"}],
		"routes":[{"source_api":"openai.responses","provider":"openai","provider_api":"openai.responses","dynamic_models":true,"weight":1}]
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
			{"name":"claude","type":"claude"},
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

func writeTestApprovedUseCaseConfig(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	key := modeldb.ModelKey{Creator: "anthropic", Family: "claude", Series: "haiku", Version: "test"}
	catalog := modeldb.NewCatalog()
	catalog.Services["anthropic"] = modeldb.Service{ID: "anthropic", Name: "Anthropic"}
	catalog.Models[key] = modeldb.ModelRecord{Key: key, Name: "Claude Haiku Test", Aliases: []string{"haiku"}}
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
	if err := modeldb.SaveJSON(catalogPath, catalog); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "llmadapter.json")
	config := `{
		"modeldb":{"catalog_path":` + strconv.Quote(catalogPath) + `},
		"providers":[
			{"name":"claude","type":"claude","modeldb_service_id":"anthropic"},
			{"name":"anthropic","type":"anthropic","api_key":"test","modeldb_service_id":"anthropic"}
		],
		"routes":[
			{"source_api":"anthropic.messages","model":"haiku","provider":"claude","provider_api":"anthropic.messages","modeldb_model":"haiku","weight":100},
			{"source_api":"anthropic.messages","model":"haiku","provider":"anthropic","provider_api":"anthropic.messages","modeldb_model":"haiku","weight":100}
		]
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(dir, "agentic_coding.json")
	evidence := `{
		"use_case":"agentic_coding",
		"rows":[
			{"public_model":"haiku","native_model":"claude-haiku-test","provider":"claude","provider_api":"anthropic.messages","status":"approved","text":"live","tools":"live","tool_continuation":"live","structured_output":"live","reasoning":"live","prompt_caching":"live","usage":"live","cache_accounting":"live"}
		]
	}`
	if err := os.WriteFile(evidencePath, []byte(evidence), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, evidencePath
}
