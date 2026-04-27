package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/codewandler/llmadapter/providers/openai/internal/responsesws"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const webSocketBetaValue = "responses_websockets=2026-02-06"

func (c *Client) shouldUseWebSocket(req unified.Request, body []byte) bool {
	if c.webSocketTransport == nil {
		return false
	}
	switch c.webSocketMode {
	case WebSocketModeEnabled:
		return true
	case WebSocketModeAuto:
		return webSocketSessionID(req, body) != "" || requestPreviousResponseID(body) != ""
	default:
		return false
	}
}

func (c *Client) openWebSocket(ctx context.Context, req unified.Request, body []byte) (transport.ByteStream, error) {
	sessionID := webSocketSessionID(req, body)
	wsBody := webSocketBody(body)
	header := http.Header{
		"Authorization": []string{"Bearer " + c.apiKey},
	}
	header.Set("OpenAI-Beta", webSocketBetaValue)
	if sessionID != "" {
		header.Set("x-client-request-id", sessionID)
	}
	metadata := unified.ProviderExecutionEvent{
		Transport:            unified.TransportWebSocket,
		InternalContinuation: unified.ContinuationReplay,
	}
	if requestPreviousResponseID(wsBody) != "" {
		metadata.InternalContinuation = unified.ContinuationPreviousResponseID
	}
	if c.webSocketSession == nil {
		c.webSocketSession = responsesws.NewSession()
	}
	unlock := c.webSocketSession.Acquire()
	stream, err := c.webSocketSession.OpenOrWrite(ctx, sessionID, wsBody, func(ctx context.Context) (transport.ByteStream, error) {
		return c.webSocketTransport.Open(ctx, &transport.Request{
			Method:     http.MethodPost,
			URL:        webSocketURL(c.baseURL),
			Header:     header,
			Body:       bytes.NewReader(wsBody),
			Extensions: req.Extensions.TransportValues(),
		})
	})
	if err != nil {
		unlock()
		return nil, err
	}
	return &webSocketResponseStream{
		inner:    stream,
		metadata: metadata,
		fail: func() {
			c.webSocketSession.CloseLocked()
		},
		closeUnlock: unlock,
		keepOpen:    sessionID != "",
	}, nil
}

func webSocketBody(body []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["type"] = "response.create"
	payload["stream"] = true
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

func webSocketURL(baseURL string) string {
	raw := strings.TrimRight(baseURL, "/") + "/v1/responses"
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	return u.String()
}

func webSocketSessionID(req unified.Request, body []byte) string {
	if req.CacheKey != "" {
		return req.CacheKey
	}
	var payload struct {
		PromptCacheKey string `json:"prompt_cache_key"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.PromptCacheKey
}

func requestPreviousResponseID(body []byte) string {
	var payload struct {
		PreviousResponseID string `json:"previous_response_id"`
	}
	_ = json.Unmarshal(body, &payload)
	return payload.PreviousResponseID
}

type webSocketResponseStream struct {
	inner        transport.ByteStream
	metadata     unified.ProviderExecutionEvent
	fail         func()
	closeUnlock  func()
	keepOpen     bool
	sentMetadata bool
	pending      bytes.Buffer
}

func (s *webSocketResponseStream) Recv(ctx context.Context) ([]byte, error) {
	if !s.sentMetadata {
		s.sentMetadata = true
		return providerMetadataFrame(s.metadata)
	}
	for {
		raw, err := s.inner.Recv(ctx)
		if err != nil {
			if s.fail != nil {
				s.fail()
				s.fail = nil
			}
			if s.pending.Len() > 0 {
				return nil, fmt.Errorf("openai responses websocket: incomplete JSON event before stream ended: %w", err)
			}
			return nil, err
		}
		s.pending.Write(raw)
		candidate := bytes.TrimSpace(s.pending.Bytes())
		if len(candidate) == 0 {
			continue
		}
		if !json.Valid(candidate) {
			continue
		}
		out := append([]byte("data: "), candidate...)
		s.pending.Reset()
		return out, nil
	}
}

func (s *webSocketResponseStream) Close() error {
	if s.closeUnlock != nil {
		defer s.closeUnlock()
		s.closeUnlock = nil
	}
	if s.keepOpen {
		return nil
	}
	return s.inner.Close()
}

func providerMetadataFrame(metadata unified.ProviderExecutionEvent) ([]byte, error) {
	raw, err := json.Marshal(map[string]any{
		"type":                  "llmadapter.provider_execution",
		"transport":             metadata.Transport,
		"internal_continuation": metadata.InternalContinuation,
	})
	if err != nil {
		return nil, err
	}
	return append([]byte("data: "), raw...), nil
}

var _ transport.ByteStream = (*webSocketResponseStream)(nil)
