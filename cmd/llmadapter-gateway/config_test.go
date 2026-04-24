package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"addr":":9090",
		"providers":[{"name":"anthropic","type":"anthropic","api_key_env":"ANTHROPIC_API_KEY","model":"native","priority":7}],
		"routes":[{"source_api":"openai.chat_completions","model":"public","provider":"anthropic","weight":11}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" || len(cfg.Providers) != 1 || len(cfg.Routes) != 1 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Routes[0].SourceAPI != adapt.ApiOpenAIChatCompletions || cfg.Routes[0].NativeModel != "native" {
		t.Fatalf("unexpected route defaults: %+v", cfg.Routes[0])
	}
	if cfg.Providers[0].Priority != 7 || cfg.Routes[0].Weight != 11 {
		t.Fatalf("unexpected routing weights: %+v %+v", cfg.Providers[0], cfg.Routes[0])
	}
}

func TestDefaultConfigIncludesGatewayRoutes(t *testing.T) {
	t.Setenv("LLMADAPTER_ADDR", "")
	t.Setenv("ANTHROPIC_API_KEY", "key")
	cfg := defaultConfigFromEnv()
	if len(cfg.Routes) != 3 {
		t.Fatalf("routes = %+v", cfg.Routes)
	}
	if cfg.Routes[0].SourceAPI != adapt.ApiOpenAIChatCompletions || cfg.Routes[1].SourceAPI != adapt.ApiAnthropicMessages || cfg.Routes[2].SourceAPI != adapt.ApiOpenAIResponses {
		t.Fatalf("unexpected default routes: %+v", cfg.Routes)
	}
}

func TestValidateConfig(t *testing.T) {
	err := validateConfig(config{
		Providers: []providerConfig{{Name: "anthropic", Type: "anthropic"}},
		Routes:    []routeConfig{{Provider: "missing"}},
	})
	if err == nil {
		t.Fatalf("expected unknown provider error")
	}
}

func TestValidateConfigRequiresProviderAPIForAmbiguousProvider(t *testing.T) {
	err := validateConfig(config{
		Providers: []providerConfig{
			{Name: "openrouter", Type: "openrouter_chat"},
			{Name: "openrouter", Type: "anthropic"},
		},
		Routes: []routeConfig{{Provider: "openrouter"}},
	})
	if err == nil {
		t.Fatalf("expected ambiguous provider endpoint error")
	}

	err = validateConfig(config{
		Providers: []providerConfig{
			{Name: "openrouter", Type: "openrouter_chat"},
			{Name: "openrouter", Type: "anthropic"},
		},
		Routes: []routeConfig{{Provider: "openrouter", ProviderAPI: adapt.ApiOpenRouterChatCompletions}},
	})
	if err != nil {
		t.Fatalf("unexpected provider endpoint validation error: %v", err)
	}
}

func TestBuildProviderOpenAIRequiresKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_KEY", "")
	_, err := buildProvider(providerConfig{Name: "openai", Type: "openai_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestBuildProviderOpenRouterRequiresKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_KEY", "")
	_, err := buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_responses"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "openrouter", Type: "openrouter_messages"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestBuildProviderMiniMaxRequiresKey(t *testing.T) {
	t.Setenv("MINIMAX_API_KEY", "")
	t.Setenv("MINIMAX_KEY", "")
	_, err := buildProvider(providerConfig{Name: "minimax", Type: "minimax_chat"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
	_, err = buildProvider(providerConfig{Name: "minimax", Type: "minimax_messages"})
	if err == nil {
		t.Fatalf("expected missing key error")
	}
}

func TestProviderEndpointMetadata(t *testing.T) {
	tests := []struct {
		providerType string
		apiKind      adapt.ApiKind
		family       adapt.ApiFamily
		tools        bool
	}{
		{"openrouter_chat", adapt.ApiOpenRouterChatCompletions, adapt.FamilyOpenAIChatCompletions, true},
		{"openrouter_responses", adapt.ApiOpenRouterResponses, adapt.FamilyOpenAIResponses, true},
		{"openrouter_messages", adapt.ApiOpenRouterAnthropicMessages, adapt.FamilyAnthropicMessages, true},
		{"minimax_chat", adapt.ApiMiniMaxChatCompletions, adapt.FamilyOpenAIChatCompletions, false},
		{"minimax_messages", adapt.ApiMiniMaxAnthropicMessages, adapt.FamilyAnthropicMessages, true},
	}
	for _, tt := range tests {
		apiKind, family, capabilities, err := providerEndpointMetadata(tt.providerType)
		if err != nil {
			t.Fatal(err)
		}
		if apiKind != tt.apiKind || family != tt.family {
			t.Fatalf("unexpected endpoint metadata: %q %q", apiKind, family)
		}
		if !capabilities.Streaming || capabilities.Tools != tt.tools {
			t.Fatalf("unexpected capabilities: %+v", capabilities)
		}
		if (tt.providerType == "openai_chat" || tt.providerType == "openrouter_chat" || tt.providerType == "openrouter_responses") && (!capabilities.JSONMode || !capabilities.JSONSchema) {
			t.Fatalf("expected JSON capabilities: %+v", capabilities)
		}
		if (tt.providerType == "openai_chat" || tt.providerType == "openrouter_chat" || tt.providerType == "openrouter_responses" || tt.providerType == "openrouter_messages") && !capabilities.Vision {
			t.Fatalf("expected vision capability: %+v", capabilities)
		}
	}
}
