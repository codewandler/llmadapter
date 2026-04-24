package providerregistry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimax "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type Descriptor struct {
	Type              string               `json:"type"`
	APIKind           adapt.ApiKind        `json:"api_kind"`
	Family            adapt.ApiFamily      `json:"family"`
	Capabilities      router.CapabilitySet `json:"capabilities"`
	DefaultAPIKeyEnvs []string             `json:"default_api_key_envs,omitempty"`
	DefaultModelEnv   string               `json:"default_model_env,omitempty"`
	DefaultModel      string               `json:"default_model,omitempty"`
}

type ClientConfig struct {
	Type    string
	APIKey  string
	BaseURL string
}

var descriptors = []Descriptor{
	{
		Type:              "anthropic",
		APIKind:           adapt.ApiAnthropicMessages,
		Family:            adapt.FamilyAnthropicMessages,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true},
		DefaultAPIKeyEnvs: []string{"ANTHROPIC_API_KEY"},
		DefaultModelEnv:   "ANTHROPIC_MODEL",
		DefaultModel:      "claude-haiku-4-5-20251001",
	},
	{
		Type:              "claude_messages",
		APIKind:           adapt.ApiAnthropicMessages,
		Family:            adapt.FamilyAnthropicMessages,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true},
		DefaultAPIKeyEnvs: []string{"CLAUDE_ACCESS_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"},
		DefaultModelEnv:   "CLAUDE_MODEL",
		DefaultModel:      "claude-haiku-4-5-20251001",
	},
	{
		Type:              "openai_chat",
		APIKind:           adapt.ApiOpenAIChatCompletions,
		Family:            adapt.FamilyOpenAIChatCompletions,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		DefaultAPIKeyEnvs: []string{"OPENAI_API_KEY", "OPENAI_KEY"},
		DefaultModelEnv:   "OPENAI_MODEL",
		DefaultModel:      "gpt-4.1-mini",
	},
	{
		Type:              "openai_responses",
		APIKind:           adapt.ApiOpenAIResponses,
		Family:            adapt.FamilyOpenAIResponses,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		DefaultAPIKeyEnvs: []string{"OPENAI_API_KEY", "OPENAI_KEY"},
		DefaultModelEnv:   "OPENAI_RESPONSES_MODEL",
		DefaultModel:      "gpt-4.1-mini",
	},
	{
		Type:              "codex_responses",
		APIKind:           adapt.ApiCodexResponses,
		Family:            adapt.FamilyOpenAIResponses,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true},
		DefaultAPIKeyEnvs: []string{codex.EnvAccessToken, codex.EnvOAuthToken},
		DefaultModelEnv:   codex.EnvModel,
		DefaultModel:      codex.DefaultModel,
	},
	{
		Type:              "openrouter_chat",
		APIKind:           adapt.ApiOpenRouterChatCompletions,
		Family:            adapt.FamilyOpenAIChatCompletions,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		DefaultAPIKeyEnvs: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:   "OPENROUTER_MODEL",
		DefaultModel:      "openai/gpt-4.1-mini",
	},
	{
		Type:              "openrouter_responses",
		APIKind:           adapt.ApiOpenRouterResponses,
		Family:            adapt.FamilyOpenAIResponses,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, JSONMode: true, JSONSchema: true},
		DefaultAPIKeyEnvs: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:   "OPENROUTER_RESPONSES_MODEL",
		DefaultModel:      "openai/gpt-4.1-mini",
	},
	{
		Type:              "openrouter_messages",
		APIKind:           adapt.ApiOpenRouterAnthropicMessages,
		Family:            adapt.FamilyAnthropicMessages,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true},
		DefaultAPIKeyEnvs: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:   "OPENROUTER_MESSAGES_MODEL",
		DefaultModel:      "anthropic/claude-sonnet-4",
	},
	{
		Type:              "minimax_chat",
		APIKind:           adapt.ApiMiniMaxChatCompletions,
		Family:            adapt.FamilyOpenAIChatCompletions,
		Capabilities:      router.CapabilitySet{Streaming: true},
		DefaultAPIKeyEnvs: []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
		DefaultModelEnv:   "MINIMAX_MODEL",
		DefaultModel:      "MiniMax-M2.7",
	},
	{
		Type:              "minimax_messages",
		APIKind:           adapt.ApiMiniMaxAnthropicMessages,
		Family:            adapt.FamilyAnthropicMessages,
		Capabilities:      router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true, ReasoningDeltas: true},
		DefaultAPIKeyEnvs: []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
		DefaultModelEnv:   "MINIMAX_MESSAGES_MODEL",
		DefaultModel:      "MiniMax-M2.7",
	},
}

