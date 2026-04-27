package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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

const maxErrorBodyBytes = 4096

func NewHTTPByteStreamTransport(cfg HTTPTransportConfig) *HTTPByteStreamTransport {
	client := cfg.Client
	if client == nil {
		client = DefaultHTTPClient()
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, apiErrorFromHTTP(resp.StatusCode, resp.Header, body)
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

func apiErrorFromHTTP(statusCode int, header http.Header, body []byte) *unified.APIError {
	apiErr := &unified.APIError{
		StatusCode:  statusCode,
		Message:     strings.TrimSpace(string(body)),
		ProviderRaw: append([]byte(nil), body...),
	}
	apiErr.RetryAfter = retryAfter(header.Get("Retry-After"))
	var generic struct {
		Error struct {
			Type    string          `json:"type"`
			Code    json.RawMessage `json:"code"`
			Message string          `json:"message"`
			Param   string          `json:"param"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &generic); err == nil && generic.Error.Message != "" {
		apiErr.Type = generic.Error.Type
		apiErr.Code = jsonScalarString(generic.Error.Code)
		apiErr.Message = generic.Error.Message
		apiErr.Param = generic.Error.Param
		return apiErr
	}
	var topLevel struct {
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Param   string          `json:"param"`
		Detail  string          `json:"detail"`
		Error   json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &topLevel); err == nil {
		switch {
		case topLevel.Message != "":
			apiErr.Type = topLevel.Type
			apiErr.Code = jsonScalarString(topLevel.Code)
			apiErr.Message = topLevel.Message
			apiErr.Param = topLevel.Param
			return apiErr
		case topLevel.Detail != "":
			apiErr.Message = topLevel.Detail
			return apiErr
		case len(topLevel.Error) > 0:
			if msg := jsonScalarString(topLevel.Error); msg != "" {
				apiErr.Message = msg
				return apiErr
			}
		}
	}
	var miniMax struct {
		BaseResp struct {
			StatusCode json.RawMessage `json:"status_code"`
			StatusMsg  string          `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &miniMax); err == nil && miniMax.BaseResp.StatusMsg != "" {
		apiErr.Type = "minimax_error"
		apiErr.Code = jsonScalarString(miniMax.BaseResp.StatusCode)
		apiErr.Message = miniMax.BaseResp.StatusMsg
		return apiErr
	}
	return apiErr
}

func jsonScalarString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return strings.TrimSpace(string(raw))
}

func retryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay > 0 {
			return delay
		}
	}
	return 0
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
