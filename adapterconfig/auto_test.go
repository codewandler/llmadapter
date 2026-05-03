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

func TestAutoMuxClientDetectsBedrockConverseAWSCredentials(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("AWS_PROFILE", "dev")
	oldDetector := autoAWSCredentialsAvailable
	autoAWSCredentialsAvailable = func() bool { return true }
	t.Cleanup(func() { autoAWSCredentialsAvailable = oldDetector })

	result, err := AutoMuxClient(AutoOptions{EnableEnv: true, EnableLocalClaude: false, EnableLocalCodex: false, UseModelDB: true})
	if err != nil {
		t.Fatal(err)
	}
	if !autoEnabled(result.Enabled, "bedrock_converse") {
		t.Fatalf("expected bedrock_converse enabled: %+v", result.Enabled)
	}
	provider, ok := findProvider(result.Config, "bedrock_converse")
	if !ok {
		t.Fatalf("missing provider config: %+v", result.Config.Providers)
	}
	if provider.APIKeyEnv != "" || provider.ModelDBServiceID != "bedrock" {
		t.Fatalf("unexpected provider config: %+v", provider)
	}
	route, ok := autoRoute(result.Config, adapt.ApiAnthropicMessages)
	if !ok {
		t.Fatalf("expected Anthropic Messages route: %+v", result.Config.Routes)
	}
	if route.Provider != "bedrock_converse" || route.ProviderAPI != adapt.ApiBedrockConverse {
		t.Fatalf("unexpected route: %+v", route)
	}
}

func TestAutoMuxClientIntentsCreatePublicRoutes(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: false,
		UseModelDB:        true,
		Intents: []AutoIntent{{
			Name:      "gpt-4.1-mini",
			SourceAPI: adapt.ApiOpenAIResponses,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Config.Routes) != 1 {
		t.Fatalf("expected one route, got %+v", result.Config.Routes)
	}
	if result.Config.Routes[0].Model != "gpt-4.1-mini" || result.Config.Routes[0].ModelDBModel != "gpt-4.1-mini" {
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

func TestAutoMuxClientModelDBIntentPrefersQualifiedService(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("CODEX_ACCESS_TOKEN", "test-codex-token")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:     true,
		UseModelDB:    true,
		DynamicModels: true,
		SourceAPI:     adapt.ApiOpenAIResponses,
		Intents: []AutoIntent{{
			Name:      "codex/gpt-5.4",
			SourceAPI: adapt.ApiOpenAIResponses,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	route, ok := autoRouteModel(result.Config, adapt.ApiOpenAIResponses, "codex/gpt-5.4")
	if !ok {
		t.Fatalf("missing codex/gpt-5.4 route: %+v", result.Config.Routes)
	}
	if route.Provider != "codex_responses" || route.ProviderAPI != adapt.ApiCodexResponses || route.ModelDBModel != "codex/gpt-5.4" {
		t.Fatalf("expected qualified codex route, got %+v", route)
	}
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "codex/gpt-5.4", Stream: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ProviderName != "codex_responses" || selected.NativeModel != "gpt-5.4" {
		t.Fatalf("unexpected selected route: %+v", selected)
	}
}

func TestAutoMuxClientQualifiedIntentDoesNotFallbackToWrongService(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:     true,
		UseModelDB:    true,
		DynamicModels: true,
		Intents: []AutoIntent{{
			Name: "openai/gpt-5.5",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := autoRouteModel(result.Config, adapt.ApiAnthropicMessages, "openai/gpt-5.5"); ok {
		t.Fatalf("qualified OpenAI intent should not create Anthropic fallback route: %+v", result.Config.Routes)
	}
	route, ok := autoRouteModel(result.Config, adapt.ApiOpenAIResponses, "openai/gpt-5.5")
	if !ok {
		t.Fatalf("missing OpenAI route: %+v", result.Config.Routes)
	}
	if route.Provider != "openai_responses" || route.ProviderAPI != adapt.ApiOpenAIResponses {
		t.Fatalf("unexpected OpenAI route: %+v", route)
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
		DynamicModels:     true,
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

func TestAutoMuxClientAutoSourceOpenRouterAliasUsesCatalogEndpoint(t *testing.T) {
	clearAutoEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "test-openrouter-key")

	result, err := AutoMuxClient(AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: false,
		UseModelDB:        true,
		DynamicModels:     true,
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
		UseModelDB:    true,
		DynamicModels: true,
		Intents:       []AutoIntent{{Name: "gpt-4.1-mini", SourceAPI: adapt.ApiOpenAIResponses}},
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
		if route.Model == "gpt-4.1-mini" && !route.DynamicModels {
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

func TestAutoMuxClientDoesNotAddBuiltInModelAliasRoutes(t *testing.T) {
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
	r, err := BuildRouter(result.Config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Route(context.Background(), adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		Unified:   unified.Request{Model: "sonnet", Stream: true},
	})
	if err == nil {
		t.Fatalf("expected catalog alias to require an explicit intent, dynamic route, or operator alias")
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

func TestResolveModelDBItemSupportsServiceQualifiedName(t *testing.T) {
	catalog, err := LoadModelDBCatalog(ModelDBConfig{})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := resolveModelDBItem(catalog, ModelDBConfig{}, "codex", modeldb.APITypeOpenAIResponses, "codex/gpt-5.4")
	if !ok {
		t.Fatal("expected qualified codex model to resolve")
	}
	if item.Offering.ServiceID != "codex" || item.Offering.WireModelID != "gpt-5.4" {
		t.Fatalf("unexpected item: %+v", item.Offering)
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
		"AWS_ACCESS_KEY_ID",
		"AWS_CONFIG_FILE",
		"AWS_CONTAINER_CREDENTIALS_FULL_URI",
		"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
		"AWS_DEFAULT_REGION",
		"AWS_EC2_METADATA_DISABLED",
		"AWS_PROFILE",
		"AWS_REGION",
		"AWS_ROLE_ARN",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_SHARED_CREDENTIALS_FILE",
		"AWS_WEB_IDENTITY_TOKEN_FILE",
		"BEDROCK_CONVERSE_MODEL",
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
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-aws-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing-aws-credentials"))
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
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
