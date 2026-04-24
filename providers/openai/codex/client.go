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
	if promptCacheKey != "" {
		header.Set(HeaderSessionID, promptCacheKey)
		header.Set(HeaderCodexWindowID, promptCacheKey+":"+defaultWindowGeneration)
	}
	return t.base.Open(ctx, &transport.Request{
		Method:     req.Method,
		URL:        t.baseURL + t.path,
		Header:     header,
		Body:       bytes.NewReader(mutated),
		Extensions: req.Extensions,
	})
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
