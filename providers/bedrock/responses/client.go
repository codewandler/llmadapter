package responses

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/codewandler/llmadapter/providers/bedrock/internal/mantleauth"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	"github.com/codewandler/llmadapter/unified"
)

const (
	EnvAPIKey          = "BEDROCK_API_KEY"
	EnvBearerToken     = "AWS_BEARER_TOKEN_BEDROCK"
	EnvRegion          = "AWS_REGION"
	EnvDefaultRegion   = "AWS_DEFAULT_REGION"
	EnvModel           = "BEDROCK_RESPONSES_MODEL"
	DefaultModel       = "openai.gpt-oss-120b"
	defaultRegion      = "us-east-1"
	defaultAPIKindName = "bedrock.responses"
)

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{apiKey: explicitTokenFromEnv(), baseURL: defaultBaseURL(), warningSource: defaultAPIKindName}
	for _, opt := range opts {
		opt.applyBedrockResponses(&cfg)
	}
	if cfg.apiKey == "" && cfg.tokenProvider == nil {
		cfg.tokenProvider = NewAWSTokenProvider(defaultRegionFromEnv())
	}
	credentialed := &mantleauth.CredentialTransport{
		Inner:                    cfg.transport,
		StaticToken:              cfg.apiKey,
		TokenProvider:            cfg.tokenProvider,
		MissingCredentialMessage: "bedrock responses credentials are not configured",
	}
	openAIOptions := []openairesponses.Option{
		openairesponses.WithAPIKey("bedrock-token-provider"),
		openairesponses.WithBaseURL(cfg.baseURL),
		openairesponses.WithWarningSource(cfg.warningSource),
		openairesponses.WithTransport(credentialed),
		openairesponses.WithBodyMutator(bedrockBodyMutator(cfg.warningSource)),
	}
	return openairesponses.NewClient(openAIOptions...)
}

func bedrockBodyMutator(source string) func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error) {
	return func(req unified.Request, body []byte) ([]byte, []unified.WarningEvent, error) {
		if req.ToolChoice == nil || req.ToolChoice.Mode == "" || req.ToolChoice.Mode == unified.ToolChoiceAuto {
			return body, nil, nil
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			return body, nil, nil
		}
		payload["tool_choice"] = json.RawMessage(`"auto"`)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		return encoded, []unified.WarningEvent{{
			Code:    "unsupported_field_dropped",
			Message: "Bedrock Mantle Responses only supports tool_choice auto; requested tool_choice was sent as auto",
			Source:  source,
			Meta: map[string]any{
				"field":         "tool_choice",
				"requested":     string(req.ToolChoice.Mode),
				"replacement":   string(unified.ToolChoiceAuto),
				"requestedName": req.ToolChoice.Name,
			},
		}}, nil
	}
}

func defaultBaseURL() string {
	return fmt.Sprintf("https://bedrock-mantle.%s.api.aws", defaultRegionFromEnv())
}

func defaultRegionFromEnv() string {
	if region := os.Getenv(EnvRegion); region != "" {
		return region
	}
	if region := os.Getenv(EnvDefaultRegion); region != "" {
		return region
	}
	return mantleauth.DefaultRegion
}
