package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
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
	if req == nil {
		return t.inner.Open(ctx, req)
	}
	replay, err := snapshotRequestBody(req)
	if err != nil {
		return nil, err
	}

	backoff := t.cfg.InitialBackoff
	var lastErr error
	for attempt := 0; attempt <= t.cfg.MaxRetries; attempt++ {
		attemptReq := *req
		attemptReq.Body = replay()
		stream, err := t.inner.Open(ctx, &attemptReq)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		if attempt == t.cfg.MaxRetries || !t.retryable(err) {
			return nil, err
		}
		delay := t.retryDelay(err, backoff)
		timer := time.NewTimer(delay)
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

func snapshotRequestBody(req *Request) (func() io.Reader, error) {
	if req == nil || req.Body == nil {
		return func() io.Reader { return nil }, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return func() io.Reader { return bytes.NewReader(body) }, nil
}

func (t *RetryTransport) retryDelay(err error, backoff time.Duration) time.Duration {
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return backoff
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

func NewDefaultRetryTransport(inner ByteStreamTransport) ByteStreamTransport {
	return NewRetryTransport(inner, RetryConfig{
		Mode:           RetryBeforeStream,
		MaxRetries:     2,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
	})
}
