package pipeline

import "context"

type Processor[E any] interface {
	Push(ctx context.Context, ev E) ([]E, error)
	Close(ctx context.Context) ([]E, error)
}
