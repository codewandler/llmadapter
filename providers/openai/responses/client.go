package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/providers/openai/internal/responsesws"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://api.openai.com"

type Client struct {
	apiKey                     string
	baseURL                    string
	warningSource              string
	rawAPIKind                 string
	supportsPreviousResponseID bool
	bodyMutator                func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error)
	transport                  transport.ByteStreamTransport
	webSocketTransport         transport.ByteStreamTransport
	webSocketMode              WebSocketMode
	webSocketSession           *responsesws.Session
}

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL, warningSource: "openai.responses", supportsPreviousResponseID: true}
	for _, opt := range opts {
		opt.applyOpenAIResponses(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}
	if cfg.transport == nil {
		cfg.transport = transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE})
	}
	if cfg.webSocketTransport == nil && cfg.webSocketMode != WebSocketModeDefault && cfg.webSocketMode != WebSocketModeDisabled {
		cfg.webSocketTransport = NewDefaultWebSocketTransport()
	}
	return &Client{
		apiKey:                     cfg.apiKey,
		baseURL:                    cfg.baseURL,
		warningSource:              cfg.warningSource,
		rawAPIKind:                 cfg.warningSource,
		supportsPreviousResponseID: cfg.supportsPreviousResponseID,
		bodyMutator:                cfg.bodyMutator,
		transport:                  cfg.transport,
		webSocketTransport:         cfg.webSocketTransport,
		webSocketMode:              cfg.webSocketMode,
		webSocketSession:           responsesws.NewSession(),
	}, nil
}

func (c *Client) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	wire, warnings := encodeRequest(req)
	warningEvents := warningEvents(c.warningSource, warnings)
	if !c.supportsPreviousResponseID && wire.PreviousResponseID != "" {
		wire.PreviousResponseID = ""
		warningEvents = append(warningEvents, unified.WarningEvent{
			Code:    "unsupported_field_dropped",
			Message: "previous_response_id is unsupported by this provider endpoint and was dropped",
			Source:  c.warningSource,
			Meta:    map[string]any{"field": "previous_response_id"},
		})
	}
	wire.Stream = true
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	if c.bodyMutator != nil {
		var warnings []unified.WarningEvent
		body, warnings, err = c.bodyMutator(req, body)
		if err != nil {
			return nil, err
		}
		warningEvents = append(warningEvents, warnings...)
	}
	if c.shouldUseWebSocket(req, body) {
		stream, err := c.openWebSocket(ctx, req, body)
		if err == nil {
			out := make(chan unified.Event)
			go c.readStream(ctx, warningEvents, stream, out)
			return out, nil
		}
		if c.webSocketMode == WebSocketModeEnabled {
			return nil, err
		}
	}
	stream, err := c.transport.Open(ctx, &transport.Request{
		Method: http.MethodPost,
		URL:    strings.TrimRight(c.baseURL, "/") + "/v1/responses",
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiKey},
			"Content-Type":  []string{"application/json"},
		},
		Body:       bytes.NewReader(body),
		Extensions: req.Extensions.TransportValues(),
	})
	if err != nil {
		return nil, err
	}
	out := make(chan unified.Event)
	go c.readStream(ctx, warningEvents, stream, out)
	return out, nil
}

func (c *Client) readStream(ctx context.Context, warnings []unified.WarningEvent, stream transport.ByteStream, out chan<- unified.Event) {
	defer close(out)
	defer stream.Close()
	for _, warning := range warnings {
		select {
		case <-ctx.Done():
			return
		case out <- warning:
		}
	}
	decoder := streamDecoder{apiKind: c.rawAPIKind}
	for {
		raw, err := stream.Recv(ctx)
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			out <- unified.ErrorEvent{Err: err}
			return
		}
		events, err := decoder.push(raw)
		if err != nil {
			out <- unified.ErrorEvent{Err: err}
			return
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
			if _, ok := ev.(unified.CompletedEvent); ok {
				return
			}
		}
	}
}

func warningEvents(source string, warnings []mappingWarning) []unified.WarningEvent {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]unified.WarningEvent, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, warning.event(source))
	}
	return out
}
