package transport

import (
	"context"
	"errors"
	"testing"
)

type limiterFunc func(context.Context) error

func (f limiterFunc) Wait(ctx context.Context) error { return f(ctx) }

func TestRateLimitedTransport(t *testing.T) {
	called := false
	inner := &FakeByteStreamTransport{}
	tr := NewRateLimitedTransport(inner, limiterFunc(func(ctx context.Context) error {
		called = true
		return nil
	}))
	if _, err := tr.Open(context.Background(), &Request{}); err != nil {
		t.Fatal(err)
	}
	if !called || len(inner.Seen) != 1 {
		t.Fatalf("limiter/open not called")
	}

	want := errors.New("limited")
	tr = NewRateLimitedTransport(inner, limiterFunc(func(ctx context.Context) error { return want }))
	if _, err := tr.Open(context.Background(), &Request{}); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
