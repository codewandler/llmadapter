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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
