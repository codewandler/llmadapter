package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/codewandler/llmadapter/adapt"
)

type config struct {
	Addr      string           `json:"addr,omitempty"`
	Providers []providerConfig `json:"providers,omitempty"`
	Routes    []routeConfig    `json:"routes,omitempty"`
}

type providerConfig struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	APIKey    string `json:"api_key,omitempty"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
	Model     string `json:"model,omitempty"`
	Priority  int    `json:"priority,omitempty"`
}

type routeConfig struct {
	SourceAPI   adapt.ApiKind `json:"source_api"`
	Model       string        `json:"model,omitempty"`
	Provider    string        `json:"provider"`
	ProviderAPI adapt.ApiKind `json:"provider_api,omitempty"`
	NativeModel string        `json:"native_model,omitempty"`
	Weight      int           `json:"weight,omitempty"`
}

func loadConfigFromEnv() (config, error) {
	path := os.Getenv("LLMADAPTER_CONFIG")
	if path == "" {
		return defaultConfigFromEnv(), nil
	}
	return loadConfig(path)
}

func loadConfig(path string) (config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return config{}, err
	}
	applyConfigDefaults(&cfg)
	return cfg, nil
}

func defaultConfigFromEnv() config {
	cfg := config{
		Addr: getenv("LLMADAPTER_ADDR", ":8080"),
		Providers: []providerConfig{
			{
				Name:   "anthropic",
				Type:   "anthropic",
				APIKey: os.Getenv("ANTHROPIC_API_KEY"),
			},
		},
		Routes: []routeConfig{
			{
				SourceAPI:   adapt.ApiOpenAIChatCompletions,
				Provider:    "anthropic",
				NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL"),
			},
			{
				SourceAPI:   adapt.ApiAnthropicMessages,
				Provider:    "anthropic",
				NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL"),
			},
			{
				SourceAPI:   adapt.ApiOpenAIResponses,
				Provider:    "anthropic",
				NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL"),
			},
		},
	}
	applyConfigDefaults(&cfg)
	return cfg
}

func applyConfigDefaults(cfg *config) {
	if cfg.Addr == "" {
		cfg.Addr = getenv("LLMADAPTER_ADDR", ":8080")
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].Type == "" {
			cfg.Providers[i].Type = cfg.Providers[i].Name
		}
	}
	for i := range cfg.Routes {
		if cfg.Routes[i].SourceAPI == "" {
			cfg.Routes[i].SourceAPI = adapt.ApiOpenAIChatCompletions
		}
		if cfg.Routes[i].NativeModel == "" {
			if provider, ok := findProvider(*cfg, cfg.Routes[i].Provider); ok {
				cfg.Routes[i].NativeModel = provider.Model
			}
		}
	}
}

func findProvider(cfg config, name string) (providerConfig, bool) {
	for _, provider := range cfg.Providers {
		if provider.Name == name {
			return provider, true
		}
	}
	return providerConfig{}, false
}

func validateConfig(cfg config) error {
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}
	for _, route := range cfg.Routes {
		if matches := countProviderEndpoints(cfg, route.Provider, route.ProviderAPI); matches == 0 {
			return fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
		} else if matches > 1 {
			return fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
		}
	}
	return nil
}

func countProviderEndpoints(cfg config, providerName string, apiKind adapt.ApiKind) int {
	count := 0
	for _, provider := range cfg.Providers {
		if provider.Name != providerName {
			continue
		}
		if apiKind != "" {
			providerAPI, _, _, err := providerEndpointMetadata(provider.Type)
			if err != nil || providerAPI != apiKind {
				continue
			}
		}
		count++
	}
	return count
}
