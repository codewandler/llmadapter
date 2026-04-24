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

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://openrouter.ai/api"

type Client struct {
	apiKey        string
	baseURL       string
	warningSource string
	transport     transport.ByteStreamTransport
}

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt.applyOpenRouterResponses(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openrouter API key is required")
	}
	if cfg.transport == nil {
		cfg.transport = transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE})
	}
	if cfg.warningSource == "" {
		cfg.warningSource = "openrouter.responses"
	}
	return &Client{apiKey: cfg.apiKey, baseURL: cfg.baseURL, warningSource: cfg.warningSource, transport: cfg.transport}, nil
}

func (c *Client) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	wire, warnings := encodeRequest(req)
	// Keep this first Responses slice stream-first. Endpoint codecs can collect when needed.
	wire.Stream = true
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	stream, err := c.transport.Open(ctx, &transport.Request{
		Method: http.MethodPost,
		URL:    strings.TrimRight(c.baseURL, "/") + "/v1/responses",
		Header: http.Header{
			"Authorization": []string{"Bearer " + c.apiKey},
			"Content-Type":  []string{"application/json"},
		},
		Body: bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}
	out := make(chan unified.Event)
	go c.readStream(ctx, warnings, stream, out)
	return out, nil
}

func (c *Client) readStream(ctx context.Context, warnings []mappingWarning, stream transport.ByteStream, out chan<- unified.Event) {
	defer close(out)
	defer stream.Close()
	for _, warning := range warnings {
		select {
		case <-ctx.Done():
			return
		case out <- warning.event(c.warningSource):
		}
	}
	decoder := streamDecoder{}
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
		}
	}
}
