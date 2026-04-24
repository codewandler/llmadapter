package adapterconfig

import (
	"os"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/providerregistry"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	"github.com/codewandler/llmadapter/unified"
)

type AutoOptions struct {
	EnableEnv         bool
	EnableLocalClaude bool
	EnableLocalCodex  bool
	UseModelDB        bool
	SourceAPI         adapt.ApiKind
	Intents           []AutoIntent
}

type AutoIntent struct {
	Name      string
	SourceAPI adapt.ApiKind
}

type AutoResult struct {
	Client  unified.Client
	Config  Config
	Enabled []AutoProvider
	Skipped []AutoProvider
}

type AutoProvider struct {
	Name   string        `json:"name"`
	Type   string        `json:"type"`
	Reason string        `json:"reason,omitempty"`
	API    adapt.ApiKind `json:"api,omitempty"`
	Model  string        `json:"model,omitempty"`
}

func AutoMuxClient(opts AutoOptions) (AutoResult, error) {
	if !opts.EnableEnv && !opts.EnableLocalClaude && !opts.EnableLocalCodex {
		opts.EnableEnv = true
		opts.EnableLocalClaude = true
		opts.EnableLocalCodex = true
	}
	cfg := Config{}
	var enabled []AutoProvider
	var skipped []AutoProvider
	for _, descriptor := range providerregistry.List() {
		provider, status, ok := autoProvider(descriptor, opts)
		if !ok {
			skipped = append(skipped, status)
			continue
		}
		cfg.Providers = append(cfg.Providers, provider)
		enabled = append(enabled, status)
	}
	cfg.Routes = autoRoutes(cfg, opts)
	ApplyDefaults(&cfg)
	sourceAPI := opts.SourceAPI
	if sourceAPI == "" {
		sourceAPI = adapt.ApiOpenAIResponses
	}
	client, err := NewMuxClient(cfg, WithSourceAPI(sourceAPI))
	if err != nil {
		return AutoResult{Config: cfg, Enabled: enabled, Skipped: skipped}, err
	}
	return AutoResult{Client: client, Config: cfg, Enabled: enabled, Skipped: skipped}, nil
}

func autoProvider(descriptor providerregistry.Descriptor, opts AutoOptions) (ProviderConfig, AutoProvider, bool) {
	model := modelFromEnv(descriptor)
	status := AutoProvider{Name: descriptor.Type, Type: descriptor.Type, API: descriptor.APIKind, Model: model}
	if descriptor.Type == "claude_messages" {
		if opts.EnableEnv {
			if key, envName := firstEnvWithName(descriptor.DefaultAPIKeyEnvs...); key != "" {
				status.Reason = "env:" + envName
				return autoProviderConfig(descriptor, envName, model, opts), status, true
			}
		}
		if opts.EnableLocalClaude && anthropic.LocalTokenStoreAvailable() {
			status.Reason = "local_claude_oauth"
			return autoProviderConfig(descriptor, "", model, opts), status, true
		}
		status.Reason = "missing Claude OAuth env/local credentials"
		return ProviderConfig{}, status, false
	}
	if descriptor.Type == "codex_responses" {
		if opts.EnableEnv {
			if key, envName := firstEnvWithName(descriptor.DefaultAPIKeyEnvs...); key != "" {
				status.Reason = "env:" + envName
				return autoProviderConfig(descriptor, envName, model, opts), status, true
			}
		}
		if opts.EnableLocalCodex && codex.LocalAvailable() {
			status.Reason = "local_codex_oauth"
			return autoProviderConfig(descriptor, "", model, opts), status, true
		}
		status.Reason = "missing Codex OAuth env/local credentials"
		return ProviderConfig{}, status, false
	}
	if !opts.EnableEnv {
		status.Reason = "env auto detection disabled"
		return ProviderConfig{}, status, false
	}
	if _, envName := firstEnvWithName(descriptor.DefaultAPIKeyEnvs...); envName != "" {
		status.Reason = "env:" + envName
		return autoProviderConfig(descriptor, envName, model, opts), status, true
	}
	status.Reason = "missing env credentials"
	return ProviderConfig{}, status, false
}

