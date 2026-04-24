package main

import (
	"context"

	"github.com/codewandler/llmadapter/pipeline"
	"github.com/codewandler/llmadapter/unified"
)

type eventProcessorClient struct {
	inner      unified.Client
	processors []pipeline.Processor[unified.Event]
}

func (c *eventProcessorClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	events, err := c.inner.Request(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan unified.Event)
	go func() {
		defer close(out)

		chain := pipeline.NewChain(c.processors...)
		emit := func(values []unified.Event) bool {
			for _, ev := range values {
				select {
				case <-ctx.Done():
					return false
				case out <- ev:
				}
			}
			return true
		}
		fail := func(err error) {
			if err == nil {
				return
			}
			select {
			case <-ctx.Done():
			case out <- unified.ErrorEvent{Err: err}:
			}
		}

		for {
			select {
			case <-ctx.Done():
				fail(ctx.Err())
				return
			case ev, ok := <-events:
				if !ok {
					flushed, err := chain.Close(ctx)
					if err != nil {
						fail(err)
						return
					}
					emit(flushed)
					return
				}
				processed, err := chain.Push(ctx, ev)
				if err != nil {
					fail(err)
					return
				}
				if !emit(processed) {
					return
				}
			}
		}
	}()
	return out, nil
}
