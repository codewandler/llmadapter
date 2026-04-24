package pipeline

import (
	"context"

	"github.com/codewandler/llmadapter/unified"
)

type CompletionInjector struct {
	seen bool
}

func (p *CompletionInjector) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
	if _, ok := ev.(unified.CompletedEvent); ok {
		p.seen = true
	}
	return []unified.Event{ev}, nil
}

func (p *CompletionInjector) Close(ctx context.Context) ([]unified.Event, error) {
	if p.seen {
		return nil, nil
	}
	p.seen = true
	return []unified.Event{unified.CompletedEvent{FinishReason: unified.FinishReasonUnknown}}, nil
}
