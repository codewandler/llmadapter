package transport

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sequenceTransport struct {
	errs  []error
	calls int
}

func (t *sequenceTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	t.calls++
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
