package adapterconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

func TestAutoMuxClientDetectsOpenAIEnv(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{EnableEnv: true, EnableLocalClaude: false, UseModelDB: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Client == nil {
		t.Fatal("expected client")
	}
	if !autoEnabled(result.Enabled, "openai_chat") || !autoEnabled(result.Enabled, "openai_responses") {
		t.Fatalf("expected OpenAI providers enabled: %+v", result.Enabled)
	}
	responsesRoute, ok := autoRoute(result.Config, adapt.ApiOpenAIResponses)
	if !ok {
		t.Fatalf("expected OpenAI Responses route: %+v", result.Config.Routes)
	}
	if responsesRoute.Provider != "openai_responses" || responsesRoute.ProviderAPI != adapt.ApiOpenAIResponses {
		t.Fatalf("unexpected Responses route: %+v", responsesRoute)
	}
	provider, ok := findProvider(result.Config, "openai_responses")
	if !ok {
		t.Fatalf("missing provider config: %+v", result.Config.Providers)
	}
	if provider.APIKeyEnv != "OPENAI_KEY" || provider.ModelDBServiceID != "openai" {
		t.Fatalf("unexpected provider config: %+v", provider)
	}
}

func TestAutoMuxClientDetectsLocalClaudeOAuth(t *testing.T) {
	clearAutoEnv(t)
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{EnableEnv: false, EnableLocalClaude: true})
	if err != nil {
		t.Fatal(err)
	}
	if !autoEnabled(result.Enabled, "claude") {
		t.Fatalf("expected claude enabled: %+v", result.Enabled)
	}
	route, ok := autoRoute(result.Config, adapt.ApiAnthropicMessages)
	if !ok {
		t.Fatalf("expected Anthropic Messages route: %+v", result.Config.Routes)
	}
	if route.Provider != "claude" {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestAutoMuxClientDetectsCodexEnv(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("CODEX_ACCESS_TOKEN", "test-codex-token")

	result, err := AutoMuxClient(AutoOptions{EnableEnv: true, UseModelDB: true})
	if err != nil {
		t.Fatal(err)
	}
	if !autoEnabled(result.Enabled, "codex_responses") {
		t.Fatalf("expected codex_responses enabled: %+v", result.Enabled)
	}
	route, ok := autoRoute(result.Config, adapt.ApiOpenAIResponses)
	if !ok {
		t.Fatalf("expected Responses route: %+v", result.Config.Routes)
	}
	if route.Provider != "codex_responses" || route.ProviderAPI != adapt.ApiCodexResponses {
		t.Fatalf("unexpected route: %+v", route)
	}
	provider, ok := findProvider(result.Config, "codex_responses")
	if !ok {
		t.Fatalf("missing provider config: %+v", result.Config.Providers)
	}
	if provider.APIKeyEnv != "CODEX_ACCESS_TOKEN" || provider.ModelDBServiceID != "codex" {
		t.Fatalf("unexpected provider config: %+v", provider)
	}
}

func TestAutoMuxClientDetectsLocalCodexOAuth(t *testing.T) {
	clearAutoEnv(t)
	path := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("CODEX_AUTH_PATH", path)
	if err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"test-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{EnableEnv: false, EnableLocalCodex: true})
	if err != nil {
		t.Fatal(err)
	}
	if !autoEnabled(result.Enabled, "codex_responses") {
		t.Fatalf("expected codex_responses enabled: %+v", result.Enabled)
	}
}

func TestAutoMuxClientIntentsCreatePublicRoutes(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: false,
		Intents: []AutoIntent{{
			Name:      "fast",
			SourceAPI: adapt.ApiOpenAIResponses,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Config.Routes) != 1 {
		t.Fatalf("expected one route, got %+v", result.Config.Routes)
	}
	if result.Config.Routes[0].Model != "fast" || result.Config.Routes[0].NativeModel == "" {
		t.Fatalf("unexpected intent route: %+v", result.Config.Routes[0])
	}
}

func TestAutoMuxClientModelDBIntentPrefersMatchingProvider(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		UseModelDB:        true,
		Intents:           []AutoIntent{{Name: "opus", SourceAPI: adapt.ApiOpenAIResponses}},
	})
	if err != nil {
		t.Fatal(err)
	}
	route, ok := autoRouteModel(result.Config, adapt.ApiOpenAIResponses, "opus")
	if !ok {
		t.Fatalf("missing opus route: %+v", result.Config.Routes)
	}
	if route.Model != "opus" || route.Provider != "claude" || route.ModelDBModel != "opus" || route.NativeModel != "" {
		t.Fatalf("unexpected modeldb intent route: %+v", route)
	}
	if route.ProviderAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("provider api = %q", route.ProviderAPI)
	}
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified: unified.Request{
			Model:     "opus",
			Stream:    true,
			Reasoning: &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ProviderName != "claude" || selected.NativeModel == "opus" || selected.NativeModel == "" || !selected.Capabilities.Reasoning {
		t.Fatalf("unexpected selected route: %+v", selected)
	}
}

func TestAutoMuxClientModelDBIntentPrefersOwnerProviderOverBroker(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		UseModelDB:        true,
		Intents:           []AutoIntent{{Name: "haiku", SourceAPI: adapt.ApiOpenAIResponses}},
	})
	if err != nil {
		t.Fatal(err)
	}
	route, ok := autoRouteModel(result.Config, adapt.ApiOpenAIResponses, "haiku")
	if !ok {
		t.Fatalf("missing haiku route: %+v", result.Config.Routes)
	}
	if route.Provider != "claude" || route.ProviderAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("expected owner provider route before broker route: %+v", route)
	}
}

