package codex

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/codewandler/llmadapter/providers/openai/internal/responsesws"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

var (
	errCodexWebSocketClosedBeforeCompleted = errors.New("codex websocket stream closed before response completed")
	errCodexWebSocketSessionLost           = errors.New("codex websocket session lost")
)

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{
		baseURL:        DefaultBaseURL,
		path:           DefaultPath,
		installationID: randomInstallationID(),
		webSocketMode:  openairesponses.WebSocketModeAuto,
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
		base = transport.NewDefaultRetryTransport(transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{
			Client:      cfg.httpClient,
			FrameFormat: transport.FrameFormatSSE,
		}))
	}
	ws := cfg.webSocketTransport
	if ws == nil && cfg.webSocketMode != openairesponses.WebSocketModeDisabled {
		ws = openairesponses.NewDefaultWebSocketTransport()
	}
	return openairesponses.NewClient(
		openairesponses.WithAPIKey("codex-auth-via-transport"),
		openairesponses.WithBaseURL(cfg.baseURL),
		openairesponses.WithTransport(&codexTransport{
			http:             base,
			webSocket:        ws,
			webSocketMode:    cfg.webSocketMode,
			auth:             auth.WithHTTPClient(cfg.httpClient),
			baseURL:          strings.TrimRight(cfg.baseURL, "/"),
			path:             cfg.path,
			installationID:   cfg.installationID,
			betaFeatures:     cfg.betaFeatures,
			continuations:    newContinuationManager(),
			webSocketSession: responsesws.NewSession(),
		}),
		openairesponses.WithWarningSource("codex.responses"),
		openairesponses.WithPreviousResponseIDSupport(false),
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
	http             transport.ByteStreamTransport
	webSocket        transport.ByteStreamTransport
	webSocketMode    openairesponses.WebSocketMode
	auth             *Auth
	baseURL          string
	path             string
	installationID   string
	betaFeatures     string
	continuations    *continuationManager
	webSocketSession *responsesws.Session
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
	header.Set("User-Agent", codexUserAgent())
	header.Set(HeaderOriginator, CodexCLIOriginator)
	header.Set(HeaderVersion, CodexCLIVersion)
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
	branchID := codexExt.BranchID
	if branchID == "" {
		branchID = defaultBranchID
	}
	decision := t.continuationDecision(sessionID, branchID, mutated)
	if codexExt.TurnState != "" {
		header.Set(HeaderCodexTurnState, codexExt.TurnState)
	} else if decision.turnState != "" {
		header.Set(HeaderCodexTurnState, decision.turnState)
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
	if t.shouldUseWebSocket(codexExt, sessionID) {
		if t.webSocketSession == nil {
			t.webSocketSession = responsesws.NewSession()
		}
		unlockWS := t.webSocketSession.Acquire()
		webSocketHeader := codexWebSocketHeader(header, sessionID)
		if codexExt.TurnState == "" && decision.turnState != "" {
			webSocketHeader.Set(HeaderCodexTurnState, decision.turnState)
		}
		webSocketBody := withPreviousResponseID(codexWebSocketBody(mutated, codexWebSocketClientMetadata(header)), decision.previousResponseID, decision.inputStart)
		openHTTP := func(ctx context.Context) (transport.ByteStream, error) {
			stream, err := t.http.Open(ctx, &transport.Request{
				Method:     req.Method,
				URL:        t.baseURL + t.path,
				Header:     header,
				Body:       bytes.NewReader(mutated),
				Extensions: req.Extensions,
			})
			if err != nil {
				return nil, err
			}
			quotaFrames := t.captureSuccessfulResponseHeaders(stream, sessionID, branchID)
			return &sseWrappedByteStream{inner: stream, preface: quotaFrames}, nil
		}
		failWebSocketSession := func() {
			if sessionID != "" && t.webSocketSession != nil {
				t.webSocketSession.CloseLocked()
				if t.continuations != nil {
					t.continuations.invalidate(sessionID, branchID)
				}
			}
		}
		stream, err := t.webSocketStream(ctx, req, webSocketHeader, sessionID, webSocketBody)
		if err == nil {
			var quotaFrames [][]byte
			if carrier, ok := stream.(transport.HeaderCarrier); ok {
				handshake := carrier.Header()
				t.continuations.setTurnState(sessionID, branchID, headerValue(handshake, HeaderCodexTurnState))
				quotaFrames = codexQuotaFramesFromHeader(handshake)
			}
			internalContinuation := unified.ContinuationReplay
			if decision.previousResponseID != "" {
				internalContinuation = unified.ContinuationPreviousResponseID
			}
			return &codexFallbackByteStream{
				active: &sseWrappedByteStream{inner: stream, wrapFrames: true, commit: decision.commit, fail: failWebSocketSession, close: unlockWS},
				fallback: func(ctx context.Context) (transport.ByteStream, error) {
					if unlockWS == nil {
						return openHTTP(ctx)
					}
					t.webSocketSession.CloseLocked()
					unlockWS()
					unlockWS = nil
					return openHTTP(ctx)
				},
				metadata: unified.ProviderExecutionEvent{
					InternalContinuation: internalContinuation,
					Transport:            unified.TransportWebSocket,
				},
				preface: quotaFrames,
			}, nil
		}
		if errors.Is(err, errCodexWebSocketSessionLost) {
			if t.continuations != nil {
				t.continuations.invalidate(sessionID, branchID)
			}
		}
		if unlockWS != nil {
			unlockWS()
		}
	}
	stream, err := t.http.Open(ctx, &transport.Request{
		Method:     req.Method,
		URL:        t.baseURL + t.path,
		Header:     header,
		Body:       bytes.NewReader(mutated),
		Extensions: req.Extensions,
	})
	if err != nil {
		return nil, err
	}
	quotaFrames := t.captureSuccessfulResponseHeaders(stream, sessionID, branchID)
	return &sseWrappedByteStream{inner: stream, metadata: unified.ProviderExecutionEvent{
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
	}, preface: quotaFrames}, nil
}

func (t *codexTransport) captureSuccessfulResponseHeaders(stream transport.ByteStream, sessionID, branchID string) [][]byte {
	carrier, ok := stream.(transport.HeaderCarrier)
	if !ok {
		return nil
	}
	header := carrier.Header()
	if t.continuations != nil {
		t.continuations.setTurnState(sessionID, branchID, headerValue(header, HeaderCodexTurnState))
	}
	return codexQuotaFramesFromHeader(header)
}

func (t *codexTransport) webSocketStream(ctx context.Context, req *transport.Request, header http.Header, sessionID string, body []byte) (transport.ByteStream, error) {
	if t.webSocketSession == nil {
		t.webSocketSession = responsesws.NewSession()
	}
	stream, err := t.webSocketSession.OpenOrWrite(ctx, sessionID, body, func(ctx context.Context) (transport.ByteStream, error) {
		return t.webSocket.Open(ctx, &transport.Request{
			Method:     req.Method,
			URL:        codexWebSocketURL(t.baseURL, t.path),
			Header:     header,
			Body:       bytes.NewReader(body),
			Extensions: req.Extensions,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errCodexWebSocketSessionLost, err)
	}
	return stream, nil
}

func (t *codexTransport) continuationDecision(sessionID, branchID string, body []byte) continuationDecision {
	if t.continuations == nil {
		return continuationDecision{}
	}
	return t.continuations.prepare(sessionID, branchID, body)
}

func (t *codexTransport) shouldUseWebSocket(ext unified.CodexExtensions, sessionID string) bool {
	if t.webSocket == nil {
		return false
	}
	if t.webSocketMode == openairesponses.WebSocketModeDisabled {
		return false
	}
	if t.webSocketMode == openairesponses.WebSocketModeEnabled {
		return sessionID != ""
	}
	switch ext.InteractionMode {
	case unified.InteractionOneShot:
		return false
	case unified.InteractionSession:
		return sessionID != ""
	case unified.InteractionAuto, "":
		return sessionID != ""
	default:
		return false
	}
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
	delete(payload, "previous_response_id")
	if text, ok := payload["text"].(map[string]any); ok && len(text) == 0 {
		delete(payload, "text")
	}

	promptCacheKey, _ := payload[HeaderPromptCacheKey].(string)
	encoded, err := json.Marshal(payload)
	return encoded, promptCacheKey, err
}

func codexWebSocketBody(body []byte, clientMetadata map[string]string) []byte {
	if len(body) == 0 {
		return []byte(`{"type":"response.create"}`)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["type"] = "response.create"
	if _, ok := payload["tools"]; !ok {
		payload["tools"] = []any{}
	}
	if _, ok := payload["tool_choice"]; !ok {
		payload["tool_choice"] = "auto"
	}
	if _, ok := payload["parallel_tool_calls"]; !ok {
		payload["parallel_tool_calls"] = true
	}
	if _, ok := payload["reasoning"]; !ok {
		payload["reasoning"] = nil
	}
	payload["stream"] = true
	payload["store"] = false
	if _, ok := payload["include"]; !ok {
		payload["include"] = []any{}
	}
	if len(clientMetadata) > 0 {
		payload["client_metadata"] = clientMetadata
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

func codexWebSocketHeader(header http.Header, sessionID string) http.Header {
	out := header.Clone()
	out.Del("Content-Type")
	out.Del("User-Agent")
	out.Del(HeaderCodexInstallationID)
	out.Set(HeaderOpenAIBeta, WebSocketBetaValue)
	if sessionID != "" {
		out.Set("x-client-request-id", sessionID)
	}
	return out
}

func codexWebSocketClientMetadata(header interface{ Get(string) string }) map[string]string {
	out := map[string]string{}
	for _, key := range []string{
		HeaderCodexInstallationID,
		HeaderCodexWindowID,
		HeaderOpenAISubagent,
		HeaderCodexParentThreadID,
		HeaderCodexTurnMetadata,
	} {
		if value := header.Get(key); value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func codexUserAgent() string {
	return fmt.Sprintf("%s/%s (%s %s; %s) %s",
		CodexCLIOriginator,
		CodexCLIVersion,
		codexOSType(),
		codexOSVersion(),
		codexArchitecture(),
		codexTerminalToken(),
	)
}

func codexOSType() string {
	if runtime.GOOS == "linux" {
		return "Linux"
	}
	return runtime.GOOS
}

func codexOSVersion() string {
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if value, ok := strings.CutPrefix(line, "VERSION_ID="); ok {
					return strings.Trim(value, `"`)
				}
			}
		}
	}
	return "unknown"
}

func codexArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func codexTerminalToken() string {
	if value := strings.TrimSpace(os.Getenv("TERM_PROGRAM")); value != "" {
		if version := strings.TrimSpace(os.Getenv("TERM_PROGRAM_VERSION")); version != "" {
			return sanitizeCodexUserAgentToken(value + "/" + version)
		}
		return sanitizeCodexUserAgentToken(value)
	}
	if os.Getenv("TERM_SESSION_ID") != "" {
		return "Apple_Terminal"
	}
	if value := strings.TrimSpace(os.Getenv("TERM")); value != "" {
		return sanitizeCodexUserAgentToken(value)
	}
	return "unknown"
}

func sanitizeCodexUserAgentToken(value string) string {
	var b strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '/' {
			b.WriteRune(ch)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

func codexWebSocketURL(baseURL, path string) string {
	raw := strings.TrimRight(baseURL, "/") + path
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

type sseWrappedByteStream struct {
	inner          transport.ByteStream
	wrapFrames     bool
	commit         func(string)
	fail           func()
	close          func()
	lastResponseID string
	completed      bool
	metadata       unified.ProviderExecutionEvent
	sentMetadata   bool
	preface        [][]byte
	pending        bytes.Buffer
}

func (s *sseWrappedByteStream) Recv(ctx context.Context) ([]byte, error) {
	if !s.sentMetadata && (s.metadata.Transport != "" || s.metadata.InternalContinuation != "") {
		s.sentMetadata = true
		return providerMetadataFrame(s.metadata)
	}
	if len(s.preface) > 0 {
		out := s.preface[0]
		s.preface = s.preface[1:]
		return out, nil
	}
	raw, err := s.recvRaw(ctx)
	if err != nil {
		if s.wrapFrames && !s.completed && s.fail != nil {
			s.fail()
			s.fail = nil
		}
		if s.pending.Len() > 0 {
			return nil, fmt.Errorf("codex websocket: incomplete JSON event before stream ended: %w", err)
		}
		if s.wrapFrames && !s.completed && errors.Is(err, io.EOF) {
			return nil, errCodexWebSocketClosedBeforeCompleted
		}
		return nil, err
	}
	if responseID := responseIDFromFrame(raw); responseID != "" {
		s.lastResponseID = responseID
	}
	if frameCompleted(raw) {
		s.completed = true
	}
	if !s.wrapFrames {
		return raw, nil
	}
	return append([]byte("data: "), raw...), nil
}

func (s *sseWrappedByteStream) recvRaw(ctx context.Context) ([]byte, error) {
	if !s.wrapFrames {
		return s.inner.Recv(ctx)
	}
	for {
		raw, err := s.inner.Recv(ctx)
		if err != nil {
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
		if quota := codexQuotaUsageJSONFromRateLimitEvent(candidate); len(quota) > 0 {
			out := append([]byte(nil), quota...)
			s.pending.Reset()
			return out, nil
		}
		out := append([]byte(nil), candidate...)
		s.pending.Reset()
		return out, nil
	}
}

func (s *sseWrappedByteStream) Close() error {
	var err error
	if s.completed && s.commit != nil {
		s.commit(s.lastResponseID)
		s.commit = nil
	} else if s.wrapFrames && s.fail != nil {
		s.fail()
		s.fail = nil
	}
	if s.close != nil {
		s.close()
		s.close = nil
	} else {
		err = s.inner.Close()
	}
	return err
}

func frameCompleted(raw []byte) bool {
	var payload struct {
		Type     string `json:"type"`
		Response struct {
			Status string `json:"status"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	return payload.Type == "response.done" || payload.Type == "response.completed" || payload.Response.Status == "completed"
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

type codexFallbackByteStream struct {
	active       transport.ByteStream
	fallback     func(context.Context) (transport.ByteStream, error)
	metadata     unified.ProviderExecutionEvent
	sentMetadata bool
	preface      [][]byte
	started      bool
	buffered     [][]byte
}

func (s *codexFallbackByteStream) Recv(ctx context.Context) ([]byte, error) {
	for {
		if s.started && !s.sentMetadata {
			s.sentMetadata = true
			return providerMetadataFrame(s.metadata)
		}
		if s.started && len(s.preface) > 0 {
			out := s.preface[0]
			s.preface = s.preface[1:]
			return out, nil
		}
		if len(s.buffered) > 0 {
			out := s.buffered[0]
			s.buffered = s.buffered[1:]
			return out, nil
		}
		raw, err := s.active.Recv(ctx)
		if err != nil {
			if !s.started && s.fallback != nil && errors.Is(err, errCodexWebSocketClosedBeforeCompleted) {
				stream, fallbackErr := s.fallback(ctx)
				if fallbackErr != nil {
					return nil, fallbackErr
				}
				s.active = stream
				s.fallback = nil
				s.metadata = unified.ProviderExecutionEvent{
					InternalContinuation: unified.ContinuationReplay,
					Transport:            unified.TransportHTTPSSE,
				}
				s.preface = nil
				s.started = true
				continue
			}
			return nil, err
		}
		if !s.started {
			s.buffered = append(s.buffered, raw)
			if isIgnorableCodexPreStartFrame(raw) {
				s.buffered = s.buffered[:len(s.buffered)-1]
				continue
			}
			if !isCodexCommitFrame(raw) {
				continue
			}
			s.started = true
			continue
		}
		return raw, nil
	}
}

func (s *codexFallbackByteStream) Close() error {
	if s.active == nil {
		return nil
	}
	return s.active.Close()
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

func isIgnorableCodexPreStartFrame(raw []byte) bool {
	eventType := codexFrameType(raw)
	if eventType == quotaUsageEventType {
		return false
	}
	return eventType != "" && !strings.HasPrefix(eventType, "response.") && eventType != "error"
}

func isCodexCommitFrame(raw []byte) bool {
	eventType := codexFrameType(raw)
	return eventType == "response.output_text.delta" ||
		eventType == "response.function_call_arguments.delta" ||
		eventType == "response.done" ||
		eventType == "response.completed"
}

func codexFrameType(raw []byte) string {
	payload := bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(raw), []byte("data: ")))
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return ""
	}
	return event.Type
}
