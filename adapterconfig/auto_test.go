package adapterconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
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
	if !autoEnabled(result.Enabled, "claude_messages") {
		t.Fatalf("expected claude_messages enabled: %+v", result.Enabled)
	}
	route, ok := autoRoute(result.Config, adapt.ApiAnthropicMessages)
	if !ok {
		t.Fatalf("expected Anthropic Messages route: %+v", result.Config.Routes)
	}
	if route.Provider != "claude_messages" {
		t.Fatalf("unexpected route: %+v", route)
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
		"CLAUDE_ACCESS_TOKEN",
		"CLAUDE_CODE_OAUTH_TOKEN",
		"CLAUDE_CONFIG_DIR",
		"CLAUDE_MODEL",
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
