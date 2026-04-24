package adapt

import (
	"context"

	"github.com/codewandler/llmadapter/unified"
)

type EventDecoder[In any, Out any] interface {
	Push(ctx context.Context, ev In) ([]Out, error)
	Close(ctx context.Context) ([]Out, error)
}

type EventEncoder[In any, Out any] interface {
	Push(ctx context.Context, ev In) ([]Out, error)
	Close(ctx context.Context) ([]Out, error)
}

type ProviderCodec[Req any, Evt any] interface {
	ApiKind() ApiKind
	EncodeRequest(ctx context.Context, req Request) (Req, error)
	NewEventDecoder() EventDecoder[Evt, unified.Event]
}

type RequestProcessor interface {
	ProcessRequest(ctx context.Context, req *Request) error
}

type ProviderRequestProcessor[Req any] interface {
	ProcessProviderRequest(ctx context.Context, req *Req) error
}

type NativeClient[Req any, Evt any] interface {
	ApiKind() ApiKind
	Request(ctx context.Context, req Req) (<-chan Evt, error)
}
