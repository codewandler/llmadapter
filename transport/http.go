package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/unified"
)

type HTTPTransportConfig struct {
	Client      *http.Client
	FrameFormat FrameFormat
}

type HTTPByteStreamTransport struct {
	client      *http.Client
	frameFormat FrameFormat
}

func NewHTTPByteStreamTransport(cfg HTTPTransportConfig) *HTTPByteStreamTransport {
	client := cfg.Client
	if client == nil {
		client = http.DefaultClient
	}
	format := cfg.FrameFormat
	if format == "" {
		format = FrameFormatRaw
	}
	return &HTTPByteStreamTransport{client: client, frameFormat: format}
}

func (t *HTTPByteStreamTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, req.Body)
	if err != nil {
		return nil, err
	}
	httpReq.Header = req.Header.Clone()

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, apiErrorFromHTTP(resp.StatusCode, body)
	}

	stream := &httpByteStream{body: resp.Body, format: t.frameFormat}
	switch t.frameFormat {
	case FrameFormatSSE:
		stream.sse = NewSSEReader(resp.Body)
	case FrameFormatNDJSON:
		stream.ndjson = NewNDJSONReader(resp.Body)
	case FrameFormatRaw, "":
		stream.format = FrameFormatRaw
	default:
		stream.format = FrameFormatRaw
	}
	return stream, nil
}

func apiErrorFromHTTP(statusCode int, body []byte) *unified.APIError {
	apiErr := &unified.APIError{
		StatusCode:  statusCode,
		Message:     strings.TrimSpace(string(body)),
		ProviderRaw: append([]byte(nil), body...),
	}
	var openAI struct {
		Error struct {
			Type    string `json:"type"`
			Code    string `json:"code"`
			Message string `json:"message"`
			Param   string `json:"param"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &openAI); err == nil && openAI.Error.Message != "" {
		apiErr.Type = openAI.Error.Type
		apiErr.Code = openAI.Error.Code
		apiErr.Message = openAI.Error.Message
		apiErr.Param = openAI.Error.Param
		return apiErr
	}
	var anthropic struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &anthropic); err == nil && anthropic.Error.Message != "" {
		apiErr.Type = anthropic.Error.Type
		apiErr.Message = anthropic.Error.Message
		return apiErr
	}
	return apiErr
}

type httpByteStream struct {
	body   io.ReadCloser
	format FrameFormat
	sse    *SSEReader
	ndjson *NDJSONReader
	raw    []byte
	sent   bool
}

func (s *httpByteStream) Recv(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	switch s.format {
	case FrameFormatSSE:
		return s.sse.Next()
	case FrameFormatNDJSON:
		return s.ndjson.Next()
	default:
		if s.sent {
			return nil, io.EOF
		}
		if s.raw == nil {
			body, err := io.ReadAll(s.body)
			if err != nil {
				return nil, err
			}
			s.raw = body
		}
		s.sent = true
		return bytes.Clone(s.raw), nil
	}
}

func (s *httpByteStream) Close() error {
	return s.body.Close()
}