func autoProviderConfig(descriptor providerregistry.Descriptor, apiKeyEnv string, model string, opts AutoOptions) ProviderConfig {
	return ProviderConfig{
		Name:             descriptor.Type,
		Type:             descriptor.Type,
		APIKeyEnv:        apiKeyEnv,
		Model:            model,
		ModelDBServiceID: autoModelDBServiceID(descriptor.Type, opts.UseModelDB),
	}
}

func autoRoutes(cfg Config, opts AutoOptions) []RouteConfig {
	if len(opts.Intents) > 0 {
		var out []RouteConfig
		for _, intent := range opts.Intents {
			route, ok := routeForIntent(cfg, intent)
			if ok {
				out = append(out, route)
			}
		}
		return out
	}
	var routes []RouteConfig
	for _, sourceAPI := range []adapt.ApiKind{adapt.ApiOpenAIResponses, adapt.ApiOpenAIChatCompletions, adapt.ApiAnthropicMessages} {
		route, ok := bestRouteForAPI(cfg, sourceAPI)
		if ok {
			routes = append(routes, route)
		}
	}
	return routes
}

func routeForIntent(cfg Config, intent AutoIntent) (RouteConfig, bool) {
	sourceAPI := intent.SourceAPI
	if sourceAPI == "" {
		sourceAPI = adapt.ApiOpenAIResponses
	}
	route, ok := bestRouteForAPI(cfg, sourceAPI)
	if !ok {
		return RouteConfig{}, false
	}
	route.Model = intent.Name
	return route, true
}

func bestRouteForAPI(cfg Config, sourceAPI adapt.ApiKind) (RouteConfig, bool) {
	provider, ok := bestProviderForAPI(cfg, sourceAPI)
	if !ok {
		return RouteConfig{}, false
	}
	descriptor, _ := descriptorForProvider(provider)
	publicModel := descriptor.DefaultModel
	if provider.Model != "" {
		publicModel = provider.Model
	}
	return RouteConfig{
		SourceAPI:   sourceAPI,
		Model:       publicModel,
		Provider:    provider.Name,
		ProviderAPI: descriptor.APIKind,
		NativeModel: provider.Model,
		Weight:      100,
	}, true
}

func bestProviderForAPI(cfg Config, sourceAPI adapt.ApiKind) (ProviderConfig, bool) {
	preferred := preferredProviderTypes(sourceAPI)
	for _, providerType := range preferred {
		for _, provider := range cfg.Providers {
			if provider.Type == providerType {
				return provider, true
			}
		}
	}
	for _, provider := range cfg.Providers {
		descriptor, ok := descriptorForProvider(provider)
		if ok && descriptor.APIKind == sourceAPI {
			return provider, true
		}
	}
	return ProviderConfig{}, false
}

func preferredProviderTypes(sourceAPI adapt.ApiKind) []string {
	switch sourceAPI {
	case adapt.ApiOpenAIResponses:
		return []string{"openai_responses", "openrouter_responses", "codex_responses", "anthropic", "claude_messages", "openrouter_messages", "minimax_messages"}
	case adapt.ApiOpenAIChatCompletions:
		return []string{"openai_chat", "openrouter_chat", "minimax_chat", "anthropic", "claude_messages"}
	case adapt.ApiAnthropicMessages:
		return []string{"anthropic", "claude_messages", "openrouter_messages", "minimax_messages"}
	default:
		return nil
	}
}

func autoModelDBServiceID(providerType string, enabled bool) string {
	if !enabled {
		return ""
	}
	switch providerType {
	case "anthropic", "claude_messages":
		return "anthropic"
	case "openai_chat", "openai_responses":
		return "openai"
	case "codex_responses":
		return "codex"
	case "openrouter_chat", "openrouter_responses", "openrouter_messages":
		return "openrouter"
	case "minimax_chat", "minimax_messages":
		return "minimax"
	default:
		return ""
	}
}

func modelFromEnv(descriptor providerregistry.Descriptor) string {
	if descriptor.DefaultModelEnv != "" {
		if value := os.Getenv(descriptor.DefaultModelEnv); value != "" {
			return value
		}
	}
	return descriptor.DefaultModel
}

func firstEnvWithName(keys ...string) (string, string) {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value, key
		}
	}
	return "", ""
}
