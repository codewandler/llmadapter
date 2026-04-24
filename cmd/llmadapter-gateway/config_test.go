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
		"providers":[{"name":"anthropic","type":"anthropic","api_key_env":"ANTHROPIC_API_KEY","model":"native"}],
		"routes":[{"source_api":"openai.chat_completions","model":"public","provider":"anthropic"}]
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
