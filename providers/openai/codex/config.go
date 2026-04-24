package codex

import "time"

const (
	ProviderName   = "codex"
	ServiceID      = "codex"
	DefaultBaseURL = "https://chatgpt.com/backend-api"
	DefaultPath    = "/codex/responses"
	DefaultModel   = "gpt-5.4"
)

const (
	EnvAuthPath    = "CODEX_AUTH_PATH"
	EnvAccessToken = "CODEX_ACCESS_TOKEN"
	EnvOAuthToken  = "CODEX_CODE_OAUTH_TOKEN"
	EnvModel       = "CODEX_MODEL"
)

const (
	AuthFilePath      = ".codex/auth.json"
	TokenEndpoint     = "https://auth.openai.com/oauth/token"
	ClientID          = "app_EMoamEEZ73f0CkXaXp7hrann"
	TokenExpiryBuffer = 5 * time.Minute
	ChatGPTAuthMode   = "chatgpt"
)

const (
	HeaderChatGPTAccountID       = "ChatGPT-Account-ID"
	HeaderSessionID              = "session_id"
	HeaderCodexWindowID          = "x-codex-window-id"
	HeaderCodexTurnState         = "x-codex-turn-state"
	HeaderCodexInstallationID    = "x-codex-installation-id"
	HeaderCodexBetaFeatures      = "x-codex-beta-features"
	HeaderCodexTurnMetadata      = "x-codex-turn-metadata"
	HeaderCodexParentThreadID    = "x-codex-parent-thread-id"
	HeaderOpenAISubagent         = "x-openai-subagent"
	HeaderOpenAIMemgenRequest    = "x-openai-memgen-request"
	HeaderTimingMetrics          = "x-responsesapi-include-timing-metrics"
	HeaderPromptCacheKey         = "prompt_cache_key"
	defaultInstructions          = "You are a helpful assistant."
	defaultWindowGeneration      = "0"
	defaultInstallationIDEntropy = 16
)
