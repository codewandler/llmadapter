package pipeline

import (
	"context"

	"github.com/codewandler/llmadapter/unified"
)

type ReasoningFilter struct {
	Expose bool
}

func (p ReasoningFilter) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
	if _, ok := ev.(unified.ReasoningDeltaEvent); ok && !p.Expose {
		return nil, nil
	}
	return []unified.Event{ev}, nil
}

func (p ReasoningFilter) Close(ctx context.Context) ([]unified.Event, error) {
	return nil, nil
}
