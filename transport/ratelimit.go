package transport

import "context"

type RateLimitedTransport struct {
	inner   ByteStreamTransport
	limiter RateLimiter
}

func NewRateLimitedTransport(inner ByteStreamTransport, limiter RateLimiter) *RateLimitedTransport {
	return &RateLimitedTransport{inner: inner, limiter: limiter}
}

func (t *RateLimitedTransport) Open(ctx context.Context, req *Request) (ByteStream, error) {
	if t.limiter != nil {
		if err := t.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}
	return t.inner.Open(ctx, req)
}
