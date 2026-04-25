package adapterconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
)

type Config struct {
	Addr           string           `json:"addr,omitempty"`
	HealthCooldown string           `json:"health_cooldown,omitempty"`
	MaxAttempts    int              `json:"max_attempts,omitempty"`
	ModelDB        ModelDBConfig    `json:"modeldb,omitempty"`
	Providers      []ProviderConfig `json:"providers,omitempty"`
	Routes         []RouteConfig    `json:"routes,omitempty"`
}

type ModelDBConfig struct {
	CatalogPath  string               `json:"catalog_path,omitempty"`
	OverlayPaths []string             `json:"overlay_paths,omitempty"`
	Aliases      []ModelDBAliasConfig `json:"aliases,omitempty"`
}

type ModelDBAliasConfig struct {
	Name        string `json:"name"`
	ServiceID   string `json:"service_id"`
	WireModelID string `json:"wire_model_id"`
}

type ProviderConfig struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	APIKey           string            `json:"api_key,omitempty"`
	APIKeyEnv        string            `json:"api_key_env,omitempty"`
	BaseURL          string            `json:"base_url,omitempty"`
	Model            string            `json:"model,omitempty"`
	ModelDBServiceID string            `json:"modeldb_service_id,omitempty"`
	Priority         int               `json:"priority,omitempty"`
	Capabilities     *CapabilityConfig `json:"capabilities,omitempty"`
}

type RouteConfig struct {
	SourceAPI          adapt.ApiKind `json:"source_api"`
	Model              string        `json:"model,omitempty"`
	Provider           string        `json:"provider"`
	ProviderAPI        adapt.ApiKind `json:"provider_api,omitempty"`
	DynamicModels      bool          `json:"dynamic_models,omitempty"`
	ModelDBModel       string        `json:"modeldb_model,omitempty"`
	NativeModel        string        `json:"native_model,omitempty"`
	ModelDBWireModelID string        `json:"modeldb_wire_model_id,omitempty"`
	Weight             int           `json:"weight,omitempty"`
}

type CapabilityConfig struct {
	Streaming  *bool `json:"streaming,omitempty"`
	Tools      *bool `json:"tools,omitempty"`
	Vision     *bool `json:"vision,omitempty"`
	AudioInput *bool `json:"audio_input,omitempty"`
	JSONMode   *bool `json:"json_mode,omitempty"`
	JSONSchema *bool `json:"json_schema,omitempty"`
	Reasoning  *bool `json:"reasoning,omitempty"`
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	ApplyDefaults(&cfg)
	return cfg, nil
}

func LoadFromEnv() (Config, error) {
	if path := os.Getenv("LLMADAPTER_CONFIG"); path != "" {
		return Load(path)
	}
	return DefaultFromEnv(), nil
}

func DefaultFromEnv() Config {
	cfg := Config{
		Addr: getenv("LLMADAPTER_ADDR", ":8080"),
		Providers: []ProviderConfig{{
			Name:   "anthropic",
			Type:   "anthropic",
			APIKey: os.Getenv("ANTHROPIC_API_KEY"),
		}},
		Routes: []RouteConfig{
			{SourceAPI: adapt.ApiOpenAIChatCompletions, Provider: "anthropic", NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL")},
			{SourceAPI: adapt.ApiAnthropicMessages, Provider: "anthropic", NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL")},
			{SourceAPI: adapt.ApiOpenAIResponses, Provider: "anthropic", NativeModel: os.Getenv("LLMADAPTER_UPSTREAM_MODEL")},
		},
	}
	ApplyDefaults(&cfg)
	return cfg
}

func ApplyDefaults(cfg *Config) {
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
		if cfg.Routes[i].NativeModel == "" && cfg.Routes[i].ModelDBModel == "" && !cfg.Routes[i].DynamicModels {
			if provider, ok := findProviderForRoute(*cfg, cfg.Routes[i]); ok {
				cfg.Routes[i].NativeModel = provider.Model
			}
		}
	}
}

func Validate(cfg Config) error {
	if cfg.HealthCooldown != "" {
		if _, err := time.ParseDuration(cfg.HealthCooldown); err != nil {
			return fmt.Errorf("invalid health_cooldown %q: %w", cfg.HealthCooldown, err)
		}
	}
	if cfg.MaxAttempts < 0 {
		return fmt.Errorf("max_attempts must be >= 0")
	}
	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}
	if len(cfg.Routes) == 0 {
		return fmt.Errorf("at least one route is required")
	}
	for _, route := range cfg.Routes {
		if route.DynamicModels && (route.Model != "" || route.NativeModel != "" || route.ModelDBModel != "" || route.ModelDBWireModelID != "") {
			return fmt.Errorf("dynamic_models route for provider %q must not set model/native_model/modeldb_model/modeldb_wire_model_id", route.Provider)
		}
		if matches := countProviderEndpoints(cfg, route.Provider, route.ProviderAPI); matches == 0 {
			return fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
		} else if matches > 1 {
			return fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
		}
	}
	return nil
}

func ApplyCapabilityOverrides(caps router.CapabilitySet, overrides *CapabilityConfig) router.CapabilitySet {
	if overrides == nil {
		return caps
	}
	if overrides.Streaming != nil {
		caps.Streaming = *overrides.Streaming
	}
	if overrides.Tools != nil {
		caps.Tools = *overrides.Tools
	}
	if overrides.Vision != nil {
		caps.Vision = *overrides.Vision
	}
	if overrides.AudioInput != nil {
		caps.AudioInput = *overrides.AudioInput
	}
	if overrides.JSONMode != nil {
		caps.JSONMode = *overrides.JSONMode
	}
	if overrides.JSONSchema != nil {
		caps.JSONSchema = *overrides.JSONSchema
	}
	if overrides.Reasoning != nil {
		caps.Reasoning = *overrides.Reasoning
	}
	return caps
}

func findProvider(cfg Config, name string) (ProviderConfig, bool) {
	for _, provider := range cfg.Providers {
		if provider.Name == name {
			return provider, true
		}
	}
	return ProviderConfig{}, false
}

func findProviderForRoute(cfg Config, route RouteConfig) (ProviderConfig, bool) {
	for _, provider := range cfg.Providers {
		if providerMatchesRoute(provider, route) {
			return provider, true
		}
	}
	return ProviderConfig{}, false
}

func countProviderEndpoints(cfg Config, providerName string, apiKind adapt.ApiKind) int {
	count := 0
	for _, provider := range cfg.Providers {
		if provider.Name != providerName {
			continue
		}
		if apiKind != "" {
			descriptor, ok := descriptorForProvider(provider)
			if !ok || descriptor.APIKind != apiKind {
				continue
			}
		}
		count++
	}
	return count
}

func providerMatchesRoute(provider ProviderConfig, route RouteConfig) bool {
	if provider.Name != route.Provider {
		return false
	}
	if route.ProviderAPI == "" {
		return true
	}
	descriptor, ok := descriptorForProvider(provider)
	return ok && descriptor.APIKind == route.ProviderAPI
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
