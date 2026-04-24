package pipeline

import "context"

type Chain[E any] struct {
	processors []Processor[E]
}

func NewChain[E any](processors ...Processor[E]) *Chain[E] {
	return &Chain[E]{processors: processors}
}

func (c *Chain[E]) Push(ctx context.Context, ev E) ([]E, error) {
	current := []E{ev}
	for _, p := range c.processors {
		next := make([]E, 0, len(current))
		for _, item := range current {
			produced, err := p.Push(ctx, item)
			if err != nil {
				return nil, err
			}
			next = append(next, produced...)
		}
		current = next
		if len(current) == 0 {
			break
		}
	}
	return current, nil
}

func (c *Chain[E]) Close(ctx context.Context) ([]E, error) {
	var out []E
	for i, p := range c.processors {
		produced, err := p.Close(ctx)
		if err != nil {
			return nil, err
		}
		cascaded, err := c.pushThrough(ctx, produced, i+1)
		if err != nil {
			return nil, err
		}
		out = append(out, cascaded...)
	}
	return out, nil
}

func (c *Chain[E]) pushThrough(ctx context.Context, values []E, start int) ([]E, error) {
	current := values
	for _, p := range c.processors[start:] {
		next := make([]E, 0, len(current))
		for _, item := range current {
			produced, err := p.Push(ctx, item)
			if err != nil {
				return nil, err
			}
			next = append(next, produced...)
		}
		current = next
		if len(current) == 0 {
			break
		}
	}
	return current, nil
}
