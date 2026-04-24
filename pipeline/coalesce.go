package pipeline

import (
	"context"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/unified"
)

type TextCoalescer struct {
	MaxBytes int
	bufs     map[int]*strings.Builder
}

func (p *TextCoalescer) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
	delta, ok := ev.(unified.TextDeltaEvent)
	if !ok {
		flushed := p.flushAll()
		return append(flushed, ev), nil
	}
	if p.MaxBytes <= 0 {
		return []unified.Event{ev}, nil
	}
	if p.bufs == nil {
		p.bufs = make(map[int]*strings.Builder)
	}
	if p.bufs[delta.Index] == nil {
		p.bufs[delta.Index] = &strings.Builder{}
	}
	p.bufs[delta.Index].WriteString(delta.Text)
	if p.bufs[delta.Index].Len() >= p.MaxBytes {
		return p.flush(delta.Index), nil
	}
	return nil, nil
}

func (p *TextCoalescer) Close(ctx context.Context) ([]unified.Event, error) {
	return p.flushAll(), nil
}

func (p *TextCoalescer) flush(index int) []unified.Event {
	if p.bufs == nil || p.bufs[index] == nil || p.bufs[index].Len() == 0 {
		return nil
	}
	text := p.bufs[index].String()
	delete(p.bufs, index)
	return []unified.Event{unified.TextDeltaEvent{Index: index, Text: text}}
}

func (p *TextCoalescer) flushAll() []unified.Event {
	if len(p.bufs) == 0 {
		return nil
	}
	out := make([]unified.Event, 0, len(p.bufs))
	indexes := make([]int, 0, len(p.bufs))
	for index := range p.bufs {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		out = append(out, p.flush(index)...)
	}
	return out
}
