package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/transport"
)

type HeaderFunc func(context.Context, *http.Request) error

type NativeClient struct {
	transport     transport.ByteStreamTransport
	baseURL       string
	apiKey        string
	version       string
	headers       http.Header
	headerFns     []HeaderFunc
	quotaProvider string
}

func (c *NativeClient) ApiKind() adapt.ApiKind {
	return adapt.ApiAnthropicMessages
}

func (c *NativeClient) Request(ctx context.Context, req MessageRequest) (<-chan Event, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(c.baseURL, "/") + "/v1/messages"
	headers := c.headers.Clone()
	headers.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		headers.Set("x-api-key", c.apiKey)
	}
	headers.Set("anthropic-version", c.version)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header = headers.Clone()
	for _, fn := range c.headerFns {
		if err := fn(ctx, httpReq); err != nil {
			return nil, err
		}
	}

	stream, err := c.transport.Open(ctx, &transport.Request{
		Method: http.MethodPost,
		URL:    httpReq.URL.String(),
		Header: httpReq.Header.Clone(),
		Body:   bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}

	out := make(chan Event)
	go func() {
		defer close(out)
		defer stream.Close()
		if carrier, ok := stream.(transport.HeaderCarrier); ok {
			if quota, ok := anthropicQuotaUsageFromHeader(carrier.Header(), c.quotaProvider); ok {
				if !sendNativeEvent(ctx, out, quota) {
					return
				}
			}
		}
		decoder := &SSEFrameDecoder{}
		for {
			raw, err := stream.Recv(ctx)
			if errors.Is(err, io.EOF) {
				flushed, closeErr := decoder.Close(ctx)
				if closeErr != nil {
					sendNativeEvent(ctx, out, ErrorEventWire{Type: "error", Error: APIErrorBody{Type: "stream_error", Message: closeErr.Error()}})
					return
				}
				for _, ev := range flushed {
					if !sendNativeEvent(ctx, out, ev) {
						return
					}
				}
				return
			}
			if err != nil {
				sendNativeEvent(ctx, out, ErrorEventWire{Type: "error", Error: APIErrorBody{Type: "stream_error", Message: err.Error()}})
				return
			}
			events, err := decoder.PushFrame(ctx, raw)
			if err != nil {
				sendNativeEvent(ctx, out, ErrorEventWire{Type: "error", Error: APIErrorBody{Type: "decode_error", Message: err.Error()}})
				return
			}
			for _, ev := range events {
				if !sendNativeEvent(ctx, out, ev) {
					return
				}
			}
		}
	}()
	return out, nil
}

func sendNativeEvent(ctx context.Context, out chan<- Event, ev Event) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- ev:
		return true
	}
}