func TestAutoMuxClientAutoSourcePrefersAnthropicRouteForAnthropicAlias(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		UseModelDB:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := r.Route(context.Background(), adapt.Request{
		Unified: unified.Request{Model: "haiku", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SourceAPI != adapt.ApiAnthropicMessages || selected.ProviderName != "claude" || selected.TargetAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected auto-source route: %+v", selected)
	}
}

func TestAutoMuxClientAutoSourceOpenRouterAliasUsesMessagesEndpoint(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: false,
		UseModelDB:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := r.Route(context.Background(), adapt.Request{
		Unified: unified.Request{Model: "haiku", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.SourceAPI != adapt.ApiAnthropicMessages || selected.ProviderName != "openrouter_messages" || selected.TargetAPI != adapt.ApiOpenRouterAnthropicMessages {
		t.Fatalf("unexpected auto-source OpenRouter route: %+v", selected)
	}
}

func TestAutoMuxClientCanAddDynamicRoutes(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:     true,
		DynamicModels: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, route := range result.Config.Routes {
		if route.SourceAPI == adapt.ApiOpenAIResponses && route.Provider == "openai_responses" && route.DynamicModels {
			found = true
			if route.Model != "" || route.NativeModel != "" || route.Weight != 1 {
				t.Fatalf("unexpected dynamic route: %+v", route)
			}
		}
	}
	if !found {
		t.Fatalf("missing dynamic route: %+v", result.Config.Routes)
	}
}

func TestAutoMuxClientCanAddDynamicRoutesWithIntents(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:     true,
		DynamicModels: true,
		Intents:       []AutoIntent{{Name: "miniagent-default", SourceAPI: adapt.ApiOpenAIResponses}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var foundIntent bool
	var foundDynamic bool
	for _, route := range result.Config.Routes {
		if route.SourceAPI != adapt.ApiOpenAIResponses || route.Provider != "openai_responses" {
			continue
		}
		if route.Model == "miniagent-default" && !route.DynamicModels {
			foundIntent = true
		}
		if route.DynamicModels && route.Model == "" && route.NativeModel == "" {
			foundDynamic = true
		}
	}
	if !foundIntent || !foundDynamic {
		t.Fatalf("expected intent and dynamic routes, foundIntent=%v foundDynamic=%v routes=%+v", foundIntent, foundDynamic, result.Config.Routes)
	}
}

func TestAutoMuxClientAddsDefaultModelAliasRoutes(t *testing.T) {
	clearAutoEnv(t)
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         false,
		EnableLocalClaude: true,
		UseModelDB:        true,
		SourceAPI:         adapt.ApiOpenAIResponses,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		alias string
		wire  string
	}{
		{"haiku", "claude-haiku-4-5-20251001"},
		{"sonnet", "claude-sonnet-4-6"},
		{"opus", "claude-opus-4-6"},
	} {
		t.Run(tt.alias, func(t *testing.T) {
			r, err := BuildRouter(result.Config)
			if err != nil {
				t.Fatal(err)
			}
			selected, err := r.Route(context.Background(), adapt.Request{
				SourceAPI: adapt.ApiOpenAIResponses,
				Unified:   unified.Request{Model: tt.alias, Stream: true},
			})
			if err != nil {
				t.Fatal(err)
			}
			if selected.ProviderName != "claude" || selected.NativeModel != tt.wire {
				t.Fatalf("unexpected alias route: %+v", selected)
			}
		})
	}
}

func TestAutoMuxClientAcceptsExternalModelDBAliases(t *testing.T) {
	clearAutoEnv(t)
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.WriteFile(filepath.Join(claudeDir, ".credentials.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         false,
		EnableLocalClaude: true,
		UseModelDB:        true,
		SourceAPI:         adapt.ApiOpenAIResponses,
		ModelDBAliases: []ModelDBAliasConfig{{
			Name:        "flagship",
			ServiceID:   "anthropic",
			WireModelID: "claude-opus-4-6",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "flagship", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.NativeModel != "claude-opus-4-6" {
		t.Fatalf("unexpected external alias route: %+v", selected)
	}
}

func TestModelDBAliasOverridesCatalogAlias(t *testing.T) {
	catalog, err := LoadModelDBCatalog(ModelDBConfig{})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := resolveModelDBItem(catalog, ModelDBConfig{
		Aliases: []ModelDBAliasConfig{{
			Name:        "opus",
			ServiceID:   "anthropic",
			WireModelID: "claude-opus-4-6",
		}},
	}, "anthropic", modeldb.APITypeAnthropicMessages, "opus")
	if !ok {
		t.Fatal("expected alias to resolve")
	}
	if item.Offering.WireModelID != "claude-opus-4-6" {
		t.Fatalf("alias resolved to %q", item.Offering.WireModelID)
	}
}

func TestDefaultModelDBAliases(t *testing.T) {
	aliases := DefaultModelDBAliases()
	want := map[string]string{
		"anthropic/haiku":  "claude-haiku-4-5-20251001",
		"anthropic/sonnet": "claude-sonnet-4-6",
		"anthropic/opus":   "claude-opus-4-6",
	}
	for _, alias := range aliases {
		key := alias.ServiceID + "/" + alias.Name
		if want[key] == "" {
			continue
		}
		if alias.WireModelID != want[key] {
			t.Fatalf("%s = %q, want %q", key, alias.WireModelID, want[key])
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing aliases: %+v", want)
	}
}

func TestAutoMuxClientErrorsWithoutDetectedProviders(t *testing.T) {
	clearAutoEnv(t)

	if _, err := AutoMuxClient(AutoOptions{EnableEnv: true, EnableLocalClaude: false}); err == nil {
		t.Fatal("expected error")
	}
}

func clearAutoEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"ANTHROPIC_API_KEY",
		"ANTHROPIC_MODEL",
		"CLAUDE_CONFIG_DIR",
		"CLAUDE_MODEL",
		"CODEX_ACCESS_TOKEN",
		"CODEX_AUTH_PATH",
		"CODEX_CODE_OAUTH_TOKEN",
		"CODEX_MODEL",
		"MINIMAX_API_KEY",
		"MINIMAX_KEY",
		"MINIMAX_MODEL",
		"MINIMAX_MESSAGES_MODEL",
		"OPENAI_API_KEY",
		"OPENAI_KEY",
		"OPENAI_MODEL",
		"OPENAI_RESPONSES_MODEL",
		"OPENROUTER_API_KEY",
		"OPENROUTER_KEY",
		"OPENROUTER_MODEL",
		"OPENROUTER_MESSAGES_MODEL",
		"OPENROUTER_RESPONSES_MODEL",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_AUTH_PATH", filepath.Join(t.TempDir(), "missing-auth.json"))
}

func autoEnabled(providers []AutoProvider, providerType string) bool {
	for _, provider := range providers {
		if provider.Type == providerType {
			return true
		}
	}
	return false
}

func autoRoute(cfg Config, sourceAPI adapt.ApiKind) (RouteConfig, bool) {
	for _, route := range cfg.Routes {
		if route.SourceAPI == sourceAPI {
			return route, true
		}
	}
	return RouteConfig{}, false
}

func autoRouteModel(cfg Config, sourceAPI adapt.ApiKind, model string) (RouteConfig, bool) {
	for _, route := range cfg.Routes {
		if route.SourceAPI == sourceAPI && route.Model == model {
			return route, true
		}
	}
	return RouteConfig{}, false
}
