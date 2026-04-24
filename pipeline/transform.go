package pipeline

import (
	"context"

	"github.com/codewandler/llmadapter/adapt"
)

type Item[T any] struct {
	Value T
	Err   error
}

func Transform[In any, Out any](ctx context.Context, in <-chan In, decoder adapt.EventDecoder[In, Out]) <-chan Item[Out] {
	out := make(chan Item[Out])
	go func() {
		defer close(out)

		emit := func(values []Out) bool {
			for _, value := range values {
				select {
				case <-ctx.Done():
					return false
				case out <- Item[Out]{Value: value}:
				}
			}
			return true
		}

		for {
			select {
			case <-ctx.Done():
				out <- Item[Out]{Err: ctx.Err()}
				return
			case ev, ok := <-in:
				if !ok {
					values, err := decoder.Close(ctx)
					if err != nil {
						out <- Item[Out]{Err: err}
						return
					}
					emit(values)
					return
				}
				values, err := decoder.Push(ctx, ev)
				if err != nil {
					out <- Item[Out]{Err: err}
					return
				}
				if !emit(values) {
					return
				}
			}
		}
	}()
	return out
}
