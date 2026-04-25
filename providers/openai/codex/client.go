package codex

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{
		baseURL:        DefaultBaseURL,
		path:           DefaultPath,
		installationID: randomInstallationID(),
	}
	for _, opt := range opts {
		opt.applyCodex(&cfg)
	}
	auth, err := codexAuth(cfg)
	if err != nil {
		return nil, err
	}
	base := cfg.transport
	if base == nil {
		base = transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{
			Client:      cfg.httpClient,
			FrameFormat: transport.FrameFormatSSE,
		})
	}
	return openrouterresponses.NewClient(
		openrouterresponses.WithAPIKey("codex-auth-via-transport"),
		openrouterresponses.WithBaseURL(cfg.baseURL),
		openrouterresponses.WithTransport(&codexTransport{
			base:           base,
			auth:           auth.WithHTTPClient(cfg.httpClient),
			baseURL:        strings.TrimRight(cfg.baseURL, "/"),
			path:           cfg.path,
			installationID: cfg.installationID,
			betaFeatures:   cfg.betaFeatures,
		}),
		openrouterresponses.WithWarningSource("codex.responses"),
	)
}

func codexAuth(cfg config) (*Auth, error) {
	if cfg.accessToken != "" {
		return NewStaticAuth(cfg.accessToken), nil
	}
	var (
		auth *Auth
		err  error
	)
	if cfg.authPath != "" {
		auth, err = LoadAuthFrom(cfg.authPath)
	} else {
		auth, err = LoadAuth()
	}
	if err != nil {
		return nil, fmt.Errorf("codex: load auth: %w", err)
	}
	return auth, nil
}

type codexTransport struct {
	base           transport.ByteStreamTransport
	auth           *Auth
	baseURL        string
	path           string
	installationID string
	betaFeatures   string
}

func (t *codexTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
	mutated, promptCacheKey, err := mutateCodexBody(req.Body)
	if err != nil {
		return nil, err
	}
	codexExt, warnings := codexExtensionsFromTransport(req.Extensions)
	if len(warnings) > 0 {
		return nil, fmt.Errorf("codex: invalid extensions: %s", warnings[0].Message)
	}
	header := req.Header.Clone()
	header.Set("Content-Type", "application/json")
	if err := t.auth.SetHeaders(ctx, header); err != nil {
		return nil, err
	}
	if t.installationID != "" {
		header.Set(HeaderCodexInstallationID, t.installationID)
	}
	if t.betaFeatures != "" {
		header.Set(HeaderCodexBetaFeatures, t.betaFeatures)
	}
	sessionID := firstNonEmpty(codexExt.SessionID, promptCacheKey)
	if sessionID != "" {
		header.Set(HeaderSessionID, sessionID)
	}
	windowID := codexExt.WindowID
	if windowID == "" && sessionID != "" {
		windowID = sessionID + ":" + defaultWindowGeneration
	}
	if windowID != "" {
		header.Set(HeaderCodexWindowID, windowID)
	}
	if codexExt.TurnState != "" {
		header.Set(HeaderCodexTurnState, codexExt.TurnState)
	}
	if codexExt.TurnMetadata != "" {
		header.Set(HeaderCodexTurnMetadata, codexExt.TurnMetadata)
	}
	if codexExt.ParentThreadID != "" {
		header.Set(HeaderCodexParentThreadID, codexExt.ParentThreadID)
	}
	if codexExt.Subagent {
		header.Set(HeaderOpenAISubagent, "true")
	}
	if codexExt.MemgenRequest {
		header.Set(HeaderOpenAIMemgenRequest, "true")
	}
	if codexExt.IncludeTimingMetrics {
		header.Set(HeaderTimingMetrics, "true")
	}
	return t.base.Open(ctx, &transport.Request{
		Method:     req.Method,
		URL:        t.baseURL + t.path,
		Header:     header,
		Body:       bytes.NewReader(mutated),
		Extensions: req.Extensions,
	})
}

func codexExtensionsFromTransport(values map[string]any) (unified.CodexExtensions, []unified.WarningEvent) {
	var e unified.Extensions
	for key, value := range values {
		raw, ok := value.(json.RawMessage)
		if !ok {
			continue
		}
		_ = e.SetRaw(key, raw)
	}
	return unified.CodexExtensionsFrom(e)
}

func mutateCodexBody(body io.Reader) ([]byte, string, error) {
	if body == nil {
		return nil, "", nil
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, "", err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return raw, "", nil
	}
	if model, _ := payload["model"].(string); model == "" {
		payload["model"] = DefaultModel
	} else {
		payload["model"] = resolveModelAlias(model)
	}
	if instructions, _ := payload["instructions"].(string); strings.TrimSpace(instructions) == "" {
		payload["instructions"] = defaultInstructions
	}
	payload["store"] = false

	delete(payload, "max_tokens")
	delete(payload, "max_output_tokens")
	delete(payload, "temperature")
	delete(payload, "top_p")
	delete(payload, "top_k")
	delete(payload, "response_format")
	delete(payload, "prompt_cache_retention")

	promptCacheKey, _ := payload[HeaderPromptCacheKey].(string)
	encoded, err := json.Marshal(payload)
	return encoded, promptCacheKey, err
}

func resolveModelAlias(model string) string {
	switch model {
	case "", "codex":
		return DefaultModel
	case "fast":
		return "gpt-5.4-mini"
	case "powerful":
		return "o3"
	default:
		return model
	}
}

func randomInstallationID() string {
	var b [defaultInstallationIDEntropy]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