func List() []Descriptor {
	out := append([]Descriptor(nil), descriptors...)
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func Lookup(providerType string) (Descriptor, bool) {
	for _, descriptor := range descriptors {
		if descriptor.Type == providerType {
			return descriptor, true
		}
	}
	return Descriptor{}, false
}

func NewClient(cfg ClientConfig) (unified.Client, error) {
	providerType := strings.TrimSpace(cfg.Type)
	if providerType == "" {
		return nil, fmt.Errorf("provider type is required")
	}
	switch providerType {
	case "anthropic":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []anthropic.Option{
			anthropic.WithAPIKey(cfg.APIKey),
			anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
				req.Unified.Stream = true
				return nil
			})),
		}
		if cfg.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
		}
		return anthropic.NewClient(opts...)
	case "claude_messages":
		opts := []anthropic.Option{
			anthropic.WithClaudeHeaders(),
			anthropic.WithClaudeCodePreflight(),
			anthropic.WithSystemCacheControl(""),
			anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
				req.Unified.Stream = true
				return nil
			})),
		}
		if cfg.APIKey != "" {
			opts = append(opts, anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken(cfg.APIKey))))
		} else {
			opts = append(opts, anthropic.WithLocalClaudeOAuth())
		}
		if cfg.BaseURL != "" {
			opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
		}
		return anthropic.NewClient(opts...)
	case "openai_chat":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
		}
		return openai.NewClient(opts...)
	case "openai_responses":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []openairesponses.Option{openairesponses.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openairesponses.WithBaseURL(cfg.BaseURL))
		}
		return openairesponses.NewClient(opts...)
	case "codex_responses":
		opts := []codex.Option{}
		if cfg.APIKey != "" {
			opts = append(opts, codex.WithAccessToken(cfg.APIKey))
		}
		if cfg.BaseURL != "" {
			opts = append(opts, codex.WithBaseURL(cfg.BaseURL))
		}
		return codex.NewClient(opts...)
	case "openrouter_chat":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []openrouter.Option{openrouter.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openrouter.WithBaseURL(cfg.BaseURL))
		}
		return openrouter.NewClient(opts...)
	case "openrouter_responses":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []openrouterresponses.Option{openrouterresponses.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openrouterresponses.WithBaseURL(cfg.BaseURL))
		}
		return openrouterresponses.NewClient(opts...)
	case "openrouter_messages":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []openroutermessages.Option{openroutermessages.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, openroutermessages.WithBaseURL(cfg.BaseURL))
		}
		return openroutermessages.NewClient(opts...)
	case "minimax_chat":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []minimax.Option{minimax.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, minimax.WithBaseURL(cfg.BaseURL))
		}
		return minimax.NewClient(opts...)
	case "minimax_messages":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("provider type %q requires api key", providerType)
		}
		opts := []minimaxmessages.Option{minimaxmessages.WithAPIKey(cfg.APIKey)}
		if cfg.BaseURL != "" {
			opts = append(opts, minimaxmessages.WithBaseURL(cfg.BaseURL))
		}
		return minimaxmessages.NewClient(opts...)
	default:
		return nil, fmt.Errorf("unsupported provider type %q", providerType)
	}
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
