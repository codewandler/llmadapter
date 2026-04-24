package transport

import (
	"context"
	"errors"
	"time"

	"github.com/codewandler/llmadapter/unified"
)

type RetryMode string

const (
	RetryNever        RetryMode = "never"
	RetryBeforeStream RetryMode = "before_stream"
)

type RetryConfig struct {
	Mode            RetryMode
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	RetryableStatus func(int) bool
}

type RetryTransport struct {
	inner ByteStreamTransport
	cfg   RetryConfig
}

func NewRetryTransport(inner ByteStreamTransport, cfg RetryConfig) *RetryTransport {
	if cfg.InitialBackoff == 0 {
		cfg.InitialBackoff = 10 * time.Millisecond
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = time.Second
	}
	if cfg.RetryableStatus == nil {
		cfg.RetryableStatus = defaultRetryableStatus
	}
	return &RetryTransport{inner: inner, cfg: cfg}
}

func (t *RetryTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	if t.cfg.Mode != RetryBeforeStream {
		return t.inner.Open(ctx, req)
	}

	backoff := t.cfg.InitialBackoff
	var lastErr error
	for attempt := 0; attempt <= t.cfg.MaxRetries; attempt++ {
		stream, err := t.inner.Open(ctx, req)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if attempt == t.cfg.MaxRetries || !t.retryable(err) {
			return nil, err
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
		if backoff > t.cfg.MaxBackoff {
			backoff = t.cfg.MaxBackoff
		}
	}
	return nil, lastErr
}

func (t *RetryTransport) retryable(err error) bool {
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) {
		return t.cfg.RetryableStatus(apiErr.StatusCode)
	}
	return true
}

func defaultRetryableStatus(status int) bool {
	switch status {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}
