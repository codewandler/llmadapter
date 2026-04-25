package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/unified"
)

func TestHTTPByteStreamTransportSSE(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.Header.Get("x-test") != "1" {
			t.Fatalf("unexpected request")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("event: a\ndata: 1\n\nevent: b\ndata: 2\n\n")),
			Header:     make(http.Header),
		}, nil
	})

	tr := NewHTTPByteStreamTransport(HTTPTransportConfig{
		Client:      &http.Client{Transport: rt},
		FrameFormat: FrameFormatSSE,
	})
	stream, err := tr.Open(context.Background(), &Request{
		Method: "POST",
		URL:    "https://example.test/messages",
		Header: http.Header{"X-Test": []string{"1"}},
		Body:   strings.NewReader("{}"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	raw, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	frame, _ := ParseSSEFrame(raw)
	if frame.Event != "a" || string(frame.Data) != "1" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
	raw, err = stream.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	frame, _ = ParseSSEFrame(raw)
	if frame.Event != "b" || string(frame.Data) != "2" {
		t.Fatalf("unexpected frame: %+v", frame)
	}
	if _, err := stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("err = %v, want EOF", err)
	}
}

func TestHTTPByteStreamTransportNon2xx(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":"bad_model","message":"bad model","param":"model"}}`)),
			Header:     http.Header{"Retry-After": []string{"3"}},
		}, nil
	})

	tr := NewHTTPByteStreamTransport(HTTPTransportConfig{Client: &http.Client{Transport: rt}})
	_, err := tr.Open(context.Background(), &Request{Method: "GET", URL: "https://example.test"})
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Type != "invalid_request_error" || apiErr.Code != "bad_model" || apiErr.Message != "bad model" || apiErr.Param != "model" || apiErr.RetryAfter != 3*time.Second {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func TestHTTPByteStreamTransportNon2xxAnthropicShape(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"authentication_error","message":"bad key"}}`)),
			Header:     make(http.Header),
		}, nil
	})

	tr := NewHTTPByteStreamTransport(HTTPTransportConfig{Client: &http.Client{Transport: rt}})
	_, err := tr.Open(context.Background(), &Request{Method: "GET", URL: "https://example.test"})
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Type != "authentication_error" || apiErr.Message != "bad key" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func TestHTTPByteStreamTransportNon2xxNumericErrorCode(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","code":400,"message":"bad request"}}`)),
			Header:     make(http.Header),
		}, nil
	})

	tr := NewHTTPByteStreamTransport(HTTPTransportConfig{Client: &http.Client{Transport: rt}})
	_, err := tr.Open(context.Background(), &Request{Method: "GET", URL: "https://example.test"})
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Type != "invalid_request_error" || apiErr.Code != "400" || apiErr.Message != "bad request" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func TestHTTPByteStreamTransportNon2xxMiniMaxShape(t *testing.T) {
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":1004,"status_msg":"invalid api key"}}`)),
			Header:     make(http.Header),
		}, nil
	})

	tr := NewHTTPByteStreamTransport(HTTPTransportConfig{Client: &http.Client{Transport: rt}})
	_, err := tr.Open(context.Background(), &Request{Method: "GET", URL: "https://example.test"})
	if err == nil {
		t.Fatalf("expected error")
	}
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || apiErr.Type != "minimax_error" || apiErr.Code != "1004" || apiErr.Message != "invalid api key" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func TestHTTPByteStreamTransportProviderErrorBodyVariants(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
		wantType   string
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "top_level_message",
			statusCode: http.StatusBadRequest,
			body:       `{"type":"invalid_request_error","code":"bad_model","message":"unknown model","param":"model"}`,
			wantType:   "invalid_request_error",
			wantCode:   "bad_model",
			wantMsg:    "unknown model",
		},
		{
			name:       "detail_message",
			statusCode: http.StatusUnauthorized,
			body:       `{"detail":"authorization failed"}`,
			wantMsg:    "authorization failed",
		},
		{
			name:       "string_error",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"error":"temporarily unavailable"}`,
			wantMsg:    "temporarily unavailable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.statusCode,
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Header:     make(http.Header),
				}, nil
			})

			tr := NewHTTPByteStreamTransport(HTTPTransportConfig{Client: &http.Client{Transport: rt}})
			_, err := tr.Open(context.Background(), &Request{Method: "GET", URL: "https://example.test"})
			if err == nil {
				t.Fatalf("expected error")
			}
			var apiErr *unified.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T, want APIError", err)
			}
			if apiErr.StatusCode != tc.statusCode || apiErr.Type != tc.wantType || apiErr.Code != tc.wantCode || apiErr.Message != tc.wantMsg {
				t.Fatalf("unexpected API error: %+v", apiErr)
			}
			if string(apiErr.ProviderRaw) != tc.body {
				t.Fatalf("provider raw = %s, want %s", apiErr.ProviderRaw, tc.body)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
