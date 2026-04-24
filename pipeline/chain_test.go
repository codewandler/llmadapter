package pipeline

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type processorFunc struct {
	push  func(context.Context, int) ([]int, error)
	close func(context.Context) ([]int, error)
}

func (p processorFunc) Push(ctx context.Context, ev int) ([]int, error) {
	return p.push(ctx, ev)
}

func (p processorFunc) Close(ctx context.Context) ([]int, error) {
	if p.close == nil {
		return nil, nil
	}
	return p.close(ctx)
}

func TestChainPushAndCloseCascade(t *testing.T) {
	chain := NewChain[int](
		processorFunc{
			push: func(ctx context.Context, ev int) ([]int, error) {
				return []int{ev, ev + 1}, nil
			},
			close: func(ctx context.Context) ([]int, error) {
				return []int{10}, nil
			},
		},
		processorFunc{
			push: func(ctx context.Context, ev int) ([]int, error) {
				return []int{ev * 2}, nil
			},
		},
	)
	got, err := chain.Push(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Push = %v, want %v", got, want)
	}
	got, err = chain.Close(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{20}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Close = %v, want %v", got, want)
	}
}

func TestChainError(t *testing.T) {
	want := errors.New("boom")
	chain := NewChain[int](processorFunc{
		push: func(ctx context.Context, ev int) ([]int, error) {
			return nil, want
		},
	})
	if _, err := chain.Push(context.Background(), 1); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
