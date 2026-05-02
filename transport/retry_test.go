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

type sequenceTransport struct {
	errs   []error
	calls  int
	bodies []string
}

func (t *sequenceTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	t.calls++
	if req != nil && req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		t.bodies = append(t.bodies, string(body))
	}
	if len(t.errs) == 0 {
		return &fakeByteStream{}, nil
	}
	err := t.errs[0]
	t.errs = t.errs[1:]
	if err != nil {
		return nil, err
	}
	return &fakeByteStream{}, nil
}

func TestRetryTransport(t *testing.T) {
	st := &sequenceTransport{errs: []error{errors.New("temporary"), nil}}
	rt := NewRetryTransport(st, RetryConfig{Mode: RetryBeforeStream, MaxRetries: 1, InitialBackoff: time.Nanosecond})
	if _, err := rt.Open(context.Background(), &Request{}); err != nil {
		t.Fatal(err)
	}
	if st.calls != 2 {
		t.Fatalf("calls = %d, want 2", st.calls)
	}

	st = &sequenceTransport{errs: []error{errors.New("temporary"), nil}}
	rt = NewRetryTransport(st, RetryConfig{Mode: RetryNever, MaxRetries: 1})
	if _, err := rt.Open(context.Background(), &Request{}); err == nil {
		t.Fatalf("expected no retry error")
	}
	if st.calls != 1 {
		t.Fatalf("calls = %d, want 1", st.calls)
	}
}

func TestRetryTransportRetriesRetryableStatusWithReplayedBody(t *testing.T) {
	st := &sequenceTransport{errs: []error{
		&unified.APIError{StatusCode: http.StatusServiceUnavailable, Message: "temporary"},
		nil,
	}}
	rt := NewRetryTransport(st, RetryConfig{Mode: RetryBeforeStream, MaxRetries: 1, InitialBackoff: time.Nanosecond})
	if _, err := rt.Open(context.Background(), &Request{Body: strings.NewReader(`{"hello":"world"}`)}); err != nil {
		t.Fatal(err)
	}
	if st.calls != 2 {
		t.Fatalf("calls = %d, want 2", st.calls)
	}
	if len(st.bodies) != 2 || st.bodies[0] != `{"hello":"world"}` || st.bodies[1] != `{"hello":"world"}` {
		t.Fatalf("bodies = %+v", st.bodies)
	}
}

func TestRetryTransportDoesNotRetryNonRetryableStatus(t *testing.T) {
	st := &sequenceTransport{errs: []error{
		&unified.APIError{StatusCode: http.StatusBadRequest, Message: "bad request"},
		nil,
	}}
	rt := NewRetryTransport(st, RetryConfig{Mode: RetryBeforeStream, MaxRetries: 1, InitialBackoff: time.Nanosecond})
	if _, err := rt.Open(context.Background(), &Request{}); err == nil {
		t.Fatal("expected error")
	}
	if st.calls != 1 {
		t.Fatalf("calls = %d, want 1", st.calls)
	}
}

func TestRetryTransportUsesRetryAfterDelay(t *testing.T) {
	rt := NewRetryTransport(&sequenceTransport{}, RetryConfig{Mode: RetryBeforeStream})
	delay := rt.retryDelay(&unified.APIError{StatusCode: http.StatusTooManyRequests, RetryAfter: 5 * time.Nanosecond}, time.Second)
	if delay != 5*time.Nanosecond {
		t.Fatalf("delay = %v, want retry-after", delay)
	}
}
