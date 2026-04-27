package transport

import (
	"context"
	"io"
	"net/http"
)

type Request struct {
	Method     string
	URL        string
	Header     http.Header
	Body       io.Reader
	Extensions map[string]any
}

type ByteStreamTransport interface {
	Open(ctx context.Context, req *Request) (ByteStream, error)
}

type ByteStream interface {
	Recv(ctx context.Context) ([]byte, error)
	Close() error
}

type ByteStreamWriter interface {
	Write(ctx context.Context, frame []byte) error
}

type HeaderCarrier interface {
	Header() http.Header
}

type FrameFormat string

const (
	FrameFormatSSE    FrameFormat = "sse"
	FrameFormatNDJSON FrameFormat = "ndjson"
	FrameFormatRaw    FrameFormat = "raw"
)

type FrameDecoder[Evt any] interface {
	PushFrame(ctx context.Context, frame []byte) ([]Evt, error)
	Close(ctx context.Context) ([]Evt, error)
}

type RateLimiter interface {
	Wait(ctx context.Context) error
}
