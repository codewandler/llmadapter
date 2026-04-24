package chatcompletions

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

const defaultBaseURL = "https://api.openai.com"

type Client struct {
	apiKey    string
	baseURL   string
	transport transport.ByteStreamTransport
}

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt.applyOpenAI(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}
	if cfg.transport == nil {
		cfg.transport = transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE})
	}
	return &Client{apiKey: cfg.apiKey, baseURL: cfg.baseURL, transport: cfg.transport}, nil
}

func (c *Client) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	wire, err := encodeRequest(req)
	if err != nil {
		return nil, err
	}
	// Keep this first OpenAI provider slice stream-first. Endpoint codecs can collect when needed.
	wire.Stream = true
	wire.StreamOptions = &streamOptionsWire{IncludeUsage: true}
	body, err := json.Marshal(wire)
	if err != nil {
		return nil, err
	}
	stream, err := c.transport.Open(ctx, &transport.Request{
		Method: http.MethodPost,
		URL:    strings.TrimRight(c.baseURL, "/") + "/v1/chat/completions",
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
	go c.readStream(ctx, stream, out)
	return out, nil
}

func (c *Client) readStream(ctx context.Context, stream transport.ByteStream, out chan<- unified.Event) {
	defer close(out)
	defer stream.Close()
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
